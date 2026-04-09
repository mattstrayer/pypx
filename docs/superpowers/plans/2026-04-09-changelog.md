# Changelog Feature Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show GitHub Releases changelogs inline in the versions timeline and version detail pages.

**Architecture:** New Go `internal/github` package fetches releases from GitHub API, new `/api/packages/{name}/changelog` endpoint caches them for 7 days. Frontend fetches changelog in parallel with versions, merges by version string, renders expandable markdown rows. `@nuxtjs/mdc` handles markdown rendering across changelog and package descriptions.

**Tech Stack:** Go (net/http, existing cache), Nuxt 4 (@nuxtjs/mdc), existing project patterns

**Spec:** `docs/superpowers/specs/2026-04-09-changelog-design.md`

---

## File Structure

### Go API

```
api/internal/github/
├── client.go          # GitHub Releases API client + repo URL extraction
└── client_test.go
api/internal/handler/
├── changelog.go       # Changelog endpoint
└── changelog_test.go
api/cmd/server/main.go # Wire new endpoint
```

### Nuxt Frontend

```
web/app/
├── types/api.ts              # Add ChangelogEntry, ChangelogData
├── composables/useApi.ts     # Add fetchChangelog
├── components/
│   ├── PackageVersions.vue   # Expandable changelog rows
│   └── PackageOverview.vue   # MDC for descriptions
├── pages/packages/[name]/
│   └── [version].vue         # Inline changelog on version page
└── nuxt.config.ts            # Add @nuxtjs/mdc module
```

---

## Phase 1: Go API

### Task 1: GitHub Client — Repo URL Extraction

**Files:**
- Create: `api/internal/github/client.go`
- Create: `api/internal/github/client_test.go`

- [ ] **Step 1: Write repo extraction tests**

Create `api/internal/github/client_test.go`:

```go
package github_test

import (
	"testing"

	"github.com/pypx/api/internal/github"
)

func TestExtractGitHubRepo(t *testing.T) {
	tests := []struct {
		name      string
		urls      map[string]string
		wantOwner string
		wantRepo  string
		wantOK    bool
	}{
		{
			name:      "source key",
			urls:      map[string]string{"Source": "https://github.com/psf/requests"},
			wantOwner: "psf", wantRepo: "requests", wantOK: true,
		},
		{
			name:      "repository key",
			urls:      map[string]string{"Repository": "https://github.com/pallets/flask"},
			wantOwner: "pallets", wantRepo: "flask", wantOK: true,
		},
		{
			name:      "source code key",
			urls:      map[string]string{"Source Code": "https://github.com/django/django"},
			wantOwner: "django", wantRepo: "django", wantOK: true,
		},
		{
			name:      "case insensitive",
			urls:      map[string]string{"source": "https://github.com/psf/requests"},
			wantOwner: "psf", wantRepo: "requests", wantOK: true,
		},
		{
			name:      "trailing slash",
			urls:      map[string]string{"Source": "https://github.com/psf/requests/"},
			wantOwner: "psf", wantRepo: "requests", wantOK: true,
		},
		{
			name:      "with tree path",
			urls:      map[string]string{"Source": "https://github.com/psf/requests/tree/main"},
			wantOwner: "psf", wantRepo: "requests", wantOK: true,
		},
		{
			name:   "no github url",
			urls:   map[string]string{"Homepage": "https://requests.readthedocs.io"},
			wantOK: false,
		},
		{
			name:   "nil urls",
			urls:   nil,
			wantOK: false,
		},
		{
			name:   "gitlab url",
			urls:   map[string]string{"Source": "https://gitlab.com/foo/bar"},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, ok := github.ExtractGitHubRepo(tt.urls)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && (owner != tt.wantOwner || repo != tt.wantRepo) {
				t.Errorf("got %s/%s, want %s/%s", owner, repo, tt.wantOwner, tt.wantRepo)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/matt/dev/pypx/api && go test ./internal/github/...
```

