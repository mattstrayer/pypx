package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/search"
	"github.com/pypx/api/internal/textfmt"
)

const popularTTL = 6 * time.Hour

// PopularHandler serves the top packages by download count.
type PopularHandler struct {
	index *search.Index
	cache cache.Cacher
}

// NewPopularHandler creates a new PopularHandler.
func NewPopularHandler(idx *search.Index, c cache.Cacher) *PopularHandler {
	return &PopularHandler{index: idx, cache: c}
}

// parsePopularLimit reads and clamps the "limit" query parameter, defaulting
// to 12 and capping at 50. Shared by Get and GetText.
func parsePopularLimit(r *http.Request) int {
	limit := 12
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}
	if limit <= 0 {
		limit = 12
	}
	if limit > 50 {
		limit = 50
	}
	return limit
}

// fetchAndCachePopular runs a live TopByDownloads query, JSON-encodes the
// result, and stores it under cacheKey (unless empty — see below). Mirrors
// stats.go's fetchAndCache.
func (h *PopularHandler) fetchAndCachePopular(limit int, cacheKey string) ([]byte, error) {
	results, err := h.index.TopByDownloads(limit)
	if err != nil {
		return nil, err
	}

	encoded, err := json.Marshal(results)
	if err != nil {
		return nil, err
	}

	// Only cache non-empty results. An empty slice means downloads haven't synced
	// yet; caching it would lock in the empty state until the entry expires.
	if len(results) > 0 {
		h.cache.Set(cacheKey, encoded, popularTTL) //nolint:errcheck
	}

	return encoded, nil
}

// resolvePopular obtains the top-by-downloads JSON for limit, using the cache
// when available (stale-while-revalidate: stale data is returned immediately
// and refreshed in the background) and fetching live on a miss. This is the
// single cache entry ("popular:{limit}") shared by both Get and GetText,
// mirroring stats.go's resolveStats — GetText unmarshals the same JSON blob
// rather than keeping its own rendered-TSV cache entry.
func (h *PopularHandler) resolvePopular(limit int) (results []search.PackageEntry, encoded []byte, ok bool) {
	cacheKey := fmt.Sprintf("popular:%d", limit)

	if data, fresh, err := h.cache.Get(cacheKey, popularTTL); err == nil && len(data) > 2 {
		// len(data) > 2 skips a cached empty array ("[]") — that means downloads
		// hadn't synced yet when it was stored; fall through to a live query.
		if err := json.Unmarshal(data, &results); err == nil {
			if !fresh {
				// Stale-while-revalidate: refresh in the background.
				// cache.RefreshInBackground deduplicates concurrent refreshes for
				// the same key via singleflight.
				cache.RefreshInBackground(h.cache, cacheKey, popularTTL, func() ([]byte, error) {
					return h.fetchAndCachePopular(limit, cacheKey)
				})
			}
			return results, data, true
		}
		// Fall through to a live query if the cached blob is corrupt.
	}

	encoded, err := h.fetchAndCachePopular(limit, cacheKey)
	if err != nil {
		log.Printf("popular: TopByDownloads error: %v", err)
		return nil, nil, false
	}
	if err := json.Unmarshal(encoded, &results); err != nil {
		log.Printf("popular: json.Unmarshal error: %v", err)
		return nil, nil, false
	}

	return results, encoded, true
}

// Get handles GET /api/popular?limit=12.
func (h *PopularHandler) Get(w http.ResponseWriter, r *http.Request) {
	limit := parsePopularLimit(r)

	_, encoded, ok := h.resolvePopular(limit)
	if !ok {
		http.Error(w, "failed to fetch popular packages", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=21600")
	w.Header().Set("Content-Type", "application/json")
	w.Write(encoded) //nolint:errcheck
}

// GetText handles GET /api/popular.txt?limit=12 — same data as Get but
// rendered as TSV via textfmt.FormatSearch, since search.PackageEntry is the
// shared shape between /api/search and /api/popular. It reads/populates the
// same "popular:{limit}" cache entry as Get rather than a separate
// "popular.txt:{limit}" entry.
func (h *PopularHandler) GetText(w http.ResponseWriter, r *http.Request) {
	limit := parsePopularLimit(r)

	results, _, ok := h.resolvePopular(limit)
	if !ok {
		http.Error(w, "failed to fetch popular packages", http.StatusInternalServerError)
		return
	}

	body := textfmt.FormatSearch(results)

	w.Header().Set("Cache-Control", "public, max-age=21600")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(body)) //nolint:errcheck
}
