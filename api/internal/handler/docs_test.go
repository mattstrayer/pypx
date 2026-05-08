package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestDocsHandlerGetText(t *testing.T) {
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"info":{"name":"httpx","version":"0.27.0"},"urls":[],"releases":{}}`)
	}))
	defer pypiSrv.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	memCache := cache.NewMemoryCache(c, 100)

	// Seed the docs cache with a known DocsResponse so the handler skips goopy.
	docsJSON := []byte(`{"package":"httpx","version":"0.27.0","available":true,"modules":[{"name":"httpx","functions":[{"name":"get","kind":"function","signature":"def get(url: str) -> Response","docstring":"Send a GET request.","parameters":[{"name":"url","type":"str","description":"URL"}],"returns":{"type":"Response"}}],"classes":[{"name":"Client","kind":"class","signature":"class Client","docstring":"An HTTP client.","methods":[{"name":"get","kind":"method","signature":"def get(self, url: str) -> Response","docstring":"Send a GET from this client.","parameters":[{"name":"self"},{"name":"url","type":"str"}],"returns":{"type":"Response"}}]}],"exceptions":[]}]}`)
	if err := memCache.Set("docs:httpx:0.27.0", docsJSON, 0); err != nil {
		t.Fatal(err)
	}

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	h := handler.NewDocsHandler(pypiClient, memCache)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/docs.txt", h.GetText)

	// Full dump
	req := httptest.NewRequest("GET", "/api/packages/httpx/docs.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain prefix", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "## httpx (module)") {
		t.Errorf("expected module heading, got:\n%s", body)
	}
	if !strings.Contains(body, "### httpx.Client") {
		t.Errorf("expected class heading, got:\n%s", body)
	}
	if !strings.Contains(body, "### httpx.get") {
		t.Errorf("expected function heading, got:\n%s", body)
	}

	// Prefix filter
	req2 := httptest.NewRequest("GET", "/api/packages/httpx/docs.txt?prefix=httpx.Client", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Fatalf("prefix status = %d", w2.Code)
	}
	body2 := w2.Body.String()
	if !strings.Contains(body2, "### httpx.Client") {
		t.Errorf("prefix output missing class: %s", body2)
	}
	// Top-level function "httpx.get" must NOT appear when prefix=httpx.Client.
	// Match a unique substring that proves the function block is absent (its heading line).
	if strings.Contains(body2, "### httpx.get —") {
		t.Errorf("prefix output should not include httpx.get top-level function: %s", body2)
	}
}

func TestDocsHandlerGetSymbol(t *testing.T) {
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"info":{"name":"httpx","version":"0.27.0"},"urls":[],"releases":{}}`)
	}))
	defer pypiSrv.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	memCache := cache.NewMemoryCache(c, 100)

	docsJSON := []byte(`{"package":"httpx","version":"0.27.0","available":true,"modules":[{"name":"httpx","functions":[],"classes":[{"name":"Client","kind":"class","signature":"class Client","methods":[{"name":"get","kind":"method","signature":"def get(self) -> Response","docstring":"Send GET."}]}],"exceptions":[]}]}`)
	if err := memCache.Set("docs:httpx:0.27.0", docsJSON, 0); err != nil {
		t.Fatal(err)
	}

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	h := handler.NewDocsHandler(pypiClient, memCache)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/docs/{symbol}", h.GetSymbol)

	// Existing dotted symbol
	req := httptest.NewRequest("GET", "/api/packages/httpx/docs/Client.get.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "### httpx.Client.get") {
		t.Errorf("expected symbol heading, got:\n%s", body)
	}
	if !strings.Contains(body, "Send GET.") {
		t.Errorf("expected docstring, got:\n%s", body)
	}

	// Missing symbol → 404
	req2 := httptest.NewRequest("GET", "/api/packages/httpx/docs/does.not.exist.txt", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != 404 {
		t.Errorf("missing symbol status = %d, want 404", w2.Code)
	}
}
