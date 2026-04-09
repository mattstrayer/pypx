package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	gh "github.com/pypx/api/internal/github"
	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/pypi"
)

const mockRequestsWithGitHub = `{
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
		"project_urls": {
			"Source": "https://github.com/psf/requests"
		},
		"classifiers": null
	},
	"releases": {},
	"urls": []
}`

const mockRequestsNoGitHub = `{
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
		"project_urls": {
			"Homepage": "https://example.com"
		},
		"classifiers": null
	},
	"releases": {},
	"urls": []
}`

const mockGitHubReleases = `[
	{
		"tag_name": "v2.31.0",
		"name": "v2.31.0",
		"body": "Release notes",
		"published_at": "2023-05-22T15:12:01Z",
		"html_url": "https://github.com/psf/requests/releases/tag/v2.31.0"
	}
]`

func setupChangelogRouter(h *handler.ChangelogHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/api/packages/{name}/changelog", h.Get)
	return r
}

func TestGetChangelog(t *testing.T) {
	// Mock PyPI server returning a package with a GitHub URL.
	mockPyPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockRequestsWithGitHub)) //nolint:errcheck
	}))
	defer mockPyPI.Close()

	// Mock GitHub server returning 1 release.
	mockGitHub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockGitHubReleases)) //nolint:errcheck
	}))
	defer mockGitHub.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer c.Close()
	mc := cache.NewMemoryCache(c, 100)

	pypiClient := pypi.NewClient(pypi.WithBaseURL(mockPyPI.URL))
	ghClient := gh.NewClient(gh.WithBaseURL(mockGitHub.URL))
	pkgHandler := handler.NewPackageHandler(pypiClient, mc)
	changelogHandler := handler.NewChangelogHandler(ghClient, mc, pkgHandler)

	router := setupChangelogRouter(changelogHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/packages/requests/changelog", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body handler.ChangelogResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body.Package != "requests" {
		t.Errorf("expected package=requests, got %q", body.Package)
	}
	if body.Source != "github_releases" {
		t.Errorf("expected source=github_releases, got %q", body.Source)
	}
	if len(body.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(body.Entries))
	}
	if body.Entries[0].Version != "2.31.0" {
		t.Errorf("expected version=2.31.0, got %q", body.Entries[0].Version)
	}
}

func TestGetChangelogNoGitHub(t *testing.T) {
	// Mock PyPI server returning a package with no GitHub URL.
	mockPyPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockRequestsNoGitHub)) //nolint:errcheck
	}))
	defer mockPyPI.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer c.Close()
	mc := cache.NewMemoryCache(c, 100)

	pypiClient := pypi.NewClient(pypi.WithBaseURL(mockPyPI.URL))
	// No GitHub server needed — we never call it when there's no GitHub URL.
	ghClient := gh.NewClient(gh.WithBaseURL("http://127.0.0.1:0"))
	pkgHandler := handler.NewPackageHandler(pypiClient, mc)
	changelogHandler := handler.NewChangelogHandler(ghClient, mc, pkgHandler)

	router := setupChangelogRouter(changelogHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/packages/requests/changelog", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body handler.ChangelogResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(body.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(body.Entries))
	}
}
