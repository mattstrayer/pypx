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

func newTestWorker(t *testing.T, mux *http.ServeMux) (*Worker, *search.Index) {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	idx, err := search.NewIndex(":memory:")
	if err != nil {
		t.Fatalf("failed to create search index: %v", err)
	}
	t.Cleanup(func() { idx.Close() })

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	t.Cleanup(func() { c.Close() })

	w := New(pypi.NewClient(), c, idx, Config{
		SimpleAPIURL:   srv.URL + "/simple/",
		TopPackagesURL: srv.URL + "/top-packages.json",
		IndexSyncEvery: time.Hour,
	})
	return w, idx
}

// TestSyncIndex_JSONFormat verifies that SyncIndex correctly parses the PyPI
// Simple API JSON response and inserts all packages into the search index.
func TestSyncIndex_JSONFormat(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/simple/", func(w http.ResponseWriter, r *http.Request) {
		// Verify the JSON Accept header is sent.
		accept := r.Header.Get("Accept")
		if accept != "application/vnd.pypi.simple.v1+json" {
			t.Errorf("expected JSON Accept header, got %q", accept)
		}
		w.Header().Set("Content-Type", "application/vnd.pypi.simple.v1+json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"projects":[{"name":"requests"},{"name":"Flask"},{"name":"django"}]}`))
	})

	w, idx := newTestWorker(t, mux)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := w.SyncIndex(ctx); err != nil {
		t.Fatalf("SyncIndex failed: %v", err)
	}

	// All three packages should be searchable.
	for _, name := range []string{"requests", "flask", "django"} {
		results, err := idx.Search(name, 10)
		if err != nil {
			t.Fatalf("Search(%q): %v", name, err)
		}
		if len(results) == 0 {
			t.Errorf("Search(%q): expected results, got none", name)
		}
	}
}

// TestSyncIndex_PreservesCase verifies that package names from the JSON API
// are stored with their original casing.
func TestSyncIndex_PreservesCase(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/simple/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.pypi.simple.v1+json")
		_, _ = w.Write([]byte(`{"projects":[{"name":"Flask"},{"name":"Jinja2"}]}`))
	})

	w, idx := newTestWorker(t, mux)

	if err := w.SyncIndex(context.Background()); err != nil {
		t.Fatalf("SyncIndex failed: %v", err)
	}

	results, _ := idx.Search("flask", 1)
	if len(results) == 0 {
		t.Fatal("no results for flask")
	}
	if results[0].Name != "Flask" {
		t.Errorf("expected preserved case 'Flask', got %q", results[0].Name)
	}
}

// TestSyncDownloads_EndToEnd verifies the full SyncDownloads flow: fetch mock
// top packages JSON, update download counts in the index, and verify search
// results are ranked by popularity.
func TestSyncDownloads_EndToEnd(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/simple/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.pypi.simple.v1+json")
		_, _ = w.Write([]byte(`{"projects":[{"name":"flask"},{"name":"django"},{"name":"obscure-pkg"}]}`))
	})
	mux.HandleFunc("/top-packages.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"rows":[{"project":"flask","download_count":50000000},{"project":"django","download_count":30000000}]}`))
	})

	w, idx := newTestWorker(t, mux)
	ctx := context.Background()

	// Sync names first, then downloads.
	if err := w.SyncIndex(ctx); err != nil {
		t.Fatalf("SyncIndex failed: %v", err)
	}
	if err := w.SyncDownloads(ctx); err != nil {
		t.Fatalf("SyncDownloads failed: %v", err)
	}

	// flask should be first (exact match), django second (30M > 0).
	results, err := idx.Search("flask", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if results[0].Name != "flask" {
		t.Errorf("expected flask first, got %q", results[0].Name)
	}
	if results[0].Downloads != 50_000_000 {
		t.Errorf("expected flask downloads=50000000, got %d", results[0].Downloads)
	}

	// obscure-pkg should have 0 downloads (not in top packages list).
	results, _ = idx.Search("obscure-pkg", 1)
	if len(results) > 0 && results[0].Downloads != 0 {
		t.Errorf("expected obscure-pkg downloads=0, got %d", results[0].Downloads)
	}
}

