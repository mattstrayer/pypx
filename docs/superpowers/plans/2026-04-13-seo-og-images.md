# SEO & OG Images Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add dynamic Open Graph images (via takumi), a two-tier sitemap, JSON-LD schema, and a global grid background to pypx.

**Architecture:** `.takumi.vue` templates in `app/components/OgImage/` are auto-detected by nuxt-og-image v6 and rendered by `@takumi-rs/core` at request time. A single Nuxt server route feeds `@nuxtjs/sitemap` v8 with two named sitemaps: `popular` (top 5000 by downloads, from the Go API) and `cached` (all browsed packages, from the Go API's new `/api/sitemap/packages` endpoint). JSON-LD is added via `useSchemaOrg` auto-imported from `nuxt-schema-org`.

**Tech Stack:** `@takumi-rs/core` (dev), `nuxt-og-image` v6 (already in `@nuxtjs/seo`), `@nuxtjs/sitemap` v8 (already in `@nuxtjs/seo`), `nuxt-schema-org` v6 (already in `@nuxtjs/seo`), Go chi v5.

**Branch:** `feat/seo-og-images` (already created)

---

## File Map

| File | Action | Purpose |
|---|---|---|
| `web/package.json` | modify | Add `@takumi-rs/core` devDependency |
| `web/nuxt.config.ts` | modify | Enable ogImage, configure sitemap sitemaps |
| `web/app/assets/css/main.css` | modify | Global grid + glow background on `body` |
| `web/app/components/OgImage/SiteCard.takumi.vue` | **create** | Static branded OG image for home + search |
| `web/app/components/OgImage/PackageCard.takumi.vue` | **create** | Dynamic OG image for package pages |
| `web/app/components/OgImage/DocsCard.takumi.vue` | **create** | OG image with "API Docs" badge for docs page |
| `web/app/pages/index.vue` | modify | Add `defineOgImage` |
| `web/app/pages/search.vue` | modify | Add `defineOgImage` |
| `web/app/pages/packages/[name].vue` | modify | Add `defineOgImage` + `useSchemaOrg` |
| `web/app/pages/packages/[name]/docs.vue` | modify | Add `defineOgImage` |
| `web/app/pages/packages/[name]/[version].vue` | modify | Add `defineOgImage` |
| `web/server/api/__sitemap__/urls.ts` | **create** | Nuxt sitemap event handler |
| `api/internal/cache/sqlite.go` | modify | Add `ListPackageNames() ([]string, error)` |
| `api/internal/handler/sitemap.go` | **create** | Go handler for sitemap endpoints |
| `api/internal/handler/sitemap_test.go` | **create** | Tests for sitemap handler |
| `api/cmd/server/main.go` | modify | Register sitemap routes |

---

## Task 1: Install @takumi-rs/core

**Files:**
- Modify: `web/package.json`

- [ ] **Step 1: Install the package**

```bash
cd web && npm install -D @takumi-rs/core
```

Expected: package added to `devDependencies` in `package.json`. `node_modules/@takumi-rs/core` exists.

- [ ] **Step 2: Verify install**

```bash
ls web/node_modules/@takumi-rs/core/package.json
```

Expected: file exists (exit 0).

- [ ] **Step 3: Commit**

```bash
cd web && git add package.json package-lock.json
git commit -m "chore(web): install @takumi-rs/core for OG image rendering"
```

---

## Task 2: Add global grid background

**Files:**
- Modify: `web/app/assets/css/main.css`

- [ ] **Step 1: Add grid and glow to `main.css`**

Append the following to the end of `web/app/assets/css/main.css`:

```css
/* Global grid background */
body::before {
  content: '';
  position: fixed;
  inset: 0;
  z-index: -1;
  background-image:
    linear-gradient(rgba(74, 222, 128, 0.04) 1px, transparent 1px),
    linear-gradient(90deg, rgba(74, 222, 128, 0.04) 1px, transparent 1px);
  background-size: 32px 32px;
  pointer-events: none;
}

body::after {
  content: '';
  position: fixed;
  top: -120px;
  right: -120px;
  width: 480px;
  height: 480px;
  background: radial-gradient(circle, rgba(74, 222, 128, 0.08), transparent 70%);
  border-radius: 50%;
  z-index: -1;
  pointer-events: none;
}
```

- [ ] **Step 2: Start dev server and verify visually**

```bash
cd web && npm run dev
```

Open http://localhost:3000. You should see a very subtle green grid texture across the entire background, with a soft green glow in the top-right corner. The content should be unaffected.

- [ ] **Step 3: Commit**

```bash
git add web/app/assets/css/main.css
git commit -m "feat(web): add global grid background and ambient glow"
```

---

## Task 3: Create SiteCard OG image template

**Files:**
- Create: `web/app/components/OgImage/SiteCard.takumi.vue`

The `.takumi.vue` suffix tells nuxt-og-image to use the Takumi renderer automatically — no nuxt.config.ts renderer config needed.

- [ ] **Step 1: Create the template**

Create `web/app/components/OgImage/SiteCard.takumi.vue`:

```vue
<template>
  <div style="width:100%;height:100%;display:flex;flex-direction:column;align-items:center;justify-content:center;background:#09090b;position:relative;">
    <!-- Grid background -->
    <div style="position:absolute;inset:0;background-image:linear-gradient(rgba(74,222,128,0.04) 1px,transparent 1px),linear-gradient(90deg,rgba(74,222,128,0.04) 1px,transparent 1px);background-size:32px 32px;"></div>
    <!-- Ambient glow top-right -->
    <div style="position:absolute;top:-80px;right:-80px;width:360px;height:360px;background:radial-gradient(circle,rgba(74,222,128,0.18),transparent 70%);border-radius:50%;"></div>
    <!-- Content -->
    <div style="position:relative;display:flex;flex-direction:column;align-items:center;gap:16px;">
      <span style="font-size:80px;font-weight:700;color:#4ade80;letter-spacing:-3px;line-height:1;">pypx</span>
      <span style="font-size:24px;color:#a1a1aa;text-align:center;max-width:600px;line-height:1.4;">The Python Package Index, reimagined</span>
      <span style="font-size:15px;color:#52525b;margin-top:12px;">pypx.app</span>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Commit**

```bash
git add web/app/components/OgImage/SiteCard.takumi.vue
git commit -m "feat(web): add SiteCard OG image template"
```

---

## Task 4: Add OG image to home and search pages

**Files:**
- Modify: `web/app/pages/index.vue`
- Modify: `web/app/pages/search.vue`

`defineOgImage` is auto-imported from `nuxt-og-image`. The third argument sets width/height to the standard 1200×630.

- [ ] **Step 1: Add `defineOgImage` to `index.vue`**

In `web/app/pages/index.vue`, add after the existing `useSeoMeta(...)` call:

```ts
defineOgImage('SiteCard', {}, { width: 1200, height: 630 })
```

- [ ] **Step 2: Add `defineOgImage` to `search.vue`**

In `web/app/pages/search.vue`, find the existing `useSeoMeta({...})` call. Add after it:

```ts
defineOgImage('SiteCard', {}, { width: 1200, height: 630 })
```

If `search.vue` has no `useSeoMeta`, add before the closing `</script>`:

```ts
useSeoMeta({
  title: 'Search — pypx',
  description: 'Search 400,000+ Python packages on pypx.',
  ogTitle: 'Search — pypx',
  ogDescription: 'Search 400,000+ Python packages on pypx.',
})
defineOgImage('SiteCard', {}, { width: 1200, height: 630 })
```

- [ ] **Step 3: Enable ogImage in nuxt.config.ts**

In `web/nuxt.config.ts`, replace:

```ts
  ogImage: {
    enabled: false,
  },
```

with:

```ts
  ogImage: {
    enabled: true,
  },
```

- [ ] **Step 4: Verify OG image renders**

Start the dev server (`npm run dev` from `web/`). Open:

```
http://localhost:3000/__og_image__/
```

You should see the nuxt-og-image playground. Navigate to the home page entry and confirm the SiteCard renders: dark background, grid texture, green `pypx` wordmark, tagline.

- [ ] **Step 5: Commit**

```bash
git add web/nuxt.config.ts web/app/pages/index.vue web/app/pages/search.vue
git commit -m "feat(web): add SiteCard OG image to home and search pages"
```

---

## Task 5: Create PackageCard OG image template

**Files:**
- Create: `web/app/components/OgImage/PackageCard.takumi.vue`

Props: `name` (required), `version` (required), `summary` (optional), `license` (optional). Downloads are not included — they aren't available in the SSR package fetch.

- [ ] **Step 1: Create the template**

Create `web/app/components/OgImage/PackageCard.takumi.vue`:

```vue
<script setup lang="ts">
defineProps<{
  name: string
  version: string
  summary?: string
  license?: string
}>()
</script>

<template>
  <div style="width:100%;height:100%;display:flex;flex-direction:column;justify-content:space-between;background:#09090b;padding:52px 60px;position:relative;box-sizing:border-box;">
    <!-- Grid background -->
    <div style="position:absolute;inset:0;background-image:linear-gradient(rgba(74,222,128,0.04) 1px,transparent 1px),linear-gradient(90deg,rgba(74,222,128,0.04) 1px,transparent 1px);background-size:32px 32px;"></div>
    <!-- Ambient glow top-right -->
    <div style="position:absolute;top:-80px;right:-80px;width:320px;height:320px;background:radial-gradient(circle,rgba(74,222,128,0.15),transparent 70%);border-radius:50%;"></div>
    <!-- Top section: name + version + meta + summary -->
    <div style="position:relative;display:flex;flex-direction:column;gap:12px;">
      <div style="display:flex;align-items:baseline;gap:14px;flex-wrap:wrap;">
        <span style="font-size:54px;font-weight:700;color:#f4f4f5;line-height:1;letter-spacing:-1.5px;">{{ name }}</span>
        <span style="font-size:16px;color:#4ade80;background:rgba(74,222,128,0.1);border:1px solid rgba(74,222,128,0.25);padding:4px 12px;border-radius:4px;font-family:monospace;flex-shrink:0;">{{ version }}</span>
      </div>
      <div v-if="license" style="font-size:14px;color:#71717a;">{{ license }}</div>
      <p v-if="summary" style="font-size:19px;color:#a1a1aa;line-height:1.45;max-width:720px;margin:0;">{{ summary }}</p>
    </div>
    <!-- Footer -->
    <div style="position:relative;display:flex;justify-content:space-between;align-items:center;">
      <span style="font-size:14px;color:#3f3f46;">pypx.app — The Python Package Index, reimagined</span>
      <span style="font-size:22px;font-weight:700;color:#4ade80;letter-spacing:-0.5px;">pypx</span>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Commit**

```bash
git add web/app/components/OgImage/PackageCard.takumi.vue
git commit -m "feat(web): add PackageCard OG image template"
```

---

## Task 6: Add PackageCard OG image to package and version pages

**Files:**
- Modify: `web/app/pages/packages/[name].vue`
- Modify: `web/app/pages/packages/[name]/[version].vue`

- [ ] **Step 1: Update `[name].vue`**

In `web/app/pages/packages/[name].vue`, add after the existing `useSeoMeta({...})` call (around line 51):

```ts
defineOgImage('PackageCard', {
  name: () => pkg.value?.name ?? '',
  version: () => pkg.value?.version ?? '',
  summary: () => pkg.value?.summary ?? undefined,
  license: () => pkg.value?.license ?? undefined,
}, { width: 1200, height: 630 })
```

- [ ] **Step 2: Update `[version].vue`**

In `web/app/pages/packages/[name]/[version].vue`, add after the existing `useSeoMeta({...})` call:

```ts
defineOgImage('PackageCard', {
  name: () => name.value,
  version: () => version.value,
  summary: () => pkg.value?.summary ?? undefined,
  license: () => pkg.value?.license ?? undefined,
}, { width: 1200, height: 630 })
```

- [ ] **Step 3: Verify in OG playground**

With the dev server running, navigate to `http://localhost:3000/__og_image__/`. Find a package page entry (e.g., after visiting `/packages/requests`). Confirm the PackageCard renders: dark background, grid texture, green glow, package name in large white text, green version badge, summary text.

- [ ] **Step 4: Commit**

```bash
git add web/app/pages/packages/\[name\].vue web/app/pages/packages/\[name\]/\[version\].vue
git commit -m "feat(web): add PackageCard OG image to package and version pages"
```

---

## Task 7: Create DocsCard OG image template

**Files:**
- Create: `web/app/components/OgImage/DocsCard.takumi.vue`

Same design as PackageCard with an "API Docs" badge above the name. No summary or license — just name and version.

- [ ] **Step 1: Create the template**

Create `web/app/components/OgImage/DocsCard.takumi.vue`:

```vue
<script setup lang="ts">
defineProps<{
  name: string
  version: string
}>()
</script>

<template>
  <div style="width:100%;height:100%;display:flex;flex-direction:column;justify-content:space-between;background:#09090b;padding:52px 60px;position:relative;box-sizing:border-box;">
    <!-- Grid background -->
    <div style="position:absolute;inset:0;background-image:linear-gradient(rgba(74,222,128,0.04) 1px,transparent 1px),linear-gradient(90deg,rgba(74,222,128,0.04) 1px,transparent 1px);background-size:32px 32px;"></div>
    <!-- Ambient glow top-right -->
    <div style="position:absolute;top:-80px;right:-80px;width:320px;height:320px;background:radial-gradient(circle,rgba(74,222,128,0.15),transparent 70%);border-radius:50%;"></div>
    <!-- Top section -->
    <div style="position:relative;display:flex;flex-direction:column;gap:14px;">
      <!-- API Docs badge -->
      <div style="display:inline-flex;align-items:center;background:rgba(74,222,128,0.1);border:1px solid rgba(74,222,128,0.25);color:#4ade80;font-size:11px;text-transform:uppercase;letter-spacing:1.8px;padding:5px 14px;border-radius:4px;width:fit-content;">
        API Docs
      </div>
      <!-- Name + version -->
      <div style="display:flex;align-items:baseline;gap:14px;flex-wrap:wrap;">
        <span style="font-size:54px;font-weight:700;color:#f4f4f5;line-height:1;letter-spacing:-1.5px;">{{ name }}</span>
        <span style="font-size:16px;color:#4ade80;background:rgba(74,222,128,0.1);border:1px solid rgba(74,222,128,0.25);padding:4px 12px;border-radius:4px;font-family:monospace;flex-shrink:0;">{{ version }}</span>
      </div>
    </div>
    <!-- Footer -->
    <div style="position:relative;display:flex;justify-content:space-between;align-items:center;">
      <span style="font-size:14px;color:#3f3f46;">pypx.app — The Python Package Index, reimagined</span>
      <span style="font-size:22px;font-weight:700;color:#4ade80;letter-spacing:-0.5px;">pypx</span>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Commit**

```bash
git add web/app/components/OgImage/DocsCard.takumi.vue
git commit -m "feat(web): add DocsCard OG image template"
```

---

## Task 8: Add DocsCard OG image to docs page

**Files:**
- Modify: `web/app/pages/packages/[name]/docs.vue`

- [ ] **Step 1: Update `docs.vue`**

In `web/app/pages/packages/[name]/docs.vue`, add after the existing `useSeoMeta({...})` call (around line 38):

```ts
defineOgImage('DocsCard', {
  name: () => pkg.value?.name ?? '',
  version: () => pkg.value?.version ?? '',
}, { width: 1200, height: 630 })
```

- [ ] **Step 2: Verify in OG playground**

With the dev server running, navigate to a docs page (e.g., `/packages/requests/docs`). In the OG playground (`/__og_image__/`), find the docs entry. Confirm: "API Docs" badge in green above the package name.

- [ ] **Step 3: Commit**

```bash
git add "web/app/pages/packages/[name]/docs.vue"
git commit -m "feat(web): add DocsCard OG image to docs page"
```

---

## Task 9: Add JSON-LD schema to package page

**Files:**
- Modify: `web/app/pages/packages/[name].vue`

`useSchemaOrg` is auto-imported from `nuxt-schema-org` (bundled in `@nuxtjs/seo`). It's reactive and safe to call with computed data.

- [ ] **Step 1: Add `useSchemaOrg` to `[name].vue`**

In `web/app/pages/packages/[name].vue`, after the `defineOgImage(...)` call, add:

```ts
useSchemaOrg([
  computed(() => pkg.value ? {
    '@type': 'SoftwareApplication' as const,
    'name': pkg.value.name,
    'description': pkg.value.summary || undefined,
    'softwareVersion': pkg.value.version,
    'applicationCategory': 'DeveloperApplication',
    'license': pkg.value.license || undefined,
    'url': `https://pypi.org/project/${pkg.value.name}/`,
  } : null),
])
```

- [ ] **Step 2: Verify JSON-LD in page source**

With the dev server running, open http://localhost:3000/packages/requests. In the browser, view page source (Cmd+U). Search for `application/ld+json`. You should see:

```json
{
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  "name": "requests",
  "softwareVersion": "2.32.3",
  "applicationCategory": "DeveloperApplication",
  ...
}
```

- [ ] **Step 3: Commit**

```bash
git add "web/app/pages/packages/[name].vue"
git commit -m "feat(web): add JSON-LD SoftwareApplication schema to package page"
```

---

## Task 10: Add ListPackageNames to SQLite cache

**Files:**
- Modify: `api/internal/cache/sqlite.go`

This method is used by the sitemap handler to enumerate all cached package names. It only belongs on the concrete `*Cache` type (not the `Cacher` interface) since the sitemap handler gets `*cache.Cache` directly.

- [ ] **Step 1: Add `ListPackageNames` to `sqlite.go`**

In `api/internal/cache/sqlite.go`, append after the `Close()` method:

```go
// ListPackageNames returns the name portion of all cache keys matching
// the "pkg:{name}" pattern, i.e. every package that has been fetched and
// cached at least once.
func (c *Cache) ListPackageNames() ([]string, error) {
	rows, err := c.db.Query(
		`SELECT SUBSTR(key, 5) FROM cache WHERE key LIKE 'pkg:%'`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}
```

- [ ] **Step 2: Write a test**

Create `api/internal/cache/sqlite_list_test.go`:

```go
package cache_test

import (
	"testing"

	"github.com/pypx/api/internal/cache"
)

func TestListPackageNames(t *testing.T) {
	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("new cache: %v", err)
	}
	defer c.Close()

	// Seed: two package keys and one non-package key.
	if err := c.Set("pkg:requests", []byte(`{}`), 0); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := c.Set("pkg:flask", []byte(`{}`), 0); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := c.Set("stats:requests:4w", []byte(`{}`), 0); err != nil {
		t.Fatalf("set: %v", err)
	}

	names, err := c.ListPackageNames()
	if err != nil {
		t.Fatalf("ListPackageNames: %v", err)
	}

	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d: %v", len(names), names)
	}

	got := make(map[string]bool)
	for _, n := range names {
		got[n] = true
	}
	for _, want := range []string{"requests", "flask"} {
		if !got[want] {
			t.Errorf("expected %q in names, got %v", want, names)
		}
	}
}
```

- [ ] **Step 3: Run the test**

```bash
cd api && go test ./internal/cache/... -run TestListPackageNames -v
```

Expected:
```
--- PASS: TestListPackageNames (0.00s)
PASS
```

- [ ] **Step 4: Commit**

```bash
git add api/internal/cache/sqlite.go api/internal/cache/sqlite_list_test.go
git commit -m "feat(api): add ListPackageNames to SQLite cache"
```

---

## Task 11: Create Go sitemap handler

**Files:**
- Create: `api/internal/handler/sitemap.go`
- Create: `api/internal/handler/sitemap_test.go`

The handler exposes two endpoints:
- `GET /api/sitemap/popular` — top 5000 package names from the search index
- `GET /api/sitemap/cached` — all package names from the SQLite cache

Both return `{"packages": ["requests", "flask", ...]}`.

- [ ] **Step 1: Create `sitemap.go`**

Create `api/internal/handler/sitemap.go`:

```go
package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/search"
)

