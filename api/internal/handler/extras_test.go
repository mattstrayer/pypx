package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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
	h := handler.NewExtrasHandler(pypiClient, condaClient, memCache)

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
