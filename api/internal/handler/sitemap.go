package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/search"
)

const sitemapPopularLimit = 5000

// SitemapHandler serves package name lists for sitemap generation.
type SitemapHandler struct {
	index       *search.Index
	sqliteCache *cache.Cache
}

// NewSitemapHandler creates a new SitemapHandler.
func NewSitemapHandler(idx *search.Index, sqliteCache *cache.Cache) *SitemapHandler {
	return &SitemapHandler{index: idx, sqliteCache: sqliteCache}
}

// sitemapResponse is the JSON shape returned by both endpoints.
type sitemapResponse struct {
	Packages []string `json:"packages"`
}

// Popular handles GET /api/sitemap/popular — returns the top N packages by
// download count (up to sitemapPopularLimit).
func (h *SitemapHandler) Popular(w http.ResponseWriter, r *http.Request) {
	entries, err := h.index.TopByDownloads(sitemapPopularLimit)
	if err != nil {
		log.Printf("sitemap: TopByDownloads error: %v", err)
		http.Error(w, "failed to fetch popular packages", http.StatusInternalServerError)
		return
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=86400") // 24h
	if err := json.NewEncoder(w).Encode(sitemapResponse{Packages: names}); err != nil {
		log.Printf("sitemap: encode error: %v", err)
	}
}

// Cached handles GET /api/sitemap/cached — returns all packages that have
// been fetched and cached in SQLite at least once.
func (h *SitemapHandler) Cached(w http.ResponseWriter, r *http.Request) {
	names, err := h.sqliteCache.ListPackageNames()
	if err != nil {
		log.Printf("sitemap: ListPackageNames error: %v", err)
		http.Error(w, "failed to list cached packages", http.StatusInternalServerError)
		return
	}
	if names == nil {
		names = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600") // 1h
	if err := json.NewEncoder(w).Encode(sitemapResponse{Packages: names}); err != nil {
		log.Printf("sitemap: encode error: %v", err)
	}
}