const sitemapPopularLimit = 5000

// SitemapHandler serves package name lists for sitemap generation.
type SitemapHandler struct {
	index       *search.Index
	sqliteCache *cache.Cache
}

// NewSitemapHandler creates a new SitemapHandler.
func NewSitemapHandler(idx *search.Index, sqliteCache *cache.Cache) *SitemapHandler {
	return &SitemapHandler{index: idx, sqliteCache: sqliteCache}
}

// sitemapResponse is the JSON shape returned by both endpoints.
type sitemapResponse struct {
	Packages []string `json:"packages"`
}

// Popular handles GET /api/sitemap/popular — returns the top N packages by
// download count (up to sitemapPopularLimit).
func (h *SitemapHandler) Popular(w http.ResponseWriter, r *http.Request) {
	entries, err := h.index.TopByDownloads(sitemapPopularLimit)
	if err != nil {
		log.Printf("sitemap: TopByDownloads error: %v", err)
		http.Error(w, "failed to fetch popular packages", http.StatusInternalServerError)
		return
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=86400") // 24h
	if err := json.NewEncoder(w).Encode(sitemapResponse{Packages: names}); err != nil {
		log.Printf("sitemap: encode error: %v", err)
	}
}

// Cached handles GET /api/sitemap/cached — returns all packages that have
// been fetched and cached in SQLite at least once.
func (h *SitemapHandler) Cached(w http.ResponseWriter, r *http.Request) {
	names, err := h.sqliteCache.ListPackageNames()
	if err != nil {
		log.Printf("sitemap: ListPackageNames error: %v", err)
		http.Error(w, "failed to list cached packages", http.StatusInternalServerError)
		return
	}
	if names == nil {
		names = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600") // 1h
	if err := json.NewEncoder(w).Encode(sitemapResponse{Packages: names}); err != nil {
		log.Printf("sitemap: encode error: %v", err)
	}
}
```

Note: `search.PackageEntry.Name` is confirmed as the correct field name.

- [ ] **Step 2: Create `sitemap_test.go`**

Create `api/internal/handler/sitemap_test.go`:

```go
package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/search"
)

func setupSitemapHandler(t *testing.T) (*chi.Mux, *cache.Cache) {
	t.Helper()
	sqliteCache, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	t.Cleanup(func() { sqliteCache.Close() })

	searchIdx, err := search.NewIndex(":memory:")
	if err != nil {
		t.Fatalf("search.NewIndex: %v", err)
	}
	t.Cleanup(func() { searchIdx.Close() })

	h := handler.NewSitemapHandler(searchIdx, sqliteCache)
	r := chi.NewRouter()
	r.Get("/api/sitemap/popular", h.Popular)
	r.Get("/api/sitemap/cached", h.Cached)
	return r, sqliteCache
}

func TestSitemapCached_Empty(t *testing.T) {
	r, _ := setupSitemapHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/sitemap/cached", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp struct {
		Packages []string `json:"packages"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Packages) != 0 {
		t.Errorf("expected 0 packages, got %d", len(resp.Packages))
	}
}

func TestSitemapCached_WithData(t *testing.T) {
	r, sqliteCache := setupSitemapHandler(t)

	// Seed two package entries.
	if err := sqliteCache.Set("pkg:requests", []byte(`{}`), time.Hour); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := sqliteCache.Set("pkg:flask", []byte(`{}`), time.Hour); err != nil {
		t.Fatalf("set: %v", err)
	}
	// Seed a non-package entry that must not appear.
	if err := sqliteCache.Set("stats:requests:4w", []byte(`{}`), time.Hour); err != nil {
		t.Fatalf("set: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/sitemap/cached", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp struct {
		Packages []string `json:"packages"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Packages) != 2 {
		t.Errorf("expected 2 packages, got %d: %v", len(resp.Packages), resp.Packages)
	}
}

