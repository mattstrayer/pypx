# Brand Guidelines Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Apply the pypx brand system — Bright Mint `#4ade80` as the single accent color on interactive/identity elements — across all frontend components via four CSS custom property tokens.

**Architecture:** Four brand tokens defined once in `main.css` inside `:root`. All components reference these tokens via Tailwind's arbitrary value syntax (`text-[var(--color-brand)]`). No Tailwind config changes required. The zinc foundation is preserved everywhere except interactive and identity moments.

**Tech Stack:** Nuxt 4, Vue 3, Tailwind CSS 4, Geist / Geist Mono fonts

**Spec:** `docs/superpowers/specs/2026-04-11-brand-guidelines-design.md`

---

## File Map

| File | Change |
|---|---|
| `web/app/assets/css/main.css` | Add 4 brand tokens to `:root`; swap indigo for brand in `.prose-invert` |
| `web/app/components/AppHeader.vue` | Logo color → brand; search focus ring → brand |
| `web/app/pages/index.vue` | Hero title → brand; search focus ring → brand |
| `web/app/components/TrendingPackages.vue` | Package name group-hover → brand |
| `web/app/components/PackageBadges.vue` | Indigo + emerald badges → brand tokens |
| `web/app/components/InstallCommand.vue` | Active tab + copy button → brand |
| `web/app/components/PackageVersions.vue` | Version link hover, install size, GitHub link → brand |
| `web/app/components/PackageStats.vue` | Period toggle active state + trend bars → brand |

---

## Task 1: Add brand tokens to main.css

**Files:**
- Modify: `web/app/assets/css/main.css`

- [ ] **Step 1: Add the four brand tokens inside `:root`**

  Open `web/app/assets/css/main.css`. The existing `:root` block currently defines `--font-sans` and `--font-mono`. Add the four brand tokens immediately after the font variables:

  ```css
  :root {
    --font-sans: "Geist", ui-sans-serif, system-ui, sans-serif;
    --font-mono: "Geist Mono", ui-monospace, "SFMono-Regular", monospace;

    /* Brand */
    --color-brand:        #4ade80;
    --color-brand-light:  #86efac;
    --color-brand-muted:  rgba(74, 222, 128, 0.08);
    --color-brand-border: rgba(74, 222, 128, 0.25);
  }
  ```

- [ ] **Step 2: Replace indigo with brand tokens in `.prose-invert`**

  Find the `.prose-invert` block (currently around line 110). Replace the two indigo link rules:

  ```css
  /* Before */
  .prose-invert {
    color: var(--color-zinc-300);
    & a {
      color: var(--color-indigo-400);
    }
    & a:hover {
      color: var(--color-indigo-300);
    }
  ```

  ```css
  /* After */
  .prose-invert {
    color: var(--color-zinc-300);
    & a {
      color: var(--color-brand);
    }
    & a:hover {
      color: var(--color-brand-light);
    }
  ```

- [ ] **Step 3: Commit**

  ```bash
  git add web/app/assets/css/main.css
  git commit -m "feat(web): add brand color tokens and update prose link colors"
  ```

---

## Task 2: Brand the wordmark and hero title

**Files:**
- Modify: `web/app/components/AppHeader.vue`
- Modify: `web/app/pages/index.vue`
- Modify: `web/app/components/TrendingPackages.vue`

- [ ] **Step 1: Update AppHeader logo color**

  In `web/app/components/AppHeader.vue`, find the logo `NuxtLink` (around line 38). Replace the text color classes:

  ```html
  <!-- Before -->
  <NuxtLink to="/" class="flex items-center gap-2 text-zinc-50 hover:text-white">
    <span class="text-lg font-bold tracking-tight">pypx</span>
  </NuxtLink>

  <!-- After -->
  <NuxtLink to="/" class="flex items-center gap-2">
    <span class="text-lg font-bold tracking-tight text-[var(--color-brand)] hover:text-[var(--color-brand-light)] transition-colors">pypx</span>
  </NuxtLink>
  ```

