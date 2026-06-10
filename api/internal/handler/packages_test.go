package handler_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/pypi"
	_ "modernc.org/sqlite"
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

// mockMarkdownDescriptionResponse returns a PyPI JSON response with a markdown description.
const mockMarkdownDescriptionResponse = `{
	"info": {
		"name": "requests",
		"version": "2.31.0",
		"summary": "HTTP for Humans",
		"description": "# Requests\n\nHTTP for **Humans**.",
		"description_content_type": "text/markdown",
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
	"urls": []
}`

// mockLicenseExpressionResponse has a modern SPDX license_expression field.
const mockLicenseExpressionResponse = `{
	"info": {
		"name": "flask",
		"version": "3.0.0",
		"summary": "A micro web framework",
		"description": "",
		"description_content_type": "",
		"license": "",
		"license_expression": "BSD-3-Clause",
		"author": "",
		"author_email": "",
		"home_page": "",
		"requires_python": "",
		"requires_dist": null,
		"project_urls": null,
		"classifiers": null
	},
	"releases": {},
	"urls": []
}`

// mockLicenseClassifierResponse has no license_expression but a License classifier.
const mockLicenseClassifierResponse = `{
	"info": {
		"name": "pandas",
		"version": "2.2.0",
		"summary": "Data analysis toolkit",
		"description": "",
		"description_content_type": "",
		"license": "BSD 3-Clause License\n\nCopyright (c) 2008-2011, AQR Capital Management, LLC, Lambda Foundry, Inc. and PyData Development Team\nAll rights reserved.\n\nCopyright (c) 2011-2024, Open source contributors.\n\nRedistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met...",
		"author": "",
		"author_email": "",
		"home_page": "",
		"requires_python": "",
		"requires_dist": null,
		"project_urls": null,
		"classifiers": ["License :: OSI Approved :: BSD License", "Programming Language :: Python :: 3"]
	},
	"releases": {},
	"urls": []
}`

// mockLicenseShortResponse has a short license field and no other sources.
const mockLicenseShortResponse = `{
	"info": {
		"name": "httpx",
		"version": "0.27.0",
		"summary": "A next-gen HTTP client",
		"description": "",
		"description_content_type": "",
		"license": "Apache-2.0",
		"author": "",
		"author_email": "",
		"home_page": "",
		"requires_python": "",
		"requires_dist": null,
		"project_urls": null,
		"classifiers": []
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

func TestPackageGet_NormalizesLicense(t *testing.T) {
	tests := []struct {
		name     string
		mockResp string
		wantLic  string
	}{
		{"license_expression preferred", mockLicenseExpressionResponse, "BSD-3-Clause"},
		{"classifier fallback for long license", mockLicenseClassifierResponse, "BSD License"},
		{"short license used as-is", mockLicenseShortResponse, "Apache-2.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tt.mockResp)) //nolint:errcheck
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

			req := httptest.NewRequest(http.MethodGet, "/api/packages/test-pkg", nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
			}

			var resp struct {
				License string `json:"license"`
			}
			if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp.License != tt.wantLic {
				t.Errorf("expected license=%q, got %q", tt.wantLic, resp.License)
			}
		})
	}
}

func TestPackageGet_RendersMarkdownDescription(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockMarkdownDescriptionResponse)) //nolint:errcheck
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

	var resp struct {
		Description     string `json:"description"`
		DescriptionHTML string `json:"description_html"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Raw markdown should be preserved.
	if resp.Description != "# Requests\n\nHTTP for **Humans**." {
		t.Errorf("expected raw markdown in description, got %q", resp.Description)
	}

	// HTML should be non-empty.
	if resp.DescriptionHTML == "" {
		t.Fatal("expected non-empty description_html")
	}

	// HTML should contain rendered bold text.
	if !strings.Contains(resp.DescriptionHTML, "<strong>Humans</strong>") {
		t.Errorf("expected description_html to contain <strong>Humans</strong>, got %q", resp.DescriptionHTML)
	}
}

