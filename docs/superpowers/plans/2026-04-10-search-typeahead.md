# Search Typeahead Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a typeahead dropdown to both the header search input and the Cmd+K command palette, powered by a shared composable and dropdown component.

**Architecture:** Extract search logic into `useSearchTypeahead()` composable. Create a `SearchDropdown.vue` component for rendering results. Wire both into `AppHeader.vue` (inline dropdown) and `CommandPalette.vue` (replacing its custom search logic).

**Tech Stack:** Vue 3 Composition API, VueUse (`useDebounceFn`, `onClickOutside`), existing `useApi().searchPackages()`, Tailwind CSS.

**Spec:** `docs/superpowers/specs/2026-04-10-search-typeahead-design.md`

---

### Task 1: Create `useSearchTypeahead` composable

**Files:**
- Create: `web/app/composables/useSearchTypeahead.ts`

- [ ] **Step 1: Create the composable**

```ts
import { useDebounceFn } from "@vueuse/core";
import type { SearchResult } from "~/types/api";

export function useSearchTypeahead() {
  const query = ref("");
  const results = ref<SearchResult[]>([]);
  const selectedIndex = ref(-1);
  const isOpen = ref(false);
  const isLoading = ref(false);
  const router = useRouter();
  const { searchPackages } = useApi();

  const performSearch = useDebounceFn(async (q: string) => {
    if (!q.trim()) {
      results.value = [];
      isLoading.value = false;
      return;
    }
    try {
      results.value = await searchPackages(q, 5);
      selectedIndex.value = -1;
    } catch {
      results.value = [];
    } finally {
      isLoading.value = false;
    }
  }, 150);

  watch(query, (val) => {
    if (val.trim()) {
      isLoading.value = true;
      isOpen.value = true;
    } else {
      isOpen.value = false;
    }
    performSearch(val);
  });

  function onKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      e.preventDefault();
      close();
      return;
    }
    if (!isOpen.value || results.value.length === 0) {
      if (e.key === "Enter") {
        e.preventDefault();
        navigateToSearch();
      }
      return;
    }
    if (e.key === "ArrowDown") {
      e.preventDefault();
      selectedIndex.value = Math.min(selectedIndex.value + 1, results.value.length - 1);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      selectedIndex.value = Math.max(selectedIndex.value - 1, -1);
    } else if (e.key === "Enter") {
      e.preventDefault();
      if (selectedIndex.value >= 0) {
        navigateToResult(results.value[selectedIndex.value]);
      } else {
        navigateToSearch();
      }
    }
  }

  function navigateToResult(result: SearchResult) {
    router.push(`/packages/${result.name}`);
    close();
  }

  function navigateToSearch() {
    if (query.value.trim()) {
      router.push({ path: "/search", query: { q: query.value.trim() } });
      close();
    }
  }

  function open() {
    isOpen.value = true;
    selectedIndex.value = -1;
  }

  function close() {
    isOpen.value = false;
    selectedIndex.value = -1;
  }

  function reset() {
    query.value = "";
    results.value = [];
    selectedIndex.value = -1;
    isOpen.value = false;
    isLoading.value = false;
  }

  return {
    query,
    results,
    selectedIndex,
    isOpen,
    isLoading,
    onKeydown,
    navigateToResult,
    open,
    close,
    reset,
  };
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/matt/dev/pypx/web && npx nuxi typecheck`
Expected: No type errors in the new file.

- [ ] **Step 3: Commit**

```bash
git add web/app/composables/useSearchTypeahead.ts
git commit -m "feat(web): add useSearchTypeahead composable"
```

---

### Task 2: Create `SearchDropdown.vue` component

**Files:**
- Create: `web/app/components/SearchDropdown.vue`

- [ ] **Step 1: Create the dropdown component**

```vue
<script setup lang="ts">
import type { SearchResult } from "~/types/api";

defineProps<{
  results: SearchResult[];
  selectedIndex: number;
  loading: boolean;
  hasQuery: boolean;
}>();

const emit = defineEmits<{
  select: [result: SearchResult];
  hover: [index: number];
}>();
</script>

<template>
  <div
    v-if="hasQuery"
    class="overflow-hidden rounded-lg border border-zinc-800 bg-zinc-900/95 shadow-xl backdrop-blur"
  >
    <!-- Loading -->
    <div v-if="loading && results.length === 0" class="px-3 py-4 text-center text-sm text-zinc-500">
      Searching...
    </div>

    <!-- Results -->
    <template v-else-if="results.length > 0">
      <button
        v-for="(result, index) in results"
        :key="result.name"
        class="flex w-full items-center gap-2 px-3 py-2 text-left transition-colors"
        :class="index === selectedIndex ? 'bg-zinc-800' : 'hover:bg-zinc-800/50'"
        @click="emit('select', result)"
        @mousemove="emit('hover', index)"
      >
        <span
          class="shrink-0 text-sm font-medium"
          :class="index === selectedIndex ? 'text-zinc-50' : 'text-zinc-200'"
        >
          {{ result.name }}
        </span>
        <span
          v-if="result.summary"
          class="truncate text-sm"
          :class="index === selectedIndex ? 'text-zinc-400' : 'text-zinc-500'"
        >
          {{ result.summary }}
        </span>
      </button>

      <!-- Footer -->
      <div
        class="flex items-center justify-between border-t border-zinc-800 px-3 py-1.5 text-xs text-zinc-600"
      >
        <span>↑↓ navigate · ↵ select · esc close</span>
        <span>{{ results.length }} results</span>
      </div>
    </template>

    <!-- No results -->
    <div v-else class="px-3 py-4 text-center text-sm text-zinc-500">No packages found</div>
  </div>
</template>
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/matt/dev/pypx/web && npx nuxi typecheck`
Expected: No type errors.

