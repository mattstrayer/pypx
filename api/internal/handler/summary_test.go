package handler_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/osv"
	"github.com/pypx/api/internal/pypi"
)

// mockHTTPXResponse is a minimal PyPI JSON response for "httpx".
const mockHTTPXResponse = `{
	"info": {
		"name": "httpx",
		"version": "0.27.0",
		"summary": "The next generation HTTP client.",
		"description": "",
		"description_content_type": "",
		"license": "BSD-3-Clause",
		"author": "Tom Christie",
		"author_email": "",
		"home_page": "https://www.python-httpx.org",
		"requires_python": ">=3.8",
		"requires_dist": null,
		"project_urls": null,
		"classifiers": null
	},
	"releases": {},
	"urls": []
}`

func TestSummaryHandlerGet(t *testing.T) {
	// Mock PyPI server.
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockHTTPXResponse)
	}))
	defer pypiSrv.Close()

	// Mock OSV server returning one vuln.
	osvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"vulns":[{"id":"GHSA-test","summary":"Test vuln","severity":[{"type":"CVSS_V3","score":"HIGH"}],"references":[{"url":"https://example.com"}]}]}`)
	}))
	defer osvSrv.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer c.Close()
	memCache := cache.NewMemoryCache(c, 100)

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	pkgHandler := handler.NewPackageHandler(pypiClient, memCache)
	osvClient := osv.NewClient(osv.WithBaseURL(osvSrv.URL))

	h := handler.NewSummaryHandler(pkgHandler, osvClient)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/summary.txt", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/packages/httpx/summary.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain*", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "name: httpx") {
		t.Errorf("body does not contain 'name: httpx'; body:\n%s", body)
	}
	if !strings.Contains(body, "## security") {
		t.Errorf("body does not contain '## security'; body:\n%s", body)
	}
}

func TestSummaryHandlerOSVError(t *testing.T) {
	// Mock PyPI server.
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockHTTPXResponse)
	}))
	defer pypiSrv.Close()

	// Mock OSV server that returns an error.
	osvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer osvSrv.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer c.Close()
	memCache := cache.NewMemoryCache(c, 100)

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	pkgHandler := handler.NewPackageHandler(pypiClient, memCache)
	osvClient := osv.NewClient(osv.WithBaseURL(osvSrv.URL))

	h := handler.NewSummaryHandler(pkgHandler, osvClient)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/summary.txt", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/packages/httpx/summary.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should still succeed — security section omitted on OSV error.
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	if !strings.Contains(body, "name: httpx") {
		t.Errorf("body does not contain 'name: httpx'; body:\n%s", body)
	}
	// Security section should be absent when OSV errored.
	if strings.Contains(body, "## security") {
		t.Errorf("body should not contain '## security' when OSV errored; body:\n%s", body)
	}
}

func TestSummaryHandlerInvalidName(t *testing.T) {
	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer c.Close()
	memCache := cache.NewMemoryCache(c, 100)

	pypiClient := pypi.NewClient()
	pkgHandler := handler.NewPackageHandler(pypiClient, memCache)
	osvClient := osv.NewClient()

	h := handler.NewSummaryHandler(pkgHandler, osvClient)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/summary.txt", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/packages/../etc/summary.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Error("expected non-200 for invalid package name")
	}
}
