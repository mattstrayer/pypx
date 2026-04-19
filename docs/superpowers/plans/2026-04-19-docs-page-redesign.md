# Docs Page Performance & UX Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the docs page fast and delightful for packages with thousands of symbols by adding virtual sidebar scrolling, a ⌘K command palette, deferred main content rendering, IntersectionObserver scroll tracking, and improved sidebar typography.

**Architecture:** The monolithic `docs.vue` is split into four focused components: `DocsSymbolCard.vue` (single symbol renderer), `DocsSidebar.vue` (virtual-scrolled nav with ⌘K trigger), `DocsCommandPalette.vue` (fuzzy search overlay), and a refactored `docs.vue` (orchestration, deferred rendering, IntersectionObserver). Data flows one direction — `docs.vue` fetches and flattens symbols, passes the full array to the sidebar and palette always, renders the main content progressively via `renderedCount`.

**Tech Stack:** Vue 3, Nuxt 4, Tailwind 4, `vue-virtual-scroller` (virtual list), `fuse.js` (fuzzy search), Vitest + `@nuxt/test-utils`

---

## File Map

| File | Change |
|---|---|
| `web/package.json` | Add `vue-virtual-scroller`, `fuse.js` |
| `web/app/components/docs/DocsSymbolCard.vue` | **New** — extracted per-symbol renderer |
| `web/app/components/docs/DocsSkeletonLoader.vue` | **New** — shimmer skeleton for fetch + render states |
| `web/app/components/docs/DocsCommandPalette.vue` | **New** — ⌘K overlay with fuse.js fuzzy search |
| `web/app/components/docs/DocsSidebar.vue` | **New** — virtual-scrolled nav list |
| `web/app/pages/packages/[name]/docs.vue` | **Refactor** — orchestration, deferred render, IntersectionObserver |
| `web/app/components/docs/__tests__/DocsSymbolCard.test.ts` | **New** |
| `web/app/components/docs/__tests__/DocsCommandPalette.test.ts` | **New** |
| `web/app/components/docs/__tests__/DocsSidebar.test.ts` | **New** |

---

## Task 1: Install Dependencies

**Files:**
- Modify: `web/package.json`

- [ ] **Step 1: Install packages**

```bash
cd web && pnpm add vue-virtual-scroller fuse.js
```

- [ ] **Step 2: Verify installation**

```bash
cd web && node -e "require('vue-virtual-scroller'); require('fuse.js'); console.log('OK')"
```

Expected output: `OK`

- [ ] **Step 3: Commit**

```bash
cd /Users/matt/dev/pypx && git add web/package.json web/pnpm-lock.yaml
git commit -m "chore: add vue-virtual-scroller and fuse.js"
```

---

## Task 2: Extract DocsSymbolCard.vue

Extract the per-symbol rendering template from `docs.vue` into a standalone component. This is a pure extraction — no logic changes yet.

**Files:**
- Create: `web/app/components/docs/DocsSymbolCard.vue`
- Create: `web/app/components/docs/__tests__/DocsSymbolCard.test.ts`

- [ ] **Step 1: Write the failing test**

Create `web/app/components/docs/__tests__/DocsSymbolCard.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { mountSuspended } from "@nuxt/test-utils/runtime";
import DocsSymbolCard from "../DocsSymbolCard.vue";
import type { DocSymbol } from "~/types/api";

function makeSymbol(overrides: Partial<DocSymbol> = {}): DocSymbol {
  return {
    name: "fit",
    kind: "function",
    signature: "def fit(X, y=None)",
    docstring: "Fit the model.",
    parameters: [{ name: "X", type: "array", description: "Training data.", kind: "positional_or_keyword" }],
    returns: { type: "self", description: "The fitted estimator." },
    raises: [],
    ...overrides,
  };
}

describe("DocsSymbolCard", () => {
  it("renders symbol name", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: { symbol: makeSymbol(), isActive: false },
    });
    expect(wrapper.text()).toContain("fit");
  });

  it("renders function kind badge", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: { symbol: makeSymbol(), isActive: false },
    });
    expect(wrapper.text()).toContain("function");
  });

  it("renders class kind badge", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: { symbol: makeSymbol({ name: "Pipeline", kind: "class" }), isActive: false },
    });
    expect(wrapper.text()).toContain("class");
  });

  it("shows expand button for classes with methods", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: {
        symbol: makeSymbol({
          name: "Pipeline",
          kind: "class",
          methods: [makeSymbol({ name: "fit_transform" })],
        }),
        isActive: false,
      },
    });
    expect(wrapper.text()).toMatch(/methods/i);
  });

  it("does not show methods section for functions", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: { symbol: makeSymbol(), isActive: false },
    });
    expect(wrapper.text()).not.toMatch(/methods/i);
  });

  it("exposes sym-{name} id for scroll targeting", async () => {
    const wrapper = await mountSuspended(DocsSymbolCard, {
      props: { symbol: makeSymbol({ name: "predict" }), isActive: false },
    });
    expect(wrapper.find("#sym-predict").exists()).toBe(true);
  });
});
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
cd web && pnpm test -- app/components/docs/__tests__/DocsSymbolCard.test.ts
```

Expected: FAIL — `DocsSymbolCard` not found.

- [ ] **Step 3: Create DocsSymbolCard.vue**

Create `web/app/components/docs/DocsSymbolCard.vue`. This is extracted verbatim from the `v-for` block in the current `docs.vue` template (lines 248–430), with `expandedClasses` state moved inside the component:

