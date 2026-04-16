package handler_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	gh "github.com/pypx/api/internal/github"
	"github.com/pypx/api/internal/gitlab"
	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/pypi"
)

// mockCache is a controllable cache.Cacher implementation for testing.
type mockCache struct {
	// getFunc is called for each Get. If nil, returns nothing.
	getFunc func(key string, ttl time.Duration) ([]byte, bool, error)
	setFunc func(key string, value []byte, ttl time.Duration) error
}

func (m *mockCache) Get(key string, ttl time.Duration) ([]byte, bool, error) {
	if m.getFunc != nil {
		return m.getFunc(key, ttl)
	}
	return nil, false, errors.New("not found")
}

func (m *mockCache) Set(key string, value []byte, ttl time.Duration) error {
	if m.setFunc != nil {
		return m.setFunc(key, value, ttl)
	}
	return nil
}

func (m *mockCache) Close() error { return nil }

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

// TestChangelogGet_ServesStaleCache verifies that when the cache holds data
// that is no longer fresh, the handler serves it immediately with a short
// Cache-Control max-age instead of reaching out to upstream.
func TestChangelogGet_ServesStaleCache(t *testing.T) {
	stalePayload := []byte(`{"package":"testpkg","source":"github_releases","repo_url":"","entries":[]}`)

	mc := &mockCache{
		getFunc: func(key string, ttl time.Duration) ([]byte, bool, error) {
			// Return stale data (fresh=false) regardless of TTL argument.
			return stalePayload, false, nil
		},
	}

	// PyPI server should never be called; if it is, fail the test.
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected call to PyPI server — handler should have served stale cache")
		http.Error(w, "unexpected", http.StatusInternalServerError)
	}))
	defer pypiSrv.Close()

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	pkgHandler := handler.NewPackageHandler(pypiClient, mc)
	ghClient := gh.NewClient()
	glClient := gitlab.NewClient()
	changelogHandler := handler.NewChangelogHandler(ghClient, glClient, mc, pkgHandler)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/changelog", changelogHandler.Get)

	req := httptest.NewRequest("GET", "/api/packages/testpkg/changelog", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=60" {
		t.Errorf("Cache-Control = %q, want %q", got, "public, max-age=60")
	}
	if rec.Body.String() != string(stalePayload) {
		t.Errorf("body = %q, want %q", rec.Body.String(), string(stalePayload))
	}
}

// TestChangelogGet_FallbackOnUpstreamFailure verifies that when PyPI returns a
// non-404 error, the handler falls back to any cached data (TTL=0 query) and
// serves it rather than returning 502.
func TestChangelogGet_FallbackOnUpstreamFailure(t *testing.T) {
	cachedPayload := []byte(`{"package":"testpkg","source":"github_releases","repo_url":"","entries":[]}`)

	// First Get call (TTL=7d) returns nothing; second call (TTL=0) returns data.
	callCount := 0
	mc := &mockCache{
		getFunc: func(key string, ttl time.Duration) ([]byte, bool, error) {
			callCount++
			if ttl == 0 {
				// Emergency fallback query — return data.
				return cachedPayload, false, nil
			}
			// Normal TTL query — cache miss.
			return nil, false, errors.New("not found")
		},
	}

	// PyPI server returns 500 (non-404 error).
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer pypiSrv.Close()

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	pkgHandler := handler.NewPackageHandler(pypiClient, mc)
	ghClient := gh.NewClient()
	glClient := gitlab.NewClient()
	changelogHandler := handler.NewChangelogHandler(ghClient, glClient, mc, pkgHandler)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/changelog", changelogHandler.Get)

	req := httptest.NewRequest("GET", "/api/packages/testpkg/changelog", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != string(cachedPayload) {
		t.Errorf("body = %q, want %q", rec.Body.String(), string(cachedPayload))
	}
}

// TestChangelogGet_Returns502WhenNoCacheAndUpstreamFails verifies that when
// PyPI returns a non-404 error and there is no cached data at all, the handler
// returns 502 Bad Gateway.
func TestChangelogGet_Returns502WhenNoCacheAndUpstreamFails(t *testing.T) {
	mc := &mockCache{
		getFunc: func(key string, ttl time.Duration) ([]byte, bool, error) {
			// Cache always misses — no data at any TTL.
			return nil, false, errors.New("not found")
		},
	}

	// PyPI server returns 500 (non-404 error).
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer pypiSrv.Close()

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	pkgHandler := handler.NewPackageHandler(pypiClient, mc)
	ghClient := gh.NewClient()
	glClient := gitlab.NewClient()
	changelogHandler := handler.NewChangelogHandler(ghClient, glClient, mc, pkgHandler)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/changelog", changelogHandler.Get)

	req := httptest.NewRequest("GET", "/api/packages/testpkg/changelog", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body: %s", rec.Code, rec.Body.String())
	}
}
