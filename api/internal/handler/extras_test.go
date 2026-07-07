package handler_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/conda"
	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/pypi"
)

func TestExtrasHandlerGet(t *testing.T) {
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// types-requests exists, requests-stubs does not
		if r.URL.Path == "/pypi/types-requests/json" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer pypiSrv.Close()

	condaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/package/conda-forge/requests" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"name":"requests","latest_version":"2.32.3"}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer condaSrv.Close()

	// Use the same cache creation pattern as other handler tests in this package
	sqliteCache, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	defer sqliteCache.Close()
	memCache := cache.NewMemoryCache(sqliteCache, 100)

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	condaClient := conda.NewClient(conda.WithBaseURL(condaSrv.URL))
	h := handler.NewExtrasHandler(pypiClient, condaClient, nil, nil, memCache)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/extras", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/packages/requests/extras", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp handler.ExtrasResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.TypeSupport.Status != "stubs" {
		t.Errorf("TypeSupport.Status = %q, want stubs", resp.TypeSupport.Status)
	}
	if resp.TypeSupport.StubsPackage != "types-requests" {
		t.Errorf("StubsPackage = %q, want types-requests", resp.TypeSupport.StubsPackage)
	}
	if resp.CondaForge == nil || !resp.CondaForge.Available {
		t.Error("CondaForge.Available should be true")
	}
}

func TestExtrasHandlerGetText(t *testing.T) {
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// types-requests exists — stubs
		if r.URL.Path == "/pypi/types-requests/json" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer pypiSrv.Close()

	condaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/package/conda-forge/requests" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"name":"requests","latest_version":"2.32.3"}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer condaSrv.Close()

	sqliteCache, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	defer sqliteCache.Close()
	memCache := cache.NewMemoryCache(sqliteCache, 100)

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	condaClient := conda.NewClient(conda.WithBaseURL(condaSrv.URL))
	h := handler.NewExtrasHandler(pypiClient, condaClient, nil, nil, memCache)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/extras.txt", h.GetText)

	req := httptest.NewRequest(http.MethodGet, "/api/packages/requests/extras.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/plain; charset=utf-8", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "package: requests") {
		t.Errorf("body missing 'package: requests'; got:\n%s", body)
	}
	if !strings.Contains(body, "type_status: stubs") {
		t.Errorf("body missing 'type_status: stubs'; got:\n%s", body)
	}
	if !strings.Contains(body, "stub_package: types-requests") {
		t.Errorf("body missing 'stub_package: types-requests'; got:\n%s", body)
	}
	if !strings.Contains(body, "available: true") {
		t.Errorf("body missing 'available: true'; got:\n%s", body)
	}
}

