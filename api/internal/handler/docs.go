package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/pypi"
	"github.com/pypx/api/internal/textfmt"
	"github.com/pypx/goopy"
	"github.com/pypx/goopy/model"
	"github.com/pypx/goopy/wheel"
	"golang.org/x/sync/semaphore"
	"golang.org/x/sync/singleflight"
)

// DocsResponse is the response for GET /api/packages/{name}/docs.
type DocsResponse struct {
	Package   string `json:"package"`
	Version   string `json:"version"`
	Available bool   `json:"available"`
	// Reason says why Available is false when the cause is known: see the
	// docsReason constants. Empty for a package that parsed but has no public API.
	Reason      string      `json:"reason,omitempty"`
	StubPackage string      `json:"stub_package,omitempty"`
	Modules     []DocModule `json:"modules"`
}

// Values of DocsResponse.Reason.
const (
	docsReasonNoArtifact = "no_artifact"
	docsReasonTooLarge   = "too_large"
)

// DocModule is a Python module in the documentation.
type DocModule struct {
	Name       string      `json:"name"`
	Functions  []DocSymbol `json:"functions"`
	Classes    []DocSymbol `json:"classes"`
	Exceptions []DocSymbol `json:"exceptions"`
	// Aliases lists names this module re-exports whose documentation is
	// published under another module.
	Aliases []DocAlias `json:"aliases,omitempty"`
}

// DocAlias is a public name that points at a symbol documented elsewhere.
type DocAlias struct {
	Name string `json:"name"`
	// Target is the dotted module and name of the documented symbol.
	Target string `json:"target"`
}

// DocSymbol is a single documented symbol (function, class, or exception).
type DocSymbol struct {
	Name       string      `json:"name"`
	Kind       string      `json:"kind"`
	Signature  string      `json:"signature"`
	Docstring  string      `json:"docstring"`
	Parameters []DocParam  `json:"parameters"`
	Returns    *DocReturn  `json:"returns,omitempty"`
	Raises     []DocRaise  `json:"raises,omitempty"`
	Methods    []DocSymbol `json:"methods,omitempty"`
	// Attributes is set on classes and exceptions, as an empty list if they have
	// none. omitzero keeps an empty list and drops the nil list of other kinds.
	Attributes []DocAttribute `json:"attributes,omitzero"`
	// Origin is the dotted module that defines a re-exported symbol.
	Origin string `json:"origin,omitempty"`
	// MethodKind is "classmethod" or "staticmethod" on a method bound that way, and
	// empty on any other symbol. The parameter list leaves out cls, so it cannot
	// show this.
	MethodKind string `json:"method_kind,omitempty"`
}

// DocAttribute is a class attribute or property.
type DocAttribute struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Default     string `json:"default,omitempty"`
	// DefaultDigest identifies the full value when Default is truncated, so a diff
	// sees a change past the cut.
	DefaultDigest string `json:"default_digest,omitempty"`
	Description   string `json:"description"`
	IsProperty    bool   `json:"is_property"`
	Writable      bool   `json:"writable"`
}

// DocParam is a function parameter.
type DocParam struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Description string `json:"description"`
	Kind        string `json:"kind,omitempty"`
	Default     string `json:"default,omitempty"`
}

// DocReturn is the return type annotation and description.
type DocReturn struct {
	Type        string `json:"type,omitempty"`
	Description string `json:"description"`
}

// DocRaise is an exception documented in a docstring.
type DocRaise struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

const (
	// extractionBudget caps the weight of the extractions running at once. The
	// weight is goopy.WeighFromPyPI: source plus compressed bytes. Live memory
	// reaches several times that once the source is parsed. Two large packages
	// such as transformers and torch never fit together, and small ones still do.
	extractionBudget = 64 << 20
	// minExtractionCost is the least an extraction is charged. It bounds how many
	// run at once when all are small, and it pays for weighing one.
	minExtractionCost = 4 << 20
	// unweighedExtractionCost is charged when the weight is unknown.
	unweighedExtractionCost = extractionBudget / 2
)

const (
	// docsExtractTimeout bounds one shared extraction, including the wait for a
	// free extraction slot.
	docsExtractTimeout = 60 * time.Second
	// docsLatestTTL is how long a name-to-latest-version answer is reused.
	docsLatestTTL = 5 * time.Minute
)

// DocsHandler serves rendered API documentation for a package.
type DocsHandler struct {
	pypi  *pypi.Client
	cache cache.Cacher
	// extract downloads and parses a distribution. Tests replace it.
	extract func(ctx context.Context, name, version string) (*model.Package, error)
	// weigh estimates the memory one extraction holds. Nil means unknown.
	weigh func(ctx context.Context, name, version string) (int64, error)
	// sf collapses concurrent extractions of one (name, version) into one.
	sf singleflight.Group
	// budget limits the weight of concurrent extractions to extractionBudget.
	budget *semaphore.Weighted
	// workTimeout bounds one shared extraction. Tests shorten it.
	workTimeout time.Duration
	// stubs replaces resolveStubs in tests.
	stubs func(ctx context.Context, name, version string) (stubIndex, string)
}

// NewDocsHandler creates a new DocsHandler.
func NewDocsHandler(pypiClient *pypi.Client, c cache.Cacher) *DocsHandler {
	return &DocsHandler{
		pypi:        pypiClient,
		cache:       c,
		extract:     goopy.ExtractDocsFromPyPI,
		weigh:       goopy.WeighFromPyPI,
		budget:      semaphore.NewWeighted(extractionBudget),
		workTimeout: docsExtractTimeout,
	}
}

