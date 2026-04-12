package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/conda"
	"github.com/pypx/api/internal/pypi"
)

const extrasTTL = 24 * time.Hour

// ExtrasResponse is the response for GET /api/packages/{name}/extras.
type ExtrasResponse struct {
	Package     string                `json:"package"`
	TypeSupport pypi.TypeSupport      `json:"type_support"`
	CondaForge  *conda.CondaForgeInfo `json:"conda_forge"`
}

// ExtrasHandler serves type support and conda-forge data.
type ExtrasHandler struct {
	pypi  *pypi.Client
	conda *conda.Client
	cache cache.Cacher
}

// NewExtrasHandler creates a new ExtrasHandler.
func NewExtrasHandler(pypiClient *pypi.Client, condaClient *conda.Client, c cache.Cacher) *ExtrasHandler {
	return &ExtrasHandler{pypi: pypiClient, conda: condaClient, cache: c}
}

// Get handles GET /api/packages/{name}/extras.
func (h *ExtrasHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !validateName(w, name) {
		return
	}

	cacheKey := "extras:" + strings.ToLower(name)

	if data, _, err := h.cache.Get(cacheKey, extrasTTL); err == nil && data != nil {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("Content-Type", "application/json")
		w.Write(data) //nolint:errcheck
		return
	}

	// Fetch type support and conda info in parallel.
	var (
		typeSupport pypi.TypeSupport
		condaInfo   conda.CondaForgeInfo
		condaErr    error
		wg          sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		typeSupport = pypi.CheckTypeSupport(h.pypi, name)
	}()
	go func() {
		defer wg.Done()
		condaInfo, condaErr = h.conda.FetchCondaInfo(name)
	}()
	wg.Wait()

	// If not already typed via stubs, check for py.typed marker in the wheel.
	if typeSupport.Status != "typed" {
		if pkg, err := h.pypi.FetchPackage(name); err == nil {
			typedKey := "typed:" + strings.ToLower(name) + ":" + pkg.Info.Version
			if data, _, err := h.cache.Get(typedKey, 0); err == nil && data != nil {
				if string(data) == "1" {
					typeSupport.Status = "typed"
				}
			} else {
				wheelURL := pypi.ExtractWheelURL(pkg.URLs)
				if wheelURL != "" && pypi.CheckPyTyped(h.pypi, wheelURL) {
					typeSupport.Status = "typed"
					h.cache.Set(typedKey, []byte("1"), 0) //nolint:errcheck
				} else {
					// Cache negative — either no wheel or wheel has no py.typed marker. Immutable per version.
					h.cache.Set(typedKey, []byte("0"), 0) //nolint:errcheck
				}
			}
		}
	}

	resp := ExtrasResponse{
		Package:     name,
		TypeSupport: typeSupport,
	}
	if condaErr == nil {
		resp.CondaForge = &condaInfo
	}

	encoded, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	h.cache.Set(cacheKey, encoded, extrasTTL) //nolint:errcheck

	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Type", "application/json")
	w.Write(encoded) //nolint:errcheck
}
