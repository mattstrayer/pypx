# Design Overhaul Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Overhaul pypx's visual design across all four surfaces (header, homepage, package page, docs page) to project developer-tool confidence and surface the site's data richness to first-time visitors.

**Architecture:** Pure frontend changes — 13 Vue component files, no API or backend modifications. Changes follow the existing CSS token system (`--color-brand`, `--color-surface`, etc.) and Tailwind 4 utility classes. The stats sparkline is hand-coded SVG computed from existing `overall` weekly data already returned by the API.

**Tech Stack:** Nuxt 4, Vue 3 (Composition API, `<script setup>`), Tailwind 4, Geist + Geist Mono fonts, pnpm

---

## File Map

| File | What changes |
|------|-------------|
| `web/app/components/AppHeader.vue` | Add `px` logomark badge left of wordmark |
| `web/app/pages/index.vue` | Eyebrow pill, headline split, search UX, feature strip, section header |
| `web/app/components/TrendingPackages.vue` | Summary min-height, proportional download bar |
| `web/app/layouts/default.vue` | Footer data attribution right side |
| `web/app/components/PackageOverview.vue` | Convert GitHub/cadence/doc_url border-t dividers → cards; fold doc_url into Links |
| `web/app/components/PackagePlatforms.vue` | Token-correct internal label; remove so card wrapper owns it |
| `web/app/components/PackageMaintainers.vue` | Replace all hardcoded `neutral-*` with design tokens |
| `web/app/components/PackageStats.vue` | Hero stats row + SVG sparkline + 2-col breakdown grid |
| `web/app/components/docs/DocsSymbolCard.vue` | Card container, active state, section labels, param/returns pill rows |
| `web/app/pages/packages/[name]/docs.vue` | Jump-to-symbol button → brand-dim style |
| `web/app/components/docs/DocsSidebar.vue` | Width 216px, SVG chevrons |
| `web/app/components/docs/DocsCommandPalette.vue` | Replace hardcoded zinc classes with design tokens |
| `web/app/components/CommandPalette.vue` | Modal container border → `brand-border` |

---

## Phase 1 — Header + Homepage

### Task 1: Logomark in AppHeader

**Files:**
- Modify: `web/app/components/AppHeader.vue`

- [ ] **Step 1: Add logomark inside the NuxtLink wordmark**

In `web/app/components/AppHeader.vue`, replace the existing `<NuxtLink to="/">` inner content:

```html
<!-- Before -->
<NuxtLink to="/" class="flex items-center gap-2">
  <span
    class="text-lg font-bold tracking-tight text-[var(--color-brand)] hover:text-[var(--color-brand-light)] transition-colors"
    >pypx</span
  >
</NuxtLink>

<!-- After -->
<NuxtLink to="/" class="flex items-center gap-2">
  <div
    class="flex h-[26px] w-[26px] shrink-0 items-center justify-center rounded-[6px] border border-[var(--color-brand-border)] bg-[var(--color-brand-muted)] font-mono text-[9px] font-semibold tracking-[-0.04em] text-[var(--color-brand)]"
  >
    px
  </div>
  <span
    class="text-lg font-bold tracking-tight text-[var(--color-brand)] hover:text-[var(--color-brand-light)] transition-colors"
    >pypx</span
  >
</NuxtLink>
```

- [ ] **Step 2: Verify in browser**

Dev server should already be running (`pnpm run dev` from `web/`). Open http://localhost:3000 — the header should show a small rounded `px` badge to the left of the "pypx" wordmark. Check both light and dark modes via the theme toggle.

- [ ] **Step 3: Commit**

```bash
git add web/app/components/AppHeader.vue
git commit -m "feat: add px logomark to AppHeader"
```

---

### Task 2: Homepage — Hero Section

**Files:**
- Modify: `web/app/pages/index.vue`

- [ ] **Step 1: Replace the hero `<section>`**

In `web/app/pages/index.vue`, replace the entire hero `<section>` block:

```html
<!-- Before -->
<section class="flex flex-col items-center pt-16 pb-12 text-center">
  <h1 class="text-5xl font-bold tracking-tight text-[var(--color-brand)]">pypx</h1>
  <p class="mt-3 max-w-lg text-lg text-muted">
    The Python Package Index, reimagined. Fast search, dependency insights, and download trends
    — all in one place.
  </p>
  <div ref="searchWrapper" class="relative mt-8 w-full max-w-xl">
    <form @submit.prevent="onSearch">
      <div class="relative">
        <input
          v-model="query"
          type="text"
          aria-label="Search Python packages"
          placeholder="Search 500,000+ Python packages..."
          class="w-full rounded-lg border border-subtle bg-surface px-4 py-3 pr-16 text-primary placeholder-muted outline-none focus:border-[var(--color-brand-light)] focus:ring-1 focus:ring-[var(--color-brand-border)]"
          @keydown="onKeydown"
          @focus="query.trim() && (isOpen = true)"
        />
        <kbd
          class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 hidden rounded bg-raised px-1.5 py-0.5 font-mono text-[10px] text-muted sm:inline"
        >
          /
        </kbd>
      </div>
    </form>

    <!-- Typeahead dropdown -->
    <div v-if="isOpen" class="absolute top-full left-0 right-0 z-50 mt-1">
      <SearchDropdown
        :results="results"
        :selected-index="selectedIndex"
        :loading="isLoading"
        :has-query="!!query.trim()"
        @select="navigateToResult"
        @hover="(i) => (selectedIndex = i)"
      />
    </div>
  </div>
</section>

<!-- After -->
<section class="flex flex-col items-center pt-16 pb-12 text-center">
  <!-- Eyebrow pill -->
  <div
    class="mb-6 inline-flex items-center gap-2 rounded-full border border-[var(--color-brand-border)] bg-[var(--color-brand-muted)] px-3 py-1.5 text-xs font-medium text-[var(--color-brand)]"
  >
    <span
      class="h-1.5 w-1.5 rounded-full bg-[var(--color-brand)] animate-pulse"
      aria-hidden="true"
    />
    500,000+ packages indexed
  </div>

  <h1
    class="text-5xl font-bold tracking-[-0.04em] text-primary leading-[1.05]"
    style="font-size: clamp(2.5rem, 6vw, 3.25rem)"
  >
    The Python Package<br />
    <span class="text-[var(--color-brand)]">Index, reimagined.</span>
  </h1>

  <p class="mt-4 max-w-lg text-lg text-muted leading-relaxed">
    Fast search, dependency insights, API docs, and security advisories — all in one place.
  </p>

  <div ref="searchWrapper" class="relative mt-8 w-full max-w-[560px]">
    <form @submit.prevent="onSearch">
      <div class="relative">
        <input
          v-model="query"
          type="text"
          aria-label="Search Python packages"
          placeholder="Search packages..."
          class="w-full rounded-xl border border-subtle bg-surface px-4 py-3.5 pr-16 text-primary placeholder-muted outline-none transition-[border-color,box-shadow] focus:border-[var(--color-brand-border)] focus:ring-2 focus:ring-[var(--color-brand-border)] focus:ring-offset-0 shadow-[0_1px_3px_rgba(0,0,0,0.15)]"
          @keydown="onKeydown"
          @focus="query.trim() && (isOpen = true)"
        />
        <kbd
          class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 rounded border border-subtle bg-raised px-1.5 py-0.5 font-mono text-[10px] text-muted"
        >
          /
        </kbd>
      </div>
    </form>

    <!-- Typeahead dropdown -->
    <div v-if="isOpen" class="absolute top-full left-0 right-0 z-50 mt-1">
      <SearchDropdown
        :results="results"
        :selected-index="selectedIndex"
        :loading="isLoading"
        :has-query="!!query.trim()"
        @select="navigateToResult"
        @hover="(i) => (selectedIndex = i)"
      />
    </div>
  </div>

  <p class="mt-2.5 text-xs text-muted opacity-70">
    ↑↓ to navigate · ↵ to open · esc to close
  </p>
</section>
```

