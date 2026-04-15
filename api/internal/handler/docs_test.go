package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/pypi"
)

func TestDocsHandlerGet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping docs handler test (requires network)")
	}

	// Mock PyPI metadata server — goopy will download the real wheel from pypi.org.
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/pypi/click/json" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"info":{"name":"click","version":"8.1.8"},"urls":[],"releases":{}}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer pypiSrv.Close()

	sqliteCache, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	defer sqliteCache.Close()
	memCache := cache.NewMemoryCache(sqliteCache, 100)

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	h := handler.NewDocsHandler(pypiClient, memCache)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/docs", h.Get)

	t.Run("returns docs for available package", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/packages/click/docs", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
		}
		var resp handler.DocsResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !resp.Available {
			t.Error("Available should be true")
		}
		if resp.Package != "click" {
			t.Errorf("Package = %q, want click", resp.Package)
		}
		if resp.Version != "8.1.8" {
			t.Errorf("Version = %q, want 8.1.8", resp.Version)
		}
		if len(resp.Modules) == 0 {
			t.Error("Modules should not be empty")
		}

		// Verify module structure has expected fields.
		foundFunc := false
		for _, mod := range resp.Modules {
			for _, fn := range mod.Functions {
				if fn.Name != "" && fn.Kind == "function" && fn.Signature != "" {
					foundFunc = true
					break
				}
			}
		}
		if !foundFunc {
			t.Error("expected at least one function with name, kind, and signature")
		}
	})

	t.Run("second request served from cache", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/packages/click/docs", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (expected cache hit)", w.Code)
		}
	})
}

func TestDocsHandlerPackageNotFound(t *testing.T) {
	// PyPI returns 404 for unknown packages.
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer pypiSrv.Close()

	sqliteCache, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	defer sqliteCache.Close()
	memCache := cache.NewMemoryCache(sqliteCache, 100)

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	h := handler.NewDocsHandler(pypiClient, memCache)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/docs", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/packages/nonexistent-pkg-xyz/docs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
