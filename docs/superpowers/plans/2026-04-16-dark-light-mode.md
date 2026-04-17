# Dark / Light Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a three-way (light / dark / system) theme system to pypx with FOUC-free SSR, a header toggle, command palette integration, and accessible brand colors.

**Architecture:** Semantic CSS tokens registered as Tailwind 4 utilities flip all colors at the `.dark` boundary set by `@nuxtjs/color-mode`. The 168 hardcoded zinc classes across 22 files are replaced mechanically with token utility classes; one-off mid-range colors use `dark:` variants. Shiki uses its built-in dual-theme API so cached HTML from the Go API is already theme-aware.

**Tech Stack:** Nuxt 4, Vue 3, Tailwind 4 (`@tailwindcss/vite`), `@nuxtjs/color-mode` v4 (already installed), Shiki (`one-dark-pro` + `github-light`), `@vueuse/nuxt`

**Branch:** `feat.dark-light-mode`

---

## Color Mapping Reference

Use this table throughout all migration tasks:

| Dark class | Replacement | Notes |
|---|---|---|
| `bg-zinc-950` | `bg-base` | token |
| `bg-zinc-950/80` | `bg-base/80` | opacity preserved |
| `bg-zinc-900` | `bg-surface` | token |
| `bg-zinc-900/50` | `bg-surface` | drop opacity — was purely decorative |
| `bg-zinc-900/95` | `bg-surface/95` | opacity preserved |
| `bg-zinc-800` | `bg-raised` | token |
| `bg-zinc-800/30` | `bg-raised/30` | opacity preserved |
| `bg-zinc-800/50` | `bg-raised/50` | opacity preserved |
| `border-zinc-800` | `border-subtle` | token |
| `border-zinc-700` | `border-zinc-300 dark:border-zinc-700` | dark: variant |
| `border-zinc-900` | `border-base` | token |
| `text-zinc-50` | `text-primary` | token |
| `text-zinc-100` | `text-zinc-900 dark:text-zinc-100` | dark: variant |
| `text-zinc-200` | `text-zinc-800 dark:text-zinc-200` | dark: variant |
| `text-zinc-300` | `text-zinc-700 dark:text-zinc-300` | dark: variant |
| `text-zinc-400` | `text-muted` | token |
| `text-zinc-500` | `text-muted` | token |
| `text-zinc-600` | `text-zinc-400 dark:text-zinc-600` | dark: variant |
| `text-zinc-700` | `text-zinc-400 dark:text-zinc-700` | dark: variant |
| `placeholder-zinc-500` | `placeholder-muted` | token |
| `border-zinc-700 border-t-zinc-300` (spinner) | `border-subtle border-t-primary` | tokens |

---

## Task 1: Configure @nuxtjs/color-mode and Tailwind dark variant

**Files:**
- Modify: `web/nuxt.config.ts`
- Modify: `web/app/assets/css/main.css`

- [ ] **Step 1: Add @nuxtjs/color-mode to nuxt.config.ts**

Replace the modules array and add colorMode config:

```ts
// web/nuxt.config.ts
export default defineNuxtConfig({
  compatibilityDate: '2026-04-09',
  devtools: { enabled: true },

  modules: [
    '@vueuse/nuxt',
    '@nuxtjs/seo',
    '@nuxtjs/color-mode',
  ],

  colorMode: {
    classSuffix: '',        // writes 'dark' not 'dark-mode' to <html>
    defaultValue: 'system',
    storageKey: 'pypx-color-mode',
  },

  css: ['~/assets/css/main.css'],
  // ... rest unchanged
```

- [ ] **Step 2: Add @variant dark to main.css**

Add directly after the `@import "tailwindcss"` line:

```css
@import "tailwindcss";
@variant dark (&:where(.dark, .dark *));
```

- [ ] **Step 3: Start the dev server and verify**

```bash
cd web && npm run dev
```

Open http://localhost:3000 in a browser. Open DevTools → Elements. The `<html>` element should have class `dark` or `light` depending on your OS preference. No console errors.

- [ ] **Step 4: Commit**

```bash
git add web/nuxt.config.ts web/app/assets/css/main.css
git commit -m "feat: configure @nuxtjs/color-mode with system default"
```

---

## Task 2: Add semantic tokens, brand tokens, Shiki CSS, prose, and decorative overrides to main.css

**Files:**
- Modify: `web/app/assets/css/main.css`

This is the single largest CSS change. Do it in one edit — replace the entire `:root {}` block and add new sections.

- [ ] **Step 1: Replace the :root block and add all token definitions**

Find the current `:root` block (lines 4–22) and replace with:

