# Maintenance Status Detection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show a "Possibly Unmaintained" or "Likely Unmaintained" badge on package pages when signals indicate a package may no longer be actively maintained. Includes a refactor to move GitHub RepoInfo from the changelog endpoint to the extras endpoint.

**Architecture:** GitHub `RepoInfo` currently lives in the changelog response. We move it to the extras endpoint (which already serves supplementary package data in parallel), add the `archived` flag, then compute maintenance status on the frontend via a Vue composable. The composable combines SSR package data (`release_cadence`) with client-side extras data (`repo_info`).

**Tech Stack:** Go (GitHub client + extras handler changes), TypeScript/Vue 3 (composable + badge), Vitest (frontend tests)

---

### Task 1: Add `archived` field to GitHub client structs

**Files:**
- Modify: `api/internal/github/client.go`

- [ ] **Step 1: Add `Archived` to `ghRepo` struct**

In `api/internal/github/client.go`, find the `ghRepo` struct (line 122) and add the `Archived` field:

```go
type ghRepo struct {
	StargazersCount int    `json:"stargazers_count"`
	ForksCount      int    `json:"forks_count"`
	OpenIssuesCount int    `json:"open_issues_count"`
	PushedAt        string `json:"pushed_at"`
	Archived        bool   `json:"archived"`
	Owner           struct {
		Login     string `json:"login"`
		Type      string `json:"type"`
		AvatarURL string `json:"avatar_url"`
	} `json:"owner"`
}
```

- [ ] **Step 2: Add `Archived` to `RepoInfo` struct**

Find the `RepoInfo` struct (line 106) and add:

```go
type RepoInfo struct {
	Stars        int       `json:"stars"`
	Forks        int       `json:"forks"`
	OpenIssues   int       `json:"open_issues"`
	LastPushedAt string    `json:"last_pushed_at"`
	Archived     bool      `json:"archived"`
	Owner        RepoOwner `json:"owner"`
}
```

- [ ] **Step 3: Map `Archived` in `FetchRepoInfo` return**

In the `FetchRepoInfo` method (line 259), add the mapping:

```go
return &RepoInfo{
	Stars:        raw.StargazersCount,
	Forks:        raw.ForksCount,
	OpenIssues:   raw.OpenIssuesCount,
	LastPushedAt: raw.PushedAt,
	Archived:     raw.Archived,
	Owner: RepoOwner{
		Login:       raw.Owner.Login,
		AvatarURL:   raw.Owner.AvatarURL,
		DisplayName: displayName,
		URL:         ownerURL,
		IsOrg:       isOrg,
	},
}, nil
```

- [ ] **Step 4: Run Go tests**

Run: `cd api && go test ./internal/github/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/github/client.go
git commit -m "feat(api): add archived field to GitHub RepoInfo struct"
```

---

### Task 2: Move RepoInfo fetching from changelog handler to extras handler

**Files:**
- Modify: `api/internal/handler/extras.go`
- Modify: `api/internal/handler/changelog.go`
- Modify: `api/cmd/server/main.go`

- [ ] **Step 1: Add GitHub client and PackageHandler to ExtrasHandler**

In `api/internal/handler/extras.go`, update the imports, struct, and constructor:

```go
import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/conda"
	gh "github.com/pypx/api/internal/github"
	"github.com/pypx/api/internal/pypi"
)
```

Update the struct and constructor:

```go
// ExtrasHandler serves type support, conda-forge data, and GitHub repo info.
type ExtrasHandler struct {
	pypi   *pypi.Client
	conda  *conda.Client
	github *gh.Client
	pkg    *PackageHandler
	cache  cache.Cacher
}

// NewExtrasHandler creates a new ExtrasHandler.
func NewExtrasHandler(pypiClient *pypi.Client, condaClient *conda.Client, ghClient *gh.Client, pkgHandler *PackageHandler, c cache.Cacher) *ExtrasHandler {
	return &ExtrasHandler{pypi: pypiClient, conda: condaClient, github: ghClient, pkg: pkgHandler, cache: c}
}
```

