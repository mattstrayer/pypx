# pypx UX Overhaul — Design Spec

**Date:** 2026-04-19  
**Branch:** feat.ux-overhaul  
**Scope:** One PR addressing all 14 UX audit findings

---

## Background

A UX audit identified 14 issues across four categories: functional breakages, design system inconsistencies, page-level experience gaps, and missing delight. This spec covers all of them in a single implementation pass.

---

## 1. Functional Fixes

### 1.1 Header Enter key navigates to package

**File:** `web/app/components/AppHeader.vue`

The header `<form>` has `@submit.prevent` but no submit handler — pressing Enter does nothing. Fix: on form submit, if a typeahead result is highlighted navigate to that result; otherwise navigate to the first result in the list. If the query is empty, do nothing.

```
form @submit.prevent="onSubmit"

function onSubmit() {
  const target = results[selectedIndex] ?? results[0]
  if (target) navigateToResult(target)
}
```

### 1.2 Delete `/search` page

**File:** `web/app/pages/search.vue` — delete entirely.

The dedicated search page duplicated the header typeahead with no additional value. All search flows through the header typeahead. Any inbound links to `/search?q=...` will 404 gracefully (Nuxt default 404 page).

### 1.3 Active tab bottom border

**File:** `web/app/pages/packages/[name].vue`

Active tab currently uses `bg-raised text-primary` with no bottom border, while hover shows a colored border. Fix: active state gets `border-b-2 border-[var(--color-brand)]`; hover becomes `hover:border-b-2 hover:border-[var(--color-brand)]/40` (slightly more subtle than active).

---

## 2. Design System Consistency

### 2.1 Sidebar design tokens — light mode fix

**File:** `web/app/components/PackageOverview.vue`

The lower sidebar sections (GitHub, Release Cadence, Doc link, Platform, Maintainers) use hardcoded `border-neutral-800`, `text-neutral-400`, `text-neutral-300` — dark-only colors that are invisible in light mode. Replace all with design tokens:

| Hardcoded | Token replacement |
|---|---|
| `border-neutral-800` | `border-subtle` |
| `text-neutral-400` | `text-muted` |
| `text-neutral-300` | `text-primary` |
| `text-neutral-500` | `text-muted` |

### 2.2 Sidebar visual structure (Option B)

**File:** `web/app/components/PackageOverview.vue`

Keep the two-tier sidebar pattern but fix consistency:
- **Details card** — stays as `rounded-lg border border-subtle bg-surface p-4`
- **Links card** — stays as `rounded-lg border border-subtle bg-surface p-4`
- **All sections below** (GitHub, Release Cadence, Doc link, Platform, Maintainers) — use `border-t border-subtle pt-3 mt-3` dividers, no card chrome. These are supplementary info, lighter visual weight is intentional.

### 2.3 Docs active symbol — brand color

**File:** `web/app/components/docs/DocsSidebar.vue`

Active symbol highlight uses `bg-blue-500/15 border-l-2 border-blue-500 text-white` — the only place blue appears in the app. Replace with brand green:

```
bg-[var(--color-brand-muted)] border-l-2 border-[var(--color-brand)] text-primary
```

### 2.4 Remove tautological "Description" section label

**File:** `web/app/components/PackageOverview.vue`

The card renders `DESCRIPTION` as a heading then shows the description. Remove the heading — the prose content is self-evident. The card border already groups it visually.

### 2.5 kbd hint consistency

**File:** `web/app/pages/index.vue`

The homepage search box shows `⌘K` as the kbd hint. The header shows `/`. Both refer to "focus search." Standardise to `/` everywhere since that's the actual keyboard shortcut registered in the app.

---

## 3. Page-Level Experience

### 3.1 Homepage restructure

**File:** `web/app/pages/index.vue`

New homepage section order:

1. **Hero** — unchanged (pypx heading, tagline, search box)
2. **Feature callout strip** — new static section, 3 columns:
   - **Dependency Insights** — "See required deps, optional extras, and Python version constraints at a glance"
   - **Download Trends** — "Weekly and monthly download stats with historical charts"
   - **API Docs** — "Browse extracted API docs from published wheels — no external docs site needed"
   - Style: `grid grid-cols-1 sm:grid-cols-3 gap-4`, each cell has a small icon, bold label, one-line description. Subtle `border border-subtle rounded-lg p-4 bg-surface` card per item.
3. **Trending This Week** — replaces "Popular Packages". Calls new `/api/trending` endpoint (see 3.2). Grid layout unchanged (`TrendingPackages` component reused).

