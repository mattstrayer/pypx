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

// Get handles GET /api/popular?limit=12.
func (h *PopularHandler) Get(w http.ResponseWriter, r *http.Request) {
	limit := parsePopularLimit(r)

	cacheKey := fmt.Sprintf("popular:%d", limit)

	if data, fresh, err := h.cache.Get(cacheKey, popularTTL); err == nil && len(data) > 2 {
		// len(data) > 2 skips a cached empty array ("[]") — that means downloads
		// hadn't synced yet when it was stored; fall through to a live query.
		w.Header().Set("Cache-Control", "public, max-age=21600")
		w.Header().Set("Content-Type", "application/json")
		w.Write(data) //nolint:errcheck
		if !fresh {
			// Stale-while-revalidate: refresh in the background.
			// cache.RefreshInBackground deduplicates concurrent refreshes for
			// the same key via singleflight.
			cache.RefreshInBackground(h.cache, cacheKey, popularTTL, func() ([]byte, error) {
				results, err := h.index.TopByDownloads(limit)
				if err != nil {
					return nil, err
				}
				if len(results) == 0 {
					return nil, fmt.Errorf("empty result, skipping cache")
				}
				return json.Marshal(results)
			})
		}
		return
	}

	results, err := h.index.TopByDownloads(limit)
	if err != nil {
		log.Printf("popular: TopByDownloads error: %v", err)
		http.Error(w, "failed to fetch popular packages", http.StatusInternalServerError)
		return
	}

	encoded, err := json.Marshal(results)
	if err != nil {
		log.Printf("popular: json.Marshal error: %v", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	// Only cache non-empty results. An empty slice means downloads haven't synced
	// yet; caching it would lock in the empty state until the entry expires.
	if len(results) > 0 {
		h.cache.Set(cacheKey, encoded, popularTTL) //nolint:errcheck
	}

	w.Header().Set("Cache-Control", "public, max-age=21600")
	w.Header().Set("Content-Type", "application/json")
	w.Write(encoded) //nolint:errcheck
}

// GetText handles GET /api/popular.txt?limit=12 — same data as Get but
// rendered as TSV via textfmt.FormatSearch, since search.PackageEntry is the
// shared shape between /api/search and /api/popular.
func (h *PopularHandler) GetText(w http.ResponseWriter, r *http.Request) {
	limit := parsePopularLimit(r)

	cacheKey := fmt.Sprintf("popular.txt:%d", limit)

	if data, fresh, err := h.cache.Get(cacheKey, popularTTL); err == nil && len(data) > 0 {
		w.Header().Set("Cache-Control", "public, max-age=21600")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(data) //nolint:errcheck
		if !fresh {
			// Stale-while-revalidate: refresh in the background.
			// cache.RefreshInBackground deduplicates concurrent refreshes for
			// the same key via singleflight.
			cache.RefreshInBackground(h.cache, cacheKey, popularTTL, func() ([]byte, error) {
				results, err := h.index.TopByDownloads(limit)
				if err != nil {
					return nil, err
				}
				if len(results) == 0 {
					return nil, fmt.Errorf("empty result, skipping cache")
				}
				return []byte(textfmt.FormatSearch(results)), nil
			})
		}
		return
	}

	results, err := h.index.TopByDownloads(limit)
	if err != nil {
		log.Printf("popular: TopByDownloads error: %v", err)
		http.Error(w, "failed to fetch popular packages", http.StatusInternalServerError)
		return
	}

	body := textfmt.FormatSearch(results)

	// Only cache non-empty results. An empty slice means downloads haven't synced
	// yet; caching it would lock in the empty state until the entry expires.
	if len(results) > 0 {
		h.cache.Set(cacheKey, []byte(body), popularTTL) //nolint:errcheck
	}

	w.Header().Set("Cache-Control", "public, max-age=21600")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(body)) //nolint:errcheck
}
