# pypx — CLAUDE.md

## What this project is

pypx is a modern frontend for PyPI (Python Package Index). It enriches PyPI's public data with search, dependency analysis, download stats, changelogs, security advisories, and API docs. Live at **https://pypx.app**.

## Stack

| Layer | Technology |
|---|---|
| API | Go 1.26, chi v5, goldmark (markdown→HTML), modernc.org/sqlite |
| Frontend | Nuxt 4, Vue 3, Tailwind 4, VueUse, @nuxtjs/seo |
| Search | SQLite FTS5 (porter + unicode61 tokenizer) |
| Cache | Two-tier: in-memory LRU (1,000 entries) + SQLite persistent TTL store |
| Doc extraction | Go, goopy (pure Go in-process wheel parser) |
| Proxy | Caddy 2 |
| Deploy | Docker Compose (Caddy + Go API + Nuxt) |

## Directory Map

```
api/                    Go API service
  cmd/server/main.go    Entry point — router, service wiring, graceful shutdown
  internal/
    cache/              Two-tier cache (memory.go, sqlite.go)
    handler/            HTTP handlers (one file per route group)
    pypi/               PyPI JSON API client
    stats/              pypistats.org client
    github/             GitHub API client + source implementations
    gitlab/             GitLab API client + source implementations
    osv/                OSV (vulnerability) client
    conda/              conda-forge availability client
    enrichment/         Pure functions: dependencies, wheels, platforms, cadence, maintainers
    changelog/          Parallel source registry + parser + renderer
    search/             FTS5 index (index.go)
    worker/             Background sync worker (background.go)
    textfmt/            Plain-text formatters for agent .txt endpoints (one file per resource)

web/                    Nuxt 4 frontend
  app/
    pages/              Route components ([name].vue, search.vue, index.vue)
    components/         Reusable UI components
    composables/        useApi.ts, useSearchTypeahead.ts, usePackageManager.ts
    types/api.ts        TypeScript interfaces matching Go response shapes
  nuxt.config.ts        Build config, runtime config, SEO config

goopy/                  Pure Go Python doc extractor (in-process)
  internal/             Parser, extractor, wheel downloader, ecosystem tests

docs/architecture/      Architecture documentation (Mermaid diagrams)
Caddyfile               Reverse proxy routing + security headers
docker-compose.yml      Full stack definition
```

## Key Files

