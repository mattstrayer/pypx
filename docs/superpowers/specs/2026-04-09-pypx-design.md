# pypx — A Modern PyPI Frontend

**Date:** 2026-04-09
**Status:** Approved

## Overview

pypx is a modern, fast, community-oriented frontend for the Python Package Index (PyPI), inspired by what npmx.dev did for npmjs.org. It provides a superior package browsing and evaluation experience by layering enriched data, modern UX, and developer-focused insights on top of PyPI's existing public APIs.

**Problem:** PyPI.org is functional but feels dated — no instant search, no at-a-glance package insights (install size, dependency health, trends), no dark mode, no keyboard navigation. Developers evaluating packages deserve better tooling.

**Solution:** A Nuxt 4 SSR frontend backed by a Go caching/enrichment API that makes package evaluation fast and delightful.

## Architecture

### System Overview

```
Browser → Cloudflare (edge cache, DDoS) → Caddy (TLS, routing)
                                            ├── /api/*  → Go API (port 8080)
                                            └── /*      → Nuxt SSR (port 3000)
```

Two services in Docker Compose behind Caddy, with Cloudflare in front:

- **Go API** (`api/`): Caching proxy + data aggregator. Fetches from PyPI JSON API and pypistats.org. Precomputes dependency trees, install sizes, compatibility badges. Exposes a clean REST API. Uses SQLite for caching.
- **Nuxt Frontend** (`web/`): Nuxt 4 with SSR. Server-side renders package pages for SEO. Client-side hydrates for interactivity. Calls Go API exclusively — never hits PyPI directly.
- **Caddy**: Reverse proxy, automatic TLS, path-based routing.
- **Cloudflare**: Edge caching for HTML pages and API responses, DDoS protection.

### Deployment

- **Target:** DigitalOcean droplet, Docker Compose
- **Domain:** pypx.app or pypx.io (TBD)
- **Services:** `caddy`, `api`, `web` + shared volume for SQLite

## Go API Design

### Responsibilities

1. **PyPI Proxy & Cache** — Fetch package metadata from `https://pypi.org/pypi/{name}/json`, cache in SQLite with TTL. Stale-while-revalidate pattern — serve cached data immediately, refresh in background.

2. **Data Enrichment** — Parse wheel/sdist metadata to extract:
   - Install size (download wheel, read RECORD for file sizes)
   - Module format indicators (py3-none-any, platform-specific wheels, etc.)
   - Python version compatibility (from `requires_python`)
   - Dependency tree (parse `requires_dist`, resolve extras, flag outdated/deprecated)

3. **Download Stats** — Proxy pypistats.org, cache aggressively. Shape data for trend charts.

### Endpoints

| Endpoint | Purpose |
|----------|---------|
| `GET /api/packages/{name}` | Full enriched package data (metadata + sizes + deps + stats) |
| `GET /api/packages/{name}/versions` | Version history with sizes |
| `GET /api/packages/{name}/dependencies` | Dependency tree with health indicators |
| `GET /api/packages/{name}/stats` | Download trends (daily/weekly/monthly) |
| `GET /api/search?q=...` | Search packages |

### Tech Choices

- Standard library `net/http` + chi router
- SQLite via `modernc.org/sqlite` (pure Go, no CGo)
- Background workers for cache warming and precomputation

## Nuxt Frontend Design

### Routes

| Route | Purpose |
|-------|---------|
| `/` | Landing page with prominent search bar, trending packages |
| `/packages/:name` | Main package page (tabbed: Overview, Dependencies, Versions, Stats) |
| `/packages/:name/:version` | Specific version view |
| `/search?q=...` | Search results with filters |

### Package Page Tabs (MVP)

- **Overview** — README rendering, install command (pip/uv/poetry/pipx toggle with copy button), metadata badges (Python versions, license, module format), maintainers, project links
- **Dependencies** — Tree view of `requires_dist`, extras breakdown, outdated/deprecated warnings
- **Versions** — Version history table with release dates, wheel sizes, Python compatibility per version
- **Stats** — Download trend charts (daily/weekly/monthly), breakdown by Python version and OS

### Key UI Features (MVP)

- **Dark/light mode** — system preference default, manual toggle, persisted in localStorage
- **Fuzzy search** — instant typeahead in header (debounced 150ms), `Cmd+K` / `Ctrl+K` command palette overlay, top 5-8 results with name, description, weekly downloads
- **Full search results page** — `/search?q=...` with filters (Python version, license, last updated, min downloads), ranked by relevance + popularity
- **Keyboard navigation** — command palette, arrow keys in results, tab between sections
- **Install command switcher** — toggle between pip/uv/poetry/pipx with copy button
- **Responsive** — mobile-first, works well on phones and tablets