```css
@theme inline {
  --color-base:    var(--color-base);
  --color-surface: var(--color-surface);
  --color-raised:  var(--color-raised);
  --color-subtle:  var(--color-subtle);
  --color-primary: var(--color-primary);
  --color-muted:   var(--color-muted);
}

:root {
  --font-sans: "Geist", ui-sans-serif, system-ui, sans-serif;
  --font-mono: "Geist Mono", ui-monospace, "SFMono-Regular", monospace;

  /* Structural tokens — light mode */
  --color-base:    #fafafa;
  --color-surface: #ffffff;
  --color-raised:  #f4f4f5;
  --color-subtle:  #e4e4e7;
  --color-primary: #18181b;
  --color-muted:   #71717a;

  /* Brand — light mode (emerald-700, 5.25:1 on #fafafa) */
  --color-brand:        #047857;
  --color-brand-light:  #059669;
  --color-brand-muted:  rgba(4, 120, 87, 0.08);
  --color-brand-border: rgba(4, 120, 87, 0.25);

  /* Python syntax tokens — light mode */
  --py-keyword:   #7c3aed;
  --py-name:      #b45309;
  --py-param:     #374151;
  --py-type:      #1d4ed8;
  --py-default:   #047857;
  --py-punct:     #6b7280;
  --py-decorator: #be123c;
}

.dark {
  /* Structural tokens — dark mode */
  --color-base:    #09090b;
  --color-surface: #18181b;
  --color-raised:  #27272a;
  --color-subtle:  #27272a;
  --color-primary: #fafafa;
  --color-muted:   #a1a1aa;

  /* Brand — dark mode (original mint) */
  --color-brand:        #4ade80;
  --color-brand-light:  #86efac;
  --color-brand-muted:  rgba(74, 222, 128, 0.08);
  --color-brand-border: rgba(74, 222, 128, 0.25);

  /* Python syntax tokens — dark mode (One Dark) */
  --py-keyword:   #c678dd;
  --py-name:      #e5c07b;
  --py-param:     #abb2bf;
  --py-type:      #61afef;
  --py-default:   #98c379;
  --py-punct:     #636d83;
  --py-decorator: #e06c75;
}
```

- [ ] **Step 2: Update the prose styles — replace .prose-invert with token-based .prose**

Find `.prose-invert` block (starts around line 142) and replace the entire `.prose-invert { ... }` block with:

```css
/* Prose colors adapt via tokens — .prose-invert no longer needed */
.prose {
  & a {
    color: var(--color-brand);
  }
  & a:hover {
    color: var(--color-brand-light);
  }
  & h1, & h2, & h3, & h4, & h5, & h6 {
    color: var(--color-primary);
  }
  & strong {
    color: var(--color-primary);
  }
  & code {
    background-color: var(--color-raised);
    color: var(--color-primary);
  }
  & pre {
    background-color: var(--color-surface);
    border: 1px solid var(--color-subtle);
  }
  & blockquote {
    border-left-color: var(--color-subtle);
    color: var(--color-muted);
  }
  & th {
    background-color: var(--color-raised);
  }
  & th, & td {
    border-color: var(--color-subtle);
  }
  & hr {
    border-color: var(--color-subtle);
  }
}
```

Also remove the hardcoded `border-left: 3px solid var(--color-zinc-700)` line from the base `.prose blockquote` rule — replace it with `border-left: 3px solid var(--color-subtle)`.

- [ ] **Step 3: Add Shiki dual-theme CSS rules**

Add after the prose section:

```css
/* Shiki dual-theme: light by default, dark under .dark */
.shiki,
.shiki span {
  color: var(--shiki-light) !important;
  background-color: var(--shiki-light-bg) !important;
}
.dark .shiki,
.dark .shiki span {
  color: var(--shiki-dark) !important;
  background-color: var(--shiki-dark-bg) !important;
}
```

- [ ] **Step 4: Update body::before and body::after for light mode**

Replace the two existing body pseudo-element blocks at the bottom of main.css:

```css
/* Global grid background */
body::before {
  content: "";
  position: fixed;
  inset: 0;
  z-index: -1;
  background-image:
    linear-gradient(rgba(4, 120, 87, 0.03) 1px, transparent 1px),
    linear-gradient(90deg, rgba(4, 120, 87, 0.03) 1px, transparent 1px);
  background-size: 32px 32px;
  pointer-events: none;
}

.dark body::before {
  background-image:
    linear-gradient(rgba(74, 222, 128, 0.04) 1px, transparent 1px),
    linear-gradient(90deg, rgba(74, 222, 128, 0.04) 1px, transparent 1px);
}

body::after {
  content: "";
  position: fixed;
  top: -120px;
  right: -120px;
  width: 480px;
  height: 480px;
  background: radial-gradient(circle, rgba(4, 120, 87, 0.04), transparent 70%);
  border-radius: 50%;
  z-index: -1;
  pointer-events: none;
}

.dark body::after {
  background: radial-gradient(circle, rgba(74, 222, 128, 0.08), transparent 70%);
}
```

- [ ] **Step 5: Verify dev server still starts cleanly**

```bash
cd web && npm run dev
```

Expected: no CSS errors, page loads in both light and dark mode (test by temporarily adding `class="dark"` to `<html>` manually in DevTools).

- [ ] **Step 6: Commit**

```bash
git add web/app/assets/css/main.css
git commit -m "feat: add semantic tokens, brand tokens, Shiki CSS, and prose theming"
```

---

## Task 3: Update Shiki to dual-theme

**Files:**
- Modify: `web/app/utils/shikiHighlight.ts`

- [ ] **Step 1: Update shikiHighlight.ts**

Replace the entire file:

```ts
import { createHighlighter, type Highlighter } from "shiki";

let highlighterPromise: Promise<Highlighter> | null = null;

function getHighlighter(): Promise<Highlighter> {
  if (!highlighterPromise) {
    highlighterPromise = createHighlighter({
      themes: ["one-dark-pro", "github-light"],
      langs: ["python"],
    });
  }
  return highlighterPromise;
}

export async function highlightPython(code: string): Promise<string> {
  if (!import.meta.server) {
    return `<pre><code class="language-python">${code}</code></pre>`;
  }

  const highlighter = await getHighlighter();
  return highlighter.codeToHtml(code, {
    lang: "python",
    themes: {
      dark: "one-dark-pro",
      light: "github-light",
    },
  });
}
```

- [ ] **Step 2: Verify the output contains CSS variables**

```bash
cd web && node -e "
const { highlightPython } = await import('./app/utils/shikiHighlight.ts');
// This won't work directly but check via dev server
"
```