- [ ] **Step 2: Add `RepoInfo` to `ExtrasResponse`**

```go
type ExtrasResponse struct {
	Package     string                `json:"package"`
	TypeSupport pypi.TypeSupport      `json:"type_support"`
	CondaForge  *conda.CondaForgeInfo `json:"conda_forge"`
	RepoInfo    *gh.RepoInfo          `json:"repo_info,omitempty"`
}
```

- [ ] **Step 3: Rewrite the `Get` method to fetch RepoInfo in parallel**

Replace the `Get` method body (after the cache check) with:

```go
	// Fetch PyPI package data for project URLs and py.typed check.
	pypiResp, pypiErr := h.pkg.FetchPackage(name)

	// Fetch type support, conda info, and repo info in parallel.
	var (
		typeSupport pypi.TypeSupport
		condaInfo   conda.CondaForgeInfo
		condaErr    error
		repoInfo    *gh.RepoInfo
		wg          sync.WaitGroup
	)

	wg.Add(3)
	go func() {
		defer wg.Done()
		typeSupport = pypi.CheckTypeSupport(h.pypi, name)
	}()
	go func() {
		defer wg.Done()
		condaInfo, condaErr = h.conda.FetchCondaInfo(name)
	}()
	go func() {
		defer wg.Done()
		if pypiErr != nil || h.github == nil {
			return
		}
		if owner, repo, ok := gh.ExtractGitHubRepo(pypiResp.Info.ProjectURLs); ok {
			repoInfo, _ = h.github.FetchRepoInfo(owner, repo)
		}
	}()
	wg.Wait()

	// If not already typed via stubs, check for py.typed marker in the wheel.
	if typeSupport.Status != "typed" && pypiErr == nil {
		typedKey := "typed:" + strings.ToLower(name) + ":" + pypiResp.Info.Version
		if data, _, err := h.cache.Get(typedKey, 0); err == nil && data != nil {
			if string(data) == "1" {
				typeSupport.Status = "typed"
			}
		} else {
			wheelURL := pypi.ExtractWheelURL(pypiResp.URLs)
			if wheelURL != "" && pypi.CheckPyTyped(h.pypi, wheelURL) {
				typeSupport.Status = "typed"
				h.cache.Set(typedKey, []byte("1"), 0) //nolint:errcheck
			} else {
				h.cache.Set(typedKey, []byte("0"), 0) //nolint:errcheck
			}
		}
	}

	resp := ExtrasResponse{
		Package:     name,
		TypeSupport: typeSupport,
		RepoInfo:    repoInfo,
	}
	if condaErr == nil {
		resp.CondaForge = &condaInfo
	}
```

Note: `FetchPackage` goes through the package handler (which has its own cache), so this doesn't add a redundant PyPI call. The repo info goroutine waits for `pypiResp` before extracting the GitHub repo — this is fine since `FetchPackage` is fast (cached).

- [ ] **Step 4: Remove `RepoInfo` from changelog handler**

In `api/internal/handler/changelog.go`, remove the `RepoInfo` field from `ChangelogResponse`:

```go
type ChangelogResponse struct {
	Package string            `json:"package"`
	Source  string            `json:"source"`
	RepoURL string            `json:"repo_url"`
	Entries []changelog.Entry `json:"entries"`
}
```

In the `buildResponse` method, remove the `FetchRepoInfo` call and the `RepoInfo` field from the return. The GitHub section (lines 112-117 and 124) should be removed:

