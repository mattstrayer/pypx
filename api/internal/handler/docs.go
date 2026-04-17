package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/pypi"
	"github.com/pypx/goopy"
	"github.com/pypx/goopy/model"
)

// DocsResponse is the response for GET /api/packages/{name}/docs.
type DocsResponse struct {
	Package   string      `json:"package"`
	Version   string      `json:"version"`
	Available bool        `json:"available"`
	Modules   []DocModule `json:"modules"`
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

	// Resolve latest version.
	pkg, err := h.pypi.FetchPackage(name)
	if err != nil {
		http.Error(w, "package not found", http.StatusNotFound)
		return
	}
	version := pkg.Info.Version

	cacheKey := "docs:" + strings.ToLower(name) + ":" + version
	errKey := "docs-err:" + strings.ToLower(name) + ":" + version

	// TTL=0 means indefinite (source is immutable per version).
	if data, _, err := h.cache.Get(cacheKey, 0); err == nil && data != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data) //nolint:errcheck
		return
	}

	// Short-circuit if extraction recently failed (5-min negative cache).
	if data, fresh, err := h.cache.Get(errKey, 300); err == nil && data != nil && fresh {
		http.Error(w, "documentation extraction failed", http.StatusBadGateway)
		return
	}

	// Extract docs using goopy (in-process, no sidecar).
	result, err := goopy.ExtractFromPyPI(r.Context(), name, version)
	if err != nil {
		h.cache.Set(errKey, []byte(err.Error()), 300) //nolint:errcheck
		http.Error(w, "documentation extraction failed", http.StatusBadGateway)
		return
	}

	// If the context was cancelled mid-extraction, goopy returns 0 modules
	// without an error. Treat this as a transient failure so future requests retry.
	if r.Context().Err() != nil {
		h.cache.Set(errKey, []byte("extraction interrupted by timeout"), 300) //nolint:errcheck
		http.Error(w, "documentation extraction failed", http.StatusBadGateway)
		return
	}

	docsResp := convertToDocsResponse(name, version, result)

	encoded, err := json.Marshal(docsResp)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	// Only cache indefinitely if we got actual content. An empty result may mean
	// a genuine binary-only package (24h retry) vs. a successfully parsed package.
	ttl := time.Duration(0)
	if !docsResp.Available {
		ttl = 24 * time.Hour // retry after 24h for packages with no extractable docs
	}
	h.cache.Set(cacheKey, encoded, ttl) //nolint:errcheck

	w.Header().Set("Content-Type", "application/json")
	w.Write(encoded) //nolint:errcheck
}

// convertToDocsResponse transforms goopy's model.Package into the API response
// format expected by the frontend.
func convertToDocsResponse(name, version string, pkg *model.Package) DocsResponse {
	var modules []DocModule

	for _, mod := range pkg.Modules {
		dm := DocModule{
			Name:       mod.Name,
			Functions:  make([]DocSymbol, 0, len(mod.Functions)),
			Classes:    make([]DocSymbol, 0),
			Exceptions: make([]DocSymbol, 0),
		}

		for _, fn := range mod.Functions {
			dm.Functions = append(dm.Functions, convertFunction(fn))
		}

		for _, cls := range mod.Classes {
			sym := convertClass(cls)
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

func convertFunction(fn *model.Function) DocSymbol {
	sym := DocSymbol{
		Name:       fn.Name,
		Kind:       "function",
		Signature:  buildFuncSignature(fn),
		Docstring:  cleanDocstringText(fn.Docstring),
		Parameters: make([]DocParam, 0, len(fn.Parameters)),
	}

	for _, p := range fn.Parameters {
		dp := DocParam{Name: p.Name}
		if p.Type != nil {
			dp.Type = p.Type.Raw
		} else if p.DocParam != nil && p.DocParam.Type != "" {
			// No source annotation — fall back to docstring-declared type.
			dp.Type = p.DocParam.Type
		}
		if p.DocParam != nil {
			dp.Description = p.DocParam.Description
		}
		dp.Kind = string(p.Kind)
		dp.Default = p.Default
		sym.Parameters = append(sym.Parameters, dp)
	}

	// Returns: annotation takes precedence; docstring fills type and/or description.
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
	}

	return sym
}

func convertClass(cls *model.Class) DocSymbol {
	sym := DocSymbol{
		Name:      cls.Name,
		Kind:      "class",
		Signature: buildClassSignature(cls),
		Docstring: cleanDocstringText(cls.Docstring),
	}
	return sym
}

func buildFuncSignature(fn *model.Function) string {
	var b strings.Builder
	if fn.IsAsync {
		b.WriteString("async ")
	}
	b.WriteString("def ")
	b.WriteString(fn.Name)
	b.WriteByte('(')

	for i, p := range fn.Parameters {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.Name)
		if p.Type != nil && p.Type.Raw != "" {
			b.WriteString(": ")
			b.WriteString(p.Type.Raw)
		}
		if p.Default != "" {
			b.WriteString(" = ")
			b.WriteString(p.Default)
		}
	}

	b.WriteByte(')')
	if fn.Returns != nil && fn.Returns.Raw != "" {
		b.WriteString(" -> ")
		b.WriteString(fn.Returns.Raw)
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