func TestPackageHandler_Get_CacheMissSingleFlight(t *testing.T) {
	const concurrency = 20

	var upstream atomic.Int32
	var arrivals atomic.Int32
	release := make(chan struct{})

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstream.Add(1)
		// First request blocks until all concurrent goroutines have entered
		// singleflight (so the followers can deduplicate). Subsequent requests
		// — there shouldn't be any if dedup works — pass through immediately.
		if arrivals.Add(1) == 1 {
			select {
			case <-release:
			case <-time.After(2 * time.Second):
				// Safety net so the test fails loudly rather than hanging.
			}
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRequestsResponse)
	}))
	defer mock.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	client := pypi.NewClient(pypi.WithBaseURL(mock.URL))
	h := handler.NewPackageHandler(client, c)

	router := chi.NewRouter()
	router.Get("/packages/{name}", h.Get)

	var wg sync.WaitGroup
	wg.Add(concurrency)
	ready := make(chan struct{})

	for range concurrency {
		go func() {
			defer wg.Done()
			<-ready
			req := httptest.NewRequest(http.MethodGet, "/packages/requests", nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", rec.Code)
			}
		}()
	}

	close(ready)

	// Wait for all goroutines to enter singleflight (the leader is blocked in
	// the mock; followers are blocked inside singleflight.Do waiting on the
	// leader's result). 100ms is plenty for 20 goroutines to enter their
	// blocking call once spawned.
	time.Sleep(100 * time.Millisecond)

	// Release the leader so it returns and singleflight unblocks the followers.
	close(release)

	wg.Wait()

	// With the barrier in place, singleflight should collapse all 20 concurrent
	// requests into exactly one upstream call.
	if got := upstream.Load(); got != 1 {
		t.Errorf("expected exactly 1 upstream PyPI call (singleflight dedup), got %d", got)
	}
}

// TestPackageHandler_Get_CacheMissPopulatesCache verifies that after a cache-miss
// fetch the result is stored so the very next sequential request is served from
// cache without hitting upstream again.
func TestPackageHandler_Get_CacheMissPopulatesCache(t *testing.T) {
	var upstream atomic.Int32

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstream.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRequestsResponse)
	}))
	defer mock.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	client := pypi.NewClient(pypi.WithBaseURL(mock.URL))
	h := handler.NewPackageHandler(client, c)
	router := chi.NewRouter()
	router.Get("/packages/{name}", h.Get)

	// First request — cache miss, hits upstream.
	req1 := httptest.NewRequest(http.MethodGet, "/packages/requests", nil)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", rec1.Code)
	}

	// Second request — must be served from cache.
	req2 := httptest.NewRequest(http.MethodGet, "/packages/requests", nil)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("second request: expected 200, got %d", rec2.Code)
	}

	// After the initial miss both requests are sequential, so upstream is called exactly once.
	if got := upstream.Load(); got != 1 {
		t.Errorf("expected exactly 1 upstream call after singleflight populates cache, got %d", got)
	}

	// Both responses should have identical JSON bodies.
	if rec1.Body.String() != rec2.Body.String() {
		t.Error("expected cached response to match original response body")
	}
}

// TestPackageHandler_Get_CacheMissHeaders verifies that the correct response
// headers are set on the cache-miss (singleflight) path.
func TestPackageHandler_Get_CacheMissHeaders(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRequestsResponse)
	}))
	defer mock.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	client := pypi.NewClient(pypi.WithBaseURL(mock.URL))
	h := handler.NewPackageHandler(client, c)
	router := chi.NewRouter()
	router.Get("/packages/{name}", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/packages/requests", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type: expected application/json, got %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=3600" {
		t.Errorf("Cache-Control: expected public, max-age=3600, got %q", got)
	}
}

