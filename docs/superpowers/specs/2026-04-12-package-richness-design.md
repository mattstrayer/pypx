# Package Richness Design

**Date:** 2026-04-12  
**Status:** Draft  
**Goal:** Enrich pypx package pages with meaningful data dimensions that help Python developers evaluate, trust, and understand packages at a glance.

---

## Overview

pypx already surfaces install size, module format, Python compatibility, license, dependencies, download trends, and changelogs. This spec covers ten additional data dimensions to make package pages significantly more useful. Features are listed in implementation priority order — the structural tab decisions (whether to add an "Ecosystem" tab or absorb into existing tabs) will be made once we know which features ship in which phase.

---

## Feature 1: GitHub Health Signals

### What It Surfaces
- Star count
- Fork count  
- Open issue count
- Days since last commit (e.g. "last commit 3 days ago")

### Data Source
GitHub REST API — we already fetch GitHub releases for changelogs. The repo URL is already extracted from `project_urls`. Adding a `GET /repos/{owner}/{repo}` call returns all of the above in a single response.

### API Key
Uses existing `GITHUB_TOKEN` env var. Falls back gracefully to unauthenticated (60 req/hr) if token absent.

### Caching
Piggyback on the existing changelog cache entry (`changelog:{package}` key). Refresh on the same schedule. Store as additional fields in the cached JSON blob.

### API Changes
- Extend `ChangelogResponse` struct with `GitHubRepo` sub-struct: `stars int`, `forks int`, `open_issues int`, `last_commit_at time.Time`
- Extend `/api/packages/{name}/changelog` response to include these fields (or expose via `/api/packages/{name}/github` if cleaner)

### Frontend Display
Small cluster of stats in the Overview sidebar or Ecosystem tab:
- ⭐ 1.2k · 🍴 234 · 🐛 42 open issues · last commit 3 days ago

### Complexity: Low
All data available in a single existing API integration. Zero new external dependencies.

### Failure Modes
- Package has no GitHub URL → field is null, section hidden
- Rate limit hit → serve stale cache; log warning

---

## Feature 2: Maintainers

### What It Surfaces
Full maintainer identity from PyPI: name, email. Where a GitHub profile can be inferred (from the repo owner or email matching), show avatar and link.

### Data Source
PyPI JSON API — `info.author`, `info.author_email`, `info.maintainer`, `info.maintainer_email` are already fetched and cached. We parse these into a structured list.

**Limitation:** PyPI's JSON API does not expose the full list of PyPI accounts with the Maintainer role (that's only in the legacy XML-RPC API or the Warehouse API). We surface what's available: author + maintainer fields, which may be a single person or a comma-separated list.

GitHub avatar: if we already have a GitHub repo URL, we know the repo owner — we can show their GitHub avatar (`https://github.com/{owner}.png?size=40`) at zero API cost.

### Caching
Already cached in main package data. No new fetches.

### API Changes
- Add `Maintainers []MaintainerInfo` to `PackageData` response
- `MaintainerInfo`: `name string`, `email string`, `github_username string` (if inferrable), `avatar_url string` (if github_username known)

### Frontend Display
Small avatar row in Overview sidebar, similar to npm's maintainer list. Clicking a maintainer opens their PyPI profile or GitHub profile.

### Complexity: Low
Data already in hand. The only "enrichment" is parsing comma-separated fields and optionally linking to GitHub profile.

### Failure Modes
- Fields are empty → section hidden
- GitHub username not inferrable → show name/email only without avatar

---

## Feature 3: Type Support

### What It Surfaces
One of three typed states as a badge:
- **Typed** — package ships a `py.typed` marker file in its wheel
- **Stubs available** — a `types-{name}` or `{name}-stubs` package exists on PyPI
- **Untyped** — neither of the above

### Data Sources

**Stubs check (cheap):** Two PyPI JSON lookups:
- `https://pypi.org/pypi/types-{name}/json` → 200 = stubs package exists
- `https://pypi.org/pypi/{name}-stubs/json` → 200 = stubs package exists

**py.typed check (expensive):** Download the wheel (zip file) and check if any file matching `*.dist-info/py.typed` or `py.typed` exists at the root. Wheels can be large (tens to hundreds of MB for binary packages) so this should be done lazily/on-demand and cached aggressively.

### Precedence
`typed` > `stubs` > `untyped`. If a package has both py.typed and a stubs package, show "typed" (the stronger signal).