// Get handles GET /api/packages/{name}/docs.
func (h *DocsHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !validateName(w, name) {
		return
	}

	encoded, err := h.fetchDocsBytes(r.Context(), name)
	if err != nil {
		switch {
		case errors.Is(err, ErrDocsPackageNotFound):
			http.Error(w, "package not found", http.StatusNotFound)
		default:
			http.Error(w, "documentation extraction failed", http.StatusBadGateway)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(encoded) //nolint:errcheck
}

// DocsStatusResponse is the response for GET /api/packages/{name}/docs/status.
type DocsStatusResponse struct {
	Available bool `json:"available"`
}

// GetStatus handles GET /api/packages/{name}/docs/status. It reports whether
// docs exist without sending the full docs payload.
func (h *DocsHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !validateName(w, name) {
		return
	}

	encoded, err := h.fetchDocsBytes(r.Context(), name)
	if err != nil {
		switch {
		case errors.Is(err, ErrDocsPackageNotFound):
			http.Error(w, "package not found", http.StatusNotFound)
		default:
			http.Error(w, "documentation extraction failed", http.StatusBadGateway)
		}
		return
	}

	available, _ := docsAvailable(encoded)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DocsStatusResponse{Available: available}) //nolint:errcheck
}

// Sentinel errors for the docs path. Callers map these to HTTP statuses.
var (
	ErrDocsPackageNotFound  = errors.New("docs: package not found")
	ErrDocsExtractionFailed = errors.New("docs: extraction failed")
	ErrDocsUpstreamFailed   = errors.New("docs: upstream metadata fetch failed")
)

// fetchDocs returns the cached or freshly-extracted DocsResponse for a package's
// LATEST version, after resolving it via PyPI.
func (h *DocsHandler) fetchDocs(ctx context.Context, name string) (DocsResponse, []byte, error) {
	version, err := h.latestVersion(ctx, name)
	if err != nil {
		return DocsResponse{}, nil, err
	}
	return h.docsAtVersion(ctx, name, version, true)
}

// fetchDocsBytes returns the encoded docs JSON for a package's latest version.
// A cache hit returns the stored bytes without decoding them.
func (h *DocsHandler) fetchDocsBytes(ctx context.Context, name string) ([]byte, error) {
	version, err := h.latestVersion(ctx, name)
	if err != nil {
		return nil, err
	}
	_, data, err := h.docsAtVersion(ctx, name, version, false)
	return data, err
}

// latestVersion resolves the latest version of a package. The answer is reused
// for docsLatestTTL. If PyPI fails, a stale answer is served.
func (h *DocsHandler) latestVersion(ctx context.Context, name string) (string, error) {
	key := "docs-latest:" + strings.ToLower(name)
	data, fresh, cerr := h.cache.Get(key, docsLatestTTL)
	if cerr == nil && data != nil && fresh {
		return string(data), nil
	}

	pkg, err := h.pypi.FetchPackage(ctx, name)
	if err != nil {
		if errors.Is(err, pypi.ErrNotFound) {
			return "", ErrDocsPackageNotFound
		}
		if cerr == nil && data != nil {
			return string(data), nil
		}
		return "", fmt.Errorf("%w: %v", ErrDocsUpstreamFailed, err)
	}
	h.cache.Set(key, []byte(pkg.Info.Version), docsLatestTTL) //nolint:errcheck
	return pkg.Info.Version, nil
}

// docsAvailable reads the "available" field of encoded docs JSON. It stops at
// that field, so it does not decode the modules. ok is false if the JSON is
// malformed or has no such field.
func docsAvailable(data []byte) (available, ok bool) {
	dec := json.NewDecoder(bytes.NewReader(data))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return false, false
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return false, false
		}
		key, _ := keyTok.(string)
		if key == "available" {
			var v bool
			if err := dec.Decode(&v); err != nil {
				return false, false
			}
			return v, true
		}
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return false, false
		}
	}
	return false, false
}

// DocsCacheKey returns the cache key for extracted docs. The key embeds
// goopy.ExtractorVersion so an extractor change makes old entries unreachable.
func DocsCacheKey(name, version string) string {
	return fmt.Sprintf("docs:v%d:%s:%s", goopy.ExtractorVersion, strings.ToLower(name), version)
}

// DocsErrKey returns the cache key for a cached docs extraction error.
func DocsErrKey(name, version string) string {
	return fmt.Sprintf("docs-err:v%d:%s:%s", goopy.ExtractorVersion, strings.ToLower(name), version)
}

// DiffTextKey and DiffJSONKey return cache keys for rendered diffs. Diffs derive
// from docs, so they carry the same extractor version.
func DiffTextKey(name, from, to string) string {
	return fmt.Sprintf("diff:v%d:%s:%s:%s", goopy.ExtractorVersion, strings.ToLower(name), from, to)
}

func DiffJSONKey(name, from, to string) string {
	return fmt.Sprintf("diffjson:v%d:%s:%s:%s", goopy.ExtractorVersion, strings.ToLower(name), from, to)
}

const (
	// docsErrTTL is how long a docs extraction failure suppresses retries.
	docsErrTTL = 5 * time.Minute
	// docsEmptyTTL is how long an Available:false result is served before
	// extraction runs again. Available:true results never expire.
	docsEmptyTTL = 24 * time.Hour
)

// isClientCancel reports whether a failure came from the caller cancelling the
// request (for example a closed browser tab). That says nothing about the
// package, so it must not write the shared error key. Deadline expiry does not
// count: a package that always times out should stay cached as failed.
func isClientCancel(ctx context.Context, err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled)
}

// fetchDocsAtVersion returns the cached or freshly-extracted DocsResponse for
// a specific (name, version) pair. Cache key is "docs:v{ExtractorVersion}:{name}:{version}",
// indefinite TTL. The diff handler uses it for arbitrary historical versions. On
// cache hit returns the original cached bytes (no re-marshal).
//
// A version with a Reason returns ErrDocsExtractionFailed: its empty module list
// says nothing about its API, so a diff must not read it as "everything removed".
func (h *DocsHandler) fetchDocsAtVersion(ctx context.Context, name, version string) (DocsResponse, []byte, error) {
	resp, data, err := h.docsAtVersion(ctx, name, version, true)
	if err == nil && resp.Reason != "" {
		return DocsResponse{}, nil, ErrDocsExtractionFailed
	}
	return resp, data, err
}

