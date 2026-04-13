# SEO & OG Images Design

**Date:** 2026-04-13  
**Status:** Approved

## Overview

Add dynamic Open Graph images, a structured sitemap, JSON-LD schema, and a global grid background to pypx. Goal: maximum SEO value and polished link unfurls when users share package pages on social media.

## Stack

- **OG image renderer:** `takumi` (replaces default Satori renderer in `nuxt-og-image`)
- **OG image module:** `nuxt-og-image` (already bundled in `@nuxtjs/seo`)
- **Sitemap:** `@nuxtjs/sitemap` (already bundled in `@nuxtjs/seo`)
- **Schema:** `useSchemaOrg()` from `@nuxtjs/seo` — no new packages needed

**New package:** `takumi`

## OG Image Templates

Three Vue SFC templates in `web/app/components/OgImage/`:

### PackageCard.vue
Used on `/packages/[name]` and `/packages/[name]/[version]`.

**Props:** `name`, `version`, `summary`, `downloads` (formatted, e.g. "280M/mo"), `license`

**Visual design:**
- Background: `#09090b`
- Grid overlay: `linear-gradient` at `rgba(74,222,128,0.04)`, `32px` spacing, both axes
- Ambient glow: `radial-gradient` top-right, `rgba(74,222,128,0.15)` → transparent, ~220px radius
- Font: Geist (loaded via Google Fonts)
- Layout (top to bottom):
  1. Package name (`46px`, `700`, `#f4f4f5`) + version badge (green-tinted mono pill)
  2. Inline metadata: `license · downloads/mo`
  3. Summary text (`16px`, `#a1a1aa`, max 2 lines)
  4. Footer: `pypx.app — The Python Package Index, reimagined` (left) · `pypx` brand mark in `#4ade80` (right)

### DocsCard.vue
Used on `/packages/[name]/docs`.

**Props:** `name`, `version`

**Visual design:** Identical to PackageCard with two changes:
- "API Docs" badge (green-tinted, uppercase, `letter-spacing: 1.5px`) rendered above the package name
- No summary text, no downloads/license metadata row

### SiteCard.vue
Used on `/` and `/search`. Static — no dynamic props.

**Visual design:** Same grid + glow background.
- Large `pypx` wordmark centered (`64px`, `700`, `#4ade80`)
- Tagline below: `"The Python Package Index, reimagined"` (`18px`, `#a1a1aa`)
- Footer: `pypx.app`

## Page Integration

Each page calls `defineOgImage` after its async data loads:

**`/packages/[name].vue`:**
```ts
defineOgImage({
  component: 'PackageCard',
  props: {
    name: pkg.value.name,
    version: pkg.value.version,
    summary: pkg.value.summary,
    downloads: formatDownloads(pkg.value.monthly_downloads),
    license: pkg.value.license,
  },
})
```

**`/packages/[name]/docs.vue`:**
```ts
defineOgImage({
  component: 'DocsCard',
  props: {
    name: pkg.value.name,
    version: pkg.value.version,
  },
})
```

**`/packages/[name]/[version].vue`:** Same as PackageCard, using versioned package data.

**`/index.vue` and `/search.vue`:**
```ts
defineOgImage({ component: 'SiteCard' })
```

## Sitemap

A Nuxt server route at `server/api/__sitemap__/urls.ts` returns dynamic package URLs. The sitemap module is configured with two named sitemaps.

### nuxt.config.ts sitemap config
```ts
sitemap: {
  sitemaps: {
    popular: {
      include: ['/packages/**'],
      sources: ['/api/__sitemap__/urls?source=popular'],
    },
    cached: {
      include: ['/packages/**'],
      sources: ['/api/__sitemap__/urls?source=cached'],
    },
  },
}
```

### `/server/api/__sitemap__/urls.ts`

**`source=popular`:** Fetches `/popular?limit=5000` from the Go API (uses `NUXT_API_BASE` env var, falls back to `http://localhost:8080`). Returns up to 5,000 package URLs:
```ts
{ loc: `/packages/${name}`, priority: 1.0, changefreq: 'daily' }
```

**`source=cached`:** Queries the SQLite cache DB (path from env `SQLITE_PATH`, default `./pypx.db`) for all keys matching `pkg:%` using `SELECT DISTINCT SUBSTR(key, 5) FROM cache WHERE key LIKE 'pkg:%'`. Returns package URLs:
```ts
{ loc: `/packages/${name}`, priority: 0.5, changefreq: 'weekly' }
```

Static routes (`/`, `/search`) are discovered automatically by the sitemap module from the Nuxt router — no manual addition needed.

### Deduplication
The sitemap module deduplicates across sitemaps by `loc` automatically. A package in both lists will appear once in `popular` only.

## JSON-LD Structured Data

On `/packages/[name].vue`, after SSR data loads, call `useSchemaOrg`:

```ts
useSchemaOrg([
  defineSoftwareApplication({
    name: pkg.value.name,
    description: pkg.value.summary,
    softwareVersion: pkg.value.version,
    applicationCategory: 'DeveloperApplication',
    license: pkg.value.license ?? undefined,
    url: `https://pypi.org/project/${pkg.value.name}/`,
  }),
])
```

Only applied to `/packages/[name]` (the main package page), not docs or versioned routes. `useSchemaOrg` is reactive — if `pkg` is null during SSR (error state), pass nothing.

## Global Grid Background

Add to `web/app/assets/css/main.css`:

```css
body::before {
  content: '';
  position: fixed;
  inset: 0;
  z-index: -1;
  background-image:
    linear-gradient(rgba(74, 222, 128, 0.04) 1px, transparent 1px),
    linear-gradient(90deg, rgba(74, 222, 128, 0.04) 1px, transparent 1px);
  background-size: 32px 32px;
  pointer-events: none;
}

body::after {
  content: '';
  position: fixed;
  top: -120px;
  right: -120px;
  width: 480px;
  height: 480px;
  background: radial-gradient(circle, rgba(74, 222, 128, 0.08), transparent 70%);
  border-radius: 50%;
  z-index: -1;
  pointer-events: none;
}
```

Uses `position: fixed` so the grid stays stable during scroll. `z-index: -1` ensures it sits behind all content. Opacity values are kept low (4% grid, 8% glow) to avoid competing with content on busy pages.

## nuxt.config.ts Changes

```ts
ogImage: {
  enabled: true,
  renderer: 'takumi',
},

sitemap: {
  sitemaps: {
    popular: {
      include: ['/packages/**'],
      sources: ['/api/__sitemap__/urls?source=popular'],
    },
    cached: {
      include: ['/packages/**'],
      sources: ['/api/__sitemap__/urls?source=cached'],
    },
  },
},
```

## Out of Scope

- Breadcrumb JSON-LD (Approach 3 — not worth the complexity for single-level routes)
- OG images for `/packages/[name]/[version]` beyond the PackageCard template (same template, no new work)
- Robots.txt changes (already configured with `allow: '/'`)
- Twitter/X card meta (nuxt-og-image emits these automatically alongside OG tags)
