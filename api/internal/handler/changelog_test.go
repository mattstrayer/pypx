package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	gh "github.com/pypx/api/internal/github"
	"github.com/pypx/api/internal/gitlab"
	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/pypi"
)

// pypiResponse builds a minimal PyPI JSON response with the given project URL.
func pypiResponse(sourceURL string) string {
	return `{"info":{"name":"testpkg","version":"1.0.0","summary":"","description":"",` +
		`"description_content_type":"","license":"","author":"","author_email":"",` +
		`"home_page":"","requires_python":"","requires_dist":null,` +
		`"project_urls":{"Source":"` + sourceURL + `"}},"releases":{},"urls":[]}`
}

func TestChangelogGet_GitHubReleases(t *testing.T) {
	ghSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/releases":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[{"tag_name":"v1.0.0","name":"Release 1.0","body":"## Changes\n- Fix foo","published_at":"2024-01-01T00:00:00Z","html_url":"https://github.com/owner/repo/releases/tag/v1.0.0"}]`))
		case "/repos/owner/repo":
			w.Write([]byte(`{"stargazers_count":10,"forks_count":2,"open_issues_count":1,"pushed_at":"2024-01-01T00:00:00Z","owner":{"login":"owner","type":"User","avatar_url":""}}`))
		case "/users/owner":
			w.Write([]byte(`{"name":"Owner Name"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ghSrv.Close()

	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(pypiResponse("https://github.com/owner/repo")))
	}))
	defer pypiSrv.Close()

	sqliteCache, _ := cache.New(":memory:")
	c := cache.NewMemoryCache(sqliteCache, 100)
	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	pkgHandler := handler.NewPackageHandler(pypiClient, c)
	ghClient := gh.NewClient(gh.WithBaseURL(ghSrv.URL))
	glClient := gitlab.NewClient()
	changelogHandler := handler.NewChangelogHandler(ghClient, glClient, c, pkgHandler)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/changelog", changelogHandler.Get)

	req := httptest.NewRequest("GET", "/api/packages/testpkg/changelog", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Source  string `json:"source"`
		Entries []struct {
			Version string `json:"version"`
		} `json:"entries"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.Source != "github_releases" {
		t.Errorf("source = %q, want github_releases", resp.Source)
	}
	if len(resp.Entries) == 0 {
		t.Error("expected at least one entry")
	}
}

func TestChangelogGet_FallsBackToChangelogFile(t *testing.T) {
	ghSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/releases":
			w.Write([]byte(`[]`))
		case "/repos/owner/repo/contents/CHANGELOG.md":
			w.Write([]byte("## [1.0.0] - 2024-01-01\nFixed stuff.\n\n## [0.9.0] - 2023-12-01\nBeta."))
		case "/repos/owner/repo/tags":
			w.Write([]byte(`[]`))
		case "/repos/owner/repo":
			w.Write([]byte(`{"stargazers_count":5,"forks_count":1,"open_issues_count":0,"pushed_at":"2024-01-01T00:00:00Z","owner":{"login":"owner","type":"User","avatar_url":""}}`))
		case "/users/owner":
			w.Write([]byte(`{"name":"Owner"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ghSrv.Close()

	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(pypiResponse("https://github.com/owner/repo")))
	}))
	defer pypiSrv.Close()

	sqliteCache, _ := cache.New(":memory:")
	c := cache.NewMemoryCache(sqliteCache, 100)
	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	pkgHandler := handler.NewPackageHandler(pypiClient, c)
	ghClient := gh.NewClient(gh.WithBaseURL(ghSrv.URL))
	glClient := gitlab.NewClient()
	changelogHandler := handler.NewChangelogHandler(ghClient, glClient, c, pkgHandler)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/changelog", changelogHandler.Get)

	req := httptest.NewRequest("GET", "/api/packages/testpkg/changelog", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Source  string `json:"source"`
		Entries []struct{ Version string `json:"version"` } `json:"entries"`
	}
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	if resp.Source != "github_changelog_file" {
		t.Errorf("source = %q, want github_changelog_file", resp.Source)
	}
	if len(resp.Entries) == 0 {
		t.Error("expected entries from CHANGELOG file")
	}
}

func TestChangelogGet_FallsBackToTags(t *testing.T) {
	ghSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/releases":
			w.Write([]byte(`[]`))
		case "/repos/owner/repo/tags":
			w.Write([]byte(`[{"name":"v1.1.0","commit":{"sha":"abc"}},{"name":"v1.0.0","commit":{"sha":"def"}}]`))
		case "/repos/owner/repo/compare/v1.0.0...v1.1.0":
			w.Write([]byte(`{"commits":[{"commit":{"message":"Add feature","author":{"date":"2024-03-01T00:00:00Z"}}}]}`))
		case "/repos/owner/repo":
			w.Write([]byte(`{"stargazers_count":0,"forks_count":0,"open_issues_count":0,"pushed_at":"2024-01-01T00:00:00Z","owner":{"login":"owner","type":"User","avatar_url":""}}`))
		case "/users/owner":
			w.Write([]byte(`{"name":"Owner"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ghSrv.Close()

	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(pypiResponse("https://github.com/owner/repo")))
	}))
	defer pypiSrv.Close()

	sqliteCache, _ := cache.New(":memory:")
	c := cache.NewMemoryCache(sqliteCache, 100)
	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	pkgHandler := handler.NewPackageHandler(pypiClient, c)
	ghClient := gh.NewClient(gh.WithBaseURL(ghSrv.URL))
	glClient := gitlab.NewClient()
	changelogHandler := handler.NewChangelogHandler(ghClient, glClient, c, pkgHandler)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/changelog", changelogHandler.Get)

	req := httptest.NewRequest("GET", "/api/packages/testpkg/changelog", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp struct {
		Source  string `json:"source"`
		Entries []struct{ Version string `json:"version"` } `json:"entries"`
	}
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	if resp.Source != "github_tags" {
		t.Errorf("source = %q, want github_tags", resp.Source)
	}
}

func TestChangelogGet_NoRepoURL_ReturnsEmpty(t *testing.T) {
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"info":{"name":"testpkg","version":"1.0.0","summary":"","description":"","description_content_type":"","license":"","author":"","author_email":"","home_page":"","requires_python":"","requires_dist":null,"project_urls":{}},"releases":{},"urls":[]}`))
	}))
	defer pypiSrv.Close()

	sqliteCache, _ := cache.New(":memory:")
	c := cache.NewMemoryCache(sqliteCache, 100)
	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	pkgHandler := handler.NewPackageHandler(pypiClient, c)
	ghClient := gh.NewClient()
	glClient := gitlab.NewClient()
	changelogHandler := handler.NewChangelogHandler(ghClient, glClient, c, pkgHandler)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/changelog", changelogHandler.Get)

	req := httptest.NewRequest("GET", "/api/packages/testpkg/changelog", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp struct {
		Entries []struct{} `json:"entries"`
		Source  string     `json:"source"`
	}
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	if len(resp.Entries) != 0 {
		t.Errorf("expected empty entries for package without repo URL")
	}
}
