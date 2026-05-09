package handler

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/enrichment"
	"github.com/pypx/api/internal/pypi"
	"github.com/pypx/api/internal/textfmt"
)

const apiDiffCap = 200

// DiffHandler serves /api/packages/{name}/diff.txt — three independent
// best-effort sections (changelog slice, dep changes, api changes) assembled
// into one markdown document.
type DiffHandler struct {
	pypi      *pypi.Client
	cache     cache.Cacher
	docs      *DocsHandler
	changelog *ChangelogHandler
	pkg       *PackageHandler
}

// NewDiffHandler creates a DiffHandler.
func NewDiffHandler(pypiClient *pypi.Client, c cache.Cacher, docs *DocsHandler, cl *ChangelogHandler, pkg *PackageHandler) *DiffHandler {
	return &DiffHandler{pypi: pypiClient, cache: c, docs: docs, changelog: cl, pkg: pkg}
}

// Get handles GET /api/packages/{name}/diff.txt?from=&to=.
func (h *DiffHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !validateName(w, name) {
		return
	}

	from := strings.TrimSpace(r.URL.Query().Get("from"))
	to := strings.TrimSpace(r.URL.Query().Get("to"))
	if from == "" || to == "" {
		http.Error(w, "both from and to are required", http.StatusBadRequest)
		return
	}
	if compareVersions(from, to) >= 0 {
		http.Error(w, "from must be older than to", http.StatusBadRequest)
		return
	}

	cacheKey := "diff:" + strings.ToLower(name) + ":" + from + ":" + to
	if data, _, cerr := h.cache.Get(cacheKey, 0); cerr == nil && data != nil {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(data) //nolint:errcheck
		return
	}

	// Existence check: fetch the package once (cached) to confirm it exists and
	// that both versions are real releases.
	pkgResp, err := h.pkg.FetchPackage(r.Context(), name)
	if err != nil {
		if errors.Is(err, pypi.ErrNotFound) {
			http.Error(w, "package not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch package", http.StatusBadGateway)
		return
	}
	if _, ok := pkgResp.Releases[from]; !ok {
		http.Error(w, "from version does not exist: "+from, http.StatusBadRequest)
		return
	}
	if _, ok := pkgResp.Releases[to]; !ok {
		http.Error(w, "to version does not exist: "+to, http.StatusBadRequest)
		return
	}

	// Fan out three best-effort fetches.
	in := &textfmt.DiffInput{Package: name, From: from, To: to}
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		entries, unavailable := h.changelog.fetchChangelogSlice(r.Context(), name, from, to)
		if unavailable != "" {
			in.ChangelogUnavailable = unavailable
		} else {
			in.Changelog = entries
		}
	}()

	go func() {
		defer wg.Done()
		dd, unavailable := h.fetchDepDiff(r.Context(), name, from, to)
		if unavailable != "" {
			in.DepChangesUnavailable = unavailable
		} else {
			in.DepChanges = dd
		}
	}()

	go func() {
		defer wg.Done()
		ad, unavailable := h.fetchAPIDiff(r.Context(), name, from, to)
		if unavailable != "" {
			in.APIChangesUnavailable = unavailable
		} else {
			in.APIChanges = ad
		}
	}()

	wg.Wait()

	body := textfmt.FormatDiff(in)
	h.cache.Set(cacheKey, []byte(body), 0) //nolint:errcheck

	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(body)) //nolint:errcheck
}