- [ ] **Step 2: Verify in browser**

Open http://localhost:3000. Check:
- Eyebrow pill with pulsing dot appears above the headline
- Headline is two lines, second line in brand green
- Search bar is slightly taller with rounded-xl corners
- Hint text appears below the search bar

- [ ] **Step 3: Commit**

```bash
git add web/app/pages/index.vue
git commit -m "feat: redesign homepage hero — eyebrow pill, split headline, improved search"
```

---

### Task 3: Homepage — Feature Strip

**Files:**
- Modify: `web/app/pages/index.vue`

- [ ] **Step 1: Add feature strip between hero and trending sections**

In `web/app/pages/index.vue`, add the feature strip block after the closing `</section>` of the hero and before the `<section class="pb-16">` trending section:

```html
<!-- Feature strip — insert between hero </section> and trending <section> -->
<div
  class="mb-4 mt-16 grid grid-cols-1 gap-px sm:grid-cols-3"
  style="background-color: var(--color-subtle); border: 1px solid var(--color-subtle); border-radius: 14px; overflow: hidden;"
>
  <div
    class="flex flex-col gap-2 bg-surface px-6 py-5 transition-colors hover:bg-raised"
  >
    <div
      class="flex h-8 w-8 items-center justify-center rounded-lg border border-[var(--color-brand-border)] bg-[var(--color-brand-muted)] text-base"
    >
      📦
    </div>
    <p class="text-[13.5px] font-semibold text-primary">API Documentation</p>
    <p class="text-[12.5px] leading-[1.55] text-muted">
      Browse extracted docs from any package — functions, classes, type signatures, and docstrings.
    </p>
  </div>
  <div
    class="flex flex-col gap-2 bg-surface px-6 py-5 transition-colors hover:bg-raised"
  >
    <div
      class="flex h-8 w-8 items-center justify-center rounded-lg border border-[var(--color-brand-border)] bg-[var(--color-brand-muted)] text-base"
    >
      🔒
    </div>
    <p class="text-[13.5px] font-semibold text-primary">Security Advisories</p>
    <p class="text-[12.5px] leading-[1.55] text-muted">
      CVE and vulnerability data from OSV.dev. Know if a package has known issues before you install.
    </p>
  </div>
  <div
    class="flex flex-col gap-2 bg-surface px-6 py-5 transition-colors hover:bg-raised"
  >
    <div
      class="flex h-8 w-8 items-center justify-center rounded-lg border border-[var(--color-brand-border)] bg-[var(--color-brand-muted)] text-base"
    >
      🌿
    </div>
    <p class="text-[13.5px] font-semibold text-primary">Dependency Analysis</p>
    <p class="text-[12.5px] leading-[1.55] text-muted">
      Full dependency tree with optional extras, platform coverage, and install size estimates.
    </p>
  </div>
</div>
```

- [ ] **Step 2: Verify in browser**

http://localhost:3000 — three feature cells should appear between the search bar and the trending section, with thin dividers between cells and hover states.

- [ ] **Step 3: Commit**

```bash
git add web/app/pages/index.vue
git commit -m "feat: add feature strip to homepage — docs, security, deps callout"
```

---

### Task 4: Homepage — Section Header + Trending Cards

**Files:**
- Modify: `web/app/pages/index.vue`
- Modify: `web/app/components/TrendingPackages.vue`

- [ ] **Step 1: Replace the trending `<h2>` with the new section header**

In `web/app/pages/index.vue`, inside the `<section class="pb-16">`, replace:

```html
<!-- Before -->
<h2 class="mb-4 text-sm font-medium uppercase tracking-wider text-muted">Trending</h2>

<!-- After -->
<div class="mb-4 flex items-center gap-3">
  <span class="text-xs font-semibold uppercase tracking-[0.07em] text-muted">Trending</span>
  <div class="h-px flex-1 bg-subtle" />
  <span class="font-mono text-[11.5px] text-muted opacity-70">top 24 by downloads · updated daily</span>
</div>
```

- [ ] **Step 2: Update TrendingPackages to compute maxDownloads and add the mini bar**

Replace the entire `web/app/components/TrendingPackages.vue` with:

```vue
<script setup lang="ts">
import type { SearchResult } from "~/types/api";

const props = defineProps<{
  packages: SearchResult[];
}>();

function formatDownloads(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(0)}K`;
  return String(n);
}

const maxDownloads = computed(() =>
  Math.max(...props.packages.map((p) => p.downloads), 1),
);
</script>

<template>
  <div class="grid grid-cols-1 gap-2.5 sm:grid-cols-2 lg:grid-cols-3">
    <NuxtLink
      v-for="pkg in packages"
      :key="pkg.name"
      :to="`/packages/${pkg.name}`"
      class="group flex flex-col gap-1.5 rounded-[10px] border border-subtle bg-surface px-4 py-3.5 transition-colors hover:border-[rgba(74,222,128,0.3)] hover:bg-[rgba(74,222,128,0.03)]"
    >
      <div class="flex items-center justify-between gap-2">
        <h3
          class="text-[13.5px] font-semibold leading-tight tracking-[-0.01em] text-primary transition-colors group-hover:text-[var(--color-brand)]"
        >
          {{ pkg.name }}
        </h3>
        <span class="shrink-0 font-mono text-[10.5px] text-muted"
          >{{ formatDownloads(pkg.downloads) }}/mo</span
        >
      </div>
      <p class="min-h-[34px] text-[11.5px] leading-[1.5] text-muted line-clamp-2">
        {{ pkg.summary }}
      </p>
      <!-- Proportional download bar -->
      <div class="mt-0.5 h-0.5 overflow-hidden rounded-full bg-raised">
        <div
          class="h-full rounded-full bg-gradient-to-r from-[rgba(74,222,128,0.5)] to-[rgba(74,222,128,0.25)]"
          :style="{ width: `${(pkg.downloads / maxDownloads) * 100}%` }"
        />
      </div>
    </NuxtLink>
  </div>
