# MCP Server over the pypx .txt Agent Surface — Design

**Epic:** Agentic (follow-up — MCP was a deliberate non-goal of the original
agentic surface, deferred "separate effort if/when adopted",
`2026-05-06-agentic-surface-design.md:13,103`)
**Plan:** `plans/022-mcp-server-design.md`
**Branch convention:** `advisor/022-mcp-server-design`
**PR convention:** Conventional commit titles (`feat(mcp): ...`, `docs(mcp): ...`).

## Goal

Expose the existing eleven pypx plain-text agent endpoints (plus the
`/llms.txt` discovery index) as MCP tools, so agents that speak MCP (Claude
Code, Cursor, etc.) can call pypx without hand-constructing URLs. All the
hard work already exists behind the `.txt` routes; an MCP server is a thin
adapter. This is a **design + one-tool spike**, not a full build.

Audience: coding agents doing package selection/evaluation (metadata, CVEs,
install size, cadence) and writing code against a package (API docs, symbol
lookup) — the same audience as the `.txt` surface, reached over a different
protocol.

Non-goals (this plan): building more than one tool; hosting a streamable-HTTP
endpoint (Caddy + docker-compose); binary distribution; mentioning MCP in
`/llms.txt` or the README. See **Deferred (next phase)**.

## Design decisions

### D1 — Transport: stdio for the spike; streamable HTTP is the production intent

**Decision:** Build the server transport-agnostic (the SDK cleanly separates
`*mcp.Server` construction from the transport passed to `server.Run`) and run
the spike over **stdio** (`&mcp.StdioTransport{}`). Record streamable HTTP
(`https://pypx.app/mcp`) as the intended production transport, deferred.

**Rationale:** stdio is the lowest-friction way to prove wiring — no proxy,
no compose changes, drivable from a shell with framed JSON-RPC. It matches
how MCP clients launch local servers today. The hosted streamable-HTTP
transport is the better long-term fit for pypx's "hosted service, zero
install" philosophy, but it requires a new Caddy `handle /mcp` block and a
docker-compose service/route, both explicitly out of scope here. Because the
SDK's `Server` is independent of transport, switching later is a `main.go`
change, not a rewrite.

### D2 — Location: a new sibling Go module `mcp/`

**Decision:** Create a new sibling module `mcp/` (module path
`github.com/pypx/mcp`), binary at `mcp/cmd/pypx-mcp`. **Not** `api/cmd/mcp`.

**Rationale:** D4 selects the HTTP-client data path, so the MCP server never
imports `api/internal/...` — the sole advantage of living inside the api
module (in-process reuse) does not apply. Given that, a separate module is
strictly cleaner: it isolates the MCP SDK's dependency tree (the SDK,
`jsonschema`, etc.) from the production API server's build, exactly as the
existing `goopy/` sibling module isolates the wheel parser. This keeps
`api/go.mod` and `api/go.sum` **unmodified**. The repo is already
multi-module (`api`, `goopy` with a `replace` directive), so a third module
is not novel. Cost: the api-module test gate (`cd api && go test ./...`)
does not cover `mcp/`; the new module is verified with the same commands run
in `mcp/`.

### D3 — SDK: `github.com/modelcontextprotocol/go-sdk` v1.6.1 (official)

**Decision:** Use the **official** MCP Go SDK,
`github.com/modelcontextprotocol/go-sdk`, pinned at **v1.6.1** (latest stable,
published 2026-05-22). Verified at execution time via pkg.go.dev and the Go
module proxy (`proxy.golang.org` returned the module zip; `go list` reports
the module's own `go 1.25.0` directive — compatible with this repo's
`go 1.26.4`).

**Rationale:** The official SDK (developed in collaboration with Google) is
the recommended choice over the earlier community `mark3labs/mcp-go`. Its
generic `mcp.AddTool[In, Out]` derives the JSON input schema from a Go struct
with `jsonschema` field tags, validates inputs automatically, and ships an
in-memory transport pair (`mcp.NewInMemoryTransports`) ideal for tests. Its
minimum Go version (1.25.0) is satisfied by our toolchain. No STOP condition
triggered: the SDK is maintained, compatible, and — being in a separate
module — adds nothing to the api server's dependency graph.

### D4 — Data path: HTTP client against the pypx API (option a)