// fetchDepDiff fetches per-version requires_dist and computes the diff.
func (h *DiffHandler) fetchDepDiff(ctx context.Context, name, from, to string) (textfmt.DepDiff, string) {
	type result struct {
		deps []enrichment.Dependency
		err  error
	}
	fromCh := make(chan result, 1)
	toCh := make(chan result, 1)

	go func() {
		resp, err := h.pypi.FetchPackageAtVersion(ctx, name, from)
		if err != nil {
			fromCh <- result{nil, err}
			return
		}
		tree := enrichment.ParseDependencies(resp.Info.RequiresDist)
		fromCh <- result{tree.Required, nil}
	}()
	go func() {
		resp, err := h.pypi.FetchPackageAtVersion(ctx, name, to)
		if err != nil {
			toCh <- result{nil, err}
			return
		}
		tree := enrichment.ParseDependencies(resp.Info.RequiresDist)
		toCh <- result{tree.Required, nil}
	}()

	fromRes := <-fromCh
	toRes := <-toCh
	if fromRes.err != nil {
		return textfmt.DepDiff{}, "could not fetch metadata for " + from
	}
	if toRes.err != nil {
		return textfmt.DepDiff{}, "could not fetch metadata for " + to
	}

	fromMap := make(map[string]string, len(fromRes.deps))
	for _, d := range fromRes.deps {
		fromMap[strings.ToLower(d.Name)] = d.Constraint
	}
	toMap := make(map[string]string, len(toRes.deps))
	for _, d := range toRes.deps {
		toMap[strings.ToLower(d.Name)] = d.Constraint
	}

	var dd textfmt.DepDiff
	for n := range toMap {
		if _, ok := fromMap[n]; !ok {
			dd.Added = append(dd.Added, n)
		}
	}
	for n := range fromMap {
		if _, ok := toMap[n]; !ok {
			dd.Removed = append(dd.Removed, n)
		}
	}
	for n, fromC := range fromMap {
		if toC, ok := toMap[n]; ok && toC != fromC {
			dd.Bumped = append(dd.Bumped, textfmt.DepBump{Name: n, FromConstraint: fromC, ToConstraint: toC})
		}
	}
	sort.Strings(dd.Added)
	sort.Strings(dd.Removed)
	sort.Slice(dd.Bumped, func(i, j int) bool { return dd.Bumped[i].Name < dd.Bumped[j].Name })
	return dd, ""
}

// fetchAPIDiff extracts docs for both versions and computes the symbol-level diff.
func (h *DiffHandler) fetchAPIDiff(ctx context.Context, name, from, to string) (textfmt.APIDiff, string) {
	type result struct {
		resp DocsResponse
		err  error
	}
	fromCh := make(chan result, 1)
	toCh := make(chan result, 1)

	go func() {
		resp, _, err := h.docs.fetchDocsAtVersion(ctx, name, from)
		fromCh <- result{resp, err}
	}()
	go func() {
		resp, _, err := h.docs.fetchDocsAtVersion(ctx, name, to)
		toCh <- result{resp, err}
	}()

	fromRes := <-fromCh
	toRes := <-toCh
	switch {
	case fromRes.err != nil && toRes.err != nil:
		return textfmt.APIDiff{}, "could not extract docs for either version"
	case fromRes.err != nil:
		return textfmt.APIDiff{}, "could not extract docs for " + from
	case toRes.err != nil:
		return textfmt.APIDiff{}, "could not extract docs for " + to
	}

	fromSyms := flattenSymbols(fromRes.resp)
	toSyms := flattenSymbols(toRes.resp)

	var ad textfmt.APIDiff
	for path := range toSyms {
		if _, ok := fromSyms[path]; !ok {
			ad.Added = append(ad.Added, path)
		}
	}
	for path := range fromSyms {
		if _, ok := toSyms[path]; !ok {
			ad.Removed = append(ad.Removed, path)
		}
	}
	for path, fromSig := range fromSyms {
		if toSig, ok := toSyms[path]; ok && toSig != fromSig {
			ad.Changed = append(ad.Changed, textfmt.APIChange{Path: path, FromSig: fromSig, ToSig: toSig})
		}
	}
	sort.Strings(ad.Added)
	sort.Strings(ad.Removed)
	sort.Slice(ad.Changed, func(i, j int) bool { return ad.Changed[i].Path < ad.Changed[j].Path })

	if len(ad.Added) > apiDiffCap {
		ad.AddedTruncated = len(ad.Added) - apiDiffCap
		ad.Added = ad.Added[:apiDiffCap]
	}
	if len(ad.Removed) > apiDiffCap {
		ad.RemovedTruncated = len(ad.Removed) - apiDiffCap
		ad.Removed = ad.Removed[:apiDiffCap]
	}
	if len(ad.Changed) > apiDiffCap {
		ad.ChangedTruncated = len(ad.Changed) - apiDiffCap
		ad.Changed = ad.Changed[:apiDiffCap]
	}
	return ad, ""
}

// flattenSymbols walks a DocsResponse tree and returns a dotted-path → signature
// map for all functions, classes, methods, and exceptions across all modules.
// On dotted-path collisions across modules (rare in valid Python), last-write-wins.
func flattenSymbols(d DocsResponse) map[string]string {
	out := make(map[string]string)
	for _, mod := range d.Modules {
		for _, fn := range mod.Functions {
			out[mod.Name+"."+fn.Name] = fn.Signature
		}
		for _, cls := range mod.Classes {
			path := mod.Name + "." + cls.Name
			out[path] = cls.Signature
			for _, m := range cls.Methods {
				out[path+"."+m.Name] = m.Signature
			}
		}
		for _, exc := range mod.Exceptions {
			out[mod.Name+"."+exc.Name] = exc.Signature
		}
	}
	return out
}