</template>
```

- [ ] **Step 3: Verify in browser**

http://localhost:3000 — check:
- Section header shows "Trending ——— top 24 by downloads · updated daily"
- Cards are slightly taller, summary text is visible (not just name + count)
- A thin green proportional bar appears at the bottom of each card
- `boto3` card has a full-width bar; lower-ranked packages have shorter bars

- [ ] **Step 4: Commit**

```bash
git add web/app/pages/index.vue web/app/components/TrendingPackages.vue
git commit -m "feat: improve trending section — section header, visible summaries, download bars"
```

---

### Task 5: Footer Data Attribution

**Files:**
- Modify: `web/app/layouts/default.vue`

- [ ] **Step 1: Update the footer to two-column flex**

In `web/app/layouts/default.vue`, replace the footer:

```html
<!-- Before -->
<footer aria-label="Site footer" class="mt-auto border-t border-subtle py-4">
  <div class="mx-auto max-w-6xl px-4 text-xs text-muted">
    pypx — not affiliated with PyPI or the PSF
  </div>
</footer>

<!-- After -->
<footer aria-label="Site footer" class="mt-auto border-t border-subtle py-4">
  <div class="mx-auto flex max-w-6xl items-center justify-between px-4 text-xs text-muted">
    <span>pypx — not affiliated with PyPI or the PSF</span>
    <span class="hidden sm:block">data from pypi.org · pypistats.org · osv.dev</span>
  </div>
</footer>
```

- [ ] **Step 2: Verify in browser**

Footer should show the disclaimer on the left and data attribution on the right. On small screens the right side is hidden.

- [ ] **Step 3: Commit**

```bash
git add web/app/layouts/default.vue
git commit -m "feat: add data attribution to footer"
```

---

## Phase 2 — Package Page Sidebar

### Task 6: Sidebar — GitHub, Cadence, and Doc Link Cards

**Files:**
- Modify: `web/app/components/PackageOverview.vue`

- [ ] **Step 1: Replace sidebar border-t divider sections with cards**

In `web/app/components/PackageOverview.vue`, replace the entire `<!-- Sidebar -->` div (everything inside `<div class="space-y-4">`) with:

```html
<!-- Sidebar -->
<div class="space-y-3">
  <!-- Metadata card -->
  <div class="rounded-lg border border-subtle bg-surface p-4">
    <h2 class="mb-3 text-[10px] font-semibold uppercase tracking-[0.08em] text-muted">Details</h2>
    <dl class="space-y-2 text-sm">
      <div class="flex justify-between gap-2">
        <dt class="text-muted">Version</dt>
        <dd class="font-mono text-zinc-700 dark:text-zinc-300">{{ pkg.version }}</dd>
      </div>
      <div v-if="pkg.license" class="flex justify-between gap-2">
        <dt class="text-muted">License</dt>
        <dd class="text-right text-zinc-700 dark:text-zinc-300">{{ pkg.license }}</dd>
      </div>
      <div v-if="pkg.requires_python" class="flex justify-between gap-2">
        <dt class="text-muted">Python</dt>
        <dd class="font-mono text-zinc-700 dark:text-zinc-300">{{ pkg.requires_python }}</dd>
      </div>
      <div v-if="maintainer" class="flex justify-between gap-2">
        <dt class="text-muted">Maintainer</dt>
        <dd class="truncate text-right text-zinc-700 dark:text-zinc-300">{{ maintainer }}</dd>
      </div>
    </dl>
  </div>

  <!-- Project links card (includes doc_url as first entry if present) -->
  <div v-if="allLinks.length > 0" class="rounded-lg border border-subtle bg-surface p-4">
    <h2 class="mb-3 text-[10px] font-semibold uppercase tracking-[0.08em] text-muted">Links</h2>
    <ul class="space-y-1.5 text-sm">
      <li v-for="link in allLinks" :key="link.label">
        <a
          :href="link.url"
          target="_blank"
          rel="noopener noreferrer"
          class="flex items-center gap-1.5 text-muted transition-colors hover:text-zinc-800 dark:hover:text-zinc-200"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-3.5 w-3.5 shrink-0"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
            <polyline points="15 3 21 3 21 9" />
            <line x1="10" y1="14" x2="21" y2="3" />
          </svg>
          {{ link.label }}
        </a>
      </li>
    </ul>
  </div>

  <!-- GitHub card -->
  <div v-if="repoInfo" class="rounded-lg border border-subtle bg-surface p-4">
    <h2 class="mb-3 text-[10px] font-semibold uppercase tracking-[0.08em] text-muted">GitHub</h2>
    <div class="flex gap-4">
      <div v-if="repoInfo.stars" class="flex flex-col gap-0.5">
        <span class="text-base font-semibold text-primary">{{ repoInfo.stars.toLocaleString() }}</span>
        <span class="text-xs text-muted">stars</span>
      </div>
      <div v-if="repoInfo.forks" class="flex flex-col gap-0.5">
        <span class="text-base font-semibold text-primary">{{ repoInfo.forks.toLocaleString() }}</span>
        <span class="text-xs text-muted">forks</span>
      </div>
      <div v-if="repoInfo.open_issues !== undefined" class="flex flex-col gap-0.5">
        <span class="text-base font-semibold text-primary">{{ repoInfo.open_issues.toLocaleString() }}</span>
        <span class="text-xs text-muted">open issues</span>
      </div>
    </div>
    <div v-if="lastPushedAgo" class="mt-2 text-xs text-muted">
      last commit {{ lastPushedAgo }}
    </div>
  </div>

  <!-- Release cadence card -->
  <div
    v-if="pkg.release_cadence?.releases_last_12mo > 0"
    class="rounded-lg border border-subtle bg-surface p-4"
  >
    <h2 class="mb-2 text-[10px] font-semibold uppercase tracking-[0.08em] text-muted">Release Cadence</h2>
    <div class="text-xl font-bold text-primary">{{ pkg.release_cadence.releases_last_12mo }}</div>
    <div class="text-xs text-muted">releases in the past year</div>
    <div
      v-if="pkg.release_cadence.avg_days_between_releases > 0"
      class="mt-0.5 text-xs text-muted"
    >
      avg {{ Math.round(pkg.release_cadence.avg_days_between_releases) }} days between releases
    </div>
  </div>

  <!-- Platform coverage card -->
  <div class="rounded-lg border border-subtle bg-surface p-4">
    <PackagePlatforms :coverage="pkg.platform_coverage" />
  </div>

  <!-- Maintainers card -->
  <div class="rounded-lg border border-subtle bg-surface p-4">
    <PackageMaintainers :maintainers="pkg.maintainers" :repo-info="repoInfo" />
  </div>