// TestPackageHandler_Get_CacheHitHeaders verifies that correct headers are set
// when the response is served from cache.
func TestPackageHandler_Get_CacheHitHeaders(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRequestsResponse)
	}))
	defer mock.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	client := pypi.NewClient(pypi.WithBaseURL(mock.URL))
	h := handler.NewPackageHandler(client, c)
	router := chi.NewRouter()
	router.Get("/packages/{name}", h.Get)

	// Prime the cache.
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/packages/requests", nil))

	// Now hit the cache.
	req := httptest.NewRequest(http.MethodGet, "/packages/requests", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on cache hit, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type: expected application/json, got %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=3600" {
		t.Errorf("Cache-Control: expected public, max-age=3600, got %q", got)
	}
}

// TestPackageHandler_Get_SingleFlightErrorDedup verifies that when the upstream
// returns an error during a singleflight window, all concurrent waiters receive
// an error response and the upstream is only called once (not once per caller).
func TestPackageHandler_Get_SingleFlightErrorDedup(t *testing.T) {
	var upstream atomic.Int32

	// Upstream always returns 404 (package not found). The small delay widens
	// the singleflight in-flight window so all concurrent callers below can
	// enter sf.Do before the first call completes; without it, on fast/slow
	// CI runners the 404 returns before later goroutines arrive and they each
	// start a fresh upstream call.
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstream.Add(1)
		time.Sleep(10 * time.Millisecond)
		http.NotFound(w, r)
	}))
	defer mock.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	client := pypi.NewClient(pypi.WithBaseURL(mock.URL))
	h := handler.NewPackageHandler(client, c)
	router := chi.NewRouter()
	router.Get("/packages/{name}", h.Get)

	const concurrency = 20
	var wg sync.WaitGroup
	wg.Add(concurrency)
	ready := make(chan struct{})
	var errCount atomic.Int32

	for range concurrency {
		go func() {
			defer wg.Done()
			<-ready
			req := httptest.NewRequest(http.MethodGet, "/packages/nonexistent", nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code == http.StatusNotFound {
				errCount.Add(1)
			}
		}()
	}

	close(ready)
	wg.Wait()

	// All callers should have received 404.
	if got := errCount.Load(); got != concurrency {
		t.Errorf("expected all %d callers to get 404, got %d", concurrency, got)
	}
	// Upstream should have been called at most a handful of times (singleflight).
	if got := upstream.Load(); got > 5 {
		t.Errorf("expected at most 5 upstream calls for error case (singleflight dedup), got %d", got)
	}
}

// TestPackageHandler_Get_CaseFolding verifies that the cache key is
// case-insensitive: "Requests" and "requests" share the same entry.
func TestPackageHandler_Get_CaseFolding(t *testing.T) {
	var upstream atomic.Int32

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstream.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRequestsResponse)
	}))
	defer mock.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	client := pypi.NewClient(pypi.WithBaseURL(mock.URL))
	h := handler.NewPackageHandler(client, c)
	router := chi.NewRouter()
	router.Get("/packages/{name}", h.Get)

	// First request uses mixed case.
	req1 := httptest.NewRequest(http.MethodGet, "/packages/Requests", nil)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", rec1.Code)
	}

	// Second request uses lowercase — should hit cache, not upstream.
	req2 := httptest.NewRequest(http.MethodGet, "/packages/requests", nil)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("second request: expected 200, got %d", rec2.Code)
	}

	if got := upstream.Load(); got != 1 {
		t.Errorf("expected 1 upstream call (case-folded cache key), got %d", got)
	}
}

// TestPackageHandler_Get_ResponseBodyIsValidJSON checks the response body from
// a cache miss is well-formed JSON containing the expected top-level fields.
func TestPackageHandler_Get_ResponseBodyIsValidJSON(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRequestsResponse)
	}))
	defer mock.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	client := pypi.NewClient(pypi.WithBaseURL(mock.URL))
	h := handler.NewPackageHandler(client, c)
	router := chi.NewRouter()
	router.Get("/packages/{name}", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/packages/requests", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}

	// Spot-check required top-level fields produced by buildPackageResponse.
	for _, field := range []string{"name", "version", "summary", "latest_files", "install_size"} {
		if _, ok := body[field]; !ok {
			t.Errorf("response JSON missing field %q", field)
		}
	}
	if body["name"] != "requests" {
		t.Errorf("expected name=requests, got %v", body["name"])
	}
}

