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

All thresholds are top-level constants in `api/internal/enrichment/health.go` for easy tuning.

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

## Data Model

### New enrichment function

File: `api/internal/enrichment/health.go`

```go
type MaintenanceStatus string

const (
    MaintenanceActive               MaintenanceStatus = ""
    MaintenancePossiblyUnmaintained MaintenanceStatus = "possibly_unmaintained"
    MaintenanceLikelyUnmaintained   MaintenanceStatus = "likely_unmaintained"
)

const (
    PossiblyUnmaintainedReleaseDays = 548   // ~18 months
    PossiblyUnmaintainedCommitDays  = 365   // 12 months
    LikelyUnmaintainedReleaseDays   = 1095  // ~3 years
    LikelyUnmaintainedCommitDays    = 730   // ~2 years
    PossiblyUnmaintainedNoRepoDays  = 730   // 2 years (PyPI-only, conservative)
    LikelyUnmaintainedNoRepoDays    = 1095  // 3 years (PyPI-only)
)

func ComputeMaintenanceStatus(lastReleasedAt time.Time, lastCommitAt *time.Time, archived bool) MaintenanceStatus
```

### Logic

1. If `archived` is true, return `LikelyUnmaintained`
2. If `lastCommitAt` is non-nil (repo linked):
   - If release age >= `LikelyUnmaintainedReleaseDays` AND commit age >= `LikelyUnmaintainedCommitDays`, return `LikelyUnmaintained`
   - If release age >= `PossiblyUnmaintainedReleaseDays` AND commit age >= `PossiblyUnmaintainedCommitDays`, return `PossiblyUnmaintained`
3. If `lastCommitAt` is nil (no repo):
   - If release age >= `LikelyUnmaintainedNoRepoDays`, return `LikelyUnmaintained`
   - If release age >= `PossiblyUnmaintainedNoRepoDays`, return `PossiblyUnmaintained`
4. Otherwise, return `MaintenanceActive` (empty string)

## GitHub Client Change

Add `Archived` field to the existing `RepoInfo` struct:

```go
type RepoInfo struct {
    Stars        int    `json:"stargazers_count"`
    Forks        int    `json:"forks_count"`
    OpenIssues   int    `json:"open_issues_count"`
    LastPushedAt string `json:"pushed_at"`
    Archived     bool   `json:"archived"`
    Owner        Owner  `json:"owner"`
}
```

No new API calls — the GitHub repo endpoint already returns this field.

## Handler Integration

In `buildPackageResponse()` in `packages.go`, after existing enrichment calls:

```go
var maintenanceStatus enrichment.MaintenanceStatus
if releaseCadence.LastReleasedAt != "" {
    lastRelease, _ := time.Parse(time.RFC3339, releaseCadence.LastReleasedAt)
    var lastCommit *time.Time
    var archived bool
    if repoInfo != nil {
        if t, err := time.Parse(time.RFC3339, repoInfo.LastPushedAt); err == nil {
            lastCommit = &t
        }
        archived = repoInfo.Archived
    }
    maintenanceStatus = enrichment.ComputeMaintenanceStatus(lastRelease, lastCommit, archived)
}
```

Add to `PackageResponse` struct:

```go
MaintenanceStatus enrichment.MaintenanceStatus `json:"maintenance_status,omitempty"`
```

No new cache keys, endpoints, or TTL changes. Rides on existing package cache (1 hour TTL, stale-while-revalidate).

## Frontend

### TypeScript type

Add to `PackageData` in `web/app/types/api.ts`:

```typescript
maintenance_status?: 'possibly_unmaintained' | 'likely_unmaintained'
```

### UI

Conditional badge in `PackageBadges` component, following the existing badge pattern:

- `possibly_unmaintained` — yellow/amber badge: "Possibly Unmaintained"
- `likely_unmaintained` — red/orange badge: "Likely Unmaintained"

No badge rendered when field is absent (active packages). No tooltip or expanded explanation in v1.

## Testing

Unit tests in `api/internal/enrichment/health_test.go`:

| Case | Inputs | Expected |
|---|---|---|
| Active (recent release + recent commit) | 30 days, 15 days, not archived | no status |
| Possibly unmaintained | 600 days release, 400 days commit, not archived | `possibly_unmaintained` |
| Likely unmaintained | 1200 days release, 800 days commit, not archived | `likely_unmaintained` |
| Archived repo | 30 days release, 15 days commit, archived | `likely_unmaintained` |
| No repo, recent release | 30 days, nil commit, not archived | no status |
| No repo, old release | 800 days, nil commit, not archived | `possibly_unmaintained` |
| No repo, very old release | 1200 days, nil commit, not archived | `likely_unmaintained` |
| Old release, recent commit | 600 days release, 15 days commit, not archived | no status |
| Recent release, old commit | 30 days release, 400 days commit, not archived | no status |

Pure function — no mocks, no I/O. Pass dates and assert output.

## Scope Exclusions

- No release frequency trend analysis (signal D) — kept simple for v1
- No tooltip or expanded explanation on the badge
- No separate endpoint — computed inline during package enrichment
- No historical tracking of maintenance status changes
- No GitLab support for archived flag (GitLab client would need similar extension later)
