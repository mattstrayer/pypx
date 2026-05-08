package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pypx/api/internal/osv"
	"github.com/pypx/api/internal/pypi"
	"github.com/pypx/api/internal/search"
	"github.com/pypx/api/internal/textfmt"
)

const compareMaxPackages = 5

// CompareHandler serves the cross-package compare endpoint. It composes data
// from existing services rather than fetching anything new.
type CompareHandler struct {
	pkg    *PackageHandler
	pypi   *pypi.Client
	osv    *osv.Client
	search *search.Index
}

// NewCompareHandler creates a CompareHandler with the dependencies it needs.
func NewCompareHandler(pkg *PackageHandler, pypiClient *pypi.Client, osvClient *osv.Client, idx *search.Index) *CompareHandler {
	return &CompareHandler{pkg: pkg, pypi: pypiClient, osv: osvClient, search: idx}
}

// Get handles GET /api/compare.txt?pkgs=a,b,c.
func (h *CompareHandler) Get(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimSpace(r.URL.Query().Get("pkgs"))
	if raw == "" {
		http.Error(w, "query parameter 'pkgs' is required", http.StatusBadRequest)
		return
	}

	parts := strings.Split(raw, ",")
	names := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for _, p := range parts {
		n := strings.ToLower(strings.TrimSpace(p))
		if n == "" || seen[n] {
			continue
		}
		if err := pypi.ValidateName(n); err != nil {
			http.Error(w, "invalid package name: "+n, http.StatusBadRequest)
			return
		}
		seen[n] = true
		names = append(names, n)
	}

	if len(names) == 0 {
		http.Error(w, "no valid package names provided", http.StatusBadRequest)
		return
	}
	if len(names) > compareMaxPackages {
		http.Error(w, "too many packages, max 5", http.StatusBadRequest)
		return
	}

	// Fan out per-package fetches in parallel; preserve input order.
	type slot struct {
		input   *textfmt.ComparePackageInput
		skipped *textfmt.SkippedPackage
	}
	results := make([]slot, len(names))
	var wg sync.WaitGroup
	wg.Add(len(names))
	for i, name := range names {
		i, name := i, name
		go func() {
			defer wg.Done()
			results[i] = h.fetchOne(r.Context(), name)
		}()
	}
	wg.Wait()

	in := &textfmt.CompareInput{}
	for _, res := range results {
		if res.skipped != nil {
			in.Skipped = append(in.Skipped, *res.skipped)
		}
		if res.input != nil {
			in.Packages = append(in.Packages, *res.input)
		}
	}

	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(textfmt.FormatCompare(in))) //nolint:errcheck
}

// fetchOne assembles one package's compare row. Returns either an input
// (success) or a SkippedPackage (failure). Best-effort secondary signals
// (vulns, downloads, typed) are not allowed to skip the whole package.
func (h *CompareHandler) fetchOne(ctx context.Context, name string) (out struct {
	input   *textfmt.ComparePackageInput
	skipped *textfmt.SkippedPackage
}) {
	resp, err := h.pkg.FetchPackage(ctx, name)
	if err != nil {
		switch {
		case errors.Is(err, pypi.ErrNotFound):
			s := textfmt.SkippedPackage{Name: name, Reason: "not found"}
			out.skipped = &s
		default:
			s := textfmt.SkippedPackage{Name: name, Reason: "fetch error"}
			out.skipped = &s
		}
		return
	}

	pkg := buildPackageResponse(resp)
	row := textfmt.ComparePackageInput{
		Name:             pkg.Name,
		Version:          pkg.Version,
		Summary:          pkg.Summary,
		License:          pkg.License,
		PythonMin:        pkg.PythonVersions.MinVersion,
		InstallSize:      pkg.InstallSize,
		ModuleFormat:     pkg.ModuleFormat,
		LastReleasedDate: dateOnly(pkg.ReleaseCadence.LastReleasedAt),
		ReleasesLast12Mo: pkg.ReleaseCadence.ReleasesLast12Mo,
		DepCount:         len(pkg.Dependencies.Required),
		RepoURL:          extractRepoURL(pkg.ProjectURLs),
		DocURL:           pkg.DocURL,
	}

	// Best-effort secondary signals — run in parallel with a short shared timeout.
	ctx2, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var sg sync.WaitGroup
	sg.Add(3)
	go func() {
		defer sg.Done()
		if vulns, verr := h.osv.FetchVulns(ctx2, name, ""); verr == nil {
			row.VulnCount = len(vulns)
		}
	}()
	go func() {
		defer sg.Done()
		if entry, ok, lerr := h.search.Lookup(name); lerr == nil && ok {
			row.Downloads30d = entry.Downloads
		}
	}()
	go func() {
		defer sg.Done()
		ts := pypi.CheckTypeSupport(ctx2, h.pypi, name)
		switch ts.Status {
		case "typed":
			row.Typed = "yes"
		case "stubs":
			row.Typed = "stubs"
		default:
			row.Typed = "no"
		}
	}()
	sg.Wait()

	out.input = &row
	return
}

// dateOnly returns the YYYY-MM-DD prefix of a timestamp string.
func dateOnly(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// extractRepoURL returns a repo URL from project_urls using a case-insensitive
// priority lookup. Returns empty if no candidate found.
func extractRepoURL(urls map[string]string) string {
	if len(urls) == 0 {
		return ""
	}
	lower := make(map[string]string, len(urls))
	for k, v := range urls {
		lower[strings.ToLower(k)] = v
	}
	for _, key := range []string{"source", "source code", "repository", "homepage", "github"} {
		if v, ok := lower[key]; ok && v != "" {
			return v
		}
	}
	return ""
}