func TestExtrasHandlerGetPyTyped(t *testing.T) {
	// Build a minimal wheel zip with py.typed.
	var wheelBuf bytes.Buffer
	zw := zip.NewWriter(&wheelBuf)
	zw.Create("typed_pkg-1.0.0.dist-info/METADATA") //nolint:errcheck
	zw.Create("typed_pkg/py.typed")                  //nolint:errcheck
	zw.Create("typed_pkg/__init__.py")               //nolint:errcheck
	zw.Close()
	wheelBytes := wheelBuf.Bytes()

	var srvURL string
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		// Stubs package checks — none exist.
		case "/pypi/types-typed-pkg/json", "/pypi/typed-pkg-stubs/json":
			w.WriteHeader(http.StatusNotFound)
		// Package metadata — return version + wheel URL pointing to this server.
		case "/pypi/typed-pkg/json":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{
				"info": {"name":"typed-pkg","version":"1.0.0"},
				"urls": [{"packagetype":"bdist_wheel","url":"%s/wheel.whl","filename":"typed_pkg-1.0.0-py3-none-any.whl","size":%d}],
				"releases": {}
			}`, srvURL, len(wheelBytes))
		// Wheel file.
		case "/wheel.whl":
			if r.Method == http.MethodHead {
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(wheelBytes)))
				w.Header().Set("Accept-Ranges", "bytes")
				return
			}
			http.ServeContent(w, r, "wheel.whl", time.Time{}, bytes.NewReader(wheelBytes))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer pypiSrv.Close()
	srvURL = pypiSrv.URL

	condaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer condaSrv.Close()

	sqliteCache, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	defer sqliteCache.Close()
	memCache := cache.NewMemoryCache(sqliteCache, 100)

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	condaClient := conda.NewClient(conda.WithBaseURL(condaSrv.URL))
	pkgHandler := handler.NewPackageHandler(pypiClient, memCache)
	h := handler.NewExtrasHandler(pypiClient, condaClient, nil, pkgHandler, memCache)

	router := chi.NewRouter()
	router.Get("/api/packages/{name}/extras", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/packages/typed-pkg/extras", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp handler.ExtrasResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.TypeSupport.Status != "typed" {
		t.Errorf("TypeSupport.Status = %q, want typed", resp.TypeSupport.Status)
	}
}

// TestExtrasHandlerGetTypedAndStubs locks the precedence when a package both
// ships a py.typed marker AND has a known stubs package: Status resolves to
// "typed" but StubsPackage is retained (extras.go:95-112).
func TestExtrasHandlerGetTypedAndStubs(t *testing.T) {
	// Build a minimal wheel zip with py.typed.
	var wheelBuf bytes.Buffer
	zw := zip.NewWriter(&wheelBuf)
	zw.Create("typed_pkg-1.0.0.dist-info/METADATA") //nolint:errcheck
	zw.Create("typed_pkg/py.typed")                  //nolint:errcheck
	zw.Create("typed_pkg/__init__.py")               //nolint:errcheck
	zw.Close()
	wheelBytes := wheelBuf.Bytes()

	var srvURL string
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		// Stubs package check — types-typed-pkg exists this time.
		case "/pypi/types-typed-pkg/json":
			w.WriteHeader(http.StatusOK)
		case "/pypi/typed-pkg-stubs/json":
			w.WriteHeader(http.StatusNotFound)
		// Package metadata — return version + wheel URL pointing to this server.
		case "/pypi/typed-pkg/json":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{
				"info": {"name":"typed-pkg","version":"1.0.0"},
				"urls": [{"packagetype":"bdist_wheel","url":"%s/wheel.whl","filename":"typed_pkg-1.0.0-py3-none-any.whl","size":%d}],
				"releases": {}
			}`, srvURL, len(wheelBytes))
		// Wheel file.
		case "/wheel.whl":
			if r.Method == http.MethodHead {
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(wheelBytes)))
				w.Header().Set("Accept-Ranges", "bytes")
				return
			}
			http.ServeContent(w, r, "wheel.whl", time.Time{}, bytes.NewReader(wheelBytes))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer pypiSrv.Close()
	srvURL = pypiSrv.URL

	condaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer condaSrv.Close()

	sqliteCache, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	defer sqliteCache.Close()
	memCache := cache.NewMemoryCache(sqliteCache, 100)

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	condaClient := conda.NewClient(conda.WithBaseURL(condaSrv.URL))
	pkgHandler := handler.NewPackageHandler(pypiClient, memCache)
	h := handler.NewExtrasHandler(pypiClient, condaClient, nil, pkgHandler, memCache)

	router := chi.NewRouter()
	router.Get("/api/packages/{name}/extras", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/packages/typed-pkg/extras", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp handler.ExtrasResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.TypeSupport.Status != "typed" {
		t.Errorf("TypeSupport.Status = %q, want typed", resp.TypeSupport.Status)
	}
	if resp.TypeSupport.StubsPackage != "types-typed-pkg" {
		t.Errorf("StubsPackage = %q, want types-typed-pkg", resp.TypeSupport.StubsPackage)
	}
}