func TestSitemapPopular_ReturnsOK(t *testing.T) {
	r, _ := setupSitemapHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/sitemap/popular", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Empty index returns 200 with empty packages list.
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp struct {
		Packages []string `json:"packages"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Just ensure it's a valid slice (may be empty if no downloads synced).
	if resp.Packages == nil {
		t.Error("packages should be a non-nil slice")
	}
}
```

- [ ] **Step 3: Run the tests**

```bash
cd api && go test ./internal/handler/... -run TestSitemap -v
```

Expected:
```
--- PASS: TestSitemapCached_Empty
--- PASS: TestSitemapCached_WithData
--- PASS: TestSitemapPopular_ReturnsOK
PASS
```

- [ ] **Step 4: Commit**

```bash
git add api/internal/handler/sitemap.go api/internal/handler/sitemap_test.go
git commit -m "feat(api): add sitemap handler with popular and cached endpoints"
```

---

## Task 12: Register sitemap routes in main.go

**Files:**
- Modify: `api/cmd/server/main.go`

- [ ] **Step 1: Wire up the sitemap handler**

In `api/cmd/server/main.go`, after the line:

```go
popularHandler := handler.NewPopularHandler(searchIdx, c)
```

Add:

```go
sitemapHandler := handler.NewSitemapHandler(searchIdx, sqliteCache)
```

Then inside the `r.Group(func(r chi.Router) { ... })` block, after the `r.Get("/api/popular", popularHandler.Get)` line, add:

```go
r.Get("/api/sitemap/popular", sitemapHandler.Popular)
r.Get("/api/sitemap/cached", sitemapHandler.Cached)
```

- [ ] **Step 2: Verify the server builds and endpoints respond**

```bash
cd api && go build ./cmd/server && echo "build OK"
```

Expected: `build OK`

Then test the endpoints:
```bash
cd api && go run ./cmd/server &
sleep 1
curl -s http://localhost:8080/api/sitemap/popular | head -c 200
curl -s http://localhost:8080/api/sitemap/cached | head -c 200
kill %1
```

Expected: both return `{"packages":[...]}` JSON.

- [ ] **Step 3: Run all Go tests to confirm no regressions**

```bash
cd api && go test ./...
```

Expected: all tests pass.

- [ ] **Step 4: Commit**

```bash
git add api/cmd/server/main.go
git commit -m "feat(api): register /api/sitemap/popular and /api/sitemap/cached routes"
```

---

## Task 13: Create Nuxt sitemap server route and configure nuxt.config.ts

**Files:**
- Create: `web/server/api/__sitemap__/urls.ts`
- Modify: `web/nuxt.config.ts`

The server route calls both Go API sitemap endpoints and merges the results into a single response. The `_sitemap` property on each URL tells `@nuxtjs/sitemap` which named sitemap to include it in. URLs in `popular` take priority 1.0; URLs in `cached` that aren't already in `popular` take priority 0.5.

- [ ] **Step 1: Create the server directory**

```bash
mkdir -p web/server/api/__sitemap__
```

- [ ] **Step 2: Create `urls.ts`**

Create `web/server/api/__sitemap__/urls.ts`:

```ts
import { defineSitemapEventHandler } from '#imports'
import type { SitemapUrlInput } from '#sitemap/types'

export default defineSitemapEventHandler(async () => {
  const config = useRuntimeConfig()
  const apiBase = config.apiBase as string

  const [popularRes, cachedRes] = await Promise.allSettled([
    $fetch<{ packages: string[] }>(`${apiBase}/api/sitemap/popular`),
    $fetch<{ packages: string[] }>(`${apiBase}/api/sitemap/cached`),
  ])

  const popularNames = new Set<string>(
    popularRes.status === 'fulfilled' ? popularRes.value.packages : [],
  )
  const cachedNames: string[] =
    cachedRes.status === 'fulfilled' ? cachedRes.value.packages : []

  const urls: SitemapUrlInput[] = []

  for (const name of popularNames) {
    urls.push({
      loc: `/packages/${name}`,
      changefreq: 'daily',
      priority: 1.0,
      _sitemap: 'popular',
    })
  }

  for (const name of cachedNames) {
    if (!popularNames.has(name)) {
      urls.push({
        loc: `/packages/${name}`,
        changefreq: 'weekly',
        priority: 0.5,
        _sitemap: 'cached',
      })
    }
  }

  return urls
})
```

- [ ] **Step 3: Configure sitemap in `nuxt.config.ts`**

In `web/nuxt.config.ts`, add a `sitemap` section after the `ogImage` block:

```ts
  sitemap: {
    sitemaps: {
      popular: {},
      cached: {},
    },
    sources: ['/api/__sitemap__/urls'],
  },
```

The full `nuxt.config.ts` `ogImage` and `sitemap` blocks should look like:

```ts
  ogImage: {
    enabled: true,
  },

  sitemap: {
    sitemaps: {
      popular: {},
      cached: {},
    },
    sources: ['/api/__sitemap__/urls'],
  },
```

- [ ] **Step 4: Verify sitemap renders**

Start the dev server with the Go API also running:

```bash
# Terminal 1
cd api && go run ./cmd/server

# Terminal 2  
cd web && npm run dev
```

Then fetch the sitemaps:

```bash
curl http://localhost:3000/sitemap_index.xml
curl http://localhost:3000/popular-sitemap.xml | head -50
curl http://localhost:3000/cached-sitemap.xml | head -50
```

Expected:
- `sitemap_index.xml` lists both `popular-sitemap.xml` and `cached-sitemap.xml`
- `popular-sitemap.xml` contains `/packages/{name}` entries with `priority=1.0`
- `cached-sitemap.xml` contains entries with `priority=0.5`

If the Go API is not running, the sitemap should return empty package lists gracefully (the `Promise.allSettled` handles failures).

- [ ] **Step 5: Commit**

```bash
git add web/server/api/__sitemap__/urls.ts web/nuxt.config.ts
git commit -m "feat(web): add dynamic sitemap with popular and cached package sitemaps"
```

---

## Task 14: Final verification and PR prep

- [ ] **Step 1: Run all Go tests**

```bash
cd api && go test ./...
```

Expected: all pass.

- [ ] **Step 2: Run Nuxt linter and formatter**

```bash
cd web && npm run lint && npm run fmt:check
```

Fix any issues before continuing.

- [ ] **Step 3: Build Nuxt for production**

```bash
cd web && npm run build
```

Expected: build completes without errors. Note any warnings about missing OG image components — resolve if present.

- [ ] **Step 4: Spot-check OG images in the playground**

With `npm run dev` running, visit `http://localhost:3000/__og_image__/` and verify:
- Home page: SiteCard renders (pypx logo, grid, glow)
- A package page: PackageCard renders (name, version badge, summary)
- A docs page: DocsCard renders ("API Docs" badge, name, version)

- [ ] **Step 5: Verify OG meta tags in page source**

```bash
curl -s http://localhost:3000/packages/requests | grep -E 'og:image|og:title|application/ld\+json' | head -10
```

Expected: `og:image` pointing to `/_ipx/...` or `/og-image/...` URL, `og:title` with package name, a `<script type="application/ld+json">` block.

- [ ] **Step 6: Final commit and push**

```bash
git push -u origin feat/seo-og-images
```

---

## Notes

- **Font:** OG templates use `system-ui` (the browser default in the rendering environment). If you want Geist, add `@font-face` loading to the nuxt.config.ts `ogImage.fonts` config — consult the nuxt-og-image v6 docs for the exact syntax as it varies by version.
- **`search.PackageEntry.Name`:** Confirmed as `Name` (`api/internal/search/index.go:13`).
- **Popular limit:** `TopByDownloads(5000)` — there's no cap at this layer. If fewer than 5000 packages have synced downloads, the list will be shorter.
- **`useSchemaOrg` reactivity:** Wrapping the schema object in `computed()` ensures it updates when `pkg` loads. If `useSchemaOrg` doesn't accept a computed directly, wrap in `watch` instead — consult nuxt-schema-org v6 docs.