- [ ] **Step 2: Update index.vue hero title**

  In `web/app/pages/index.vue`, find the `<h1>` element:

  ```html
  <!-- Before -->
  <h1 class="text-5xl font-bold tracking-tight text-zinc-50">pypx</h1>

  <!-- After -->
  <h1 class="text-5xl font-bold tracking-tight text-[var(--color-brand)]">pypx</h1>
  ```

- [ ] **Step 3: Update TrendingPackages package name hover**

  In `web/app/components/TrendingPackages.vue`, find the `<h3>` inside the card link:

  ```html
  <!-- Before -->
  <h3 class="font-semibold text-zinc-50 group-hover:text-white">{{ pkg.name }}</h3>

  <!-- After -->
  <h3 class="font-semibold text-zinc-50 group-hover:text-[var(--color-brand)] transition-colors">{{ pkg.name }}</h3>
  ```

- [ ] **Step 4: Start dev server and verify visually**

  ```bash
  cd web && pnpm dev
  ```

  Open `http://localhost:3000`. Check:
  - Header "pypx" wordmark is bright mint, transitions to lighter mint on hover
  - Homepage hero "pypx" title is bright mint
  - Hovering a package card turns the package name mint

- [ ] **Step 5: Commit**

  ```bash
  git add web/app/components/AppHeader.vue web/app/pages/index.vue web/app/components/TrendingPackages.vue
  git commit -m "feat(web): apply brand color to wordmark and package name hover"
  ```

---

## Task 3: Brand focus rings on search inputs

**Files:**
- Modify: `web/app/components/AppHeader.vue`
- Modify: `web/app/pages/index.vue`

- [ ] **Step 1: Update AppHeader search input focus ring**

  In `web/app/components/AppHeader.vue`, find the search `<input>` element (around line 52). Replace the focus classes:

  ```html
  <!-- Before -->
  <input
    v-model="query"
    type="text"
    placeholder="Search packages..."
    class="w-full rounded-md border border-zinc-800 bg-zinc-900 py-1.5 pl-8 pr-12 text-sm text-zinc-50 placeholder-zinc-500 outline-none focus:border-zinc-600 focus:ring-1 focus:ring-zinc-600"
    @keydown="onKeydown"
    @focus="query.trim() && (isOpen = true)"
  />

  <!-- After -->
  <input
    v-model="query"
    type="text"
    placeholder="Search packages..."
    class="w-full rounded-md border border-zinc-800 bg-zinc-900 py-1.5 pl-8 pr-12 text-sm text-zinc-50 placeholder-zinc-500 outline-none focus:border-[var(--color-brand-light)] focus:ring-1 focus:ring-[var(--color-brand-border)]"
    @keydown="onKeydown"
    @focus="query.trim() && (isOpen = true)"
  />
  ```

- [ ] **Step 2: Update index.vue homepage search input focus ring**

  In `web/app/pages/index.vue`, find the hero search `<input>`:

  ```html
  <!-- Before -->
  <input
    v-model="searchQuery"
    type="text"
    placeholder="Search 500,000+ Python packages..."
    class="w-full rounded-lg border border-zinc-800 bg-zinc-900 px-4 py-3 text-zinc-50 placeholder-zinc-500 outline-none focus:border-zinc-600 focus:ring-1 focus:ring-zinc-600"
  />

  <!-- After -->
  <input
    v-model="searchQuery"
    type="text"
    placeholder="Search 500,000+ Python packages..."
    class="w-full rounded-lg border border-zinc-800 bg-zinc-900 px-4 py-3 text-zinc-50 placeholder-zinc-500 outline-none focus:border-[var(--color-brand-light)] focus:ring-1 focus:ring-[var(--color-brand-border)]"
  />
  ```

- [ ] **Step 3: Verify visually**

  With dev server running, click into the header search bar and the homepage search bar. Both should show a mint-tinted border and subtle mint ring on focus instead of the previous zinc ring.

- [ ] **Step 4: Commit**

  ```bash
  git add web/app/components/AppHeader.vue web/app/pages/index.vue
  git commit -m "feat(web): apply brand focus ring to search inputs"
  ```

---

## Task 4: Update PackageBadges.vue

**Files:**
- Modify: `web/app/components/PackageBadges.vue`

