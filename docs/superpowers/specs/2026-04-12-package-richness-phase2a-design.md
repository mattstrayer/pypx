# Package Richness Phase 2a Design

## Overview

Phase 2a adds three features to pypx: auto-generated API docs (rendered from Python source via a griffe sidecar), RST README rendering, and py.typed wheel inspection to complete the type support feature from Phase 1.

**In scope:**
1. Rendered API docs — new `/packages/{name}/docs` route, Python sidecar (griffe), fixed sidebar layout
2. RST README rendering — Go RST renderer, same pattern as existing markdown renderer
3. py.typed detection — HTTP Range request on wheel zip to check for `py.typed` marker, updates extras type_support status

**Out of scope / cataloged for future:**
- Version switching on the docs page (start with latest only; add `?version=X.Y.Z` in a follow-up)
- Go/Rust native doc extraction: revisit when Astral/uv's Rust-based Python tooling matures enough to provide a stable API surface for AST introspection. This could eliminate the Python sidecar entirely.

---

## Feature 1: Rendered API Docs

### Architecture

A Python docs sidecar (`docs-worker/`) runs as a 4th container in docker compose alongside caddy/api/web. It exposes a single endpoint:

```
POST /generate
Body: {"name": "requests", "version": "2.33.1"}
Returns: griffe JSON object (modules, classes, functions, docstrings)
```

The Go API handler (`api/internal/handler/docs.go`) follows the standard cache-then-fetch pattern:
1. Check SQLite for `docs:{name}:{version}` — return immediately if found
2. On miss: call sidecar via HTTP, receive griffe JSON, store in SQLite with **no TTL** (source is immutable per version), return to client

The sidecar:
- Downloads only `.py` files from the wheel zip (central directory inspection + selective extraction — avoids downloading binary `.so` / `.pyd` files)
- Runs `griffe.load()` on the extracted source
- Returns the serialized griffe object as JSON
- For binary-only packages (no `.py` files found): returns `{"empty": true, "reason": "no_python_source"}`

### Sidecar implementation

```
docs-worker/
  main.py          # FastAPI app (~50 lines)
  requirements.txt # griffe, fastapi, uvicorn, httpx
  Dockerfile
```

The sidecar is called lazily on first request. Popular packages will warm the cache naturally; no pre-generation needed.

### New API endpoint

```
GET /api/packages/{name}/docs
```

Response shape (abbreviated):
```json
{
  "package": "requests",
  "version": "2.33.1",
  "available": true,
  "modules": [
    {
      "name": "requests",
      "functions": [
        {
          "name": "get",
          "kind": "function",
          "signature": "get(url: str, **kwargs) -> Response",
          "docstring": "Sends a GET request...",
          "parameters": [
            {"name": "url", "type": "str", "description": "URL for the request"},
            {"name": "**kwargs", "type": null, "description": "Optional arguments passed to request()"}
          ],
          "returns": {"type": "Response", "description": ""}
        }
      ],
      "classes": [...],
      "exceptions": [...]
    }
  ]
}
```

When `available: false` (binary-only package), the Docs tab is hidden in the frontend.

### Caching

- Cache key: `docs:{name}:{version}`
- TTL: none (indefinite — source code is immutable per version)
- On sidecar error: return 502, do not cache, client can retry

### Routes

```
GET /api/packages/{name}/docs     → latest version docs
```

Version switching is **not implemented in this phase**. The handler resolves the latest version by calling `fetchPackage(name)` to get `info.Version`, then uses that as the cache key.

### Frontend

New Nuxt page: `web/app/pages/packages/[name]/docs.vue`

Layout:
- Fixed left sidebar (200px): symbol index grouped by kind (Functions, Classes, Exceptions), active symbol highlighted with green left border
- Scrolling main content: per-symbol sections with syntax-highlighted signature, description, parameters table, return type
- **Docs tab** added to the package page tab strip — only rendered when `available: true`
- No version switcher in this phase (future item)

Graceful states:
- Loading: spinner in main content area, sidebar shows skeleton
- `available: false`: tab hidden entirely
- Sidecar timeout / 502: "Documentation could not be generated" message with link to the package's external docs if available

### Future: Version switching

When implemented, the route becomes:
```
GET /api/packages/{name}/docs?version=X.Y.Z
```
Frontend adds a version dropdown to the docs page header. Each version is independently cached.

---

## Feature 2: RST README Rendering

### Architecture

New Go package: `api/internal/rst/`

```go
// Render converts RST input to HTML. Returns an error if the input
// cannot be parsed; callers should fall back to rendering the raw text.
func Render(input string) (string, error)
```

Identical interface to `api/internal/markdown/`. `buildPackageResponse` in `packages.go` checks `description_content_type`:
- Contains `text/x-rst` or `text/x-restructuredtext` → RST renderer
- Contains `text/markdown` → existing markdown renderer
- Otherwise → return raw description, no HTML