Instead, navigate to any package docs page (e.g. http://localhost:3000/packages/requests/docs), open DevTools, inspect a `<pre>` element inside a `.shiki` block. Confirm it has `--shiki-light` and `--shiki-dark` CSS variables on `<span>` elements, not hardcoded color values.

- [ ] **Step 3: Commit**

```bash
git add web/app/utils/shikiHighlight.ts
git commit -m "feat: switch Shiki to dual-theme for light/dark code blocks"
```

---

## Task 4: Create ThemeToggle.vue component

**Files:**
- Create: `web/app/components/ThemeToggle.vue`

- [ ] **Step 1: Create ThemeToggle.vue**

```vue
<script setup lang="ts">
const colorMode = useColorMode();

const modes = ['light', 'dark', 'system'] as const;

function cycle() {
  const current = modes.indexOf(colorMode.preference as typeof modes[number]);
  colorMode.preference = modes[(current + 1) % modes.length];
}

const ariaLabel = computed(() => {
  const next = modes[(modes.indexOf(colorMode.preference as typeof modes[number]) + 1) % modes.length];
  return `Switch to ${next} mode`;
});
</script>

<template>
  <button
    type="button"
    :aria-label="ariaLabel"
    class="rounded-md p-1.5 text-muted transition-colors hover:bg-raised hover:text-primary"
    @click="cycle"
  >
    <!-- Sun — light mode -->
    <svg
      v-if="colorMode.value === 'light'"
      xmlns="http://www.w3.org/2000/svg"
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      <circle cx="12" cy="12" r="5" />
      <line x1="12" y1="1" x2="12" y2="3" />
      <line x1="12" y1="21" x2="12" y2="23" />
      <line x1="4.22" y1="4.22" x2="5.64" y2="5.64" />
      <line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
      <line x1="1" y1="12" x2="3" y2="12" />
      <line x1="21" y1="12" x2="23" y2="12" />
      <line x1="4.22" y1="19.78" x2="5.64" y2="18.36" />
      <line x1="18.36" y1="5.64" x2="19.78" y2="4.22" />
    </svg>
    <!-- Moon — dark mode -->
    <svg
      v-else-if="colorMode.value === 'dark'"
      xmlns="http://www.w3.org/2000/svg"
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
    </svg>
    <!-- Monitor — system -->
    <svg
      v-else
      xmlns="http://www.w3.org/2000/svg"
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      <rect x="2" y="3" width="20" height="14" rx="2" />
      <line x1="8" y1="21" x2="16" y2="21" />
      <line x1="12" y1="17" x2="12" y2="21" />
    </svg>
  </button>
</template>
```

- [ ] **Step 2: Write a unit test for the cycle logic**

Create `web/app/components/__tests__/ThemeToggle.test.ts`:

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import ThemeToggle from '../ThemeToggle.vue';

const mockColorMode = { preference: 'system', value: 'dark' };

vi.mock('#imports', () => ({
  useColorMode: () => mockColorMode,
  computed: (fn: () => unknown) => ({ value: fn() }),
}));

describe('ThemeToggle', () => {
  beforeEach(() => {
    mockColorMode.preference = 'system';
    mockColorMode.value = 'dark';
  });

  it('cycles system → light on first click', async () => {
    const wrapper = mount(ThemeToggle);
    await wrapper.find('button').trigger('click');
    expect(mockColorMode.preference).toBe('light');
  });

  it('cycles light → dark', async () => {
    mockColorMode.preference = 'light';
    const wrapper = mount(ThemeToggle);
    await wrapper.find('button').trigger('click');
    expect(mockColorMode.preference).toBe('dark');
  });

  it('cycles dark → system', async () => {
    mockColorMode.preference = 'dark';
    const wrapper = mount(ThemeToggle);
    await wrapper.find('button').trigger('click');
    expect(mockColorMode.preference).toBe('system');
  });
});
```

- [ ] **Step 3: Run the test**

```bash
cd web && npm run test -- ThemeToggle
```

Expected: 3 tests pass.

- [ ] **Step 4: Commit**

```bash
git add web/app/components/ThemeToggle.vue web/app/components/__tests__/ThemeToggle.test.ts
git commit -m "feat: add ThemeToggle component with cycle logic"
```

---

## Task 5: Update AppHeader.vue — zinc migration + ThemeToggle

**Files:**
- Modify: `web/app/components/AppHeader.vue`

- [ ] **Step 1: Update AppHeader.vue**

Replace the `<template>` section with:

```html
<template>
  <header class="sticky top-0 z-50 border-b border-subtle bg-base/80 backdrop-blur-sm">
    <div class="mx-auto flex h-14 max-w-6xl items-center gap-6 px-4">
      <NuxtLink to="/" class="flex items-center gap-2">
        <span
          class="text-lg font-bold tracking-tight text-[var(--color-brand)] hover:text-[var(--color-brand-light)] transition-colors"
          >pypx</span
        >
      </NuxtLink>

      <div v-if="!hideSearch" ref="searchWrapper" class="relative flex-1 max-w-md">
        <form @submit.prevent>
          <div class="relative">
            <svg
              class="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted"
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <circle cx="11" cy="11" r="8" />
              <path d="m21 21-4.3-4.3" />
            </svg>
            <input
              v-model="query"
              type="text"
              placeholder="Search packages..."
              aria-label="Search Python packages"
              class="w-full rounded-md border border-subtle bg-surface py-1.5 pl-8 pr-12 text-sm text-primary placeholder-muted outline-none focus:border-[var(--color-brand-light)] focus:ring-1 focus:ring-[var(--color-brand-border)]"
              @keydown="onKeydown"
              @focus="query.trim() && (isOpen = true)"
            />
            <kbd
              class="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 hidden rounded bg-raised px-1.5 py-0.5 font-mono text-[10px] text-muted sm:inline"
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

      <ThemeToggle class="ml-auto" />
    </div>
  </header>
</template>
```

- [ ] **Step 2: Verify in dev server**

Navigate to any page. Confirm the ThemeToggle button appears at the far right of the header. Click it — the icon should cycle Sun → Moon → Monitor → Sun. The page background should switch between light and dark.

- [ ] **Step 3: Commit**

```bash
git add web/app/components/AppHeader.vue
git commit -m "feat: migrate AppHeader to tokens and add ThemeToggle"
```

---

## Task 6: Update CommandPalette.vue — zinc migration + Theme section

**Files:**
- Modify: `web/app/components/CommandPalette.vue`

- [ ] **Step 1: Replace the entire file**

```vue
<script setup lang="ts">
const { query, results, selectedIndex, isOpen, isLoading, onKeydown, navigateToResult, reset } =
  useSearchTypeahead();
const inputRef = ref<HTMLInputElement | null>(null);
const isModalOpen = ref(false);
const colorMode = useColorMode();

function openModal() {
  isModalOpen.value = true;
  reset();
  nextTick(() => inputRef.value?.focus());
}

function closeModal() {
  isModalOpen.value = false;
  reset();
}

function onModalKeydown(e: KeyboardEvent) {
  if (e.key === "Escape") {
    e.preventDefault();
    closeModal();
    return;
  }
  onKeydown(e);
}

const route = useRoute();

function onGlobalKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === "k") {
    e.preventDefault();
    if (route.path === "/") {
      const heroInput = document.querySelector<HTMLInputElement>('main input[type="text"]');
      if (heroInput) {
        heroInput.focus();
        return;
      }
    }
    if (isModalOpen.value) {
      closeModal();
    } else {
      openModal();
    }
  }
}

