# Changelog Sources — Design Spec

**Date:** 2026-04-12
**Status:** Approved

## Problem

The existing changelog feature fetches data exclusively from GitHub Releases. Packages that don't publish releases (e.g. `six`, `click`, many older Python libraries) show no changelog data at all, even when rich history is available via CHANGELOG files or git tags.

## Goal

Extend the changelog feature to show data for packages that don't use GitHub Releases, using a fallback chain of additional sources. The existing Versions tab UI (expandable rows) is reused unchanged. A subtle source badge is added to indicate where data came from.

---

## Architecture

### New package: `internal/changelog`

Owns the source abstraction and parallel fetch coordination.

```
api/internal/
├── changelog/
│   ├── source.go       # Source interface, Entry type, Result type
│   ├── registry.go     # Parallel fetch + priority-based result selection
│   └── parser.go       # CHANGELOG file parsing logic
├── github/
│   ├── client.go               (existing, extended with tag/file methods)
│   ├── releases_source.go      # GitHubReleasesSource
│   ├── file_source.go          # GitHubChangelogFileSource
│   └── tags_source.go          # GitHubTagsSource
└── gitlab/
    ├── client.go               # new — GitLab API client
    ├── file_source.go          # GitLabChangelogFileSource
    └── tags_source.go          # GitLabTagsSource
```

### Source interface

```go
type Source interface {
    Name() string
    Priority() int  // lower = higher priority
    Fetch(ctx context.Context, repoURL string) ([]Entry, error)
}
```

### Registry

Receives an ordered source list, fires all concurrently via goroutines, waits for all to complete, returns the highest-priority non-empty result. Uses a 15-second derived context so a slow source cannot block others.

```go
type Result struct {
    Source  string
    Entries []Entry
}
```

### Platform detection

The handler calls `ExtractGitHubRepo()` (existing) and `ExtractGitLabRepo()` (new) on the PyPI project URL. Based on which matches, it constructs the appropriate source list and passes it to the registry.

---

## Sources

### Priority order

| Priority | GitHub repo | GitLab repo |
|----------|-------------|-------------|
| 0 | `GitHubReleasesSource` | `GitLabReleasesSource` |
| 1 | `GitHubChangelogFileSource` | `GitLabChangelogFileSource` |
| 2 | `GitHubTagsSource` | `GitLabTagsSource` |

### GitHubReleasesSource / GitLabReleasesSource

Existing behaviour, unchanged. Fetches structured GitHub Release objects.

GitLab: GitLab has a Releases API (`GET /projects/{id}/releases`) — implement a `GitLabReleasesSource` at priority 0 for GitLab repos.

### GitHubChangelogFileSource / GitLabChangelogFileSource

Fetches raw file content via the GitHub Contents API (GitLab Raw File API), trying filenames in order:

```
CHANGELOG.md, CHANGELOG.rst, CHANGELOG.txt, CHANGELOG
CHANGES.md, CHANGES.rst, CHANGES.txt, CHANGES
HISTORY.md, HISTORY.rst, HISTORY.txt, HISTORY
```

First file found is parsed. See **CHANGELOG File Parsing** section for parsing logic.

### GitHubTagsSource / GitLabTagsSource

1. Fetch all tags: `GET /repos/{owner}/{repo}/tags` (GitHub) or `GET /projects/{id}/repository/tags` (GitLab)
2. Filter to tags that look like version strings (strip leading `v`, must parse as semver or calver)
3. Sort descending by version, take up to 50
4. For each consecutive tag pair `(newer, older)`, fetch commits between them
5. Filter noise commits (messages matching `^Merge`, `^bump version`, `^chore:`, `^release:`, case-insensitive)
6. Cap at 30 commits per version entry
7. Format body as markdown bullet list of commit messages