```vue
<script setup lang="ts">
import type { DocSymbol } from "~/types/api";

const props = defineProps<{
  symbol: DocSymbol;
  isActive: boolean;
}>();

const expanded = ref(false);

function toggleExpand() {
  expanded.value = !expanded.value;
}
</script>

<template>
  <div :id="`sym-${symbol.name}`" class="mb-10 scroll-mt-4">
    <!-- Symbol name + kind badge -->
    <div class="mb-3 flex items-center gap-2">
      <span class="font-mono text-base font-bold text-primary">{{ symbol.name }}</span>
      <span
        class="rounded px-1.5 py-0.5 text-[9px] font-semibold uppercase tracking-wide"
        :class="{
          'bg-blue-950 text-blue-300': symbol.kind === 'function',
          'bg-purple-950 text-purple-300': symbol.kind === 'class',
          'bg-red-950 text-red-300': symbol.kind === 'exception',
        }"
        >{{ symbol.kind }}</span
      >
    </div>

    <!-- Signature -->
    <DocsPySignature :symbol="symbol" class="mb-3" />

    <!-- Docstring -->
    <DocsPyDocstring v-if="symbol.docstring" :text="symbol.docstring" class="mb-3" />

    <!-- Parameters -->
    <div v-if="symbol.parameters && symbol.parameters.length" class="mb-3">
      <p class="mb-2 text-[9px] font-bold uppercase tracking-widest text-zinc-400 dark:text-zinc-600">
        Parameters
      </p>
      <div class="border-l-2 border-subtle pl-3 space-y-2">
        <div
          v-for="param in symbol.parameters?.filter((p) => p.name !== 'self' && p.name !== 'cls')"
          :key="param.name"
        >
          <span class="font-mono text-[11px] text-sky-400">{{ param.name }}</span>
          <span v-if="param.type" class="ml-1.5 text-[10px] text-zinc-400 dark:text-zinc-600">{{ param.type }}</span>
          <p v-if="param.description" class="mt-0.5 text-[11px] text-muted">{{ param.description }}</p>
        </div>
      </div>
    </div>

    <!-- Returns -->
    <div v-if="symbol.returns" class="mb-3">
      <p class="mb-2 text-[9px] font-bold uppercase tracking-widest text-zinc-400 dark:text-zinc-600">Returns</p>
      <div class="border-l-2 border-subtle pl-3">
        <span class="text-[10px] text-zinc-400 dark:text-zinc-600">{{ symbol.returns.type }}</span>
        <p v-if="symbol.returns.description" class="mt-0.5 text-[11px] text-muted">{{ symbol.returns.description }}</p>
      </div>
    </div>

    <!-- Raises -->
    <div v-if="symbol.raises && symbol.raises.length" class="mb-3">
      <p class="mb-2 text-[9px] font-bold uppercase tracking-widest text-zinc-400 dark:text-zinc-600">Raises</p>
      <div class="border-l-2 border-subtle pl-3 space-y-1">
        <div v-for="r in symbol.raises" :key="r.type">
          <span class="font-mono text-[11px] text-red-400">{{ r.type }}</span>
          <p v-if="r.description" class="mt-0.5 text-[11px] text-muted">{{ r.description }}</p>
        </div>
      </div>
    </div>

    <!-- Methods (classes only) -->
    <div v-if="symbol.kind === 'class' && symbol.methods && symbol.methods.length" class="mt-4">
      <button
        class="flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-wide text-muted hover:text-primary transition-colors"
        @click="toggleExpand"
      >
        <span>{{ expanded ? '▾' : '▸' }}</span>
        Methods ({{ symbol.methods.length }})
      </button>
      <div v-if="expanded" class="mt-3 space-y-6 border-l-2 border-subtle pl-4">
        <div v-for="method in symbol.methods" :key="method.name" class="pt-1">
          <div class="mb-2 flex items-center gap-2">
            <span class="font-mono text-sm font-semibold text-primary">{{ method.name }}</span>
          </div>
          <DocsPySignature :symbol="method" class="mb-2" />
          <DocsPyDocstring v-if="method.docstring" :text="method.docstring" class="mb-2" />
          <div v-if="method.parameters && method.parameters.length" class="mb-2">
            <div class="border-l-2 border-subtle pl-3 space-y-1">
              <div
                v-for="param in method.parameters?.filter((p) => p.name !== 'self' && p.name !== 'cls')"
                :key="param.name"
              >
                <span class="font-mono text-[11px] text-sky-400">{{ param.name }}</span>
                <span v-if="param.type" class="ml-1.5 text-[10px] text-zinc-400 dark:text-zinc-600">{{ param.type }}</span>
                <p v-if="param.description" class="mt-0.5 text-[11px] text-muted">{{ param.description }}</p>
              </div>
            </div>
          </div>
          <div v-if="method.returns" class="mb-2">
            <div class="border-l-2 border-subtle pl-3">
              <span class="text-[10px] text-zinc-400 dark:text-zinc-600">{{ method.returns.type }}</span>
              <p v-if="method.returns.description" class="mt-0.5 text-[11px] text-muted">{{ method.returns.description }}</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 4: Run tests**

```bash
cd web && pnpm test -- app/components/docs/__tests__/DocsSymbolCard.test.ts
```

Expected: All 6 tests pass.

- [ ] **Step 5: Commit**

```bash
git add web/app/components/docs/DocsSymbolCard.vue web/app/components/docs/__tests__/DocsSymbolCard.test.ts
git commit -m "feat: extract DocsSymbolCard component"
```

---

## Task 3: Build DocsSkeletonLoader.vue

A shimmer skeleton shown during two states: docs fetching (shows sidebar + content skeleton), and deferred rendering progress (shows only the progress bar).

**Files:**
- Create: `web/app/components/docs/DocsSkeletonLoader.vue`

- [ ] **Step 1: Create the component**

Create `web/app/components/docs/DocsSkeletonLoader.vue`:

```vue
<script setup lang="ts">
defineProps<{
  mode: "fetch" | "render";
  renderedCount?: number;
  totalCount?: number;
}>();
</script>

