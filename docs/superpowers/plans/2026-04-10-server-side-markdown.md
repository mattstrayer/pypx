# Server-Side Markdown Rendering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move markdown-to-HTML rendering from the client (`@nuxtjs/mdc`, 220KB gzip) to the Go API, so the frontend receives pre-rendered HTML and ships zero markdown processing JS.

**Architecture:** Add a `markdown` package to the Go API using `goldmark` (CommonMark + GFM). The two endpoints that return raw markdown (`/api/packages/{name}` and `/api/packages/{name}/changelog`) will return a new `description_html` / `body_html` field alongside the existing raw fields. The frontend replaces `<MDC :value="...">` with `<div v-html="...">` and the `@nuxtjs/mdc` module is removed entirely.

**Tech Stack:** Go + `github.com/yuin/goldmark` (with goldmark-highlighting for syntax highlighting), Nuxt 4 + Tailwind CSS

---

### Task 1: Add goldmark markdown renderer to Go API

**Files:**
- Create: `api/internal/markdown/render.go`
- Create: `api/internal/markdown/render_test.go`
- Modify: `api/go.mod`

- [ ] **Step 1: Add goldmark dependency**

```bash
cd api && go get github.com/yuin/goldmark github.com/yuin/goldmark-highlighting/v2
```

- [ ] **Step 2: Write the failing test**

Create `api/internal/markdown/render_test.go`:

```go
package markdown_test

import (
	"testing"

	"github.com/pypx/api/internal/markdown"
)

func TestRender_Heading(t *testing.T) {
	html, err := markdown.Render("# Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if html != "<h1>Hello</h1>\n" {
		t.Errorf("got %q, want %q", html, "<h1>Hello</h1>\n")
	}
}

func TestRender_GFM(t *testing.T) {
	html, err := markdown.Render("~~strike~~ and https://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// GFM should auto-link URLs and render strikethrough
	if len(html) == 0 {
		t.Error("expected non-empty HTML")
	}
	if !contains(html, "<del>strike</del>") {
		t.Errorf("expected strikethrough, got %q", html)
	}
	if !contains(html, "https://example.com") {
		t.Errorf("expected autolinked URL, got %q", html)
	}
}

func TestRender_EmptyInput(t *testing.T) {
	html, err := markdown.Render("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if html != "" {
		t.Errorf("expected empty string, got %q", html)
	}
}

func TestRender_PlainText(t *testing.T) {
	html, err := markdown.Render("Just plain text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if html != "<p>Just plain text</p>\n" {
		t.Errorf("got %q", html)
	}
}

func TestRender_CodeBlock(t *testing.T) {
	input := "```python\nprint('hello')\n```"
	html, err := markdown.Render(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(html, "<pre") {
		t.Errorf("expected <pre> block, got %q", html)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd api && go test ./internal/markdown/ -v
```

Expected: compilation error — package doesn't exist yet.

- [ ] **Step 4: Write the implementation**

Create `api/internal/markdown/render.go`:

```go
package markdown

import (
	"bytes"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

var md = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		highlighting.NewHighlighting(
			highlighting.WithStyle("monokai"),
			highlighting.WithFormatOptions(),
		),
	),
	goldmark.WithRendererOptions(
		html.WithUnsafe(), // PyPI descriptions contain raw HTML
	),
)

// Render converts a markdown string to HTML.
// Returns empty string for empty input.
func Render(src string) (string, error) {
	if src == "" {
		return "", nil
	}
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd api && go test ./internal/markdown/ -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd api && git add internal/markdown/ go.mod go.sum
git commit -m "feat(api): add goldmark markdown renderer"
```

---

### Task 2: Add `description_html` to package endpoint

**Files:**
- Modify: `api/internal/handler/packages.go:29-48` (PackageResponse struct)
- Modify: `api/internal/handler/packages.go:281-315` (buildPackageResponse)
- Modify: `api/internal/handler/packages_test.go`

- [ ] **Step 1: Write the failing test**

Add to `api/internal/handler/packages_test.go`. First update the mock to include markdown description. Find the existing `mockRequestsResponse` const and create a new one below it:

