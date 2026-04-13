# Maintenance Status Detection

## Summary

Surface a "Possibly Unmaintained" or "Likely Unmaintained" badge on the package page when signals indicate a package may no longer be actively maintained. No badge is shown for active packages — active is the default assumption.

## Signals

Four signals drive the assessment:

| Signal | Source | Already Available? |
|---|---|---|
| Time since last PyPI release | `ReleaseCadence.LastReleasedAt` | Yes |
| Time since last GitHub commit | `RepoInfo.LastPushedAt` | Yes |
| GitHub "archived" flag | `RepoInfo.Archived` | No — requires adding `archived` field to `RepoInfo` struct |
| Release frequency trend | `ReleaseCadence.QuarterlyCounts` | Yes (not used in v1 — kept simple) |

Signal D (release frequency trend) is available but intentionally excluded from the initial implementation to keep the logic simple. It can be added as a modifier later.

## Thresholds

All thresholds are constants in `web/app/composables/useMaintenanceStatus.ts` for easy tuning.

### With GitHub repo linked

Both release AND commit thresholds must be exceeded (AND logic). A package with an old release but recent commits is likely still maintained.

| Level | Last Release | Last Commit |
|---|---|---|
| Possibly Unmaintained | 18+ months (548 days) | 12+ months (365 days) |
| Likely Unmaintained | 3+ years (1,095 days) | 2+ years (730 days) |

**Exception:** If the repo is archived on GitHub, immediately classify as "Likely Unmaintained" regardless of dates.

### Without GitHub repo (PyPI-only)

Conservative thresholds since we have less signal:

| Level | Last Release |
|---|---|
| Possibly Unmaintained | 2+ years (730 days) |
| Likely Unmaintained | 3+ years (1,095 days) |

## Architectural Refactor: Move RepoInfo to Extras Endpoint

### Problem

GitHub `RepoInfo` is currently fetched inside the changelog handler and piggybacked onto the changelog response. This means:
- The changelog endpoint does double duty (changelog entries + repo metadata)
- Any feature needing repo info (like maintenance status) must depend on the changelog response
- The frontend extracts `repoInfo` from changelog data to display in the sidebar — an indirect dependency

### Solution

Move `RepoInfo` fetching into the extras handler, which already serves supplementary package data (type support, conda-forge info) in parallel. The extras endpoint is a natural home for repo metadata.

**Changes:**
- Extras handler gains a GitHub client and fetches `RepoInfo` in parallel with type support and conda
- Extras response gains `repo_info` field
- Changelog handler drops `RepoInfo` fetching and `repo_info` from its response
- Frontend: `repoInfo` comes from extras instead of changelog

## Data Model

### Frontend composable

File: `web/app/composables/useMaintenanceStatus.ts`

```typescript
export type MaintenanceStatus = 'possibly_unmaintained' | 'likely_unmaintained';

export const THRESHOLDS = {
  POSSIBLY_UNMAINTAINED_RELEASE_DAYS: 548,   // ~18 months
  POSSIBLY_UNMAINTAINED_COMMIT_DAYS: 365,    // 12 months
  LIKELY_UNMAINTAINED_RELEASE_DAYS: 1095,    // ~3 years
  LIKELY_UNMAINTAINED_COMMIT_DAYS: 730,      // ~2 years
  POSSIBLY_UNMAINTAINED_NO_REPO_DAYS: 730,   // 2 years (PyPI-only, conservative)
  LIKELY_UNMAINTAINED_NO_REPO_DAYS: 1095,    // 3 years (PyPI-only)
} as const;

export function computeMaintenanceStatus(
  lastReleasedAt: string | undefined,
  lastCommitAt: string | undefined,
  archived: boolean,
): MaintenanceStatus | undefined;
```

### Logic

1. If `archived` → immediately return `likely_unmaintained`
2. If `lastCommitAt` is defined (repo linked):
   - If release age >= `LIKELY_UNMAINTAINED_RELEASE_DAYS` AND commit age >= `LIKELY_UNMAINTAINED_COMMIT_DAYS`, return `likely_unmaintained`
   - If release age >= `POSSIBLY_UNMAINTAINED_RELEASE_DAYS` AND commit age >= `POSSIBLY_UNMAINTAINED_COMMIT_DAYS`, return `possibly_unmaintained`
3. If `lastCommitAt` is undefined (no repo):
   - If release age >= `LIKELY_UNMAINTAINED_NO_REPO_DAYS`, return `likely_unmaintained`
   - If release age >= `POSSIBLY_UNMAINTAINED_NO_REPO_DAYS`, return `possibly_unmaintained`
4. Otherwise, return `undefined` (active, no badge shown)

## GitHub Client Change

Add `Archived` field to `ghRepo` (internal) and `RepoInfo` (public) structs. Map it in `FetchRepoInfo`. No new API calls — the GitHub repo endpoint already returns `archived`.

## Backend Changes

### Extras handler

- Add GitHub client dependency to `ExtrasHandler`
- Extract GitHub repo from PyPI project URLs (using existing `gh.ExtractGitHubRepo`)
- Fetch `RepoInfo` in parallel with type support and conda
- Add `RepoInfo` to `ExtrasResponse`

### Changelog handler

- Remove `RepoInfo` fetching from `buildResponse()`
- Remove `RepoInfo` field from `ChangelogResponse`
- Remove GitHub client dependency from `ChangelogHandler` (if no longer needed for anything else — check first)

## Frontend Changes

### TypeScript types

- Add `archived?: boolean` to `RepoInfo` interface
- Add `repo_info?: RepoInfo` to `ExtrasData` interface
- Remove `repo_info` from `ChangelogData` interface

### Package page

- Source `repoInfo` from extras instead of changelog
- Pass `maintenanceStatus` (from composable) to `PackageBadges`

### PackageBadges component

- Accept new `maintenanceStatus` prop
- Render amber badge for `possibly_unmaintained`
- Render red badge for `likely_unmaintained`

## Testing

Unit tests in `web/app/composables/__tests__/useMaintenanceStatus.test.ts`:

| Case | Inputs | Expected |
|---|---|---|
| Active (recent release + recent commit) | 30 days, 15 days, not archived | undefined |
| Possibly unmaintained | 600 days release, 400 days commit, not archived | `possibly_unmaintained` |
| Likely unmaintained | 1200 days release, 800 days commit, not archived | `likely_unmaintained` |
| Archived repo | 30 days release, 15 days commit, archived | `likely_unmaintained` |
| No repo, recent release | 30 days, undefined commit, not archived | undefined |
| No repo, old release | 800 days, undefined commit, not archived | `possibly_unmaintained` |
| No repo, very old release | 1200 days, undefined commit, not archived | `likely_unmaintained` |
| Old release, recent commit | 600 days release, 15 days commit, not archived | undefined |
| Recent release, old commit | 30 days release, 400 days commit, not archived | undefined |

Pure function — no mocks, no I/O. Pass dates and assert output.

## Scope Exclusions

- No release frequency trend analysis (signal D) — kept simple for v1
- No tooltip or expanded explanation on the badge
- No historical tracking of maintenance status changes
- No GitLab support for archived flag (GitLab client would need similar extension later)
