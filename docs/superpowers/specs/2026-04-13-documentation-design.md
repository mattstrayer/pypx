---
title: pypx Documentation & CLAUDE.md Design
date: 2026-04-13
status: approved
---

# Documentation Design Spec

## Goal

Create a comprehensive documentation suite for pypx covering: project CLAUDE.md (AI context), a polished GitHub README, and a `docs/architecture/` directory with full Mermaid diagrams covering every major subsystem.

## Audience

- **CLAUDE.md**: AI coding assistants — dense, structured, file-referenced, no prose padding
- **README.md**: Developers evaluating or onboarding to pypx — polished, scannable, GitHub-rendered
- **docs/architecture/**: Contributing developers and future maintainers — rich narrative with diagrams

## File Set

| File | Purpose |
|---|---|
| `README.md` | Hero, features, quick-start, config, architecture summary, links |
| `CLAUDE.md` | AI context: stack, key files, patterns, commands, gotchas |
| `docs/architecture/index.md` | Map of all architecture docs |
| `docs/architecture/system-overview.md` | High-level diagram, service roles, full request flow |
| `docs/architecture/api.md` | Go API handlers, routing, response shapes |
| `docs/architecture/caching.md` | Two-tier cache, TTLs, stale-while-revalidate, key naming |
| `docs/architecture/data-sources.md` | All external APIs, purpose, rate limits |
| `docs/architecture/search.md` | FTS5 index, background worker, sync cadence |
| `docs/architecture/changelog.md` | Source registry pattern, parallel fetch, priority |
| `docs/architecture/frontend.md` | Nuxt SSR strategy, composables, data fetching split |
| `docs/architecture/docs-worker.md` | Python sidecar, griffe, wheel extraction flow |
| `docs/architecture/deployment.md` | Docker Compose, Caddy, Cloudflare, env vars |

## README Structure

1. Title + tagline + badges
2. Features list
3. Screenshot placeholder
4. Quick Start (3 commands)
5. Configuration (env var table)
6. Architecture overview (1 paragraph + link to docs)
7. Contributing (commit conventions)
8. License

## CLAUDE.md Content Areas

- Stack summary (Go 1.26, chi, SQLite/FTS5, Nuxt 4, Vue 3, Tailwind 4, Python sidecar)
- Directory map (api/, web/, docs-worker/, docs/)
- Key files (12-15 entries, one-line purpose each)
- Development commands (run API, run web, run full stack)
- Patterns to follow (cache keys, handler structure, source interface, enrichment functions)
- SQL gotcha (use `?` not `$1`)
- GitHub/GitLab token behavior
- Cache TTL reference table

## Diagram Plan

Each architecture file gets Mermaid diagrams:

- **system-overview**: `graph LR` top-level flow: Browser → Cloudflare → Caddy → (Nuxt | Go API → docs-worker)
- **api**: `graph TD` handler routing tree; sequence diagram for package request
- **caching**: `graph TD` memory-hit → SQLite-hit → live-fetch → background-revalidate
- **data-sources**: `graph LR` API → [PyPI, pypistats, GitHub, GitLab, OSV, conda, hugovk]
- **search**: sequence diagram: worker sync loop → PyPI Simple API → FTS5 index
- **changelog**: `graph TD` registry → parallel sources → priority selection → rendered HTML
- **frontend**: sequence diagram: SSR blocking fetch vs client-side parallel fetches
- **docs-worker**: `graph TD` wheel download → griffe parse → structured JSON → cache
- **deployment**: `graph LR` Docker Compose service graph with ports and volumes

## Constraints

- All Mermaid diagrams must be GitHub-renderable (no unsupported node types)
- File paths in CLAUDE.md use repo-relative paths
- No fictional features — only what exists in code as of 2026-04-13
- Conventional commits for the doc commit (`docs: add architecture documentation and README`)