| File | Purpose |
|---|---|
| `api/cmd/server/main.go` | Service wiring, router setup, env var defaults |
| `api/internal/cache/sqlite.go` | Persistent TTL cache with stale-while-revalidate |
| `api/internal/cache/memory.go` | LRU memory layer wrapping SQLite cache |
| `api/internal/handler/packages.go` | Core package endpoint — enrichment orchestration |
| `api/internal/handler/changelog.go` | Changelog endpoint — delegates to registry |
| `api/internal/handler/llms.go` | Discovery endpoint for plain-text routes |
| `api/internal/textfmt/` | Pure formatter functions emitting plain text for /api/.../*.txt routes |
| `api/internal/changelog/registry.go` | Parallel source fetch with priority selection |
| `api/internal/search/index.go` | FTS5 index: Search(), TopByDownloads(), UpsertBatch() |
| `api/internal/worker/background.go` | Index sync: SyncIndex() + SyncDownloads(), runs every 6h |
| `api/internal/enrichment/dependencies.go` | PEP 508 `requires_dist` parser |
| `api/internal/enrichment/wheels.go` | Install size + platform coverage from distribution files |
| `web/app/composables/useApi.ts` | API wrapper — handles server vs. client base URL |
| `web/app/pages/packages/[name].vue` | Main package page — SSR + client-side parallel loads |
| `web/app/types/api.ts` | TypeScript type shim — re-exports from `api.gen.ts`; run `cd api && go run ./cmd/gentypes -out ../web/app/types/api.gen.ts` after changing a response struct |
| `web/app/types/api.gen.ts` | Generated TypeScript interfaces (DO NOT EDIT — commit after regenerating) |
| `api/cmd/gentypes/main.go` | Generator — reflects on Go response structs to emit `api.gen.ts` |
| `web/nuxt.config.ts` | Runtime config: `apiBase` (server), `public.apiBase` (client); `fonts` (self-hosted Geist) |
| `scripts/check-fonts-bundled.mjs` | Post-build guard: brand fonts must ship self-hosted, never from a third-party host. Run via `make check-fonts` after a web build; CI runs it in `web-build`. |

## Development Commands

The root `Makefile` is the source of truth for dev/test/lint/gentypes commands — run `make help` for the full list.

```bash
make dev-api      # Run Go API on :8080
make dev-web      # Run Nuxt dev server on :3000 (proxies /api/* to :8080)
make test         # Run all test suites (api, goopy, web)
make test-api     # Go API tests
make test-web     # Web tests (vitest)
make test-goopy   # goopy tests (-short skips the scheduled ecosystem suite)
make lint         # go vet (api, goopy) + oxlint (web)
make typecheck    # Web typecheck (nuxi typecheck)
make gentypes     # Regenerate web/app/types/api.gen.ts from Go response structs

# Full stack
docker compose up --build     # Production-like stack
```

## Patterns to Follow

### Cache keys
```
raw:{name}              Raw PyPI JSON response
pkg:{name}              Enriched package response
stats:{name}:{period}   Download stats (period: 4w, 3m, 6m)
changelog:{name}        Rendered changelog HTML
docs:{name}:{version}   API docs (indefinite TTL)
docs-err:{name}:{version}  Docs extraction error (5 min TTL)
```

### Handler structure
Every handler follows: check cache → fetch live data → enrich → cache → return JSON. For stale-while-revalidate: return stale immediately, trigger background goroutine to refresh.

### Changelog source interface
```go
type Source interface {
    Fetch(ctx context.Context) ([]Entry, error)
    Priority() int
}
```
Registry fires all sources concurrently, picks first non-empty result by ascending priority number.

### Enrichment functions
All enrichment is pure functions in `api/internal/enrichment/`. Input: raw PyPI data. Output: computed structs. No side effects, no I/O.

### Frontend data loading
- **SSR (blocking):** Package metadata (`/api/packages/{name}`) — fetched server-side via `useAsyncData`
- **Client-side (parallel, non-blocking):** Changelog, security, extras, docs — loaded after hydration with `server: false`

## SQL Gotcha

Always use `?` placeholders, not `$1`/`$2`. modernc.org/sqlite uses `?` binding syntax.

```go
// Correct
db.QueryRow("SELECT value FROM cache WHERE key = ?", key)

// Wrong — will break
db.QueryRow("SELECT value FROM cache WHERE key = $1", key)
```

## Cache TTLs

| Endpoint | TTL | Notes |
|---|---|---|
| Package metadata | 1 hour | Stale-while-revalidate |
| Stats | 24 hours | Stale-while-revalidate |
| Changelog | 7 days | Fresh only |
| Security | 24 hours | Stale-while-revalidate |
| Extras | 24 hours | Stale-while-revalidate |
| Docs | Indefinite | Keyed by version — never stale |
| Docs errors | 5 minutes | Short TTL to allow retry |
| Search | 5 min (HTTP header) | Not in SQLite cache |

## GitHub/GitLab Tokens

Both are optional. Without `GITHUB_TOKEN`, GitHub API is rate-limited to 60 req/hr (per IP). With token: 5,000 req/hr. Set in `.env` for local dev or as Docker env vars in production. GitLab token similarly optional — only needed for packages hosted on GitLab.

## goopy (Python Doc Extraction)

The Go API extracts Python API documentation in-process using the `goopy` library (`goopy/` directory). goopy downloads wheels from PyPI, parses Python source with a recursive-descent parser, and extracts structured documentation (functions, classes, parameters, docstrings, type annotations).

The `/api/packages/{name}/docs` route has a **60-second timeout** (vs. 30s for all other routes) to accommodate wheel download + goopy parse time.

## External Services

| Service | Used for | Auth |
|---|---|---|
| `pypi.org/pypi/{name}/json` | Package metadata | None |
| `pypi.org/simple/?format=application/vnd.pypi.simple.v1+json` | All package names (780K+) | None |
| `pypistats.org/api/packages/{name}/...` | Download stats | None |
| `api.github.com` | Releases, tags, raw files, repo info | Optional token |
| `gitlab.com/api/v4` | GitLab releases, files | Optional token |
| `api.osv.dev` | CVE/vulnerability data | None |
| `conda.anaconda.org` | conda-forge availability | None |
| `hugovk.dev/top-pypi-packages/...` | Top 30-day download counts | None |
