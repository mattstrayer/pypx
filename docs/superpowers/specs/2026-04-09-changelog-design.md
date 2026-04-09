# Changelog on Versions Page

**Date:** 2026-04-09
**Status:** Approved
**Parent spec:** `2026-04-09-pypx-design.md`

## Overview

Add changelog visibility to the pypx versions page by fetching GitHub Releases data and merging it into the existing version timeline UI. Changelogs are co-located with their version — not shown as a separate section.

**Problem:** When evaluating whether to upgrade a package, developers need to see what changed in each version. Currently the versions tab only shows release date, size, and format — no changelog content.

**Solution:** New Go API endpoint fetches GitHub Releases, caches them aggressively, and returns per-version changelog entries. The frontend merges this data into the versions timeline, making changelog content expandable inline.

## Data Source Strategy

**MVP:** GitHub Releases API only. Most well-maintained Python packages publish GitHub Releases with markdown bodies. This covers the majority of packages developers care about evaluating.

**Future enhancement:** Parse `CHANGELOG.md`, `CHANGELOG.rst`, or `CHANGES.txt` from sdist/wheel as a fallback for packages without GitHub Releases.

**Why GitHub Releases first:**
- Already structured per-version (no parsing needed)
- Markdown body is usually curated and clean
- PyPI `project_urls` reliably contains the GitHub repo URL
- CHANGELOG file parsing is a rabbit hole of format inconsistencies

## Go API

### GitHub Client (`api/internal/github/client.go`)

New internal package for GitHub Releases API interaction.

**Repo URL extraction:**
- Reads `project_urls` from PyPI package metadata
- Scans keys (case-insensitive): `Source`, `Repository`, `Source Code`, `Homepage`, `GitHub`
- Matches URLs against `github.com/{owner}/{repo}` pattern
- Returns `(owner, repo)` or empty if no GitHub URL found

**Releases fetching:**
- `GET https://api.github.com/repos/{owner}/{repo}/releases?per_page=100`
- Follows pagination if needed (Link header)
- Parses each release: tag name, title, body (markdown), published_at, html_url
- Maps tag → version by stripping leading `v` (e.g., `v2.31.0` → `2.31.0`)
- HTTP timeout: 15 seconds

**Authentication:**
- Unauthenticated by default (60 requests/hr)
- Optional `GITHUB_TOKEN` env var adds `Authorization: Bearer {token}` header (5000 requests/hr)
- With 7-day cache TTL, unauthenticated rate limit is sufficient for normal traffic

**Types:**

```go
type Release struct {
    Version     string `json:"version"`
    TagName     string `json:"tag_name"`
    Title       string `json:"title"`
    Body        string `json:"body"`
    PublishedAt string `json:"published_at"`
    URL         string `json:"url"`
}
```

### Changelog Endpoint (`api/internal/handler/changelog.go`)

**Route:** `GET /api/packages/{name}/changelog`

**Flow:**
1. Fetch PyPI package data (via existing `fetchPackage` — uses cached PyPI data)
2. Extract GitHub repo from `project_urls`
3. If no GitHub repo found, return empty entries (not an error)
4. Check SQLite cache with key `changelog:{lowercase_name}`, TTL 7 days
5. On miss, fetch GitHub Releases via client
6. Cache and return

**Response format:**

```json
{
  "package": "requests",
  "source": "github_releases",
  "repo_url": "https://github.com/psf/requests",
  "entries": [
    {
      "version": "2.31.0",
      "title": "v2.31.0",
      "body": "## Changes\n- Fixed...",
      "published_at": "2023-05-22T15:00:00Z",
      "url": "https://github.com/psf/requests/releases/tag/v2.31.0"
    }
  ]
}
```

- `entries` is always an array (empty if no GitHub repo or no releases)
- `source` indicates data origin (for future: `"changelog_file"` when CHANGELOG parsing is added)
- `repo_url` included so the frontend can link to the full releases page

**Cache TTL:** 7 days (changelogs for released versions don't change)

## Frontend Integration

### Markdown Rendering

Add `@nuxtjs/mdc` module to the Nuxt project. This also fixes the existing issue where package descriptions on the Overview tab are rendered as plain text instead of markdown.

- Register in `nuxt.config.ts` modules array
- Use `<MDC>` component for rendering changelog bodies and package descriptions

### Versions Tab (`PackageVersions.vue`)

**Data fetching:**
- Fetch both `/api/packages/{name}/versions` and `/api/packages/{name}/changelog` in parallel using `useAsyncData`
- Build a lookup map from changelog entries: `version → entry`
- Merge into the sorted versions list via computed property

**UI changes:**
- Versions with changelog entries show a subtle indicator (small chevron icon or "changelog" text badge) on the row
- Clicking a version row with a changelog expands it to reveal the changelog body rendered as markdown below the row metadata
- Clicking again collapses it
- Versions without changelog entries remain non-expandable (no indicator)
- Expanded changelog content gets the same zinc styling: bg-zinc-900/50 border-zinc-800, rendered markdown in text-zinc-300

**No structural change to the table** — the expandable area appears as an additional row below the version row, spanning all columns.

### Version Page (`/packages/:name/:version`)

- Fetch changelog data alongside existing package and versions data
- Find the matching entry for the current version
- Render the changelog body as markdown inline within the version detail content, below the install command and metadata grid
- If no changelog entry exists for that version, nothing extra is shown — no empty state, no "no changelog" message
- Include a link to the GitHub release page (`entry.url`) when available

### Package Overview Tab (`PackageOverview.vue`)

- Replace the plain-text `whitespace-pre-wrap` description rendering with `<MDC>` component
- Respects `description_content_type` — render as markdown when type is `text/markdown`, plain text otherwise

## API Types (TypeScript)

```ts
export interface ChangelogEntry {
  version: string
  title: string
  body: string
  published_at: string
  url: string
}

export interface ChangelogData {
  package: string
  source: string
  repo_url: string
  entries: ChangelogEntry[]
}
```

Add `fetchChangelog(name: string): Promise<ChangelogData>` to the `useApi` composable.

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `GITHUB_TOKEN` | (none) | Optional GitHub API token for higher rate limits |

## Future Enhancements (Out of Scope)

- CHANGELOG.md / CHANGELOG.rst / CHANGES.txt file parsing as fallback
- Diff view between two version changelogs
- Changelog search/filtering
- Pre-fetching changelogs for top packages via background worker
