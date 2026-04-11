# Dynamic Popular Packages Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the hardcoded "Popular Packages" homepage section with a live `/api/popular` endpoint backed by the existing SQLite search index download counts, expanding from 6 to 12 packages.

**Architecture:** Add `TopByDownloads` to the search index, wire it into a new `PopularHandler` with 6-hour cache, register the route, then update the homepage to fetch from it with a skeleton loading state.

**Tech Stack:** Go (chi router, database/sql, SQLite), Vue 3 (script setup, useFetch), Nuxt 3

---

### Task 1: Add `TopByDownloads` to the search index

**Files:**
- Modify: `api/internal/search/index.go` (after `Search` method, ~line 194)
- Modify: `api/internal/search/index_test.go` (after `TestSearchEmpty`, ~line 325)

- [ ] **Step 1: Write the failing test**

Add to `api/internal/search/index_test.go` after `TestSearchEmpty`:

```go
// TestTopByDownloads verifies that the top packages are returned in descending
// download order, filtered to only packages with downloads > 0.
func TestTopByDownloads(t *testing.T) {
	idx := mustNewIndex(t)

	packages := []PackageEntry{
		{Name: "numpy", Summary: "Scientific computing", Downloads: 80_000_000},
		{Name: "requests", Summary: "HTTP for Humans", Downloads: 50_000_000},
		{Name: "obscure-pkg", Summary: "Not in top list", Downloads: 0},
		{Name: "flask", Summary: "A micro web framework", Downloads: 30_000_000},
	}
	if err := idx.UpsertBatch(packages); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}

	results, err := idx.TopByDownloads(3)
	if err != nil {
		t.Fatalf("TopByDownloads: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Name != "numpy" {
		t.Errorf("expected first result 'numpy', got %q", results[0].Name)
	}
	if results[1].Name != "requests" {
		t.Errorf("expected second result 'requests', got %q", results[1].Name)
	}
	if results[2].Name != "flask" {
		t.Errorf("expected third result 'flask', got %q", results[2].Name)
	}
	// obscure-pkg must not appear (downloads = 0)
	for _, r := range results {
		if r.Name == "obscure-pkg" {
			t.Errorf("expected 'obscure-pkg' (downloads=0) to be excluded")
		}
	}
}

// TestTopByDownloads_Empty verifies an empty result set when no packages have
// downloads > 0.
func TestTopByDownloads_Empty(t *testing.T) {
	idx := mustNewIndex(t)

	if err := idx.UpsertBatch([]PackageEntry{
		{Name: "foo", Summary: "A package", Downloads: 0},
	}); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}

	results, err := idx.TopByDownloads(10)
	if err != nil {
		t.Fatalf("TopByDownloads: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd /Users/matt/dev/pypx/api && go test ./internal/search/... -run TestTopByDownloads -v
```

Expected: compile error — `idx.TopByDownloads undefined`

- [ ] **Step 3: Implement `TopByDownloads` in the search index**

Add this method to `api/internal/search/index.go` after the `Search` method (before `UpdateDownloadsBatch`):

