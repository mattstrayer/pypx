package handler

import "net/http"

// LLMSHandler serves the static /llms.txt index used by agents to discover
// the plain-text endpoints available on this API.
type LLMSHandler struct{}

// NewLLMSHandler returns a handler for /llms.txt.
func NewLLMSHandler() *LLMSHandler { return &LLMSHandler{} }

const llmsBody = `# pypx — A modern PyPI frontend (agent-friendly endpoints)

> pypx exposes plain-text variants of all its JSON endpoints so coding
> agents can grep package data without parsing JSON. All routes return
> ` + "`text/plain; charset=utf-8`" + ` and are read-only.

## Endpoints

- /api/packages/{name}.txt — Package metadata (key:value plus dependency list).
- /api/packages/{name}/changelog.txt — Markdown changelog (one ## heading per version).
- /api/packages/{name}/security.txt — Vulnerabilities; vuln_count: 0 if clean.
- /api/packages/{name}/extras.txt — Type support, conda-forge availability, repo info.
- /api/packages/{name}/summary.txt — One-screen agent briefing (≤2KB).
- /api/search.txt?q= — TSV search results: name<TAB>downloads<TAB>summary.

## Examples

` + "```" + `
curl https://pypx.app/api/packages/httpx.txt
curl https://pypx.app/api/packages/httpx/security.txt
curl 'https://pypx.app/api/search.txt?q=http+client&limit=10'
` + "```" + `

## Conventions

- Package names follow PEP 503 normalization (case-insensitive).
- Lines starting with ` + "`#`" + ` in TSV are headers; agents may skip them.
- Empty fields are omitted entirely (no ` + "`key:`" + ` with no value).
- Cache headers match the JSON variants.

## Source

https://github.com/mattstrayer/pypx
`

// ServeHTTP writes the llms.txt body.
func (h *LLMSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(llmsBody)) //nolint:errcheck
}
