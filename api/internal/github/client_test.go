package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---------------------------------------------------------------------------
// ExtractGitHubRepo tests
// ---------------------------------------------------------------------------

func TestExtractGitHubRepo_SourceKey(t *testing.T) {
	urls := map[string]string{"Source": "https://github.com/psf/requests"}
	owner, repo, ok := ExtractGitHubRepo(urls)
	if !ok || owner != "psf" || repo != "requests" {
		t.Fatalf("got (%q, %q, %v), want (psf, requests, true)", owner, repo, ok)
	}
}

func TestExtractGitHubRepo_RepositoryKey(t *testing.T) {
	urls := map[string]string{"Repository": "https://github.com/pallets/flask"}
	owner, repo, ok := ExtractGitHubRepo(urls)
	if !ok || owner != "pallets" || repo != "flask" {
		t.Fatalf("got (%q, %q, %v), want (pallets, flask, true)", owner, repo, ok)
	}
}

func TestExtractGitHubRepo_SourceCodeKey(t *testing.T) {
	urls := map[string]string{"Source Code": "https://github.com/encode/httpx"}
	owner, repo, ok := ExtractGitHubRepo(urls)
	if !ok || owner != "encode" || repo != "httpx" {
		t.Fatalf("got (%q, %q, %v), want (encode, httpx, true)", owner, repo, ok)
	}
}

func TestExtractGitHubRepo_CaseInsensitiveKey(t *testing.T) {
	urls := map[string]string{"SOURCE CODE": "https://github.com/encode/httpx"}
	owner, repo, ok := ExtractGitHubRepo(urls)
	if !ok || owner != "encode" || repo != "httpx" {
		t.Fatalf("got (%q, %q, %v), want (encode, httpx, true)", owner, repo, ok)
	}
}

func TestExtractGitHubRepo_TrailingSlash(t *testing.T) {
	urls := map[string]string{"Source": "https://github.com/psf/requests/"}
	owner, repo, ok := ExtractGitHubRepo(urls)
	if !ok || owner != "psf" || repo != "requests" {
		t.Fatalf("got (%q, %q, %v), want (psf, requests, true)", owner, repo, ok)
	}
}

func TestExtractGitHubRepo_WithTreePath(t *testing.T) {
	urls := map[string]string{"Source": "https://github.com/psf/requests/tree/main"}
	owner, repo, ok := ExtractGitHubRepo(urls)
	if !ok || owner != "psf" || repo != "requests" {
		t.Fatalf("got (%q, %q, %v), want (psf, requests, true)", owner, repo, ok)
	}
}

func TestExtractGitHubRepo_DotGitSuffix(t *testing.T) {
	urls := map[string]string{"Source": "https://github.com/psf/requests.git"}
	owner, repo, ok := ExtractGitHubRepo(urls)
	if !ok || owner != "psf" || repo != "requests" {
		t.Fatalf("got (%q, %q, %v), want (psf, requests, true)", owner, repo, ok)
	}
}

func TestExtractGitHubRepo_NoGitHubURL(t *testing.T) {
	urls := map[string]string{"Homepage": "https://www.example.com"}
	_, _, ok := ExtractGitHubRepo(urls)
	if ok {
		t.Fatal("expected ok=false for non-GitHub URL")
	}
}

func TestExtractGitHubRepo_NilURLs(t *testing.T) {
	_, _, ok := ExtractGitHubRepo(nil)
	if ok {
		t.Fatal("expected ok=false for nil map")
	}
}

func TestExtractGitHubRepo_GitLabURL(t *testing.T) {
	urls := map[string]string{"Source": "https://gitlab.com/user/repo"}
	_, _, ok := ExtractGitHubRepo(urls)
	if ok {
		t.Fatal("expected ok=false for GitLab URL")
	}
}

func TestExtractGitHubRepo_FallbackScan(t *testing.T) {
	// None of the priority keys; value should still be found via fallback.
	urls := map[string]string{"Tracker": "https://github.com/psf/requests/issues"}
	owner, repo, ok := ExtractGitHubRepo(urls)
	if !ok || owner != "psf" || repo != "requests" {
		t.Fatalf("got (%q, %q, %v), want (psf, requests, true)", owner, repo, ok)
	}
}

// ---------------------------------------------------------------------------
// FetchReleases tests
// ---------------------------------------------------------------------------

