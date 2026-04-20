# pypx Design Overhaul — Spec

**Date:** 2026-04-20
**Direction:** B+C — Developer-Tool Premium with Data-Dashboard elements
**Scope:** Global header, homepage, package page (overview + stats tabs), docs page

---

## Summary

pypx has excellent data (security, docs, deps, platform coverage, release cadence) but the UI undersells it. A first-time visitor can't tell why this is better than pypi.org. The overhaul pursues two goals simultaneously:

1. **Visual confidence** — stronger type hierarchy, consistent component language, a logomark, a homepage that communicates value.
2. **Better data presentation** — hero download numbers, a sparkline trend chart, a consistent sidebar card system.

No new data sources or API changes are needed. The stats sparkline uses the existing `overall` weekly-bucketed data already returned by `/api/packages/{name}/stats`.

---

## 1. Global: Header

**Current:** Text-only "pypx" wordmark, no visual identity.

**New:**
- Add a logomark: a 26×26px rounded square (background: `brand-dim`, border: `brand-border`) containing `px` in Geist Mono at 9px. Sits left of the "pypx" wordmark text.
- No other header changes needed — search, theme toggle, layout all stay.

**Files:** `web/app/components/AppHeader.vue`

---

## 2. Homepage: Hero Section

**Current:** Large "pypx" heading in brand color, subtitle, search bar. Functional but flat.

**New:**
- **Eyebrow pill** — `500,000+ packages indexed` in a pill (brand-dim bg, brand-border border, brand text, pulsing dot). Sits above the h1.
- **Headline** — split across two lines: `The Python Package` / `Index, reimagined.` Second line in brand color. Font size 52px, weight 700, tracking -0.04em.
- **Search bar** — height bumped to 48px, border-radius 12px. Focus ring uses `rgba(74,222,128,0.08)` box-shadow + brand border. Subtle `box-shadow: 0 1px 3px rgba(0,0,0,0.3)` at rest.
- **Search hint** — add `↑↓ to navigate · ↵ to open · esc to close` in small muted text below the input (replaces the invisible kbd hint).

**Files:** `web/app/pages/index.vue`

---

## 3. Homepage: Feature Strip

**Current:** Nothing communicates what makes pypx special.

**New:** A 3-column grid strip directly below the search area (margin-top: 64px from search). Uses a 1px-gap grid layout on a `subtle` background so the dividers between cells appear as thin lines.

Each cell: `surface` background, 22px vertical padding, hover to `raised`. Contains:
- A 32×32 icon badge (brand-dim bg, brand-border border, 8px border-radius)
- A title (13.5px, 600 weight)
- A description (12.5px, muted, ~2 lines)

Three features:
| Icon | Title | Description |
|------|-------|-------------|
| 📦 | API Documentation | Browse extracted docs from any package — functions, classes, type signatures, and docstrings. |
| 🔒 | Security Advisories | CVE and vulnerability data from OSV.dev. Know if a package has known issues before you install. |
| 🌿 | Dependency Analysis | Full dependency tree with optional extras, platform coverage, and install size estimates. |

**Files:** `web/app/pages/index.vue`

---

## 4. Homepage: Section Header + Trending Cards

**Current:** All-caps "TRENDING" label in small muted text. Cards show name + downloads only; summary is visually clipped.

**New section header:** A flex row with the label, a 1px `subtle` line that fills remaining space, and a right-side meta note ("top 24 by downloads · updated daily" in mono/muted). Replaces the bare `<h2>`.

**New trending cards:**
- Same 3-column grid, same border/bg tokens.
- Card height increases slightly — summary text gets `min-height: 34px` and is fully visible (2-line clamp, but height guarantees it shows).
- Add a **mini proportional bar** at the bottom of each card: 2px height, `raised` track, gradient brand fill. Width is `(downloads / maxDownloads) * 100%` — computed in `TrendingPackages.vue` where the max is already available.
- Hover state: `border-color: rgba(74,222,128,0.3)` + `background: rgba(74,222,128,0.03)`.
- Package name hover color: brand (already exists, keep).

**Files:** `web/app/pages/index.vue`, `web/app/components/TrendingPackages.vue`

---

## 5. Homepage: Footer

**Current:** "pypx — not affiliated with PyPI or the PSF"