// docsAtVersion is fetchDocsAtVersion with a choice about decoding. With
// decode false, it returns only the bytes and the DocsResponse is zero.
func (h *DocsHandler) docsAtVersion(ctx context.Context, name, version string, decode bool) (DocsResponse, []byte, error) {
	if resp, data, ok := h.cachedDocs(name, version, decode); ok {
		return resp, data, nil
	}

	errKey := DocsErrKey(name, version)
	if data, fresh, cerr := h.cache.Get(errKey, docsErrTTL); cerr == nil && data != nil && fresh {
		return DocsResponse{}, nil, ErrDocsExtractionFailed
	}

	if ctx.Err() != nil {
		return DocsResponse{}, nil, ErrDocsExtractionFailed
	}

	// Concurrent misses for one (name, version) share one extraction. The shared
	// work runs on its own context, so one caller leaving does not fail the
	// others. A caller whose own context ends returns without writing anything.
	ch := h.sf.DoChan(strings.ToLower(name)+":"+version, func() (any, error) {
		workCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), h.workTimeout)
		defer cancel()
		return h.extractAndCache(workCtx, name, version)
	})
	select {
	case res := <-ch:
		if res.Err != nil {
			return DocsResponse{}, nil, res.Err
		}
		encoded := res.Val.([]byte)
		if !decode {
			return DocsResponse{}, encoded, nil
		}
		var resp DocsResponse
		if err := json.Unmarshal(encoded, &resp); err != nil {
			return DocsResponse{}, nil, ErrDocsExtractionFailed
		}
		return resp, encoded, nil
	case <-ctx.Done():
		return DocsResponse{}, nil, ErrDocsExtractionFailed
	}
}

// cachedDocs returns a usable cached docs entry. A corrupt entry, or an
// Available:false entry past docsEmptyTTL, counts as a miss.
func (h *DocsHandler) cachedDocs(name, version string, decode bool) (DocsResponse, []byte, bool) {
	data, fresh, err := h.cache.Get(DocsCacheKey(name, version), docsEmptyTTL)
	if err != nil || data == nil {
		return DocsResponse{}, nil, false
	}
	if decode {
		var resp DocsResponse
		if json.Unmarshal(data, &resp) == nil && (resp.Available || fresh) {
			return resp, data, true
		}
		return DocsResponse{}, nil, false
	}
	if available, ok := docsAvailable(data); ok && (available || fresh) {
		return DocsResponse{}, data, true
	}
	return DocsResponse{}, nil, false
}

// admit waits until the extraction's weight fits in the budget, and returns the
// amount to release. A weight above the budget is charged as the whole budget, so
// that extraction runs alone. The wait ends when ctx ends.
func (h *DocsHandler) admit(ctx context.Context, name, version string) (int64, error) {
	cost, err := h.extractionCost(ctx, name, version)
	if err != nil {
		return 0, err
	}
	if err := h.budget.Acquire(ctx, cost); err != nil {
		return 0, err
	}
	return cost, nil
}

// extractionCost weighs an extraction while holding minExtractionCost, so a burst
// of cold packages cannot read many central directories at once. A failure to
// weigh is not an error here: the extraction itself reports the real failure.
func (h *DocsHandler) extractionCost(ctx context.Context, name, version string) (int64, error) {
	if h.weigh == nil {
		return unweighedExtractionCost, nil
	}
	if err := h.budget.Acquire(ctx, minExtractionCost); err != nil {
		return 0, err
	}
	defer h.budget.Release(minExtractionCost)

	weight, err := h.weigh(ctx, name, version)
	if err != nil {
		return unweighedExtractionCost, nil
	}
	return min(max(weight, minExtractionCost), extractionBudget), nil
}

// extractAndCache runs one extraction within the memory budget and stores the
// result. ctx is detached from any request, so only a real failure or the work
// timeout writes the error key.
func (h *DocsHandler) extractAndCache(ctx context.Context, name, version string) (encoded []byte, err error) {
	errKey := DocsErrKey(name, version)

	// singleflight re-panics in a new goroutine that nothing can recover, so a
	// panic here would end the process. Contain it and cache it as a failure.
	defer func() {
		if p := recover(); p != nil {
			log.Printf("docs: panic extracting %s==%s: %v\n%s", name, version, p, debug.Stack())
			h.cache.Set(errKey, []byte(fmt.Sprintf("panic: %v", p)), docsErrTTL) //nolint:errcheck
			encoded, err = nil, ErrDocsExtractionFailed
		}
	}()

	// Stubs come first, outside the budget. A slow stub download then delays only
	// this package, and other extractions can run while it waits.
	stubs, stubPkgName := h.findStubs(ctx, name, version)

	cost, aerr := h.admit(ctx, name, version)
	if aerr != nil {
		return nil, ErrDocsExtractionFailed
	}
	defer h.budget.Release(cost)

	result, gerr := h.extract(ctx, name, version)
	if gerr != nil {
		if isClientCancel(ctx, gerr) {
			return nil, ErrDocsExtractionFailed
		}
		// PyPI answered 404 for this version. No key is written, so the next request
		// asks PyPI again and also gets a 404.
		if errors.Is(gerr, wheel.ErrNotFound) {
			return nil, ErrDocsPackageNotFound
		}
		if reason := docsFailureReason(gerr); reason != "" {
			return h.storeDocs(name, version, DocsResponse{
				Package: name, Version: version, Reason: reason, Modules: []DocModule{},
			})
		}
		log.Printf("docs: extraction failed for %s==%s: %v", name, version, gerr)
		h.cache.Set(errKey, []byte(gerr.Error()), docsErrTTL) //nolint:errcheck
		return nil, ErrDocsExtractionFailed
	}
	log.Print(extractionLogLine(name, version, result))
	if ctx.Err() != nil {
		if isClientCancel(ctx, nil) {
			return nil, ErrDocsExtractionFailed
		}
		h.cache.Set(errKey, []byte("extraction interrupted by timeout"), docsErrTTL) //nolint:errcheck
		return nil, ErrDocsExtractionFailed
	}

	docsResp := convertToDocsResponse(name, version, result, stubs)
	docsResp.StubPackage = stubPkgName

	// resolveStubs swallows its errors, so a timeout during stub resolution
	// would otherwise cache an incomplete result.
	if ctx.Err() != nil {
		return nil, ErrDocsExtractionFailed
	}

	return h.storeDocs(name, version, docsResp)
}