func TestFetchReleases(t *testing.T) {
	payload := []map[string]string{
		{
			"tag_name":     "v1.2.3",
			"name":         "Release 1.2.3",
			"body":         "Bug fixes.",
			"published_at": "2024-01-15T10:00:00Z",
			"html_url":     "https://github.com/psf/requests/releases/tag/v1.2.3",
		},
		{
			"tag_name":     "1.0.0",
			"name":         "First release",
			"body":         "Initial.",
			"published_at": "2023-06-01T08:00:00Z",
			"html_url":     "https://github.com/psf/requests/releases/tag/1.0.0",
		},
	}
	body, _ := json.Marshal(payload)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	releases, err := c.FetchReleases("psf", "requests")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(releases) != 2 {
		t.Fatalf("want 2 releases, got %d", len(releases))
	}

	// v-prefix stripped
	if releases[0].Version != "1.2.3" {
		t.Errorf("want Version=1.2.3, got %q", releases[0].Version)
	}
	if releases[0].TagName != "v1.2.3" {
		t.Errorf("want TagName=v1.2.3, got %q", releases[0].TagName)
	}
	if releases[0].Title != "Release 1.2.3" {
		t.Errorf("want Title=%q, got %q", "Release 1.2.3", releases[0].Title)
	}
	if releases[0].Body != "Bug fixes." {
		t.Errorf("want Body=%q, got %q", "Bug fixes.", releases[0].Body)
	}
	if releases[0].PublishedAt != "2024-01-15T10:00:00Z" {
		t.Errorf("unexpected PublishedAt: %q", releases[0].PublishedAt)
	}
	if releases[0].URL != "https://github.com/psf/requests/releases/tag/v1.2.3" {
		t.Errorf("unexpected URL: %q", releases[0].URL)
	}

	// No leading v to strip — version equals tag_name
	if releases[1].Version != "1.0.0" {
		t.Errorf("want Version=1.0.0, got %q", releases[1].Version)
	}
	if releases[1].TagName != "1.0.0" {
		t.Errorf("want TagName=1.0.0, got %q", releases[1].TagName)
	}
}

func TestFetchReleasesWithToken(t *testing.T) {
	var gotAuthHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithToken("ghp_testtoken"))
	_, err := c.FetchReleases("psf", "requests")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "Bearer ghp_testtoken"
	if gotAuthHeader != want {
		t.Errorf("want Authorization=%q, got %q", want, gotAuthHeader)
	}
}

func TestFetchReleasesNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	releases, err := c.FetchReleases("nobody", "nonexistent")
	if err != nil {
		t.Fatalf("expected no error on 404, got: %v", err)
	}
	if releases == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(releases) != 0 {
		t.Fatalf("expected 0 releases, got %d", len(releases))
	}
}

func TestFetchReleasesForbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	releases, err := c.FetchReleases("private", "repo")
	if err != nil {
		t.Fatalf("expected no error on 403, got: %v", err)
	}
	if len(releases) != 0 {
		t.Fatalf("expected 0 releases, got %d", len(releases))
	}
}

// ---------------------------------------------------------------------------
// FetchRepoInfo tests
// ---------------------------------------------------------------------------

func TestFetchRepoInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/psf/requests":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{
				"stargazers_count": 52000,
				"forks_count": 9500,
				"open_issues_count": 180,
				"pushed_at": "2025-03-01T10:00:00Z",
				"owner": {
					"login": "psf",
					"type": "Organization",
					"avatar_url": "https://avatars.githubusercontent.com/u/1"
				}
			}`)
		case "/orgs/psf":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"login": "psf", "name": "Python Software Foundation"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	info, err := c.FetchRepoInfo("psf", "requests")
	if err != nil {
		t.Fatalf("FetchRepoInfo() error: %v", err)
	}
	if info == nil {
		t.Fatal("FetchRepoInfo() returned nil, want non-nil")
	}
	if info.Stars != 52000 {
		t.Errorf("Stars = %d, want 52000", info.Stars)
	}
	if info.Owner.Login != "psf" {
		t.Errorf("Owner.Login = %q, want %q", info.Owner.Login, "psf")
	}
	if info.Owner.DisplayName != "Python Software Foundation" {
		t.Errorf("Owner.DisplayName = %q, want %q", info.Owner.DisplayName, "Python Software Foundation")
	}
	if !info.Owner.IsOrg {
		t.Error("Owner.IsOrg should be true for Organization type")
	}
}

func TestFetchRepoInfoNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	info, err := c.FetchRepoInfo("missing", "repo")
	if err != nil {
		t.Fatalf("FetchRepoInfo() on 404 should not error: %v", err)
	}
	if info != nil {
		t.Errorf("FetchRepoInfo() on 404 should return nil, got %+v", info)
	}
}

func TestFetchRepoInfoUserOwner(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/kennethreitz/requests":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{
				"stargazers_count": 1000,
				"forks_count": 200,
				"open_issues_count": 10,
				"pushed_at": "2024-01-01T00:00:00Z",
				"owner": {
					"login": "kennethreitz",
					"type": "User",
					"avatar_url": "https://avatars.githubusercontent.com/u/119893"
				}
			}`)
		case "/users/kennethreitz":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"login": "kennethreitz", "name": "Kenneth Reitz"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	info, err := c.FetchRepoInfo("kennethreitz", "requests")
	if err != nil {
		t.Fatalf("FetchRepoInfo() error: %v", err)
	}
	if info.Owner.IsOrg {
		t.Error("Owner.IsOrg should be false for User type")
	}
	if info.Owner.DisplayName != "Kenneth Reitz" {
		t.Errorf("DisplayName = %q, want Kenneth Reitz", info.Owner.DisplayName)
	}
}
