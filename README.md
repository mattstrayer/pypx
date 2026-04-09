# pypx

A modern frontend for the [Python Package Index](https://pypi.org), inspired by [npmx.dev](https://npmx.dev).

**[pypx.app](https://pypx.app)**

## Why

PyPI.org works but feels dated — no instant search, no at-a-glance package insights, no dark mode, no keyboard navigation. pypx fixes that by layering enriched data and modern UX on top of PyPI's public APIs.

## Features

- **Fast fuzzy search** with instant typeahead and `Cmd+K` command palette
- **Package insights** — install size, dependency tree, module format, Python version compatibility
- **Download trends** — daily/weekly/monthly stats with Python version and OS breakdowns
- **Changelogs** — GitHub Releases rendered inline on the versions timeline
- **Install command switcher** — pip, uv, poetry, pipx with one-click copy
- **Dark-first design** — terminal hybrid aesthetic with Geist fonts and zinc palette
- **Keyboard navigation** — command palette, arrow keys, tab between sections
- **SSR** — server-side rendered for fast loads and SEO

## Architecture

```
Browser → Cloudflare → Caddy → Nuxt SSR (port 3000)
                             → Go API  (port 8080)
```

- **Go API** — caching proxy + data enrichment layer. Fetches from PyPI JSON API, pypistats.org, and GitHub Releases. SQLite cache with in-memory LRU. Background worker indexes 780K+ packages for search via FTS5.
- **Nuxt 4** — SSR frontend with Nuxt UI, Tailwind 4, and `@nuxtjs/mdc` for markdown rendering.
- **Caddy** — reverse proxy with automatic TLS.
- **Cloudflare** — edge caching, DDoS protection, bot management.

## Quick Start

### Development

```bash
# API
cd api && go run ./cmd/server/

# Frontend (separate terminal)
cd web && pnpm install && pnpm dev
```

The Nuxt dev server proxies `/api/*` to `localhost:8080` automatically.

### Production

```bash
cp .env.example .env
# Edit .env: set DOMAIN and optionally GITHUB_TOKEN

docker compose up -d --build
```

See [docs/cloudflare-setup.md](docs/cloudflare-setup.md) for CDN configuration.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DOMAIN` | `localhost` | Domain for Caddy TLS |
| `GITHUB_TOKEN` | — | Optional GitHub API token for higher rate limits (60/hr → 5000/hr) |
| `API_PORT` | `8080` | Go API listen port |
| `SQLITE_PATH` | `pypx.db` | SQLite database path |

## Tech Stack

| Component | Technology |
|-----------|-----------|
| API | Go 1.26, chi, modernc.org/sqlite |
| Frontend | Nuxt 4, Vue 3, Nuxt UI, Tailwind 4, VueUse |
| Markdown | @nuxtjs/mdc |
| SEO | @nuxtjs/seo |
| Search | SQLite FTS5 |
| Cache | SQLite + in-memory LRU |
| Proxy | Caddy 2 |
| Deploy | Docker Compose, DigitalOcean, Cloudflare |

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /api/health` | Health check |
| `GET /api/packages/{name}` | Enriched package metadata |
| `GET /api/packages/{name}/versions` | Version history with sizes |
| `GET /api/packages/{name}/dependencies` | Dependency tree with extras |
| `GET /api/packages/{name}/stats` | Download trends and breakdowns |
| `GET /api/packages/{name}/changelog` | GitHub Releases changelog |
| `GET /api/search?q=...` | Full-text package search |

## License

MIT
