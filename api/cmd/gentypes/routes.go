package main

// routes declares the HTTP surface of the API for OpenAPI generation. It
// mirrors the chi router in api/cmd/server/main.go one-for-one: every route
// registered there has exactly one entry here, except ".txt" twins, which are
// derived from the JSON entry via TextTwin (see openapi.go).
//
// Keep this table in sync with main.go. The generated document is committed
// (api/internal/handler/openapi.gen.json), so a drift shows up as a diff.

// paramDef describes a single path or query parameter.
type paramDef struct {
	Name        string
	In          string // "path" | "query"
	Required    bool
	Type        string // JSON Schema primitive: "string" | "integer"
	Enum        []string
	Description string
}

// routeDef describes one route. A route whose ResponseRef is empty and which
// carries no InlineSchema responds with text/plain.
type routeDef struct {
	Path        string
	Summary     string
	Description string     // extra prose, prepended to generated notes
	Params      []paramDef // path + query params, shared with the .txt twin
	TwinParams  []paramDef // extra query params accepted only by the .txt twin
	ResponseRef string     // component schema name; "" = text/plain
	ArrayOf     bool       // response is an array of ResponseRef
	Inline      map[string]any
	TextTwin    bool   // emit a sibling "<path>.txt" with a text/plain response
	Timeout     string // "30s" | "60s"
}

// nameParam is the PEP 503-normalized package name path parameter shared by
// every /api/packages/{name}... route.
var nameParam = paramDef{
	Name:        "name",
	In:          "path",
	Required:    true,
	Type:        "string",
	Description: "PyPI project name (case-insensitive, PEP 503 normalized).",
}

// stringArrayObject is the shape returned by the sitemap endpoints.
var sitemapSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"packages": map[string]any{
			"type":  "array",
			"items": map[string]any{"type": "string"},
		},
	},
	"required": []any{"packages"},
}