// storeDocs encodes and caches one result. An Available:false row is extracted
// again after docsEmptyTTL. The caller gets the bytes only, so the decoded form is
// not held next to them.
func (h *DocsHandler) storeDocs(name, version string, resp DocsResponse) ([]byte, error) {
	encoded, err := encodeDocs(resp)
	if err != nil {
		return nil, fmt.Errorf("encode docs: %w", err)
	}
	h.cache.Set(DocsCacheKey(name, version), encoded, 0) //nolint:errcheck
	return encoded, nil
}

// maxEncodedDocs caps the encoded docs of one package. Sources are capped at 200 MB,
// so real packages stay far below it. It also keeps the size sum from overflowing.
const maxEncodedDocs = 1 << 30

// errEncodedDocsTooLarge means a package's encoded docs exceed maxEncodedDocs.
var errEncodedDocsTooLarge = errors.New("docs: encoded docs exceed size limit")

// encodeDocs returns the same bytes as json.Marshal(resp), one module at a time.
// json.Marshal holds the whole response and a buffer that grows by doubling, then
// copies that buffer. For a package with 60 MiB of docs that exceeds the live
// memory of the rest of the extraction. Here each module is released once encoded.
func encodeDocs(resp DocsResponse) ([]byte, error) {
	return encodeDocsWithin(resp, maxEncodedDocs)
}

// encodeDocsWithin is encodeDocs with the size limit as a parameter, so a test can
// reach it without a gigabyte of input.
func encodeDocsWithin(resp DocsResponse, limit int) ([]byte, error) {
	modules := resp.Modules
	resp.Modules = []DocModule{}
	head, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	// Modules is the last field, so head ends with its empty array and the brace.
	head = head[:len(head)-len("]}")]

	if len(head) > limit-len(modules)-len("]}") {
		return nil, errEncodedDocsTooLarge
	}
	chunks := make([][]byte, len(modules))
	size := len(head) + len(modules) + len("]}")
	for i := range modules {
		chunk, err := json.Marshal(&modules[i])
		if err != nil {
			return nil, err
		}
		if len(chunk) > limit-size {
			return nil, errEncodedDocsTooLarge
		}
		chunks[i] = chunk
		size += len(chunk)
		modules[i] = DocModule{}
	}

	out := make([]byte, 0, size)
	out = append(out, head...)
	for i, chunk := range chunks {
		if i > 0 {
			out = append(out, ',')
		}
		out = append(out, chunk...)
		chunks[i] = nil
	}
	return append(out, "]}"...), nil
}

// docsFailureReason returns the Reason for an extraction failure that will repeat
// for this release, or "" for any other failure.
func docsFailureReason(err error) string {
	switch {
	case errors.Is(err, wheel.ErrNoArtifact):
		return docsReasonNoArtifact
	case errors.Is(err, wheel.ErrTooLarge):
		return docsReasonTooLarge
	}
	return ""
}

