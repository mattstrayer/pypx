package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

	// Mock GitHub server returning 1 release or repo info.
	mockGitHub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		path := r.URL.Path
		if strings.Contains(path, "/releases") {
			w.Write([]byte(mockGitHubReleases)) //nolint:errcheck
		} else if strings.HasPrefix(path, "/repos/") {
			w.Write([]byte(`{
				"stargazers_count": 1000,
				"forks_count": 100,
				"open_issues_count": 10,
				"pushed_at": "2025-01-01T00:00:00Z",
				"owner": {
					"login": "psf",
					"type": "Organization",
					"avatar_url": "https://avatars.githubusercontent.com/u/1"
				}
			}`)) //nolint:errcheck
		} else if strings.HasPrefix(path, "/orgs/") {
			w.Write([]byte(`{"name": "Python Software Foundation"}`)) //nolint:errcheck
		}
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

func TestChangelogGet_RendersBodyHTML(t *testing.T) {
	// Mock PyPI server returning a package with a GitHub URL.
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockRequestsWithGitHub)) //nolint:errcheck
	}))
	defer pypiSrv.Close()

	// Mock GitHub server returning a release with markdown body or repo info.
	ghSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		path := r.URL.Path
		if strings.Contains(path, "/releases") {
			w.Write([]byte(`[{
				"tag_name": "v2.31.0",
				"name": "v2.31.0",
				"body": "## What's Changed\n\n- Fixed **bug** in auth",
				"published_at": "2023-05-22T15:12:01Z",
				"html_url": "https://github.com/psf/requests/releases/tag/v2.31.0"
			}]`)) //nolint:errcheck
		} else if strings.HasPrefix(path, "/repos/") {
			w.Write([]byte(`{
				"stargazers_count": 1000,
				"forks_count": 100,
				"open_issues_count": 10,
				"pushed_at": "2025-01-01T00:00:00Z",
				"owner": {
					"login": "psf",
					"type": "Organization",
					"avatar_url": "https://avatars.githubusercontent.com/u/1"
				}
			}`)) //nolint:errcheck
		} else if strings.HasPrefix(path, "/orgs/") {
			w.Write([]byte(`{"name": "Python Software Foundation"}`)) //nolint:errcheck
		}
	}))
	defer ghSrv.Close()

	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer c.Close()
	mc := cache.NewMemoryCache(c, 100)

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	ghClient := gh.NewClient(gh.WithBaseURL(ghSrv.URL))
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

	if len(body.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(body.Entries))
	}

	entry := body.Entries[0]
	if entry.BodyHTML == "" {
		t.Fatal("expected body_html to be non-empty")
	}
	if !strings.Contains(entry.BodyHTML, "<strong>bug</strong>") {
		t.Errorf("expected body_html to contain <strong>bug</strong>, got %q", entry.BodyHTML)
	}
}