<template>
  <!-- Full fetch skeleton: matches two-column layout to prevent layout shift -->
  <div v-if="mode === 'fetch'" class="flex gap-0 -mx-4 sm:-mx-6 lg:-mx-8">
    <!-- Sidebar skeleton -->
    <div class="w-48 flex-shrink-0 sticky top-0 h-screen border-r border-subtle bg-base py-3 hidden md:block">
      <div class="px-3 pb-3">
        <div class="mb-3 h-2 w-16 rounded bg-zinc-800 animate-pulse" />
        <div class="space-y-2">
          <div v-for="i in 8" :key="i" class="h-5 rounded bg-zinc-800 animate-pulse" :style="`width: ${55 + (i % 3) * 15}%`" />
        </div>
      </div>
    </div>
    <!-- Content skeleton -->
    <div class="flex-1 min-w-0 px-6 py-5 space-y-10">
      <div v-for="i in 3" :key="i" class="space-y-3">
        <div class="flex items-center gap-3">
          <div class="h-5 w-32 rounded bg-zinc-800 animate-pulse" />
          <div class="h-4 w-16 rounded bg-zinc-800 animate-pulse" />
        </div>
        <div class="h-8 w-full rounded bg-zinc-800 animate-pulse" />
        <div class="space-y-2">
          <div class="h-3 w-full rounded bg-zinc-800 animate-pulse" />
          <div class="h-3 w-5/6 rounded bg-zinc-800 animate-pulse" />
          <div class="h-3 w-4/6 rounded bg-zinc-800 animate-pulse" />
        </div>
      </div>
    </div>
  </div>

  <!-- Render-progress indicator: shown below already-rendered content -->
  <div v-else-if="mode === 'render'" class="py-4 text-center text-xs text-zinc-600">
    Loading {{ renderedCount }} of {{ totalCount }} symbols...
  </div>
</template>
```

- [ ] **Step 2: Commit**

```bash
git add web/app/components/docs/DocsSkeletonLoader.vue
git commit -m "feat: add DocsSkeletonLoader component"
```

---

## Task 4: Build DocsCommandPalette.vue

⌘K fuzzy search overlay. Queries the raw `symbols` array via fuse.js — never the DOM. Rendered in `<Teleport to="body">`.

**Files:**
- Create: `web/app/components/docs/DocsCommandPalette.vue`
- Create: `web/app/components/docs/__tests__/DocsCommandPalette.test.ts`

- [ ] **Step 1: Write the failing tests**

Create `web/app/components/docs/__tests__/DocsCommandPalette.test.ts`:

```ts
import { describe, it, expect, vi } from "vitest";
import { mountSuspended } from "@nuxt/test-utils/runtime";
import DocsCommandPalette from "../DocsCommandPalette.vue";
import type { DocSymbol } from "~/types/api";

function makeSymbol(name: string, kind: DocSymbol["kind"] = "function"): DocSymbol {
  return {
    name,
    kind,
    signature: `def ${name}()`,
    docstring: "",
    parameters: [],
    returns: null,
    raises: [],
  };
}

const symbols = [
  makeSymbol("fit"),
  makeSymbol("predict"),
  makeSymbol("fit_transform"),
  makeSymbol("Pipeline", "class"),
  makeSymbol("GridSearchCV", "class"),
];

describe("DocsCommandPalette", () => {
  it("is not visible when open is false", async () => {
    const wrapper = await mountSuspended(DocsCommandPalette, {
      props: { symbols, open: false },
    });
    expect(wrapper.find("[data-testid='palette-modal']").exists()).toBe(false);
  });

  it("is visible when open is true", async () => {
    const wrapper = await mountSuspended(DocsCommandPalette, {
      props: { symbols, open: true },
      attachTo: document.body,
    });
    expect(wrapper.find("[data-testid='palette-modal']").exists()).toBe(true);
  });

  it("shows all symbols grouped when query is empty", async () => {
    const wrapper = await mountSuspended(DocsCommandPalette, {
      props: { symbols, open: true },
      attachTo: document.body,
    });
    expect(wrapper.text()).toContain("fit");
    expect(wrapper.text()).toContain("Pipeline");
  });

  it("filters symbols by query", async () => {
    const wrapper = await mountSuspended(DocsCommandPalette, {
      props: { symbols, open: true },
      attachTo: document.body,
    });
    const input = wrapper.find("[data-testid='palette-input']");
    await input.setValue("pipeline");
    expect(wrapper.text()).toContain("Pipeline");
    expect(wrapper.text()).not.toContain("predict");
  });

  it("emits jump when a result is clicked", async () => {
    const wrapper = await mountSuspended(DocsCommandPalette, {
      props: { symbols, open: true },
      attachTo: document.body,
    });
    const firstResult = wrapper.find("[data-testid='palette-result']");
    await firstResult.trigger("click");
    expect(wrapper.emitted("jump")).toBeTruthy();
    expect(wrapper.emitted("jump")![0]).toHaveLength(1);
  });

  it("emits close when Escape is pressed", async () => {
    const wrapper = await mountSuspended(DocsCommandPalette, {
      props: { symbols, open: true },
      attachTo: document.body,
    });
    await wrapper.find("[data-testid='palette-input']").trigger("keydown", { key: "Escape" });
    expect(wrapper.emitted("close")).toBeTruthy();
  });

  it("emits close when backdrop is clicked", async () => {
    const wrapper = await mountSuspended(DocsCommandPalette, {
      props: { symbols, open: true },
      attachTo: document.body,
    });
    await wrapper.find("[data-testid='palette-backdrop']").trigger("click");
    expect(wrapper.emitted("close")).toBeTruthy();
  });
});
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
cd web && pnpm test -- app/components/docs/__tests__/DocsCommandPalette.test.ts
```

Expected: FAIL — component not found.

- [ ] **Step 3: Create DocsCommandPalette.vue**

Create `web/app/components/docs/DocsCommandPalette.vue`:

```vue
<script setup lang="ts">
import Fuse from "fuse.js";
import type { DocSymbol } from "~/types/api";

