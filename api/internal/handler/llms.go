package handler

import (
	"net/http"
	"strconv"
	"strings"
)

// PackageCounter reports how many packages the search index holds.
type PackageCounter interface{ Count() (int, error) }

// LLMSHandler serves the /llms.txt index used by agents to discover the
// plain-text and JSON endpoints available on this API.
type LLMSHandler struct{ counter PackageCounter }

// NewLLMSHandler returns a handler for /llms.txt backed by counter for the
// dynamic package count in the header blurb.
func NewLLMSHandler(counter PackageCounter) *LLMSHandler {
	return &LLMSHandler{counter: counter}
}

// humanComma formats n with thousands separators, e.g. 543210 -> "543,210".
func humanComma(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}

	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}

	if neg {
		return "-" + string(out)
	}
	return string(out)
}

// siteBase is the public origin used to build absolute links in llms.txt.
// llms.txt is fetched standalone by agents and crawlers, so relative paths are
// not reliably resolvable — the llmstxt.org format expects real Markdown links.
// Matches the production server declared in cmd/gentypes/openapi.go.
const siteBase = "https://pypx.app"

const llmsHead = `# pypx — A modern PyPI frontend (agent-friendly endpoints)

> pypx exposes plain-text variants of its main JSON endpoints so agents can
> grep package data without parsing JSON. Routes return
> ` + "`text/plain; charset=utf-8`" + ` and are read-only.
> `

const llmsBody = `

## Endpoints

Each link below is a live example of the route named in the link text.

- [/api/packages/{name}.txt](` + siteBase + `/api/packages/httpx.txt): Package metadata (key:value plus dependency list).
- [/api/packages/{name}/changelog.txt](` + siteBase + `/api/packages/httpx/changelog.txt): Markdown changelog (one ## heading per version).
- [/api/packages/{name}/security.txt](` + siteBase + `/api/packages/httpx/security.txt): Vulnerabilities; ?version=; vuln_count: 0 if clean.
- [/api/packages/{name}/extras.txt](` + siteBase + `/api/packages/httpx/extras.txt): Type support, conda-forge availability, repo info.
- [/api/packages/{name}/summary.txt](` + siteBase + `/api/packages/httpx/summary.txt): One-screen agent briefing (≤2KB).
- [/api/search.txt?q=](` + siteBase + `/api/search.txt?q=http+client): TSV: name<TAB>downloads<TAB>summary; ?limit= (default 20, max 100).
- [/api/packages/{name}/docs.txt](` + siteBase + `/api/packages/requests/docs.txt): API documentation; ?prefix= filters by dotted path.
- [/api/packages/{name}/docs/{symbol}.txt](` + siteBase + `/api/packages/requests/docs/requests.api.get.txt): Single symbol; {symbol} is the fully-qualified dotted path or its bare leaf name.
- [/api/packages/{name}/symbols.txt?q=](` + siteBase + `/api/packages/requests/symbols.txt?q=get): TSV symbol search; ?kind= and ?limit=.
- [/api/packages/{name}/diff.txt?from=X&to=Y](` + siteBase + `/api/packages/httpx/diff.txt?from=0.27.0&to=0.28.0): Markdown diff: changelog slice, dependency + API changes.
- [/api/compare.txt?pkgs=a,b,c](` + siteBase + `/api/compare.txt?pkgs=httpx,requests): Compares up to 5 packages; missing ones get a leading ` + "`# skipped:`" + ` line.
- [/api/packages/{name}/stats.txt?period=4w|3m|6m](` + siteBase + `/api/packages/httpx/stats.txt?period=4w): Downloads, python versions, systems.
- [/api/popular.txt?limit=](` + siteBase + `/api/popular.txt?limit=20): TSV top packages by downloads (max 50).

## JSON endpoints

Same data as .txt, structured. GET /api returns a JSON pointer to this file.

- [/api/packages/{name}](` + siteBase + `/api/packages/httpx): metadata; /versions, /dependencies, /stats?period=4w|3m|6m
- [/api/search?q=&limit=](` + siteBase + `/api/search?q=http+client): plus /api/compare?pkgs=, /api/popular?limit= (default 12, max 50), /api/health
- [/api/openapi.json](` + siteBase + `/api/openapi.json): OpenAPI 3.1 spec for the JSON API.
- /api/packages/{name}/changelog | /security?version= | /extras | /docs | /diff?from=&to=
- Drop ` + "`.txt`" + ` for JSON — except summary/symbols/docs/{symbol} (text-only). /versions, /dependencies, /health: JSON-only.

## Rate limits

- Origin: 30 req/s, burst 60/IP. Edge: 60 req/min/IP on /api/* (10-min block).
- 429s include Retry-After and X-RateLimit-* headers.

## Examples

` + "```" + `
curl https://pypx.app/api/packages/httpx.txt
curl 'https://pypx.app/api/search.txt?q=http+client&limit=10'
curl 'https://pypx.app/api/compare.txt?pkgs=httpx,requests,aiohttp'
` + "```" + `

## Conventions

- Package names follow PEP 503 (case-insensitive); TSV ` + "`#`" + ` lines are headers.
- Empty fields are omitted (no ` + "`key:`" + ` with no value); cache headers match JSON.
- ` + "`Accept: text/plain`" + ` on a JSON route returns its ` + "`.txt`" + ` twin — except /docs/{symbol} (suffix only).

## Source

- [pypx on GitHub](https://github.com/mattstrayer/pypx): source code, issues, and license.
`

// ServeHTTP writes the llms.txt body, injecting a dynamic package count.
func (h *LLMSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	countLine := "500,000+ packages indexed."
	if h.counter != nil {
		if n, err := h.counter.Count(); err == nil && n > 0 {
			countLine = humanComma(n) + " packages indexed."
		}
	}

	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(llmsHead + countLine + llmsBody)) //nolint:errcheck
}