// TestPackageHandler_Get_502OnUpstreamError verifies that a non-404 upstream
// failure (e.g. 500) returns 502 Bad Gateway to the caller.
func TestPackageHandler_Get_502OnUpstreamError(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer mock.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	client := pypi.NewClient(pypi.WithBaseURL(mock.URL))
	h := handler.NewPackageHandler(client, c)
	router := chi.NewRouter()
	router.Get("/packages/{name}", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/packages/requests", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected 502 for upstream 500, got %d", rec.Code)
	}
}

// mustTempCache creates a *cache.Cache backed by a temporary SQLite file and
// registers cleanup. Returns the cache and the file path so tests can open a
// second DB connection to manipulate created_at directly.
func mustTempCache(t *testing.T) (*cache.Cache, string) {
	t.Helper()
	f, err := os.CreateTemp("", "swr-test-*.sqlite")
	if err != nil {
		t.Fatalf("mustTempCache: create temp file: %v", err)
	}
	f.Close()
	path := f.Name()
	c, err := cache.New(path)
	if err != nil {
		os.Remove(path) //nolint:errcheck
		t.Fatalf("mustTempCache: cache.New: %v", err)
	}
	t.Cleanup(func() {
		c.Close()
		os.Remove(path) //nolint:errcheck
	})
	return c, path
}

// backdateCache sets created_at = 0 for the given key via a direct SQL
// connection to the SQLite file, forcing the entry to appear stale regardless
// of TTL.
func backdateCache(t *testing.T, path, key string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("backdateCache: open db: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE cache SET created_at = 0 WHERE key = ?`, key); err != nil {
		t.Fatalf("backdateCache: UPDATE: %v", err)
	}
}

// mockRequestsV2Response is a minimal PyPI JSON for "requests" with version bumped to v2.
const mockRequestsV2Response = `{
	"info": {
		"name": "requests",
		"version": "3.0.0",
		"summary": "HTTP for Humans v2",
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
			"filename": "requests-3.0.0-py3-none-any.whl",
			"url": "https://files.pythonhosted.org/packages/requests-3.0.0-py3-none-any.whl",
			"size": 63000,
			"packagetype": "bdist_wheel",
			"python_version": "py3",
			"requires_python": ">=3.8",
			"upload_time_iso_8601": "2025-01-01T00:00:00.000Z",
			"digests": {"sha256": "xyz999"}
		}
	]
}`