The RST renderer should handle the most common directives used in PyPI READMEs:
- `.. code-block:: lang` → syntax-highlighted `<pre><code>` block
- `.. note::`, `.. warning::` → styled callout divs
- `.. image::` → `<img>` tag
- `.. toctree::` → omitted (internal Sphinx directive, not meaningful on pypx)
- Inline markup: `` `code` ``, `**bold**`, `*italic*`, `` :func:`name` `` → renders name as code

**Library selection:** Evaluate `github.com/nicois/rst` and any other available Go RST parsers. If no library meets quality bar, implement a lightweight directive-aware converter that handles the common cases above rather than full RST spec compliance. The goal is "renders well for 90% of PyPI packages" not "full Sphinx compatibility".

### Detection

`buildPackageResponse` already has:
```go
if strings.Contains(info.DescriptionType, "text/markdown") {
    descHTML, _ = markdown.Render(info.Description)
}
```

Extend to:
```go
switch {
case strings.Contains(info.DescriptionType, "text/markdown"):
    descHTML, _ = markdown.Render(info.Description)
case strings.Contains(info.DescriptionType, "text/x-rst"),
     strings.Contains(info.DescriptionType, "text/x-restructuredtext"):
    descHTML, _ = rst.Render(info.Description)
}
```

---

## Feature 3: py.typed Detection

### Architecture

Extends the existing extras endpoint. When `CheckTypeSupport` returns `stubs` or `untyped`, the extras handler additionally checks for a `py.typed` marker in the package wheel. If found, upgrades the status to `typed`.

### Wheel inspection

The Go API:
1. Fetches the latest wheel URL from PyPI metadata (already available via `fetchPackage`)
2. Issues an HTTP `Range: bytes=-65536` request to read the last 64KB of the wheel (zip file) — this is enough to cover the zip central directory for most packages
3. Parses the zip central directory to get the list of all filenames
4. Checks if any filename matches `*.dist-info/py.typed` or is exactly `py.typed`

Implementation in a new function: `pypi.CheckPyTyped(c *Client, wheelURL string) bool`

If the wheel is over **50MB** (checked via `Content-Length` header before downloading), skip the check and return `false` — the stubs/untyped result stands. This prevents unexpectedly large downloads for ML packages.

### Cache

- Cache key: `typed:{name}:{version}`
- TTL: indefinite (wheel content is immutable per version)
- On error (network failure, malformed zip): return `false`, do not cache, allow retry on next request

### Extras endpoint changes

`ExtrasResponse` gains no new fields — `TypeSupport.Status` is updated from `"stubs"` or `"untyped"` to `"typed"` when `py.typed` is found. The existing `type_support` field in the response covers this.

The check is performed in the extras handler **after** `CheckTypeSupport`, only when `status != "typed"` (i.e., stubs check didn't already resolve it). This avoids the wheel download for packages that already declare inline types via an already-known typed package.

---

## File Map

| File | Change |
|------|--------|
| `docs-worker/main.py` | New — FastAPI sidecar |
| `docs-worker/requirements.txt` | New |
| `docs-worker/Dockerfile` | New |
| `docker-compose.yml` | Add `docs-worker` service |
| `api/internal/rst/rst.go` | New — RST renderer |
| `api/internal/rst/rst_test.go` | New |
| `api/internal/pypi/typed.go` | New — py.typed wheel inspection |
| `api/internal/pypi/typed_test.go` | New |
| `api/internal/handler/docs.go` | New — docs handler |
| `api/internal/handler/docs_test.go` | New |
| `api/internal/handler/extras.go` | Extend — call CheckPyTyped |
| `api/internal/handler/packages.go` | Extend — RST rendering branch |
| `api/cmd/server/main.go` | Wire docs handler + route |
| `web/app/types/api.ts` | Add DocsData type |
| `web/app/composables/useApi.ts` | Add fetchDocs() |
| `web/app/pages/packages/[name]/docs.vue` | New — docs page |
| `web/app/pages/packages/[name].vue` | Add Docs tab (conditional) |

---

## Error Handling & Graceful Degradation

| Scenario | Behaviour |
|----------|-----------|
| Binary-only wheel (no .py files) | Docs tab hidden; `available: false` in API response |
| Sidecar unreachable | 502 from API; frontend shows error message; no caching |
| Wheel > 50MB for py.typed check | Skip check; return existing stubs/untyped status |
| Network error on wheel fetch for py.typed | Return false; no caching; retry next request |
| RST parse error | Return raw description text; no HTML rendering |
| Package has no description_content_type | Existing behaviour (return raw text) |

---

## Future Items (not in this phase)

- **Version switching on docs page:** `GET /api/packages/{name}/docs?version=X.Y.Z`, version dropdown in docs page header, independent per-version cache entries
- **Go/Rust doc extraction:** Revisit when Astral/uv's Rust-based Python tooling provides a stable API for AST introspection — could eliminate the Python sidecar entirely. Monitor `uv`'s internal module resolution and the broader Ruff/Astral ecosystem for extractable primitives.
- **Pre-warming popular packages:** Background worker pre-generates docs for the top 1000 packages on startup
- **Search within docs:** Full-text search across symbol names and docstrings for a given package
