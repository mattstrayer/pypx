# Agent Discoverability — Design

**Date:** 2026-07-28
**Status:** Approved pending review
**Epic:** Agentic surface, phase 4+ (follows phases 1–3: llms.txt + .txt endpoints, docs/symbols, compare/diff)

## Problem

The `.txt` agent surface (11 endpoints + `/llms.txt`) is complete but **undiscoverable**. An agent landing on https://pypx.app with zero prior knowledge cannot find any of it:

- Nothing references `/llms.txt` — not robots.txt, not the sitemap, not any `<link>` tag, not the footer, not JSON-LD. (Footer + API-root discovery links were the phase-1 spec's stated mitigation and were never built.)
- `Accept: text/plain` / `text/markdown` content negotiation does not exist; the `.txt` suffix must be guessed. Claude Code, Cursor, and OpenCode send `Accept: text/markdown` in 2026 (Checkly survey, Feb 2026); Cloudflare shipped edge support for the pattern.
- `GET /api` 404s; llms.txt mentions zero JSON endpoints; there is no OpenAPI spec.
- 429 responses carry no `Retry-After` / `X-RateLimit-*`; limits are unadvertised.
- `stats` and `popular` have no `.txt` variants despite download trends being a headline feature.
- `docs/cloudflare-setup.md` describes Bot Fight Mode ON + a CAPTCHA rule on `/api/*`, but live testing (2026-07-28, from residential and datacenter egress incl. Anthropic infra, with ClaudeBot/GPTBot/python-requests/curl UAs) shows **no blocking or challenges** — the dashboard was changed and the doc drifted.

2026 landscape research verdict: no major agent auto-fetches `/llms.txt`; discovery comes from crawlable HTML + sitemaps → robots.txt → JSON-LD → content negotiation → llms.txt-as-paste-target. Breadcrumbs and negotiation matter more than the index file itself.

## Audience

Primary: **coding agents / CLIs** (Claude Code, Cursor, etc.) hitting the site via curl/fetch. Browsing-agent improvements land where they stack cheaply (robots.txt, JSON-LD already present).

## Design — four phases, one branch + PR each

### Phase 1 — Edge: docs-to-reality sync + continuous verification

The edge is already open (verified). Remaining work:

1. **Update `docs/cloudflare-setup.md`** to match the actual dashboard ruleset (owner confirms current state; Bot Fight Mode / CAPTCHA rule sections rewritten to whatever is live, with an explicit "agents must pass" requirement stated).
2. **Add edge cache rule for `/llms.txt`** (outside `/api/*`, currently uncached at the edge) — dashboard change, documented in the same doc.
3. **`scripts/check-agent-access.sh`** — curls `/llms.txt`, one `.txt` package route, and `search.txt` with agent user-agents (ClaudeBot, GPTBot, plain curl); fails on non-200, non-text content-type, or challenge markers (`cf-mitigated`, "Just a moment"). Runnable manually and from CI on a schedule, so a future Cloudflare default change cannot silently re-block agents.

### Phase 2 — Discovery breadcrumbs

1. **llms.txt overhaul** (`api/internal/handler/llms.go`):
   - Add a compact `## JSON endpoints` section (packages, versions, dependencies, stats, security, extras, search, compare, diff, popular, health).
   - Document previously-undocumented params (`limit` on search, etc.).
   - Add `## Rate limits` (30 req/s origin, 60 req/min edge block; 429 semantics).
   - Link `/api/openapi.json` (phase 4).
   - Dynamic package count in the blurb (from the search index), as the original epic spec intended. Keep total ≤ ~2.5 KB. No `/llms-full.txt` (stays rejected).
2. **`GET /api` root index**: small JSON `{ "llms": "/llms.txt", "openapi": "/api/openapi.json", "docs": "https://github.com/..." }` instead of a 404.
3. **robots.txt**: comment line `# Agent API index: https://pypx.app/llms.txt`; explicit `Allow: /` groups for GPTBot, ClaudeBot, Claude-User, OAI-SearchBot, PerplexityBot (belt-and-suspenders signal; behavior already permissive).
4. **HTML breadcrumbs** (web):
   - `<link rel="alternate" type="text/plain" href="https://pypx.app/api/packages/{name}.txt">` on package pages.
   - Footer link: "API for agents → /llms.txt".

### Phase 3 — Content negotiation + protocol hygiene

1. **Accept-header negotiation middleware** (Go): on JSON API routes that have a `.txt` sibling, `Accept: text/plain` or `text/markdown` (when preferred over JSON per the header's q-values) returns the existing textfmt output; responses gain `Vary: Accept`. The `.txt` suffix remains the canonical, cache-stable path; negotiated responses are byte-identical to their `.txt` twin.
2. **Rate-limit transparency** (`api/internal/middleware/ratelimit.go`): 429s gain `Retry-After` and `X-RateLimit-Limit` / `X-RateLimit-Remaining` headers.
3. **New formatters**: `stats.txt` (download trends) and `popular.txt` (TSV, mirroring search.txt conventions) in `api/internal/textfmt/`, wired in main.go and added to llms.txt.
4. **Caddy write-timeout fix**: raise `write` timeout (currently 30s) above the API's 60s docs/diff handler timeout so cold `docs.txt` extractions are not severed at the proxy.

### Phase 4 — OpenAPI 3.1

- Served at `/api/openapi.json`, linked from llms.txt, `/api` root, and robots.txt comment.
- **Generated by extending `api/cmd/gentypes`**: it already reflects on every Go response struct for the TS types; add an OpenAPI emitter reusing the same reflection for schemas. Paths/params/response-type mappings declared in one Go table in the generator. Committed artifact + CI check that regeneration is clean (same pattern as `api.gen.ts`).
- This reverses the original epic's "no OpenAPI" decision deliberately: 2026 agents parse OpenAPI natively, and gentypes-based generation removes the drift cost that motivated the original rejection.

## Explicitly not building

- MCP server (out of scope by user request; plan 022 spike remains branch-only).
- NLWeb, agents.json, `ai-catalog.json`/ARD (no adoption yet — revisit ARD ~2027-01), Content-Signal, ai.txt.
- `/llms-full.txt` (re-affirmed rejection).
- HTML-page content negotiation at Caddy (JSON-API negotiation covers the coding-agent audience).
- Web Bot Auth verification (server-side trust tooling, not discovery; nothing to restrict).

## Error handling

- Negotiation middleware falls through to JSON on any ambiguous/absent Accept header; it never 406s.
- `check-agent-access.sh` distinguishes network failure from challenge/block (exit codes) so CI alerts are actionable.
- OpenAPI generator failures break `make gentypes` / CI, never the server at runtime (spec is a committed static artifact).

## Testing

- Go: handler tests for `/api` root, negotiation middleware (q-value cases, Vary header, byte-parity with `.txt` twin), 429 headers, new formatters (golden files, matching existing textfmt test style).
- Web: vitest/snapshot coverage for the `<link rel="alternate">` head entries and footer link.
- Live: `scripts/check-agent-access.sh` post-deploy.
- OpenAPI: CI regeneration check + spec validation (swagger-cli or kin-openapi validate) in the generator test.

## Verification of success

An agent given only "https://pypx.app" can, without human help: fetch the homepage → find `/llms.txt` via link/footer/robots → learn the full `.txt` and JSON syntax → fetch any package's data in plain text → learn rate limits without tripping them. Each phase's PR demonstrates its slice of that path.