Expected: compilation error.

- [ ] **Step 3: Implement repo extraction and client skeleton**

Create `api/internal/github/client.go`:

```go
package github

import (
	"net/http"
	"regexp"
	"strings"
	"time"
)

var githubRepoPattern = regexp.MustCompile(`github\.com/([a-zA-Z0-9_.-]+)/([a-zA-Z0-9_.-]+)`)

// ExtractGitHubRepo extracts owner and repo from a PyPI project_urls map.
// Scans known keys (case-insensitive) for GitHub URLs.
func ExtractGitHubRepo(projectURLs map[string]string) (owner, repo string, ok bool) {
	if projectURLs == nil {
		return "", "", false
	}

	// Priority order for key matching
	priorities := []string{"source", "repository", "source code", "github", "homepage", "code"}

	for _, priority := range priorities {
		for key, url := range projectURLs {
			if strings.EqualFold(strings.TrimSpace(key), priority) {
				if match := githubRepoPattern.FindStringSubmatch(url); len(match) == 3 {
					return match[1], strings.TrimSuffix(match[2], ".git"), true
				}
			}
		}
	}

	// Fallback: scan all values for any GitHub URL
	for _, url := range projectURLs {
		if match := githubRepoPattern.FindStringSubmatch(url); len(match) == 3 {
			return match[1], strings.TrimSuffix(match[2], ".git"), true
		}
	}

	return "", "", false
}

type Release struct {
	Version     string `json:"version"`
	TagName     string `json:"tag_name"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
	URL         string `json:"url"`
}

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type Option func(*Client)

func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = url }
}

func WithToken(token string) Option {
	return func(c *Client) { c.token = token }
}

func NewClient(opts ...Option) *Client {
	c := &Client{
		baseURL: "https://api.github.com",
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/matt/dev/pypx/api && go test ./internal/github/... -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/matt/dev/pypx
git add api/internal/github/
git commit -m "feat(api): add GitHub repo URL extraction from PyPI project_urls"
```

---

### Task 2: GitHub Client — Fetch Releases

**Files:**
- Modify: `api/internal/github/client.go`
- Modify: `api/internal/github/client_test.go`

- [ ] **Step 1: Write fetch releases test**

Add to `api/internal/github/client_test.go`:

```go
import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pypx/api/internal/github"
)

func TestFetchReleases(t *testing.T) {
	mockReleases := []map[string]any{
		{
			"tag_name":     "v2.31.0",
			"name":         "v2.31.0",
			"body":         "## Changes\n- Fixed bug",
			"published_at": "2023-05-22T15:00:00Z",
			"html_url":     "https://github.com/psf/requests/releases/tag/v2.31.0",
		},
		{
			"tag_name":     "v2.30.0",
			"name":         "Release 2.30.0",
			"body":         "## What's new\n- Added feature",
			"published_at": "2023-05-03T12:00:00Z",
			"html_url":     "https://github.com/psf/requests/releases/tag/v2.30.0",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/psf/requests/releases" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Query().Get("per_page") != "100" {
			t.Error("expected per_page=100")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockReleases)
	}))
	defer server.Close()

	client := github.NewClient(github.WithBaseURL(server.URL))
	releases, err := client.FetchReleases("psf", "requests")
	if err != nil {
		t.Fatalf("failed to fetch: %v", err)
	}
	if len(releases) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(releases))
	}
	if releases[0].Version != "2.31.0" {
		t.Errorf("expected version '2.31.0', got '%s'", releases[0].Version)
	}
	if releases[0].TagName != "v2.31.0" {
		t.Errorf("expected tag 'v2.31.0', got '%s'", releases[0].TagName)
	}
	if releases[0].Body != "## Changes\n- Fixed bug" {
		t.Errorf("unexpected body: %s", releases[0].Body)
	}
	if releases[1].Version != "2.30.0" {
		t.Errorf("expected version '2.30.0', got '%s'", releases[1].Version)
	}
}

func TestFetchReleasesWithToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token-123" {
			t.Errorf("expected Bearer token, got '%s'", auth)
		}
		json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer server.Close()

	client := github.NewClient(github.WithBaseURL(server.URL), github.WithToken("test-token-123"))
	releases, err := client.FetchReleases("psf", "requests")
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if len(releases) != 0 {
		t.Errorf("expected 0 releases, got %d", len(releases))
	}
}

func TestFetchReleasesNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := github.NewClient(github.WithBaseURL(server.URL))
	releases, err := client.FetchReleases("nonexistent", "repo")
	if err != nil {
		t.Fatalf("should not error on 404: %v", err)
	}
	if len(releases) != 0 {
		t.Errorf("expected 0 releases for 404, got %d", len(releases))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/matt/dev/pypx/api && go test ./internal/github/... -run TestFetchReleases
```

Expected: compilation error — `FetchReleases` not defined.

- [ ] **Step 3: Implement FetchReleases**

Add to `api/internal/github/client.go`:

```go
import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type ghRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
	HTMLURL     string `json:"html_url"`
}

// FetchReleases fetches GitHub releases for the given owner/repo.
// Returns empty slice (not error) if repo not found or has no releases.
func (c *Client) FetchReleases(owner, repo string) ([]Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=100", c.baseURL, owner, repo)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching releases for %s/%s: %w", owner, repo, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		return []Release{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for %s/%s releases", resp.StatusCode, owner, repo)
	}

	var ghReleases []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&ghReleases); err != nil {
		return nil, fmt.Errorf("decoding releases for %s/%s: %w", owner, repo, err)
	}

	releases := make([]Release, 0, len(ghReleases))
	for _, r := range ghReleases {
		releases = append(releases, Release{
			Version:     stripLeadingV(r.TagName),
			TagName:     r.TagName,
			Title:       r.Name,
			Body:        r.Body,
			PublishedAt: r.PublishedAt,
			URL:         r.HTMLURL,
		})
	}

	return releases, nil
}

func stripLeadingV(tag string) string {
	if strings.HasPrefix(tag, "v") {
		return tag[1:]
	}
	return tag
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/matt/dev/pypx/api && go test ./internal/github/... -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/matt/dev/pypx
git add api/internal/github/
git commit -m "feat(api): add GitHub Releases client with token support"
```

---

### Task 3: Changelog Endpoint

**Files:**
- Create: `api/internal/handler/changelog.go`
- Create: `api/internal/handler/changelog_test.go`
- Modify: `api/cmd/server/main.go`

- [ ] **Step 1: Write changelog handler test**

Create `api/internal/handler/changelog_test.go`:

```go
package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/github"
	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/pypi"
)