- [ ] **Step 3: Commit**

```bash
git add web/app/components/SearchDropdown.vue
git commit -m "feat(web): add SearchDropdown component"
```

---

### Task 3: Wire typeahead into `AppHeader.vue`

**Files:**
- Modify: `web/app/components/AppHeader.vue`

- [ ] **Step 1: Replace AppHeader with composable + dropdown**

Replace the entire contents of `web/app/components/AppHeader.vue` with:

```vue
<script setup lang="ts">
const { query, results, selectedIndex, isOpen, isLoading, onKeydown, navigateToResult, close } =
  useSearchTypeahead();
const colorMode = useColorMode();
const searchWrapper = ref<HTMLElement | null>(null);

function toggleColorMode() {
  colorMode.preference = colorMode.value === "dark" ? "light" : "dark";
}

// Close dropdown when clicking outside the search wrapper
function onClickOutside(e: MouseEvent) {
  if (searchWrapper.value && !searchWrapper.value.contains(e.target as Node)) {
    close();
  }
}

onMounted(() => {
  document.addEventListener("mousedown", onClickOutside);
});

onUnmounted(() => {
  document.removeEventListener("mousedown", onClickOutside);
});
</script>

<template>
  <header class="sticky top-0 z-50 border-b border-zinc-800 bg-zinc-950/80 backdrop-blur-sm">
    <div class="mx-auto flex h-14 max-w-6xl items-center gap-6 px-4">
      <NuxtLink to="/" class="flex items-center gap-2 text-zinc-50 hover:text-white">
        <span class="text-lg font-bold tracking-tight">pypx</span>
      </NuxtLink>

      <div ref="searchWrapper" class="relative flex-1 max-w-md">
        <form @submit.prevent>
          <div class="relative">
            <svg
              class="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-zinc-500"
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
              class="w-full rounded-md border border-zinc-800 bg-zinc-900 py-1.5 pl-8 pr-12 text-sm text-zinc-50 placeholder-zinc-500 outline-none focus:border-zinc-600 focus:ring-1 focus:ring-zinc-600"
              @keydown="onKeydown"
              @focus="query.trim() && (isOpen = true)"
            />
            <kbd
              class="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 rounded bg-zinc-800 px-1.5 py-0.5 font-mono text-[10px] text-zinc-500"
            >
              ⌘K
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

      <nav class="flex items-center gap-4">
        <button
          type="button"
          :aria-label="colorMode.value === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'"
          class="rounded-md p-1.5 text-zinc-400 hover:bg-zinc-800 hover:text-zinc-50"
          @click="toggleColorMode"
        >
          <svg
            v-if="colorMode.value === 'dark'"
            class="size-5"
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z" />
          </svg>
          <svg
            v-else
            class="size-5"
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <circle cx="12" cy="12" r="4" />
            <path
              d="M12 2v2m0 16v2M4.93 4.93l1.41 1.41m11.32 11.32 1.41 1.41M2 12h2m16 0h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41"
            />
          </svg>
        </button>
      </nav>
    </div>
  </header>
</template>
```

Key changes from original:
- Uses `useSearchTypeahead()` instead of local `searchQuery` + `onSearch`
- Input binds `v-model="query"` and `@keydown="onKeydown"` from composable
- `@focus` reopens dropdown if query is non-empty
- Adds `<SearchDropdown>` positioned absolutely below input
- Adds `⌘K` kbd badge in the input (right side)
- Input padding changed from `pr-2.5` to `pr-12` to make room for badge
- Click-outside listener closes dropdown
- No more form submit handler — Enter behavior is in the composable

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/matt/dev/pypx/web && npx nuxi typecheck`
Expected: No type errors.

- [ ] **Step 3: Smoke test in browser**

Run: `cd /Users/matt/dev/pypx && docker compose up -d`
Visit `http://localhost:3000`. Type in the header search input and verify:
- Dropdown appears after typing
- Arrow keys navigate results
- Enter on a result navigates to `/packages/{name}`
- Enter with no selection navigates to `/search?q=...`
- Escape closes dropdown
- Clicking outside closes dropdown
- ⌘K badge is visible in the input

