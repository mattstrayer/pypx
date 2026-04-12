package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/pypi"
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
	Name       string     `json:"name"`
	Kind       string     `json:"kind"`
	Signature  string     `json:"signature"`
	Docstring  string     `json:"docstring"`
	Parameters []DocParam `json:"parameters,omitempty"`
	Returns    *DocReturn `json:"returns,omitempty"`
}

// DocParam is a function parameter.
type DocParam struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Description string `json:"description"`
}

// DocReturn is the return type annotation and description.
type DocReturn struct {
	Type        string `json:"type,omitempty"`
	Description string `json:"description"`
}

// sidecarRequest is the body sent to the docs-worker sidecar.
type sidecarRequest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// sidecarResponse is the JSON returned by the docs-worker sidecar.
type sidecarResponse struct {
	Empty   bool        `json:"empty"`
	Reason  string      `json:"reason"`
	Modules []DocModule `json:"modules"`
}

// DocsHandler serves rendered API documentation for a package.
type DocsHandler struct {
	pypi       *pypi.Client
	cache      cache.Cacher
	sidecarURL string
	httpClient *http.Client
}

// NewDocsHandler creates a new DocsHandler.
func NewDocsHandler(pypiClient *pypi.Client, c cache.Cacher, sidecarURL string) *DocsHandler {
	return &DocsHandler{
		pypi:       pypiClient,
		cache:      c,
		sidecarURL: sidecarURL,
		httpClient: &http.Client{Timeout: 120 * time.Second},
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

	// Short-circuit if the sidecar recently failed for this package (5-min negative cache).
	if data, fresh, err := h.cache.Get(errKey, 300); err == nil && data != nil && fresh {
		http.Error(w, "documentation service unavailable", http.StatusBadGateway)
		return
	}

	// Call the docs-worker sidecar.
	reqBody, _ := json.Marshal(sidecarRequest{Name: name, Version: version})
	resp, err := h.httpClient.Post(h.sidecarURL+"/generate", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		h.cache.Set(errKey, []byte("1"), 300) //nolint:errcheck
		http.Error(w, "documentation service unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		h.cache.Set(errKey, []byte("1"), 300) //nolint:errcheck
		http.Error(w, "documentation service error", http.StatusBadGateway)
		return
	}

	var sidecar sidecarResponse
	if err := json.NewDecoder(resp.Body).Decode(&sidecar); err != nil {
		http.Error(w, "failed to decode docs response", http.StatusInternalServerError)
		return
	}

	modules := sidecar.Modules
	if modules == nil {
		modules = []DocModule{}
	}

	docsResp := DocsResponse{
		Package:   name,
		Version:   version,
		Available: !sidecar.Empty,
		Modules:   modules,
	}

	encoded, err := json.Marshal(docsResp)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	h.cache.Set(cacheKey, encoded, 0) //nolint:errcheck

	w.Header().Set("Content-Type", "application/json")
	w.Write(encoded) //nolint:errcheck
}
