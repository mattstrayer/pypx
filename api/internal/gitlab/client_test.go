package gitlab_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/pypx/api/internal/circuitbreaker"
	"github.com/pypx/api/internal/gitlab"
)

func TestExtractGitLabRepo(t *testing.T) {
	tests := []struct {
		name        string
		projectURLs map[string]string
		wantPath    string
		wantOK      bool
	}{
		{
			name:        "source key with gitlab URL",
			projectURLs: map[string]string{"Source": "https://gitlab.com/user/myproject"},
			wantPath:    "user/myproject",
			wantOK:      true,
		},
		{
			name:        "repository key case insensitive",
			projectURLs: map[string]string{"repository": "https://gitlab.com/group/sub/project"},
			wantPath:    "group/sub/project",
			wantOK:      true,
		},
		{
			name:        "github URL not matched",
			projectURLs: map[string]string{"Source": "https://github.com/user/repo"},
			wantPath:    "",
			wantOK:      false,
		},
		{
			name:        "no URLs",
			projectURLs: map[string]string{},
			wantPath:    "",
			wantOK:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, _, ok := gitlab.ExtractGitLabRepo(tt.projectURLs)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if path != tt.wantPath {
				t.Errorf("path = %q, want %q", path, tt.wantPath)
			}
		})
	}
}

func TestGitLabClient_FetchReleases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawPath := r.URL.RawPath
		if rawPath == "" {
			rawPath = r.URL.Path
		}
		if rawPath == "/api/v4/projects/user%2Frepo/releases" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `[
				{"tag_name":"v1.0.0","name":"Release 1.0.0","description":"Some notes","released_at":"2024-01-01T00:00:00Z"}
			]`)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := gitlab.NewClient(gitlab.WithBaseURL(srv.URL))
	releases, err := c.FetchReleases(context.Background(), "user/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(releases) != 1 {
		t.Fatalf("expected 1 release, got %d", len(releases))
	}
	if releases[0].Version != "1.0.0" {
		t.Errorf("Version = %q, want 1.0.0", releases[0].Version)
	}
}

func TestGitLabClient_FetchRawFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawPath := r.URL.RawPath
		if rawPath == "" {
			rawPath = r.URL.Path
		}
		if rawPath == "/api/v4/projects/user%2Frepo/repository/files/CHANGELOG.md/raw" {
			fmt.Fprint(w, "## 1.0.0\ncontent")
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := gitlab.NewClient(gitlab.WithBaseURL(srv.URL))
	content, filename, err := c.FetchRawFile(context.Background(), "user/repo", []string{"CHANGELOG.md", "CHANGES.md"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filename != "CHANGELOG.md" {
		t.Errorf("filename = %q, want CHANGELOG.md", filename)
	}
	_ = content
}

func TestGitLabClient_FetchTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawPath := r.URL.RawPath
		if rawPath == "" {
			rawPath = r.URL.Path
		}
		if rawPath == "/api/v4/projects/user%2Frepo/repository/tags" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `[
				{"name":"v1.1.0","commit":{"id":"abc"}},
				{"name":"not-a-version","commit":{"id":"xyz"}},
				{"name":"v1.0.0","commit":{"id":"def"}}
			]`)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := gitlab.NewClient(gitlab.WithBaseURL(srv.URL))
	tags, err := c.FetchTags(context.Background(), "user/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 version tags, got %d", len(tags))
	}
}

func TestGitLabClient_FetchCompare(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawPath := r.URL.RawPath
		if rawPath == "" {
			rawPath = r.URL.Path
		}
		if rawPath == "/api/v4/projects/user%2Frepo/repository/compare" && r.URL.RawQuery == "from=v1.0.0&to=v1.1.0" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{
				"commits": [
					{"message":"feat: add feature\n\nBody","authored_date":"2024-01-15T00:00:00Z"},
					{"message":"Merge branch 'main'","authored_date":"2024-01-14T00:00:00Z"},
					{"message":"chore: update deps","authored_date":"2024-01-13T00:00:00Z"}
				]
			}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := gitlab.NewClient(gitlab.WithBaseURL(srv.URL))
	messages, headDate, err := c.FetchCompare(context.Background(), "user/repo", "v1.0.0", "v1.1.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0] != "feat: add feature" {
		t.Errorf("message = %q, want \"feat: add feature\"", messages[0])
	}
	if headDate != "2024-01-13T00:00:00Z" {
		t.Errorf("headDate = %q, want \"2024-01-13T00:00:00Z\"", headDate)
	}
}

// TestGitLabClient_BreakerOpensOn5xx verifies that after 5 consecutive 5xx
// responses the breaker trips open and subsequent calls fail fast without
// reaching the server.
func TestGitLabClient_BreakerOpensOn5xx(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := gitlab.NewClient(gitlab.WithBaseURL(srv.URL))

	for i := 0; i < 5; i++ {
		if _, err := c.FetchTags(context.Background(), "user/repo"); err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}
	if got := atomic.LoadInt32(&hits); got != 5 {
		t.Fatalf("expected 5 server hits after 5 failing calls, got %d", got)
	}

	// 6th call should fail fast without hitting the server — breaker is open.
	if _, err := c.FetchTags(context.Background(), "user/repo"); err == nil {
		t.Fatal("expected error once breaker is open, got nil")
	} else if !errors.Is(err, circuitbreaker.ErrOpen) {
		t.Errorf("expected circuitbreaker.ErrOpen, got %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 5 {
		t.Errorf("expected server hits to stay at 5 once breaker is open, got %d", got)
	}
}

// TestGitLabClient_BreakerNotTrippedBy404 verifies that repeated 404
// responses (repo doesn't exist) do NOT trip the breaker — only transport
// errors and 5xx responses should count as failures.
func TestGitLabClient_BreakerNotTrippedBy404(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := gitlab.NewClient(gitlab.WithBaseURL(srv.URL))

	for i := 0; i < 10; i++ {
		if _, err := c.FetchTags(context.Background(), "user/repo"); err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}
	if got := atomic.LoadInt32(&hits); got != 10 {
		t.Errorf("expected all 10 calls to reach the server (breaker should not open on 404s), got %d hits", got)
	}
}
