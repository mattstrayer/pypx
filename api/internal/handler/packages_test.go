package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/pypi"
)

// mockPyPIResponse returns a minimal PyPI JSON response for "requests".
const mockRequestsResponse = `{
	"info": {
		"name": "requests",
		"version": "2.31.0",
		"summary": "HTTP for Humans",
		"description": "",
		"description_content_type": "",
		"license": "",
		"author": "",
		"author_email": "",
		"home_page": "",
		"requires_python": "",
		"requires_dist": null,
		"project_urls": null,
		"classifiers": null
	},
	"releases": {},
	"urls": [
		{
			"filename": "requests-2.31.0-py3-none-any.whl",
			"url": "https://files.pythonhosted.org/packages/requests-2.31.0-py3-none-any.whl",
			"size": 62574,
			"packagetype": "bdist_wheel",
			"python_version": "py3",
			"requires_python": ">=3.7",
			"upload_time_iso_8601": "2023-05-22T15:12:01.000Z",
			"digests": {"sha256": "abc123"}
		}
	]
}`

func setupRouter(h *handler.PackageHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/api/packages/{name}", h.Get)
	return r
}

func TestGetPackage(t *testing.T) {
	// Mock PyPI server.
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockRequestsResponse)) //nolint:errcheck
	}))
	defer mock.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer c.Close()

	client := pypi.NewClient(pypi.WithBaseURL(mock.URL))
	h := handler.NewPackageHandler(client, c)
	router := setupRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/packages/requests", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if got := body["name"]; got != "requests" {
		t.Errorf("expected name=requests, got %v", got)
	}
}

func TestGetPackageCached(t *testing.T) {
	var callCount atomic.Int32

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockRequestsResponse)) //nolint:errcheck
	}))
	defer mock.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer c.Close()

	client := pypi.NewClient(pypi.WithBaseURL(mock.URL))
	h := handler.NewPackageHandler(client, c)
	router := setupRouter(h)

	// First request.
	req1 := httptest.NewRequest(http.MethodGet, "/api/packages/requests", nil)
	rr1 := httptest.NewRecorder()
	router.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", rr1.Code)
	}

	// Second request — should be served from cache.
	req2 := httptest.NewRequest(http.MethodGet, "/api/packages/requests", nil)
	rr2 := httptest.NewRecorder()
	router.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("second request: expected 200, got %d", rr2.Code)
	}

	if n := callCount.Load(); n != 1 {
		t.Errorf("expected PyPI server to be called once, got %d", n)
	}
}

func TestGetPackageNotFound(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer mock.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer c.Close()

	client := pypi.NewClient(pypi.WithBaseURL(mock.URL))
	h := handler.NewPackageHandler(client, c)
	router := setupRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/packages/nonexistent-package-xyz", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}
