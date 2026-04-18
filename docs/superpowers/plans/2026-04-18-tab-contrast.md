# Tab Navigation Contrast & Hover Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bump inactive tab text to always-readable contrast and replace the plain text-lightening hover with a muted brand-green underline accent.

**Architecture:** Pure CSS class changes in a single Vue file. No logic, no new components, no new CSS rules — only Tailwind utility classes on existing elements.

**Tech Stack:** Vue 3, Tailwind 4, Nuxt 4

---

### Task 1: Update tab classes in `[name].vue`

**Files:**
- Modify: `web/app/pages/packages/[name].vue:145-164`

Current state of the relevant section (lines 139–165):

```vue
<div class="mb-6 flex gap-1 overflow-x-auto border-b border-subtle pb-0">
  <!-- In-page tabs -->
  <button
    v-for="tab in inPageTabs"
    :key="tab.key"
    class="group cursor-pointer whitespace-nowrap rounded-t px-4 py-2 text-sm font-medium transition-colors"
    :class="
      activeTab === tab.key
        ? 'bg-raised text-primary'
        : 'text-muted hover:text-zinc-700 dark:hover:text-zinc-300'
    "
    @click="activeTab = tab.key"
  >
    {{ tab.label }}
    <KbdHint :keys="tab.shortcut" />
  </button>

  <!-- Docs tab — link to separate route, shown only when available -->
  <NuxtLink
    v-if="docsData?.available"
    :to="`/packages/${pkg.name}/docs`"
    class="cursor-pointer whitespace-nowrap rounded-t px-4 py-2 text-sm font-medium text-muted transition-colors hover:text-zinc-700 dark:hover:text-zinc-300"
  >
    Docs
  </NuxtLink>
</div>
```

- [ ] **Step 1: Apply the three class changes**

Replace the tab `class` and `:class` bindings, and the NuxtLink `class`:

```vue
<div class="mb-6 flex gap-1 overflow-x-auto border-b border-subtle pb-0">
  <!-- In-page tabs -->
  <button
    v-for="tab in inPageTabs"
    :key="tab.key"
    class="group cursor-pointer whitespace-nowrap rounded-t px-4 py-2 text-sm font-medium transition-colors border-b-2 border-transparent"
    :class="
      activeTab === tab.key
        ? 'bg-raised text-primary'
        : 'text-zinc-700 dark:text-zinc-300 hover:border-[rgba(4,120,87,0.65)] dark:hover:border-[rgba(74,222,128,0.65)]'
    "
    @click="activeTab = tab.key"
  >
    {{ tab.label }}
    <KbdHint :keys="tab.shortcut" />
  </button>

  <!-- Docs tab — link to separate route, shown only when available -->
  <NuxtLink
    v-if="docsData?.available"
    :to="`/packages/${pkg.name}/docs`"
    class="cursor-pointer whitespace-nowrap rounded-t px-4 py-2 text-sm font-medium text-zinc-700 dark:text-zinc-300 transition-colors border-b-2 border-transparent hover:border-[rgba(4,120,87,0.65)] dark:hover:border-[rgba(74,222,128,0.65)]"
  >
    Docs
  </NuxtLink>
</div>
```

What changed:
- `border-b-2 border-transparent` added to the shared `class` on `<button>` — gives all tabs consistent height so nothing jumps on hover
- Active `:class`: `bg-raised text-primary` — unchanged
- Inactive `:class`: `text-muted hover:text-zinc-700 dark:hover:text-zinc-300` → `text-zinc-700 dark:text-zinc-300 hover:border-[rgba(4,120,87,0.65)] dark:hover:border-[rgba(74,222,128,0.65)]`
- `<NuxtLink>`: same pattern — `text-muted` → `text-zinc-700 dark:text-zinc-300`, old hover text classes removed, green border hover added, `border-b-2 border-transparent` added

- [ ] **Step 2: Start the dev server and visually verify**

```bash
cd web && npm run dev
```

Open `http://localhost:3000/packages/requests` (or any package page) and check:

1. **Default state:** inactive tabs should be clearly readable without hovering — not dim/muted
2. **Hover state:** hovering an inactive tab shows a green underline, no text color change
3. **Active state:** active tab still shows the `bg-raised` pill, no underline
4. **No height jump:** switching between hover and non-hover states should not shift tab height
5. **Light mode:** toggle to light mode — hover underline should be a darker emerald green (not the bright mint)
6. **Docs tab:** if visible, same behavior as other inactive tabs

- [ ] **Step 3: Commit**

```bash
git add web/app/pages/packages/\[name\].vue
git commit -m "feat: improve tab contrast and add green underline hover accent"
```