var routes = []routeDef{
	{
		Path:        "/api",
		Summary:     "API root index",
		Description: "Machine-readable index of the API surface. Agents probing the root " +
			"get a pointer to /llms.txt rather than a 404.",
		ResponseRef: "APIRootResponse",
		Timeout:     "30s",
	},
	{
		Path:        "/api/",
		Summary:     "API root index (trailing-slash alias)",
		Description: "Identical to GET /api.",
		ResponseRef: "APIRootResponse",
		Timeout:     "30s",
	},
	{
		Path:    "/api/health",
		Summary: "Liveness probe",
		Inline: map[string]any{
			"type":       "object",
			"properties": map[string]any{"status": map[string]any{"type": "string"}},
			"required":   []any{"status"},
		},
		Timeout: "30s",
	},
	{
		Path:        "/api/openapi.json",
		Summary:     "OpenAPI 3.1 document",
		Description: "The generated OpenAPI 3.1 spec describing this JSON API, including this route.",
		Inline:      map[string]any{},
		Timeout:     "30s",
	},
	{
		Path:        "/api/packages/{name}",
		Summary:     "Enriched package metadata",
		Description: "PyPI metadata enriched with dependencies, wheel/platform coverage, release cadence and maintainers.",
		Params:      []paramDef{nameParam},
		ResponseRef: "PackageResponse",
		TextTwin:    true,
		Timeout:     "30s",
	},
	{
		Path:        "/api/packages/{name}/versions",
		Summary:     "Release history",
		Description: "All published versions with their distribution files, newest first.",
		Params:      []paramDef{nameParam},
		ResponseRef: "VersionInfo",
		ArrayOf:     true,
		Timeout:     "30s",
	},
	{
		Path:        "/api/packages/{name}/dependencies",
		Summary:     "Parsed dependency tree",
		Description: "PEP 508 `requires_dist` parsed into required dependencies and extras.",
		Params:      []paramDef{nameParam},
		ResponseRef: "DependencyTree",
		Timeout:     "30s",
	},
	{
		Path:        "/api/packages/{name}/changelog",
		Summary:     "Changelog entries",
		Description: "Release notes gathered from GitHub/GitLab releases, tags, or an in-repo changelog file.",
		Params:      []paramDef{nameParam},
		ResponseRef: "ChangelogResponse",
		TextTwin:    true,
		Timeout:     "30s",
	},
	{
		Path:        "/api/packages/{name}/stats",
		Summary:     "Download statistics",
		Description: "Download counts, weekly series, and Python-version / system breakdowns from pypistats.org.",
		Params: []paramDef{
			nameParam,
			{
				Name: "period", In: "query", Type: "string",
				Enum:        []string{"4w", "3m", "6m"},
				Description: "Reporting window. Defaults to 4w; unknown values fall back to 4w.",
			},
		},
		ResponseRef: "CombinedStats",
		TextTwin:    true,
		Timeout:     "30s",
	},
	{
		Path:        "/api/packages/{name}/security",
		Summary:     "Known vulnerabilities",
		Description: "OSV advisories affecting the package.",
		Params: []paramDef{
			nameParam,
			{
				Name: "version", In: "query", Type: "string",
				Description: "Restrict advisories to this version. Defaults to the latest release.",
			},
		},
		ResponseRef: "SecurityResponse",
		TextTwin:    true,
		Timeout:     "30s",
	},
	{
		Path:        "/api/packages/{name}/summary.txt",
		Summary:     "One-screen package summary (text only)",
		Description: "Condensed overview combining metadata and security signals. No JSON representation exists.",
		Params:      []paramDef{nameParam},
		Timeout:     "30s",
	},
	{
		Path:        "/api/packages/{name}/extras",
		Summary:     "Supplementary signals",
		Description: "Type-checking support, conda-forge availability, and repository health signals.",
		Params:      []paramDef{nameParam},
		ResponseRef: "ExtrasResponse",
		TextTwin:    true,
		Timeout:     "30s",
	},
	{
		Path:        "/api/search",
		Summary:     "Full-text package search",
		Description: "SQLite FTS5 search across package names and summaries, ranked by relevance and downloads.",
		Params: []paramDef{
			{Name: "q", In: "query", Required: true, Type: "string", Description: "Search query."},
			{Name: "limit", In: "query", Type: "integer", Description: "Maximum results, 1-100. Defaults to 20."},
		},
		ResponseRef: "PackageEntry",
		ArrayOf:     true,
		TextTwin:    true,
		Timeout:     "30s",
	},
	{
		Path:        "/api/compare",
		Summary:     "Side-by-side package comparison",
		Description: "Compares up to 5 packages on downloads, maintenance, typing, and vulnerabilities.",
		Params: []paramDef{
			{
				Name: "pkgs", In: "query", Required: true, Type: "string",
				Description: "Comma-separated package names, 1-5 entries.",
			},
		},
		ResponseRef: "CompareInput",
		TextTwin:    true,
		Timeout:     "30s",
	},
	{
		Path:        "/api/popular",
		Summary:     "Most-downloaded packages",
		Description: "Top packages by 30-day download count.",
		Params: []paramDef{
			{Name: "limit", In: "query", Type: "integer", Description: "Maximum results, 1-50. Defaults to 12."},
		},
		ResponseRef: "PackageEntry",
		ArrayOf:     true,
		TextTwin:    true,
		Timeout:     "30s",
	},
	{
		Path:        "/api/sitemap/popular",
		Summary:     "Popular package names for sitemap generation",
		Description: "Names of the top 5,000 packages by download count.",
		Inline:      sitemapSchema,
		Timeout:     "30s",
	},
	{
		Path:        "/api/sitemap/cached",
		Summary:     "Cached package names for sitemap generation",
		Description: "Names of every package fetched and cached at least once.",
		Inline:      sitemapSchema,
		Timeout:     "30s",
	},
	{
		Path:        "/llms.txt",
		Summary:     "Agent discovery index (text only)",
		Description: "Index of the plain-text and JSON endpoints available on this API, per the llms.txt convention.",
		Timeout:     "30s",
	},
	{
		Path:        "/api/packages/{name}/docs",
		Summary:     "Extracted API documentation",
		Description: "Public API surface (modules, classes, functions, parameters, docstrings) extracted from the latest wheel.",
		Params:      []paramDef{nameParam},
		TwinParams: []paramDef{
			{Name: "prefix", In: "query", Type: "string", Description: "Only render symbols under this dotted path prefix."},
		},
		ResponseRef: "DocsResponse",
		TextTwin:    true,
		Timeout:     "60s",
	},
	{
		Path:        "/api/packages/{name}/docs/{symbol}",
		Summary:     "Documentation for a single symbol (text only)",
		Description: "Renders one symbol's signature and docstring. Content negotiation does not apply to this route: it always returns text/plain, and a trailing \".txt\" on the symbol is stripped.",
		Params: []paramDef{
			nameParam,
			{
				Name: "symbol", In: "path", Required: true, Type: "string",
				Description: "Dotted symbol path, optionally module-qualified (e.g. \"Client\" or \"httpx.Client\"). A trailing \".txt\" is ignored.",
			},
		},
		Timeout: "60s",
	},
	{
		Path:        "/api/packages/{name}/symbols.txt",
		Summary:     "Symbol index (text only)",
		Description: "Flat, filterable list of every extracted symbol. No JSON representation exists.",
		Params: []paramDef{
			nameParam,
			{Name: "q", In: "query", Type: "string", Description: "Substring filter on the symbol path."},
			{
				Name: "kind", In: "query", Type: "string",
				Enum:        []string{"function", "class", "exception"},
				Description: "Restrict to one symbol kind.",
			},
			{Name: "limit", In: "query", Type: "integer", Description: "Maximum symbols, 1-500. Defaults to 100."},
		},
		Timeout: "60s",
	},
	{
		Path:        "/api/packages/{name}/diff",
		Summary:     "Version-to-version diff",
		Description: "Changelog slice, dependency changes, and public API changes between two releases.",
		Params: []paramDef{
			nameParam,
			{Name: "from", In: "query", Required: true, Type: "string", Description: "Older version."},
			{Name: "to", In: "query", Required: true, Type: "string", Description: "Newer version. Must be strictly newer than from."},
		},
		ResponseRef: "DiffInput",
		TextTwin:    true,
		Timeout:     "60s",
	},
}
