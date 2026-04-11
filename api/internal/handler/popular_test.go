package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/search"
)

func mustPopularIndex(t *testing.T) *search.Index {
	t.Helper()
	idx, err := search.NewIndex(":memory:")
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	t.Cleanup(func() { idx.Close() })
	return idx
}

func mustPopularCache(t *testing.T) cache.Cacher {
	t.Helper()
	sqliteCache, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	mc := cache.NewMemoryCache(sqliteCache, 100)
	t.Cleanup(func() { mc.Close() })
	return mc
}

func TestPopular_ReturnsTopPackages(t *testing.T) {
	idx := mustPopularIndex(t)
	c := mustPopularCache(t)

	packages := []search.PackageEntry{
		{Name: "numpy", Summary: "Scientific computing", Downloads: 80_000_000},
		{Name: "requests", Summary: "HTTP for Humans", Downloads: 50_000_000},
		{Name: "flask", Summary: "A micro web framework", Downloads: 30_000_000},
	}
	if err := idx.UpsertBatch(packages); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}
	if err := idx.UpdateDownloadsBatch(packages); err != nil {
		t.Fatalf("UpdateDownloadsBatch: %v", err)
	}

	h := handler.NewPopularHandler(idx, c)

	req := httptest.NewRequest(http.MethodGet, "/api/popular?limit=3", nil)
	w := httptest.NewRecorder()
	h.Get(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	var results []search.PackageEntry
	if err := json.NewDecoder(res.Body).Decode(&results); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Name != "numpy" {
		t.Errorf("expected first result 'numpy', got %q", results[0].Name)
	}
}

func TestPopular_DefaultLimit(t *testing.T) {
	idx := mustPopularIndex(t)
	c := mustPopularCache(t)

	// Insert 15 packages with non-zero downloads.
	packages := make([]search.PackageEntry, 15)
	for i := range packages {
		packages[i] = search.PackageEntry{
			Name:      "pkg-" + string(rune('a'+i)),
			Summary:   "A package",
			Downloads: int64(1000 * (15 - i)),
		}
	}
	if err := idx.UpsertBatch(packages); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}
	if err := idx.UpdateDownloadsBatch(packages); err != nil {
		t.Fatalf("UpdateDownloadsBatch: %v", err)
	}

	h := handler.NewPopularHandler(idx, c)

	// No limit param — should default to 12.
	req := httptest.NewRequest(http.MethodGet, "/api/popular", nil)
	w := httptest.NewRecorder()
	h.Get(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	var results []search.PackageEntry
	if err := json.NewDecoder(res.Body).Decode(&results); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(results) != 12 {
		t.Errorf("expected 12 results (default), got %d", len(results))
	}
}

func TestPopular_LimitClamped(t *testing.T) {
	idx := mustPopularIndex(t)
	c := mustPopularCache(t)

	// Seed 60 packages with non-zero downloads so clamping is observable.
	packages := make([]search.PackageEntry, 60)
	for i := range packages {
		packages[i] = search.PackageEntry{
			Name:      fmt.Sprintf("pkg-%02d", i),
			Summary:   "A package",
			Downloads: int64(1000 * (60 - i)),
		}
	}
	if err := idx.UpsertBatch(packages); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}
	if err := idx.UpdateDownloadsBatch(packages); err != nil {
		t.Fatalf("UpdateDownloadsBatch: %v", err)
	}

	h := handler.NewPopularHandler(idx, c)

	req := httptest.NewRequest(http.MethodGet, "/api/popular?limit=999", nil)
	w := httptest.NewRecorder()
	h.Get(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	var results []search.PackageEntry
	if err := json.NewDecoder(res.Body).Decode(&results); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(results) != 50 {
		t.Errorf("expected 50 results (clamped from 999), got %d", len(results))
	}
}

func TestPopular_EmptyIndex(t *testing.T) {
	idx := mustPopularIndex(t)
	c := mustPopularCache(t)

	h := handler.NewPopularHandler(idx, c)

	req := httptest.NewRequest(http.MethodGet, "/api/popular", nil)
	w := httptest.NewRecorder()
	h.Get(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	var results []search.PackageEntry
	if err := json.NewDecoder(res.Body).Decode(&results); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty array, got %d results", len(results))
	}
}
