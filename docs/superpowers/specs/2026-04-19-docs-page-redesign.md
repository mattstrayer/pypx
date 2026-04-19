# Docs Page Performance & UX Redesign

**Date:** 2026-04-19  
**Scope:** `web/app/pages/packages/[name]/docs.vue` and related components  
**Goal:** Make the docs page fast, readable, and delightful for packages with thousands of symbols.

---

## Problem Statement

The current docs page renders all symbols into the DOM at once (no virtual scrolling), has an 11px muted sidebar that is hard to read, no search/jump capability beyond clicking sidebar items, and no scroll tracking — the active sidebar item only updates on click. For large packages (pandas, scipy, sqlalchemy), this results in sluggish scroll performance and a frustrating navigation experience.

---

## Design Decisions

| Concern | Decision |
|---|---|
| Target scale | Thousands of symbols (worst case, e.g. pandas, scipy) |
| Sidebar search | ⌘K floating command palette |
| Scroll tracking | IntersectionObserver + sidebar auto-scroll to active item |
| Sidebar typography | 12px monospace, full-width active row highlight, count badges |
| Main content render | Deferred `requestIdleCallback` batches |
| Loading states | Skeleton during fetch, progress counter during deferred render |

---

## Architecture

### Component Split

The current monolithic `docs.vue` is refactored into four focused components:

| Component | Responsibility |
|---|---|
| `docs.vue` | Orchestration, data fetch, loading/error states, scroll tracking |
| `DocsSidebar.vue` | Virtual-scrolled nav list + ⌘K trigger pill |
| `DocsCommandPalette.vue` | Overlay fuzzy search, keyboard navigation |
| `DocsSymbolCard.vue` | Single symbol renderer (extracted from current inline template) |

Data flows one direction: `docs.vue` fetches and flattens symbols → passes full array to sidebar and palette (always), renders main content progressively via `renderedCount`.

### New Dependencies

- `vue-virtual-scroller` — virtual list for sidebar (`RecyclerScroller`)
- `fuse.js` — lightweight fuzzy search (4kb gzipped) for command palette

---

## Loading States

Three distinct states, all matching the loaded layout to prevent layout shift:

### 1. Fetching (0 → data arrives)
- Sidebar: ~8 gray pill skeleton rows with shimmer animation
- Main content: 3 skeleton symbol cards (name bar + body placeholder) with shimmer
- Same two-column layout as loaded state

### 2. Rendering (data arrived → deferred render completing)
- Sidebar: fully populated immediately (virtual scroll, instant)
- Main content: rendered symbols visible at top; bottom edge shows progress counter: `"Loading 40 of 312 symbols..."`
- Not a blocker — content above the counter is fully readable and interactive

### 3. Error (fetch failed / goopy timeout)
- Existing error state, no changes needed

---

## Virtual Sidebar (`DocsSidebar.vue`)

### Item Array Structure

The sidebar uses a single flat array of mixed item types:

```ts
type SidebarItem =
  | { type: 'header'; kind: 'functions' | 'classes' | 'exceptions'; count: number }
  | { type: 'symbol'; name: string; kind: string }
```

Section headers live inside the virtual list — not outside it — so the entire sidebar is one continuous virtualized scroll.

### Row Heights (fixed, required by RecyclerScroller)

- Symbol row: **28px**
- Section header row: **36px**

### Active Row Styling

```
bg-blue-500/15  text-white  border-l-2 border-blue-500
```

Applied via `:class` comparing `item.name === activeSymbol`.

### Sidebar Auto-Scroll

When `activeSymbol` changes (click or IntersectionObserver), call `scroller.scrollToItem(activeIndex)` to keep the active row visible in the sidebar viewport.

### ⌘K Trigger

A pill button pinned above the virtual scroller (not part of the virtual list):

```
[ 🔍  Jump to symbol   ⌘K ]
```

Displays `⌘K` on Mac, `Ctrl+K` on Windows/Linux (detected via `navigator.platform`).

### Typography

- Symbol names: 12px monospace, `text-zinc-300`
- Active symbol: 12px monospace, `text-white`
- Section header labels: 10px sans-serif, uppercase, `text-zinc-500`
- Count badge: right-aligned, `text-zinc-600`, `bg-zinc-800` pill

---

## Deferred Main Content Rendering

### Rendering Strategy