</div>
```

- [ ] **Step 2: Update the `<script setup>` to add `allLinks` computed**

In `web/app/components/PackageOverview.vue`, add `allLinks` to the computed section (after `projectLinks`):

```typescript
const allLinks = computed(() => {
  const links = projectLinks.value.slice();
  if (pkg.doc_url && !links.some((l) => l.url === pkg.doc_url)) {
    links.unshift({ label: "Documentation", url: pkg.doc_url });
  }
  return links;
});
```

- [ ] **Step 3: Verify in browser**

Navigate to http://localhost:3000/packages/requests. The sidebar should show:
- All sections as uniform cards (same border, bg, padding)
- GitHub section shows star/fork/issue counts as stacked stat blocks
- Release cadence shows a large number + supporting text
- No bare border-t dividers anywhere in the sidebar

- [ ] **Step 4: Commit**

```bash
git add web/app/components/PackageOverview.vue
git commit -m "feat: convert package sidebar to consistent card system"
```

---

### Task 7: Sidebar — PackagePlatforms Token Fix

**Files:**
- Modify: `web/app/components/PackagePlatforms.vue`

The component's internal heading is now redundant since `PackageOverview` wraps it in a card. Remove it and keep only the content. Also the platform pills use hardcoded zinc/neutral classes — leave those as-is (they're intentionally neutral semantic badges, not structural tokens).

- [ ] **Step 1: Remove the internal heading from PackagePlatforms**

In `web/app/components/PackagePlatforms.vue`, replace the template:

```html
<!-- Before -->
<template>
  <div v-if="hasAnyCoverage">
    <div class="text-xs font-medium text-muted uppercase tracking-wide mb-2">Platforms</div>
    <div class="flex flex-wrap gap-1.5">
      <span
        v-for="p in supported"
        :key="p.key"
        :title="p.label"
        class="inline-flex items-center px-2 py-0.5 rounded text-xs font-mono bg-zinc-100 text-zinc-600 ring-1 ring-zinc-200 dark:bg-neutral-800 dark:text-neutral-300 dark:ring-neutral-700"
      >
        {{ p.short }}
      </span>
    </div>
  </div>
</template>

<!-- After -->
<template>
  <div v-if="hasAnyCoverage">
    <h2 class="mb-2.5 text-[10px] font-semibold uppercase tracking-[0.08em] text-muted">Platforms</h2>
    <div class="flex flex-wrap gap-1.5">
      <span
        v-for="p in supported"
        :key="p.key"
        :title="p.label"
        class="inline-flex items-center rounded px-2 py-0.5 font-mono text-xs bg-zinc-100 text-zinc-600 ring-1 ring-zinc-200 dark:bg-neutral-800 dark:text-neutral-300 dark:ring-neutral-700"
      >
        {{ p.short }}
      </span>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Verify in browser**