```go
	if owner, repo, ok := gh.ExtractGitHubRepo(projectURLs); ok {
		repoURL := "https://github.com/" + owner + "/" + repo
		sources := []changelog.Source{
			&gh.ReleasesSource{Client: h.github, Owner: owner, Repo: repo},
			&gh.FileSource{Client: h.github, Owner: owner, Repo: repo},
			&gh.TagsSource{Client: h.github, Owner: owner, Repo: repo},
		}
		reg := changelog.NewRegistry(sources...)
		result := reg.Fetch(ctx)

		entries := renderHTML(result.Entries)

		return ChangelogResponse{
			Package: pkgName,
			Source:  result.Source,
			RepoURL: repoURL,
			Entries: entries,
		}
	}
```

Also remove the `"log"` import if it's only used for the repo info error log.

- [ ] **Step 5: Update `main.go` wiring**

In `api/cmd/server/main.go`, update the `NewExtrasHandler` call (line 72) to pass the GitHub client and package handler:

```go
extrasHandler := handler.NewExtrasHandler(pypiClient, condaClient, ghClient, pkgHandler, c)
```

- [ ] **Step 6: Run all Go tests**

Run: `cd api && go test ./...`
Expected: Compilation errors in extras tests (constructor signature changed) and changelog tests (RepoInfo field removed from response).

- [ ] **Step 7: Fix extras tests**

In `api/internal/handler/extras_test.go`, update both test functions. The `NewExtrasHandler` calls need the new parameters. Pass `nil` for the GitHub client and package handler since the existing tests don't need repo info:

Where the tests call:
```go
h := handler.NewExtrasHandler(pypiClient, condaClient, memCache)
```

Change to:
```go
h := handler.NewExtrasHandler(pypiClient, condaClient, nil, nil, memCache)
```

Update both occurrences (lines 51 and 132).

- [ ] **Step 8: Fix changelog tests**

In `api/internal/handler/changelog_test.go`, the `TestChangelogGet_GitHubReleases` test (line 25) checks `resp.RepoInfo` — remove that check. Find and remove any assertions on `RepoInfo` from all changelog tests.

Specifically in `TestChangelogGet_GitHubReleases`, the mock server handler for `/repos/owner/repo` (line 31-32) can remain since the changelog handler still uses the GitHub client for releases/tags/files. But remove any assertion like:
```go
if resp.RepoInfo == nil { ... }
```

The `/users/owner` handler (line 33-34) can also remain — it's used by `FetchRepoInfo` but also by release/tag processing if they need owner info.

- [ ] **Step 9: Run all Go tests again**

Run: `cd api && go test ./...`
Expected: PASS

- [ ] **Step 10: Commit**

```bash
git add api/internal/handler/extras.go api/internal/handler/changelog.go api/cmd/server/main.go api/internal/handler/extras_test.go api/internal/handler/changelog_test.go
git commit -m "refactor(api): move GitHub RepoInfo from changelog to extras endpoint"
```

---

### Task 3: Update frontend TypeScript types

**Files:**
- Modify: `web/app/types/api.ts`

- [ ] **Step 1: Add `archived` to `RepoInfo` interface**

In `web/app/types/api.ts`, update the `RepoInfo` interface (line 113):

```typescript
export interface RepoInfo {
  stars: number;
  forks: number;
  open_issues: number;
  last_pushed_at: string;
  archived?: boolean;
  owner: RepoOwner;
}
```

- [ ] **Step 2: Add `repo_info` to `ExtrasData` interface**

Find the `ExtrasData` interface and add `repo_info`:

```typescript
export interface ExtrasData {
  package: string;
  type_support: TypeSupport;
  conda_forge: CondaForgeInfo | null;
  repo_info?: RepoInfo;
}
```

- [ ] **Step 3: Remove `repo_info` from `ChangelogData` interface**

Update the `ChangelogData` interface (line 131) to remove `repo_info`:

```typescript
export interface ChangelogData {
  package: string;
  source: string;
  repo_url: string;
  entries: ChangelogEntry[];
}
```

- [ ] **Step 4: Commit**

```bash
git add web/app/types/api.ts
git commit -m "refactor(web): move repo_info type from ChangelogData to ExtrasData"
```

---