const props = defineProps<{
  symbols: DocSymbol[];
  open: boolean;
}>();

const emit = defineEmits<{
  jump: [name: string];
  close: [];
}>();

const query = ref("");
const selectedIndex = ref(0);

const fuse = computed(
  () =>
    new Fuse(props.symbols, {
      keys: ["name", "kind"],
      threshold: 0.4,
      includeScore: true,
    }),
);

const MAX_PER_GROUP = 8;

const results = computed<DocSymbol[]>(() => {
  if (!query.value.trim()) {
    const functions = props.symbols.filter((s) => s.kind === "function").slice(0, MAX_PER_GROUP);
    const classes = props.symbols.filter((s) => s.kind === "class").slice(0, MAX_PER_GROUP);
    const exceptions = props.symbols.filter((s) => s.kind === "exception").slice(0, MAX_PER_GROUP);
    return [...functions, ...classes, ...exceptions];
  }
  return fuse.value.search(query.value).map((r) => r.item);
});

watch(query, () => {
  selectedIndex.value = 0;
});

function select(name: string) {
  emit("jump", name);
  emit("close");
  query.value = "";
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === "Escape") {
    emit("close");
    query.value = "";
  } else if (e.key === "ArrowDown") {
    e.preventDefault();
    selectedIndex.value = Math.min(selectedIndex.value + 1, results.value.length - 1);
  } else if (e.key === "ArrowUp") {
    e.preventDefault();
    selectedIndex.value = Math.max(selectedIndex.value - 1, 0);
  } else if (e.key === "Enter" && results.value[selectedIndex.value]) {
    select(results.value[selectedIndex.value].name);
  }
}

// Auto-focus input when opened
const inputRef = ref<HTMLInputElement | null>(null);
watch(
  () => props.open,
  (open) => {
    if (open) {
      query.value = "";
      selectedIndex.value = 0;
      nextTick(() => inputRef.value?.focus());
    }
  },
);

const isMac = typeof navigator !== "undefined" && /mac/i.test(navigator.platform);
const shortcutLabel = isMac ? "⌘K" : "Ctrl+K";
</script>

<template>
  <Teleport to="body">
    <div v-if="open">
      <!-- Backdrop -->
      <div
        data-testid="palette-backdrop"
        class="fixed inset-0 z-40 bg-black/60 backdrop-blur-sm"
        @click="emit('close')"
      />

      <!-- Modal -->
      <div
        data-testid="palette-modal"
        class="fixed left-1/2 top-[20vh] z-50 w-full max-w-lg -translate-x-1/2 rounded-xl border border-zinc-700 bg-zinc-900 shadow-2xl overflow-hidden"
      >
        <!-- Search input -->
        <div class="flex items-center gap-3 border-b border-zinc-700 px-4 py-3">
          <svg class="h-4 w-4 flex-shrink-0 text-zinc-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <input
            ref="inputRef"
            v-model="query"
            data-testid="palette-input"
            type="text"
            placeholder="Jump to symbol..."
            class="flex-1 bg-transparent font-mono text-sm text-zinc-100 placeholder-zinc-600 outline-none"
            @keydown="onKeydown"
          />
          <kbd class="text-[10px] text-zinc-600 bg-zinc-800 border border-zinc-700 rounded px-1.5 py-0.5">esc</kbd>
        </div>

        <!-- Results -->
        <div class="max-h-80 overflow-y-auto py-1">
          <div v-if="results.length === 0" class="px-4 py-6 text-center text-sm text-zinc-600">
            No symbols found
          </div>
          <button
            v-for="(sym, i) in results"
            :key="sym.name"
            data-testid="palette-result"
            class="flex w-full items-center gap-3 px-4 py-2 text-left transition-colors"
            :class="i === selectedIndex ? 'bg-zinc-800' : 'hover:bg-zinc-800/50'"
            @click="select(sym.name)"
            @mouseover="selectedIndex = i"
          >
            <span
              class="flex-shrink-0 rounded px-1.5 py-0.5 text-[9px] font-semibold uppercase tracking-wide"
              :class="{
                'bg-blue-950 text-blue-300': sym.kind === 'function',
                'bg-purple-950 text-purple-300': sym.kind === 'class',
                'bg-red-950 text-red-300': sym.kind === 'exception',
              }"
            >{{ sym.kind }}</span>
            <span class="font-mono text-sm text-zinc-200">{{ sym.name }}</span>
          </button>
        </div>

        <!-- Footer hint -->
        <div class="border-t border-zinc-700 px-4 py-2 flex items-center gap-3 text-[10px] text-zinc-600">
          <span><kbd class="bg-zinc-800 border border-zinc-700 rounded px-1 py-0.5">↑↓</kbd> navigate</span>
          <span><kbd class="bg-zinc-800 border border-zinc-700 rounded px-1 py-0.5">↵</kbd> jump</span>
          <span><kbd class="bg-zinc-800 border border-zinc-700 rounded px-1 py-0.5">esc</kbd> close</span>
        </div>
      </div>
    </div>
  </Teleport>