### 3.2 New API endpoint: `/api/trending`

**File:** `api/internal/handler/` (new handler) + `api/cmd/server/main.go` (route registration)

Returns packages sorted by recent download velocity (7-day download growth rate) rather than all-time total. Implementation options in priority order:
1. Query pypistats.org for recent download delta if available
2. Fallback: sort top-N packages by `recent_downloads / total_downloads` ratio as a proxy for trending
3. Cache key: `trending` with 1-hour TTL

The frontend `useApi` composable gets a new `fetchTrending(limit)` method alongside the existing `fetchPopular`.

### 3.3 Footer — all pages

**File:** `web/app/layouts/default.vue`

Add a slim footer inside the layout's root `<div>`, after `<main>`. The root div already has `min-h-screen` — add `flex flex-col` and `mt-auto` on the footer so it's pushed to the bottom on short pages:

```html
<footer class="mt-auto border-t border-subtle py-4">
  <div class="mx-auto flex max-w-6xl items-center justify-between px-4 text-xs text-muted">
    <span>pypx — not affiliated with PyPI or the PSF</span>
    <div class="flex gap-4">
      <a href="https://github.com/mattstrayer/pypx" ...>GitHub</a>
      <a href="https://pypi.org" ...>PyPI</a>
    </div>
  </div>
</footer>
```

### 3.4 Package page skeleton loader

**File:** `web/app/pages/packages/[name].vue`

Replace the centered spinner during package load with a skeleton that mirrors the two-column layout:

- Left column: h1 block (`h-8 w-48`), badge row (`h-5 w-full`), tab strip (`h-10 w-full`), content card (`h-64 w-full`)
- Right column: details card (`h-40 w-full`), links card (`h-24 w-full`)

Use `animate-pulse rounded bg-raised/50` on each block. Same pattern already used on the homepage popular packages skeleton.

---

## 4. Docs Page Reconnection

### 4.1 Restore global header on docs route

**File:** `web/app/pages/packages/[name]/docs.vue`

Currently the docs page renders its own isolated layout with no `AppHeader`. Add `AppHeader` at the top of the docs page layout — it provides the pypx logo, global search, and theme toggle, keeping the user oriented within the main app.

The docs layout is full-width (sidebar + content), so `AppHeader` should use its standard `max-w-6xl` container but the docs content below it can break out to full-width as it does today.

### 4.2 Slim docs context bar

**File:** `web/app/pages/packages/[name]/docs.vue`

Add a secondary bar directly below `AppHeader`, above the docs sidebar+content area:

```html
<div class="border-b border-subtle bg-base/80 backdrop-blur-sm px-4 py-2 flex items-center gap-3 text-sm">
  <NuxtLink :to="`/packages/${name}`" class="text-[var(--color-brand)] hover:text-[var(--color-brand-light)]">
    ← {{ name }}
  </NuxtLink>
  <span class="text-muted">·</span>
  <span class="font-mono text-xs text-muted">v{{ version }}</span>
  <span class="text-muted truncate hidden sm:block">{{ summary }}</span>
  <button class="ml-auto ..." @click="openPalette">Jump to symbol ⌘K</button>
</div>
```

The existing bespoke docs header (which currently lives inside the main content area) is removed and replaced by this bar.

---

## Out of Scope (deferred)

- **Breadcrumb / back-to-search** — not needed since `/search` page is deleted. Header logo provides home navigation.
- **Search result descriptions** — tied to the deleted `/search` page. Typeahead already shows summaries when available.
- **Docs mobile navigation** — sidebar is `hidden md:flex` with ⌘K as fallback; mobile UX improvement deferred.

---

## Files Changed Summary

| File | Change |
|---|---|
| `web/app/components/AppHeader.vue` | Add form submit handler |
| `web/app/pages/search.vue` | **Delete** |
| `web/app/pages/packages/[name].vue` | Skeleton loader, active tab border fix |
| `web/app/components/PackageOverview.vue` | Token fixes, sidebar structure, remove Description label |
| `web/app/components/docs/DocsSidebar.vue` | Brand color for active symbol |
| `web/app/pages/index.vue` | Feature callout, trending section, kbd hint fix |
| `web/app/pages/packages/[name]/docs.vue` | AppHeader + context bar |
| `web/app/composables/useApi.ts` | Add `fetchTrending()` |
| `web/app/layouts/default.vue` | Footer, flex-col layout |
| `api/internal/handler/trending.go` | **New** trending endpoint |
| `api/cmd/server/main.go` | Register `/api/trending` route |