### Tech Choices

- Nuxt 4 with SSR mode
- Nuxt UI (built on Radix Vue) for component primitives
- Tailwind 4 for styling
- VueUse for composables (keyboard shortcuts, localStorage, clipboard)

### Visual Design

**Terminal Hybrid** aesthetic:
- **Typography:** Geist Sans for headings and body text, Geist Mono for data, code elements, and badges
- **Color palette:** Zinc scale (dark-first). Near-black backgrounds (#09090b), zinc borders (#27272a), white text (#fafafa), muted secondary text (#a1a1aa)
- **Accent colors:** Green (#4ade80) for positive metrics/sizes, amber (#f59e0b) for warnings/dep counts, indigo (#818cf8) for version/Python info
- **Cards:** Rounded corners (8px), zinc borders, monospace data presentation inside clean card layouts
- **Overall feel:** Modern SaaS with terminal DNA — clean sans-serif readability with monospace precision for data. Dark mode is the primary design target.

## Data Flow & Caching

### Request Lifecycle (Package Page)

1. Browser hits Cloudflare → cache HIT returns instantly (pages cached 1 hour at edge)
2. Cache MISS → Caddy routes to Nuxt SSR
3. Nuxt server-side calls Go API
4. Go checks SQLite cache:
   - **Fresh** (< TTL): return immediately
   - **Stale**: return cached data, queue background refresh
   - **Missing**: fetch from PyPI + pypistats.org, enrich, cache, return
5. Nuxt renders full HTML, returns to browser
6. Client hydrates, subsequent navigation is SPA-style

### Cache Layers (Outer to Inner)

1. **Cloudflare** — edge cache (Cache-Control headers from Caddy/Go)
2. **Go SQLite** — application cache for enriched package data
3. **Go in-memory** — hot packages (top 1000) for sub-ms response

### Cache TTLs

| Data | TTL |
|------|-----|
| Package metadata | 1 hour |
| Download stats | 24 hours |
| Search index | 6 hours |
| Wheel metadata / install sizes | 7 days |

### Background Workers

- **Cache warmer:** pre-fetch top 1000 packages on startup
- **Stale refresher:** re-fetch expired entries in background
- **Stats aggregator:** pull pypistats.org data nightly

## Search

### MVP

Build a local search index from the start using SQLite FTS5. Ingest PyPI Simple API package names + cached metadata. This avoids reliance on PyPI's XMLRPC search (which has been unreliable/deprecated). Results augmented with enriched metadata (sizes, dep counts, downloads).

### Enhancement

Improve search ranking with download-weighted scoring, typo tolerance, and prefix matching as the metadata cache grows.

### Search UX

- Header search bar on every page, `Cmd+K` / `Ctrl+K` opens command palette overlay
- Instant typeahead (debounced 150ms), top 5-8 results with name, description snippet, weekly downloads
- Full results page with filters: Python version, license, last updated, min downloads
- Ranked by relevance + popularity (download-weighted)

## Download Statistics Strategy

**MVP:** pypistats.org API — free, pre-aggregated daily/weekly/monthly counts per package. Provides breakdowns by Python version and OS.

**Future:** Google BigQuery (`bigquery-public-data.pypi.file_downloads`) for custom trend analysis, comparative charts, and deeper analytics. Adds cost (~$5/TB scanned) but full control over data.

## Post-MVP Roadmap

In rough priority order:

1. **Code browser** — Browse wheel/sdist file contents in-browser with linkable file paths
2. **Command palette enhancements** — Navigate to versions, jump to deps, quick compare
3. **Version comparison** — Diff changelogs, dep changes, size changes between versions
4. **Security & provenance** — Surface PyPI attestations, known vulnerabilities (OSV database), Sigstore signatures
5. **BigQuery integration** — Custom download trend analysis, comparative charts
6. **(Maybe) Social features** — Package claiming, maintainer profiles, "similar packages" recommendations
7. **(Maybe) User accounts** — Saved/starred packages, preferences sync, release notifications

## Data Sources

| Source | What it provides | Auth required | Rate limits |
|--------|-----------------|---------------|-------------|
| PyPI JSON API (`pypi.org/pypi/{name}/json`) | Package metadata, releases, file URLs | None | Modest (be polite) |
| PyPI Simple API (`pypi.org/simple/`) | Full package name list | None | Minimal |
| pypistats.org API | Download stats (daily/weekly/monthly) | None | Reasonable |
| Wheel files (via PyPI CDN) | Install size, RECORD, module format | None | Standard CDN |
| Google BigQuery (future) | Raw download events | GCP project | Pay per query |