// writeReasonText sends the plain-text answer for a release with a Reason. It
// reports false, and writes nothing, if resp has none. The answer can change with
// the next release, so it does not get the immutable header of real docs.
func writeReasonText(w http.ResponseWriter, resp DocsResponse) bool {
	var why string
	switch resp.Reason {
	case "":
		return false
	case docsReasonTooLarge:
		why = "this release is too large to read"
	default:
		why = "this release has no wheel and no readable Python source"
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintf(w, "No API documentation for %s %s: %s.\n", resp.Package, resp.Version, why)
	return true
}

// findStubs returns the stub index for a package. Tests replace it through h.stubs.
func (h *DocsHandler) findStubs(ctx context.Context, name, version string) (stubIndex, string) {
	if h.stubs != nil {
		return h.stubs(ctx, name, version)
	}
	return resolveStubs(ctx, h.pypi, strings.ToLower(name), version)
}

// resolveStubs fetches and parses a stub package for the given source package.
// Returns the populated stubIndex and the stub package name used (empty if none).
// All failures are silent — stubs are best-effort enrichment only.
func resolveStubs(ctx context.Context, pypiClient *pypi.Client, pkgName, sourceVersion string) (stubIndex, string) {
	stubPkgName, ok := lookupStubPackage(pkgName)
	if !ok {
		return make(stubIndex), ""
	}

	stubPyPI, err := pypiClient.FetchPackage(ctx, stubPkgName)
	if err != nil {
		log.Printf("stubs: failed to fetch %s metadata: %v", stubPkgName, err)
		return make(stubIndex), ""
	}

	sourceMajor := strings.SplitN(sourceVersion, ".", 2)[0]
	stubVersion := resolveStubVersion(stubPyPI.Releases, sourceMajor)
	if stubVersion == "" {
		log.Printf("stubs: no releases found for %s", stubPkgName)
		return make(stubIndex), ""
	}

	stubPkg, err := fetchStubPackage(ctx, stubPkgName, stubPyPI.Releases, stubVersion)
	if err != nil {
		log.Printf("stubs: failed to extract %s@%s: %v", stubPkgName, stubVersion, err)
		return make(stubIndex), ""
	}

	return buildStubIndex(stubPkg), stubPkgName
}

// convertToDocsResponse transforms goopy's model.Package into the API response
// format expected by the frontend.
func convertToDocsResponse(name, version string, pkg *model.Package, stubs stubIndex) DocsResponse {
	var modules []DocModule
	type moduleAliases struct {
		index   int
		aliases []*model.Alias
	}
	var aliases []moduleAliases
	exceptions := exceptionClasses(pkg)

	for _, mod := range pkg.Modules {
		dm := DocModule{
			Name:       mod.Name,
			Functions:  make([]DocSymbol, 0, len(mod.Functions)),
			Classes:    make([]DocSymbol, 0),
			Exceptions: make([]DocSymbol, 0),
		}

		for _, fn := range mod.Functions {
			dm.Functions = append(dm.Functions, convertFunction(fn, stubForFunction(stubs, mod.Name, fn)))
		}

		for _, cls := range mod.Classes {
			sym := convertClass(cls, stubs, mod.Name)
			if exceptions[cls] {
				sym.Kind = "exception"
				dm.Exceptions = append(dm.Exceptions, sym)
			} else {
				dm.Classes = append(dm.Classes, sym)
			}
		}

		if len(dm.Functions) > 0 || len(dm.Classes) > 0 || len(dm.Exceptions) > 0 || len(mod.Aliases) > 0 {
			modules = append(modules, dm)
			if len(mod.Aliases) > 0 {
				aliases = append(aliases, moduleAliases{len(modules) - 1, mod.Aliases})
			}
		}
	}

	// The API documents functions, classes and exceptions. Keep only the aliases
	// whose target is one of those, then drop any module left empty.
	documented := map[string]bool{}
	for _, dm := range modules {
		for _, group := range [][]DocSymbol{dm.Functions, dm.Classes, dm.Exceptions} {
			for _, sym := range group {
				documented[dm.Name+"."+sym.Name] = true
			}
		}
	}
	for _, ma := range aliases {
		for _, a := range ma.aliases {
			if documented[a.Target] {
				modules[ma.index].Aliases = append(modules[ma.index].Aliases, DocAlias{Name: a.Name, Target: a.Target})
			}
		}
	}
	modules = slices.DeleteFunc(modules, func(dm DocModule) bool {
		return len(dm.Functions) == 0 && len(dm.Classes) == 0 && len(dm.Exceptions) == 0 && len(dm.Aliases) == 0
	})

	if modules == nil {
		modules = []DocModule{}
	}

	return DocsResponse{
		Package:   name,
		Version:   version,
		Available: len(modules) > 0,
		Modules:   modules,
	}
}

// originKey returns the dotted path of a re-exported definition in its defining
// module, or "" if the definition is not a re-export.
func originKey(origin, originName, name string) string {
	if origin == "" {
		return ""
	}
	if originName != "" {
		name = originName
	}
	return origin + "." + name
}

// stubForFunction finds a function's stub under its published path, then under
// the path where it is defined. A stub package need not re-export a name the
// same way the source package does.
func stubForFunction(stubs stubIndex, modName string, fn *model.Function) *model.Function {
	if s := stubs[modName+"."+fn.Name]; s != nil {
		return s
	}
	if key := originKey(fn.Origin, fn.OriginName, fn.Name); key != "" {
		return stubs[key]
	}
	return nil
}

func convertFunction(fn *model.Function, stubFn *model.Function) DocSymbol {
	sym := DocSymbol{
		Name:       fn.Name,
		Kind:       "function",
		Docstring:  cleanDocstringText(fn.Docstring),
		Parameters: make([]DocParam, 0, len(fn.Parameters)),
		Origin:     fn.Origin,
		MethodKind: methodKind(fn),
	}
	// Note: Signature is set after the parameter loop so stub-filled types appear.

	// Build stub parameter lookup by name for O(1) access.
	var stubParams map[string]*model.Parameter
	if stubFn != nil && len(stubFn.Parameters) > 0 {
		stubParams = make(map[string]*model.Parameter, len(stubFn.Parameters))
		for _, sp := range stubFn.Parameters {
			stubParams[sp.Name] = sp
		}
	}

	for _, p := range fn.Parameters {
		dp := DocParam{Name: p.Name}
		if p.Type != nil {
			dp.Type = p.Type.Raw
		} else if p.DocParam != nil && p.DocParam.Type != "" {
			dp.Type = p.DocParam.Type
		} else if sp, ok := stubParams[p.Name]; ok && sp.Type != nil && sp.Type.Raw != "" {
			// No source annotation or docstring type — fall back to stub.
			dp.Type = sp.Type.Raw
		}
		if p.DocParam != nil {
			dp.Description = p.DocParam.Description
		}
		dp.Kind = string(p.Kind)
		dp.Default = p.Default
		sym.Parameters = append(sym.Parameters, dp)
	}

	// Returns: source annotation > docstring > stub.
	if fn.Returns != nil {
		r := &DocReturn{Type: fn.Returns.Raw}
		if fn.Docstring != nil && fn.Docstring.Returns != nil {
			r.Description = fn.Docstring.Returns.Description
		}
		sym.Returns = r
	} else if fn.Docstring != nil && fn.Docstring.Returns != nil && fn.Docstring.Returns.Type != "" {
		sym.Returns = &DocReturn{
			Type:        fn.Docstring.Returns.Type,
			Description: fn.Docstring.Returns.Description,
		}
	} else if stubFn != nil && stubFn.Returns != nil && stubFn.Returns.Raw != "" {
		sym.Returns = &DocReturn{Type: stubFn.Returns.Raw}
	}

	// Generators document Yields in place of Returns; show it as the return description.
	if fn.Docstring != nil && fn.Docstring.Returns == nil && fn.Docstring.Yields != nil {
		if sym.Returns == nil {
			sym.Returns = &DocReturn{}
		}
		if sym.Returns.Description == "" {
			sym.Returns.Description = fn.Docstring.Yields.Description
		}
	}

	// Build signature after merge so stub-filled types appear in the displayed signature.
	sym.Signature = buildMergedSignature(fn, sym.Parameters, sym.Returns)

	// Raises: populate from docstring (stubs don't carry raise docs).
	if fn.Docstring != nil && len(fn.Docstring.Raises) > 0 {
		sym.Raises = make([]DocRaise, 0, len(fn.Docstring.Raises))
		for _, r := range fn.Docstring.Raises {
			sym.Raises = append(sym.Raises, DocRaise{
				Type:        r.Type,
				Description: r.Description,
			})
		}
	}

	return sym
}

// methodKind reads how a method is bound from its decorators. A decorator keeps its
// module prefix in the model, as in abc.abstractclassmethod.
func methodKind(fn *model.Function) string {
	if !fn.IsMethod {
		return ""
	}
	for _, dec := range fn.Decorators {
		switch dec[strings.LastIndex(dec, ".")+1:] {
		case "classmethod", "abstractclassmethod":
			return "classmethod"
		case "staticmethod", "abstractstaticmethod":
			return "staticmethod"
		}
	}
	return ""
}

func convertClass(cls *model.Class, stubs stubIndex, modName string) DocSymbol {
	sym := DocSymbol{
		Name:      cls.Name,
		Kind:      "class",
		Signature: buildClassSignature(cls),
		Docstring: cleanDocstringText(cls.Docstring),
		Origin:    cls.Origin,
	}

	// A member's stub is under the class's published path or under its origin.
	originPath := originKey(cls.Origin, cls.OriginName, cls.Name)
	stubForMember := func(member string) *model.Function {
		if s := stubs[modName+"."+cls.Name+"."+member]; s != nil {
			return s
		}
		if originPath != "" {
			return stubs[originPath+"."+member]
		}
		return nil
	}

	// Constructor parameters; empty slice (not null) so JSON matches the type.
	sym.Parameters = make([]DocParam, 0)
	if cls.Constructor != nil {
		ctor := convertFunction(cls.Constructor, stubForMember("__init__"))
		sym.Parameters = ctor.Parameters
	}

	if len(cls.Methods) > 0 {
		sym.Methods = make([]DocSymbol, 0, len(cls.Methods))
		for _, m := range cls.Methods {
			sym.Methods = append(sym.Methods, convertFunction(m, stubForMember(m.Name)))
		}
	}

	sym.Attributes = make([]DocAttribute, 0, len(cls.Attributes))
	for _, a := range cls.Attributes {
		if isDunder(a.Name) {
			continue
		}
		attr := DocAttribute{
			Name:        a.Name,
			Description: cleanDocstringText(a.Docstring),
			IsProperty:  a.IsProperty,
			Writable:    a.Writable,
		}
		// An __init__ assignment holds an expression, not a default.
		if !a.Instance {
			attr.Default = truncateDefault(a.Value)
			if attr.Default != a.Value {
				sum := sha256.Sum256([]byte(a.Value))
				attr.DefaultDigest = hex.EncodeToString(sum[:8])
			}
		}
		if a.Type != nil {
			attr.Type = a.Type.Raw
		}
		sym.Attributes = append(sym.Attributes, attr)
	}

	return sym
}

// isDunder reports whether name has the form __name__. A mangled __name is not one.
func isDunder(name string) bool {
	return len(name) > 4 && strings.HasPrefix(name, "__") && strings.HasSuffix(name, "__")
}

// maxDefaultRunes caps an attribute default in API output, ellipsis included.
const maxDefaultRunes = 120

// truncateDefault shortens a long default on a rune boundary.
func truncateDefault(s string) string {
	if utf8.RuneCountInString(s) <= maxDefaultRunes {
		return s
	}
	return string([]rune(s)[:maxDefaultRunes-1]) + "…"
}

// buildMergedSignature builds a function signature using the already-merged DocParam
// slice (which may include stub-backfilled types) rather than the raw model parameters.
func buildMergedSignature(fn *model.Function, params []DocParam, returns *DocReturn) string {
	var b strings.Builder
	if fn.IsAsync {
		b.WriteString("async ")
	}
	b.WriteString("def ")
	b.WriteString(fn.Name)
	b.WriteByte('(')

	for i, dp := range params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(dp.Name)
		if dp.Type != "" {
			b.WriteString(": ")
			b.WriteString(dp.Type)
		}
		if dp.Default != "" {
			b.WriteString(" = ")
			b.WriteString(dp.Default)
		}
	}

	b.WriteByte(')')
	if returns != nil && returns.Type != "" {
		b.WriteString(" -> ")
		b.WriteString(returns.Type)
	}
	return b.String()
}