The component currently uses three color schemes: emerald (install size), indigo (python version + module format), and zinc (license) and amber (deps count). Standardize emerald and indigo to brand tokens. Leave zinc (license) and amber (deps count) as semantic colors — they carry distinct meaning.

- [ ] **Step 1: Replace badge color classes**

  Replace the entire `<template>` block:

  ```html
  <!-- Before -->
  <template>
    <div class="flex flex-wrap gap-2">
      <span
        v-if="pkg.install_size"
        class="inline-flex items-center rounded bg-emerald-500/10 px-2 py-0.5 font-mono text-xs text-emerald-400 ring-1 ring-emerald-500/20"
      >
        {{ formatSize(pkg.install_size) }}
      </span>
      <span
        v-if="pkg.python_versions?.min_version"
        class="inline-flex items-center rounded bg-indigo-500/10 px-2 py-0.5 font-mono text-xs text-indigo-400 ring-1 ring-indigo-500/20"
      >
        Python {{ pkg.python_versions.min_version }}+
      </span>
      <span
        v-if="pkg.module_format && pkg.module_format !== 'sdist-only'"
        class="inline-flex items-center rounded bg-indigo-500/10 px-2 py-0.5 font-mono text-xs text-indigo-400 ring-1 ring-indigo-500/20"
      >
        {{ pkg.module_format }}
      </span>
      <span
        v-if="pkg.license"
        class="inline-flex items-center rounded bg-zinc-500/10 px-2 py-0.5 font-mono text-xs text-zinc-400 ring-1 ring-zinc-500/20"
      >
        {{ pkg.license }}
      </span>
      <span
        v-if="pkg.dependencies?.required?.length"
        class="inline-flex items-center rounded bg-amber-500/10 px-2 py-0.5 font-mono text-xs text-amber-400 ring-1 ring-amber-500/20"
      >
        {{ pkg.dependencies.required.length }} deps
      </span>
    </div>
  </template>

  <!-- After -->
  <template>
    <div class="flex flex-wrap gap-2">
      <span
        v-if="pkg.install_size"
        class="inline-flex items-center rounded bg-[var(--color-brand-muted)] px-2 py-0.5 font-mono text-xs text-[var(--color-brand)] ring-1 ring-[var(--color-brand-border)]"
      >
        {{ formatSize(pkg.install_size) }}
      </span>
      <span
        v-if="pkg.python_versions?.min_version"
        class="inline-flex items-center rounded bg-[var(--color-brand-muted)] px-2 py-0.5 font-mono text-xs text-[var(--color-brand)] ring-1 ring-[var(--color-brand-border)]"
      >
        Python {{ pkg.python_versions.min_version }}+
      </span>
      <span
        v-if="pkg.module_format && pkg.module_format !== 'sdist-only'"
        class="inline-flex items-center rounded bg-[var(--color-brand-muted)] px-2 py-0.5 font-mono text-xs text-[var(--color-brand)] ring-1 ring-[var(--color-brand-border)]"
      >
        {{ pkg.module_format }}
      </span>
      <span
        v-if="pkg.license"
        class="inline-flex items-center rounded bg-zinc-500/10 px-2 py-0.5 font-mono text-xs text-zinc-400 ring-1 ring-zinc-500/20"
      >
        {{ pkg.license }}
      </span>
      <span
        v-if="pkg.dependencies?.required?.length"
        class="inline-flex items-center rounded bg-amber-500/10 px-2 py-0.5 font-mono text-xs text-amber-400 ring-1 ring-amber-500/20"
      >
        {{ pkg.dependencies.required.length }} deps
      </span>
    </div>
  </template>
  ```

- [ ] **Step 2: Verify visually**

  Navigate to any package page (e.g. `http://localhost:3000/packages/numpy`). The install size, Python version, and module format badges should be mint. License badge stays zinc. Deps count badge stays amber.

- [ ] **Step 3: Commit**

  ```bash
  git add web/app/components/PackageBadges.vue
  git commit -m "feat(web): apply brand tokens to package metadata badges"
  ```

---

## Task 5: Update InstallCommand.vue