**New:** Two-column flex layout:
- Left: existing disclaimer
- Right: "data from pypi.org · pypistats.org · osv.dev" in same muted style

**Files:** `web/app/layouts/default.vue`

---

## 6. Package Page: Sidebar Consistency

**Current:** Top two sections are proper cards (bordered, `surface` bg). Below them: GitHub stats, doc link, release cadence, platform coverage, maintainers all use bare `border-t` dividers — visually inconsistent.

**New:** Every sidebar section becomes a consistent card:

```
.sidebar-card {
  background: surface;
  border: 1px solid subtle;
  border-radius: 10px;
  padding: 14px 16px;
}
.sidebar-card-label {
  font-size: 10px; font-weight: 600;
  letter-spacing: 0.08em; text-transform: uppercase;
  color: muted; margin-bottom: 10px;
}
```

**Sections and their new card content:**

- **Details** — unchanged, already a card.
- **Links** — unchanged, already a card.
- **GitHub** — stars, forks, open issues as three `gh-stat` blocks (num + label stacked). Last commit below as small muted text. Only shown when `repoInfo` is present.
- **Release Cadence** — large number (`releases_last_12mo`) as headline, avg days as subtitle. Only shown when `releases_last_12mo > 0`.
- **Platform Coverage** — keep existing `PackagePlatforms` component content but wrap in card.
- **Maintainers** — keep existing `PackageMaintainers` content but wrap in card.

Remove all bare `border-t` dividers from the sidebar.

**Files:** `web/app/components/PackageOverview.vue`, `web/app/components/PackagePlatforms.vue`, `web/app/components/PackageMaintainers.vue`

---

## 7. Package Page: Stats Tab — Hero Numbers

**Current:** Period toggle buttons, then immediately into bar charts.

**New:** Insert a 3-column hero row above the chart:

| Card | Value | Sub-label |
|------|-------|-----------|
| Last N period | sum of `overall` downloads | "downloads" |
| Weekly average | total / weeks | "downloads / week" |
| Peak week | max single `DataPoint` | week label (e.g. "Mar 24 – Mar 30") |

These are computed from the existing `overall` array — no new API data needed. Computed in the Vue component as `computed()` properties.

**Files:** `web/app/components/PackageStats.vue`

---

## 8. Package Page: Stats Tab — Sparkline Chart

**Current:** `overall` (weekly buckets) rendered as horizontal bar chart rows.

**New:** Replace the `overall` bar chart with an SVG area sparkline:

- Pure SVG — no charting library dependency.
- Viewbox `0 0 800 120`, `preserveAspectRatio="none"`, width 100%.
- **Area fill:** `linearGradient` from `rgba(74,222,128,0.25)` → `rgba(74,222,128,0)` top to bottom.
- **Line:** `rgba(74,222,128,0.8)`, 1.8px stroke, smooth cubic bezier path.
- **Dots:** one per data point, 3px radius, brand color. Last dot 4px (emphasis).
- **Grid lines:** 3 horizontal lines at 25%, 50%, 75% of chart height in `rgba(255,255,255,0.04)`.
- **X-axis labels:** first, middle, and last week labels in mono 9.5px muted text below the chart.
- Coordinate mapping: y-axis inverted (SVG 0 = top), scale = `(1 - downloads/max) * 100` mapped to 10–110px range (10px padding top/bottom).
- Smooth curve: cubic bezier using control points 1/3 and 2/3 of the way between adjacent points.

The bar charts for Python versions and OS breakdown move to a **2-column side-by-side grid** below the sparkline (previously stacked vertically). Bar height increases from 4px → 6px. Color coding: indigo for Python versions, amber for OS (already in the existing component).

**Files:** `web/app/components/PackageStats.vue`

---

## 9. Docs Page: Symbol Cards

**Current:** Each symbol is separated by `border-t border-base` — nearly invisible in dark mode. No visual containment. Active symbol only indicated in sidebar, not in the card itself.

**New:** Wrap every `DocsSymbolCard` in a proper card container:

```
border: 1px solid subtle
background: surface
border-radius: 12px
padding: 20px 22px
margin-bottom: 16px
scroll-margin-top: 16px
```

**Active card state:** When `isActive` prop is true:
- `border-color: brand-border`
- `background: rgba(74,222,128,0.03)`
- `box-shadow: 0 0 0 1px rgba(74,222,128,0.08)`

