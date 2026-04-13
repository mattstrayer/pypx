# pypx Architecture

This directory documents how pypx works under the hood. Each file covers one subsystem with diagrams and enough prose to understand the design decisions.

## Documents

| File | What it covers |
|---|---|
| [system-overview.md](system-overview.md) | Top-level architecture, service roles, full request flow from browser to response |
| [api.md](api.md) | Go API — routing, handlers, enrichment orchestration, response shapes |
| [caching.md](caching.md) | Two-tier cache design, TTL strategy, stale-while-revalidate pattern |
| [data-sources.md](data-sources.md) | All external APIs and services pypx calls, what each provides, rate limits |
| [search.md](search.md) | FTS5 search index, background sync worker, query sanitization |
| [changelog.md](changelog.md) | Parallel source registry, priority selection, markdown rendering |
| [frontend.md](frontend.md) | Nuxt SSR strategy, composables, SSR vs. client-side data split |
| [docs-worker.md](docs-worker.md) | Python sidecar, wheel download, griffe API doc extraction |
| [deployment.md](deployment.md) | Docker Compose services, Caddy routing, Cloudflare edge, env vars |

## Quick mental model

```
User types "requests" → FTS5 search (SQLite) → results from 780K-package index
User visits /packages/requests →
  Nuxt SSR fetches Go API → Go API checks two-tier cache →
    cache miss: fetch PyPI + enrich (deps, wheels, Python versions, cadence) → cache 1hr
    cache hit: return immediately, refresh in background if stale
  Page renders server-side HTML
  Browser hydrates → parallel client-side fetches:
    changelog (GitHub Releases / CHANGELOG.md / GitLab)
    security (OSV CVEs)
    extras (type stubs, conda-forge)
    docs (griffe wheel extraction, cached indefinitely per version)
```