**Files:**
- Modify: `web/app/components/InstallCommand.vue`

- [ ] **Step 1: Brand the active tab state**

  Find the `:class` binding on the tab `<button>` (around line 24). Replace the active branch:

  ```html
  <!-- Before -->
  :class="
    activeManager === mgr ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-500 hover:text-zinc-300'
  "

  <!-- After -->
  :class="
    activeManager === mgr
      ? 'bg-[var(--color-brand-muted)] text-[var(--color-brand)] ring-1 ring-[var(--color-brand-border)]'
      : 'text-zinc-500 hover:text-zinc-300'
  "
  ```

- [ ] **Step 2: Brand the copy button and its copied state**

  Find the copy `<button>` and both SVG icons inside it. Replace:

  ```html
  <!-- Before -->
  <button
    class="shrink-0 rounded p-1.5 text-zinc-500 transition-colors hover:bg-zinc-800 hover:text-zinc-300"
    :title="copied ? 'Copied!' : 'Copy'"
    @click="copy()"
  >
    <svg
      v-if="copied"
      xmlns="http://www.w3.org/2000/svg"
      class="h-4 w-4 text-emerald-400"
      ...
    >
      <polyline points="20 6 9 17 4 12" />
    </svg>
    <svg
      v-else
      xmlns="http://www.w3.org/2000/svg"
      class="h-4 w-4"
      ...
    >
      ...
    </svg>
  </button>

  <!-- After -->
  <button
    class="shrink-0 rounded p-1.5 text-zinc-500 transition-colors hover:bg-[var(--color-brand-muted)] hover:text-[var(--color-brand)]"
    :title="copied ? 'Copied!' : 'Copy'"
    @click="copy()"
  >
    <svg
      v-if="copied"
      xmlns="http://www.w3.org/2000/svg"
      class="h-4 w-4 text-[var(--color-brand)]"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <polyline points="20 6 9 17 4 12" />
    </svg>
    <svg
      v-else
      xmlns="http://www.w3.org/2000/svg"
      class="h-4 w-4"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
    </svg>
  </button>
  ```

- [ ] **Step 3: Verify visually**

  On a package page, check the install command widget:
  - The active tab (e.g. "uv") should have a mint-tinted background and mint text
  - Hovering the copy icon should show mint
  - Clicking copy should show the checkmark in mint

- [ ] **Step 4: Commit**

  ```bash
  git add web/app/components/InstallCommand.vue
  git commit -m "feat(web): apply brand tokens to install command widget"
  ```

---

## Task 6: Update PackageVersions.vue

**Files:**
- Modify: `web/app/components/PackageVersions.vue`

- [ ] **Step 1: Brand version link hover and install size**

  Find the version link `NuxtLink` in the table (around line 70):

  ```html
  <!-- Before -->
  <NuxtLink
    :to="`/packages/${name}/${v.version}`"
    class="font-mono hover:text-indigo-400 text-zinc-200 transition-colors"
    @click.stop
  >

  <!-- After -->
  <NuxtLink
    :to="`/packages/${name}/${v.version}`"
    class="font-mono hover:text-[var(--color-brand)] text-zinc-200 transition-colors"
    @click.stop
  >
  ```

  Find the install size `<td>` (around line 75):

  ```html
  <!-- Before -->
  <td class="py-3 pr-6 font-mono text-emerald-400">{{ formatSize(v.install_size) }}</td>

  <!-- After -->
  <td class="py-3 pr-6 font-mono text-[var(--color-brand)]">{{ formatSize(v.install_size) }}</td>
  ```

- [ ] **Step 2: Brand the GitHub changelog link**

  Find the `<a>` tag linking to GitHub (at the bottom of the changelog expansion row):

  ```html
  <!-- Before -->
  <a
    :href="entry.url"
    target="_blank"
    class="text-xs text-indigo-400 hover:text-indigo-300 transition-colors"
  >View on GitHub →</a>

  <!-- After -->
  <a
    :href="entry.url"
    target="_blank"
    class="text-xs text-[var(--color-brand)] hover:text-[var(--color-brand-light)] transition-colors"
  >View on GitHub →</a>
  ```

