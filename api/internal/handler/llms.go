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

const llmsHead = `# pypx — A modern PyPI frontend (agent-friendly endpoints)

> pypx exposes plain-text variants of all its JSON endpoints so coding
> agents can grep package data without parsing JSON. All routes return
> ` + "`text/plain; charset=utf-8`" + ` and are read-only.
> `

const llmsBody = `

## Endpoints

- /api/packages/{name}.txt — Package metadata (key:value plus dependency list).
- /api/packages/{name}/changelog.txt — Markdown changelog (one ## heading per version).
- /api/packages/{name}/security.txt — Vulnerabilities; supports ?version= ; vuln_count: 0 if clean.
- /api/packages/{name}/extras.txt — Type support, conda-forge availability, repo info.
- /api/packages/{name}/summary.txt — One-screen agent briefing (≤2KB).
- /api/search.txt?q= — TSV search results: name<TAB>downloads<TAB>summary; supports ?limit= (default 20).
- /api/packages/{name}/docs.txt — API documentation; supports ?prefix= to filter by dotted path.
- /api/packages/{name}/docs/{symbol}.txt — Single symbol (e.g. /docs/Client.get.txt).
- /api/packages/{name}/symbols.txt?q= — TSV symbol search; supports ?kind= and ?limit=.
- /api/packages/{name}/diff.txt?from=X&to=Y — Markdown diff between two versions: changelog slice, dependency changes, API changes (added/removed/signature-changed).
- /api/compare.txt?pkgs=a,b,c — Side-by-side comparison of up to 5 packages. Missing packages emit a leading ` + "`# skipped:`" + ` line.

## JSON endpoints

Same data as .txt, structured. GET /api returns this index as JSON.

- /api/packages/{name} — enriched metadata; /versions, /dependencies, /api/packages/{name}/stats?period=4w|3m|6m
- /api/packages/{name}/changelog | /security?version= | /extras | /docs | /docs/{symbol} | /diff?from=&to=
- /api/search?q=&limit= | /api/compare?pkgs= | /api/popular?limit= (max 50) | /api/health

## Rate limits

- Origin: 30 req/s sustained, burst 60 (per IP). Edge: 60 req/min per IP on /api/* (10-min block).
- 429 responses include Retry-After and X-RateLimit-* headers.

## Examples

` + "```" + `
curl https://pypx.app/api/packages/httpx.txt
curl https://pypx.app/api/packages/httpx/security.txt
curl 'https://pypx.app/api/search.txt?q=http+client&limit=10'
curl 'https://pypx.app/api/compare.txt?pkgs=httpx,requests,aiohttp'
` + "```" + `

## Conventions

- Package names follow PEP 503 normalization (case-insensitive).
- Lines starting with ` + "`#`" + ` in TSV are headers; agents may skip them.
- Empty fields are omitted entirely (no ` + "`key:`" + ` with no value).
- Cache headers match the JSON variants.

## Source

https://github.com/mattstrayer/pypx
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