```go
// TopByDownloads returns the top limit packages ordered by downloads descending,
// excluding packages with zero downloads (those not yet synced from the top
// packages dataset).
func (idx *Index) TopByDownloads(limit int) ([]PackageEntry, error) {
	if limit <= 0 {
		limit = 12
	}

	rows, err := idx.db.Query(`
		SELECT name, summary, downloads
		FROM packages_meta
		WHERE downloads > 0
		ORDER BY downloads DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("search: top by downloads: %w", err)
	}
	defer rows.Close()

	var results []PackageEntry
	for rows.Next() {
		var e PackageEntry
		if err := rows.Scan(&e.Name, &e.Summary, &e.Downloads); err != nil {
			return nil, fmt.Errorf("search: top by downloads scan: %w", err)
		}
		results = append(results, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search: top by downloads rows: %w", err)
	}
	if results == nil {
		results = []PackageEntry{}
	}
	return results, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
cd /Users/matt/dev/pypx/api && go test ./internal/search/... -run TestTopByDownloads -v
```

Expected: `PASS` for both `TestTopByDownloads` and `TestTopByDownloads_Empty`

- [ ] **Step 5: Run the full search test suite to check for regressions**

```bash
cd /Users/matt/dev/pypx/api && go test ./internal/search/... -v
```

Expected: all tests pass

- [ ] **Step 6: Commit**

```bash
cd /Users/matt/dev/pypx && git add api/internal/search/index.go api/internal/search/index_test.go
git commit -m "feat(search): add TopByDownloads method to search index"
```

---

### Task 2: Add `PopularHandler`

**Files:**
- Create: `api/internal/handler/popular.go`
- Create: `api/internal/handler/popular_test.go`

- [ ] **Step 1: Write the failing test**

Create `api/internal/handler/popular_test.go`:

```go
package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/search"
)

func mustPopularIndex(t *testing.T) *search.Index {
	t.Helper()
	idx, err := search.NewIndex(":memory:")
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	t.Cleanup(func() { idx.Close() })
	return idx
}

func mustPopularCache(t *testing.T) cache.Cacher {
	t.Helper()
	sqliteCache, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	mc := cache.NewMemoryCache(sqliteCache, 100)
	t.Cleanup(func() { mc.Close() })
	return mc
}

func TestPopular_ReturnsTopPackages(t *testing.T) {
	idx := mustPopularIndex(t)
	c := mustPopularCache(t)

	packages := []search.PackageEntry{
		{Name: "numpy", Summary: "Scientific computing", Downloads: 80_000_000},
		{Name: "requests", Summary: "HTTP for Humans", Downloads: 50_000_000},
		{Name: "flask", Summary: "A micro web framework", Downloads: 30_000_000},
	}
	if err := idx.UpsertBatch(packages); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}
	if err := idx.UpdateDownloadsBatch(packages); err != nil {
		t.Fatalf("UpdateDownloadsBatch: %v", err)
	}

	h := handler.NewPopularHandler(idx, c)

	req := httptest.NewRequest(http.MethodGet, "/api/popular?limit=3", nil)
	w := httptest.NewRecorder()
	h.Get(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	var results []search.PackageEntry
	if err := json.NewDecoder(res.Body).Decode(&results); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Name != "numpy" {
		t.Errorf("expected first result 'numpy', got %q", results[0].Name)
	}
}

func TestPopular_DefaultLimit(t *testing.T) {
	idx := mustPopularIndex(t)
	c := mustPopularCache(t)

	// Insert 15 packages with non-zero downloads.
	packages := make([]search.PackageEntry, 15)
	for i := range packages {
		packages[i] = search.PackageEntry{
			Name:      "pkg-" + string(rune('a'+i)),
			Summary:   "A package",
			Downloads: int64(1000 * (15 - i)),
		}
	}
	if err := idx.UpsertBatch(packages); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}
	if err := idx.UpdateDownloadsBatch(packages); err != nil {
		t.Fatalf("UpdateDownloadsBatch: %v", err)
	}

	h := handler.NewPopularHandler(idx, c)

	// No limit param — should default to 12.
	req := httptest.NewRequest(http.MethodGet, "/api/popular", nil)
	w := httptest.NewRecorder()
	h.Get(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	var results []search.PackageEntry
	if err := json.NewDecoder(res.Body).Decode(&results); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(results) != 12 {
		t.Errorf("expected 12 results (default), got %d", len(results))
	}
}

func TestPopular_LimitClamped(t *testing.T) {
	idx := mustPopularIndex(t)
	c := mustPopularCache(t)

	h := handler.NewPopularHandler(idx, c)

	req := httptest.NewRequest(http.MethodGet, "/api/popular?limit=999", nil)
	w := httptest.NewRecorder()
	h.Get(w, req)

	// Should not error — limit is clamped to 50 internally.
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Result().StatusCode)
	}
}