- [ ] **Step 3: Verify visually**

  On a package page, go to the Versions tab:
  - Hovering a version number should turn it mint
  - Install sizes (the size column) should be mint
  - Expand a changelog entry and the "View on GitHub →" link should be mint

- [ ] **Step 4: Commit**

  ```bash
  git add web/app/components/PackageVersions.vue
  git commit -m "feat(web): apply brand tokens to package versions table"
  ```

---

## Task 7: Update PackageStats.vue

**Files:**
- Modify: `web/app/components/PackageStats.vue`

The stats component uses three bar chart sections with distinct colors (emerald for overall trend, indigo for Python versions, amber for OS). Brand the overall trend only — Python version and OS breakdowns keep their semantic colors since they distinguish different data dimensions. Also brand the period toggle active state.

- [ ] **Step 1: Brand the period toggle active state**

  Find the `:class` binding on the period `<button>` (around line 73):

  ```html
  <!-- Before -->
  :class="
    period === opt.value ? 'bg-zinc-700 text-zinc-100' : 'text-zinc-500 hover:text-zinc-300'
  "

  <!-- After -->
  :class="
    period === opt.value
      ? 'bg-[var(--color-brand-muted)] text-[var(--color-brand)] ring-1 ring-[var(--color-brand-border)]'
      : 'text-zinc-500 hover:text-zinc-300'
  "
  ```

- [ ] **Step 2: Brand the overall download trend bars**

  Find the "Download Trends" section bar (around line 103). This is the `div` with `class="h-4 rounded-sm bg-emerald-500/30"`:

  ```html
  <!-- Before -->
  <div
    class="h-4 rounded-sm bg-emerald-500/30"
    :style="{ width: barWidth(point.downloads, maxDownloads(overallTrend)) }"
  />

  <!-- After -->
  <div
    class="h-4 rounded-sm bg-[var(--color-brand-border)]"
    :style="{ width: barWidth(point.downloads, maxDownloads(overallTrend)) }"
  />
  ```

  Use `--color-brand-border` (25% opacity) rather than `--color-brand-muted` (8%) — bars need enough opacity to be readable as data.

- [ ] **Step 3: Verify visually**

  On a package page, go to the Stats tab:
  - The active period button ("4 weeks" / "3 months" / "6 months") should show mint background
  - Overall download trend bars should be mint-tinted
  - Python version bars remain indigo; OS bars remain amber

- [ ] **Step 4: Commit**

  ```bash
  git add web/app/components/PackageStats.vue
  git commit -m "feat(web): apply brand tokens to stats period toggle and trend bars"
  ```

---

## Task 8: Final visual pass

- [ ] **Step 1: Run through all pages with dev server**

  ```bash
  cd web && pnpm dev
  ```

  Check each surface:
  - `/` — homepage: mint hero title, mint wordmark in header, mint search focus ring, mint package name on card hover
  - `/search?q=numpy` — search results: mint focus ring on search bar
  - `/packages/numpy` — package page: mint badges, mint active install tab, mint copy hover, mint version link hover, mint install sizes, mint GitHub link, mint period toggle, mint trend bars
  - Any prose/changelog content: mint links

- [ ] **Step 2: Check for any remaining indigo or raw emerald references**

  ```bash
  cd web && grep -r "indigo\|emerald" app/components app/pages app/layouts app/assets
  ```

  Expected remaining hits (these are intentional):
  - `PackageStats.vue`: `text-indigo-400`, `bg-indigo-500/30` (Python version bars — semantic, keep)
  - `PackageStats.vue`: `text-amber-400`, `bg-amber-500/30` (OS bars — semantic, keep)
  - `PackageBadges.vue`: `text-amber-400`, `bg-amber-500/10` (deps count — semantic, keep)
  - `PackageBadges.vue`: `text-zinc-400`, `bg-zinc-500/10` (license — neutral, keep)

  Any other hits are unintentional — fix them.

- [ ] **Step 3: Commit if any fixes were made, then tag the work done**

  ```bash
  git add -p
  git commit -m "fix(web): clean up remaining unbranded indigo/emerald references"
  ```