func buildClassSignature(cls *model.Class) string {
	var b strings.Builder
	b.WriteString("class ")
	b.WriteString(cls.Name)
	if len(cls.BaseClasses) > 0 {
		b.WriteByte('(')
		for i, base := range cls.BaseClasses {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(base.Name)
		}
		b.WriteByte(')')
	}
	return b.String()
}

// structuredSections lists the section names the API returns as structured
// data (params, returns, raises, yields). cleanDocstringText removes them from
// the prose. Every other section, such as Examples or Notes, stays.
var structuredSections = map[string]bool{
	"args": true, "arguments": true, "parameters": true, "params": true,
	"other parameters": true, "returns": true, "return": true,
	"raises": true, "raise": true, "yields": true, "yield": true,
}

// cleanDocstringText returns the docstring prose with the sections removed
// that the API renders as structured data. All other text stays in order.
func cleanDocstringText(ds *model.Docstring) string {
	if ds == nil {
		return ""
	}
	lines := strings.Split(ds.Text, "\n")
	var removed []bool
	switch ds.Style {
	case model.DocstringGoogle:
		removed = markGoogleRemovals(lines)
	case model.DocstringNumpy:
		removed = markNumpyRemovals(lines)
	case model.DocstringSphinx:
		removed = markSphinxRemovals(lines)
	default:
		return strings.TrimSpace(ds.Text)
	}

	// Rebuild the text. A blank line that directly follows another blank line
	// across a removal is dropped, so a removal never leaves 3+ newlines.
	out := make([]string, 0, len(lines))
	removedSinceEmit := false
	for i, line := range lines {
		if removed[i] {
			removedSinceEmit = true
			continue
		}
		if removedSinceEmit && strings.TrimSpace(line) == "" &&
			len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
			continue
		}
		removedSinceEmit = false
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// unmarkTrailingBlanks keeps blank lines at the end of a removed span, so the
// gap between the removed section and the next text survives.
func unmarkTrailingBlanks(lines []string, removed []bool, start, end int) {
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	for k := start; k < end; k++ {
		removed[k] = true
	}
}

func isFlushLeft(line string) bool {
	return line != "" && line[0] != ' ' && line[0] != '\t'
}

func isGoogleHeader(line string) bool {
	return isFlushLeft(line) && strings.HasSuffix(line, ":") &&
		isGoogleSection(strings.TrimSuffix(line, ":"))
}

// markGoogleRemovals marks structured sections. A section runs from its
// flush-left header to the next flush-left header, or to the next flush-left
// non-blank line that follows a blank line.
func markGoogleRemovals(lines []string) []bool {
	removed := make([]bool, len(lines))
	for i := 0; i < len(lines); i++ {
		t := strings.TrimRight(lines[i], " \t")
		if !isGoogleHeader(t) || !structuredSections[strings.ToLower(strings.TrimSuffix(t, ":"))] {
			continue
		}
		j := i + 1
		sawBlank := false
		for ; j < len(lines); j++ {
			l := strings.TrimRight(lines[j], " \t")
			if l == "" {
				sawBlank = true
				continue
			}
			if isFlushLeft(l) && (isGoogleHeader(l) || sawBlank) {
				break
			}
		}
		unmarkTrailingBlanks(lines, removed, i, j)
		i = j - 1
	}
	return removed
}

func isNumpyUnderline(line string) bool {
	t := strings.TrimSpace(line)
	return len(t) >= 3 && strings.Trim(t, "-") == ""
}

// markNumpyRemovals marks structured sections. A section runs from a
// name-plus-dashes pair to the next name-plus-dashes pair.
func markNumpyRemovals(lines []string) []bool {
	removed := make([]bool, len(lines))
	isHeader := func(i int) bool {
		return i+1 < len(lines) && strings.TrimSpace(lines[i]) != "" && isNumpyUnderline(lines[i+1])
	}
	for i := 0; i < len(lines); i++ {
		if !isHeader(i) || !structuredSections[strings.ToLower(strings.TrimSpace(lines[i]))] {
			continue
		}
		j := i + 2
		for j < len(lines) && !isHeader(j) {
			j++
		}
		unmarkTrailingBlanks(lines, removed, i, j)
		i = j - 1
	}
	return removed
}

// markSphinxRemovals marks each field line and its indented continuation lines.
func markSphinxRemovals(lines []string) []bool {
	removed := make([]bool, len(lines))
	for i := 0; i < len(lines); i++ {
		if !sphinxFieldRe.MatchString(strings.TrimSpace(lines[i])) {
			continue
		}
		removed[i] = true
		for i+1 < len(lines) && strings.TrimSpace(lines[i+1]) != "" && !isFlushLeft(lines[i+1]) {
			i++
			removed[i] = true
		}
	}
	return removed
}

// sphinxFieldRe matches a Sphinx field list line such as ":param x:" or
// ":raises ValueError:". Inline roles like ":class:`Foo`" do not match.
var sphinxFieldRe = regexp.MustCompile(
	`^:(param|parameter|arg|argument|key|keyword|type|returns?|rtype|raises?|except|exception|var|ivar|cvar|meta)\b[^:]*:`,
)

func isGoogleSection(word string) bool {
	switch word {
	case "Args", "Arguments", "Parameters", "Params", "Other Parameters",
		"Returns", "Return", "Raises", "Raise",
		"Note", "Notes", "Example", "Examples",
		"Attributes", "Yields", "Yield", "Todo",
		"References", "See Also", "Warnings", "Warns":
		return true
	}
	return false
}

// baseSegment returns the last dotted segment of a base class, without type
// arguments: "errors.Base[T]" gives "Base".
func baseSegment(base string) string {
	base, _, _ = strings.Cut(base, "[")
	return base[strings.LastIndex(base, ".")+1:]
}

// isExceptionName reports whether a base class name is an exception by its name.
// The builtins in the switch are the ones that do not follow the suffix rule.
func isExceptionName(base string) bool {
	segment := baseSegment(base)
	switch segment {
	case "BaseExceptionGroup", "ExceptionGroup", "KeyboardInterrupt", "StopIteration", "StopAsyncIteration":
		return true
	}
	return strings.HasSuffix(segment, "Error") || strings.HasSuffix(segment, "Exception") ||
		strings.HasSuffix(segment, "Warning") || strings.HasSuffix(segment, "Exit")
}

// exceptionClasses returns the classes of pkg that inherit from an exception. A
// plain base name is looked up in the module that defines the class, which for a
// copied class is its Origin. A base that is not in the output stays unknown: its
// class is an exception only if the base's own name says so.
func exceptionClasses(pkg *model.Package) map[*model.Class]bool {
	byPath := map[string]*model.Class{}
	aliases := map[string]string{}
	for _, mod := range pkg.Modules {
		for _, cls := range mod.Classes {
			byPath[mod.Name+"."+cls.Name] = cls
			if key := originKey(cls.Origin, cls.OriginName, cls.Name); key != "" {
				byPath[key] = cls
			}
		}
		for _, a := range mod.Aliases {
			aliases[mod.Name+"."+a.Name] = a.Target
		}
	}

	// bases holds the classes in the output that each class inherits from.
	bases := map[*model.Class][]*model.Class{}
	exceptions := map[*model.Class]bool{}
	for _, mod := range pkg.Modules {
		for _, cls := range mod.Classes {
			scope := mod.Name
			if cls.Origin != "" {
				scope = cls.Origin
			}
			for _, base := range cls.BaseClasses {
				if isExceptionName(base.Name) {
					exceptions[cls] = true
				}
				// A dotted base names another module, which this data cannot resolve.
				name, _, _ := strings.Cut(base.Name, "[")
				if strings.Contains(name, ".") {
					continue
				}
				path := scope + "." + name
				if target, ok := aliases[path]; ok {
					path = target
				}
				if parent := byPath[path]; parent != nil {
					bases[cls] = append(bases[cls], parent)
				}
			}
		}
	}

	for changed := true; changed; {
		changed = false
		for cls, parents := range bases {
			if !exceptions[cls] && slices.ContainsFunc(parents, func(p *model.Class) bool { return exceptions[p] }) {
				exceptions[cls] = true
				changed = true
			}
		}
	}
	return exceptions
}

// GetText handles GET /api/packages/{name}/docs.txt[?prefix=].
func (h *DocsHandler) GetText(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !validateName(w, name) {
		return
	}

	resp, _, err := h.fetchDocs(r.Context(), name)
	if err != nil {
		switch {
		case errors.Is(err, ErrDocsPackageNotFound):
			http.Error(w, "package not found", http.StatusNotFound)
		default:
			http.Error(w, "documentation extraction failed", http.StatusBadGateway)
		}
		return
	}

	if writeReasonText(w, resp) {
		return
	}

	prefix := r.URL.Query().Get("prefix")
	body := textfmt.FormatDocs(docsInputFrom(resp), prefix)

	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(body)) //nolint:errcheck
}