// TestSyncDownloads_CaseInsensitive verifies that SyncDownloads correctly
// updates downloads for packages with different casing between PyPI Simple
// API (e.g. "Flask") and the top packages dataset (e.g. "flask").
func TestSyncDownloads_CaseInsensitive(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/simple/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.pypi.simple.v1+json")
		// PyPI returns mixed-case names.
		_, _ = w.Write([]byte(`{"projects":[{"name":"Flask"},{"name":"Jinja2"},{"name":"MarkupSafe"}]}`))
	})
	mux.HandleFunc("/top-packages.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Top packages uses lowercase.
		_, _ = w.Write([]byte(`{"rows":[{"project":"flask","download_count":220000000},{"project":"jinja2","download_count":300000000},{"project":"markupsafe","download_count":400000000}]}`))
	})

	w, idx := newTestWorker(t, mux)
	ctx := context.Background()

	if err := w.SyncIndex(ctx); err != nil {
		t.Fatalf("SyncIndex: %v", err)
	}
	if err := w.SyncDownloads(ctx); err != nil {
		t.Fatalf("SyncDownloads: %v", err)
	}

	// All three should have non-zero downloads despite case mismatch.
	for _, tc := range []struct {
		query     string
		wantName  string
		wantDL    int64
	}{
		{"flask", "Flask", 220_000_000},
		{"jinja2", "Jinja2", 300_000_000},
		{"markupsafe", "MarkupSafe", 400_000_000},
	} {
		results, err := idx.Search(tc.query, 1)
		if err != nil {
			t.Fatalf("Search(%q): %v", tc.query, err)
		}
		if len(results) == 0 {
			t.Fatalf("Search(%q): no results", tc.query)
		}
		if results[0].Name != tc.wantName {
			t.Errorf("Search(%q): expected name %q, got %q", tc.query, tc.wantName, results[0].Name)
		}
		if results[0].Downloads != tc.wantDL {
			t.Errorf("Search(%q): expected downloads=%d, got %d", tc.query, tc.wantDL, results[0].Downloads)
		}
	}
}

// TestSyncDownloads_HTTPError verifies that SyncDownloads returns an error
// when the top packages endpoint is unavailable.
func TestSyncDownloads_HTTPError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/simple/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.pypi.simple.v1+json")
		_, _ = w.Write([]byte(`{"projects":[{"name":"flask"}]}`))
	})
	mux.HandleFunc("/top-packages.json", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	})

	w, _ := newTestWorker(t, mux)
	ctx := context.Background()

	if err := w.SyncIndex(ctx); err != nil {
		t.Fatalf("SyncIndex: %v", err)
	}

	err := w.SyncDownloads(ctx)
	if err == nil {
		t.Error("expected error from SyncDownloads when endpoint returns 503, got nil")
	}
}

// TestSyncIndex_Idempotent verifies that running SyncIndex twice does not
// create duplicate entries in the search index.
func TestSyncIndex_Idempotent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/simple/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.pypi.simple.v1+json")
		_, _ = w.Write([]byte(`{"projects":[{"name":"flask"},{"name":"django"}]}`))
	})

	w, idx := newTestWorker(t, mux)
	ctx := context.Background()

	// Run sync twice.
	if err := w.SyncIndex(ctx); err != nil {
		t.Fatalf("first SyncIndex: %v", err)
	}
	if err := w.SyncIndex(ctx); err != nil {
		t.Fatalf("second SyncIndex: %v", err)
	}

	// Should have exactly 1 result for "django", not 2.
	results, _ := idx.Search("django", 10)
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'django' after double sync, got %d", len(results))
	}
}
