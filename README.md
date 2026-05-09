# pypx

**The Python Package Index, reimagined.**

pypx is a fast, modern frontend for PyPI — built for developers who want more than a name and a description. Get instant search, dependency trees, download trends, vulnerability data, inline changelogs, and API docs in one place.

[![CI](https://github.com/mattstrayer/pypx/actions/workflows/ci.yml/badge.svg)](https://github.com/mattstrayer/pypx/actions/workflows/ci.yml)
[![CodeQL](https://github.com/mattstrayer/pypx/actions/workflows/codeql.yml/badge.svg)](https://github.com/mattstrayer/pypx/actions/workflows/codeql.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/mattstrayer/pypx/badge)](https://scorecard.dev/viewer/?uri=github.com/mattstrayer/pypx)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://golang.org)
[![Nuxt](https://img.shields.io/badge/Nuxt-4-00DC82?logo=nuxt.js&logoColor=white)](https://nuxt.com)
[![Discussions](https://img.shields.io/github/discussions/mattstrayer/pypx?logo=github)](https://github.com/mattstrayer/pypx/discussions)

**[pypx.app](https://pypx.app)** — inspired by [npmx.dev](https://npmx.dev)

---

## Why

PyPI.org works — but it feels dated. No instant search, no at-a-glance package insights, no dark mode, no keyboard navigation. pypx layers enriched data and modern UX on top of PyPI's public APIs without scraping or replacing anything.

---

## Features

- **Instant search** — FTS5-powered fuzzy search across 780K+ packages with typeahead and `Cmd+K` command palette
- **Package insights** — install size, wheel platform coverage, Python version compatibility, release cadence
- **Dependency tree** — parsed PEP 508 `requires_dist` with required vs. optional extras split out
- **Download trends** — 4-week, 3-month, and 6-month charts broken down by Python version and OS
- **Inline changelogs** — GitHub Releases, CHANGELOG.md files, and GitLab Releases rendered as markdown
- **Security advisories** — OSV database CVE data per package, shown on the versions timeline
- **API docs** — extracted in-process from published wheels via goopy: modules, function signatures, docstrings
- **Install command switcher** — pip, uv, poetry, and pipx commands with one-click copy
- **Dark-first terminal aesthetic** — Geist fonts, zinc palette, full keyboard navigation
- **SSR + edge caching** — fast initial loads via Nuxt SSR, Cloudflare edge caching for repeat visits

---

## Quick Start

```bash
git clone https://github.com/mattstrayer/pypx.git
cd pypx
cp .env.example .env        # add GITHUB_TOKEN for higher rate limits (optional)
docker compose up --build
```

Visit [http://localhost](http://localhost).

> On first boot the background worker syncs the full PyPI package index (~780K packages). Search results populate within a few minutes.

---

## Configuration

| Variable | Default | Description |
|---|---|---|
| `DOMAIN` | `localhost` | Domain for Caddy TLS certificate (e.g. `pypx.app`) |
| `GITHUB_TOKEN` | _(empty)_ | GitHub PAT — raises API rate limit from 60 to 5,000 req/hr |
| `GITLAB_TOKEN` | _(empty)_ | GitLab PAT for GitLab-hosted packages |
| `API_PORT` | `8080` | Go API listen port |
| `SQLITE_PATH` | `pypx.db` | SQLite database path (cache + search index live alongside it) |
| `NUXT_API_BASE` | `http://localhost:8080` | Server-side API URL used by Nuxt SSR |
| `NUXT_PUBLIC_API_BASE` | `/api` | Client-side API base (proxied through Caddy in production) |

In Docker Compose, `NUXT_API_BASE` is automatically set to the internal `http://api:8080` address. You only need to provide `DOMAIN`, `GITHUB_TOKEN`, and optionally `GITLAB_TOKEN` in your `.env`.

---

## Architecture

```
Browser → Cloudflare → Caddy → Nuxt SSR    (port 3000)
                             → Go API      (port 8080)
```

- **Go API** — chi-based HTTP server. Orchestrates PyPI, pypistats, GitHub, GitLab, OSV, and conda-forge. Two-tier cache (LRU memory + SQLite). Background worker syncs 780K+ package names into an FTS5 search index every 6 hours. API doc extraction runs in-process via goopy (pure Go wheel parser).
- **Nuxt 4** — SSR frontend in Vue 3 + Tailwind 4. Critical data fetched server-side; secondary data (changelog, security, docs) loaded in parallel client-side after render.
- **Caddy** — automatic TLS, routes `/api/*` to Go API, everything else to Nuxt.
- **Cloudflare** — edge caching, DDoS protection, bot management.

Full architecture documentation: [`docs/architecture/`](docs/architecture/)

---

## API Endpoints

| Endpoint | Cache TTL | Description |
|---|---|---|
| `GET /api/health` | — | Health check |
| `GET /api/packages/{name}` | 1 hour | Enriched package metadata |
| `GET /api/packages/{name}/versions` | 1 hour | Full version history with file lists |
| `GET /api/packages/{name}/dependencies` | 1 hour | Parsed dependency tree with extras |
| `GET /api/packages/{name}/stats?period=4w\|3m\|6m` | 24 hours | Download trends and breakdowns |
| `GET /api/packages/{name}/changelog` | 7 days | Changelog from GitHub/GitLab/file sources |
| `GET /api/packages/{name}/security` | 24 hours | OSV vulnerability advisories |
| `GET /api/packages/{name}/extras` | 24 hours | Type annotation support, conda-forge availability |
| `GET /api/packages/{name}/docs` | Indefinite | API docs extracted in-process from wheel via goopy |
| `GET /api/search?q=...&limit=20` | 5 minutes | FTS5 full-text package search |
| `GET /api/popular` | 1 hour | Top packages by 30-day downloads |

### Agentic plain-text endpoints

| Endpoint | Cache TTL | Content-Type | Description |
|---|---|---|---|
| `GET /llms.txt` | 1 hour | `text/plain; charset=utf-8` | Discovery index of available `.txt` routes |
| `GET /api/packages/{name}.txt` | 1 hour | `text/plain; charset=utf-8` | Package metadata (key:value pairs + dependencies) |
| `GET /api/packages/{name}/changelog.txt` | 7 days | `text/plain; charset=utf-8` | Markdown changelog (one section per version) |
| `GET /api/packages/{name}/security.txt` | 24 hours | `text/plain; charset=utf-8` | Vulnerability list (supports `?version=`) |
| `GET /api/packages/{name}/extras.txt` | 24 hours | `text/plain; charset=utf-8` | Type support, conda-forge availability, repo info |
| `GET /api/packages/{name}/summary.txt` | 1 hour | `text/plain; charset=utf-8` | Agent briefing (≤2KB) |
| `GET /api/search.txt?q=...&limit=` | 5 minutes | `text/plain; charset=utf-8` | TSV search results (name, downloads, summary) |
| `GET /api/packages/{name}/docs.txt?prefix=` | Indefinite | `text/plain; charset=utf-8` | API documentation; supports `?prefix=` filter |
| `GET /api/packages/{name}/docs/{symbol}.txt` | Indefinite | `text/plain; charset=utf-8` | Single symbol by dotted path |
| `GET /api/packages/{name}/symbols.txt?q=&kind=&limit=` | Indefinite | `text/plain; charset=utf-8` | TSV symbol search with filters |
| `GET /api/packages/{name}/diff.txt?from=&to=` | Indefinite | `text/plain; charset=utf-8` | Diff between two versions |
| `GET /api/compare.txt?pkgs=` | 1 hour | `text/plain; charset=utf-8` | Side-by-side compare of up to 5 packages |

---

## Development

**Prerequisites:** Go 1.26+, Node.js 20+ with pnpm, Docker + Compose

```bash
# Go API
cd api && go run ./cmd/server

# Nuxt frontend (separate terminal)
cd web && pnpm install && pnpm run dev

# Run tests
cd api && go test ./...
cd web && pnpm run test
```

The Nuxt dev server proxies `/api/*` to `localhost:8080` automatically (via `docker-compose.override.yml` which configures local development proxying).

---

## Tech Stack

| Layer | Technology |
|---|---|
| API | Go 1.26, chi v5, goldmark, modernc.org/sqlite |
| Frontend | Nuxt 4, Vue 3, Tailwind 4, VueUse, @nuxtjs/seo |
| Search | SQLite FTS5 (porter + unicode61 tokenizer) |
| Cache | SQLite (persistent TTL) + in-memory LRU (1,000 entries) |
| Doc extraction | Go, goopy (in-process wheel parser) |
| Proxy | Caddy 2 (automatic TLS) |
| Deploy | Docker Compose, DigitalOcean Droplet, Cloudflare |

---

## Contributing

Pull requests, bug reports, and feature ideas are all welcome. Start with [`CONTRIBUTING.md`](CONTRIBUTING.md) for setup and conventions, and please read the [Code of Conduct](CODE_OF_CONDUCT.md) before participating.

- File bugs or feature requests via [GitHub Issues](https://github.com/mattstrayer/pypx/issues)
- Commit messages and PR titles follow [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `refactor:`, `docs:`, `chore:`, `test:`
- One logical change per PR; run `go test ./...` and `pnpm run test` before pushing

To report a security vulnerability, follow the disclosure process in [`SECURITY.md`](SECURITY.md) — please do **not** open a public issue.

---

## Acknowledgements

pypx stands on a lot of public infrastructure. Huge thanks to the maintainers of:

- **[PyPI](https://pypi.org/)** and the [Python Packaging Authority](https://www.pypa.io/) — package metadata and the `simple` index
- **[pypistats.org](https://pypistats.org/)** — aggregated download statistics
- **[OSV.dev](https://osv.dev/)** — open-source vulnerability database
- **[conda-forge](https://conda-forge.org/)** — community-maintained Conda channel
- **[hugovk/top-pypi-packages](https://github.com/hugovk/top-pypi-packages)** — Hugo van Kemenade's top-PyPI-packages dataset
- **[GitHub](https://github.com/)** and **[GitLab](https://gitlab.com/)** — release notes, READMEs, and repository metadata
- **[Geist](https://vercel.com/font)** — typography
- Inspiration from **[npmx.dev](https://npmx.dev)**

pypx is not affiliated with the Python Software Foundation, the Python Packaging Authority, or any of the services it consumes.

---

## License

[MIT](LICENSE)