// docsInputFrom converts the JSON-shaped DocsResponse into the textfmt input
// shape. Defined here so textfmt does not need to import the handler package.
func docsInputFrom(d DocsResponse) *textfmt.DocsInput {
	out := &textfmt.DocsInput{
		Package:     d.Package,
		Version:     d.Version,
		Available:   d.Available,
		StubPackage: d.StubPackage,
	}
	for _, m := range d.Modules {
		mod := textfmt.DocModuleInput{Name: m.Name}
		for _, a := range m.Aliases {
			mod.Aliases = append(mod.Aliases, textfmt.DocAliasInput{Name: a.Name, Target: a.Target})
		}
		for _, fn := range m.Functions {
			mod.Functions = append(mod.Functions, docSymbolFrom(fn))
		}
		for _, cls := range m.Classes {
			mod.Classes = append(mod.Classes, docSymbolFrom(cls))
		}
		for _, exc := range m.Exceptions {
			mod.Exceptions = append(mod.Exceptions, docSymbolFrom(exc))
		}
		out.Modules = append(out.Modules, mod)
	}
	return out
}

func docSymbolFrom(s DocSymbol) textfmt.DocSymbolInput {
	out := textfmt.DocSymbolInput{
		Name:      s.Name,
		Kind:      s.Kind,
		Signature: s.Signature,
		Docstring: s.Docstring,
	}
	for _, p := range s.Parameters {
		out.Parameters = append(out.Parameters, textfmt.DocParamInput{
			Name: p.Name, Type: p.Type, Description: p.Description, Kind: p.Kind, Default: p.Default,
		})
	}
	if s.Returns != nil {
		out.Returns = &textfmt.DocReturnInput{Type: s.Returns.Type, Description: s.Returns.Description}
	}
	for _, r := range s.Raises {
		out.Raises = append(out.Raises, textfmt.DocRaiseInput{Type: r.Type, Description: r.Description})
	}
	for _, m := range s.Methods {
		out.Methods = append(out.Methods, docSymbolFrom(m))
	}
	for _, a := range s.Attributes {
		out.Attributes = append(out.Attributes, attributeInputFrom(a))
	}
	return out
}