```ts
const allSymbols = computed(() => [
  ...allFunctions.value,
  ...allClasses.value,
  ...allExceptions.value,
])
const renderedCount = ref(0)
const BATCH_SIZE = 20

function scheduleNextBatch() {
  requestIdleCallback(() => {
    renderedCount.value = Math.min(renderedCount.value + BATCH_SIZE, allSymbols.value.length)
    if (renderedCount.value < allSymbols.value.length) scheduleNextBatch()
  }, { timeout: 500 })
}

watch(allSymbols, () => {
  renderedCount.value = BATCH_SIZE
  scheduleNextBatch()
}, { immediate: true })
```

The template renders `allSymbols.value.slice(0, renderedCount)`.

### Progress Indicator

```html
<div v-if="renderedCount < allSymbols.length" class="text-xs text-zinc-600 py-4 text-center">
  Loading {{ renderedCount }} of {{ allSymbols.length }} symbols...
</div>
```

### Jumping to Unrendered Symbols

If ⌘K selects a symbol whose index exceeds `renderedCount`:

1. Cancel any pending `requestIdleCallback` batch
2. Set `renderedCount.value = targetIndex + 1`
3. `await nextTick()`
4. `scrollIntoView` the element normally

This ensures the target is always in the DOM before scrolling.

---

## ⌘K Command Palette (`DocsCommandPalette.vue`)

### Trigger / Dismiss

- Open: `Cmd+K` (Mac) or `Ctrl+K` (Win/Linux) global keydown listener, or clicking the sidebar pill
- Close: `Escape`, clicking the backdrop, or selecting an item

### Search

`fuse.js` indexes `name` and `kind` fields from the raw `allSymbols` array. Initialized once on data arrival. Queried on every keystroke — no debounce needed at this data size.

### UX Behavior

- **Empty query:** All symbols grouped by kind (Functions / Classes / Exceptions), capped at 8 per group
- **With query:** Flat list sorted by fuse.js score, match characters highlighted
- **Keyboard:** Arrow keys navigate, `Enter` selects, `Escape` closes
- **On select:** Close palette → fast-forward `renderedCount` if needed → `scrollIntoView` → update `activeSymbol`

### Rendering

Rendered in `<Teleport to="body">` to escape the sidebar's stacking context and overlay correctly at full viewport level.

### Props / Emits

```ts
// Props
symbols: DocSymbol[]

// Emits
jump: (symbolName: string) => void
```

---

## Scroll Tracking (IntersectionObserver)

### Observer Configuration

```ts
const observer = new IntersectionObserver((entries) => {
  for (const entry of entries) {
    if (entry.isIntersecting) {
      activeSymbol.value = entry.target.dataset.symbol
      break
    }
  }
}, {
  rootMargin: '-10% 0px -80% 0px', // triggers when symbol enters top ~10% of viewport
  threshold: 0,
})
```

The `rootMargin` ensures the active item reflects what the user is reading at the top of the screen, not content barely visible at the bottom.

### Observing Deferred Elements

New symbol elements are observed as they're added to the DOM:

```ts
watch(renderedCount, (newCount, oldCount) => {
  for (let i = oldCount; i < newCount; i++) {
    const el = document.getElementById(`sym-${allSymbols.value[i].name}`)
    if (el) observer.observe(el)
  }
})
```

### Cleanup

```ts
onUnmounted(() => observer.disconnect())
```

---

## Sidebar Typography Specification

Replacing current 11px muted text with:

| Element | Size | Color | Notes |
|---|---|---|---|
| Symbol name | 12px mono | `text-zinc-300` | Full-width clickable row, 28px tall |
| Active symbol | 12px mono | `text-white` | `bg-blue-500/15`, `border-l-2 border-blue-500` |
| Section header | 10px sans | `text-zinc-500` | Uppercase, `font-weight: 500`, 36px tall |
| Count badge | 10px sans | `text-zinc-600` | Right-aligned, `bg-zinc-800` pill |
| Hover state | — | `bg-zinc-800/50` | Applied on all non-active rows |

---

## Files Changed

| File | Change |
|---|---|
| `web/app/pages/packages/[name]/docs.vue` | Major refactor — orchestration only |
| `web/app/components/docs/DocsSidebar.vue` | New — virtual sidebar |
| `web/app/components/docs/DocsCommandPalette.vue` | New — ⌘K overlay |
| `web/app/components/docs/DocsSymbolCard.vue` | New — extracted symbol renderer |
| `web/package.json` | Add `vue-virtual-scroller`, `fuse.js` |

---

## Out of Scope

- True virtual scrolling of main content (requires fixed item heights; docstrings vary too much)
- Mobile command palette (sidebar is already hidden on mobile; ⌘K is desktop-only)
- Persistent last-visited symbol across page navigation
