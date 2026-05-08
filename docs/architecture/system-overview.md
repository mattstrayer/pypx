# System Overview

pypx is four cooperating services behind a Caddy reverse proxy, with Cloudflare sitting at the edge.

## Service Map

```mermaid
graph LR
    Browser -->|HTTPS| CF[Cloudflare Edge]
    CF -->|Cache miss| Caddy

    subgraph VPS ["DigitalOcean Droplet"]
        Caddy -->|"/api/*"| API["Go API\n:8080\n(+ goopy in-process)"]
        Caddy -->|"/*"| Nuxt["Nuxt SSR\n:3000"]
        API -->|SQLite files| DB[("SQLite\npypx.db\npypx.db-search")]
    end

    API -->|HTTP| PyPI["pypi.org"]
    API -->|HTTP| Stats["pypistats.org"]
    API -->|HTTP| GH["api.github.com"]
    API -->|HTTP| GL["gitlab.com/api/v4"]
    API -->|HTTP| OSV["api.osv.dev"]
    API -->|HTTP| Conda["conda.anaconda.org"]
    API -->|HTTP| Hugo["hugovk.dev"]
```

## Service Roles

### Caddy
The public entry point. Handles TLS termination (automatic certificates from Let's Encrypt). Routes `/api/*` requests to the Go API and all other requests to Nuxt. Adds security headers (`X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`). Trusts Cloudflare proxy IPs for real client IP forwarding.

### Go API (`:8080`)
The data layer. It:
- Fetches from multiple external APIs (PyPI, pypistats, GitHub, GitLab, OSV, conda-forge)
- Enriches raw data (dependency parsing, wheel analysis, Python version extraction, release cadence)
- Caches all results in a two-tier store (memory LRU + SQLite)
- Maintains a full-text search index of 780K+ PyPI packages via a background worker
- Serves JSON to both the Nuxt SSR server and the browser
- Serves plain-text variants for agentic consumers (`/llms.txt`, `*.txt` routes)

### Nuxt SSR (`:3000`)
The presentation layer. It:
- Renders package pages server-side for fast initial loads and SEO
- Fetches critical data (package metadata) blocking on the server
- Hydrates in the browser and fires parallel non-blocking requests for secondary data (changelog, security, docs, extras)
- Serves a Vue 3 SPA after hydration

### goopy (in-process)
API doc extraction runs in-process within the Go API using the goopy library. Given a package name and version, goopy downloads the wheel from PyPI and parses Python source with a recursive-descent parser to extract modules, classes, functions, docstrings, and type annotations, returning structured JSON. Results are cached indefinitely per version.

### SQLite
Two separate database files:
- `pypx.db` — the TTL cache store (all API response caching)
- `pypx.db-search` — the FTS5 full-text search index (package names, summaries, download counts)

Both run with WAL mode enabled for concurrent read performance.

## Full Request Flow

### Search request

```mermaid
sequenceDiagram
    participant B as Browser
    participant CF as Cloudflare
    participant C as Caddy
    participant API as Go API
    participant FTS as SQLite FTS5

    B->>CF: GET /api/search?q=requests
    CF->>C: Cache miss — forward request
    C->>API: GET /api/search?q=requests
    API->>FTS: Search("requests", limit=20)
    FTS-->>API: Ranked results (name, summary, downloads)
    API-->>C: JSON response + Cache-Control: 5min
    C-->>CF: Response
    CF-->>B: Response (cached for next 5min)
```

### Package page request

```mermaid
sequenceDiagram
    participant B as Browser
    participant CF as Cloudflare
    participant C as Caddy
    participant Nuxt as Nuxt SSR
    participant API as Go API
    participant Cache as SQLite Cache
    participant PyPI as pypi.org

    B->>CF: GET /packages/requests
    CF->>C: Forward (no cache for HTML)
    C->>Nuxt: GET /packages/requests
    Nuxt->>API: GET /api/packages/requests (server-side)
    API->>Cache: lookup "pkg:requests"
    alt Cache hit (fresh)
        Cache-->>API: Cached JSON
    else Cache miss or stale
        API->>PyPI: GET /pypi/requests/json
        PyPI-->>API: Raw package data
        API->>API: Enrich (deps, wheels, platforms, cadence)
        API->>Cache: Store "pkg:requests" (TTL: 1hr)
        Note over API: If stale: return stale immediately,<br/>refresh in background goroutine
    end
    API-->>Nuxt: Enriched JSON
    Nuxt-->>B: Server-rendered HTML

    Note over B: Hydration complete — parallel client-side requests fire
    par
        B->>API: GET /api/packages/requests/changelog
    and
        B->>API: GET /api/packages/requests/security
    and
        B->>API: GET /api/packages/requests/extras
    end
    API-->>B: Changelog HTML (cached 7d)
    API-->>B: CVE data (cached 24h)
    API-->>B: Extras data (cached 24h)
```

## Memory Limits (Docker Compose)

| Service | Memory limit |
|---|---|
| Caddy | 128 MB |
| Go API | 512 MB |
| Nuxt SSR | 256 MB |