On a package page with platform data (e.g. http://localhost:3000/packages/numpy), the Platforms card should show the heading in the matching card-label style, then the platform pills below.

- [ ] **Step 3: Commit**

```bash
git add web/app/components/PackagePlatforms.vue
git commit -m "fix: align PackagePlatforms heading to card-label style"
```

---

### Task 8: Sidebar — PackageMaintainers Token Fix

**Files:**
- Modify: `web/app/components/PackageMaintainers.vue`

This component uses hardcoded `neutral-*` colors throughout — they bypass the CSS token system and won't theme-switch correctly.

- [ ] **Step 1: Replace the entire component template with token-correct version**

Replace the full template in `web/app/components/PackageMaintainers.vue`:

```html
<template>
  <div v-if="hasAny">
    <h2 class="mb-2.5 text-[10px] font-semibold uppercase tracking-[0.08em] text-muted">Maintainers</h2>

    <!-- GitHub org/owner badge -->
    <div v-if="repoInfo?.owner" class="mb-2">
      <a
        :href="repoInfo.owner.url"
        target="_blank"
        rel="noopener noreferrer"
        class="inline-flex items-center gap-2 text-sm text-zinc-700 transition-colors hover:text-primary dark:text-zinc-300"
      >
        <img
          :src="repoInfo.owner.avatar_url"
          :alt="repoInfo.owner.login"
          class="h-5 w-5 rounded-full"
        />
        <span class="font-medium">
          {{ repoInfo.owner.display_name || repoInfo.owner.login }}
        </span>
        <span
          v-if="repoInfo.owner.is_org"
          class="rounded bg-raised px-1.5 py-0.5 text-xs text-muted ring-1 ring-subtle"
        >
          org
        </span>
      </a>
    </div>

    <!-- Individual maintainers -->
    <ul v-if="displayMaintainers.length" class="space-y-1">
      <li v-for="m in displayMaintainers" :key="m.email || m.name" class="text-sm text-muted">
        <span v-if="m.name" class="text-zinc-700 dark:text-zinc-300">{{ m.name }}</span>
        <span v-if="m.name && m.email" class="text-muted"> · </span>
        <a
          v-if="m.email"
          :href="`mailto:${m.email}`"
          class="text-muted transition-colors hover:text-primary"
        >
          {{ m.email }}
        </a>
      </li>
    </ul>
  </div>
</template>
```

- [ ] **Step 2: Verify in browser**

On http://localhost:3000/packages/requests — maintainers section should render correctly in both light and dark mode. The org badge and individual maintainer names should use the theme tokens.

- [ ] **Step 3: Commit**

```bash
git add web/app/components/PackageMaintainers.vue
git commit -m "fix: replace hardcoded neutral-* tokens in PackageMaintainers"
```

---

## Phase 3 — Stats Tab

### Task 9: Stats Tab — Hero Numbers Row

**Files:**
- Modify: `web/app/components/PackageStats.vue`

- [ ] **Step 1: Add hero stat computed properties**

In `web/app/components/PackageStats.vue`, add these computed properties after the existing `overallTrend`, `pythonVersions`, `systems` computeds:

```typescript
const totalDownloads = computed(() =>
  overallTrend.value.reduce((sum, p) => sum + p.downloads, 0),
);

const weeklyAverage = computed(() =>
  overallTrend.value.length
    ? Math.round(totalDownloads.value / overallTrend.value.length)
    : 0,
);

const peakWeek = computed(() => {
  if (!overallTrend.value.length) return null;
  return overallTrend.value.reduce((max, p) =>
    p.downloads > max.downloads ? p : max,
  );
});
```

- [ ] **Step 2: Add hero stats row to the template**

In `web/app/components/PackageStats.vue`, inside the `<div v-else-if="stats">` block, add the hero row before `<!-- Download Trends -->`:

```html
<!-- Hero stats row -->
<div v-if="overallTrend.length" class="mb-6 grid grid-cols-3 gap-3">
  <div class="rounded-lg border border-subtle bg-surface px-4 py-3">
    <div class="mb-1.5 text-[10.5px] font-medium uppercase tracking-[0.07em] text-muted">
      {{ period === '4w' ? 'Last 4 weeks' : period === '3m' ? 'Last 3 months' : 'Last 6 months' }}
    </div>
    <div class="text-2xl font-bold tracking-tight text-primary">
      {{ formatDownloads(totalDownloads) }}
    </div>
    <div class="mt-0.5 text-xs text-muted">downloads</div>
  </div>
  <div class="rounded-lg border border-subtle bg-surface px-4 py-3">
    <div class="mb-1.5 text-[10.5px] font-medium uppercase tracking-[0.07em] text-muted">
      Weekly average
    </div>
    <div class="text-2xl font-bold tracking-tight text-primary">
      {{ formatDownloads(weeklyAverage) }}
    </div>
    <div class="mt-0.5 text-xs text-muted">downloads / week</div>
  </div>
  <div class="rounded-lg border border-subtle bg-surface px-4 py-3">
    <div class="mb-1.5 text-[10.5px] font-medium uppercase tracking-[0.07em] text-muted">
      Peak week
    </div>
    <div class="text-2xl font-bold tracking-tight text-primary">
      {{ peakWeek ? formatDownloads(peakWeek.downloads) : '—' }}
    </div>
    <div class="mt-0.5 text-xs text-muted">{{ peakWeek?.category ?? '' }}</div>
  </div>
</div>
```

- [ ] **Step 3: Verify in browser**

Navigate to any package stats tab (e.g. http://localhost:3000/packages/requests, click Stats). The three hero cards should appear at the top showing total, weekly average, and peak week data.

- [ ] **Step 4: Commit**

```bash
git add web/app/components/PackageStats.vue
git commit -m "feat: add hero download stats row to stats tab"
```

---

### Task 10: Stats Tab — Sparkline Chart + 2-Column Breakdown

**Files:**
- Modify: `web/app/components/PackageStats.vue`

- [ ] **Step 1: Add sparkline computed properties**

In `web/app/components/PackageStats.vue`, add after the `peakWeek` computed:

```typescript
interface SplinePoint {
  x: number;
  y: number;
}

const sparklinePoints = computed<SplinePoint[]>(() => {
  const data = overallTrend.value;
  if (data.length < 2) return [];
  const max = Math.max(...data.map((d) => d.downloads));
  if (max === 0) return [];
  return data.map((d, i) => ({
    x: (i / (data.length - 1)) * 800,
    y: 10 + (1 - d.downloads / max) * 100,
  }));
});

function buildSplinePath(pts: SplinePoint[]): string {
  if (pts.length < 2) return "";
  let d = `M ${pts[0].x.toFixed(1)} ${pts[0].y.toFixed(1)}`;
  for (let i = 0; i < pts.length - 1; i++) {
    const p0 = pts[i];
    const p1 = pts[i + 1];
    const cp1x = (p0.x + (p1.x - p0.x) / 3).toFixed(1);
    const cp2x = (p0.x + (2 * (p1.x - p0.x)) / 3).toFixed(1);
    d += ` C ${cp1x} ${p0.y.toFixed(1)} ${cp2x} ${p1.y.toFixed(1)} ${p1.x.toFixed(1)} ${p1.y.toFixed(1)}`;
  }
  return d;
}

const sparklinePath = computed(() => buildSplinePath(sparklinePoints.value));

const sparklineAreaPath = computed(() => {
  const pts = sparklinePoints.value;
  if (pts.length < 2) return "";
  const line = sparklinePath.value;
  const last = pts[pts.length - 1];
  const first = pts[0];
  return `${line} L ${last.x.toFixed(1)} 120 L ${first.x.toFixed(1)} 120 Z`;
});

const sparklineXLabels = computed(() => {
  const data = overallTrend.value;
  if (data.length < 2) return [];
  const mid = Math.floor((data.length - 1) / 2);
  return [data[0]?.category ?? "", data[mid]?.category ?? "", data[data.length - 1]?.category ?? ""];
});
```

- [ ] **Step 2: Replace the Download Trends bar chart with the sparkline**

In the template, replace the `<!-- Download Trends -->` section:

```html
<!-- Before -->
<div v-if="overallTrend.length">
  <h2 class="mb-4 text-sm font-medium uppercase tracking-wider text-muted">
    Download Trends
  </h2>
  <div class="space-y-2">
    <div v-for="point in overallTrend" :key="point.category" class="flex items-center gap-3">
      <span class="w-20 shrink-0 font-mono text-xs text-muted">{{ point.category }}</span>
      <div class="flex-1">
        <div
          class="h-4 rounded-sm bg-emerald-500/60 dark:bg-emerald-400/30"
          :style="{ width: barWidth(point.downloads, maxDownloads(overallTrend)) }"
        />
      </div>
      <span class="w-12 shrink-0 text-right font-mono text-xs text-muted">
        {{ formatDownloads(point.downloads) }}
      </span>
    </div>
  </div>
</div>

<!-- After -->
<div v-if="sparklinePoints.length" class="mb-6">
  <div class="mb-3 flex items-center justify-between">
    <h2 class="text-xs font-semibold uppercase tracking-[0.07em] text-muted">Download Trend</h2>
  </div>
  <div class="overflow-hidden rounded-lg border border-subtle bg-surface px-4 pb-2.5 pt-4">
    <svg
      viewBox="0 0 800 120"
      preserveAspectRatio="none"
      class="w-full"
      style="height: 120px; display: block"
      aria-hidden="true"
    >
      <defs>
        <linearGradient id="spark-grad" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stop-color="rgba(74,222,128,0.25)" />
          <stop offset="100%" stop-color="rgba(74,222,128,0)" />
        </linearGradient>
      </defs>
      <!-- Grid lines -->
      <line x1="0" y1="35" x2="800" y2="35" stroke="rgba(255,255,255,0.04)" stroke-width="1" />
      <line x1="0" y1="60" x2="800" y2="60" stroke="rgba(255,255,255,0.04)" stroke-width="1" />
      <line x1="0" y1="85" x2="800" y2="85" stroke="rgba(255,255,255,0.04)" stroke-width="1" />
      <!-- Area fill -->
      <path :d="sparklineAreaPath" fill="url(#spark-grad)" />
      <!-- Line -->
      <path
        :d="sparklinePath"
        fill="none"
        stroke="rgba(74,222,128,0.8)"
        stroke-width="1.8"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <!-- Dots -->
      <circle
        v-for="(pt, i) in sparklinePoints"
        :key="i"
        :cx="pt.x"
        :cy="pt.y"
        :r="i === sparklinePoints.length - 1 ? 4 : 3"
        fill="rgba(74,222,128,0.9)"
      />
    </svg>
    <!-- X-axis labels -->
    <div class="mt-1.5 flex justify-between">
      <span
        v-for="label in sparklineXLabels"
        :key="label"
        class="font-mono text-[9.5px] text-muted opacity-60"
        >{{ label }}</span
      >
    </div>
  </div>
</div>
```

- [ ] **Step 3: Move Python version + OS breakdown to 2-column grid**

Replace the separate `<!-- By Python Version -->` and `<!-- By Operating System -->` sections with a 2-column grid:

```html
<!-- Replace both sections with: -->
<div v-if="pythonVersions.length || systems.length" class="grid gap-6 sm:grid-cols-2">
  <!-- By Python Version -->
  <div v-if="pythonVersions.length">
    <h2 class="mb-3 text-xs font-semibold uppercase tracking-[0.07em] text-muted">
      By Python Version
    </h2>
    <div class="space-y-2">
      <div
        v-for="point in pythonVersions"
        :key="point.category"
        class="flex items-center gap-3"
      >
        <span class="w-12 shrink-0 font-mono text-xs text-indigo-600 dark:text-indigo-400">{{
          point.category
        }}</span>
        <div class="flex-1">
          <div
            class="h-1.5 rounded-full bg-indigo-500/50 dark:bg-indigo-500/30"
            :style="{ width: barWidth(point.downloads, maxDownloads(pythonVersions)) }"
          />
        </div>
        <span class="w-10 shrink-0 text-right font-mono text-xs text-muted">
          {{ formatDownloads(point.downloads) }}
        </span>
      </div>
    </div>
  </div>

  <!-- By Operating System -->
  <div v-if="systems.length">
    <h2 class="mb-3 text-xs font-semibold uppercase tracking-[0.07em] text-muted">
      By Operating System
    </h2>
    <div class="space-y-2">
      <div v-for="point in systems" :key="point.category" class="flex items-center gap-3">
        <span class="w-12 shrink-0 font-mono text-xs text-amber-700 dark:text-amber-400">{{
          point.category
        }}</span>
        <div class="flex-1">
          <div
            class="h-1.5 rounded-full bg-amber-500/50 dark:bg-amber-400/30"
            :style="{ width: barWidth(point.downloads, maxDownloads(systems)) }"
          />
        </div>
        <span class="w-10 shrink-0 text-right font-mono text-xs text-muted">
          {{ formatDownloads(point.downloads) }}
        </span>
      </div>
    </div>
  </div>
</div>
```

- [ ] **Step 4: Verify in browser**

On a package stats tab:
- The sparkline area chart replaces the old "Download Trends" bar chart
- The line is smooth with dots at each data point
- Python version and OS breakdown are side by side
- Bars are 6px (h-1.5) instead of 4px

- [ ] **Step 5: Commit**

```bash
git add web/app/components/PackageStats.vue
git commit -m "feat: sparkline chart and 2-col breakdown in stats tab"
```

---

## Phase 4 — Docs Page + Command Palettes

### Task 11: Docs Symbol Cards

**Files:**
- Modify: `web/app/components/docs/DocsSymbolCard.vue`

- [ ] **Step 1: Replace the entire template**

Replace the full template in `web/app/components/docs/DocsSymbolCard.vue`:

```html
<template>
  <div
    :id="`sym-${encodeURIComponent(symbol.name)}`"
    class="mb-4 scroll-mt-4 rounded-xl border bg-surface p-5 transition-[border-color,background-color]"
    :class="
      isActive
        ? 'border-[var(--color-brand-border)] bg-[rgba(74,222,128,0.03)] shadow-[0_0_0_1px_rgba(74,222,128,0.08)]'
        : 'border-subtle hover:border-zinc-300 dark:hover:border-zinc-600'
    "
  >
    <!-- Symbol name + kind badge -->
    <div class="mb-3 flex items-center gap-2.5">
      <span class="font-mono text-[15px] font-bold text-primary">{{ symbol.name }}</span>
      <span
        class="rounded border px-1.5 py-0.5 text-[9px] font-bold uppercase tracking-[0.08em]"
        :class="{
          'border-blue-500/20 bg-blue-950/60 text-blue-300': symbol.kind === 'function',
          'border-purple-500/20 bg-purple-950/60 text-purple-300': symbol.kind === 'class',
          'border-red-500/20 bg-red-950/60 text-red-300': symbol.kind === 'exception',
        }"
        >{{ symbol.kind }}</span
      >
    </div>

    <!-- Signature -->
    <DocsPySignature :symbol="symbol" class="mb-3" />

    <!-- Docstring -->
    <DocsPyDocstring v-if="symbol.docstring" :text="symbol.docstring" class="mb-4" />

    <!-- Parameters -->
    <div v-if="symbol.parameters && symbol.parameters.length" class="mb-4">
      <p class="section-label">Parameters</p>
      <div class="space-y-1.5">
        <div
          v-for="param in symbol.parameters?.filter((p) => p.name !== 'self' && p.name !== 'cls')"
          :key="param.name"
          class="flex items-baseline gap-2 rounded-md border-l-2 border-subtle bg-raised px-3 py-1.5"
        >
          <span class="font-mono text-[12px] text-sky-400">{{ param.name }}</span>
          <span v-if="param.type" class="font-mono text-[11px] text-muted">{{ param.type }}</span>
          <span v-if="param.description" class="ml-1 text-[11.5px] text-muted">{{
            param.description
          }}</span>
        </div>
      </div>
    </div>

    <!-- Returns -->
    <div v-if="symbol.returns" class="mb-4">
      <p class="section-label">Returns</p>
      <div
        class="flex items-baseline gap-2 rounded-md border-l-2 border-[rgba(74,222,128,0.4)] bg-raised px-3 py-1.5"
      >
        <span v-if="symbol.returns.type" class="font-mono text-[12px] text-sky-400">{{
          symbol.returns.type
        }}</span>
        <span v-if="symbol.returns.description" class="text-[11.5px] text-muted">{{
          symbol.returns.description
        }}</span>
      </div>
    </div>

    <!-- Raises -->
    <div v-if="symbol.raises && symbol.raises.length" class="mb-4">
      <p class="section-label">Raises</p>
      <div class="space-y-1.5">
        <div
          v-for="r in symbol.raises"
          :key="r.type"
          class="flex items-baseline gap-2 rounded-md border-l-2 border-red-500/30 bg-raised px-3 py-1.5"
        >
          <span class="font-mono text-[12px] text-red-400">{{ r.type }}</span>
          <span v-if="r.description" class="text-[11.5px] text-muted">{{ r.description }}</span>
        </div>
      </div>
    </div>

    <!-- Methods (classes only) -->
    <div v-if="symbol.kind === 'class' && symbol.methods && symbol.methods.length" class="mb-1">
      <button
        class="flex cursor-pointer items-center gap-2 rounded-md px-2.5 py-1.5 text-[10px] font-semibold uppercase tracking-[0.07em] text-muted transition-colors hover:bg-raised hover:text-primary"
        @click="toggleExpand"
      >
        <svg
          class="h-3 w-3 text-[var(--color-brand)] transition-transform duration-150"
          :class="expanded ? 'rotate-0' : '-rotate-90'"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2.5"
        >
          <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
        </svg>
        Methods
        <span class="rounded-full bg-raised px-2 py-0.5 text-[9px] text-muted ring-1 ring-subtle">
          {{ symbol.methods.length }}
        </span>
      </button>

      <div v-if="expanded" class="mt-3 space-y-5 border-l-2 border-subtle pl-4">
        <div v-for="method in symbol.methods" :key="method.name">
          <div class="mb-2 flex items-center gap-2">
            <span class="font-mono text-[12px] font-semibold text-primary">{{ method.name }}</span>
            <span
              class="rounded border border-blue-500/20 bg-blue-950/60 px-1.5 py-0.5 text-[9px] font-bold uppercase tracking-[0.08em] text-blue-300"
              >method</span
            >
          </div>

          <DocsPySignature :symbol="method" class="mb-2" />
          <DocsPyDocstring v-if="method.docstring" :text="method.docstring" class="mb-2" />

          <div
            v-if="
              method.parameters &&
              method.parameters.filter((p) => p.name !== 'self' && p.name !== 'cls').length
            "
            class="mb-2"
          >
            <p class="section-label">Parameters</p>
            <div class="space-y-1">
              <div
                v-for="param in method.parameters.filter(
                  (p) => p.name !== 'self' && p.name !== 'cls',
                )"
                :key="param.name"
                class="flex items-baseline gap-2 rounded-md border-l-2 border-subtle bg-raised px-3 py-1.5"
              >
                <span class="font-mono text-[12px] text-sky-400">{{ param.name }}</span>
                <span v-if="param.type" class="font-mono text-[11px] text-muted">{{
                  param.type
                }}</span>
                <span v-if="param.description" class="ml-1 text-[11.5px] text-muted">{{
                  param.description
                }}</span>
              </div>
            </div>
          </div>

          <div v-if="method.returns" class="mb-2">
            <p class="section-label">Returns</p>
            <div
              class="flex items-baseline gap-2 rounded-md border-l-2 border-[rgba(74,222,128,0.4)] bg-raised px-3 py-1.5"
            >
              <span v-if="method.returns.type" class="font-mono text-[12px] text-sky-400">{{
                method.returns.type
              }}</span>
              <span v-if="method.returns.description" class="text-[11.5px] text-muted">{{
                method.returns.description
              }}</span>
            </div>
          </div>

          <div v-if="method.raises && method.raises.length" class="mb-2">
            <p class="section-label">Raises</p>
            <div class="space-y-1">
              <div
                v-for="r in method.raises"
                :key="r.type"
                class="flex items-baseline gap-2 rounded-md border-l-2 border-red-500/30 bg-raised px-3 py-1.5"
              >
                <span class="font-mono text-[12px] text-red-400">{{ r.type }}</span>
                <span v-if="r.description" class="text-[11.5px] text-muted">{{ r.description }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Add the `section-label` utility class to `main.css`**

In `web/app/assets/css/main.css`, add after the existing `.prose-sm` block:

```css
.section-label {
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--color-muted);
  margin-bottom: 8px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.section-label::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--color-subtle);
  opacity: 0.5;
}
```

- [ ] **Step 3: Verify in browser**

Navigate to http://localhost:3000/packages/requests/docs:
- Each symbol is in a rounded card with a border
- Scrolling causes the active card to get a green border tint
- Param rows have a left accent border and raised background
- Returns row has a green left accent
- Methods toggle shows a chevron SVG and count badge
- Section labels have a subtle horizontal rule extending to the right

- [ ] **Step 4: Commit**

```bash
git add web/app/components/docs/DocsSymbolCard.vue web/app/assets/css/main.css
git commit -m "feat: symbol cards with containment, active state, and pill param rows"
```

---

### Task 12: Docs Context Bar + Sidebar Polish

**Files:**
- Modify: `web/app/pages/packages/[name]/docs.vue`
- Modify: `web/app/components/docs/DocsSidebar.vue`

- [ ] **Step 1: Update the Jump to symbol button in docs.vue**

In `web/app/pages/packages/[name]/docs.vue`, replace the jump button:

```html
<!-- Before -->
<button
  class="ml-auto flex cursor-pointer items-center gap-1.5 rounded-md bg-raised px-2.5 py-1.5 text-[11px] text-muted transition-colors hover:bg-raised hover:text-primary"
  @click="paletteOpen = true"
