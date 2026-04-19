# pypx UX Overhaul Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Address all 14 UX audit findings: functional bugs, design-system inconsistencies, page-level experience gaps, and docs reconnection — in one cohesive PR.

**Architecture:** Pure frontend + minor layout changes. No new API endpoints needed — the existing `/api/popular` already returns 30-day download data, so "Trending" is a label rename. All design token fixes replace hardcoded `neutral-*` colors with CSS custom property tokens already defined in `main.css`. Docs reconnection is achieved by restyling the existing inline context section into a proper full-width bar.

**Tech Stack:** Vue 3 (Composition API, `<script setup>`), Nuxt 4, Tailwind 4, CSS custom properties (`--color-*` tokens). Dev server: `pnpm dev` from `web/`. Tests: `pnpm test` (Vitest) from `web/`.

---

## File Map

| File | Change |
|---|---|
| `web/app/components/AppHeader.vue` | Add `onSubmit` handler to form |
| `web/app/pages/search.vue` | **Delete** |
| `web/app/pages/packages/[name].vue` | Active tab border + skeleton loader |
| `web/app/components/PackageOverview.vue` | Token fixes, sidebar structure, remove Description label |
| `web/app/components/docs/DocsSidebar.vue` | Brand color for active symbol |
| `web/app/pages/index.vue` | kbd hint, feature callout, rename Popular→Trending |
| `web/app/layouts/default.vue` | Footer + `flex flex-col` |
| `web/app/pages/packages/[name]/docs.vue` | Restyle context section as proper bar |

---

## Task 1: Delete /search page + fix header Enter key

**Files:**
- Delete: `web/app/pages/search.vue`
- Modify: `web/app/components/AppHeader.vue`

The header `<form>` has `@submit.prevent` with no handler — pressing Enter does nothing. Fix: navigate to the highlighted or first typeahead result on submit. Then delete the now-redundant search page.

- [ ] **Step 1: Delete the search page**

```bash
rm web/app/pages/search.vue
```

- [ ] **Step 2: Add `onSubmit` to AppHeader**

Open `web/app/components/AppHeader.vue`. In the `<script setup>` block, after the `close` destructure, add:

```typescript
function onSubmit() {
  const target = results.value[selectedIndex.value] ?? results.value[0];
  if (target) {
    navigateToResult(target);
    close();
  }
}
```

In the template, change the form opening tag from:

```html
<form @submit.prevent>
```

to:

```html
<form @submit.prevent="onSubmit">
```

- [ ] **Step 3: Verify dev server compiles without errors**

```bash
cd web && pnpm dev
```

Navigate to `http://localhost:3000`. Type "requests" in the header search box. Press Enter. Expected: navigates to `/packages/requests`. Navigate to `http://localhost:3000/search` — expected: 404 page.

- [ ] **Step 4: Commit**

```bash
git add web/app/components/AppHeader.vue
git rm web/app/pages/search.vue
git commit -m "feat: header Enter navigates to package, remove redundant /search page"
```

---

## Task 2: Active tab bottom border indicator

**Files:**
- Modify: `web/app/pages/packages/[name].vue`