onMounted(() => {
  window.addEventListener("keydown", onGlobalKeydown);
});

onUnmounted(() => {
  window.removeEventListener("keydown", onGlobalKeydown);
});
</script>

<template>
  <Teleport to="body">
    <Transition
      enter-active-class="transition duration-150 ease-out"
      enter-from-class="opacity-0"
      enter-to-class="opacity-100"
      leave-active-class="transition duration-100 ease-in"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0"
    >
      <div
        v-if="isModalOpen"
        class="fixed inset-0 z-[100] flex items-start justify-center bg-black/60 pt-[20vh]"
        @mousedown.self="closeModal"
      >
        <div
          class="w-full max-w-lg overflow-hidden rounded-xl border border-subtle bg-surface shadow-2xl"
        >
          <!-- Input area -->
          <div class="flex items-center gap-3 border-b border-subtle px-4 py-3">
            <svg
              class="size-4 shrink-0 text-muted"
              xmlns="http://www.w3.org/2000/svg"
              fill="none"
              viewBox="0 0 24 24"
              stroke-width="1.5"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z"
              />
            </svg>
            <input
              ref="inputRef"
              v-model="query"
              type="text"
              placeholder="Search packages..."
              aria-label="Search Python packages"
              class="min-w-0 flex-1 bg-transparent text-sm text-primary placeholder-muted outline-none"
              @keydown="onModalKeydown"
            />
            <kbd
              class="hidden shrink-0 rounded border border-subtle px-1.5 py-0.5 text-xs text-muted sm:block"
            >
              ESC
            </kbd>
          </div>

          <!-- Search results -->
          <SearchDropdown
            :results="results"
            :selected-index="selectedIndex"
            :loading="isLoading"
            :has-query="!!query.trim()"
            class="border-0 rounded-none shadow-none"
            @select="
              (r) => {
                navigateToResult(r);
                closeModal();
              }
            "
            @hover="(i) => (selectedIndex = i)"
          />

          <!-- Theme section — shown when no search query -->
          <div v-if="!query.trim()" class="border-t border-subtle px-2 py-2">
            <p class="px-2 pb-1 pt-0.5 text-[10px] font-semibold uppercase tracking-widest text-muted">
              Theme
            </p>
            <button
              v-for="item in [
                { mode: 'light', label: 'Light mode' },
                { mode: 'dark', label: 'Dark mode' },
                { mode: 'system', label: 'System default' },
              ]"
              :key="item.mode"
              type="button"
              class="flex w-full items-center gap-3 rounded-md px-2 py-1.5 text-sm text-primary transition-colors hover:bg-raised"
              @click="colorMode.preference = item.mode; closeModal()"
            >
              <!-- Sun icon -->
              <svg v-if="item.mode === 'light'" xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-muted" aria-hidden="true">
                <circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/>
              </svg>
              <!-- Moon icon -->
              <svg v-else-if="item.mode === 'dark'" xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-muted" aria-hidden="true">
                <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
              </svg>
              <!-- Monitor icon -->
              <svg v-else xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-muted" aria-hidden="true">
                <rect x="2" y="3" width="20" height="14" rx="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/>
              </svg>
              <span>{{ item.label }}</span>
              <span v-if="colorMode.preference === item.mode" class="ml-auto text-[var(--color-brand)]">✓</span>
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
```

- [ ] **Step 2: Verify in dev server**

Open the command palette (⌘K) with an empty search box. Confirm a "Theme" section appears below the results area with Light / Dark / System options. Clicking one should change the theme immediately, close the palette, and show a checkmark on the active item when reopened.

- [ ] **Step 3: Commit**

```bash
git add web/app/components/CommandPalette.vue
git commit -m "feat: migrate CommandPalette to tokens and add Theme section"
```

---

## Task 7: Fix PyDocstring.vue — zinc class and scoped styles

**Files:**
- Modify: `web/app/components/docs/PyDocstring.vue`

- [ ] **Step 1: Update the zinc class on the wrapper div**

Line 36: `text-zinc-400` → `text-muted`

```html
<div
  v-if="formattedHtml"
  class="docstring-content text-sm leading-relaxed text-muted"
  v-html="formattedHtml"