>

<!-- After -->
<button
  class="ml-auto flex cursor-pointer items-center gap-1.5 rounded-md border border-[var(--color-brand-border)] bg-[var(--color-brand-muted)] px-2.5 py-1.5 text-[11px] text-[var(--color-brand)] transition-colors hover:bg-[rgba(74,222,128,0.16)]"
  @click="paletteOpen = true"
>
```

- [ ] **Step 2: Update DocsSidebar width and section header chevrons**

In `web/app/components/docs/DocsSidebar.vue`, replace the outer `<div>` class to increase width:

```html
<!-- Before -->
<div
  class="w-48 flex-shrink-0 sticky top-0 h-screen flex flex-col border-r border-subtle bg-base hidden md:flex"
>

<!-- After -->
<div
  class="hidden w-[216px] flex-shrink-0 flex-col border-r border-subtle bg-base md:sticky md:top-0 md:flex md:h-screen"
>
```

Then replace the SVG inside the section header button (the collapse chevron):

```html
<!-- Before -->
<svg
  class="h-3 w-3 flex-shrink-0 text-[var(--color-brand)] transition-transform duration-150"
  :class="collapsed.has(item.kind) ? '-rotate-90' : 'rotate-0'"
  fill="none"
  viewBox="0 0 24 24"
  stroke="currentColor"
  stroke-width="2.5"