**Decision:** Each tool issues a GET to `{PYPX_API_BASE}/api/....txt` (or
`/llms.txt`) and returns the response body verbatim as MCP text content.
`PYPX_API_BASE` comes from an env var, default `https://pypx.app`, with the
trailing slash trimmed. Package names are URL-path-escaped. A 10s per-request
`http.Client` timeout is used.

**Rationale:** Zero refactoring, and it inherits the API's caching, rate
limiting (`main.go:107` — 30 req/s, burst 60), validation, and enrichment
orchestration for free. It is trivially testable with `httptest.NewServer`.
The alternatives were rejected:

- **(b) In-process handler reuse** would require constructing the full
  handler graph that `main.go:41-93` builds inline, and there is no shared
  wiring package — so it means either duplicating ~50 lines of wiring or
  extracting wiring out of `main()`, which is an out-of-scope refactor of a
  locked file and a STOP condition. Not needed once D2 = separate module
  makes internal imports impossible anyway.
- **(c) Direct reuse of `internal/pypi` + `textfmt`** would re-implement
  handler-level caching, validation, and enrichment orchestration — a second
  code path that could drift from the real endpoints. Rejected.

The one real cost is an extra HTTP hop and dependence on a running/hosted API;
acceptable, since the whole point is to mirror the public `.txt` surface
exactly.

Note (D2↔D4 coupling): choosing the separate module (D2) forecloses (b)/(c)
and thereby *decides* D4 = HTTP. Both point the same way, so there is no
tension.

### D5 — Tool schema: one MCP tool per `.txt` endpoint

**Decision:** Map each of the eleven `.txt` endpoints to exactly one MCP tool.
Tool **output is the raw `.txt` body returned as MCP text content** — the
formats are stable, grep-friendly, and golden-file tested
(`2026-05-06-agentic-surface-design.md:112`); the MCP layer does **not**
invent a second rendering. **Error mapping:** API `404` → MCP tool error
`package not found: <name>` (or the analogous "not found" message); any other
non-`200` → tool error including the HTTP status code. Per-tool descriptions
are cribbed from the `/llms.txt` one-liners (`llms.go:20-31`).

Full inventory (source: `api/internal/handler/llms.go:20-31`). Only
`get_package` is built in this spike; the rest are **deferred**.

| Tool | Wraps | Params | Built now? |
|---|---|---|---|
| `get_package` | `/api/packages/{name}.txt` | `name` (required) | **yes** |
| `get_summary` | `/api/packages/{name}/summary.txt` | `name` | deferred |
| `search_packages` | `/api/search.txt?q=` | `query`, `limit?` | deferred |
| `get_changelog` | `/api/packages/{name}/changelog.txt` | `name` | deferred |
| `get_security` | `/api/packages/{name}/security.txt?version=` | `name`, `version?` | deferred |
| `get_extras` | `/api/packages/{name}/extras.txt` | `name` | deferred |
| `get_docs` | `/api/packages/{name}/docs.txt?prefix=` | `name`, `prefix?` | deferred |
| `get_symbol` | `/api/packages/{name}/docs/{symbol}.txt` | `name`, `symbol` | deferred |
| `search_symbols` | `/api/packages/{name}/symbols.txt?q=&kind=&limit=` | `name`, `query`, `kind?`, `limit?` | deferred |
| `diff_versions` | `/api/packages/{name}/diff.txt?from=&to=` | `name`, `from`, `to` | deferred |
| `compare_packages` | `/api/compare.txt?pkgs=` | `packages` (array, max 5) | deferred |

Notes on parameters (for the deferred tools): `version?` on security is a
query param; `prefix?` filters docs by dotted path; `search_symbols` takes
`kind?` (`function|class|method|module`) and `limit?`; `diff_versions`
requires `from` and `to`; `compare_packages` takes an array of up to 5 names
joined as `?pkgs=a,b,c`. The docs/symbols/diff endpoints run under a 60s API
timeout server-side, so their tool-side client timeout should be raised above
the default 10s when they are built.

## Architecture

```
MCP client (Claude Code / Cursor)
        │  JSON-RPC over stdio (spike)  /  streamable HTTP (prod, deferred)
        ▼
  pypx-mcp  (mcp/ module — modelcontextprotocol/go-sdk v1.6.1)
        │  mcp.AddTool(get_package)  →  fetchPackageText(name)
        │  HTTP GET {PYPX_API_BASE}/api/packages/{name}.txt
        ▼
  pypx API (api/ module, https://pypx.app)  ── existing .txt route, unchanged
```

