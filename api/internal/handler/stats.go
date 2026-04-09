package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/stats"
)

const statsTTL = 24 * time.Hour

// CombinedStats holds all download statistics for a package.
type CombinedStats struct {
	Package        string            `json:"package"`
	Overall        []stats.DataPoint `json:"overall"`
	PythonVersions []stats.DataPoint `json:"python_versions"`
	Systems        []stats.DataPoint `json:"systems"`
}

// StatsHandler serves package download statistics requests.
type StatsHandler struct {
	stats *stats.Client
	cache *cache.Cache
}

// NewStatsHandler creates a new StatsHandler.
func NewStatsHandler(statsClient *stats.Client, c *cache.Cache) *StatsHandler {
	return &StatsHandler{stats: statsClient, cache: c}
}

// Get handles GET /api/packages/{name}/stats.
func (h *StatsHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	cacheKey := "stats:" + strings.ToLower(name)

	// Check cache.
	if data, fresh, err := h.cache.Get(cacheKey, statsTTL); err == nil && data != nil && fresh {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data) //nolint:errcheck
		return
	}

	// Fetch all three stat types; don't fail if one errors.
	var overall, pythonVersions, systems []stats.DataPoint

	if resp, err := h.stats.FetchOverall(name); err == nil {
		overall = resp.Data
	}
	if resp, err := h.stats.FetchPythonVersions(name); err == nil {
		pythonVersions = resp.Data
	}
	if resp, err := h.stats.FetchSystem(name); err == nil {
		systems = resp.Data
	}

	combined := CombinedStats{
		Package:        name,
		Overall:        overall,
		PythonVersions: pythonVersions,
		Systems:        systems,
	}

	encoded, err := json.Marshal(combined)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	h.cache.Set(cacheKey, encoded, statsTTL) //nolint:errcheck

	w.Header().Set("Content-Type", "application/json")
	w.Write(encoded) //nolint:errcheck
}