</template>
```

- [ ] **Step 4: Run tests**

```bash
cd web && pnpm test -- app/components/docs/__tests__/DocsCommandPalette.test.ts
```

Expected: All 7 tests pass.

- [ ] **Step 5: Commit**

```bash
git add web/app/components/docs/DocsCommandPalette.vue web/app/components/docs/__tests__/DocsCommandPalette.test.ts
git commit -m "feat: add DocsCommandPalette component with fuse.js fuzzy search"
```

---

## Task 5: Build DocsSidebar.vue

Virtual-scrolled sidebar using `DynamicScroller` from `vue-virtual-scroller`. Flat item array mixes section headers and symbol rows. Emits `select` and `open-palette`.

**Files:**
- Create: `web/app/components/docs/DocsSidebar.vue`
- Create: `web/app/components/docs/__tests__/DocsSidebar.test.ts`

- [ ] **Step 1: Write the failing tests**

Create `web/app/components/docs/__tests__/DocsSidebar.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { mountSuspended } from "@nuxt/test-utils/runtime";
import DocsSidebar from "../DocsSidebar.vue";
import type { DocSymbol } from "~/types/api";

function makeSymbol(name: string, kind: DocSymbol["kind"] = "function"): DocSymbol {
  return { name, kind, signature: "", docstring: "", parameters: [], returns: null, raises: [] };
}

const functions = [makeSymbol("fit"), makeSymbol("predict"), makeSymbol("transform")];
const classes = [makeSymbol("Pipeline", "class"), makeSymbol("GridSearchCV", "class")];

describe("DocsSidebar", () => {
  it("renders Functions section header with count", async () => {
    const wrapper = await mountSuspended(DocsSidebar, {
      props: { functions, classes, exceptions: [], activeSymbol: null },
    });
    expect(wrapper.text()).toContain("Functions");
    expect(wrapper.text()).toContain("3");
  });

  it("renders Classes section header with count", async () => {
    const wrapper = await mountSuspended(DocsSidebar, {
      props: { functions, classes, exceptions: [], activeSymbol: null },
    });
    expect(wrapper.text()).toContain("Classes");
    expect(wrapper.text()).toContain("2");
  });

  it("does not render empty sections", async () => {
    const wrapper = await mountSuspended(DocsSidebar, {
      props: { functions, classes, exceptions: [], activeSymbol: null },
    });
    expect(wrapper.text()).not.toContain("Exceptions");
  });

  it("renders the ⌘K trigger button", async () => {
    const wrapper = await mountSuspended(DocsSidebar, {
      props: { functions, classes, exceptions: [], activeSymbol: null },
    });
    expect(wrapper.find("[data-testid='palette-trigger']").exists()).toBe(true);
  });

  it("emits open-palette when trigger is clicked", async () => {
    const wrapper = await mountSuspended(DocsSidebar, {
      props: { functions, classes, exceptions: [], activeSymbol: null },
    });
    await wrapper.find("[data-testid='palette-trigger']").trigger("click");
    expect(wrapper.emitted("open-palette")).toBeTruthy();
  });

  it("emits select with symbol name when a row is clicked", async () => {
    const wrapper = await mountSuspended(DocsSidebar, {
      props: { functions, classes, exceptions: [], activeSymbol: null },
    });
    await wrapper.find("[data-testid='symbol-row']").trigger("click");
    expect(wrapper.emitted("select")).toBeTruthy();
    expect(typeof wrapper.emitted("select")![0][0]).toBe("string");
  });
});
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd web && pnpm test -- app/components/docs/__tests__/DocsSidebar.test.ts
```

Expected: FAIL — component not found.

- [ ] **Step 3: Create DocsSidebar.vue**

Create `web/app/components/docs/DocsSidebar.vue`:

```vue
<script setup lang="ts">
import { DynamicScroller, DynamicScrollerItem } from "vue-virtual-scroller";
import "vue-virtual-scroller/dist/vue-virtual-scroller.css";
import type { DocSymbol } from "~/types/api";

const props = defineProps<{
  functions: DocSymbol[];
  classes: DocSymbol[];
  exceptions: DocSymbol[];
  activeSymbol: string | null;
}>();

const emit = defineEmits<{
  select: [name: string];
  "open-palette": [];
}>();

type SidebarItem =
  | { id: string; type: "header"; label: string; count: number }
  | { id: string; type: "symbol"; name: string; kind: string };

