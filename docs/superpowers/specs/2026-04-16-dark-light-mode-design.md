# Dark / Light Mode Design Spec

**Date:** 2026-04-16
**Branch:** `feat.dark-light-mode`
**Status:** Approved — ready for implementation

---

## Overview

Add a three-way theme system (light / dark / system) to pypx with user preference persisted via cookie (SSR-safe, no FOUC). The existing UI is entirely dark-mode with hardcoded zinc classes; this work introduces a semantic token layer that flips all colors at a single CSS boundary, plus a new `ThemeToggle` component in the header and command palette.

---

## Decisions Made

| Question | Decision |
|---|---|
| Number of modes | Three-way: light / dark / system |
| Default | System (respects OS preference) |
| Toggle placement | Header (right side) + command palette |
| Toggle style | Monotone icon button — Moon / Sun / Monitor SVGs |
| Light palette | Soft off-white: zinc-50 page, white surfaces, zinc-200 borders |
| Color system | Hybrid: semantic CSS tokens as Tailwind utilities + `dark:` for exceptions |
| Brand green on light | emerald-700 (#047857) — 5.25:1 contrast on zinc-50 ✅ AA |

---

## Section 1 — Semantic Token System

Six tokens cover ~90% of all 168 zinc class occurrences across 22 files. Registered as `@theme inline` in `main.css` so Tailwind generates real utility classes.

### Token definitions

| Token | Tailwind class | Light value | Dark value | Used for |
|---|---|---|---|---|
| `--color-base` | `bg-base` | zinc-50 `#fafafa` | zinc-950 `#09090b` | Page background, layout wrapper |
| `--color-surface` | `bg-surface` | white `#ffffff` | zinc-900 `#18181b` | Cards, panels, dropdowns |
| `--color-raised` | `bg-raised` | zinc-100 `#f4f4f5` | zinc-800 `#27272a` | Inputs, kbd hints, hover states, inline code |
| `--color-subtle` | `border-subtle`, `bg-subtle` | zinc-200 `#e4e4e7` | zinc-800 `#27272a` | All borders — cards, inputs, header, tables |
| `--color-primary` | `text-primary` | zinc-900 `#18181b` | zinc-50 `#fafafa` | Headings, package names, labels |
| `--color-muted` | `text-muted` | zinc-500 `#71717a` | zinc-400 `#a1a1aa` | Descriptions, metadata, placeholders |

> **Note:** In Tailwind 4, `@theme inline { --color-foo: var(--color-foo); }` generates utilities named after the variable suffix. Variable names are chosen to match the intended utility name exactly — e.g. `--color-raised` → `bg-raised`, `--color-subtle` → `border-subtle`.

### CSS structure in `main.css`

```css
@theme inline {
  --color-base: var(--color-base);
  --color-surface: var(--color-surface);
  --color-raised: var(--color-raised);
  --color-subtle: var(--color-subtle);
  --color-primary: var(--color-primary);
  --color-muted: var(--color-muted);
}

:root {
  --color-base:    #fafafa;
  --color-surface: #ffffff;
  --color-raised:  #f4f4f5;
  --color-subtle:  #e4e4e7;
  --color-primary: #18181b;
  --color-muted:   #71717a;
}

.dark {
  --color-base:    #09090b;
  --color-surface: #18181b;
  --color-raised:  #27272a;
  --color-subtle:  #27272a;
  --color-primary: #fafafa;
  --color-muted:   #a1a1aa;
}
```

### Migration pattern per component

```
bg-zinc-950      →  bg-base
bg-zinc-900      →  bg-surface
bg-zinc-800      →  bg-raised
border-zinc-800  →  border-subtle
text-zinc-50     →  text-primary
text-zinc-400    →  text-muted
text-zinc-500    →  text-muted
```

One-off colors that don't map cleanly to a token use `dark:` variants directly.

---

## Section 2 — Brand Color Tokens

The mint brand green (#4ade80) has only **1.67:1** contrast on zinc-50 — a complete WCAG failure. All brand tokens get light-mode values in `:root` and restore their original dark values under `.dark`.

### Contrast audit (on zinc-50 #fafafa)

| Color | Ratio | Result |
|---|---|---|
| #4ade80 green-400 (current brand) | 1.67:1 | ❌ FAIL |
| #22c55e green-500 | 2.18:1 | ❌ FAIL |
| #16a34a green-600 | 3.16:1 | ⚠️ AA-Large only |
| #15803d green-700 | 4.81:1 | ✅ AA |
| **#047857 emerald-700** | **5.25:1** | **✅ AA — selected** |

emerald-700 is preferred over green-700: it is cool and blue-shifted (vs earthy/forest), feels closer to the brand's "tech" character, and has more contrast headroom.

### Brand token values

```css
:root {
  --color-brand:        #047857;              /* emerald-700 */
  --color-brand-light:  #059669;              /* emerald-600 — hover states only */
  --color-brand-muted:  rgba(4, 120, 87, 0.08);
  --color-brand-border: rgba(4, 120, 87, 0.25);
}

.dark {
  --color-brand:        #4ade80;              /* original mint */
  --color-brand-light:  #86efac;
  --color-brand-muted:  rgba(74, 222, 128, 0.08);
  --color-brand-border: rgba(74, 222, 128, 0.25);
}
```

---

## Section 3 — Preference Persistence

**Module:** `@nuxtjs/color-mode` v4 (already installed, not yet wired up).

**Why not VueUse `useDark`:** VueUse uses localStorage, which is unavailable during SSR. This causes a flash of the wrong theme on first load. `@nuxtjs/color-mode` uses a cookie that Nuxt reads server-side, rendering the correct theme on the initial response.

### `nuxt.config.ts` addition

```ts
modules: [
  '@vueuse/nuxt',
  '@nuxtjs/seo',
  '@nuxtjs/color-mode',
],

colorMode: {
  classSuffix: '',       // writes 'dark' not 'dark-mode' to <html>
  defaultValue: 'system',
  storageKey: 'pypx-color-mode',
},
```

### Tailwind 4 dark variant

Add to `main.css` after `@import "tailwindcss"`:

```css
@variant dark (&:where(.dark, .dark *));
```

This makes `dark:` utility variants activate when `.dark` is on any ancestor element (i.e. `<html class="dark">`).

---

## Section 4 — ThemeToggle Component

**File:** `web/app/components/ThemeToggle.vue`

- Calls `useColorMode()` from `#imports` (auto-imported by `@nuxtjs/color-mode`)
- Cycles through `light → dark → system` on click
- Displays the icon for the current resolved mode:
  - **Moon** — dark
  - **Sun** — light
  - **Monitor** — system
- All icons are inline SVG, `stroke="currentColor"`, 18×18, `stroke-width="2"`
- Styled as an icon button: `rounded-md p-1.5 text-muted hover:text-primary hover:bg-raised transition-colors`
- `aria-label` describes the *next* action (what clicking will do): when current mode is `light` → `"Switch to dark mode"`, when `dark` → `"Switch to system theme"`, when `system` → `"Switch to light mode"`

### Placement in `AppHeader.vue`

Added at the far right of the header flex row, after the search wrapper:

```html
<div class="ml-auto flex items-center gap-2">
  <ThemeToggle />
</div>
```

### Command palette integration

`CommandPalette.vue` gets a `Theme` section with three items:

```
☀ Light mode       — sets colorMode.preference = 'light'
🌙 Dark mode        — sets colorMode.preference = 'dark'
⬜ System default   — sets colorMode.preference = 'system'
```

Active mode item shows a checkmark indicator. Icons are monotone inline SVG matching the toggle.

---

## Section 5 — Shiki Dual-Theme

**File:** `web/app/utils/shikiHighlight.ts`

Load both themes and use the dual-theme API:

```ts
highlighterPromise = createHighlighter({
  themes: ['one-dark-pro', 'github-light'],
  langs: ['python'],
});

// in highlightPython():
return highlighter.codeToHtml(code, {
  lang: 'python',
  themes: {
    dark: 'one-dark-pro',
    light: 'github-light',
  },
});
```

Shiki outputs `--shiki-dark` / `--shiki-light` CSS variables on each token span instead of hard-coded colors.

Add to `main.css`:

```css
.shiki, .shiki span {
  color: var(--shiki-light);
  background-color: var(--shiki-light-bg);
}
.dark .shiki, .dark .shiki span {
  color: var(--shiki-dark);
  background-color: var(--shiki-dark-bg);
}
```

The HTML output cached by the Go API is theme-aware with no backend changes needed.

---

## Section 6 — Python Syntax Tokens (Light Mode)

The `--py-*` variables currently hold One Dark palette values tuned for dark backgrounds. They need accessible alternatives for light mode. The `.dark` block restores the originals.

```css
:root {
  --py-keyword:   #7c3aed;  /* violet-700 — dark purple on white */
  --py-name:      #b45309;  /* amber-700 */
  --py-param:     #374151;  /* gray-700 */
  --py-type:      #1d4ed8;  /* blue-700 */
  --py-default:   #047857;  /* emerald-700 — matches brand */
  --py-punct:     #6b7280;  /* gray-500 */
  --py-decorator: #be123c;  /* rose-700 */
}

.dark {
  --py-keyword:   #c678dd;
  --py-name:      #e5c07b;
  --py-param:     #abb2bf;
  --py-type:      #61afef;
  --py-default:   #98c379;
  --py-punct:     #636d83;
  --py-decorator: #e06c75;
}
```

`PySignature.vue` and `PyDocstring.vue` reference these via `var(--py-*)` already — no component changes.

`PyDocstring.vue` has three hardcoded hex values in `<style scoped>` that need replacing with semantic tokens:

```css
/* before */
background: #27272a;   /* inline code bg */
background: #0f0f10;   /* pre block bg */
border: 1px solid #27272a;

/* after */
background: var(--color-surface-raised);
background: var(--color-base);
border: 1px solid var(--color-border);
```

---

## Section 7 — Markdown Prose

`.prose-invert` is currently applied alongside `.prose` everywhere markdown is rendered. Under the token system, `.prose` becomes self-adapting:

- Remove `.prose-invert` from all render sites
- Update `.prose` styles in `main.css` to use semantic tokens for link color, heading color, code background, blockquote border, table borders, `<hr>` color
- The `.prose-invert` class definition can be deleted entirely

---

## Section 8 — Decorative Elements

`body::before` (grid) and `body::after` (radial gradient) use hard-coded dark-mode rgba values.

Light mode drops the opacity significantly so both decorations become subtle textures rather than visual statements. The rgba brand color also shifts to the emerald-700 equivalents.

```css
/* Grid lines */
body::before {
  background-image:
    linear-gradient(rgba(4, 120, 87, 0.03) 1px, transparent 1px),
    linear-gradient(90deg, rgba(4, 120, 87, 0.03) 1px, transparent 1px);
}
.dark body::before {
  background-image:
    linear-gradient(rgba(74, 222, 128, 0.04) 1px, transparent 1px),
    linear-gradient(90deg, rgba(74, 222, 128, 0.04) 1px, transparent 1px);
}

/* Radial glow */
body::after {
  background: radial-gradient(circle, rgba(4, 120, 87, 0.04), transparent 70%);
}
.dark body::after {
  background: radial-gradient(circle, rgba(74, 222, 128, 0.08), transparent 70%);
}
```

---

## Section 9 — Component Migration Scope

**22 files** require mechanical zinc → token class substitutions. No logic changes.

Files with the most occurrences:
- `pages/packages/[name]/docs.vue` — 30 occurrences
- `pages/packages/[name]/[version].vue` — 26 occurrences
- `components/PackageOverview.vue` — 16 occurrences
- `components/PackageVersions.vue` — 12 occurrences

**New files:**
- `components/ThemeToggle.vue`

**Modified files (non-mechanical):**
- `nuxt.config.ts` — add `@nuxtjs/color-mode` module + config
- `assets/css/main.css` — tokens, dark variant, Shiki rules, prose update, py tokens, decorative elements
- `utils/shikiHighlight.ts` — dual-theme Shiki
- `components/PyDocstring.vue` — 3 hardcoded hex → tokens
- `components/AppHeader.vue` — add `<ThemeToggle />`
- `components/CommandPalette.vue` — add Theme section
- `layouts/default.vue` — `bg-zinc-950 text-zinc-50` → `bg-base text-primary`

---

## Testing Checklist

- [ ] Light mode renders correctly on first SSR load (no FOUC)
- [ ] Dark mode renders correctly on first SSR load
- [ ] System mode follows OS preference on first load
- [ ] Preference persists across page navigations
- [ ] Preference persists after browser close/reopen
- [ ] Theme toggle cycles correctly (light → dark → system → light)
- [ ] Command palette theme items set mode correctly
- [ ] Shiki code blocks render correct theme in both modes
- [ ] Python signatures render legible token colors in both modes
- [ ] Prose markdown renders without `.prose-invert` in both modes
- [ ] Brand green passes contrast check in light mode (emerald-700 5.25:1)
- [ ] No hydration mismatch warnings in console