func attributeInputFrom(a DocAttribute) textfmt.DocAttributeInput {
	return textfmt.DocAttributeInput{
		Name: a.Name, Type: a.Type, Default: a.Default, Description: a.Description,
		IsProperty: a.IsProperty, Writable: a.Writable,
	}
}

// GetSymbol handles GET /api/packages/{name}/docs/{symbol}.txt.
//
// The {symbol} param captures the dotted path including the .txt extension
// (e.g. "Client.get.txt"). The handler strips the extension, then tries
// each module name as a candidate prefix to form the full dotted path
// understood by textfmt.FormatSymbol. If the symbol path is already module-
// qualified (e.g. "httpx.Client"), that's tried as-is too.
func (h *DocsHandler) GetSymbol(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !validateName(w, name) {
		return
	}
	symbolPath := chi.URLParam(r, "symbol")
	if symbolPath == "" {
		http.Error(w, "symbol required", http.StatusBadRequest)
		return
	}

	// Strip .txt extension from the captured symbol.
	symbolPath = strings.TrimSuffix(symbolPath, ".txt")

	resp, _, err := h.fetchDocs(r.Context(), name)
	if err != nil {
		switch {
		case errors.Is(err, ErrDocsPackageNotFound):
			http.Error(w, "package not found", http.StatusNotFound)
		default:
			http.Error(w, "documentation extraction failed", http.StatusBadGateway)
		}
		return
	}

	input := docsInputFrom(resp)

	// Try each module name as a candidate prefix.
	for _, mod := range input.Modules {
		full := mod.Name + "." + symbolPath
		if out, ok := textfmt.FormatSymbol(input, full); ok {
			h.writeSymbolText(w, out)
			return
		}
	}

	// Fall back: maybe the path is already module-qualified.
	if out, ok := textfmt.FormatSymbol(input, symbolPath); ok {
		h.writeSymbolText(w, out)
		return
	}

	http.Error(w, "symbol not found", http.StatusNotFound)
}

func (h *DocsHandler) writeSymbolText(w http.ResponseWriter, body string) {
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(body)) //nolint:errcheck
}

// GetSymbols handles GET /api/packages/{name}/symbols.txt[?q=&kind=&limit=].
func (h *DocsHandler) GetSymbols(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !validateName(w, name) {
		return
	}

	resp, _, err := h.fetchDocs(r.Context(), name)
	if err != nil {
		switch {
		case errors.Is(err, ErrDocsPackageNotFound):
			http.Error(w, "package not found", http.StatusNotFound)
		default:
			http.Error(w, "documentation extraction failed", http.StatusBadGateway)
		}
		return
	}

	if writeReasonText(w, resp) {
		return
	}

	q := r.URL.Query().Get("q")
	kind := r.URL.Query().Get("kind")
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, perr := strconv.Atoi(raw); perr == nil {
			limit = n
		}
	}
	if limit > 500 {
		limit = 500
	}
	if limit <= 0 {
		limit = 100
	}

	body := textfmt.FormatSymbols(docsInputFrom(resp), q, kind, limit)

	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(body)) //nolint:errcheck
}
