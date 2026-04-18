# Tab Navigation Contrast & Hover Redesign

**Date:** 2026-04-18
**Scope:** `web/app/pages/packages/[name].vue` — in-page tab bar only

## Problem

Inactive tabs use `text-muted` (`#71717a` light / `#a1a1aa` dark), which is low contrast and hard to read at a glance. The only contrast improvement happens on hover (`hover:text-zinc-700 dark:hover:text-zinc-300`), meaning tabs are only readable when you mouse over them. The hover state also provides no brand presence — it's a plain text lightening.

## Decision

Three targeted changes to the tab button classes in `[name].vue:142-164`:

### 1. Default inactive text — bump contrast

**Before:** `text-muted hover:text-zinc-700 dark:hover:text-zinc-300`
**After:** `text-zinc-700 dark:text-zinc-300`

Inactive tabs are always readable. The text contrast that previously only appeared on hover becomes the baseline.

### 2. Hover state — green underline accent

Replace the text-color hover with a brand-colored bottom border at 65% opacity.

All tabs (active and inactive) get `border-b-2 border-transparent` to hold consistent height. Inactive tabs on hover get:

```
hover:border-[rgba(4,120,87,0.65)] dark:hover:border-[rgba(74,222,128,0.65)]
```

Light mode uses the brand emerald (`#047857`) at 65% opacity.
Dark mode uses the dark-mode brand green (`#4ade80`) at 65% opacity.

The opacity level was chosen visually — full brand green read as too loud; 55% was slightly weak; 65% is the confirmed sweet spot.

### 3. Active tab — height consistency

Add `border-b-2 border-transparent` to the active tab class alongside the existing `bg-raised text-primary`. This ensures no height jump as tabs transition between states.

## Affected Elements

- `<button>` tabs (Overview, Dependencies, Versions, Stats) — `v-for` loop at line 142
- `<NuxtLink>` Docs tab — line 158, gets identical inactive treatment

## Not Changed

- Active tab background (`bg-raised`) and text (`text-primary`) — unchanged
- `--color-brand` is not added to `@theme inline` (the change is localized to one component; arbitrary rgba values are cleaner than a theme change for this scope)
- No other components touched
- Light mode tab bar border (`border-subtle`) — unchanged
