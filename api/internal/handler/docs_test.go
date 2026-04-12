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
	// Mock PyPI server returning package metadata.
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/pypi/requests/json" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"info":{"name":"requests","version":"2.33.1"},"urls":[],"releases":{}}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer pypiSrv.Close()

	// Mock sidecar returning docs JSON.
	sidecarSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/generate" && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{
				"empty": false,
				"reason": "",
				"modules": [{
					"name": "requests",
					"functions": [{"name":"get","kind":"function","signature":"def get(url: str)","docstring":"Sends a GET request.","parameters":[{"name":"url","type":"str","description":"The URL."}],"returns":{"type":"Response","description":""}}],
					"classes": [],
					"exceptions": []
				}]
			}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer sidecarSrv.Close()

	sqliteCache, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	defer sqliteCache.Close()
	memCache := cache.NewMemoryCache(sqliteCache, 100)

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	h := handler.NewDocsHandler(pypiClient, memCache, sidecarSrv.URL)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/docs", h.Get)

	t.Run("returns docs for available package", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/packages/requests/docs", nil)
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
		if resp.Package != "requests" {
			t.Errorf("Package = %q, want requests", resp.Package)
		}
		if resp.Version != "2.33.1" {
			t.Errorf("Version = %q, want 2.33.1", resp.Version)
		}
		if len(resp.Modules) == 0 {
			t.Error("Modules should not be empty")
		}
	})

	t.Run("second request served from cache", func(t *testing.T) {
		// Shut down the sidecar — cache should serve the second request.
		sidecarSrv.Close()

		req := httptest.NewRequest(http.MethodGet, "/api/packages/requests/docs", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (expected cache hit)", w.Code)
		}
	})
}

func TestDocsHandlerSidecarUnavailable(t *testing.T) {
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"info":{"name":"mypkg","version":"1.0.0"},"urls":[],"releases":{}}`)
	}))
	defer pypiSrv.Close()

	sqliteCache, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	defer sqliteCache.Close()
	memCache := cache.NewMemoryCache(sqliteCache, 100)

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	// Point at a port nothing is listening on.
	h := handler.NewDocsHandler(pypiClient, memCache, "http://127.0.0.1:19999")

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/docs", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/packages/mypkg/docs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}
