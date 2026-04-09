package handler_test

import (
	"encoding/json"
	"fmt"
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

// mockRequestsVersionsResponse returns a PyPI JSON response with two releases.
const mockRequestsVersionsResponse = `{
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
	"releases": {
		"2.31.0": [
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
		],
		"2.30.0": [
			{
				"filename": "requests-2.30.0-py3-none-any.whl",
				"url": "https://files.pythonhosted.org/packages/requests-2.30.0-py3-none-any.whl",
				"size": 62000,
				"packagetype": "bdist_wheel",
				"python_version": "py3",
				"requires_python": ">=3.7",
				"upload_time_iso_8601": "2023-04-26T10:00:00.000Z",
				"digests": {"sha256": "def456"}
			}
		]
	},
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

// mockDepsResponse returns a PyPI JSON response with requires_dist containing
// 2 required deps and 1 extra ("socks").
const mockDepsResponse = `{
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
		"requires_python": ">=3.7",
		"requires_dist": [
			"charset-normalizer (<4,>=2)",
			"urllib3 (>=1.21.1)",
			"PySocks (>=1.5.6); extra == \"socks\""
		],
		"project_urls": null,
		"classifiers": null
	},
	"releases": {},
	"urls": []
}`

func setupRouter(h *handler.PackageHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/api/packages/{name}", h.Get)
	r.Get("/api/packages/{name}/versions", h.GetVersions)
	r.Get("/api/packages/{name}/dependencies", h.GetDependencies)
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

	// install_size should equal the wheel size from the mock (62574).
	if got, ok := body["install_size"].(float64); !ok || got != 62574 {
		t.Errorf("expected install_size=62574, got %v", body["install_size"])
	}

	// module_format should be a non-empty string.
	if got, ok := body["module_format"].(string); !ok || got == "" {
		t.Errorf("expected non-empty module_format, got %v", body["module_format"])
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

func TestGetVersions(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockRequestsVersionsResponse)) //nolint:errcheck
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

	req := httptest.NewRequest(http.MethodGet, "/api/packages/requests/versions", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var versions []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&versions); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(versions))
	}
}

func TestGetPackageInvalidName(t *testing.T) {
	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer c.Close()

	// No mock PyPI server needed — validation must short-circuit before any HTTP call.
	client := pypi.NewClient(pypi.WithBaseURL("http://127.0.0.1:0"))
	h := handler.NewPackageHandler(client, c)
	router := setupRouter(h)

	// Names that are routable by chi but fail our validation.
	// (Slash/space characters can't be used in httptest URLs, but are
	// covered by TestValidateName at the unit level.)
	invalidNames := []string{
		"-leading-hyphen",
		"trailing-hyphen-",
		".hidden",
	}

	endpoints := []string{
		"/api/packages/%s",
		"/api/packages/%s/versions",
		"/api/packages/%s/dependencies",
	}

	for _, name := range invalidNames {
		for _, endpointFmt := range endpoints {
			path := fmt.Sprintf(endpointFmt, name)
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("GET %s (name=%q): expected 400, got %d", path, name, rr.Code)
			}
		}
	}
}

func TestGetDependencies(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockDepsResponse)) //nolint:errcheck
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

	req := httptest.NewRequest(http.MethodGet, "/api/packages/requests/dependencies", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var tree struct {
		Required []map[string]interface{}            `json:"required"`
		Extras   map[string][]map[string]interface{} `json:"extras"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&tree); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(tree.Required) != 2 {
		t.Errorf("expected 2 required deps, got %d", len(tree.Required))
	}

	socksDeps, ok := tree.Extras["socks"]
	if !ok {
		t.Fatal("expected extras to contain 'socks' key")
	}
	if len(socksDeps) != 1 {
		t.Errorf("expected 1 dep in socks extra, got %d", len(socksDeps))
	}
}