/>
```

- [ ] **Step 2: Replace hardcoded hex values in scoped styles**

Replace the `<style scoped>` block:

```css
<style scoped>
.docstring-content :deep(p) {
  margin-bottom: 0.75rem;
}
.docstring-content :deep(code) {
  background: var(--color-raised);
  padding: 1px 4px;
  border-radius: 3px;
  color: var(--py-name);
  font-size: 0.85em;
  font-family: var(--font-mono);
}
.docstring-content :deep(pre) {
  background: var(--color-base);
  border: 1px solid var(--color-subtle);
  border-radius: 6px;
  padding: 12px 16px;
  margin-bottom: 0.75rem;
  overflow-x: auto;
}
.docstring-content :deep(pre code) {
  background: transparent;
  padding: 0;
  border-radius: 0;
  color: inherit;
  font-size: 11px;
}
</style>
```

- [ ] **Step 3: Commit**

```bash
git add web/app/components/docs/PyDocstring.vue
git commit -m "fix: replace hardcoded hex colors in PyDocstring with CSS tokens"
```

---

## Task 8: Migrate small components — layouts, PySignature, KbdHint, PackageBadges

**Files:**
- Modify: `web/app/layouts/default.vue`
- Modify: `web/app/components/docs/PySignature.vue`
- Modify: `web/app/components/KbdHint.vue`
- Modify: `web/app/components/PackageBadges.vue`

- [ ] **Step 1: layouts/default.vue line 7**

```html
<!-- before -->
<div class="min-h-screen bg-zinc-950 text-zinc-50">
<!-- after -->
<div class="min-h-screen bg-base text-primary">
```

- [ ] **Step 2: PySignature.vue line 139**

```html
<!-- before -->
class="rounded-md border border-zinc-800 bg-zinc-900 px-4 py-2.5 font-mono text-[11px] leading-relaxed overflow-x-auto"
<!-- after -->
class="rounded-md border border-subtle bg-surface px-4 py-2.5 font-mono text-[11px] leading-relaxed overflow-x-auto"
```

- [ ] **Step 3: KbdHint.vue line 9**

```html
<!-- before -->
class="pointer-events-none ml-1 hidden rounded bg-zinc-800 px-1 py-0.5 font-mono text-[10px] text-zinc-600 group-hover:text-zinc-400 sm:inline-block"
<!-- after -->
class="pointer-events-none ml-1 hidden rounded bg-raised px-1 py-0.5 font-mono text-[10px] text-zinc-400 dark:text-zinc-600 group-hover:text-muted sm:inline-block"
```

- [ ] **Step 4: PackageBadges.vue line 47**

```html
<!-- before -->
class="inline-flex items-center rounded bg-zinc-500/10 px-2 py-0.5 font-mono text-xs text-zinc-400 ring-1 ring-zinc-500/20"
<!-- after -->
class="inline-flex items-center rounded bg-raised/50 px-2 py-0.5 font-mono text-xs text-muted ring-1 ring-subtle/50"
```

- [ ] **Step 5: Commit**

```bash
git add web/app/layouts/default.vue web/app/components/docs/PySignature.vue web/app/components/KbdHint.vue web/app/components/PackageBadges.vue
git commit -m "refactor: migrate small components to semantic tokens"
```

---

## Task 9: Migrate SearchDropdown, SearchResults, TrendingPackages, ReadmeOutline

**Files:**
- Modify: `web/app/components/SearchDropdown.vue`
- Modify: `web/app/components/SearchResults.vue`
- Modify: `web/app/components/TrendingPackages.vue`
- Modify: `web/app/components/ReadmeOutline.vue`

- [ ] **Step 1: SearchDropdown.vue**

Apply these replacements:

- Line 22: `border-zinc-800 bg-zinc-900/95` → `border-subtle bg-surface/95`
- Line 25: `text-zinc-500` → `text-muted`
- Line 37: `bg-zinc-800` → `bg-raised` / `hover:bg-zinc-800/50` → `hover:bg-raised/50`
- Line 43: `text-zinc-50` → `text-primary` / `text-zinc-200` → `text-zinc-800 dark:text-zinc-200`
- Line 50: `text-zinc-400` → `text-muted` / `text-zinc-500` → `text-muted`
- Line 58: `border-zinc-800 text-zinc-600` → `border-subtle text-zinc-400 dark:text-zinc-600`
- Line 66: `text-zinc-500` → `text-muted`

- [ ] **Step 2: SearchResults.vue**

- Line 21: `border-zinc-800 bg-zinc-900/50 hover:border-zinc-700 hover:bg-zinc-900` → `border-subtle bg-surface hover:border-zinc-300 dark:hover:border-zinc-700 hover:bg-surface`
- Line 24: `text-zinc-50` → `text-primary`
- Line 27: `text-zinc-400` → `text-muted`
- Line 29: `text-zinc-500` → `text-muted`

- [ ] **Step 3: TrendingPackages.vue**

- Line 21: `border-zinc-800 bg-zinc-900/50 hover:border-zinc-700 hover:bg-zinc-900` → `border-subtle bg-surface hover:border-zinc-300 dark:hover:border-zinc-700 hover:bg-surface`
- Line 25: `text-zinc-50` → `text-primary`
- Line 29: `text-zinc-500` → `text-muted`
- Line 31: `text-zinc-400` → `text-muted`

- [ ] **Step 4: ReadmeOutline.vue**

- Line 88: `text-zinc-500` → `text-muted`
- Line 89: `border-zinc-800` → `border-subtle`
- Lines 95–96: `text-zinc-200` → `text-zinc-800 dark:text-zinc-200` / `text-zinc-500 hover:text-zinc-300` → `text-muted hover:text-zinc-700 dark:hover:text-zinc-300`

- [ ] **Step 5: Commit**

```bash
git add web/app/components/SearchDropdown.vue web/app/components/SearchResults.vue web/app/components/TrendingPackages.vue web/app/components/ReadmeOutline.vue
git commit -m "refactor: migrate search and card components to semantic tokens"
```

---

## Task 10: Migrate InstallCommand.vue and PackageDependencies.vue

**Files:**
- Modify: `web/app/components/InstallCommand.vue`
- Modify: `web/app/components/PackageDependencies.vue`

- [ ] **Step 1: InstallCommand.vue**

- Line 17: `border-zinc-800 bg-zinc-900` → `border-subtle bg-surface`
- Line 19: `border-zinc-800` → `border-subtle`
- Line 27: `text-zinc-500 hover:text-zinc-300` → `text-muted hover:text-zinc-700 dark:hover:text-zinc-300`
- Line 37: `text-zinc-300` → `text-zinc-700 dark:text-zinc-300`
- Line 38: `text-zinc-500` → `text-muted`
- Line 41: `text-zinc-500` → `text-muted`

- [ ] **Step 2: PackageDependencies.vue**

- Line 22: `text-zinc-500` → `text-muted`
- Line 29: `border-zinc-800 bg-zinc-900/50` → `border-subtle bg-surface`
- Line 33: `text-zinc-50` → `text-primary`
- Line 37: `text-zinc-500` → `text-muted`
- Line 40: `text-zinc-500` → `text-muted`
- Line 45: `text-zinc-500` → `text-muted`
- Line 54: `bg-zinc-800 text-zinc-400 hover:text-zinc-200` → `bg-raised text-muted hover:text-zinc-800 dark:hover:text-zinc-200`
- Line 65: `border-zinc-800 bg-zinc-900/50` → `border-subtle bg-surface`
- Line 69: `text-zinc-50` → `text-primary`
- Line 73: `text-zinc-500` → `text-muted`

- [ ] **Step 3: Commit**

```bash
git add web/app/components/InstallCommand.vue web/app/components/PackageDependencies.vue
git commit -m "refactor: migrate InstallCommand and PackageDependencies to tokens"
```

---

## Task 11: Migrate PackageOverview.vue

**Files:**
- Modify: `web/app/components/PackageOverview.vue`

This file also uses `.prose.prose-invert` — remove `.prose-invert` from that class.

- [ ] **Step 1: Apply all replacements**

- Line 33: `border-zinc-800 bg-zinc-900/50` → `border-subtle bg-surface`
- Line 35: `text-zinc-500` → `text-muted`
- Line 43: `prose prose-invert prose-sm` → `prose prose-sm` (remove `prose-invert`)
- Line 48: `text-zinc-300` → `text-zinc-700 dark:text-zinc-300`
- Line 66: `border-zinc-800 bg-zinc-900/50` → `border-subtle bg-surface`
- Line 67: `text-zinc-500` → `text-muted`
- Lines 70,74,78,82: `text-zinc-500` → `text-muted`
- Lines 71,75,79,83: `text-zinc-300` → `text-zinc-700 dark:text-zinc-300`
- Line 91: `border-zinc-800 bg-zinc-900/50` → `border-subtle bg-surface`
- Line 93: `text-zinc-500` → `text-muted`
- Line 100: `text-zinc-400 hover:text-zinc-200` → `text-muted hover:text-zinc-800 dark:hover:text-zinc-200`

- [ ] **Step 2: Commit**

```bash
git add web/app/components/PackageOverview.vue
git commit -m "refactor: migrate PackageOverview to tokens, remove prose-invert"
```

---

## Task 12: Migrate PackageStats.vue and PackageVersions.vue

**Files:**
- Modify: `web/app/components/PackageStats.vue`
- Modify: `web/app/components/PackageVersions.vue`

- [ ] **Step 1: PackageStats.vue**

- Line 79: `text-zinc-500 hover:text-zinc-300` → `text-muted hover:text-zinc-700 dark:hover:text-zinc-300`
- Line 85: `text-zinc-600` → `text-zinc-400 dark:text-zinc-600`
- Line 92 (spinner): `border-zinc-700 border-t-zinc-300` → `border-subtle border-t-primary`
- Lines 98, 103, 110, 119, 137, 146: `text-zinc-500` → `text-muted` / `text-zinc-400` → `text-muted`

- [ ] **Step 2: PackageVersions.vue**

- Line 79 (spinner): `border-zinc-700 border-t-zinc-300` → `border-subtle border-t-primary`
- Line 85: `bg-zinc-800 border-zinc-700 text-zinc-400` → `bg-raised border-subtle text-muted`
- Line 126: `border-zinc-800 text-zinc-500` → `border-subtle text-muted`
- Line 136: `border-zinc-800` → `border-subtle`
- Line 137: `hover:bg-zinc-800/30` → `hover:bg-raised/30`
- Line 144: `text-zinc-500` → `text-muted`
- Line 149: `text-zinc-200` → `text-zinc-800 dark:text-zinc-200`
- Line 156: `text-zinc-400` → `text-muted`
- Line 160: `text-zinc-500` → `text-muted`
- Line 169: `bg-zinc-900/50 border-zinc-800` → `bg-surface border-subtle`
- Line 171: `text-zinc-200` → `text-zinc-800 dark:text-zinc-200`
- Line 172: `text-zinc-500` → `text-muted`
- Line 173: Remove `prose-invert` from `prose prose-invert prose-sm max-w-none mb-3`

- [ ] **Step 3: Commit**

```bash
git add web/app/components/PackageStats.vue web/app/components/PackageVersions.vue
git commit -m "refactor: migrate PackageStats and PackageVersions to tokens"
```

---

## Task 13: Migrate useDocstringFormat.ts

**Files:**
- Modify: `web/app/composables/useDocstringFormat.ts`

The two zinc classes appear inside template literal strings that generate HTML.

- [ ] **Step 1: Update both occurrences**

Line 132:
```ts
// before
return `<p class="text-[11px] italic text-zinc-600">${label}</p>`;
// after
return `<p class="text-[11px] italic text-zinc-400 dark:text-zinc-600">${label}</p>`;
```

Line 136:
```ts
// before
return `<p class="text-[11px] italic text-zinc-600">${content}</p>`;
// after
return `<p class="text-[11px] italic text-zinc-400 dark:text-zinc-600">${content}</p>`;
```

- [ ] **Step 2: Commit**

```bash
git add web/app/composables/useDocstringFormat.ts
git commit -m "refactor: update inline HTML classes in useDocstringFormat for light mode"
```

---

## Task 14: Migrate pages/index.vue and pages/search.vue

**Files:**
- Modify: `web/app/pages/index.vue`
- Modify: `web/app/pages/search.vue`

- [ ] **Step 1: index.vue**

- Line 52: `text-zinc-400` → `text-muted`
- Line 64: `border-zinc-800 bg-zinc-900 text-zinc-50 placeholder-zinc-500` → `border-subtle bg-surface text-primary placeholder-muted`
- Line 69: `bg-zinc-800 text-zinc-500` → `bg-raised text-muted`
- Line 91: `text-zinc-500` → `text-muted`
- Line 100: `border-zinc-800 bg-zinc-800/50` → `border-subtle bg-raised/50`
- Line 105: `text-zinc-500` → `text-muted`
- Line 111: `text-zinc-500` → `text-muted`

- [ ] **Step 2: search.vue**

- Line 40: `border-zinc-800 bg-zinc-900 text-zinc-50 placeholder-zinc-500 focus:border-zinc-600 focus:ring-zinc-600` → `border-subtle bg-surface text-primary placeholder-muted focus:border-[var(--color-brand-light)] focus:ring-1 focus:ring-[var(--color-brand-border)]`
- Line 46 (spinner): `border-zinc-700 border-t-zinc-300` → `border-subtle border-t-primary`
- Line 51: `text-zinc-500` → `text-muted`
- Line 54: `text-zinc-300` → `text-zinc-700 dark:text-zinc-300`
- Line 61: `text-zinc-300` → `text-zinc-700 dark:text-zinc-300`
- Line 62: `text-zinc-500` → `text-muted`
- Line 63: `text-zinc-400` → `text-muted`

- [ ] **Step 3: Commit**

```bash
git add web/app/pages/index.vue web/app/pages/search.vue
git commit -m "refactor: migrate index and search pages to semantic tokens"
```

---

## Task 15: Migrate pages/packages/[name].vue

**Files:**
- Modify: `web/app/pages/packages/[name].vue`

- [ ] **Step 1: Apply replacements**

- Line 109 (spinner): `border-zinc-700 border-t-zinc-300` → `border-subtle border-t-primary`
- Line 114: `text-zinc-300` → `text-zinc-700 dark:text-zinc-300`
- Line 115: `text-zinc-500` → `text-muted`
- Line 123: `text-zinc-50` → `text-primary`
- Line 124: `bg-zinc-800 text-zinc-400` → `bg-raised text-muted`
- Line 128: `text-zinc-400` → `text-muted`
- Line 140: `border-zinc-800` → `border-subtle`
- Line 147: `bg-zinc-800 text-zinc-50` → `bg-raised text-primary` / `text-zinc-500 hover:text-zinc-300` → `text-muted hover:text-zinc-700 dark:hover:text-zinc-300`
- Line 159: `text-zinc-500 hover:text-zinc-300` → `text-muted hover:text-zinc-700 dark:hover:text-zinc-300`

- [ ] **Step 2: Commit**

```bash
git add "web/app/pages/packages/[name].vue"
git commit -m "refactor: migrate package page to semantic tokens"
```

---

## Task 16: Migrate pages/packages/[name]/[version].vue

**Files:**
- Modify: `web/app/pages/packages/[name]/[version].vue`

- [ ] **Step 1: Apply replacements**

- Line 62: `text-zinc-500 hover:text-zinc-300` → `text-muted hover:text-zinc-700 dark:hover:text-zinc-300`
- Line 70: `text-zinc-300` → `text-zinc-700 dark:text-zinc-300`
- Line 71: `text-zinc-500` → `text-muted`
- Line 79: `text-zinc-50` → `text-primary`
- Line 80: `bg-zinc-800 text-zinc-400` → `bg-raised text-muted`
- Line 84: `text-zinc-400` → `text-muted`
- Lines 94, 98, 104, 110: `border-zinc-800 bg-zinc-900` → `border-subtle bg-surface`
- Lines 95, 99, 105, 111: `text-zinc-500` → `text-muted`
- Lines 96, 100, 106, 112: `text-zinc-200` → `text-zinc-800 dark:text-zinc-200`
- Line 119: `border-zinc-800 bg-zinc-900` → `border-subtle bg-surface`
- Line 121: `border-zinc-800` → `border-subtle`
- Line 122: `text-zinc-300` → `text-zinc-700 dark:text-zinc-300`
- Line 130: `text-zinc-300` → `text-zinc-700 dark:text-zinc-300`
- Line 139: `border-zinc-800 bg-zinc-900/50` → `border-subtle bg-surface`
- Line 140: `text-zinc-100` → `text-zinc-900 dark:text-zinc-100`
- Line 141: `text-zinc-500` → `text-muted`
- Line 148: `hover:text-zinc-300` → `hover:text-zinc-700 dark:hover:text-zinc-300`
- Line 152: Remove `prose-invert` from class — `prose prose-invert prose-sm max-w-none` → `prose prose-sm max-w-none`
- Line 161 (spinner): `border-zinc-700 border-t-zinc-300` → `border-subtle border-t-primary`

- [ ] **Step 2: Commit**

```bash
git add "web/app/pages/packages/[name]/[version].vue"
git commit -m "refactor: migrate version page to semantic tokens"
```

---

## Task 17: Migrate pages/packages/[name]/docs.vue

**Files:**
- Modify: `web/app/pages/packages/[name]/docs.vue`

This file has 30 zinc class occurrences — the most of any file.

- [ ] **Step 1: Apply replacements**

- Line 66 (spinner): `border-zinc-700 border-t-zinc-300` → `border-subtle border-t-primary`
- Line 71: `text-zinc-300` → `text-zinc-700 dark:text-zinc-300`
- Line 81: `text-zinc-50 hover:text-zinc-300` → `text-primary hover:text-zinc-700 dark:hover:text-zinc-300`
- Line 84: `bg-zinc-800 text-zinc-400` → `bg-raised text-muted`
- Line 88: `text-zinc-400` → `text-muted`
- Line 92: `border-zinc-800` → `border-subtle`
- Line 97: `text-zinc-500 hover:text-zinc-300` → `text-muted hover:text-zinc-700 dark:hover:text-zinc-300`
- Line 101: `bg-zinc-800 text-zinc-50` → `bg-raised text-primary`
- Line 108 (spinner): `border-zinc-700 border-t-zinc-300` → `border-subtle border-t-primary`
- Line 113: `text-zinc-400` → `text-muted`
- Line 114: `text-zinc-500` → `text-muted`
- Line 131: `border-zinc-800 bg-zinc-950` → `border-subtle bg-base`
- Line 133: `text-zinc-600` → `text-zinc-400 dark:text-zinc-600`
- Line 140: `text-zinc-500` → `text-muted`
- Line 142: `text-zinc-700` → `text-zinc-400 dark:text-zinc-700`
- Line 151: `text-zinc-500 hover:text-zinc-300` → `text-muted hover:text-zinc-700 dark:hover:text-zinc-300`
- Line 162: `text-zinc-500` → `text-muted`
- Line 164: `text-zinc-700` → `text-zinc-400 dark:text-zinc-700`
- Line 173: `text-zinc-500 hover:text-zinc-300` → `text-muted hover:text-zinc-700 dark:hover:text-zinc-300`
- Line 184: `text-zinc-500` → `text-muted`
- Line 186: `text-zinc-700` → `text-zinc-400 dark:text-zinc-700`
- Line 195: `text-zinc-500 hover:text-zinc-300` → `text-muted hover:text-zinc-700 dark:hover:text-zinc-300`
- Line 213: `text-zinc-50` → `text-primary`
- Line 233: `text-zinc-600` → `text-zinc-400 dark:text-zinc-600`
- Line 236: `border-zinc-800` → `border-subtle`
- Line 244: `text-zinc-600` → `text-zinc-400 dark:text-zinc-600`
- Line 247: `text-zinc-500` → `text-muted`
- Line 256: `text-zinc-600` → `text-zinc-400 dark:text-zinc-600`
- Line 262: `text-zinc-500` → `text-muted`
- Line 267: `border-zinc-900` → `border-base`

- [ ] **Step 2: Commit**

```bash
git add "web/app/pages/packages/[name]/docs.vue"
git commit -m "refactor: migrate docs page to semantic tokens"
```

---

## Task 18: End-to-end verification

- [ ] **Step 1: Run the full test suite**

```bash
cd web && npm run test
```

Expected: all tests pass (ThemeToggle tests included).

- [ ] **Step 2: Build the app**

```bash
cd web && npm run build
```

Expected: clean build with no type errors.

- [ ] **Step 3: Start the preview server and walk through the checklist**

```bash
cd web && npm run preview
```

Walk through each item:

- [ ] Light mode renders correctly on first SSR load — open a private window to test without cookie
- [ ] Dark mode renders correctly on first SSR load — set cookie `pypx-color-mode=dark` in DevTools then hard refresh
- [ ] System mode follows OS preference — clear the cookie, change OS appearance setting
- [ ] Preference persists across page navigations
- [ ] Preference persists after browser close/reopen
- [ ] Theme toggle cycles correctly: Sun (light) → Moon (dark) → Monitor (system) → Sun
- [ ] Command palette (⌘K) shows Theme section with correct active checkmark
- [ ] Shiki code blocks use github-light theme in light mode, one-dark-pro in dark mode — check `/packages/requests/docs`
- [ ] Python signatures (PySignature) show accessible token colors in both modes
- [ ] Prose markdown renders without `.prose-invert` in both modes — check package description and changelog
- [ ] Brand green (header logo, links) is emerald-700 in light, mint in dark
- [ ] No hydration mismatch warnings in browser console
- [ ] No zinc-* classes remain in rendered HTML (spot-check DevTools on the package page)

- [ ] **Step 4: Final commit**

```bash
git add -A
git commit -m "feat: complete dark/light mode implementation"
```