**Entry fields:**
- `version`: normalized tag name (v-stripped)
- `tag_name`: original tag string
- `title`: tag name
- `published_at`: tag commit date (ISO-8601)
- `body`: markdown bullet list of commits
- `body_html`: server-side rendered HTML
- `url`: compare view URL (e.g. `https://github.com/owner/repo/compare/v1.15.0...v1.16.0`)

---

## CHANGELOG File Parsing

### Strict mode

If fewer than 2 sections with recognizable version strings are detected, return `[]Entry{}`. The registry will fall through to the next source.

### Supported heading patterns

Detected in priority order (first match per line wins):

| Pattern | Example | Tools |
|---------|---------|-------|
| `## [VERSION] - DATE` | `## [1.0.0] - 2023-06-20` | Keep a Changelog, git-cliff, release-please, conventional-changelog |
| `## [vVERSION](url) (DATE)` | `## [v1.17.0](...) (2026-03-13)` | github-changelog-generator |
| `## VERSION (DATE)` | `## 2.31.0 (2026-04-08)` | OpenAI SDK, requests HISTORY.md |
| `## [VERSION]` | `## [1.0.0]` | bare bracket, no date |
| `## v?VERSION` | `## v1.0.0` or `## 1.0.0` | bare Markdown heading |
| `# v?VERSION` | `# 1.0.0` | H1 style |
| RST `Version X.Y.Z` + underline | `Version 8.3.2\n-----------` | Click, towncrier RST |
| RST bare version + underline | `1.0.0\n------` | Sphinx-style |

### Special handling

- `[Unreleased]` sections are silently skipped
- Sections without a recognizable date get an empty `published_at` (frontend handles this gracefully)
- Version strings are normalized the same way as GitHub release tags (strip leading `v`)
- Body text between headings is rendered to HTML server-side via the existing `markdown.Render()` call

---

## API Response

Shape is unchanged. The `source` field gains new values:

| Value | Meaning |
|-------|---------|
| `github_releases` | GitHub Releases (existing) |
| `github_changelog_file` | Parsed CHANGELOG/CHANGES/HISTORY file from GitHub |
| `github_tags` | Commit list between GitHub tags |
| `gitlab_releases` | GitLab Releases |
| `gitlab_changelog_file` | Parsed file from GitLab |
| `gitlab_tags` | Commit list between GitLab tags |
| `none` | No data found from any source |

No new fields. No frontend type changes required.

---

## Frontend Changes

### Source badge

When `source` is not `github_releases` (and entries exist), a small right-aligned badge is shown above the versions table:

- Positioned in the table header row, right-aligned
- Contents: GitHub/GitLab icon + source label (e.g. "via git tags", "via CHANGELOG.md")
- Styling: `bg-zinc-800 border border-zinc-700 text-zinc-400 text-[10px] px-1.5 py-0.5 rounded font-mono`
- Only rendered when changelog data is present and source is not the default

### Source label mapping

| `source` value | Badge text |
|---------------|-----------|
| `github_changelog_file` | `via CHANGELOG` |
| `github_tags` | `via git tags` |
| `gitlab_changelog_file` | `via CHANGELOG` |
| `gitlab_tags` | `via git tags` |

---

## Caching

Unchanged. The final `Result` (whichever source won) is cached at the handler level with the existing 7-day TTL. Cache key: `changelog:{package}`. The `source` field is persisted in the cached response.

---

## Error Handling

- Individual source failures (network error, 404, parse failure) are logged and contribute an empty result — they do not block other sources
- If all sources return empty or error, the response returns `source: "none"` with an empty `entries` array (same as current behaviour for packages with no repo URL)
- The 15-second context timeout on the registry prevents any single slow source from hanging the request

---

## Out of Scope

- Bitbucket, Forgejo, Codeberg, and other forge platforms (can be added later as new `Source` implementations)
- Searching or filtering within changelog entries
- Diff view between versions
- Pre-fetching changelogs for popular packages
- Showing the full unparsed CHANGELOG file as a fallback when strict parsing fails
