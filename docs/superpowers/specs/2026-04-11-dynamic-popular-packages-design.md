# Dynamic Popular Packages — Design Spec

**Date:** 2026-04-11
**Status:** Approved

## Overview

Replace the hardcoded "Popular Packages" section on the homepage with a live feed of the top 12 packages by 30-day download count, sourced from the existing search index. Add two more rows of packages (6 → 12 total cards).

## Background

The background worker already fetches the hugovk/top-pypi-packages 30-day dataset every 6 hours and stores download counts in the SQLite FTS5 search index (`packages_meta` table). This data is used to sort search results but is not exposed as a standalone endpoint. The homepage currently hardcodes 6 packages with fake download numbers.

## Architecture

### Backend — `GET /api/popular`

**New file:** `api/internal/handler/popular.go`

- A `PopularHandler` that holds a reference to the existing `*search.Index`.
- Queries `packages_meta` for the top N packages: `SELECT name, summary, downloads FROM packages_meta WHERE downloads > 0 ORDER BY downloads DESC LIMIT ?`
- Default limit: 12. Max limit: 50. Parsed from `?limit=` query param; clamped to max.
- Response: `[]SearchResult` (existing type — `name`, `summary`, `downloads`) serialized as JSON.
- Cached in the existing multi-layer cache (memory LRU + SQLite) with a 6-hour TTL, matching the worker sync interval.
- Cache key: `popular:{limit}`.
- Route registered in `main.go`: `GET /api/popular`

The `search.Index` needs one new exported method: `TopByDownloads(limit int) ([]SearchResult, error)` — a direct SQL query, no FTS involved.

### Frontend — `index.vue`

- Replace the hardcoded `trendingPackages` ref with a `useFetch('/api/popular?limit=12')`.
- While pending: render a 12-card skeleton grid (same dimensions as real cards, zinc-800 background, no text).
- On error: render an empty state — the grid area is hidden and a single muted line reads "Could not load popular packages."
- No changes to `TrendingPackages.vue` — it already accepts `SearchResult[]` and renders the responsive 3-column grid.
- No new types needed — `SearchResult` is already defined in `~/types/api.ts`.

## Data Flow

```
Worker (every 6h)
  → hugovk top-pypi-packages JSON
  → UpdateDownloadsBatch → packages_meta.downloads

GET /api/popular?limit=12
  → PopularHandler.Get()
  → cache hit (6h TTL) → return []SearchResult
  → cache miss → search.Index.TopByDownloads(12) → cache → return

Homepage (index.vue)
  → useFetch /api/popular?limit=12
  → TrendingPackages.vue (12 cards, 4 rows × 3 cols)
```

## Error Handling

- If the search index has no packages with `downloads > 0` (e.g. fresh server before first sync), the endpoint returns an empty array `[]` with HTTP 200. The frontend shows the empty state.
- Upstream failures (hugovk dataset unavailable) leave existing cached download counts intact; the popular endpoint continues serving stale-but-valid data until the next successful sync.

## Out of Scope

- Pagination or infinite scroll of popular packages.
- Category filtering (e.g. "popular in ML").
- Per-package real-time stats on the homepage cards (downloads shown are 30-day totals from the index, not live pypistats).
