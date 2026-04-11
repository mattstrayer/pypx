package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
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
	cache cache.Cacher
}

// NewStatsHandler creates a new StatsHandler.
func NewStatsHandler(statsClient *stats.Client, c cache.Cacher) *StatsHandler {
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

// aggregateByWeek groups daily data points into ISO-week buckets, keeping only
// the "without_mirrors" category and returning the last maxWeeks weeks.
func aggregateByWeek(data []stats.DataPoint, maxWeeks int) []stats.DataPoint {
	type weekKey struct {
		year int
		week int
	}

	totals := make(map[weekKey]int64)
	for _, d := range data {
		if d.Category != "without_mirrors" {
			continue
		}
		t, err := time.Parse("2006-01-02", d.Date)
		if err != nil {
			continue
		}
		y, w := t.ISOWeek()
		totals[weekKey{y, w}] += d.Downloads
	}

	keys := make([]weekKey, 0, len(totals))
	for k := range totals {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].year != keys[j].year {
			return keys[i].year < keys[j].year
		}
		return keys[i].week < keys[j].week
	})

	if len(keys) > maxWeeks {
		keys = keys[len(keys)-maxWeeks:]
	}

	result := make([]stats.DataPoint, 0, len(keys))
	for _, k := range keys {
		// Label as "YYYY-Www" for clean display.
		label := time.Date(k.year, 1, 1, 0, 0, 0, 0, time.UTC).
			AddDate(0, 0, (k.week-1)*7).Format("Jan 02")
		result = append(result, stats.DataPoint{
			Category:  label,
			Downloads: totals[k],
		})
	}
	return result
}

// aggregateByCategory sums downloads per unique category, sorts descending by
// downloads, and returns the top N entries.
func aggregateByCategory(data []stats.DataPoint, topN int) []stats.DataPoint {
	totals := make(map[string]int64)
	for _, d := range data {
		if d.Category == "null" || d.Category == "" {
			continue
		}
		totals[d.Category] += d.Downloads
	}

	result := make([]stats.DataPoint, 0, len(totals))
	for cat, dl := range totals {
		result = append(result, stats.DataPoint{Category: cat, Downloads: dl})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Downloads > result[j].Downloads
	})

	if topN > 0 && len(result) > topN {
		result = result[:topN]
	}
	return result
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
		overall = aggregateByWeek(resp.Data, 12)
	}
	if resp, err := h.stats.FetchPythonVersions(name); err != nil {
		log.Printf("stats: FetchPythonVersions(%q) error: %v", name, err)
	} else {
		pythonVersions = aggregateByCategory(resp.Data, 8)
	}
	if resp, err := h.stats.FetchSystem(name); err != nil {
		log.Printf("stats: FetchSystem(%q) error: %v", name, err)
	} else {
		systems = aggregateByCategory(resp.Data, 0)
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