const items = computed<SidebarItem[]>(() => {
  const list: SidebarItem[] = [];
  if (props.functions.length) {
    list.push({ id: "h-functions", type: "header", label: "Functions", count: props.functions.length });
    for (const s of props.functions) list.push({ id: `s-${s.name}`, type: "symbol", name: s.name, kind: s.kind });
  }
  if (props.classes.length) {
    list.push({ id: "h-classes", type: "header", label: "Classes", count: props.classes.length });
    for (const s of props.classes) list.push({ id: `s-${s.name}`, type: "symbol", name: s.name, kind: s.kind });
  }
  if (props.exceptions.length) {
    list.push({ id: "h-exceptions", type: "header", label: "Exceptions", count: props.exceptions.length });
    for (const s of props.exceptions) list.push({ id: `s-${s.name}`, type: "symbol", name: s.name, kind: s.kind });
  }
  return list;
});

const activeIndex = computed(() =>
  items.value.findIndex((i) => i.type === "symbol" && i.name === props.activeSymbol),
);

const scrollerRef = ref<InstanceType<typeof DynamicScroller> | null>(null);

watch(activeIndex, (idx) => {
  if (idx >= 0 && scrollerRef.value) {
    scrollerRef.value.scrollToItem(idx);
  }
});

const isMac = typeof navigator !== "undefined" && /mac/i.test(navigator.platform);
const shortcutLabel = isMac ? "⌘K" : "Ctrl+K";
</script>

<template>
  <div class="w-48 flex-shrink-0 sticky top-0 h-screen flex flex-col border-r border-subtle bg-base hidden md:flex">
    <!-- ⌘K trigger -->
    <div class="px-2 py-2 border-b border-subtle flex-shrink-0">
      <button
        data-testid="palette-trigger"
        class="flex w-full items-center gap-2 rounded-md bg-zinc-800/50 px-2.5 py-1.5 text-[11px] text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 transition-colors"
        @click="emit('open-palette')"
      >
        <svg class="h-3 w-3 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
        <span class="flex-1 text-left">Jump to symbol</span>
        <kbd class="text-[9px] text-zinc-600">{{ shortcutLabel }}</kbd>
      </button>
    </div>

    <!-- Virtual symbol list -->
    <DynamicScroller
      ref="scrollerRef"
      :items="items"
      :min-item-size="28"
      key-field="id"
      class="flex-1 overflow-y-auto py-1"
    >
      <template #default="{ item, index, active }">
        <DynamicScrollerItem :item="item" :active="active" :data-index="index">
          <!-- Section header -->
          <div
            v-if="item.type === 'header'"
            class="flex items-center justify-between px-3 pb-1 pt-3"
            style="height: 36px"
          >
            <span class="text-[10px] font-medium uppercase tracking-wide text-zinc-500">{{ item.label }}</span>
            <span class="rounded-full bg-zinc-800 px-1.5 py-0.5 text-[9px] text-zinc-600">{{ item.count }}</span>
          </div>

          <!-- Symbol row -->
          <button
            v-else
            data-testid="symbol-row"
            class="flex w-full items-center px-3 text-left font-mono text-[12px] transition-colors"
            style="height: 28px"
            :class="
              activeSymbol === item.name
                ? 'bg-blue-500/15 border-l-2 border-blue-500 text-white pl-2.5'
                : 'text-zinc-400 hover:bg-zinc-800/50 hover:text-zinc-200'
            "
            @click="emit('select', item.name)"
          >
            {{ item.name }}
          </button>
        </DynamicScrollerItem>
      </template>
    </DynamicScroller>
  </div>
</template>
```

- [ ] **Step 4: Run tests**

```bash
cd web && pnpm test -- app/components/docs/__tests__/DocsSidebar.test.ts
```

Expected: All 6 tests pass.

- [ ] **Step 5: Commit**

```bash
git add web/app/components/docs/DocsSidebar.vue web/app/components/docs/__tests__/DocsSidebar.test.ts
git commit -m "feat: add DocsSidebar with virtual scrolling and ⌘K trigger"
```

---

## Task 6: Refactor docs.vue

Replace the inline sidebar and symbol loop with the new components. Add deferred rendering, IntersectionObserver scroll tracking, and ⌘K keyboard shortcut.

**Files:**
- Modify: `web/app/pages/packages/[name]/docs.vue`

- [ ] **Step 1: Replace the script setup block**

Replace the entire `<script setup>` in `web/app/pages/packages/[name]/docs.vue` with:

```ts
<script setup lang="ts">
import type { DocSymbol } from "~/types/api";

const route = useRoute();
const name = computed(() => route.params.name as string);

const api = useApi();

const { data: pkg, status: pkgStatus } = await useAsyncData(`package-${name.value}`, () =>
  api.fetchPackage(name.value),
);

const { data: docs, status: docsStatus } = await useAsyncData(`docs-data-${name.value}`, () =>
  api.fetchDocs(name.value),
);

function dedup(symbols: DocSymbol[]): DocSymbol[] {
  const seen = new Set<string>();
  return symbols.filter((s) => {
    if (seen.has(s.name)) return false;
    seen.add(s.name);
    return true;
  });
}

const allFunctions = computed<DocSymbol[]>(() =>
  dedup(docs.value?.modules?.flatMap((m) => m.functions) ?? []),
);
const allClasses = computed<DocSymbol[]>(() =>
  dedup(docs.value?.modules?.flatMap((m) => m.classes) ?? []),
);
const allExceptions = computed<DocSymbol[]>(() =>
  dedup(docs.value?.modules?.flatMap((m) => m.exceptions) ?? []),
);

