package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/osv"
	"github.com/pypx/api/internal/textfmt"
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

// fetchSecurity returns security data for a package, using the cache when available.
// Returns the decoded SecurityResponse, the raw JSON bytes, and any error.
// On upstream failure, falls back to stale cache data if available.
func (h *SecurityHandler) fetchSecurity(ctx context.Context, name, version string) (SecurityResponse, []byte, error) {
	cacheKey := "security:" + strings.ToLower(name)
	if version != "" {
		cacheKey += ":" + version
	}

	if data, _, err := h.cache.Get(cacheKey, securityTTL); err == nil && data != nil {
		var resp SecurityResponse
		if jsonErr := json.Unmarshal(data, &resp); jsonErr == nil {
			return resp, data, nil
		}
	}

	vulns, err := h.osv.FetchVulns(ctx, name, version)
	if err != nil {
		// Serve stale cache on error rather than failing the request.
		if data, _, cacheErr := h.cache.Get(cacheKey, 0); cacheErr == nil && data != nil {
			var resp SecurityResponse
			if jsonErr := json.Unmarshal(data, &resp); jsonErr == nil {
				return resp, data, nil
			}
		}
		return SecurityResponse{}, nil, err
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
		return SecurityResponse{}, nil, err
	}

	h.cache.Set(cacheKey, encoded, securityTTL) //nolint:errcheck

	return resp, encoded, nil
}

// Get handles GET /api/packages/{name}/security.
// Accepts an optional ?version= query param; when provided only vulnerabilities
// affecting that specific version are returned.
func (h *SecurityHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !validateName(w, name) {
		return
	}

	version := r.URL.Query().Get("version")

	_, encoded, err := h.fetchSecurity(r.Context(), name, version)
	if err != nil {
		http.Error(w, "failed to fetch security data", http.StatusBadGateway)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Type", "application/json")
	w.Write(encoded) //nolint:errcheck
}

// GetText handles GET /api/packages/{name}/security.txt.
func (h *SecurityHandler) GetText(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !validateName(w, name) {
		return
	}
	version := r.URL.Query().Get("version")

	resp, _, err := h.fetchSecurity(r.Context(), name, version)
	if err != nil {
		http.Error(w, "failed to fetch security data", http.StatusBadGateway)
		return
	}

	body := textfmt.FormatSecurity(&textfmt.SecurityInput{
		Package:   resp.Package,
		CheckedAt: resp.CheckedAt,
		Vulns:     resp.Vulns,
	})

	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(body)) //nolint:errcheck
}
