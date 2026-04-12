# Mobile Friendliness & Cursor Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix `cursor-pointer` on all interactive buttons and make the package detail UI responsive on mobile viewports.

**Architecture:** Pure CSS class changes across 5 Vue SFCs. No logic changes, no new components, no new files. Each task is a focused edit to one component followed by a commit.

**Tech Stack:** Vue 3, Nuxt 4, Tailwind CSS v4, pnpm

---

## File Map

| File | Change |
|------|--------|
| `web/app/components/InstallCommand.vue` | Add `cursor-pointer` to manager tab buttons and copy button |
| `web/app/pages/packages/[name].vue` | Add `cursor-pointer` + `whitespace-nowrap` to tab buttons; `overflow-x-auto` on tab row; `flex-wrap` on package header |
| `web/app/components/PackageStats.vue` | Add `cursor-pointer` to period buttons; `flex-wrap` on period row |
| `web/app/components/PackageVersions.vue` | Hide Size + Format columns on mobile with `hidden sm:table-cell` |
| `web/app/components/AppHeader.vue` | Hide `⌘K` kbd hint on mobile with `hidden sm:inline` |

---

### Task 1: Cursor + mobile fixes in InstallCommand.vue

**Files:**
- Modify: `web/app/components/InstallCommand.vue`

- [ ] **Step 1: Add `cursor-pointer` to manager tab buttons**

In `InstallCommand.vue`, the manager tab buttons currently have:
```html
<button
  v-for="mgr in managers"
  :key="mgr"
  class="rounded-t px-3 py-1.5 text-xs font-medium transition-colors"
```

Change to:
```html
<button
  v-for="mgr in managers"
  :key="mgr"
  class="cursor-pointer rounded-t px-3 py-1.5 text-xs font-medium transition-colors"
```

- [ ] **Step 2: Add `cursor-pointer` to the copy button**

The copy button currently has:
```html
<button
  class="shrink-0 rounded p-1.5 text-zinc-500 transition-colors hover:bg-[var(--color-brand-muted)] hover:text-[var(--color-brand)]"
```

Change to:
```html
<button
  class="cursor-pointer shrink-0 rounded p-1.5 text-zinc-500 transition-colors hover:bg-[var(--color-brand-muted)] hover:text-[var(--color-brand)]"
```

- [ ] **Step 3: Commit**

```bash
git add web/app/components/InstallCommand.vue
git commit -m "fix(web): add cursor-pointer to install command buttons"
```

---

### Task 2: Cursor + mobile fixes in [name].vue

**Files:**
- Modify: `web/app/pages/packages/[name].vue`

- [ ] **Step 1: Add `flex-wrap` to the package header**

The package name + version badge container currently has:
```html
<div class="flex items-baseline gap-3">
```

Change to:
```html
<div class="flex flex-wrap items-baseline gap-3">
```

- [ ] **Step 2: Add `overflow-x-auto` to tab row and `cursor-pointer` + `whitespace-nowrap` to tab buttons**

The tab row container currently has:
```html
<div class="mb-6 flex gap-1 border-b border-zinc-800 pb-0">
```

Change to:
```html
<div class="mb-6 flex gap-1 overflow-x-auto border-b border-zinc-800 pb-0">
```

The tab buttons currently have:
```html
<button
  v-for="tab in tabs"
  :key="tab.key"
  class="rounded-t px-4 py-2 text-sm font-medium transition-colors"
```

Change to:
```html
<button
  v-for="tab in tabs"
  :key="tab.key"
  class="cursor-pointer whitespace-nowrap rounded-t px-4 py-2 text-sm font-medium transition-colors"
```

- [ ] **Step 3: Commit**

```bash
git add web/app/pages/packages/[name].vue
git commit -m "fix(web): add cursor-pointer, scrollable tabs, and flex-wrap on package header"
```

---

### Task 3: Cursor + mobile fixes in PackageStats.vue

**Files:**
- Modify: `web/app/components/PackageStats.vue`

- [ ] **Step 1: Add `cursor-pointer` to period buttons and `flex-wrap` to period row**

The period row container currently has:
```html
<div class="mb-6 flex items-center gap-1">
```

Change to:
```html
<div class="mb-6 flex flex-wrap items-center gap-1">
```

The period buttons currently have:
```html
<button
  v-for="opt in periodOptions"
  :key="opt.value"
  class="rounded-md px-3 py-1.5 font-mono text-xs transition-colors"
```

Change to:
```html
<button
  v-for="opt in periodOptions"
  :key="opt.value"
  class="cursor-pointer rounded-md px-3 py-1.5 font-mono text-xs transition-colors"
```

- [ ] **Step 2: Commit**

```bash
git add web/app/components/PackageStats.vue
git commit -m "fix(web): add cursor-pointer to stats period buttons and flex-wrap on period row"
```

---

### Task 4: Mobile column hiding in PackageVersions.vue

**Files:**
- Modify: `web/app/components/PackageVersions.vue`

- [ ] **Step 1: Hide Size and Format table headers on mobile**

The table header row currently has:
```html
<tr class="border-b border-zinc-800 text-left text-zinc-500">
  <th class="pb-2 pr-6 font-medium">Version</th>
  <th class="pb-2 pr-6 font-medium">Released</th>
  <th class="pb-2 pr-6 font-medium">Size</th>
  <th class="pb-2 font-medium">Format</th>
</tr>
```

Change to:
```html
<tr class="border-b border-zinc-800 text-left text-zinc-500">
  <th class="pb-2 pr-6 font-medium">Version</th>
  <th class="pb-2 pr-6 font-medium">Released</th>
  <th class="hidden pb-2 pr-6 font-medium sm:table-cell">Size</th>
  <th class="hidden pb-2 font-medium sm:table-cell">Format</th>
</tr>
```

- [ ] **Step 2: Hide Size and Format table cells on mobile**

The version row `<td>` cells currently have:
```html
<td class="py-3 pr-6 font-mono text-[var(--color-brand)]">
  {{ formatSize(v.install_size) }}
</td>
<td class="py-3 font-mono text-xs text-zinc-500">{{ v.module_format || "—" }}</td>
```

Change to:
```html
<td class="hidden py-3 pr-6 font-mono text-[var(--color-brand)] sm:table-cell">
  {{ formatSize(v.install_size) }}
</td>
<td class="hidden py-3 font-mono text-xs text-zinc-500 sm:table-cell">{{ v.module_format || "—" }}</td>
```

The changelog expanded row uses `colspan="4"` — leave it unchanged. Hidden columns still count toward colspan in HTML, so the cell continues to span the full row correctly.

- [ ] **Step 3: Commit**

```bash
git add web/app/components/PackageVersions.vue
git commit -m "fix(web): hide Size and Format columns on mobile in versions table"
```

---

### Task 5: Hide ⌘K hint on mobile in AppHeader.vue

**Files:**
- Modify: `web/app/components/AppHeader.vue`

- [ ] **Step 1: Add `hidden sm:inline` to the kbd element**

The `<kbd>` element currently has:
```html
<kbd
  class="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 rounded bg-zinc-800 px-1.5 py-0.5 font-mono text-[10px] text-zinc-500"
>
  ⌘K
</kbd>
```

Change to:
```html
<kbd
  class="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 hidden rounded bg-zinc-800 px-1.5 py-0.5 font-mono text-[10px] text-zinc-500 sm:inline"
>
  ⌘K
</kbd>
```

- [ ] **Step 2: Commit**

```bash
git add web/app/components/AppHeader.vue
git commit -m "fix(web): hide cmd-K hint on mobile in header search"
```