```go
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
```

Then add the test function:

```go
func TestPackageGet_RendersMarkdownDescription(t *testing.T) {
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMarkdownDescriptionResponse)
	}))
	defer pypiSrv.Close()

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	c := cache.NewMemoryCache(cache.NewSQLiteCache(":memory:"), 100)
	h := handler.NewPackageHandler(pypiClient, c)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}", h.Get)

	req := httptest.NewRequest("GET", "/api/packages/requests", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp handler.PackageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	// Raw description still present
	if resp.Description != "# Requests\n\nHTTP for **Humans**." {
		t.Errorf("description = %q, want raw markdown", resp.Description)
	}

	// HTML rendering present
	if resp.DescriptionHTML == "" {
		t.Error("description_html is empty, expected rendered HTML")
	}
	if !containsSubstring(resp.DescriptionHTML, "<strong>Humans</strong>") {
		t.Errorf("description_html = %q, expected <strong>Humans</strong>", resp.DescriptionHTML)
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && strings.Contains(s, sub)
}
```

Add `"strings"` to the imports at the top of the test file if not already present.

- [ ] **Step 2: Run test to verify it fails**

```bash
cd api && go test ./internal/handler/ -run TestPackageGet_RendersMarkdownDescription -v
```

Expected: FAIL — `resp.DescriptionHTML` field does not exist.

- [ ] **Step 3: Update PackageResponse struct and buildPackageResponse**

In `api/internal/handler/packages.go`, add the `DescriptionHTML` field to `PackageResponse` (after line 34):

```go
type PackageResponse struct {
	Name            string                       `json:"name"`
	Version         string                       `json:"version"`
	Summary         string                       `json:"summary"`
	Description     string                       `json:"description"`
	DescType        string                       `json:"description_content_type"`
	DescriptionHTML string                       `json:"description_html"`
	License         string                       `json:"license"`
	Author          string                       `json:"author"`
	AuthorEmail     string                       `json:"author_email"`
	HomePage        string                       `json:"home_page"`
	RequiresPython  string                       `json:"requires_python"`
	RequiresDist    []string                     `json:"requires_dist"`
	ProjectURLs     map[string]string            `json:"project_urls"`
	Classifiers     []string                     `json:"classifiers"`
	LatestFiles     []FileInfo                   `json:"latest_files"`
	InstallSize     int64                        `json:"install_size"`
	ModuleFormat    string                       `json:"module_format"`
	PythonVersions  enrichment.PythonVersionInfo `json:"python_versions"`
	Dependencies    enrichment.DependencyTree    `json:"dependencies"`
}
```

In the `buildPackageResponse` function, add markdown rendering after line 299 (`Description: info.Description,`). Add the import for the markdown package and render conditionally:

Add to imports:

```go
"github.com/pypx/api/internal/markdown"
```

Update `buildPackageResponse` to render markdown:

```go
func buildPackageResponse(r *pypi.PyPIResponse) PackageResponse {
	info := r.Info

	files := make([]FileInfo, 0, len(r.URLs))
	for _, f := range r.URLs {
		files = append(files, FileInfo{
			Filename:    f.Filename,
			Size:        f.Size,
			PackageType: f.PackageType,
			PythonVer:   f.PythonVer,
			UploadTime:  f.UploadTime,
		})
	}

	var descHTML string
	if strings.Contains(info.DescriptionType, "text/markdown") {
		descHTML, _ = markdown.Render(info.Description)
	}

	return PackageResponse{
		Name:            info.Name,
		Version:         info.Version,
		Summary:         info.Summary,
		Description:     info.Description,
		DescType:        info.DescriptionType,
		DescriptionHTML: descHTML,
		License:         info.License,
		Author:          info.Author,
		AuthorEmail:     info.AuthorEmail,
		HomePage:        info.HomePage,
		RequiresPython:  info.RequiresPython,
		RequiresDist:    info.RequiresDist,
		ProjectURLs:     info.ProjectURLs,
		Classifiers:     info.Classifiers,
		LatestFiles:     files,
		InstallSize:     enrichment.ExtractInstallSize(r.URLs),
		ModuleFormat:    enrichment.ExtractModuleFormat(r.URLs),
		PythonVersions:  enrichment.ExtractPythonVersions(info.RequiresPython),
		Dependencies:    enrichment.ParseDependencies(info.RequiresDist),
	}
}
```

