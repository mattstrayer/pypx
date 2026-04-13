package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/search"
)

func setupSitemapHandler(t *testing.T) (*chi.Mux, *cache.Cache) {
	t.Helper()
	sqliteCache, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	t.Cleanup(func() { sqliteCache.Close() })

	searchIdx, err := search.NewIndex(":memory:")
	if err != nil {
		t.Fatalf("search.NewIndex: %v", err)
	}
	t.Cleanup(func() { searchIdx.Close() })

	h := handler.NewSitemapHandler(searchIdx, sqliteCache)
	r := chi.NewRouter()
	r.Get("/api/sitemap/popular", h.Popular)
	r.Get("/api/sitemap/cached", h.Cached)
	return r, sqliteCache
}

func TestSitemapCached_Empty(t *testing.T) {
	r, _ := setupSitemapHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/sitemap/cached", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp struct {
		Packages []string `json:"packages"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Packages) != 0 {
		t.Errorf("expected 0 packages, got %d", len(resp.Packages))
	}
}

func TestSitemapCached_WithData(t *testing.T) {
	r, sqliteCache := setupSitemapHandler(t)

	// Seed two package entries.
	if err := sqliteCache.Set("pkg:requests", []byte(`{}`), time.Hour); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := sqliteCache.Set("pkg:flask", []byte(`{}`), time.Hour); err != nil {
		t.Fatalf("set: %v", err)
	}
	// Seed a non-package entry that must not appear.
	if err := sqliteCache.Set("stats:requests:4w", []byte(`{}`), time.Hour); err != nil {
		t.Fatalf("set: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/sitemap/cached", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp struct {
		Packages []string `json:"packages"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Packages) != 2 {
		t.Errorf("expected 2 packages, got %d: %v", len(resp.Packages), resp.Packages)
	}
}

func TestSitemapPopular_ReturnsOK(t *testing.T) {
	r, _ := setupSitemapHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/sitemap/popular", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Empty index returns 200 with empty packages list.
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp struct {
		Packages []string `json:"packages"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Just ensure it's a valid slice (may be empty if no downloads synced).
	if resp.Packages == nil {
		t.Error("packages should be a non-nil slice")
	}
}