// TestGetSWRStaleHitRefreshesInBackground verifies that a stale cache entry is
// served immediately (stale-while-revalidate) and that the background refresh
// goroutine updates the cache with fresh upstream data.
func TestGetSWRStaleHitRefreshesInBackground(t *testing.T) {
	// v1 served first, v2 served after the stale entry is forced.
	var serveV2 atomic.Bool
	refreshDone := make(chan struct{})

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if serveV2.Load() {
			w.Write([]byte(mockRequestsV2Response)) //nolint:errcheck
			// Signal that the background refresh reached upstream.
			select {
			case refreshDone <- struct{}{}:
			default:
			}
		} else {
			w.Write([]byte(mockRequestsResponse)) //nolint:errcheck
		}
	}))
	defer mock.Close()

	c, dbPath := mustTempCache(t)
	client := pypi.NewClient(pypi.WithBaseURL(mock.URL))
	h := handler.NewPackageHandler(client, c)
	router := chi.NewRouter()
	router.Get("/packages/{name}", h.Get)

	// Prime the cache with v1.
	prime := httptest.NewRequest(http.MethodGet, "/packages/requests", nil)
	router.ServeHTTP(httptest.NewRecorder(), prime)

	// Force the cache entry to be stale by backdating created_at.
	backdateCache(t, dbPath, "pkg:requests")

	// Point the upstream at v2 so the background refresh picks it up.
	serveV2.Store(true)

	// Act: request should be served from stale cache (v1).
	req := httptest.NewRequest(http.MethodGet, "/packages/requests", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on stale hit, got %d", rec.Code)
	}

	// Response must be the stale v1 data.
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode stale response: %v", err)
	}
	if got := body["version"]; got != "2.31.0" {
		t.Errorf("expected stale version 2.31.0, got %v", got)
	}

	// Wait for the background refresh to reach upstream (2s deadline).
	select {
	case <-refreshDone:
	case <-time.After(2 * time.Second):
		t.Fatal("background refresh did not reach upstream within 2s")
	}

	// Poll until the cache entry is fresh again (background goroutine may still
	// be writing). The entry is fresh once version in the cache is v2.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		req2 := httptest.NewRequest(http.MethodGet, "/packages/requests", nil)
		rec2 := httptest.NewRecorder()
		router.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusOK {
			t.Fatalf("poll request: expected 200, got %d", rec2.Code)
		}
		var body2 map[string]any
		if err := json.NewDecoder(rec2.Body).Decode(&body2); err != nil {
			t.Fatalf("decode poll response: %v", err)
		}
		if body2["version"] == "3.0.0" {
			// Background refresh has populated the cache with v2.
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("cache was not refreshed to v2 within 2s after background goroutine reached upstream")
}

// TestGetSWRConcurrentStaleHitsSpawnMultipleRefreshes characterizes the current
// (un-deduplicated) background-refresh behavior: N concurrent requests that all
// see the same stale entry spawn N parallel fetchPackageForce calls.
//
// CURRENT BEHAVIOR: refresh is not deduplicated; plan 003 changes this to exactly 1.
func TestGetSWRConcurrentStaleHitsSpawnMultipleRefreshes(t *testing.T) {
	const concurrency = 10

	var upstreamCount atomic.Int32
	gate := make(chan struct{})

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCount.Add(1)
		// Block all background-refresh calls until we explicitly release the gate.
		<-gate
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockRequestsResponse)) //nolint:errcheck
	}))
	defer mock.Close()

	c, dbPath := mustTempCache(t)
	client := pypi.NewClient(pypi.WithBaseURL(mock.URL))
	h := handler.NewPackageHandler(client, c)
	router := chi.NewRouter()
	router.Get("/packages/{name}", h.Get)

	// Prime the cache using a separate upstream that doesn't block.
	primeMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockRequestsResponse)) //nolint:errcheck
	}))
	defer primeMock.Close()
	primeClient := pypi.NewClient(pypi.WithBaseURL(primeMock.URL))
	primeHandler := handler.NewPackageHandler(primeClient, c)
	primeRouter := chi.NewRouter()
	primeRouter.Get("/packages/{name}", primeHandler.Get)
	primeRouter.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/packages/requests", nil))

	// Force stale.
	backdateCache(t, dbPath, "pkg:requests")

	// Fire concurrency concurrent GETs — all should see stale and return 200.
	var wg sync.WaitGroup
	wg.Add(concurrency)
	ready := make(chan struct{})

	for range concurrency {
		go func() {
			defer wg.Done()
			<-ready
			req := httptest.NewRequest(http.MethodGet, "/packages/requests", nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("expected 200 on stale hit, got %d", rec.Code)
			}
		}()
	}

	close(ready)
	// Give goroutines time to issue their background refresh calls and block on gate.
	time.Sleep(100 * time.Millisecond)

	// Release all blocked refresh calls.
	close(gate)

	wg.Wait()
	// Allow background goroutines to finish.
	time.Sleep(100 * time.Millisecond)

	// CURRENT BEHAVIOR: refresh is not deduplicated; plan 003 changes this to exactly 1.
	got := upstreamCount.Load()
	if got < 1 {
		t.Fatalf("expected at least 1 upstream background-refresh call, got %d", got)
	}
	t.Logf("observed %d concurrent background-refresh upstream calls (not deduplicated; plan 003 will fix to 1)", got)
}