- [ ] **Step 4: Run tests**

```bash
cd api && go test ./internal/handler/ -v
```

Expected: all PASS (new test + existing tests).

- [ ] **Step 5: Commit**

```bash
cd api && git add internal/handler/packages.go internal/handler/packages_test.go
git commit -m "feat(api): render markdown descriptions to HTML server-side"
```

---

### Task 3: Add `body_html` to changelog endpoint

**Files:**
- Modify: `api/internal/github/client.go:82-90` (Release struct)
- Modify: `api/internal/handler/changelog.go:42-107` (Get handler)
- Modify: `api/internal/handler/changelog_test.go`

- [ ] **Step 1: Write the failing test**

Add to `api/internal/handler/changelog_test.go`. Check the existing test file to understand the test pattern used, then add:

```go
func TestChangelogGet_RendersBodyHTML(t *testing.T) {
	// Mock PyPI server returning a package with GitHub project URL.
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"info": {
				"name": "requests",
				"version": "2.31.0",
				"summary": "",
				"description": "",
				"description_content_type": "",
				"license": "",
				"author": "",
				"author_email": "",
				"home_page": "",
				"requires_python": "",
				"requires_dist": null,
				"project_urls": {"Source": "https://github.com/psf/requests"},
				"classifiers": null
			},
			"releases": {},
			"urls": []
		}`)
	}))
	defer pypiSrv.Close()

	// Mock GitHub server returning a release with markdown body.
	ghSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{
			"tag_name": "v2.31.0",
			"name": "v2.31.0",
			"body": "## What's Changed\n\n- Fixed **bug** in auth",
			"published_at": "2023-05-22T00:00:00Z",
			"html_url": "https://github.com/psf/requests/releases/tag/v2.31.0"
		}]`)
	}))
	defer ghSrv.Close()

	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	c := cache.NewMemoryCache(cache.NewSQLiteCache(":memory:"), 100)
	pkgHandler := handler.NewPackageHandler(pypiClient, c)
	ghClient := gh.NewClient(gh.WithBaseURL(ghSrv.URL + "/repos"))
	chHandler := handler.NewChangelogHandler(ghClient, c, pkgHandler)

	router := chi.NewRouter()
	router.Get("/api/packages/{name}/changelog", chHandler.Get)

	req := httptest.NewRequest("GET", "/api/packages/requests/changelog", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp handler.ChangelogResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if len(resp.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(resp.Entries))
	}

	entry := resp.Entries[0]
	if entry.BodyHTML == "" {
		t.Error("body_html is empty, expected rendered HTML")
	}
	if !strings.Contains(entry.BodyHTML, "<strong>bug</strong>") {
		t.Errorf("body_html = %q, expected <strong>bug</strong>", entry.BodyHTML)
	}
}
```

Make sure these imports are at the top of the test file:

```go
import (
	"encoding/json"
	"fmt"
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd api && go test ./internal/handler/ -run TestChangelogGet_RendersBodyHTML -v
```

Expected: FAIL — `entry.BodyHTML` field does not exist.

- [ ] **Step 3: Add BodyHTML to Release struct and render in changelog handler**

In `api/internal/github/client.go`, add `BodyHTML` field to the `Release` struct (after line 88):

```go
type Release struct {
	Version     string `json:"version"`
	TagName     string `json:"tag_name"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	BodyHTML    string `json:"body_html"`
	PublishedAt string `json:"published_at"`
	URL         string `json:"url"`
}
```

In `api/internal/handler/changelog.go`, render markdown for each release body before caching. Add the import:

```go
"github.com/pypx/api/internal/markdown"
```

In the `Get` method, after `releases` are fetched (after line 82, before `if releases == nil`), add:

```go
	// Render markdown bodies to HTML.
	for i := range releases {
		if releases[i].Body != "" {
			releases[i].BodyHTML, _ = markdown.Render(releases[i].Body)
		}
	}