The fetch logic (`fetchPackageText`) is kept in a small function separate from
the MCP tool registration so tests can target it directly and point it at an
`httptest` server via `PYPX_API_BASE`.

## Test approach

`mcp/internal/tools` (or the tool package) has a `_test.go` covering:
1. **Happy path** — `httptest.NewServer` serves a canned body at
   `/api/packages/httpx.txt`; the fetch returns that body verbatim.
2. **Not found** — server returns 404 → fetch returns an error mentioning the
   package name.
3. **In-memory transport (end-to-end)** — the SDK's
   `mcp.NewInMemoryTransports()` connects a client and the real server;
   `tools/list` returns exactly `get_package`, and `tools/call get_package`
   round-trips the canned body. (The SDK provides this, so it is included
   rather than skipped.)

Structural pattern cribbed from `api/internal/handler/extras_test.go`.

## Smoke check

stdio, framed JSON-RPC piped into the built binary. stdin is held open via a
FIFO so the session isn't torn down by EOF before the server responds
(feeding-then-closing stdin immediately races the SDK's shutdown and yields no
output — a test-harness artifact, not a server bug; the in-memory transport
test covers the same round-trip deterministically).

```bash
cd mcp
go build -o /tmp/pypx-mcp ./cmd/pypx-mcp
mkfifo /tmp/pipe
/tmp/pypx-mcp </tmp/pipe >/tmp/out 2>/tmp/err &
exec 3>/tmp/pipe
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' >&3
sleep 2; exec 3>&-; wait   # then: cat /tmp/out
```

Output (stdout; stderr empty, exit 0):

```json
{"jsonrpc":"2.0","id":1,"result":{"capabilities":{"logging":{},"tools":{"listChanged":true}},"protocolVersion":"2025-06-18","serverInfo":{"name":"pypx","version":"0.1.0"}}}
{"jsonrpc":"2.0","id":2,"result":{"tools":[{"description":"Package metadata (key:value plus dependency list) for a PyPI package.","inputSchema":{"type":"object","properties":{"name":{"type":"string","description":"the PyPI package name (PEP 503 normalized, case-insensitive)"}},"required":["name"],"additionalProperties":false},"name":"get_package"}]}}
```

The `initialize` result names the server `pypx` v0.1.0; `tools/list` returns
exactly one tool, `get_package`, with a `name` (required string) input schema.

## Risks

- **Format churn (inherited):** the `.txt` formats are a semi-public API
  locked by golden-file tests (`2026-05-06-agentic-surface-design.md:112`).
  The MCP tools return bodies verbatim, so any formatter change now breaks
  MCP clients too. A reviewer should confirm the tools do not re-render.
- **`PYPX_API_BASE` handling:** default, trailing-slash trimming, and URL
  escaping of package names all need review (covered in the slice).
- **Hosted dependency:** stdio users still hit the public API over the
  network; there is no offline mode. Acceptable — the tool mirrors the public
  surface by design.
- **Transport parity:** the spike proves stdio only; streamable-HTTP hosting
  is unproven until the deferred phase builds the Caddy/compose wiring.

## Deferred (next phase)

Explicitly **not** built in this plan; a follow-up plan should be written
against this list:

1. **The other ten tools** (D5 table): `get_summary`, `search_packages`,
   `get_changelog`, `get_security`, `get_extras`, `get_docs`, `get_symbol`,
   `search_symbols`, `diff_versions`, `compare_packages` — including the
   raised client timeout for the 60s docs/symbols/diff endpoints and the
   `packages` array → `?pkgs=` joining with the max-5 cap for
   `compare_packages`.
2. **Streamable-HTTP hosting:** a `mcp.StreamableHTTPHandler`, a Caddyfile
   `handle /mcp` block, and a docker-compose service/route so
   `https://pypx.app/mcp` works with zero client install.
3. **Discovery:** mention the MCP server in `/llms.txt`
   (`api/internal/handler/llms.go`) and in the README.
4. **Binary distribution:** if stdio transport is kept for local use,
   goreleaser (or similar) to publish `pypx-mcp` binaries.
5. **In-process reuse (only if ever wanted):** if a future decision favors
   D4 option (b), first extract the `main.go:41-93` service wiring into a
   shared constructor package (a `wiring`/`app` package returning the handler
   set), then have an in-module `api/cmd/mcp` consume it. This is a refactor
   of a currently-locked file and must be its own plan.
```

