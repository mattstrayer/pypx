# Mobile Friendliness & Cursor Fixes

**Date:** 2026-04-11  
**Status:** Approved

## Overview

Two focused improvements to the web frontend:
1. Add `cursor-pointer` to all interactive button elements that were missing it (Tailwind preflight sets buttons to `cursor: default`)
2. Fix mobile layout issues across the package detail and home pages

## Cursor Fixes

Add `cursor-pointer` class to every `<button>` in the following files:

| File | Buttons |
|------|---------|
| `web/app/components/InstallCommand.vue` | Package manager tabs (`uv`, `pip`, `poetry`, `pipx`) + copy button |
| `web/app/pages/packages/[name].vue` | Package detail tabs (`Overview`, `Dependencies`, `Versions`, `Stats`) |
| `web/app/components/PackageStats.vue` | Period toggle buttons (`4w`, `3m`, `6m`) |

No logic changes — purely additive class additions.

## Mobile Layout Fixes

### `PackageVersions.vue` — Column hiding (approach B)

Hide the `Size` and `Format` columns on mobile viewports. These are secondary metadata; the primary use case is finding a version by number and release date.

- Add `hidden sm:table-cell` to the `<th>` for Size and Format
- Add `hidden sm:table-cell` to every `<td>` for Size and Format columns

### `AppHeader.vue` — Hide ⌘K hint on mobile

The `<kbd>⌘K</kbd>` shortcut hint inside the search input has no utility on mobile (no keyboard, no shortcut). Hide it with `hidden sm:inline`.

### `[name].vue` — Scrollable tabs

The four package detail tabs (`Overview`, `Dependencies`, `Versions`, `Stats`) are in a horizontal flex row. On a 320px screen they get cramped.

- Add `overflow-x-auto` to the tab row container div
- Add `whitespace-nowrap` to each tab `<button>` so text never wraps mid-word

### `[name].vue` — Wrapping package header

The package name + version badge use `flex items-baseline gap-3`. Long package names can push the badge off-screen.

- Change to `flex flex-wrap items-baseline gap-3` so the badge wraps below the name on small screens.

### `PackageStats.vue` — Period row wrapping

The period toggle row uses `flex items-center gap-1`. The date range label appended after the buttons can overflow on small screens.

- Add `flex-wrap` to the container so the date range label wraps to a second line when needed.

## Files Changed

- `web/app/components/InstallCommand.vue`
- `web/app/components/PackageStats.vue`
- `web/app/components/PackageVersions.vue`
- `web/app/components/AppHeader.vue`
- `web/app/pages/packages/[name].vue`

## Out of Scope

- Card layout for versions table (deferred, B chosen over C)
- Responsive navigation (header is simple enough as-is)
- Touch-specific interactions