### Task 4: Update frontend to source repoInfo from extras

**Files:**
- Modify: `web/app/pages/packages/[name].vue`

- [ ] **Step 1: Change repoInfo source from changelog to extras**

In `web/app/pages/packages/[name].vue`, find line 37:

```typescript
const repoInfo = computed(() => changelog.value?.repo_info ?? null);
```

Replace with:

```typescript
const repoInfo = computed(() => extras.value?.repo_info ?? null);
```

- [ ] **Step 2: Run the frontend test suite**

Run: `cd web && npx vitest run`
Expected: PASS (no tests directly assert on where repoInfo comes from)

- [ ] **Step 3: Commit**

```bash
git add web/app/pages/packages/[name].vue
git commit -m "refactor(web): source repoInfo from extras instead of changelog"
```

---

### Task 5: Create `useMaintenanceStatus` composable with tests

**Files:**
- Create: `web/app/composables/useMaintenanceStatus.ts`
- Create: `web/app/composables/__tests__/useMaintenanceStatus.test.ts`

- [ ] **Step 1: Write the test file**

Create `web/app/composables/__tests__/useMaintenanceStatus.test.ts`:

```typescript
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { computeMaintenanceStatus, THRESHOLDS } from "../useMaintenanceStatus";

describe("computeMaintenanceStatus", () => {
  const NOW = new Date("2026-04-13T00:00:00Z");

  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  function daysAgo(days: number): string {
    const d = new Date(NOW);
    d.setDate(d.getDate() - days);
    return d.toISOString();
  }

  it("returns undefined for active package with repo", () => {
    const result = computeMaintenanceStatus(daysAgo(30), daysAgo(15), false);
    expect(result).toBeUndefined();
  });

  it("returns possibly_unmaintained when both thresholds exceeded", () => {
    const result = computeMaintenanceStatus(
      daysAgo(THRESHOLDS.POSSIBLY_UNMAINTAINED_RELEASE_DAYS + 1),
      daysAgo(THRESHOLDS.POSSIBLY_UNMAINTAINED_COMMIT_DAYS + 1),
      false,
    );
    expect(result).toBe("possibly_unmaintained");
  });

  it("returns likely_unmaintained when both high thresholds exceeded", () => {
    const result = computeMaintenanceStatus(
      daysAgo(THRESHOLDS.LIKELY_UNMAINTAINED_RELEASE_DAYS + 1),
      daysAgo(THRESHOLDS.LIKELY_UNMAINTAINED_COMMIT_DAYS + 1),
      false,
    );
    expect(result).toBe("likely_unmaintained");
  });

  it("returns likely_unmaintained immediately for archived repos", () => {
    const result = computeMaintenanceStatus(daysAgo(30), daysAgo(15), true);
    expect(result).toBe("likely_unmaintained");
  });

  it("returns undefined for no-repo active package", () => {
    const result = computeMaintenanceStatus(daysAgo(30), undefined, false);
    expect(result).toBeUndefined();
  });

  it("returns possibly_unmaintained for no-repo old release", () => {
    const result = computeMaintenanceStatus(
      daysAgo(THRESHOLDS.POSSIBLY_UNMAINTAINED_NO_REPO_DAYS + 1),
      undefined,
      false,
    );
    expect(result).toBe("possibly_unmaintained");
  });

  it("returns likely_unmaintained for no-repo very old release", () => {
    const result = computeMaintenanceStatus(
      daysAgo(THRESHOLDS.LIKELY_UNMAINTAINED_NO_REPO_DAYS + 1),
      undefined,
      false,
    );
    expect(result).toBe("likely_unmaintained");
  });

  it("returns undefined when release is old but commit is recent", () => {
    const result = computeMaintenanceStatus(
      daysAgo(THRESHOLDS.POSSIBLY_UNMAINTAINED_RELEASE_DAYS + 1),
      daysAgo(15),
      false,
    );
    expect(result).toBeUndefined();
  });

  it("returns undefined when commit is old but release is recent", () => {
    const result = computeMaintenanceStatus(
      daysAgo(30),
      daysAgo(THRESHOLDS.POSSIBLY_UNMAINTAINED_COMMIT_DAYS + 1),
      false,
    );
    expect(result).toBeUndefined();
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd web && npx vitest run app/composables/__tests__/useMaintenanceStatus.test.ts`
Expected: FAIL — module not found

