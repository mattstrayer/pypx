package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/stats"
	"github.com/pypx/api/internal/textfmt"
)

const statsTTL = 24 * time.Hour

// DateRange represents the bounding dates of the filtered data.
type DateRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// CombinedStats holds all download statistics for a package.
type CombinedStats struct {
	Package        string            `json:"package"`
	Period         string            `json:"period"`
	DateRange      *DateRange        `json:"date_range,omitempty"`
	Overall        []stats.DataPoint `json:"overall"`
	PythonVersions []stats.DataPoint `json:"python_versions"`
	Systems        []stats.DataPoint `json:"systems"`
}

// periodConfig holds the parameters for a given time period.
type periodConfig struct {
	days       int
	maxWeeks   int
	pythonTopN int
}

var periods = map[string]periodConfig{
	"4w": {days: 28, maxWeeks: 4, pythonTopN: 8},
	"3m": {days: 90, maxWeeks: 12, pythonTopN: 8},
	"6m": {days: 180, maxWeeks: 24, pythonTopN: 8},
}

func parsePeriod(s string) (string, periodConfig) {
	if cfg, ok := periods[s]; ok {
		return s, cfg
	}
	return "4w", periods["4w"]
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

// resolveStats obtains a CombinedStats for name, using the cache when
// available (stale-while-revalidate: stale data is returned immediately and
// refreshed in the background) and fetching synchronously on a miss. It
// returns the decoded CombinedStats and the raw encoded JSON bytes (needed by
// Get to avoid a redundant re-encode); ok is false if fetching failed.
func (h *StatsHandler) resolveStats(r *http.Request, name string) (combined CombinedStats, encoded []byte, ok bool) {
	period, cfg := parsePeriod(r.URL.Query().Get("period"))
	cacheKey := "stats:" + strings.ToLower(name) + ":" + period

	// Check cache — serve stale data rather than blocking on a re-fetch.
	if data, fresh, err := h.cache.Get(cacheKey, statsTTL); err == nil && data != nil {
		// Unmarshal before triggering any background refresh: a corrupt entry
		// must take only the synchronous fetch path below, not both a
		// background refresh AND a synchronous re-fetch.
		if err := json.Unmarshal(data, &combined); err == nil {
			if !fresh {
				// Background revalidation: re-fetch and update cache silently.
				// cache.RefreshInBackground deduplicates concurrent refreshes for
				// the same key via singleflight. fetchAndCache already calls
				// h.cache.Set internally; returning the encoded bytes here causes
				// an idempotent second write with the same value, which is harmless.
				cache.RefreshInBackground(h.cache, cacheKey, statsTTL, func() ([]byte, error) {
					refreshed := h.fetchAndCache(context.Background(), name, period, cfg, cacheKey)
					if refreshed == nil {
						return nil, fmt.Errorf("background stats refresh failed for %q/%s", name, period)
					}
					return refreshed, nil
				})
			}
			return combined, data, true
		}
		// Fall through to a synchronous fetch if the cached blob is corrupt.
	}

	// Cache miss — fetch synchronously.
	encoded = h.fetchAndCache(r.Context(), name, period, cfg, cacheKey)
	if encoded == nil {
		return CombinedStats{}, nil, false
	}
	if err := json.Unmarshal(encoded, &combined); err != nil {
		return CombinedStats{}, nil, false
	}
	return combined, encoded, true
}

// Get handles GET /api/packages/{name}/stats?period=4w.
func (h *StatsHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	_, encoded, ok := h.resolveStats(r, name)
	if !ok {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Type", "application/json")
	w.Write(encoded) //nolint:errcheck
}

// GetText handles GET /api/packages/{name}/stats.txt?period=4w.
func (h *StatsHandler) GetText(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	combined, _, ok := h.resolveStats(r, name)
	if !ok {
		http.Error(w, "failed to fetch stats", http.StatusInternalServerError)
		return
	}

	input := textfmt.StatsInput{
		Package:        combined.Package,
		Period:         combined.Period,
		Overall:        toStatPoints(combined.Overall),
		PythonVersions: toStatPoints(combined.PythonVersions),
		Systems:        toStatPoints(combined.Systems),
	}
	if combined.DateRange != nil {
		input.DateFrom = combined.DateRange.From
		input.DateTo = combined.DateRange.To
	}

	body := textfmt.FormatStats(input)

	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(body)) //nolint:errcheck
}

// toStatPoints maps stats.DataPoint (Category label) to textfmt.StatPoint.
func toStatPoints(points []stats.DataPoint) []textfmt.StatPoint {
	if len(points) == 0 {
		return nil
	}
	out := make([]textfmt.StatPoint, len(points))
	for i, p := range points {
		out[i] = textfmt.StatPoint{Label: p.Category, Downloads: p.Downloads}
	}
	return out
}

// filterByDate removes data points whose date falls outside the window.
func filterByDate(data []stats.DataPoint, cutoff time.Time) []stats.DataPoint {
	filtered := make([]stats.DataPoint, 0, len(data))
	for _, d := range data {
		t, err := time.Parse("2006-01-02", d.Date)
		if err != nil {
			continue
		}
		if !t.Before(cutoff) {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

// computeDateRange returns the min and max dates found in any of the provided
// data slices.
func computeDateRange(slices ...[]stats.DataPoint) *DateRange {
	var minDate, maxDate string
	for _, data := range slices {
		for _, d := range data {
			if d.Date == "" {
				continue
			}
			if minDate == "" || d.Date < minDate {
				minDate = d.Date
			}
			if maxDate == "" || d.Date > maxDate {
				maxDate = d.Date
			}
		}
	}
	if minDate == "" {
		return nil
	}
	return &DateRange{From: minDate, To: maxDate}
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

// fetchAndCache fetches all stat types from pypistats.org, filters by period,
// aggregates, stores in cache, and returns the encoded JSON.
func (h *StatsHandler) fetchAndCache(ctx context.Context, name, period string, cfg periodConfig, cacheKey string) []byte {
	lower := strings.ToLower(name)
	cutoff := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -cfg.days)

	var rawOverall, rawPython, rawSystem []stats.DataPoint

	if resp, err := h.stats.FetchOverall(ctx, lower); err != nil {
		log.Printf("stats: FetchOverall(%q) error: %v", name, err)
	} else {
		rawOverall = resp.Data
	}
	if resp, err := h.stats.FetchPythonVersions(ctx, lower); err != nil {
		log.Printf("stats: FetchPythonVersions(%q) error: %v", name, err)
	} else {
		rawPython = resp.Data
	}
	if resp, err := h.stats.FetchSystem(ctx, lower); err != nil {
		log.Printf("stats: FetchSystem(%q) error: %v", name, err)
	} else {
		rawSystem = resp.Data
	}

	// Filter by date window before aggregating.
	filteredOverall := filterByDate(rawOverall, cutoff)
	filteredPython := filterByDate(rawPython, cutoff)
	filteredSystem := filterByDate(rawSystem, cutoff)

	overall := aggregateByWeek(filteredOverall, cfg.maxWeeks)
	pythonVersions := aggregateByCategory(filteredPython, cfg.pythonTopN)
	systems := aggregateByCategory(filteredSystem, 0)

	combined := CombinedStats{
		Package:        name,
		Period:         period,
		DateRange:      computeDateRange(filteredOverall, filteredPython, filteredSystem),
		Overall:        overall,
		PythonVersions: pythonVersions,
		Systems:        systems,
	}

	encoded, err := json.Marshal(combined)
	if err != nil {
		log.Printf("stats: json.Marshal(%q) error: %v", name, err)
		return nil
	}

	// Don't cache empty results — they're likely caused by transient upstream
	// errors and would block retries for the entire TTL.
	if len(overall) > 0 || len(pythonVersions) > 0 || len(systems) > 0 {
		h.cache.Set(cacheKey, encoded, statsTTL) //nolint:errcheck
	}
	return encoded
}
