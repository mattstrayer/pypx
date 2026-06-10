package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

// TestPopularSWRStaleHitRefreshes verifies that when the popular cache entry is
// stale, it is served immediately and the background goroutine updates the cache
// with fresh data from the index.
func TestPopularSWRStaleHitRefreshes(t *testing.T) {
	idx := mustPopularIndex(t)

	// Seed v1 data: one package.
	v1 := []search.PackageEntry{
		{Name: "numpy", Summary: "Scientific computing", Downloads: 80_000_000},
	}
	if err := idx.UpsertBatch(v1); err != nil {
		t.Fatalf("UpsertBatch v1: %v", err)
	}
	if err := idx.UpdateDownloadsBatch(v1); err != nil {
		t.Fatalf("UpdateDownloadsBatch v1: %v", err)
	}

	// Use a file-backed cache so we can backdate created_at directly.
	sqliteCache, dbPath := mustTempCache(t)
	c := cache.NewMemoryCache(sqliteCache, 100)
	t.Cleanup(func() { c.Close() })

	h := handler.NewPopularHandler(idx, c)

	// Prime the cache by making the first request (cache miss → live query → cached).
	prime := httptest.NewRequest(http.MethodGet, "/api/popular?limit=1", nil)
	router := func(r *httptest.ResponseRecorder) {
		h.Get(r, prime)
	}
	primeRec := httptest.NewRecorder()
	router(primeRec)
	if primeRec.Code != http.StatusOK {
		t.Fatalf("prime: expected 200, got %d: %s", primeRec.Code, primeRec.Body.String())
	}

	// Force stale via direct SQL.
	backdateCache(t, dbPath, "popular:1")

	// Seed v2 data: update downloads significantly so we can distinguish v2.
	v2 := []search.PackageEntry{
		{Name: "numpy", Summary: "Scientific computing updated", Downloads: 999_999_999},
	}
	if err := idx.UpsertBatch(v2); err != nil {
		t.Fatalf("UpsertBatch v2: %v", err)
	}
	if err := idx.UpdateDownloadsBatch(v2); err != nil {
		t.Fatalf("UpdateDownloadsBatch v2: %v", err)
	}

	// Act: request should be served from stale cache (v1 — 80M downloads).
	// We need to bypass the memory cache which may still have the fresh entry.
	// Create a fresh handler+cache so the memory tier is empty.
	sqliteCache2, _ := cache.New(dbPath) // second connection to same file
	t.Cleanup(func() { sqliteCache2.Close() })
	mc2 := cache.NewMemoryCache(sqliteCache2, 100)
	t.Cleanup(func() { mc2.Close() })
	h2 := handler.NewPopularHandler(idx, mc2)

	req := httptest.NewRequest(http.MethodGet, "/api/popular?limit=1", nil)
	rec := httptest.NewRecorder()
	h2.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("stale hit: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Response must be the stale v1 data (80M downloads).
	var staleResults []search.PackageEntry
	if err := json.NewDecoder(rec.Body).Decode(&staleResults); err != nil {
		t.Fatalf("decode stale response: %v", err)
	}
	if len(staleResults) == 0 {
		t.Fatal("expected at least one result from stale cache")
	}
	if staleResults[0].Downloads != 80_000_000 {
		t.Errorf("expected stale downloads=80000000, got %d", staleResults[0].Downloads)
	}

	// Poll until the cache is refreshed to v2 (background goroutine updates the cache).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		req2 := httptest.NewRequest(http.MethodGet, "/api/popular?limit=1", nil)
		rec2 := httptest.NewRecorder()
		h2.Get(rec2, req2)
		if rec2.Code != http.StatusOK {
			t.Fatalf("poll: expected 200, got %d", rec2.Code)
		}
		var results2 []search.PackageEntry
		if err := json.NewDecoder(rec2.Body).Decode(&results2); err != nil {
			t.Fatalf("decode poll response: %v", err)
		}
		if len(results2) > 0 && results2[0].Downloads == 999_999_999 {
			// Cache was refreshed to v2.
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("popular cache was not refreshed to v2 data within 2s")
}