The active tab has no bottom border (hover has one, active doesn't — backwards). The base class already includes `border-b-2 border-transparent`; the active state just needs to set the border color.

- [ ] **Step 1: Fix the active tab class binding**

In `web/app/pages/packages/[name].vue`, find the `:class` binding on the tab `<button>` (around line 146). Change:

```typescript
activeTab === tab.key
  ? 'bg-raised text-primary'
  : 'text-zinc-700 dark:text-zinc-300 hover:border-[rgba(4,120,87,0.65)] dark:hover:border-[rgba(74,222,128,0.65)]'
```

to:

```typescript
activeTab === tab.key
  ? 'bg-raised text-primary border-[var(--color-brand)]'
  : 'text-zinc-700 dark:text-zinc-300 hover:border-[rgba(4,120,87,0.65)] dark:hover:border-[rgba(74,222,128,0.65)]'
```

- [ ] **Step 2: Verify visually**

Run dev server (`cd web && pnpm dev`). Navigate to `http://localhost:3000/packages/requests`. Confirm the active tab has a green bottom border. Switch tabs — confirm the border follows the active tab.

- [ ] **Step 3: Commit**

```bash
git add web/app/pages/packages/[name].vue
git commit -m "fix: active tab shows bottom border indicator"
```

---

## Task 3: Sidebar design token fixes + remove Description label

**Files:**
- Modify: `web/app/components/PackageOverview.vue`

Two problems: (a) hardcoded `neutral-*` colors in the lower sidebar sections break light mode; (b) the "DESCRIPTION" card label is tautological. Fix both.

- [ ] **Step 1: Remove the Description card label**

In `web/app/components/PackageOverview.vue`, find and delete this line (inside the description card, around line 39):

```html
<h2 class="mb-3 text-sm font-semibold uppercase tracking-wider text-muted">Description</h2>
```

- [ ] **Step 2: Fix all hardcoded dark colors in the sidebar**

In the same file, replace all occurrences of hardcoded dark-only colors with design tokens. Make these replacements throughout the entire file:

| Find | Replace with |
|---|---|
| `border-neutral-800` | `border-subtle` |
| `text-neutral-400` | `text-muted` |
| `text-neutral-300` | `text-primary` |
| `text-neutral-500` | `text-muted` |

There are approximately 8–10 occurrences across the GitHub, Release Cadence, Doc link, Platform, and Maintainers sections. Use find-and-replace to catch all of them.

- [ ] **Step 3: Verify in both light and dark mode**

Run dev server. Navigate to `http://localhost:3000/packages/requests`. Click the theme toggle (moon/sun icon in the header) to switch to light mode. Confirm all sidebar sections are visible and readable — particularly the GitHub stats, Release Cadence, and Maintainers sections. Switch back to dark and confirm nothing broke.

- [ ] **Step 4: Commit**

```bash
git add web/app/components/PackageOverview.vue
git commit -m "fix: sidebar design tokens for light mode, remove tautological Description label"
```

---

## Task 4: Docs sidebar active symbol — brand color

**Files:**
- Modify: `web/app/components/docs/DocsSidebar.vue`

The active symbol highlight uses hardcoded blue (`bg-blue-500/15 border-l-2 border-blue-500 text-white`) — the only blue in the app. Replace with the emerald brand color.

- [ ] **Step 1: Replace blue with brand color in the symbol row class binding**

In `web/app/components/docs/DocsSidebar.vue`, find the symbol row `<button>` `:class` binding (around line 106). Change:

```typescript
activeSymbol === item.name
  ? 'bg-blue-500/15 border-l-2 border-blue-500 text-white pl-2.5'
  : 'text-zinc-400 hover:bg-zinc-800/50 hover:text-zinc-200'
```

to:

```typescript
activeSymbol === item.name
  ? 'bg-[var(--color-brand-muted)] border-l-2 border-[var(--color-brand)] text-primary pl-2.5'
  : 'text-zinc-400 hover:bg-zinc-800/50 hover:text-zinc-200'
```

- [ ] **Step 2: Run the DocsSidebar tests to verify no regressions**

```bash
cd web && pnpm test web/app/components/docs/__tests__/DocsSidebar.test.ts
```

Expected: all tests pass (the test exercises click/keyboard behavior, not colors, so all should pass).

- [ ] **Step 3: Verify visually**

Navigate to `http://localhost:3000/packages/numpy/docs`. Click a function in the sidebar. Confirm the active symbol shows a green left border and green-tinted background (not blue). Switch to light mode and confirm it remains readable.

- [ ] **Step 4: Commit**

```bash
git add web/app/components/docs/DocsSidebar.vue
git commit -m "fix: docs sidebar active symbol uses brand color instead of blue"
```

---

## Task 5: Homepage — kbd hint, feature callout, rename Popular to Trending

**Files:**
- Modify: `web/app/pages/index.vue`

Three changes: (1) fix kbd hint from `⌘K` to `/`; (2) add a 3-column feature callout strip; (3) rename "Popular Packages" to "Trending" (the API already returns 30-day download data — this is a label change only).

- [ ] **Step 1: Fix the kbd hint**

In `web/app/pages/index.vue`, find the `<kbd>` element in the hero search box. Change:

```html
<kbd
  class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 hidden rounded bg-raised px-1.5 py-0.5 font-mono text-[10px] text-muted sm:inline"
>
  ⌘K
</kbd>
```

to:

```html
<kbd
  class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 hidden rounded bg-raised px-1.5 py-0.5 font-mono text-[10px] text-muted sm:inline"
>
  /
</kbd>
```

- [ ] **Step 2: Rename "Popular Packages" to "Trending"**

Change the section heading (around line where `POPULAR_LIMIT` is referenced):

```html
<h2 class="mb-4 text-sm font-medium uppercase tracking-wider text-muted">Popular Packages</h2>
```

to:

```html
<h2 class="mb-4 text-sm font-medium uppercase tracking-wider text-muted">Trending</h2>
```

Also rename the `useAsyncData` key and destructured variable:

```typescript
const { data: trendingPackages, status } = await useAsyncData("trending", () =>
  api.fetchPopular(POPULAR_LIMIT),
);
```

Then update the three template references that use `popularPackages` to use `trendingPackages`:

```html
<!-- was: v-else-if="popularPackages?.length" :packages="popularPackages" -->
<TrendingPackages v-else-if="trendingPackages?.length" :packages="trendingPackages" />
```

- [ ] **Step 3: Add feature callout strip between hero and trending sections**

In the template, insert the following section between `</section>` (hero close) and `<section class="pb-16">` (trending section open):

```html
<!-- Feature callout strip -->
<section class="pb-10">
  <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
    <div class="rounded-lg border border-subtle bg-surface p-4">
      <div class="mb-2 flex items-center gap-2">
        <svg class="h-4 w-4 text-[var(--color-brand)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M4 7h16M4 12h8m-8 5h16" />
        </svg>
        <span class="text-sm font-semibold text-primary">Dependency Insights</span>
      </div>
      <p class="text-sm text-muted">Required deps, optional extras, and Python version constraints at a glance.</p>
    </div>
    <div class="rounded-lg border border-subtle bg-surface p-4">
      <div class="mb-2 flex items-center gap-2">
        <svg class="h-4 w-4 text-[var(--color-brand)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M3 17l4-8 4 4 4-6 4 3" />
        </svg>
        <span class="text-sm font-semibold text-primary">Download Trends</span>
      </div>
      <p class="text-sm text-muted">Weekly and monthly download stats with historical charts.</p>
    </div>
    <div class="rounded-lg border border-subtle bg-surface p-4">
      <div class="mb-2 flex items-center gap-2">
        <svg class="h-4 w-4 text-[var(--color-brand)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M9 12h6m-6 4h6m-7-8h8a2 2 0 012 2v10a2 2 0 01-2 2H7a2 2 0 01-2-2V10a2 2 0 012-2z" />
        </svg>
        <span class="text-sm font-semibold text-primary">API Docs</span>
      </div>
      <p class="text-sm text-muted">Browse extracted API docs from published wheels — no external docs site needed.</p>
    </div>
  </div>
</section>
```

- [ ] **Step 4: Verify visually**

Run dev server. Navigate to `http://localhost:3000`. Confirm: (a) the kbd hint shows `/`; (b) the 3-column feature strip appears between the hero and the package grid; (c) the section is labelled "Trending" not "Popular Packages"; (d) responsive — on narrow viewport the 3 columns stack vertically.

- [ ] **Step 5: Commit**

```bash
git add web/app/pages/index.vue
git commit -m "feat: homepage feature callout strip, rename Popular to Trending, fix kbd hint"
```

---

## Task 6: Footer + flex-col layout

**Files:**
- Modify: `web/app/layouts/default.vue`

Add a micro footer to all pages. Make the root div `flex flex-col` so short pages push the footer to the bottom.

- [ ] **Step 1: Update the layout**

Replace the entire content of `web/app/layouts/default.vue` with:

```vue
<script setup lang="ts">
const route = useRoute();
const isHomepage = computed(() => route.path === "/");
</script>

<template>
  <div class="flex min-h-screen flex-col bg-base text-primary">
    <AppHeader :hide-search="isHomepage" />
    <main class="mx-auto w-full max-w-6xl flex-1 px-4 py-8">
      <slot />
    </main>
    <footer class="mt-auto border-t border-subtle py-4">
      <div class="mx-auto flex max-w-6xl items-center justify-between px-4 text-xs text-muted">
        <span>pypx — not affiliated with PyPI or the PSF</span>
        <div class="flex gap-4">
          <a
            href="https://github.com/mattstrayer/pypx"
            target="_blank"
            rel="noopener noreferrer"
            class="transition-colors hover:text-primary"
          >GitHub</a>
          <a
            href="https://pypi.org"
            target="_blank"
            rel="noopener noreferrer"
            class="transition-colors hover:text-primary"
          >PyPI</a>
        </div>
      </div>
    </footer>
  </div>
</template>
```

- [ ] **Step 2: Verify visually**

Navigate to `http://localhost:3000`. Confirm footer is visible at the bottom. Navigate to `/packages/requests` — confirm the footer appears below the package content. Check light and dark mode.

On the homepage (short content), confirm the footer is pushed to the bottom of the viewport, not floating in the middle.

- [ ] **Step 3: Commit**

```bash
git add web/app/layouts/default.vue
git commit -m "feat: add micro footer, flex-col layout for short pages"
```

---

## Task 7: Package page skeleton loader

**Files:**
- Modify: `web/app/pages/packages/[name].vue`

Replace the centered spinner shown during package data load with a skeleton that mirrors the two-column package page layout. This eliminates the content layout jump when data arrives.

- [ ] **Step 1: Replace the spinner with a skeleton**

In `web/app/pages/packages/[name].vue`, find the loading state block (around line 108):

```html
<!-- Loading state -->
<div v-if="status === 'pending'" class="flex items-center justify-center py-24">
  <div class="h-8 w-8 animate-spin rounded-full border-2 border-subtle border-t-primary" />
</div>
```

Replace it with:

```html
<!-- Loading skeleton -->
<div v-if="status === 'pending'">
  <!-- Package header skeleton -->
  <div class="mb-6">
    <div class="flex items-baseline gap-3">
      <div class="h-8 w-40 animate-pulse rounded bg-raised" />
      <div class="h-5 w-16 animate-pulse rounded bg-raised" />
    </div>
    <div class="mt-2 h-4 w-80 animate-pulse rounded bg-raised" />
    <div class="mt-3 flex gap-2">
      <div class="h-5 w-14 animate-pulse rounded bg-raised" />
      <div class="h-5 w-20 animate-pulse rounded bg-raised" />
      <div class="h-5 w-16 animate-pulse rounded bg-raised" />
    </div>
  </div>
  <!-- Tab strip skeleton -->
  <div class="mb-6 flex gap-1 border-b border-subtle pb-0">
    <div v-for="i in 4" :key="i" class="h-9 w-24 animate-pulse rounded-t bg-raised" />
  </div>
  <!-- Two-column content skeleton -->
  <div class="grid gap-6 lg:grid-cols-[1fr_300px]">
    <div class="space-y-4">
      <div class="h-12 w-full animate-pulse rounded-lg bg-raised" />
      <div class="h-64 w-full animate-pulse rounded-lg bg-raised" />
    </div>
    <div class="space-y-4">
      <div class="h-36 w-full animate-pulse rounded-lg bg-raised" />
      <div class="h-24 w-full animate-pulse rounded-lg bg-raised" />
    </div>
  </div>
</div>
```

- [ ] **Step 2: Verify visually**

Navigate to `http://localhost:3000/packages/requests`. If the page loads from cache instantly, open DevTools → Network → set throttling to "Slow 4G" and hard-refresh. Confirm the skeleton appears with the correct two-column layout shape before data arrives. Confirm no layout jump when data arrives.

- [ ] **Step 3: Commit**

```bash
git add web/app/pages/packages/[name].vue
git commit -m "feat: package page skeleton loader replaces spinner"
```

---

## Task 8: Docs page context bar

**Files:**
- Modify: `web/app/pages/packages/[name]/docs.vue`

The docs page has an inline "header" (back arrow, name, version, summary) sitting inside the main content area. Restyle it as a proper full-width context bar with a bottom border, sitting visually above the sidebar+content layout. This makes it feel like a secondary navigation bar rather than page content.

- [ ] **Step 1: Replace the inline header with a styled context bar**

In `web/app/pages/packages/[name]/docs.vue`, find the existing header block inside the `v-else-if="pkg"` branch (around line 219):

```html
<!-- Header -->
<div class="mb-6">
  <div class="flex flex-wrap items-baseline gap-3">
    <NuxtLink
      :to="`/packages/${pkg.name}`"
      class="text-3xl font-bold text-primary hover:text-zinc-700 dark:hover:text-zinc-300 transition-colors"
      ><span class="mr-1 text-2xl font-normal text-muted">←</span>{{ pkg.name }}</NuxtLink
    >
    <span class="rounded bg-raised px-2 py-0.5 font-mono text-sm text-muted">
      v{{ pkg.version }}
    </span>
  </div>
  <p v-if="pkg.summary" class="mt-2 text-muted">{{ pkg.summary }}</p>
</div>
```

Replace it with:

```html
<!-- Docs context bar -->
<div class="-mx-4 sm:-mx-6 lg:-mx-8 mb-0 border-b border-subtle bg-base/90 backdrop-blur-sm">
  <div class="mx-auto flex max-w-6xl items-center gap-3 px-4 py-2">
    <NuxtLink
      :to="`/packages/${pkg.name}`"
      class="flex items-center gap-1 text-sm font-medium text-[var(--color-brand)] transition-colors hover:text-[var(--color-brand-light)]"
    >
      <span>←</span>
      <span>{{ pkg.name }}</span>
    </NuxtLink>
    <span class="text-muted">·</span>
    <span class="font-mono text-xs text-muted">v{{ pkg.version }}</span>
    <span v-if="pkg.summary" class="hidden truncate text-xs text-muted sm:block">
      {{ pkg.summary }}
    </span>
    <button
      class="ml-auto flex cursor-pointer items-center gap-1.5 rounded-md bg-zinc-800/50 px-2.5 py-1.5 text-[11px] text-zinc-500 transition-colors hover:bg-zinc-800 hover:text-zinc-300"
      @click="paletteOpen = true"
    >
      <svg class="h-3 w-3 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
        <path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
      </svg>
      <span>Jump to symbol</span>
      <kbd class="text-[9px] text-zinc-600">⌘K</kbd>
    </button>
  </div>
</div>
```

- [ ] **Step 2: Verify visually**

Navigate to `http://localhost:3000/packages/numpy/docs`. Confirm: (a) the context bar spans the full width of the page; (b) it has a subtle bottom border separating it from the sidebar+content area; (c) the back arrow link navigates to `/packages/numpy`; (d) the ⌘K button opens the symbol palette; (e) on mobile, the summary is hidden but name, version, and ⌘K button remain visible.

- [ ] **Step 3: Commit**

```bash
git add "web/app/pages/packages/[name]/docs.vue"
git commit -m "feat: docs page context bar replaces inline header"
```

---

## Final Verification

- [ ] **Run the full frontend test suite**

```bash
cd web && pnpm test
```

Expected: all tests pass.

- [ ] **Visual smoke test — all pages**

With dev server running (`cd web && pnpm dev`):

1. `/` — feature callout visible, "Trending" section, `/` kbd hint, footer present
2. `/packages/requests` — active tab has green border, skeleton on slow load, sidebar sections readable in light mode
3. `/packages/numpy/docs` — context bar full-width with border, ⌘K button, active symbol is green not blue
4. Type "flask" in header, press Enter — navigates to `/packages/flask`
5. Toggle light/dark mode on `/packages/requests` — sidebar sections remain readable in both

- [ ] **Final commit**

```bash
git add -A
git commit -m "chore: ux overhaul — verification complete"
```
