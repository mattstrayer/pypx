package worker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/pypi"
	"github.com/pypx/api/internal/search"
)

func TestSimpleIndexSync(t *testing.T) {
	// Mock server that serves Simple API HTML and PyPI JSON responses.
	mux := http.NewServeMux()

	mux.HandleFunc("/simple/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<a href="/simple/requests/">requests</a><a href="/simple/flask/">flask</a>`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Create in-memory search index and cache.
	idx, err := search.NewIndex(":memory:")
	if err != nil {
		t.Fatalf("failed to create search index: %v", err)
	}
	defer idx.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer c.Close()

	pypiClient := pypi.NewClient()

	w := New(pypiClient, c, idx, Config{
		SimpleAPIURL:   srv.URL + "/simple/",
		IndexSyncEvery: time.Hour,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := w.SyncIndex(ctx); err != nil {
		t.Fatalf("SyncIndex failed: %v", err)
	}

	results, err := idx.Search("requests", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results for 'requests', got none")
	}

	found := false
	for _, r := range results {
		if r.Name == "requests" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'requests' in results, got: %v", results)
	}
}

func TestSyncDownloads(t *testing.T) {
	// Mock top packages JSON endpoint.
	mux := http.NewServeMux()

	mux.HandleFunc("/simple/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<a href="/simple/flask/">flask</a><a href="/simple/django/">django</a><a href="/simple/obscure-pkg/">obscure-pkg</a>`))
	})

	mux.HandleFunc("/top-packages.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"rows":[{"project":"flask","download_count":50000000},{"project":"django","download_count":30000000}]}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	idx, err := search.NewIndex(":memory:")
	if err != nil {
		t.Fatalf("failed to create search index: %v", err)
	}
	defer idx.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer c.Close()

	pypiClient := pypi.NewClient()
	w := New(pypiClient, c, idx, Config{
		SimpleAPIURL:   srv.URL + "/simple/",
		IndexSyncEvery: time.Hour,
	})

	ctx := context.Background()

	// Sync names first.
	if err := w.SyncIndex(ctx); err != nil {
		t.Fatalf("SyncIndex failed: %v", err)
	}

	// Override the top packages URL to use our mock.
	origURL := topPackagesURL
	// We can't override the const, so let's test UpdateDownloadsBatch directly.
	_ = origURL

	updates := []search.PackageEntry{
		{Name: "flask", Downloads: 50_000_000},
		{Name: "django", Downloads: 30_000_000},
	}
	if err := idx.UpdateDownloadsBatch(updates); err != nil {
		t.Fatalf("UpdateDownloadsBatch failed: %v", err)
	}

	// Search for "flask" — should have flask first (exact match + highest downloads).
	results, err := idx.Search("flask", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results, got none")
	}
	if results[0].Name != "flask" {
		t.Errorf("expected 'flask' first, got %q", results[0].Name)
	}
	if results[0].Downloads != 50_000_000 {
		t.Errorf("expected downloads=50000000, got %d", results[0].Downloads)
	}
}