- [ ] **Step 3: Write the composable**

Create `web/app/composables/useMaintenanceStatus.ts`:

```typescript
import type { PackageData, RepoInfo } from "~/types/api";

export type MaintenanceStatus = "possibly_unmaintained" | "likely_unmaintained";

export const THRESHOLDS = {
  POSSIBLY_UNMAINTAINED_RELEASE_DAYS: 548, // ~18 months
  POSSIBLY_UNMAINTAINED_COMMIT_DAYS: 365, // 12 months
  LIKELY_UNMAINTAINED_RELEASE_DAYS: 1095, // ~3 years
  LIKELY_UNMAINTAINED_COMMIT_DAYS: 730, // ~2 years
  POSSIBLY_UNMAINTAINED_NO_REPO_DAYS: 730, // 2 years (PyPI-only, conservative)
  LIKELY_UNMAINTAINED_NO_REPO_DAYS: 1095, // 3 years (PyPI-only)
} as const;

function daysBetween(dateStr: string): number {
  return (Date.now() - new Date(dateStr).getTime()) / (1000 * 60 * 60 * 24);
}

/**
 * Pure function that computes maintenance status from release and commit dates.
 * Returns undefined for active packages.
 */
export function computeMaintenanceStatus(
  lastReleasedAt: string | undefined,
  lastCommitAt: string | undefined,
  archived: boolean,
): MaintenanceStatus | undefined {
  if (!lastReleasedAt) return undefined;

  if (archived) return "likely_unmaintained";

  const releaseDays = daysBetween(lastReleasedAt);

  if (lastCommitAt !== undefined) {
    const commitDays = daysBetween(lastCommitAt);

    if (
      releaseDays >= THRESHOLDS.LIKELY_UNMAINTAINED_RELEASE_DAYS &&
      commitDays >= THRESHOLDS.LIKELY_UNMAINTAINED_COMMIT_DAYS
    ) {
      return "likely_unmaintained";
    }
    if (
      releaseDays >= THRESHOLDS.POSSIBLY_UNMAINTAINED_RELEASE_DAYS &&
      commitDays >= THRESHOLDS.POSSIBLY_UNMAINTAINED_COMMIT_DAYS
    ) {
      return "possibly_unmaintained";
    }
    return undefined;
  }

  // No repo: conservative PyPI-only thresholds
  if (releaseDays >= THRESHOLDS.LIKELY_UNMAINTAINED_NO_REPO_DAYS) {
    return "likely_unmaintained";
  }
  if (releaseDays >= THRESHOLDS.POSSIBLY_UNMAINTAINED_NO_REPO_DAYS) {
    return "possibly_unmaintained";
  }
  return undefined;
}

/**
 * Composable that returns a computed maintenance status for a package.
 */
export function useMaintenanceStatus(
  pkg: Ref<PackageData | null>,
  repoInfo: Ref<RepoInfo | null | undefined>,
) {
  return computed(() => {
    if (!pkg.value) return undefined;

    const lastReleasedAt = pkg.value.release_cadence?.last_released_at;
    const lastCommitAt = repoInfo.value?.last_pushed_at;
    const archived = repoInfo.value?.archived ?? false;

    return computeMaintenanceStatus(lastReleasedAt, lastCommitAt, archived);
  });
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd web && npx vitest run app/composables/__tests__/useMaintenanceStatus.test.ts`
Expected: PASS (all 9 tests)

- [ ] **Step 5: Commit**