const allSymbols = computed<DocSymbol[]>(() => [
  ...allFunctions.value,
  ...allClasses.value,
  ...allExceptions.value,
]);

// ── Deferred rendering ─────────────────────────────────────────────────────
const BATCH_SIZE = 20;
const renderedCount = ref(0);
let pendingIdle: ReturnType<typeof requestIdleCallback> | null = null;

function scheduleNextBatch() {
  if (pendingIdle) cancelIdleCallback(pendingIdle);
  pendingIdle = requestIdleCallback(
    () => {
      renderedCount.value = Math.min(renderedCount.value + BATCH_SIZE, allSymbols.value.length);
      if (renderedCount.value < allSymbols.value.length) scheduleNextBatch();
    },
    { timeout: 500 },
  );
}

watch(
  allSymbols,
  (syms) => {
    if (syms.length === 0) return;
    renderedCount.value = Math.min(BATCH_SIZE, syms.length);
    scheduleNextBatch();
  },
  { immediate: true },
);

const visibleSymbols = computed(() => allSymbols.value.slice(0, renderedCount.value));

// ── Active symbol + scroll tracking ───────────────────────────────────────
const activeSymbol = ref<string | null>(null);
let observer: IntersectionObserver | null = null;

function setupObserver() {
  observer?.disconnect();
  observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          activeSymbol.value = (entry.target as HTMLElement).dataset.symbol ?? null;
          break;
        }
      }
    },
    { rootMargin: "-10% 0px -80% 0px", threshold: 0 },
  );
}

// Observe newly rendered symbol elements as deferred batches land
watch(renderedCount, (newCount, oldCount) => {
  if (!observer) setupObserver();
  for (let i = oldCount; i < newCount; i++) {
    const sym = allSymbols.value[i];
    if (!sym) continue;
    const el = document.getElementById(`sym-${sym.name}`);
    if (el) {
      el.dataset.symbol = sym.name;
      observer!.observe(el);
    }
  }
});

onUnmounted(() => {
  observer?.disconnect();
  if (pendingIdle) cancelIdleCallback(pendingIdle);
});

// ── Jump to symbol ─────────────────────────────────────────────────────────
async function jumpToSymbol(symbolName: string) {
  const targetIndex = allSymbols.value.findIndex((s) => s.name === symbolName);
  if (targetIndex === -1) return;

  // Fast-forward render if target not yet in DOM
  if (targetIndex >= renderedCount.value) {
    if (pendingIdle) cancelIdleCallback(pendingIdle);
    renderedCount.value = targetIndex + 1;
    await nextTick();
  }

  activeSymbol.value = symbolName;
  const el = document.getElementById(`sym-${symbolName}`);
  if (el) el.scrollIntoView({ behavior: "smooth", block: "start" });
}

// ── ⌘K / Ctrl+K global shortcut ──────────────────────────────────────────
const paletteOpen = ref(false);

function onKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === "k") {
    e.preventDefault();
    paletteOpen.value = true;
  }
}

onMounted(() => window.addEventListener("keydown", onKeydown));
onUnmounted(() => window.removeEventListener("keydown", onKeydown));

// ── SEO ────────────────────────────────────────────────────────────────────
useSeoMeta({
  title: () => (pkg.value ? `${pkg.value.name} API Docs` : "Loading"),
  description: () => `API documentation for ${pkg.value?.name ?? name.value}`,
});