- [ ] **Step 4: Commit**

```bash
git add web/app/components/AppHeader.vue
git commit -m "feat(web): wire typeahead into header search input"
```

---

### Task 4: Refactor `CommandPalette.vue` to use shared composable

**Files:**
- Modify: `web/app/components/CommandPalette.vue`

- [ ] **Step 1: Replace CommandPalette with composable + dropdown**

Replace the entire contents of `web/app/components/CommandPalette.vue` with:

```vue
<script setup lang="ts">
const { query, results, selectedIndex, isOpen, isLoading, onKeydown, navigateToResult, reset } =
  useSearchTypeahead();
const inputRef = ref<HTMLInputElement | null>(null);
const isModalOpen = ref(false);

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

function onGlobalKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === "k") {
    e.preventDefault();
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
        <div class="w-full max-w-lg overflow-hidden rounded-xl border border-zinc-700 bg-zinc-900 shadow-2xl">
          <!-- Input area -->
          <div class="flex items-center gap-3 border-b border-zinc-800 px-4 py-3">
            <svg
              class="size-4 shrink-0 text-zinc-400"
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
              class="min-w-0 flex-1 bg-transparent text-sm text-zinc-100 placeholder-zinc-500 outline-none"
              @keydown="onModalKeydown"
            />
            <kbd
              class="hidden shrink-0 rounded border border-zinc-700 px-1.5 py-0.5 text-xs text-zinc-500 sm:block"
            >
              ESC
            </kbd>
          </div>

          <!-- Results via shared dropdown (inline, not absolutely positioned) -->
          <SearchDropdown
            :results="results"
            :selected-index="selectedIndex"
            :loading="isLoading"
            :has-query="!!query.trim()"
            class="border-0 rounded-none shadow-none"
            @select="(r) => { navigateToResult(r); closeModal(); }"
            @hover="(i) => (selectedIndex = i)"
          />
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
```

Key changes from original:
- Removes all inline search logic: `useDebounceFn` import, `performSearch`, `watch(query)`, `onKeydown`, `navigateToResult`, `selectedIndex`, `results`, `isLoading`
- Uses `useSearchTypeahead()` composable for all search state and behavior
- Renames `isOpen` → `isModalOpen` to avoid collision with composable's `isOpen` (which controls dropdown visibility)
- Uses `onModalKeydown` wrapper that intercepts Escape to close the modal, delegates everything else to `onKeydown`
- Renders `<SearchDropdown>` inline in the modal body instead of custom result buttons
- On select: navigates to result AND closes modal
- `openModal()` calls `reset()` to clear previous search state
- The `max-h-80 overflow-y-auto` scroll behavior moves to the dropdown container (already handled by its fixed 5-result limit)

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/matt/dev/pypx/web && npx nuxi typecheck`
Expected: No type errors. The `useDebounceFn` import is now only in the composable.

- [ ] **Step 3: Smoke test Cmd+K in browser**

Visit `http://localhost:3000`. Press Cmd+K and verify:
- Modal opens with empty input
- Typing shows results via shared dropdown
- Arrow keys navigate, Enter selects
- Escape closes modal
- Clicking backdrop closes modal
- Result click navigates to package page

- [ ] **Step 4: Commit**

```bash
git add web/app/components/CommandPalette.vue
git commit -m "refactor(web): use shared typeahead composable in command palette"
```

---

### Task 5: Final verification and cleanup

**Files:**
- Review: all modified files

- [ ] **Step 1: Run full type check**

Run: `cd /Users/matt/dev/pypx/web && npx nuxi typecheck`
Expected: Clean pass, no errors.

- [ ] **Step 2: Run linter**

Run: `cd /Users/matt/dev/pypx/web && npx oxlint .`
Expected: No new errors.

- [ ] **Step 3: End-to-end smoke test**

Test these flows in the browser:

1. **Header typeahead**: Type "flask" → see 5 results → arrow to "flask" → Enter → lands on `/packages/flask`
2. **Header Enter-to-search**: Type "flask" → press Enter immediately (no arrow) → lands on `/search?q=flask`
3. **Header click-outside**: Type "flask" → click somewhere else → dropdown closes
4. **Header focus reopen**: Click back into input (query still "flask") → dropdown reopens
5. **Cmd+K modal**: Press Cmd+K → type "requests" → see results → click one → navigates, modal closes
6. **Cmd+K Escape**: Press Cmd+K → type something → press Escape → modal closes
7. **Empty query**: Clear the input → dropdown hidden (no "No results" shown)
8. **Landing page**: Visit `/` → hero search still works (submit navigates to `/search`)

- [ ] **Step 4: Commit any fixes if needed**

If any issues found in step 3, fix and commit with appropriate message.