```bash
git add web/app/composables/useMaintenanceStatus.ts web/app/composables/__tests__/useMaintenanceStatus.test.ts
git commit -m "feat(web): add useMaintenanceStatus composable with tests"
```

---

### Task 6: Add maintenance badge to PackageBadges component

**Files:**
- Modify: `web/app/components/PackageBadges.vue`
- Modify: `web/app/pages/packages/[name].vue`

- [ ] **Step 1: Update PackageBadges props**

In `web/app/components/PackageBadges.vue`, update the script section to accept the new prop:

```typescript
<script setup lang="ts">
import { computed } from "vue";
import type { PackageData, ExtrasData, SecurityData } from "~/types/api";
import type { MaintenanceStatus } from "~/composables/useMaintenanceStatus";

const props = defineProps<{
  pkg: PackageData;
  extras?: ExtrasData | null;
  security?: SecurityData | null;
  maintenanceStatus?: MaintenanceStatus;
}>();

function formatSize(bytes: number): string {
  if (bytes >= 1_048_576) return `${(bytes / 1_048_576).toFixed(1)} MB`;
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${bytes} B`;
}

const vulnCount = computed(() => props.security?.vulns?.length ?? 0);
const typeStatus = computed(() => props.extras?.type_support?.status);
const condaAvailable = computed(() => props.extras?.conda_forge?.available);
const condaUrl = computed(() => props.extras?.conda_forge?.url ?? null);
</script>
```

- [ ] **Step 2: Add maintenance badge to template**

In the template, after the security badge closing `</a>` tag (line 90), add before the closing `</div>`:

```vue
    <!-- Maintenance status badge -->
    <span
      v-if="maintenanceStatus === 'possibly_unmaintained'"
      class="inline-flex items-center gap-1 px-2 py-1 rounded text-xs font-medium ring-1 bg-amber-950 text-amber-300 ring-amber-800"
    >
      Possibly Unmaintained
    </span>
    <span
      v-if="maintenanceStatus === 'likely_unmaintained'"
      class="inline-flex items-center gap-1 px-2 py-1 rounded text-xs font-medium ring-1 bg-red-950 text-red-300 ring-red-800"
    >
      Likely Unmaintained
    </span>
```

- [ ] **Step 3: Wire up composable in package page**

In `web/app/pages/packages/[name].vue`, after the `repoInfo` computed (which was updated in Task 4), add:

```typescript
const maintenanceStatus = useMaintenanceStatus(pkg, repoInfo);
```

Then update the PackageBadges usage (line 110):

```vue
<PackageBadges :pkg="pkg" :extras="extras" :security="security" :maintenance-status="maintenanceStatus" />
```

- [ ] **Step 4: Run the full test suite**

Run: `cd web && npx vitest run`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/app/components/PackageBadges.vue web/app/pages/packages/[name].vue
git commit -m "feat(web): add maintenance status badge to package page"
```

---

### Task 7: Manual verification

**Files:** None (testing only)

- [ ] **Step 1: Start the full stack**

Run the API and frontend dev servers. Ensure the API has no GitHub token issues.

- [ ] **Step 2: Test an active package**

Navigate to a popular, actively maintained package (e.g., `requests`, `flask`). Verify:
- No maintenance badge is shown
- GitHub repo info (stars, forks, last commit) still appears in the sidebar
- All existing badges still work

- [ ] **Step 3: Test a likely unmaintained package**

Navigate to a package known to be abandoned (e.g., `nose` — last release 2015, or `pep8` — last release 2016). Verify the "Likely Unmaintained" badge appears in red.

- [ ] **Step 4: Test a package with no GitHub repo**

Navigate to a package without project URLs pointing to GitHub. Verify:
- No repo info in sidebar (as before)
- Badge logic works using PyPI-only thresholds

- [ ] **Step 5: Verify changelog still works**

Navigate to a package with a changelog and verify the changelog tab still renders correctly without the `repo_info` field.