This makes the sidebar and card respond together when scrolling.

**Section labels** ("Parameters", "Returns", "Raises", "Methods"): Bump from 9px → 10px. Add `::after` horizontal rule (same pattern as trending section header) to visually separate content blocks without needing extra margin.

**Parameter rows:** Replace current `border-l-2 border-subtle pl-3` with pill rows:
- `background: raised`, `border-radius: 6px`, `padding: 6px 10px`
- Left accent: `border-left: 2px solid subtle`
- Layout: `param-name` (sky-400 mono) + `param-type` (muted mono) + `param-desc` (muted, right-aligned)

**Returns row:** Same pill treatment but left accent uses `rgba(74,222,128,0.4)` — visually distinguishes return from params.

**Methods toggle button:** Replace unicode `▾`/`▸` with proper SVG chevron. Style as a `raised` hover button with brand chevron icon and a count badge.

**Files:** `web/app/components/docs/DocsSymbolCard.vue`

---

## 10. Docs Page: Context Bar + Sidebar Polish

**Context bar** (`docs.vue`):
- "Jump to symbol" button: change from `bg-raised` to `brand-dim` bg + `brand-border` border + `brand` text. Consistent with the new button language used across the redesign.
- No layout changes needed.

**Sidebar** (`DocsSidebar.vue`):
- Width: 192px → 216px. Accommodates longer symbol names.
- Section headers: replace unicode chevron with SVG chevron (consistent with symbol card methods toggle).
- Count badge: `brand-dim` bg + `brand` text (was: plain `brand-muted` — same tokens, already correct).
- Symbol rows: no changes needed, active state is already correct.

**Files:** `web/app/pages/packages/[name]/docs.vue`, `web/app/components/docs/DocsSidebar.vue`

---

## What's Not Changing

- CSS token system (`--color-base`, `--color-surface`, etc.) — no changes.
- Font stack (Geist + Geist Mono) — no changes.
- Grid background overlay — no changes.
- Dark/light mode implementation — no changes.
- All keyboard shortcuts, command palette — no changes.
- API, Go backend — no changes.
- Docs page data fetching, virtual scroller, deferred rendering, ⌘K palette — no changes.
- Tab structure on package page — no changes.
- Badge system (`PackageBadges.vue`) — no changes.

---

## Implementation Phases

### Phase 1 — Header + Homepage
- Logomark in AppHeader
- Hero eyebrow, headline split, search bar improvements, search hint
- Feature strip (3-column)
- Section header component (label + line + meta)
- Trending card improvements (summary height, mini bar)
- Footer data attribution

### Phase 2 — Package Page Sidebar
- Audit all sidebar sections
- Convert all to consistent card pattern
- GitHub card redesign (stat blocks)
- Release cadence card (big number)
- Wrap PackagePlatforms and PackageMaintainers in cards

### Phase 3 — Stats Tab
- Hero numbers row (3 computed cards)
- SVG sparkline replacing overall bar chart
- 2-column breakdown grid for Python versions + OS

### Phase 4 — Docs Page
- Symbol cards (card container, active state)
- Section labels (size + rule)
- Parameter/returns pill rows
- Methods toggle button (SVG chevron, count badge)
- Context bar "Jump to symbol" button styling
- DocsSidebar width + SVG chevron headers

---

## Files Touched

| File | Change |
|------|--------|
| `web/app/components/AppHeader.vue` | Add logomark |
| `web/app/pages/index.vue` | Hero, feature strip, section header, search hint |
| `web/app/components/TrendingPackages.vue` | Summary height, mini download bar |
| `web/app/layouts/default.vue` | Footer data attribution |
| `web/app/components/PackageOverview.vue` | Sidebar cards, GitHub card, cadence card |
| `web/app/components/PackagePlatforms.vue` | Wrap in card |
| `web/app/components/PackageMaintainers.vue` | Wrap in card |
| `web/app/components/PackageStats.vue` | Hero numbers, sparkline, 2-col breakdown |
| `web/app/components/docs/DocsSymbolCard.vue` | Card container, active state, section labels, param/returns rows |
| `web/app/pages/packages/[name]/docs.vue` | Jump to symbol button styling |
| `web/app/components/docs/DocsSidebar.vue` | Width 216px, SVG chevrons |
