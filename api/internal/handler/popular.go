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

// Get handles GET /api/popular?limit=12.
func (h *PopularHandler) Get(w http.ResponseWriter, r *http.Request) {
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

	cacheKey := fmt.Sprintf("popular:%d", limit)

	if data, _, err := h.cache.Get(cacheKey, popularTTL); err == nil && data != nil {
		w.Header().Set("Cache-Control", "public, max-age=21600")
		w.Header().Set("Content-Type", "application/json")
		w.Write(data) //nolint:errcheck
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

	h.cache.Set(cacheKey, encoded, popularTTL) //nolint:errcheck

	w.Header().Set("Cache-Control", "public, max-age=21600")
	w.Header().Set("Content-Type", "application/json")
	w.Write(append(encoded, '\n')) //nolint:errcheck
}
