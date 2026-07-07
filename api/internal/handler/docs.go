package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/pypi"
	"github.com/pypx/api/internal/textfmt"
	"github.com/pypx/goopy"
	"github.com/pypx/goopy/model"
)

// DocsResponse is the response for GET /api/packages/{name}/docs.
type DocsResponse struct {
	Package     string      `json:"package"`
	Version     string      `json:"version"`
	Available   bool        `json:"available"`
	StubPackage string      `json:"stub_package,omitempty"`
	Modules     []DocModule `json:"modules"`
}

// DocModule is a Python module in the documentation.
type DocModule struct {
	Name       string      `json:"name"`
	Functions  []DocSymbol `json:"functions"`
	Classes    []DocSymbol `json:"classes"`
	Exceptions []DocSymbol `json:"exceptions"`
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

// DocsHandler serves rendered API documentation for a package.
type DocsHandler struct {
	pypi  *pypi.Client
	cache cache.Cacher
}

// NewDocsHandler creates a new DocsHandler.
func NewDocsHandler(pypiClient *pypi.Client, c cache.Cacher) *DocsHandler {
	return &DocsHandler{
		pypi:  pypiClient,
		cache: c,
	}
}

// Get handles GET /api/packages/{name}/docs.
func (h *DocsHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !validateName(w, name) {
		return
	}

	_, encoded, err := h.fetchDocs(r.Context(), name)
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

// Sentinel errors for the docs path. Callers map these to HTTP statuses.
var (
	ErrDocsPackageNotFound  = errors.New("docs: package not found")
	ErrDocsExtractionFailed = errors.New("docs: extraction failed")
	ErrDocsUpstreamFailed   = errors.New("docs: upstream metadata fetch failed")
)

// fetchDocs returns the cached or freshly-extracted DocsResponse for a package's
// LATEST version. Wraps fetchDocsAtVersion after resolving via PyPI.
func (h *DocsHandler) fetchDocs(ctx context.Context, name string) (DocsResponse, []byte, error) {
	pkg, err := h.pypi.FetchPackage(ctx, name)
	if err != nil {
		if errors.Is(err, pypi.ErrNotFound) {
			return DocsResponse{}, nil, ErrDocsPackageNotFound
		}
		return DocsResponse{}, nil, fmt.Errorf("%w: %v", ErrDocsUpstreamFailed, err)
	}
	return h.fetchDocsAtVersion(ctx, name, pkg.Info.Version)
}

// fetchDocsAtVersion returns the cached or freshly-extracted DocsResponse for
// a specific (name, version) pair. Cache key is "docs:{name}:{version}",
// indefinite TTL. Used by both fetchDocs (latest) and the diff handler
// (arbitrary historical versions). On cache hit returns the original cached
// bytes (no re-marshal).
func (h *DocsHandler) fetchDocsAtVersion(ctx context.Context, name, version string) (DocsResponse, []byte, error) {
	cacheKey := "docs:" + strings.ToLower(name) + ":" + version
	errKey := "docs-err:" + strings.ToLower(name) + ":" + version

	if data, _, cerr := h.cache.Get(cacheKey, 0); cerr == nil && data != nil {
		var resp DocsResponse
		if uerr := json.Unmarshal(data, &resp); uerr == nil {
			return resp, data, nil
		}
		// Corrupt cache entry; fall through to live extraction.
	}

	if data, fresh, cerr := h.cache.Get(errKey, 300); cerr == nil && data != nil && fresh {
		return DocsResponse{}, nil, ErrDocsExtractionFailed
	}

	result, gerr := goopy.ExtractFromPyPI(ctx, name, version)
	if gerr != nil {
		h.cache.Set(errKey, []byte(gerr.Error()), 300) //nolint:errcheck
		return DocsResponse{}, nil, ErrDocsExtractionFailed
	}
	if ctx.Err() != nil {
		h.cache.Set(errKey, []byte("extraction interrupted by timeout"), 300) //nolint:errcheck
		return DocsResponse{}, nil, ErrDocsExtractionFailed
	}

	stubs, stubPkgName := resolveStubs(ctx, h.pypi, strings.ToLower(name), version)
	docsResp := convertToDocsResponse(name, version, result, stubs)
	docsResp.StubPackage = stubPkgName

	encoded, merr := json.Marshal(docsResp)
	if merr != nil {
		return DocsResponse{}, nil, fmt.Errorf("encode docs: %w", merr)
	}

	ttl := time.Duration(0)
	if !docsResp.Available {
		ttl = 24 * time.Hour
	}
	h.cache.Set(cacheKey, encoded, ttl) //nolint:errcheck
	return docsResp, encoded, nil
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

	for _, mod := range pkg.Modules {
		dm := DocModule{
			Name:       mod.Name,
			Functions:  make([]DocSymbol, 0, len(mod.Functions)),
			Classes:    make([]DocSymbol, 0),
			Exceptions: make([]DocSymbol, 0),
		}

		for _, fn := range mod.Functions {
			stubFn := stubs[mod.Name+"."+fn.Name]
			dm.Functions = append(dm.Functions, convertFunction(fn, stubFn))
		}

		for _, cls := range mod.Classes {
			sym := convertClass(cls, stubs, mod.Name)
			if isException(cls) {
				sym.Kind = "exception"
				dm.Exceptions = append(dm.Exceptions, sym)
			} else {
				dm.Classes = append(dm.Classes, sym)
			}
		}

		if len(dm.Functions) > 0 || len(dm.Classes) > 0 || len(dm.Exceptions) > 0 {
			modules = append(modules, dm)
		}
	}

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

func convertFunction(fn *model.Function, stubFn *model.Function) DocSymbol {
	sym := DocSymbol{
		Name:       fn.Name,
		Kind:       "function",
		Docstring:  cleanDocstringText(fn.Docstring),
		Parameters: make([]DocParam, 0, len(fn.Parameters)),
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

func convertClass(cls *model.Class, stubs stubIndex, modName string) DocSymbol {
	sym := DocSymbol{
		Name:      cls.Name,
		Kind:      "class",
		Signature: buildClassSignature(cls),
		Docstring: cleanDocstringText(cls.Docstring),
	}

	if len(cls.Methods) > 0 {
		sym.Methods = make([]DocSymbol, 0, len(cls.Methods))
		for _, m := range cls.Methods {
			stubFn := stubs[modName+"."+cls.Name+"."+m.Name]
			sym.Methods = append(sym.Methods, convertFunction(m, stubFn))
		}
	}

	return sym
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

// cleanDocstringText returns the prose summary from a parsed docstring,
// stripping Sphinx field lists and Google section headers.
func cleanDocstringText(ds *model.Docstring) string {
	if ds == nil {
		return ""
	}
	lines := strings.Split(ds.Text, "\n")
	cutAt := len(lines)
	for i, line := range lines {
		if isDocstringFieldLine(line, ds.Style) {
			cutAt = i
			break
		}
	}
	result := strings.Join(lines[:cutAt], "\n")
	return strings.TrimSpace(result)
}

func isDocstringFieldLine(line string, style model.DocstringStyle) bool {
	t := strings.TrimSpace(line)
	switch style {
	case model.DocstringSphinx:
		// Matches ":word...: ..." field list lines. False-positives are possible
		// for RST rubrics like ":Note:" embedded in prose, but these are rare.
		return len(t) > 1 && t[0] == ':' && strings.ContainsRune(t[1:], ':')
	case model.DocstringGoogle:
		if line != t || !strings.HasSuffix(t, ":") {
			return false
		}
		return isGoogleSection(strings.TrimSuffix(t, ":"))
	case model.DocstringNumpy:
		// NumPy section headers are flush-left lines matching known section names
		// (the dashes underline follows on the next line, which we don't need to check).
		if line != t {
			return false // indented lines are content, not headers
		}
		return isGoogleSection(t)
	}
	return false
}

func isGoogleSection(word string) bool {
	switch word {
	case "Args", "Arguments", "Parameters", "Params",
		"Returns", "Return", "Raises", "Raise",
		"Note", "Notes", "Example", "Examples",
		"Attributes", "Yields", "Yield", "Todo",
		"References", "See Also":
		return true
	}
	return false
}

// isException returns true if the class inherits from an exception base.
func isException(cls *model.Class) bool {
	for _, base := range cls.BaseClasses {
		name := base.Name
		if strings.Contains(name, "Exception") || strings.Contains(name, "Error") ||
			strings.Contains(name, "Warning") || name == "BaseException" {
			return true
		}
	}
	return false
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
	return out
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