### Recommended Approach (Phase 1)
Ship stubs check only. It's cheap, accurate, and already tells users "you can get types for this." Add py.typed wheel inspection in a follow-up.

### Caching
Cache typed state with the main package data. Keyed as `typed:{package_name}`. TTL: 7 days (type status changes rarely).

### API Changes
- Add `TypeSupport TypeSupportInfo` to `PackageData`
- `TypeSupportInfo`: `status string` (one of: "typed", "stubs", "untyped"), `stubs_package string` (e.g. "types-requests")

### Frontend Display
Badge in the existing badge row on the package header (alongside install size, Python version, module format, license, dep count).

### Complexity: Medium (Low for Phase 1 stubs-only)

### Failure Modes
- PyPI lookup fails → status shown as unknown, no badge rendered
- Package name doesn't map to a stubs package under either convention → correctly shows "untyped"

---

## Feature 4: Security / Vulnerabilities

### What It Surfaces
- **No known vulnerabilities** badge (green)
- **N vulnerabilities** badge (red/amber) with expandable list showing:
  - CVE / GHSA ID
  - Severity (Critical / High / Medium / Low)
  - Affected version range
  - Fixed in version (if patched)
  - Link to advisory

### Data Source
[OSV.dev](https://osv.dev) — free, open, no API key required.

Request: `POST https://api.osv.dev/v1/query`
```json
{
  "package": { "name": "{package}", "ecosystem": "PyPI" }
}
```

Response: array of vulnerability objects with `id`, `severity`, `affected[].ranges`, `affected[].versions`.

### Caching
Separate cache key: `security:{package_name}`. TTL: 24 hours (vulnerabilities can be disclosed at any time).

### API Changes
- New endpoint: `GET /api/packages/{name}/security`
- Response: `{ vulns: VulnInfo[], checked_at: time }` 
- `VulnInfo`: `id string`, `severity string`, `summary string`, `affected_range string`, `fixed_in string`, `url string`
- Also add `vuln_count int` to main `PackageData` for badge rendering without a separate fetch

### Frontend Display
- Badge in header badge row: green lock icon "Secure" or red shield "3 CVEs"
- Expandable panel (either in Overview or Ecosystem tab) with full vulnerability list
- Version-specific pages show only vulns affecting that specific version

### Complexity: Low-Medium
New external API integration with simple request/response. The version-range matching logic (to filter vulns by version) is the only nuanced part.

### Failure Modes
- OSV API unreachable → serve stale cache; show "Security data unavailable" instead of badge
- Package not in OSV → correctly shows 0 vulnerabilities

---

## Feature 5: Wheel Platform Coverage

### What It Surfaces
A platform support matrix for the latest release showing which environments have prebuilt wheels:
- Pure Python (`py3-none-any`)
- Linux x86_64
- Linux ARM64 (aarch64)
- macOS x86_64 (Intel)
- macOS ARM64 (Apple Silicon)
- Windows x86_64
- musl/Alpine Linux

### Data Source
PyPI JSON API — wheel filenames are already fetched in our version/files data. Parse the `{python}-{abi}-{platform}` tag component of each `.whl` filename.

Platform tag parsing rules:
- `none-any` → Pure Python
- `manylinux*_x86_64` → Linux x86_64
- `manylinux*_aarch64` → Linux ARM64
- `macosx*_x86_64` → macOS Intel
- `macosx*_arm64` or `macosx*_universal2` → macOS Apple Silicon
- `win_amd64` → Windows x86_64
- `musllinux*_x86_64` → musl/Alpine

### Caching
Already cached — zero new fetches required.

### API Changes
- Add `PlatformCoverage PlatformCoverageInfo` to `PackageData` (derived at parse time)
- `PlatformCoverageInfo`: `pure_python bool`, `linux_x86 bool`, `linux_arm64 bool`, `macos_x86 bool`, `macos_arm64 bool`, `windows_x86 bool`, `musl bool`

### Frontend Display
Small icon grid of platform logos/icons with checkmarks. Could live in the Overview sidebar or the Versions tab alongside the existing module format badge. On the Versions tab, could show per-version platform coverage.

### Complexity: Low
All data already in hand. Pure parsing logic addition.

### Failure Modes
- Sdist-only package → all platform flags false; display "Source distribution only — requires local compilation"
- Unknown platform tags → log and ignore; display what's known

---

## Feature 6: Release Cadence

### What It Surfaces
- Sparkline or mini bar chart: releases per quarter over the past 2 years
- "Last released N days ago" (already derivable from latest version upload_time)
- "X releases in the past 12 months"
- Average time between releases

### Data Source
Version history — already fetched and cached. Pure derivation from `upload_time` fields across all versions.

### Caching
Already cached.

### API Changes
- Add `ReleaseCadence CadenceInfo` to the versions response or package response
- `CadenceInfo`: `releases_last_12mo int`, `avg_days_between_releases float`, `last_released_at time.Time`, `quarterly_counts []QuarterCount`
- `QuarterCount`: `quarter string` (e.g. "2025 Q1"), `count int`

### Frontend Display
Compact sparkline in the Overview sidebar ("12 releases in the past year, avg 30 days apart") or a small chart in the Versions tab above the version list.

### Complexity: Low
Pure arithmetic on existing data.

---

## Feature 7: Documentation

### 7a. Doc Link (Trivial)
Surface the `Documentation` key from `project_urls` with prime real estate in the Overview sidebar — a prominent "Docs" button. Already have the data; just needs better display priority.

**Complexity: Trivial**

---

### 7b. Rendered API Docs (High Complexity — Separate Phase)

**What It Surfaces**  
A full, beautiful, navigable API reference page — every public module, class, function, and method with:
- Full signature with type-annotated parameters
- Return type
- Docstring content (Google, NumPy, and Sphinx styles all parsed)
- Parameter descriptions
- Raised exceptions
- Examples (from docstring)

Available at: `/packages/{name}/{version}/docs`

**How It Works**

1. **Download wheel** — fetch the smallest pure-Python or source wheel from PyPI (URL already in version data)
2. **Extract source** — wheels are zip archives; extract all `.py` files
3. **Parse with griffe** — [griffe](https://mkdocstrings.github.io/griffe/) is a well-maintained Python AST parser (used by mkdocstrings) that extracts the full API surface without executing code
4. **Store as JSON** — structured API data stored in SQLite under `docs:{package}:{version}`
5. **Serve and render** — Go API serves the JSON; Nuxt frontend renders it as a navigable reference page

**Architecture Note**  
Our API is Go. Griffe is Python. Two options:
- **Option A (recommended): Python worker sidecar** — a small Python script/service that accepts `{package, version, wheel_url}` and returns griffe JSON. Go calls it via subprocess or HTTP.
- **Option B: Pure Go Python AST parsing** — limited, fragile tooling exists; not recommended.

The Python worker approach is clean: a single ~100-line Python script using `griffe` + `httpx` to download and parse. Go manages caching and orchestration.

**Frontend Page Structure**
- Left sidebar: module/class tree navigation (collapsible)
- Main pane: selected symbol with full signature, type hints, docstring sections
- Search within docs (filter symbols by name)
- Version switcher (switch between documented versions)
- "View on GitHub" link for each symbol (if repo URL known + line numbers available)

**Cache Strategy**  
Key: `docs:{package}:{version}`. TTL: indefinite — a specific version's source never changes.  
Generation: triggered lazily on first request; background job can pre-generate for popular packages.

**Complexity: High — Separate Phase**

---

### 7c. Dash Docset Export (Highest Complexity — Phase After 7b)

**What It Surfaces**  
A downloadable `.docset` bundle consumable by [Dash](https://kapeli.com/dash) (macOS), [Zeal](https://zealdocs.org/) (Linux/Windows), and compatible tools.

**How It Works**  
Dash docsets are: a SQLite index (`docSet.dsidx`) + HTML files + a `Info.plist`. Once we have griffe JSON from 7b, we can generate the HTML and SQLite index programmatically. The docset is then served as a downloadable archive from `/packages/{name}/{version}/docs/dash`.

**Complexity: High — Builds on 7b**

---

## Feature 8: Conda / conda-forge Availability

### What It Surfaces
Badge: "Available on conda-forge" with the latest conda version, or absent if not available.

### Data Source
Anaconda API: `GET https://api.anaconda.org/package/conda-forge/{name}`  
Returns: package metadata including `latest_version` if the package exists on conda-forge. Returns 404 if not available.

### Caching
Separate cache key: `conda:{package_name}`. TTL: 24 hours.

### API Changes
- Add `CondaForge *CondaForgeInfo` to `PackageData` (nullable — null if not on conda-forge)
- `CondaForgeInfo`: `version string`, `url string`

### Frontend Display
Badge in the header badge row or Overview sidebar. Links to the conda-forge package page.

### Complexity: Low
Simple HTTP GET with a 200/404 check. No auth required.

### Failure Modes
- Anaconda API down → serve stale cache; omit badge (don't show "not available" erroneously)
- Package name on conda-forge differs from PyPI name → known limitation; no workaround without a mapping table

---

## Feature 9: Reverse Dependencies

### What It Surfaces
"Used by N packages" with a list of the most popular/notable dependent packages.

### Data Source
**Best available free option: libraries.io**  
Endpoint: `GET https://libraries.io/api/pypi/{name}/dependents?api_key={key}`  
Returns: paginated list of dependent packages with download counts.  
Rate limit: 60 requests/minute on free tier.  
API key: required (free registration).

**Alternative (future): Build our own index**  
Extend our search worker to also index `requires_dist` for all 780K packages. Then reverse-lookup is a local SQLite query. This is more robust but requires significantly more storage and indexing time.

### Caching
Separate cache key: `dependents:{package_name}`. TTL: 7 days (dependent counts change slowly). Serve stale on rate limit.

### API Changes
- New endpoint: `GET /api/packages/{name}/dependents`
- Response: `{ count: int, top: DependentInfo[], source: "libraries.io" }`
- `DependentInfo`: `name string`, `summary string`, `downloads int`

### Frontend Display
In Ecosystem tab: "Used by 1,243 packages" with top 10 listed. Each links to their pypx page.

### Complexity: High
New API key dependency, rate limit management required, paginated data, and the count quality depends on libraries.io data freshness.

### Failure Modes
- libraries.io API down or rate limited → serve stale; show count from cache
- libraries.io data stale → acceptable, display "as of {date}"

---

## Feature 10: Similar / Alternative Packages

### What It Surfaces
4–6 related packages a developer might also evaluate (e.g., on the `requests` page: `httpx`, `aiohttp`, `urllib3`).

### Data Source
**Phase 1 — Classifier-based similarity:**  
PyPI trove classifiers encode the package's domain (e.g., `Framework :: Django`, `Topic :: Internet :: WWW/HTTP`). Packages sharing classifiers are semantically related. We'd need to extend our search index to store classifiers per package, then at query time find packages sharing ≥2 classifiers and rank by download count.

**Phase 2 — libraries.io sourcerank + related packages:**  
libraries.io has a `GET /api/pypi/{name}/related` endpoint returning related packages based on their own algorithm.

### Caching
Cache key: `similar:{package_name}`. TTL: 7 days.

### API Changes
- New endpoint: `GET /api/packages/{name}/similar`
- Response: `{ packages: SimilarPackage[] }`
- `SimilarPackage`: `name string`, `summary string`, `downloads int`, `reason string` (e.g. "shares classifiers")

### Frontend Display
"You might also like" section in Ecosystem tab. 4–6 cards with name, summary, and install command.

### Complexity: High
Classifier-based approach requires extending our 780K-package search index. Quality will be variable — classifiers are self-reported and inconsistently applied by package authors.

---

## Implementation Order & Complexity Summary

| # | Feature | Complexity | New API Dep | Extra Fetches |
|---|---------|------------|-------------|---------------|
| 1 | GitHub Health Signals | Low | No (existing) | 1 per package |
| 2 | Maintainers | Low | No | 0 |
| 3 | Type Support (stubs check) | Low | No | 2 per package |
| 4 | Security / OSV.dev | Low-Med | OSV.dev (free, no key) | 1 per package/day |
| 5 | Wheel Platform Coverage | Low | No | 0 |
| 6 | Release Cadence | Low | No | 0 |
| 7a | Doc Links | Trivial | No | 0 |
| 7b | Rendered API Docs | High | griffe (Python) | wheel download per version |
| 7c | Dash Docset Export | High | Builds on 7b | 0 additional |
| 8 | Conda Availability | Low | Anaconda API (free, no key) | 1 per package/day |
| 9 | Reverse Dependencies | High | libraries.io (free key) | 1 per package/week |
| 10 | Similar Packages | High | Optional: libraries.io | 0 (classifier-based) |

## Page Structure Decision

To be decided based on which features ship in Phase 1. Candidate approach: add a new **Ecosystem** tab for features 1, 9, 10 and release cadence; enrich the Overview badge row and sidebar with features 2, 3, 4, 5, 7a, 8; keep docs (7b/7c) on its own dedicated page route.

---

## Out of Scope

- Executing package code for analysis (security risk)
- Hosting/mirroring package files
- Generating docs for packages with C extensions only (no Python source in wheel)
- Real-time vulnerability monitoring / alerting