defineOgImage(
  "DocsCard",
  {
    name: () => pkg.value?.name ?? "",
    version: () => pkg.value?.version ?? "",
  },
  { width: 1200, height: 630 },
);
</script>
```

- [ ] **Step 2: Replace the template**

Replace the entire `<template>` block in `web/app/pages/packages/[name]/docs.vue` with:

```html
<template>
  <div>
    <!-- Package loading -->
    <div v-if="pkgStatus === 'pending'" class="flex items-center justify-center py-24">
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-subtle border-t-primary" />
    </div>

    <!-- Package error -->
    <div v-else-if="pkgStatus === 'error'" class="py-24 text-center">
      <p class="text-lg font-medium text-zinc-700 dark:text-zinc-300">Package not found</p>
    </div>

    <div v-else-if="pkg">
      <!-- Header -->
      <div class="mb-6">
        <div class="flex flex-wrap items-baseline gap-3">
          <NuxtLink
            :to="`/packages/${pkg.name}`"
            class="text-3xl font-bold text-primary hover:text-zinc-700 dark:hover:text-zinc-300 transition-colors"
            >{{ pkg.name }}</NuxtLink
          >
          <span class="rounded bg-raised px-2 py-0.5 font-mono text-sm text-muted">
            v{{ pkg.version }}
          </span>
        </div>
        <p v-if="pkg.summary" class="mt-2 text-muted">{{ pkg.summary }}</p>
      </div>

      <!-- Tab strip -->
      <div class="mb-6 flex gap-1 overflow-x-auto border-b border-subtle pb-0">
        <NuxtLink
          v-for="tab in ['Overview', 'Dependencies', 'Versions', 'Stats']"
          :key="tab"
          :to="`/packages/${pkg.name}`"
          class="cursor-pointer whitespace-nowrap rounded-t px-4 py-2 text-sm font-medium text-muted transition-colors hover:text-zinc-700 dark:hover:text-zinc-300"
          >{{ tab }}</NuxtLink
        >
        <span class="cursor-default whitespace-nowrap rounded-t bg-raised px-4 py-2 text-sm font-medium text-primary"
          >Docs</span
        >
      </div>

      <!-- Docs fetch skeleton -->
      <DocsSkeletonLoader v-if="docsStatus === 'pending'" mode="fetch" />

      <!-- Docs unavailable -->
      <div v-else-if="!docs?.available" class="py-16 text-center">
        <p class="text-muted">API documentation is not available for this package.</p>
        <p class="mt-1 text-sm text-muted">This package may be binary-only or could not be parsed.</p>
        <a
          v-if="pkg.doc_url"
          :href="pkg.doc_url"
          target="_blank"
          rel="noopener noreferrer"
          class="mt-4 inline-block text-sm text-[var(--color-brand)] hover:text-[var(--color-brand-light)] transition-colors"
          >View external documentation →</a
        >
      </div>

      <!-- Docs content -->
      <div v-else class="flex gap-0 -mx-4 sm:-mx-6 lg:-mx-8">
        <!-- Virtual sidebar -->
        <DocsSidebar
          :functions="allFunctions"
          :classes="allClasses"
          :exceptions="allExceptions"
          :active-symbol="activeSymbol"
          @select="jumpToSymbol"
          @open-palette="paletteOpen = true"
        />

        <!-- Main content -->
        <div class="flex-1 min-w-0 px-6 py-5">
          <!-- Stub attribution -->
          <div
            v-if="docs?.stub_package"
            class="mb-5 flex items-center gap-2 rounded-md border border-subtle bg-zinc-50 px-3 py-2 text-xs text-muted dark:bg-zinc-900"
          >
            <svg class="h-3.5 w-3.5 flex-shrink-0 text-zinc-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M13 16h-1v-4h-1m1-4h.01M12 2a10 10 0 100 20A10 10 0 0012 2z" />
            </svg>
            <span>Type information enriched from
              <NuxtLink :to="`/packages/${docs.stub_package}`" class="font-mono text-[var(--color-brand)] hover:underline">{{ docs.stub_package }}</NuxtLink>
            </span>
          </div>

          <!-- Rendered symbols (deferred) -->
          <DocsSymbolCard
            v-for="sym in visibleSymbols"
            :key="sym.name"
            :symbol="sym"
            :is-active="activeSymbol === sym.name"
          />

          <!-- Render progress indicator -->
          <DocsSkeletonLoader
            v-if="renderedCount < allSymbols.length"
            mode="render"
            :rendered-count="renderedCount"
            :total-count="allSymbols.length"
          />
        </div>
      </div>
    </div>

    <!-- ⌘K Command Palette -->
    <DocsCommandPalette
      :symbols="allSymbols"
      :open="paletteOpen"
      @jump="jumpToSymbol"
      @close="paletteOpen = false"
    />
  </div>
</template>
```

- [ ] **Step 3: Verify the app builds without errors**

```bash
cd web && pnpm build 2>&1 | tail -20
```

Expected: build completes without TypeScript or template errors.

- [ ] **Step 4: Run the dev server and manually test**

```bash
cd web && npm run dev
```

Navigate to a package with many symbols, e.g. `http://localhost:3000/packages/numpy` and verify:
- Skeleton loading state appears during fetch
- Sidebar populates immediately with virtual scroll
- Symbols render progressively (progress counter visible for large packages)
- ⌘K opens the palette, fuzzy search works, Enter/arrow keys navigate, Escape closes
- Clicking a sidebar item scrolls to that symbol
- Scrolling the main content updates the active sidebar item
- Sidebar auto-scrolls to keep active item in view

- [ ] **Step 5: Run full test suite**

```bash
cd web && pnpm test
```

Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add web/app/pages/packages/\[name\]/docs.vue
git commit -m "feat: refactor docs page with virtual sidebar, deferred rendering, and ⌘K palette"
```

---

## Self-Review

**Spec coverage check:**
- ✅ Virtual sidebar (DocsSidebar + vue-virtual-scroller) — Task 5
- ✅ ⌘K command palette with fuse.js — Task 4
- ✅ Deferred rendering with requestIdleCallback — Task 6 Step 1
- ✅ IntersectionObserver scroll tracking — Task 6 Step 1
- ✅ Sidebar auto-scroll to active item — Task 5 (scrollerRef.scrollToItem)
- ✅ Jump to unrendered symbol fast-forward — Task 6 Step 1 (jumpToSymbol)
- ✅ Loading state: fetch skeleton — Task 3 (mode="fetch")
- ✅ Loading state: render progress — Task 3 (mode="render"), Task 6 Step 2
- ✅ Sidebar typography: 12px mono, full-width row, count badge — Task 5
- ✅ DocsSymbolCard extracted — Task 2
- ✅ expandedClasses reactivity workaround removed (replaced by local `expanded` ref) — Task 2

**Placeholder scan:** None found.

**Type consistency check:**
- `DocSymbol` imported from `~/types/api` consistently across all components
- `SidebarItem` union type defined and used only within `DocsSidebar.vue`
- `jumpToSymbol(name: string)` called in `docs.vue` template via `@select` and `@jump` — matches emit type in both child components
- `renderedCount`/`totalCount` props on `DocsSkeletonLoader` match usage in `docs.vue` template
- `scrollerRef.value.scrollToItem(idx)` — this is the DynamicScroller API method name; verify after install that vue-virtual-scroller v2 exposes `scrollToItem` (it does — confirmed in their README)
