# pypx Brand Guidelines

**Date:** 2026-04-11  
**Status:** Approved  

---

## Overview

pypx is a dark-first developer tool for exploring Python packages. The brand should feel like a terminal that grew up — precise, fast, and confident — without tipping into "trying too hard to be hacker-y." Green is the single accent; zinc does everything else.

---

## Color

### Brand Tokens

Four CSS custom properties defined in `web/app/assets/css/main.css`, inside `:root`:

```css
:root {
  --color-brand:        #4ade80;               /* green-400 — primary accent */
  --color-brand-light:  #86efac;               /* green-300 — hover / emphasis */
  --color-brand-muted:  rgba(74, 222, 128, 0.08);  /* subtle fill */
  --color-brand-border: rgba(74, 222, 128, 0.25);  /* badge/pill borders */
}
```

### Token Usage Map

| Token | Used on |
|---|---|
| `--color-brand` | Logo wordmark, prose links (default), badge text, active nav indicator, primary button text/border |
| `--color-brand-light` | Link hover, button hover, focus ring, search border on focus |
| `--color-brand-muted` | Badge/pill/tag background fills, subtle row hover backgrounds |
| `--color-brand-border` | Badge/pill/tag borders (always paired with `--color-brand-muted` fill) |

### What stays zinc

Everything structural stays zinc. No green on:
- Page/surface backgrounds (`bg-zinc-950`, `bg-zinc-900`)
- Card borders in their resting state (`border-zinc-800`)
- Body text or secondary text (`text-zinc-400`, `text-zinc-500`)
- Section labels like "POPULAR PACKAGES"
- Dividers, separators, layout chrome

The rule: **green means "you can interact with this" or "this is pypx."** Zinc means structure.

### Existing indigo references

The codebase currently uses `indigo-400` / `indigo-300` for prose links in `.prose-invert`. Replace with `--color-brand` / `--color-brand-light` as part of implementation.

---

## Logo & Wordmark

**Treatment:** Plain bold text. No icon, no mark.

- Font: Geist, `font-weight: 700`, `tracking-tight`
- Color: `--color-brand` (`#4ade80`) — always, in all contexts
- The wordmark is the only place green appears unconditionally (i.e. not triggered by interaction)

**Do not:**
- Render the wordmark in zinc or white
- Add decorative elements (chevrons, boxes, underlines) to the wordmark
- Change the font weight or tracking

---

## Typography

No changes. Geist is already the right call.

| Role | Font | Notes |
|---|---|---|
| Body / UI | Geist (sans) | All prose, labels, nav, descriptions |
| Code / Mono | Geist Mono | Code blocks, package names in badges, keyboard shortcuts, download stats, install commands |

The terminal hybrid feel comes from mixing Geist and Geist Mono deliberately — mono for data and commands, sans for everything human-readable.

---

## Interactive Element Patterns

### Links (prose)
```css
color: var(--color-brand);
/* hover: */
color: var(--color-brand-light);
```

### Focus rings (all inputs)
```css
border-color: var(--color-brand-light);
box-shadow: 0 0 0 2px var(--color-brand-muted);
```
Replace the current `focus:border-zinc-600 focus:ring-1 focus:ring-zinc-600` pattern.

### Badges / pills
```css
background: var(--color-brand-muted);
color: var(--color-brand);
border: 1px solid var(--color-brand-border);
```
Replace the current `bg-zinc-800 text-zinc-500` pattern on package metadata badges.

### Copy / primary action buttons
```css
color: var(--color-brand);
border-color: var(--color-brand-border);
background: var(--color-brand-muted);
/* hover: */
color: var(--color-brand-light);
border-color: var(--color-brand);
```

### Active nav / selected states
```css
color: var(--color-brand);
```

---

## Implementation Scope

The following files need updates during implementation:

| File | Change |
|---|---|
| `web/app/assets/css/main.css` | Add 4 brand tokens to `:root`; replace indigo with brand in `.prose-invert` |
| `web/app/pages/index.vue` | Hero title color; search input focus ring |
| `web/app/components/AppHeader.vue` | Logo link: `text-zinc-50 hover:text-white` → `text-[var(--color-brand)] hover:text-[var(--color-brand-light)]`; search input focus ring |
| `web/app/components/TrendingPackages.vue` | Package name hover (optional); no badges here currently |
| `web/app/components/PackageBadges.vue` | Badge background/text/border → brand tokens |
| `web/app/components/InstallCommand.vue` | Copy button → brand tokens |
| `web/app/components/PackageVersions.vue` | Any active/selected version state |
| `web/app/components/PackageStats.vue` | Chart line color (if applicable) |

---

## Tailwind Usage

The brand tokens are CSS custom properties, not Tailwind config extensions. Reference them with Tailwind's arbitrary value syntax:

```html
<!-- text -->
<span class="text-[var(--color-brand)]">pypx</span>

<!-- background -->
<div class="bg-[var(--color-brand-muted)]">...</div>

<!-- border -->
<div class="border-[var(--color-brand-border)]">...</div>
```

Prefer arbitrary values over inline styles to keep the codebase consistent with its Tailwind-first approach. Use inline styles only when a value needs to be dynamic (e.g. driven by a prop or computed).

---

## Do / Don't

| Do | Don't |
|---|---|
| Use green to signal interactivity | Use green on decorative or structural elements |
| Let zinc carry the layout | Add green backgrounds to cards or sections |
| Use Geist Mono for data (stats, versions, commands) | Mix sans/mono randomly |
| Keep the wordmark always green | Render "pypx" in white or zinc |
| Pair `brand-muted` fill with `brand-border` border | Use `brand-border` without a fill (too subtle) |
