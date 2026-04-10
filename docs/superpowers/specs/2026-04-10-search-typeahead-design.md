# Search Typeahead Design

**Date:** 2026-04-10
**Status:** Draft

## Overview

Add a typeahead dropdown to both search surfaces (header input and Cmd+K modal) so users get instant package suggestions as they type. Both surfaces share a single composable and dropdown component.

## Current State

- **Header input** (`AppHeader.vue`): Text input that submits to `/search?q=...` on Enter. No real-time suggestions.
- **Command palette** (`CommandPalette.vue`): Cmd+K modal with debounced search, keyboard nav, result list. Has its own inline search logic.
- **API**: `GET /api/search?q=&limit=` backed by SQLite FTS5 with prefix matching. 150ms debounce in the command palette.
- **Search results page** (`search.vue`): Full results page at `/search?q=...` with result cards.

## Design

### Architecture: Shared Composable + Dumb Dropdown

**`useSearchTypeahead()` composable** — owns all search + interaction logic:

- `query`: ref bound to input via v-model
- `results`: ref populated from API response
- `selectedIndex`: ref for keyboard navigation (-1 = no selection)
- `isOpen`: ref controlling dropdown visibility
- `isLoading`: ref for loading state
- Debounced API call (150ms) via `useApi().searchPackages(query, 5)` — 5 results max
- Opens dropdown on input focus if query is non-empty
- `open()`, `close()`, `reset()` methods

Keyboard handling (provided as a single `onKeydown` handler):

| Key | Behavior |
|-----|----------|
| Arrow Down | Move selection down (from -1 to 0, clamp at last result) |
| Arrow Up | Move selection up (clamp at -1 = no selection) |
| Enter (selectedIndex >= 0) | Navigate to `/packages/{name}` of selected result |
| Enter (selectedIndex == -1) | Navigate to `/search?q=...` (full results page) |
| Escape | Close dropdown, blur input |

Click-outside detection closes the dropdown.

**`SearchDropdown.vue`** — stateless render component:

- Props: `results: SearchResult[]`, `selectedIndex: number`, `loading: boolean`
- Emits: `select(result: SearchResult)`, `hover(index: number)`
- Renders up to 5 result rows: package name (font-medium, zinc-200) + summary (truncated, zinc-500)
- Selected row gets `bg-zinc-800` highlight
- Footer row with keyboard hints: `↑↓ navigate · ↵ select · esc close`
- Empty state: hidden when no query; "No results" text when query has no matches
- Loading state: subtle spinner or skeleton in place of results

### Consumer: Header Input (`AppHeader.vue`)

- Replace plain `<input>` with v-model bound to composable's `query`
- Add `⌘K` badge inside input (right side), styled as `bg-zinc-800 text-zinc-500 font-mono text-xs px-1.5 py-0.5 rounded`
- Render `<SearchDropdown>` positioned absolutely below the input (`top-full mt-1`)
- On form submit: still navigates to `/search?q=...` (composable's Enter-with-no-selection handles this)
- Cmd+K global shortcut: opens the command palette modal (unchanged)
- Click on `⌘K` badge: also opens the command palette modal

### Consumer: Command Palette (`CommandPalette.vue`)

- Refactor to use `useSearchTypeahead()` instead of its own inline `performSearch`, `selectedIndex`, and `onKeydown`
- Render `<SearchDropdown>` inline inside the modal body (below the input, no absolute positioning needed)
- Keep existing modal open/close behavior (Cmd+K toggle, Escape close, backdrop click close)
- Remove duplicated search logic: `useDebounceFn`, manual `selectedIndex` management, manual keyboard handler

### What Changes

| File | Change |
|------|--------|
| `composables/useSearchTypeahead.ts` | **New** — shared composable |
| `components/SearchDropdown.vue` | **New** — shared dropdown component |
| `components/AppHeader.vue` | **Modified** — wire up composable, add dropdown + ⌘K badge |
| `components/CommandPalette.vue` | **Modified** — replace inline search logic with composable |

### What Doesn't Change

- API endpoint (`GET /api/search`) — no backend changes needed
- Search results page (`search.vue`) — still the full results destination
- `useApi().searchPackages()` — already exists, reused as-is
- Landing page search form (`index.vue`) — remains a simple submit-to-search-page form

## Interaction Flow

```
User types in header input
  → composable debounces 150ms
  → calls searchPackages(query, 5)
  → results populate dropdown
  → user arrows to result, presses Enter → /packages/{name}
  → user presses Enter with no selection → /search?q=...
  → user clicks result → /packages/{name}
  → user clicks outside → dropdown closes
  → user presses Escape → dropdown closes, input blurs

User presses Cmd+K
  → command palette modal opens
  → same composable instance powers it
  → identical behavior as above, just inside the modal
  → Escape closes the entire modal
```

## Visual Design

- Dropdown: `bg-zinc-900/95 backdrop-blur border border-zinc-800 rounded-lg shadow-xl`
- Result row: `px-3 py-2` with name (zinc-200, font-medium, text-sm) and summary (zinc-500, text-sm, truncate)
- Selected row: `bg-zinc-800`
- Footer: `border-t border-zinc-800 px-3 py-1.5 text-xs text-zinc-600`
- ⌘K badge in header input: `bg-zinc-800 text-zinc-500 font-mono text-[10px] px-1.5 py-0.5 rounded`
- Max 5 results shown

## Edge Cases

- **Empty query**: Dropdown hidden, no API call
- **Query shorter than 2 chars**: Still search (FTS5 handles single-char queries fine, and exact package names like "s3" exist)
- **API error**: Close dropdown silently, user can still press Enter to go to full search page
- **Rapid typing**: Debounce prevents excess calls; only the last query fires
- **SSR**: Composable only activates client-side (dropdown is interactive-only). No SSR rendering of dropdown.
- **Mobile**: Dropdown renders below input as normal. No Cmd+K on mobile; header input is the primary surface.
