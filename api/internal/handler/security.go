package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/osv"
	"github.com/pypx/api/internal/pypi"
)

const securityTTL = 24 * time.Hour

// SecurityResponse is the response for GET /api/packages/{name}/security.
type SecurityResponse struct {
	Package   string         `json:"package"`
	Vulns     []osv.VulnInfo `json:"vulns"`
	CheckedAt string         `json:"checked_at"`
}

// SecurityHandler serves vulnerability data from OSV.dev.
type SecurityHandler struct {
	osv   *osv.Client
	cache cache.Cacher
}

// NewSecurityHandler creates a new SecurityHandler.
func NewSecurityHandler(osvClient *osv.Client, c cache.Cacher) *SecurityHandler {
	return &SecurityHandler{osv: osvClient, cache: c}
}

// Get handles GET /api/packages/{name}/security.
func (h *SecurityHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := pypi.ValidateName(name); err != nil {
		http.Error(w, "invalid package name", http.StatusBadRequest)
		return
	}

	cacheKey := "security:" + strings.ToLower(name)

	if data, fresh, err := h.cache.Get(cacheKey, securityTTL); err == nil && data != nil && fresh {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("Content-Type", "application/json")
		w.Write(data) //nolint:errcheck
		return
	}

	vulns, err := h.osv.FetchVulns(name)
	if err != nil {
		// Serve stale cache on error rather than failing the request.
		if data, _, cacheErr := h.cache.Get(cacheKey, 0); cacheErr == nil && data != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write(data) //nolint:errcheck
			return
		}
		http.Error(w, "failed to fetch security data", http.StatusBadGateway)
		return
	}

	if vulns == nil {
		vulns = []osv.VulnInfo{}
	}

	resp := SecurityResponse{
		Package:   name,
		Vulns:     vulns,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}

	encoded, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	h.cache.Set(cacheKey, encoded, securityTTL) //nolint:errcheck

	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Type", "application/json")
	w.Write(encoded) //nolint:errcheck
}
