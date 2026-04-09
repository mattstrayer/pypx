package handler

import (
	"encoding/json"
	"log"
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

	// Check cache — serve stale data rather than blocking on a re-fetch.
	if data, fresh, err := h.cache.Get(cacheKey, statsTTL); err == nil && data != nil {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("Content-Type", "application/json")
		w.Write(data) //nolint:errcheck

		if !fresh {
			// Background revalidation: re-fetch and update cache silently.
			go func() {
				h.fetchAndCache(name, cacheKey)
			}()
		}
		return
	}

	// Cache miss — fetch synchronously.
	encoded := h.fetchAndCache(name, cacheKey)
	if encoded == nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Type", "application/json")
	w.Write(encoded) //nolint:errcheck
}

// fetchAndCache fetches all stat types from pypistats.org, builds a
// CombinedStats, stores it in the cache, and returns the encoded JSON.
// Errors from individual stat endpoints are logged but do not abort the call.
// Returns nil if JSON marshalling fails.
func (h *StatsHandler) fetchAndCache(name, cacheKey string) []byte {
	var overall, pythonVersions, systems []stats.DataPoint

	if resp, err := h.stats.FetchOverall(name); err != nil {
		log.Printf("stats: FetchOverall(%q) error: %v", name, err)
	} else {
		overall = resp.Data
	}
	if resp, err := h.stats.FetchPythonVersions(name); err != nil {
		log.Printf("stats: FetchPythonVersions(%q) error: %v", name, err)
	} else {
		pythonVersions = resp.Data
	}
	if resp, err := h.stats.FetchSystem(name); err != nil {
		log.Printf("stats: FetchSystem(%q) error: %v", name, err)
	} else {
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
		log.Printf("stats: json.Marshal(%q) error: %v", name, err)
		return nil
	}

	h.cache.Set(cacheKey, encoded, statsTTL) //nolint:errcheck
	return encoded
}