func TestPopular_EmptyIndex(t *testing.T) {
	idx := mustPopularIndex(t)
	c := mustPopularCache(t)

	h := handler.NewPopularHandler(idx, c)

	req := httptest.NewRequest(http.MethodGet, "/api/popular", nil)
	w := httptest.NewRecorder()
	h.Get(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	var results []search.PackageEntry
	if err := json.NewDecoder(res.Body).Decode(&results); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty array, got %d results", len(results))
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd /Users/matt/dev/pypx/api && go test ./internal/handler/... -run TestPopular -v
```

Expected: compile error — `handler.NewPopularHandler undefined`

- [ ] **Step 3: Implement `PopularHandler`**

Create `api/internal/handler/popular.go`:

```go
package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/search"
)

const popularTTL = 6 * time.Hour

// PopularHandler serves the top packages by download count.
type PopularHandler struct {
	index *search.Index
	cache cache.Cacher
}

// NewPopularHandler creates a new PopularHandler.
func NewPopularHandler(idx *search.Index, c cache.Cacher) *PopularHandler {
	return &PopularHandler{index: idx, cache: c}
}

// Get handles GET /api/popular?limit=12.
func (h *PopularHandler) Get(w http.ResponseWriter, r *http.Request) {
	limit := 12
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}
	if limit <= 0 {
		limit = 12
	}
	if limit > 50 {
		limit = 50
	}

	cacheKey := fmt.Sprintf("popular:%d", limit)

	if data, _, err := h.cache.Get(cacheKey, popularTTL); err == nil && data != nil {
		w.Header().Set("Cache-Control", "public, max-age=21600")
		w.Header().Set("Content-Type", "application/json")
		w.Write(data) //nolint:errcheck
		return
	}

	results, err := h.index.TopByDownloads(limit)
	if err != nil {
		log.Printf("popular: TopByDownloads error: %v", err)
		http.Error(w, "failed to fetch popular packages", http.StatusInternalServerError)
		return
	}

	encoded, err := json.Marshal(results)
	if err != nil {
		log.Printf("popular: json.Marshal error: %v", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	h.cache.Set(cacheKey, encoded, popularTTL) //nolint:errcheck

	w.Header().Set("Cache-Control", "public, max-age=21600")
	w.Header().Set("Content-Type", "application/json")
	w.Write(encoded) //nolint:errcheck
}
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
cd /Users/matt/dev/pypx/api && go test ./internal/handler/... -run TestPopular -v
```

Expected: all four `TestPopular_*` tests pass

- [ ] **Step 5: Run the full handler test suite**

```bash
cd /Users/matt/dev/pypx/api && go test ./internal/handler/... -v
```

Expected: all tests pass

- [ ] **Step 6: Commit**

```bash
cd /Users/matt/dev/pypx && git add api/internal/handler/popular.go api/internal/handler/popular_test.go
git commit -m "feat(api): add PopularHandler for top packages by downloads"
```

---

### Task 3: Register the route in `main.go`

**Files:**
- Modify: `api/cmd/server/main.go`

- [ ] **Step 1: Register the route**

In `api/cmd/server/main.go`, add the popular handler after the search handler is created (around line 55):

```go
popularHandler := handler.NewPopularHandler(searchIdx, c)
```

Then add the route after `r.Get("/api/search", searchHandler.Search)`:

```go
r.Get("/api/popular", popularHandler.Get)
```

- [ ] **Step 2: Build to verify no compile errors**

```bash
cd /Users/matt/dev/pypx/api && go build ./...
```

Expected: exits with no output (success)

- [ ] **Step 3: Commit**

```bash
cd /Users/matt/dev/pypx && git add api/cmd/server/main.go
git commit -m "feat(api): register GET /api/popular route"
```

---

### Task 4: Add `fetchPopular` to the web API composable

**Files:**
- Modify: `web/app/composables/useApi.ts`

- [ ] **Step 1: Add `fetchPopular` to `useApi`**

In `web/app/composables/useApi.ts`, add the import for `SearchResult` (already imported). Add this function inside `useApi()` after `searchPackages`:

```ts
async function fetchPopular(limit = 12): Promise<SearchResult[]> {
  return $fetch<SearchResult[]>(`${baseURL}/popular`, {
    params: { limit },
  });
}
```

And add it to the return object:

```ts
return {
  fetchPackage,
  fetchVersions,
  fetchDependencies,
  fetchStats,
  searchPackages,
  fetchPopular,
  fetchChangelog,
};
```

- [ ] **Step 2: Commit**

```bash
cd /Users/matt/dev/pypx && git add web/app/composables/useApi.ts
git commit -m "feat(web): add fetchPopular to useApi composable"
```

---

### Task 5: Wire the homepage to live data with skeleton loading

**Files:**
- Modify: `web/app/pages/index.vue`

- [ ] **Step 1: Replace hardcoded data with live fetch + skeleton + error state**

Replace the entire contents of `web/app/pages/index.vue` with:

```vue
<script setup lang="ts">
const searchQuery = ref("");
const router = useRouter();

function onSearch() {
  if (searchQuery.value.trim()) {
    router.push({ path: "/search", query: { q: searchQuery.value.trim() } });
  }
}

const config = useRuntimeConfig();
const { data: popularPackages, status } = await useFetch<
  Array<{ name: string; summary: string; downloads: number }>
>(`${config.public.apiBase}/popular`, {
  params: { limit: 12 },
});

useSeoMeta({
  title: "pypx — A modern PyPI frontend",
  description:
    "Explore Python packages with enriched insights, fast search, dependency analysis, and download trends.",
  ogTitle: "pypx — A modern PyPI frontend",
  ogDescription:
    "Explore Python packages with enriched insights, fast search, dependency analysis, and download trends.",
});
</script>

<template>
  <div>
    <section class="flex flex-col items-center pt-16 pb-12 text-center">
      <h1 class="text-5xl font-bold tracking-tight text-zinc-50">pypx</h1>
      <p class="mt-3 max-w-lg text-lg text-zinc-400">
        The Python Package Index, reimagined. Fast search, dependency insights, and download trends
        — all in one place.
      </p>
      <form class="mt-8 w-full max-w-xl" @submit.prevent="onSearch">
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Search 500,000+ Python packages..."
          class="w-full rounded-lg border border-zinc-800 bg-zinc-900 px-4 py-3 text-zinc-50 placeholder-zinc-500 outline-none focus:border-zinc-600 focus:ring-1 focus:ring-zinc-600"
        />
      </form>
    </section>

    <section class="pb-16">
      <h2 class="mb-4 text-sm font-medium uppercase tracking-wider text-zinc-500">
        Popular Packages
      </h2>

      <!-- Skeleton loading state -->
      <div v-if="status === 'pending'" class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <div
          v-for="i in 12"
          :key="i"
          class="h-20 animate-pulse rounded-lg border border-zinc-800 bg-zinc-800/50"
        />
      </div>

      <!-- Error state -->
      <p v-else-if="status === 'error'" class="text-sm text-zinc-500">
        Could not load popular packages.
      </p>

      <!-- Data -->
      <TrendingPackages v-else-if="popularPackages?.length" :packages="popularPackages" />
    </section>
  </div>
</template>
```

- [ ] **Step 2: Start the dev server and verify the homepage loads**

```bash
cd /Users/matt/dev/pypx/web && pnpm dev
```

Open `http://localhost:3000` in a browser. Verify:
- The skeleton grid (12 grey cards) briefly appears while fetching
- Real package names and download counts appear once loaded
- The grid shows 4 rows × 3 columns (12 packages)
- Clicking a package card navigates to its package page

- [ ] **Step 3: Commit**

```bash
cd /Users/matt/dev/pypx && git add web/app/pages/index.vue
git commit -m "feat(web): wire homepage popular packages to live API data"
```
