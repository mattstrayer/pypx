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