>
  <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
</svg>

<!-- After (same SVG, just verify it's already using the correct path — no change needed if identical) -->
```

Note: the existing SVG chevron in DocsSidebar is already correct (it uses `d="M19 9l-7 7-7-7"`). Only the width change and button brand color are needed here.

- [ ] **Step 3: Verify in browser**

On http://localhost:3000/packages/requests/docs:
- The "Jump to symbol" button is now brand-green (matching the new button language)
- Sidebar is slightly wider — long symbol names like `check_compatibility` are no longer truncated

- [ ] **Step 4: Commit**

```bash
git add web/app/pages/packages/[name]/docs.vue web/app/components/docs/DocsSidebar.vue
git commit -m "feat: docs context bar brand button, wider sidebar"
```

---

### Task 13: Command Palette Token Consistency

**Files:**
- Modify: `web/app/components/docs/DocsCommandPalette.vue`
- Modify: `web/app/components/CommandPalette.vue`

- [ ] **Step 1: Fix DocsCommandPalette hardcoded zinc tokens**

In `web/app/components/docs/DocsCommandPalette.vue`, replace the modal and its children. Find and replace all hardcoded zinc/neutral classes in the template:

```html
<!-- Modal container: before -->
class="fixed left-1/2 top-[20vh] z-50 w-full max-w-lg -translate-x-1/2 rounded-xl border border-zinc-700 bg-zinc-900 shadow-2xl overflow-hidden"

<!-- Modal container: after -->
class="fixed left-1/2 top-[20vh] z-50 w-full max-w-lg -translate-x-1/2 overflow-hidden rounded-xl border border-subtle bg-surface shadow-2xl"
```

```html
<!-- Input row border: before -->
class="flex items-center gap-3 border-b border-zinc-700 px-4 py-3"
<!-- after -->
class="flex items-center gap-3 border-b border-subtle px-4 py-3"
```

```html
<!-- Search icon: before -->
class="h-4 w-4 flex-shrink-0 text-zinc-500"
<!-- after -->
class="h-4 w-4 flex-shrink-0 text-muted"
```

```html
<!-- Input field: before -->
class="flex-1 bg-transparent font-mono text-sm text-zinc-100 placeholder-zinc-600 outline-none"
<!-- after -->
class="flex-1 bg-transparent font-mono text-sm text-primary placeholder-muted outline-none"
```

```html
<!-- ESC kbd: before -->
class="text-[10px] text-zinc-600 bg-zinc-800 border border-zinc-700 rounded px-1.5 py-0.5"
<!-- after -->
class="rounded border border-subtle bg-raised px-1.5 py-0.5 text-[10px] text-muted"
```

```html
<!-- No results: before -->
class="px-4 py-6 text-center text-sm text-zinc-600"
<!-- after -->
class="px-4 py-6 text-center text-sm text-muted"
```

```html
<!-- Section row selected: before -->
:class="i === selectedIndex ? 'bg-zinc-800' : 'hover:bg-zinc-800/50'"
<!-- after -->
:class="i === selectedIndex ? 'bg-raised' : 'hover:bg-raised/50'"
```

```html
<!-- Section "#" badge: before -->
class="flex-shrink-0 rounded px-1.5 py-0.5 text-[9px] font-semibold uppercase tracking-wide bg-zinc-700 text-zinc-300"
<!-- after -->
class="flex-shrink-0 rounded bg-raised px-1.5 py-0.5 text-[9px] font-semibold uppercase tracking-wide text-muted ring-1 ring-subtle"
```

```html
<!-- Section label text: before -->
class="font-mono text-sm text-zinc-300"
<!-- after -->
class="font-mono text-sm text-primary"
```

```html
<!-- Section count: before -->
class="ml-auto text-[10px] text-zinc-600"
<!-- after -->
class="ml-auto text-[10px] text-muted"
```

```html
<!-- Symbol row selected: before -->
:class="i === selectedIndex ? 'bg-zinc-800' : 'hover:bg-zinc-800/50'"
<!-- after -->
:class="i === selectedIndex ? 'bg-raised' : 'hover:bg-raised/50'"
```

```html
<!-- Symbol name: before -->
class="font-mono text-sm text-zinc-200"
<!-- after -->
class="font-mono text-sm text-primary"
```

```html
<!-- Footer border + text: before -->
class="border-t border-zinc-700 px-4 py-2 flex items-center gap-3 text-[10px] text-zinc-600"
<!-- after -->
class="flex items-center gap-3 border-t border-subtle px-4 py-2 text-[10px] text-muted"
```

```html
<!-- Footer kbd buttons: before -->
class="bg-zinc-800 border border-zinc-700 rounded px-1 py-0.5"
<!-- after -->
class="rounded border border-subtle bg-raised px-1 py-0.5"
```

- [ ] **Step 2: Update CommandPalette modal border to brand-border**

In `web/app/components/CommandPalette.vue`, find the modal container and update its border class:

```html
<!-- Before -->
class="w-full max-w-lg overflow-hidden rounded-xl border border-subtle bg-surface shadow-2xl"

<!-- After -->
class="w-full max-w-lg overflow-hidden rounded-xl border border-[var(--color-brand-border)] bg-surface shadow-2xl"
```

- [ ] **Step 3: Verify in browser**

Open the global command palette (⌘K from any non-docs page):
- Modal border has a subtle green tint

Open the docs command palette (⌘K from a docs page):
- Palette renders correctly in both light and dark mode
- No zinc colors remain — everything uses theme tokens

- [ ] **Step 4: Commit**

```bash
git add web/app/components/docs/DocsCommandPalette.vue web/app/components/CommandPalette.vue
git commit -m "fix: replace hardcoded zinc tokens in DocsCommandPalette, brand-border on CommandPalette"
```

---

## Self-Review Checklist

Spec coverage verified:
- [x] §1 Header logomark → Task 1
- [x] §2 Hero eyebrow, headline, search → Task 2
- [x] §3 Feature strip → Task 3
- [x] §4 Section header, trending cards → Task 4
- [x] §5 Footer attribution → Task 5
- [x] §6 Sidebar cards (GitHub, cadence, doc_url) → Task 6
- [x] §7 PackagePlatforms wrap → Tasks 6 + 7
- [x] §8 Hero numbers → Task 9
- [x] §8 Sparkline + 2-col breakdown → Task 10
- [x] §9 Symbol cards, active state, section labels, param/returns rows → Task 11
- [x] §10 Context bar button, sidebar width/chevrons → Task 12
- [x] §11 DocsCommandPalette token fix → Task 13
- [x] §11 CommandPalette border → Task 13
- [x] PackageMaintainers token fix → Task 8