```

- [ ] **Step 4: Run tests**

```bash
cd api && go test ./internal/handler/ -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
cd api && git add internal/github/client.go internal/handler/changelog.go internal/handler/changelog_test.go
git commit -m "feat(api): render changelog markdown bodies to HTML server-side"
```

---

### Task 4: Update frontend to use pre-rendered HTML and remove MDC

**Files:**
- Modify: `web/app/types/api.ts`
- Modify: `web/app/components/PackageOverview.vue`
- Modify: `web/app/components/PackageVersions.vue`
- Modify: `web/app/pages/packages/[name]/[version].vue`
- Modify: `web/nuxt.config.ts`
- Modify: `web/package.json`

- [ ] **Step 1: Update TypeScript types**

In `web/app/types/api.ts`, add `description_html` to `PackageData` (after line 5):

```ts
export interface PackageData {
  name: string
  version: string
  summary: string
  description: string
  description_content_type: string
  description_html: string
  license: string
  author: string
  author_email: string
  home_page: string
  requires_python: string
  requires_dist: string[]
  project_urls: Record<string, string>
  classifiers: string[]
  latest_files: FileInfo[]
  install_size: number
  module_format: string
  python_versions: PythonVersionInfo
  dependencies: DependencyTree
}
```

Add `body_html` to `ChangelogEntry` (after line 75):

```ts
export interface ChangelogEntry {
  version: string
  tag_name: string
  title: string
  body: string
  body_html: string
  published_at: string
  url: string
}
```

- [ ] **Step 2: Update PackageOverview.vue**

Replace the full file `web/app/components/PackageOverview.vue`:

In the `<script setup>` section, remove the `isMarkdown` computed (lines 12-14).

In the template, replace the description block (lines 31-44) with:

```vue
      <div
        v-if="pkg.description"
        class="rounded-lg border border-zinc-800 bg-zinc-900/50 p-5"
      >
        <h2 class="mb-3 text-sm font-semibold uppercase tracking-wider text-zinc-500">
          Description
        </h2>
        <div v-if="pkg.description_html" class="prose prose-invert prose-sm max-w-none" v-html="pkg.description_html" />
        <div v-else class="whitespace-pre-wrap text-sm leading-relaxed text-zinc-300">
          {{ pkg.description }}
        </div>
      </div>
```

- [ ] **Step 3: Update PackageVersions.vue**

In `web/app/components/PackageVersions.vue`, replace the MDC line (line 111):

```html
                  <MDC :value="entry.body" />
```

With:

```html
                  <div v-if="entry.body_html" v-html="entry.body_html" />
                  <div v-else class="whitespace-pre-wrap">{{ entry.body }}</div>
```

- [ ] **Step 4: Update version page**

In `web/app/pages/packages/[name]/[version].vue`, replace the MDC line (line 130):

```html
          <MDC :value="changelogEntry.body" />
```

With:

```html
          <div v-if="changelogEntry.body_html" v-html="changelogEntry.body_html" />
          <div v-else class="whitespace-pre-wrap">{{ changelogEntry.body }}</div>
```

- [ ] **Step 5: Remove @nuxtjs/mdc module**

In `web/nuxt.config.ts`, remove `'@nuxtjs/mdc'` from the modules array:

```ts
  modules: [
    '@nuxtjs/color-mode',
    '@vueuse/nuxt',
    '@nuxtjs/seo',
  ],
```

Remove the `_mdc` route rule exclusion since it's no longer needed:

```ts
  routeRules: {
    '/api/**': {
      proxy: { to: `${process.env.API_BASE || 'http://localhost:8080'}/**` },
    },
  },