func TestGetChangelog(t *testing.T) {
	// Mock PyPI — returns package with GitHub source URL
	mockPyPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := pypi.PyPIResponse{
			Info: pypi.PackageInfo{
				Name:    "requests",
				Version: "2.31.0",
				ProjectURLs: map[string]string{
					"Source": "https://github.com/psf/requests",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockPyPI.Close()

	// Mock GitHub — returns releases
	mockGitHub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		releases := []map[string]any{
			{
				"tag_name":     "v2.31.0",
				"name":         "v2.31.0",
				"body":         "## Changes\n- Fixed bug",
				"published_at": "2023-05-22T15:00:00Z",
				"html_url":     "https://github.com/psf/requests/releases/tag/v2.31.0",
			},
		}
		json.NewEncoder(w).Encode(releases)
	}))
	defer mockGitHub.Close()

	c, _ := cache.New(":memory:")
	defer c.Close()
	mc := cache.NewMemoryCache(c, 100)

	pypiClient := pypi.NewClient(pypi.WithBaseURL(mockPyPI.URL))
	ghClient := github.NewClient(github.WithBaseURL(mockGitHub.URL))
	pkgHandler := handler.NewPackageHandler(pypiClient, mc)
	chHandler := handler.NewChangelogHandler(ghClient, mc, pkgHandler)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/changelog", chHandler.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/packages/requests/changelog", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["package"] != "requests" {
		t.Errorf("expected package 'requests', got '%v'", result["package"])
	}
	if result["source"] != "github_releases" {
		t.Errorf("expected source 'github_releases', got '%v'", result["source"])
	}
	entries := result["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	entry := entries[0].(map[string]any)
	if entry["version"] != "2.31.0" {
		t.Errorf("expected version '2.31.0', got '%v'", entry["version"])
	}
}

func TestGetChangelogNoGitHub(t *testing.T) {
	// Package with no GitHub URL
	mockPyPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := pypi.PyPIResponse{
			Info: pypi.PackageInfo{
				Name:        "somepkg",
				ProjectURLs: map[string]string{"Homepage": "https://example.com"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockPyPI.Close()

	c, _ := cache.New(":memory:")
	defer c.Close()
	mc := cache.NewMemoryCache(c, 100)

	pypiClient := pypi.NewClient(pypi.WithBaseURL(mockPyPI.URL))
	ghClient := github.NewClient()
	pkgHandler := handler.NewPackageHandler(pypiClient, mc)
	chHandler := handler.NewChangelogHandler(ghClient, mc, pkgHandler)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/changelog", chHandler.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/packages/somepkg/changelog", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	entries := result["entries"].([]any)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/matt/dev/pypx/api && go test ./internal/handler/... -run TestGetChangelog
```

Expected: compilation error.

- [ ] **Step 3: Implement changelog handler**

Create `api/internal/handler/changelog.go`:

```go
package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	gh "github.com/pypx/api/internal/github"
	"github.com/pypx/api/internal/pypi"
)

const changelogTTL = 7 * 24 * time.Hour // 7 days

type ChangelogHandler struct {
	github *gh.Client
	cache  cache.Cacher
	pkg    *PackageHandler
}

func NewChangelogHandler(ghClient *gh.Client, c cache.Cacher, pkgHandler *PackageHandler) *ChangelogHandler {
	return &ChangelogHandler{github: ghClient, cache: c, pkg: pkgHandler}
}

type ChangelogResponse struct {
	Package string       `json:"package"`
	Source  string       `json:"source"`
	RepoURL string      `json:"repo_url"`
	Entries []gh.Release `json:"entries"`
}

func (h *ChangelogHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := pypi.ValidateName(name); err != nil {
		http.Error(w, `{"error":"invalid package name"}`, http.StatusBadRequest)
		return
	}

	cacheKey := "changelog:" + strings.ToLower(name)

	// Check cache
	data, fresh, _ := h.cache.Get(cacheKey, changelogTTL)
	if data != nil && fresh {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=604800")
		w.Write(data)
		return
	}

	// Fetch PyPI package to get project URLs
	pkg, err := h.pkg.FetchPackage(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, `{"error":"package not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	resp := ChangelogResponse{
		Package: name,
		Source:  "github_releases",
		Entries: []gh.Release{},
	}

	// Extract GitHub repo
	owner, repo, ok := gh.ExtractGitHubRepo(pkg.Info.ProjectURLs)
	if ok {
		resp.RepoURL = "https://github.com/" + owner + "/" + repo
		releases, err := h.github.FetchReleases(owner, repo)
		if err == nil && len(releases) > 0 {
			resp.Entries = releases
		}
	}

	respJSON, _ := json.Marshal(resp)
	h.cache.Set(cacheKey, respJSON, changelogTTL)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=604800")
	w.Write(respJSON)
}
```

- [ ] **Step 4: Export FetchPackage on PackageHandler**

In `api/internal/handler/packages.go`, the `fetchPackage` method is lowercase (unexported). The changelog handler needs to call it. Add an exported wrapper:

```go
// FetchPackage is the exported version of fetchPackage for use by other handlers.
func (h *PackageHandler) FetchPackage(name string) (*pypi.PyPIResponse, error) {
	return h.fetchPackage(name)
}
```

- [ ] **Step 5: Wire into main.go**

Add to `api/cmd/server/main.go`:

```go
	ghToken := os.Getenv("GITHUB_TOKEN")
	var ghOpts []github.Option
	if ghToken != "" {
		ghOpts = append(ghOpts, github.WithToken(ghToken))
	}
	ghClient := github.NewClient(ghOpts...)
	changelogHandler := handler.NewChangelogHandler(ghClient, c, pkgHandler)
```

Add route:

```go
	r.Get("/api/packages/{name}/changelog", changelogHandler.Get)
```

Add import: `"github.com/pypx/api/internal/github"`

- [ ] **Step 6: Run all tests**

```bash
cd /Users/matt/dev/pypx/api && go test ./... -v
```

Expected: all tests PASS.

- [ ] **Step 7: Commit**

```bash
cd /Users/matt/dev/pypx
git add api/
git commit -m "feat(api): add changelog endpoint fetching GitHub Releases"
```

---

## Phase 2: Frontend

### Task 4: Add @nuxtjs/mdc and Changelog Types

**Files:**
- Modify: `web/app/types/api.ts`
- Modify: `web/app/composables/useApi.ts`
- Modify: `web/nuxt.config.ts`

- [ ] **Step 1: Install @nuxtjs/mdc**

```bash
cd /Users/matt/dev/pypx/web && pnpm add @nuxtjs/mdc
```

- [ ] **Step 2: Add to nuxt.config.ts modules**

Add `'@nuxtjs/mdc'` to the modules array in `web/nuxt.config.ts`.

- [ ] **Step 3: Add TypeScript types**

Add to `web/app/types/api.ts`:

```ts
export interface ChangelogEntry {
  version: string
  tag_name: string
  title: string
  body: string
  published_at: string
  url: string
}

export interface ChangelogData {
  package: string
  source: string
  repo_url: string
  entries: ChangelogEntry[]
}
```

- [ ] **Step 4: Add fetchChangelog to composable**

Add to `web/app/composables/useApi.ts`, inside the `useApi` function:

```ts
  async function fetchChangelog(name: string): Promise<ChangelogData> {
    return $fetch<ChangelogData>(`${baseURL}/packages/${name}/changelog`)
  }
```

Add to the return statement: `fetchChangelog`.

Add `ChangelogData` to the import from `~/types/api`.

- [ ] **Step 5: Commit**

```bash
cd /Users/matt/dev/pypx
git add web/
git commit -m "feat(web): add @nuxtjs/mdc module and changelog API types"
```

---

### Task 5: Expandable Changelog in Versions Tab

**Files:**
- Modify: `web/app/components/PackageVersions.vue`

- [ ] **Step 1: Update PackageVersions.vue**

Read the current `web/app/components/PackageVersions.vue` first.

Modifications needed:

1. **Fetch changelog in parallel** — add a second `useAsyncData` call for changelog data
2. **Build lookup map** — computed that maps `version → ChangelogEntry`
3. **Track expanded state** — `ref<Set<string>>` for which versions are expanded
4. **Add expand indicator** — chevron or badge on rows with changelog entries
5. **Add expandable row** — when expanded, render changelog body with `<MDC>` below the version row

The updated component should:
- Import `ChangelogEntry` type
- Call `useApi().fetchChangelog(props.name)` with a separate `useAsyncData`
- Create `changelogMap` computed: `new Map(changelog.entries.map(e => [e.version, e]))` 
- Track `expandedVersions` as a `Set<string>` ref
- For each version row, check `changelogMap.has(ver.version)` to show a chevron
- On click, toggle the version in `expandedVersions`
- When expanded, show a `<tr>` below with `<td colspan="4">` containing the changelog body rendered via `<MDC :value="entry.body" />`
- Include a link to the GitHub release page: `entry.url`

Style the expanded area:
- `bg-zinc-900/50 border border-zinc-800 rounded-lg p-4 my-2`
- Changelog title as a small heading
- Published date
- MDC-rendered body in `prose prose-invert prose-sm` classes
- Link to full release on GitHub

- [ ] **Step 2: Commit**

```bash
cd /Users/matt/dev/pypx
git add web/
git commit -m "feat(web): add expandable changelog entries to versions tab"
```

---

### Task 6: Changelog on Version Detail Page

**Files:**
- Modify: `web/app/pages/packages/[name]/[version].vue`

- [ ] **Step 1: Update version page**

Read the current `web/app/pages/packages/[name]/[version].vue` first.

Modifications needed:

1. **Fetch changelog** — add `useAsyncData` for `fetchChangelog(name.value)`
2. **Find matching entry** — computed that finds `entries.find(e => e.version === version.value)`
3. **Render inline** — below the metadata grid (Released/Size/Format/Files) and distribution files section, render the changelog entry if it exists
4. **Use MDC** — render `entry.body` with `<MDC :value="entry.body" />`

The changelog section should only appear when a matching entry exists. No empty state or "no changelog" message.

Include:
- Changelog title as heading
- Published date
- Link to GitHub release page
- MDC-rendered body

Style: same zinc theme, `prose prose-invert prose-sm` for the markdown content.

- [ ] **Step 2: Commit**

```bash
cd /Users/matt/dev/pypx
git add web/
git commit -m "feat(web): show changelog on version detail page"
```

---

### Task 7: Markdown Rendering for Package Descriptions

**Files:**
- Modify: `web/app/components/PackageOverview.vue`

- [ ] **Step 1: Update PackageOverview.vue**

Read the current `web/app/components/PackageOverview.vue` first.

Replace the plain-text description rendering with conditional markdown rendering:

- If `pkg.description_content_type` contains `text/markdown`, render with `<MDC :value="pkg.description" />`
- If `text/x-rst`, keep as plain text for now (RST rendering is out of scope)
- Otherwise, render as plain text with `whitespace-pre-wrap`

Wrap the MDC output in `prose prose-invert` classes for proper markdown styling.

- [ ] **Step 2: Commit**

```bash
cd /Users/matt/dev/pypx
git add web/
git commit -m "feat(web): render markdown package descriptions with MDC"
```

---

### Task 8: Smoke Test

- [ ] **Step 1: Verify Go API changelog endpoint**

```bash
cd /Users/matt/dev/pypx/api && go run ./cmd/server/ &
sleep 2
curl -s http://localhost:8080/api/packages/requests/changelog | python3 -c "
import sys, json
d = json.load(sys.stdin)
print(f'package={d[\"package\"]} source={d[\"source\"]} entries={len(d[\"entries\"])}')
if d['entries']:
    print(f'latest: {d[\"entries\"][0][\"version\"]} - {d[\"entries\"][0][\"title\"][:50]}')
"
kill %1 2>/dev/null
```

Expected: entries with real GitHub Release data for requests.

- [ ] **Step 2: Verify Nuxt builds**

```bash
cd /Users/matt/dev/pypx/web && pnpm build
```

Expected: clean build.

- [ ] **Step 3: Commit any final adjustments**

```bash
cd /Users/matt/dev/pypx
git diff --quiet || git add -A && git commit -m "chore: final changelog integration adjustments"
```

---

## Summary

**8 tasks across 2 phases:**

| Phase | Tasks | What it delivers |
|-------|-------|-----------------|
| 1. Go API | 1–3 | GitHub client (repo extraction + releases fetch), changelog endpoint |
| 2. Frontend | 4–8 | @nuxtjs/mdc, expandable changelog in versions tab, changelog on version page, markdown descriptions, smoke test |