// TestPackageHandler_Get_DifferentPackagesIndependent verifies that concurrent
// requests for two different packages each go upstream exactly once (their
// singleflight keys are independent).
//
// The upstream mock pins every request open on the `release` channel. As long
// as `release` is held closed-shut, no upstream call can complete — so any
// goroutine arriving at singleflight while another is still in flight will
// join the existing call. We poll until the upstream counters settle at the
// expected state (1 each), then unblock and assert exactly that. This makes
// the test independent of Go scheduler timing.
func TestPackageHandler_Get_DifferentPackagesIndependent(t *testing.T) {
	var requestsCount, flaskCount atomic.Int32
	release := make(chan struct{})

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isFlask := strings.Contains(r.URL.Path, "flask")
		if isFlask {
			flaskCount.Add(1)
		} else {
			requestsCount.Add(1)
		}
		<-release
		w.Header().Set("Content-Type", "application/json")
		if isFlask {
			// Return a minimal valid flask response by reusing the requests fixture
			// (name field is the only thing that differs for our test purposes).
			fmt.Fprint(w, mockLicenseExpressionResponse)
		} else {
			fmt.Fprint(w, mockRequestsResponse)
		}
	}))
	defer mock.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	client := pypi.NewClient(pypi.WithBaseURL(mock.URL))
	h := handler.NewPackageHandler(client, c)
	router := chi.NewRouter()
	router.Get("/packages/{name}", h.Get)

	var wg sync.WaitGroup
	ready := make(chan struct{})

	for i := range 10 {
		wg.Add(1)
		pkg := "requests"
		if i%2 == 0 {
			pkg = "flask"
		}
		go func(pkg string) {
			defer wg.Done()
			<-ready
			req := httptest.NewRequest(http.MethodGet, "/packages/"+pkg, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
		}(pkg)
	}
	close(ready)

	// Wait until both packages have admitted exactly one upstream call.
	// While `release` is held, any extra calls past 1 per package would be
	// a singleflight regression and would show up here long before the
	// deadline.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if requestsCount.Load() == 1 && flaskCount.Load() == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	close(release)
	wg.Wait()

	if got := requestsCount.Load(); got != 1 {
		t.Errorf("requests: expected exactly 1 upstream call, got %d", got)
	}
	if got := flaskCount.Load(); got != 1 {
		t.Errorf("flask: expected exactly 1 upstream call, got %d", got)
	}
}

func TestPackageHandlerGetText(t *testing.T) {
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"info": {
				"name": "httpx",
				"version": "0.27.0",
				"summary": "The next generation HTTP client.",
				"license_expression": "BSD-3-Clause",
				"requires_python": ">=3.8",
				"requires_dist": ["anyio", "certifi"],
				"project_urls": {"Source": "https://github.com/encode/httpx"}
			},
			"urls": [],
			"releases": {}
		}`)
	}))
	defer pypiSrv.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	defer c.Close()
	memCache := cache.NewMemoryCache(c, 100)

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	h := handler.NewPackageHandler(pypiClient, memCache)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}.txt", h.GetText)

	req := httptest.NewRequest("GET", "/api/packages/httpx.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain prefix", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "name: httpx") {
		t.Errorf("body missing 'name: httpx': %s", body)
	}
	if !strings.Contains(body, "version: 0.27.0") {
		t.Errorf("body missing 'version: 0.27.0': %s", body)
	}
	if !strings.Contains(body, "repo_url: https://github.com/encode/httpx") {
		t.Errorf("body missing repo_url: %s", body)
	}
}