```

In the Caddyfile (`Caddyfile`), remove the `_mdc` handler block:

```
    handle /api/_mdc/* {
        reverse_proxy web:3000
    }
```

So the handle section becomes just:

```
    handle /api/* {
        reverse_proxy api:8080
    }

    handle {
        reverse_proxy web:3000
    }
```

- [ ] **Step 6: Uninstall @nuxtjs/mdc**

```bash
cd web && pnpm remove @nuxtjs/mdc
```

- [ ] **Step 7: Build and verify**

```bash
cd web && rm -rf .nuxt .output && npx nuxt build 2>&1 | tail -5
```

Expected: build succeeds, entry CSS file present.

Check the JS payload dropped significantly:

```bash
find .output/public/_nuxt -name "*.js" -exec cat {} + | wc -c
```

Expected: total JS well under 500KB (was 979KB).

- [ ] **Step 8: Commit**

```bash
git add web/app/types/api.ts web/app/components/PackageOverview.vue web/app/components/PackageVersions.vue web/app/pages/packages/\[name\]/\[version\].vue web/nuxt.config.ts web/package.json web/pnpm-lock.yaml Caddyfile
git commit -m "refactor: replace client-side MDC with server-rendered markdown HTML

Remove @nuxtjs/mdc (unified/remark/rehype) from client bundle.
Markdown is now rendered to HTML by the Go API using goldmark.
Expected JS payload reduction: ~500KB raw / ~170KB gzip."
```

---

### Task 5: Add prose Tailwind typography styles

**Files:**
- Modify: `web/app/assets/css/main.css`
- Modify: `web/package.json`

The `prose prose-invert` classes used on the `v-html` containers come from `@tailwindcss/typography`. With MDC removed, we need this plugin directly.

- [ ] **Step 1: Install @tailwindcss/typography**

```bash
cd web && pnpm add @tailwindcss/typography
```

- [ ] **Step 2: Import in main.css**

Add the typography plugin import to `web/app/assets/css/main.css` after the tailwindcss import:

```css
@import "tailwindcss";
@import "@tailwindcss/typography";
@import url('https://fonts.googleapis.com/css2?family=Geist:wght@400;500;600;700&family=Geist+Mono:wght@400;500;600;700&display=swap');
```

- [ ] **Step 3: Build and verify prose styles work**

```bash
cd web && rm -rf .nuxt .output && npx nuxt build 2>&1 | grep "entry.*css"
```

Expected: entry CSS size increases slightly (typography styles added). Build succeeds.

- [ ] **Step 4: Commit**

```bash
git add web/app/assets/css/main.css web/package.json web/pnpm-lock.yaml
git commit -m "feat(web): add @tailwindcss/typography for prose styles"
```

---

### Task 6: Docker rebuild and end-to-end verification

**Files:** None (verification only)

- [ ] **Step 1: Rebuild both containers**

```bash
docker compose build --no-cache 2>&1 | tail -5
```

- [ ] **Step 2: Start and verify**

```bash
docker compose up -d && sleep 10
```

Verify the API returns `description_html`:

```bash
curl -sk https://localhost/api/packages/requests | python3 -c "import sys,json; d=json.load(sys.stdin); print('description_html present:', bool(d.get('description_html'))); print('first 100 chars:', d.get('description_html','')[:100])"
```

Expected: `description_html present: True` with HTML content.

Verify changelog returns `body_html`:

```bash
curl -sk https://localhost/api/packages/requests/changelog | python3 -c "import sys,json; d=json.load(sys.stdin); entries=d.get('entries',[]); print('entries:', len(entries)); print('body_html present:', bool(entries[0].get('body_html'))) if entries else print('no entries')"
```

Verify the frontend serves correct HTML and reduced JS:

```bash
curl -sk https://localhost/ | grep -o 'entry\.[^"]*\.css'
find web/.output/public/_nuxt -name "*.js" -exec cat {} + | wc -c
```

Expected: total JS significantly smaller than 979KB.

- [ ] **Step 3: Verify no MDC artifacts remain**

```bash
grep -r "MDC\|@nuxtjs/mdc\|mdc" web/.output/ 2>/dev/null | grep -v ".map" | head -5
```

Expected: no matches (MDC completely removed).

- [ ] **Step 4: Commit (if any cleanup needed)**

If no changes needed, skip. Otherwise commit any fixes.
