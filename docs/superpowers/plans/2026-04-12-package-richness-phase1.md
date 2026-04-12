# Package Richness Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enrich package pages with 8 new data dimensions: GitHub health signals, maintainers + org ownership, type support (stubs), security/vulnerabilities, wheel platform coverage, release cadence, documentation links, and conda-forge availability.

**Architecture:** New enrichment functions are added to `api/internal/enrichment/` for data derived from existing PyPI responses (zero extra fetches). New external clients (OSV, Conda) live in their own packages. Two new endpoints (`/security`, `/extras`) are added for data fetched out-of-band. The GitHub changelog handler is extended to also return repo health signals. The frontend fetches `/security` and `/extras` in parallel after page load, keeping the main package response fast.

**Tech Stack:** Go (chi, net/http, encoding/json), Vue 3 (script setup, Nuxt 4), Tailwind CSS 4. Tests use Go's `testing` + `net/http/httptest`. No new runtime dependencies.

**Spec:** `docs/superpowers/specs/2026-04-12-package-richness-design.md`

**Covers:** Features 1–6, 7a, 8. Features 7b (Rendered API Docs), 7c (Dash Export), 9 (Reverse Deps), and 10 (Similar Packages) are separate plans.

---

## File Map

**New Go files:**
- `api/internal/enrichment/platforms.go` — `ExtractPlatformCoverage(files []pypi.ReleaseFile) PlatformCoverage`
- `api/internal/enrichment/platforms_test.go`
- `api/internal/enrichment/cadence.go` — `ComputeReleaseCadence(releases map[string][]pypi.ReleaseFile) ReleaseCadence`
- `api/internal/enrichment/cadence_test.go`
- `api/internal/enrichment/maintainers.go` — `ParseMaintainers(info pypi.PackageInfo) []Maintainer`, `ExtractDocURL(projectURLs map[string]string) string`
- `api/internal/enrichment/maintainers_test.go`
- `api/internal/osv/client.go` — OSV.dev API client
- `api/internal/osv/client_test.go`
- `api/internal/conda/client.go` — conda-forge API client
- `api/internal/conda/client_test.go`
- `api/internal/pypi/stubs.go` — `CheckTypeSupport(c *Client, name string) TypeSupport`, `PackageExists(c *Client, name string) bool`
- `api/internal/pypi/stubs_test.go`
- `api/internal/handler/security.go` — `GET /api/packages/{name}/security`
- `api/internal/handler/security_test.go`
- `api/internal/handler/extras.go` — `GET /api/packages/{name}/extras` (type support + conda)
- `api/internal/handler/extras_test.go`

**Modified Go files:**
- `api/internal/pypi/client.go` — add `Maintainer`, `MaintainerEmail` fields to `PackageInfo`
- `api/internal/github/client.go` — add `RepoInfo`, `RepoOwner` types; `FetchRepoInfo`, `FetchOwnerName` methods
- `api/internal/github/client_test.go` — extend with RepoInfo tests
- `api/internal/handler/packages.go` — add `PlatformCoverage`, `ReleaseCadence`, `Maintainers`, `DocURL` to `PackageResponse`; wire in `buildPackageResponse`
- `api/internal/handler/packages_test.go` — update test fixture
- `api/internal/handler/changelog.go` — add `RepoInfo *github.RepoInfo` to `ChangelogResponse`; wire fetch
- `api/cmd/server/main.go` — add osv, conda clients; wire security + extras handlers + routes

**New Vue files:**
- `web/app/components/PackagePlatforms.vue` — platform coverage icon grid
- `web/app/components/PackageMaintainers.vue` — maintainer avatar list + org badge

**Modified Vue files:**
- `web/app/types/api.ts` — add all new interfaces + extend `PackageData`, `ChangelogData`
- `web/app/composables/useApi.ts` — add `fetchSecurity`, `fetchExtras`
- `web/app/components/PackageBadges.vue` — add type support + conda badges; security badge triggers from extras
- `web/app/components/PackageOverview.vue` — add doc link button, maintainers, GitHub signals, platform coverage, release cadence
- `web/app/pages/packages/[name].vue` — parallel-fetch security + extras

---

## Task 1: Wheel Platform Coverage Enrichment

**Files:**
- Create: `api/internal/enrichment/platforms.go`
- Create: `api/internal/enrichment/platforms_test.go`

- [ ] **Step 1: Write the failing test**

```go
// api/internal/enrichment/platforms_test.go
package enrichment_test

import (
	"testing"

	"github.com/pypx/api/internal/enrichment"
	"github.com/pypx/api/internal/pypi"
)

func TestExtractPlatformCoverage(t *testing.T) {
	tests := []struct {
		name  string
		files []pypi.ReleaseFile
		want  enrichment.PlatformCoverage
	}{
		{
			name: "pure python wheel",
			files: []pypi.ReleaseFile{
				{Filename: "requests-2.32.3-py3-none-any.whl", PackageType: "bdist_wheel"},
			},
			want: enrichment.PlatformCoverage{PurePython: true},
		},
		{
			name: "linux x86 manylinux",
			files: []pypi.ReleaseFile{
				{Filename: "numpy-2.0.0-cp312-cp312-manylinux_2_17_x86_64.manylinux2014_x86_64.whl", PackageType: "bdist_wheel"},
			},
			want: enrichment.PlatformCoverage{LinuxX86: true},
		},
		{
			name: "macos arm64",
			files: []pypi.ReleaseFile{
				{Filename: "cryptography-42.0.0-cp312-cp312-macosx_14_0_arm64.whl", PackageType: "bdist_wheel"},
			},
			want: enrichment.PlatformCoverage{MacOSARM64: true},
		},
		{
			name: "windows amd64",
			files: []pypi.ReleaseFile{
				{Filename: "psutil-5.9.8-cp312-cp312-win_amd64.whl", PackageType: "bdist_wheel"},
			},
			want: enrichment.PlatformCoverage{Windows: true},
		},
		{
			name: "musl linux",
			files: []pypi.ReleaseFile{
				{Filename: "cryptography-42.0.0-cp312-cp312-musllinux_1_2_x86_64.whl", PackageType: "bdist_wheel"},
			},
			want: enrichment.PlatformCoverage{Musl: true},
		},
		{
			name: "linux arm64 aarch64",
			files: []pypi.ReleaseFile{
				{Filename: "numpy-2.0.0-cp312-cp312-manylinux_2_17_aarch64.manylinux2014_aarch64.whl", PackageType: "bdist_wheel"},
			},
			want: enrichment.PlatformCoverage{LinuxARM64: true},
		},
		{
			name: "macos x86 intel",
			files: []pypi.ReleaseFile{
				{Filename: "cryptography-42.0.0-cp312-cp312-macosx_10_9_x86_64.whl", PackageType: "bdist_wheel"},
			},
			want: enrichment.PlatformCoverage{MacOSX86: true},
		},
		{
			name: "universal2 sets both macos",
			files: []pypi.ReleaseFile{
				{Filename: "pillow-10.0.0-cp312-cp312-macosx_10_10_universal2.whl", PackageType: "bdist_wheel"},
			},
			want: enrichment.PlatformCoverage{MacOSX86: true, MacOSARM64: true},
		},
		{
			name: "sdist only returns empty",
			files: []pypi.ReleaseFile{
				{Filename: "mypackage-1.0.0.tar.gz", PackageType: "sdist"},
			},
			want: enrichment.PlatformCoverage{},
		},
		{
			name: "multi-platform package",
			files: []pypi.ReleaseFile{
				{Filename: "cryptography-42.0.0-py3-none-any.whl", PackageType: "bdist_wheel"},
				{Filename: "cryptography-42.0.0-cp312-cp312-manylinux_2_17_x86_64.manylinux2014_x86_64.whl", PackageType: "bdist_wheel"},
				{Filename: "cryptography-42.0.0-cp312-cp312-win_amd64.whl", PackageType: "bdist_wheel"},
			},
			want: enrichment.PlatformCoverage{PurePython: true, LinuxX86: true, Windows: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := enrichment.ExtractPlatformCoverage(tt.files)
			if got != tt.want {
				t.Errorf("ExtractPlatformCoverage() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd api && go test ./internal/enrichment/... -run TestExtractPlatformCoverage -v
```

Expected: `FAIL — undefined: enrichment.PlatformCoverage`

- [ ] **Step 3: Implement platform coverage**

```go
// api/internal/enrichment/platforms.go
package enrichment

import (
	"strings"

	"github.com/pypx/api/internal/pypi"
)

// PlatformCoverage indicates which OS/architecture combinations have prebuilt
// wheels for a release. All fields are false for sdist-only packages.
type PlatformCoverage struct {
	PurePython bool `json:"pure_python"`
	LinuxX86   bool `json:"linux_x86_64"`
	LinuxARM64 bool `json:"linux_arm64"`
	MacOSX86   bool `json:"macos_x86_64"`
	MacOSARM64 bool `json:"macos_arm64"`
	Windows    bool `json:"windows_x86_64"`
	Musl       bool `json:"musl"`
}

// ExtractPlatformCoverage parses wheel filenames from the provided release
// files and returns which platforms have prebuilt wheels available.
func ExtractPlatformCoverage(files []pypi.ReleaseFile) PlatformCoverage {
	var cov PlatformCoverage
	for _, f := range files {
		if f.PackageType != "bdist_wheel" {
			continue
		}
		platform := extractPlatformTag(f.Filename)
		if platform == "" {
			continue
		}
		p := strings.ToLower(platform)
		switch {
		case p == "any":
			cov.PurePython = true
		case strings.Contains(p, "manylinux") && strings.Contains(p, "x86_64"):
			cov.LinuxX86 = true
		case strings.Contains(p, "manylinux") && strings.Contains(p, "aarch64"):
			cov.LinuxARM64 = true
		case strings.Contains(p, "musllinux"):
			cov.Musl = true
		case strings.Contains(p, "macosx") && strings.Contains(p, "arm64"):
			cov.MacOSARM64 = true
		case strings.Contains(p, "macosx") && strings.Contains(p, "universal2"):
			cov.MacOSX86 = true
			cov.MacOSARM64 = true
		case strings.Contains(p, "macosx") && strings.Contains(p, "x86_64"):
			cov.MacOSX86 = true
		case strings.Contains(p, "win_amd64") || strings.Contains(p, "win32"):
			cov.Windows = true
		}
	}
	return cov
}

// extractPlatformTag returns the platform segment of a wheel filename.
// Wheel format: {name}-{ver}(-{build})?-{python}-{abi}-{platform}.whl
func extractPlatformTag(filename string) string {
	name := strings.TrimSuffix(filename, ".whl")
	parts := strings.Split(name, "-")
	if len(parts) < 5 {
		return ""
	}
	return parts[len(parts)-1]
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd api && go test ./internal/enrichment/... -run TestExtractPlatformCoverage -v
```

Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
cd api && git add internal/enrichment/platforms.go internal/enrichment/platforms_test.go
git commit -m "feat(api): add wheel platform coverage enrichment"
```

---

## Task 2: Release Cadence Enrichment

**Files:**
- Create: `api/internal/enrichment/cadence.go`
- Create: `api/internal/enrichment/cadence_test.go`

- [ ] **Step 1: Write the failing test**

```go
// api/internal/enrichment/cadence_test.go
package enrichment_test

import (
	"testing"

	"github.com/pypx/api/internal/enrichment"
	"github.com/pypx/api/internal/pypi"
)

func TestComputeReleaseCadence(t *testing.T) {
	releases := map[string][]pypi.ReleaseFile{
		"1.0.0": {{UploadTime: "2023-01-15T10:00:00Z", PackageType: "bdist_wheel"}},
		"1.1.0": {{UploadTime: "2023-04-20T10:00:00Z", PackageType: "bdist_wheel"}},
		"1.2.0": {{UploadTime: "2023-07-10T10:00:00Z", PackageType: "bdist_wheel"}},
		"2.0.0": {{UploadTime: "2025-01-01T10:00:00Z", PackageType: "bdist_wheel"}},
		"2.1.0": {{UploadTime: "2025-03-01T10:00:00Z", PackageType: "bdist_wheel"}},
	}

	cadence := enrichment.ComputeReleaseCadence(releases)

	if cadence.LastReleasedAt == "" {
		t.Error("LastReleasedAt should not be empty")
	}
	if cadence.ReleasesLast12Mo < 1 {
		t.Errorf("ReleasesLast12Mo = %d, want >= 1", cadence.ReleasesLast12Mo)
	}
	if cadence.AvgDaysBetweenReleases <= 0 {
		t.Errorf("AvgDaysBetweenReleases = %f, want > 0", cadence.AvgDaysBetweenReleases)
	}
	if len(cadence.QuarterlyCounts) == 0 {
		t.Error("QuarterlyCounts should not be empty")
	}
}

func TestComputeReleaseCadenceEmpty(t *testing.T) {
	cadence := enrichment.ComputeReleaseCadence(map[string][]pypi.ReleaseFile{})
	if cadence.LastReleasedAt != "" {
		t.Errorf("expected empty LastReleasedAt for empty releases, got %q", cadence.LastReleasedAt)
	}
	if cadence.AvgDaysBetweenReleases != 0 {
		t.Errorf("expected 0 avg days for empty releases, got %f", cadence.AvgDaysBetweenReleases)
	}
}

func TestComputeReleaseCadenceSingleRelease(t *testing.T) {
	releases := map[string][]pypi.ReleaseFile{
		"1.0.0": {{UploadTime: "2025-01-01T10:00:00Z", PackageType: "bdist_wheel"}},
	}
	cadence := enrichment.ComputeReleaseCadence(releases)
	if cadence.AvgDaysBetweenReleases != 0 {
		t.Errorf("single release: avg days should be 0, got %f", cadence.AvgDaysBetweenReleases)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd api && go test ./internal/enrichment/... -run TestComputeReleaseCadence -v
```

Expected: `FAIL — undefined: enrichment.ComputeReleaseCadence`

- [ ] **Step 3: Implement release cadence**

```go
// api/internal/enrichment/cadence.go
package enrichment

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/pypx/api/internal/pypi"
)

// ReleaseCadence summarises how often a package publishes new versions.
type ReleaseCadence struct {
	ReleasesLast12Mo       int            `json:"releases_last_12mo"`
	AvgDaysBetweenReleases float64        `json:"avg_days_between_releases"`
	LastReleasedAt         string         `json:"last_released_at"`
	QuarterlyCounts        []QuarterCount `json:"quarterly_counts"`
}

// QuarterCount holds the number of releases in a calendar quarter.
type QuarterCount struct {
	Quarter string `json:"quarter"` // e.g. "2025 Q1"
	Count   int    `json:"count"`
}

// ComputeReleaseCadence derives release frequency from all version files.
// Versions with no parseable upload_time are silently skipped.
func ComputeReleaseCadence(releases map[string][]pypi.ReleaseFile) ReleaseCadence {
	// Collect one timestamp per version (first parseable file wins).
	var times []time.Time
	for _, files := range releases {
		for _, f := range files {
			if f.UploadTime == "" {
				continue
			}
			t, err := time.Parse(time.RFC3339Nano, f.UploadTime)
			if err != nil {
				t, err = time.Parse(time.RFC3339, f.UploadTime)
			}
			if err == nil {
				times = append(times, t)
				break
			}
		}
	}

	if len(times) == 0 {
		return ReleaseCadence{}
	}

	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })

	now := time.Now()
	oneYearAgo := now.AddDate(-1, 0, 0)
	twoYearsAgo := now.AddDate(-2, 0, 0)

	var cadence ReleaseCadence
	cadence.LastReleasedAt = times[len(times)-1].UTC().Format(time.RFC3339)

	for _, t := range times {
		if t.After(oneYearAgo) {
			cadence.ReleasesLast12Mo++
		}
	}

	if len(times) >= 2 {
		totalDays := times[len(times)-1].Sub(times[0]).Hours() / 24
		avg := totalDays / float64(len(times)-1)
		cadence.AvgDaysBetweenReleases = math.Round(avg*10) / 10
	}

	// Build quarterly counts covering the last 2 years, in chronological order.
	quarterMap := make(map[string]int)
	for _, t := range times {
		if t.Before(twoYearsAgo) {
			continue
		}
		q := quarterLabel(t)
		quarterMap[q]++
	}

	for t := startOfQuarter(twoYearsAgo); !t.After(now); t = t.AddDate(0, 3, 0) {
		q := quarterLabel(t)
		cadence.QuarterlyCounts = append(cadence.QuarterlyCounts, QuarterCount{
			Quarter: q,
			Count:   quarterMap[q],
		})
	}

	return cadence
}

func quarterLabel(t time.Time) string {
	q := (int(t.Month())-1)/3 + 1
	return fmt.Sprintf("%d Q%d", t.Year(), q)
}

func startOfQuarter(t time.Time) time.Time {
	month := ((int(t.Month()) - 1) / 3) * 3 + 1
	return time.Date(t.Year(), time.Month(month), 1, 0, 0, 0, 0, time.UTC)
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd api && go test ./internal/enrichment/... -run TestComputeReleaseCadence -v
```

Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
cd api && git add internal/enrichment/cadence.go internal/enrichment/cadence_test.go
git commit -m "feat(api): add release cadence enrichment"
```

---

## Task 3: Maintainers + Doc URL Enrichment

**Files:**
- Modify: `api/internal/pypi/client.go` — add `Maintainer`, `MaintainerEmail` to `PackageInfo`
- Create: `api/internal/enrichment/maintainers.go`
- Create: `api/internal/enrichment/maintainers_test.go`

- [ ] **Step 1: Add missing fields to PackageInfo**

In `api/internal/pypi/client.go`, add `Maintainer` and `MaintainerEmail` after the `AuthorEmail` line (line 44):

```go
// Before:
AuthorEmail       string            `json:"author_email"`
HomePage          string            `json:"home_page"`

// After:
AuthorEmail       string            `json:"author_email"`
Maintainer        string            `json:"maintainer"`
MaintainerEmail   string            `json:"maintainer_email"`
HomePage          string            `json:"home_page"`
```

- [ ] **Step 2: Write the failing test**

```go
// api/internal/enrichment/maintainers_test.go
package enrichment_test

import (
	"testing"

	"github.com/pypx/api/internal/enrichment"
	"github.com/pypx/api/internal/pypi"
)

func TestParseMaintainers(t *testing.T) {
	tests := []struct {
		name string
		info pypi.PackageInfo
		want []enrichment.Maintainer
	}{
		{
			name: "rfc2822 author email with name",
			info: pypi.PackageInfo{
				AuthorEmail: "Kenneth Reitz <me@kennethreitz.org>",
			},
			want: []enrichment.Maintainer{
				{Name: "Kenneth Reitz", Email: "me@kennethreitz.org"},
			},
		},
		{
			name: "multiple authors in email field",
			info: pypi.PackageInfo{
				AuthorEmail: "Alice <alice@example.com>, Bob <bob@example.com>",
			},
			want: []enrichment.Maintainer{
				{Name: "Alice", Email: "alice@example.com"},
				{Name: "Bob", Email: "bob@example.com"},
			},
		},
		{
			name: "bare email only",
			info: pypi.PackageInfo{
				AuthorEmail: "someone@example.com",
			},
			want: []enrichment.Maintainer{
				{Email: "someone@example.com"},
			},
		},
		{
			name: "separate author name field",
			info: pypi.PackageInfo{
				Author:      "Guido van Rossum",
				AuthorEmail: "",
			},
			want: []enrichment.Maintainer{
				{Name: "Guido van Rossum"},
			},
		},
		{
			name: "maintainer fields used when author is empty",
			info: pypi.PackageInfo{
				Author:           "",
				AuthorEmail:      "",
				Maintainer:       "Django Software Foundation",
				MaintainerEmail:  "foundation@djangoproject.com",
			},
			want: []enrichment.Maintainer{
				{Name: "Django Software Foundation", Email: "foundation@djangoproject.com"},
			},
		},
		{
			name: "empty fields returns nil",
			info: pypi.PackageInfo{},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := enrichment.ParseMaintainers(tt.info)
			if len(got) != len(tt.want) {
				t.Fatalf("ParseMaintainers() returned %d items, want %d\ngot:  %+v\nwant: %+v",
					len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("item %d: got %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtractDocURL(t *testing.T) {
	tests := []struct {
		name        string
		projectURLs map[string]string
		want        string
	}{
		{
			name:        "documentation key",
			projectURLs: map[string]string{"Documentation": "https://docs.example.com"},
			want:        "https://docs.example.com",
		},
		{
			name:        "case insensitive docs key",
			projectURLs: map[string]string{"docs": "https://docs.example.com"},
			want:        "https://docs.example.com",
		},
		{
			name:        "no doc url returns empty",
			projectURLs: map[string]string{"Source": "https://github.com/example/pkg"},
			want:        "",
		},
		{
			name:        "nil map returns empty",
			projectURLs: nil,
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := enrichment.ExtractDocURL(tt.projectURLs)
			if got != tt.want {
				t.Errorf("ExtractDocURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd api && go test ./internal/enrichment/... -run "TestParseMaintainers|TestExtractDocURL" -v
```

Expected: `FAIL — undefined: enrichment.Maintainer`

- [ ] **Step 4: Implement maintainers + doc URL**

```go
// api/internal/enrichment/maintainers.go
package enrichment

import (
	"regexp"
	"strings"

	"github.com/pypx/api/internal/pypi"
)

// Maintainer represents a single package author or maintainer.
type Maintainer struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

var angleEmailRe = regexp.MustCompile(`^(.*?)\s*<([^>]+)>\s*$`)

// ParseMaintainers extracts a structured list of maintainers from PyPI
// package metadata. It prefers the author_email / maintainer_email fields
// (RFC 2822 "Name <addr>" format), falling back to the plain name fields.
func ParseMaintainers(info pypi.PackageInfo) []Maintainer {
	// Try author_email first (most packages put names+emails here).
	if info.AuthorEmail != "" {
		m := parseMaintainersFromEmailField(info.AuthorEmail, info.Author)
		if len(m) > 0 {
			return m
		}
	}

	// Fall back to plain author name.
	if info.Author != "" {
		var result []Maintainer
		for _, name := range splitNames(info.Author) {
			result = append(result, Maintainer{Name: name})
		}
		return result
	}

	// Try maintainer fields.
	if info.MaintainerEmail != "" {
		m := parseMaintainersFromEmailField(info.MaintainerEmail, info.Maintainer)
		if len(m) > 0 {
			return m
		}
	}

	if info.Maintainer != "" {
		var result []Maintainer
		for _, name := range splitNames(info.Maintainer) {
			result = append(result, Maintainer{Name: name})
		}
		return result
	}

	return nil
}

// parseMaintainersFromEmailField parses a comma-separated list of
// "Name <email>" entries. nameField is used as a fallback when the email
// entry contains no name portion.
func parseMaintainersFromEmailField(emailField, nameField string) []Maintainer {
	entries := splitEmailList(emailField)
	names := splitNames(nameField)

	var result []Maintainer
	for i, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		m := parseOneEntry(entry)
		// If the email field didn't include a name, use the positional name.
		if m.Name == "" && i < len(names) {
			m.Name = names[i]
		}
		if m.Name != "" || m.Email != "" {
			result = append(result, m)
		}
	}
	return result
}

// splitEmailList splits on commas that are not inside angle brackets.
func splitEmailList(s string) []string {
	var parts []string
	depth, start := 0, 0
	for i, ch := range s {
		switch ch {
		case '<':
			depth++
		case '>':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, s[start:])
}

// splitNames splits a plain name field on commas or semicolons.
func splitNames(s string) []string {
	if s == "" {
		return nil
	}
	var names []string
	for _, n := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' }) {
		n = strings.TrimSpace(n)
		if n != "" {
			names = append(names, n)
		}
	}
	return names
}

func parseOneEntry(s string) Maintainer {
	if m := angleEmailRe.FindStringSubmatch(s); m != nil {
		return Maintainer{
			Name:  strings.TrimSpace(m[1]),
			Email: strings.TrimSpace(m[2]),
		}
	}
	if strings.Contains(s, "@") {
		return Maintainer{Email: s}
	}
	return Maintainer{Name: s}
}

// docURLKeys are checked in priority order against project_urls keys
// (case-insensitive) to find the documentation URL.
var docURLKeys = []string{"documentation", "docs", "doc"}

// ExtractDocURL returns the documentation URL from a project_urls map,
// or an empty string if no documentation link is found.
func ExtractDocURL(projectURLs map[string]string) string {
	if len(projectURLs) == 0 {
		return ""
	}
	lower := make(map[string]string, len(projectURLs))
	for k, v := range projectURLs {
		lower[strings.ToLower(k)] = v
	}
	for _, key := range docURLKeys {
		if url, ok := lower[key]; ok {
			return url
		}
	}
	return ""
}
```

- [ ] **Step 5: Run test to verify it passes**

```bash
cd api && go test ./internal/enrichment/... -run "TestParseMaintainers|TestExtractDocURL" -v
```

Expected: `PASS`

- [ ] **Step 6: Commit**

```bash
cd api && git add internal/pypi/client.go internal/enrichment/maintainers.go internal/enrichment/maintainers_test.go
git commit -m "feat(api): add maintainers parsing and doc URL extraction"
```

---

## Task 4: GitHub Repo Health Signals

**Files:**
- Modify: `api/internal/github/client.go`
- Modify: `api/internal/github/client_test.go`

- [ ] **Step 1: Write failing tests**

Add to `api/internal/github/client_test.go`:

```go
func TestFetchRepoInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/psf/requests":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{
				"stargazers_count": 52000,
				"forks_count": 9500,
				"open_issues_count": 180,
				"pushed_at": "2025-03-01T10:00:00Z",
				"owner": {
					"login": "psf",
					"type": "Organization",
					"avatar_url": "https://avatars.githubusercontent.com/u/1"
				}
			}`)
		case "/orgs/psf":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"login": "psf", "name": "Python Software Foundation"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := github.NewClient(github.WithBaseURL(srv.URL))
	info, err := c.FetchRepoInfo("psf", "requests")
	if err != nil {
		t.Fatalf("FetchRepoInfo() error: %v", err)
	}
	if info.Stars != 52000 {
		t.Errorf("Stars = %d, want 52000", info.Stars)
	}
	if info.Owner.Login != "psf" {
		t.Errorf("Owner.Login = %q, want %q", info.Owner.Login, "psf")
	}
	if info.Owner.DisplayName != "Python Software Foundation" {
		t.Errorf("Owner.DisplayName = %q, want %q", info.Owner.DisplayName, "Python Software Foundation")
	}
	if info.Owner.IsOrg != true {
		t.Error("Owner.IsOrg should be true for Organization type")
	}
}

func TestFetchRepoInfoNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := github.NewClient(github.WithBaseURL(srv.URL))
	info, err := c.FetchRepoInfo("missing", "repo")
	if err != nil {
		t.Fatalf("FetchRepoInfo() on 404 should not error: %v", err)
	}
	if info != nil {
		t.Errorf("FetchRepoInfo() on 404 should return nil, got %+v", info)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd api && go test ./internal/github/... -run "TestFetchRepoInfo" -v
```

Expected: `FAIL — undefined: c.FetchRepoInfo`

- [ ] **Step 3: Add RepoInfo types and FetchRepoInfo to github/client.go**

Add the following to `api/internal/github/client.go` after the existing types (after line 91):

```go
// RepoOwner holds the GitHub account (user or org) that owns a repository.
type RepoOwner struct {
	Login       string `json:"login"`
	AvatarURL   string `json:"avatar_url"`
	DisplayName string `json:"display_name"` // org/user name, fetched separately
	URL         string `json:"url"`
	IsOrg       bool   `json:"is_org"`
}

// RepoInfo holds health signals for a GitHub repository.
type RepoInfo struct {
	Stars       int       `json:"stars"`
	Forks       int       `json:"forks"`
	OpenIssues  int       `json:"open_issues"`
	LastPushedAt string   `json:"last_pushed_at"`
	Owner       RepoOwner `json:"owner"`
}

// ghRepo is the internal JSON shape from GET /repos/{owner}/{repo}.
type ghRepo struct {
	StargazersCount int    `json:"stargazers_count"`
	ForksCount      int    `json:"forks_count"`
	OpenIssuesCount int    `json:"open_issues_count"`
	PushedAt        string `json:"pushed_at"`
	Owner           struct {
		Login     string `json:"login"`
		Type      string `json:"type"`
		AvatarURL string `json:"avatar_url"`
	} `json:"owner"`
}

// ghOrgUser is the internal JSON shape from GET /orgs/{login} or /users/{login}.
type ghOrgUser struct {
	Name string `json:"name"`
}

// FetchRepoInfo retrieves health signals for owner/repo.
// Returns nil (no error) on 404 or 403 — the package simply has no GitHub repo.
func (c *Client) FetchRepoInfo(owner, repo string) (*RepoInfo, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, repo)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("github: build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: repo request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: unexpected status %d for %s/%s", resp.StatusCode, owner, repo)
	}

	var raw ghRepo
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("github: decode repo: %w", err)
	}

	isOrg := strings.EqualFold(raw.Owner.Type, "Organization")
	displayName := c.fetchOwnerName(raw.Owner.Login, isOrg)

	ownerURL := "https://github.com/" + raw.Owner.Login
	return &RepoInfo{
		Stars:        raw.StargazersCount,
		Forks:        raw.ForksCount,
		OpenIssues:   raw.OpenIssuesCount,
		LastPushedAt: raw.PushedAt,
		Owner: RepoOwner{
			Login:       raw.Owner.Login,
			AvatarURL:   raw.Owner.AvatarURL,
			DisplayName: displayName,
			URL:         ownerURL,
			IsOrg:       isOrg,
		},
	}, nil
}

// fetchOwnerName calls /orgs/{login} or /users/{login} to get the display name.
// Returns login as fallback if the request fails.
func (c *Client) fetchOwnerName(login string, isOrg bool) string {
	path := "users"
	if isOrg {
		path = "orgs"
	}
	url := fmt.Sprintf("%s/%s/%s", c.baseURL, path, login)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return login
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return login
	}
	defer resp.Body.Close()

	var ou ghOrgUser
	if err := json.NewDecoder(resp.Body).Decode(&ou); err != nil || ou.Name == "" {
		return login
	}
	return ou.Name
}
```

Also add `"strings"` to the import block if not already present (it is — `strings` is already imported).

- [ ] **Step 4: Run test to verify it passes**

```bash
cd api && go test ./internal/github/... -run "TestFetchRepoInfo" -v
```

Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
cd api && git add internal/github/client.go internal/github/client_test.go
git commit -m "feat(api): add GitHub repo health signals (stars, forks, issues, org)"
```

---

## Task 5: OSV Security Client + Handler

**Files:**
- Create: `api/internal/osv/client.go`
- Create: `api/internal/osv/client_test.go`
- Create: `api/internal/handler/security.go`
- Create: `api/internal/handler/security_test.go`

- [ ] **Step 1: Write failing OSV client test**

```go
// api/internal/osv/client_test.go
package osv_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pypx/api/internal/osv"
)

func TestFetchVulns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/query" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"vulns": [{
				"id": "GHSA-9wx4-h78v-vm56",
				"summary": "Requests session cookies can leak",
				"severity": [{"type": "CVSS_V3", "score": "MEDIUM"}],
				"affected": [{
					"ranges": [{"type": "ECOSYSTEM", "events": [{"introduced": "0"}, {"fixed": "2.32.0"}]}]
				}],
				"references": [{"url": "https://github.com/advisories/GHSA-9wx4-h78v-vm56"}]
			}]
		}`)
	}))
	defer srv.Close()

	c := osv.NewClient(osv.WithBaseURL(srv.URL))
	vulns, err := c.FetchVulns("requests")
	if err != nil {
		t.Fatalf("FetchVulns() error: %v", err)
	}
	if len(vulns) != 1 {
		t.Fatalf("expected 1 vuln, got %d", len(vulns))
	}
	if vulns[0].ID != "GHSA-9wx4-h78v-vm56" {
		t.Errorf("vuln ID = %q, want GHSA-9wx4-h78v-vm56", vulns[0].ID)
	}
	if vulns[0].Severity != "MEDIUM" {
		t.Errorf("severity = %q, want MEDIUM", vulns[0].Severity)
	}
	if vulns[0].URL == "" {
		t.Error("vuln URL should not be empty")
	}
}

func TestFetchVulnsNoVulns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	c := osv.NewClient(osv.WithBaseURL(srv.URL))
	vulns, err := c.FetchVulns("safe-package")
	if err != nil {
		t.Fatalf("FetchVulns() error: %v", err)
	}
	if len(vulns) != 0 {
		t.Errorf("expected 0 vulns, got %d", len(vulns))
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
cd api && go test ./internal/osv/... -v
```

Expected: `FAIL — no Go files in .../osv`

- [ ] **Step 3: Implement OSV client**

```go
// api/internal/osv/client.go
package osv

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// VulnInfo is a single vulnerability record from OSV.
type VulnInfo struct {
	ID            string `json:"id"`
	Summary       string `json:"summary"`
	Severity      string `json:"severity"`
	AffectedRange string `json:"affected_range"`
	FixedIn       string `json:"fixed_in,omitempty"`
	URL           string `json:"url"`
}

// osvQueryRequest is the POST body sent to the OSV API.
type osvQueryRequest struct {
	Package struct {
		Name      string `json:"name"`
		Ecosystem string `json:"ecosystem"`
	} `json:"package"`
}

// osvResponse is the top-level JSON response from OSV.
type osvResponse struct {
	Vulns []struct {
		ID       string `json:"id"`
		Summary  string `json:"summary"`
		Severity []struct {
			Type  string `json:"type"`
			Score string `json:"score"`
		} `json:"severity"`
		Affected []struct {
			Ranges []struct {
				Type   string `json:"type"`
				Events []struct {
					Introduced string `json:"introduced,omitempty"`
					Fixed      string `json:"fixed,omitempty"`
				} `json:"events"`
			} `json:"ranges"`
		} `json:"affected"`
		References []struct {
			URL string `json:"url"`
		} `json:"references"`
	} `json:"vulns"`
}

// Client queries the OSV.dev vulnerability database.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the OSV API base URL (for testing).
func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = url }
}

// NewClient creates a new OSV client.
func NewClient(opts ...Option) *Client {
	c := &Client{
		baseURL:    "https://api.osv.dev",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// FetchVulns returns all known vulnerabilities for the named PyPI package.
func (c *Client) FetchVulns(name string) ([]VulnInfo, error) {
	var body osvQueryRequest
	body.Package.Name = name
	body.Package.Ecosystem = "PyPI"

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("osv: encode request: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/v1/query", "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("osv: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("osv: unexpected status %d", resp.StatusCode)
	}

	var result osvResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("osv: decode response: %w", err)
	}

	vulns := make([]VulnInfo, 0, len(result.Vulns))
	for _, v := range result.Vulns {
		vi := VulnInfo{
			ID:      v.ID,
			Summary: v.Summary,
		}

		// Extract severity from the first entry.
		if len(v.Severity) > 0 {
			vi.Severity = v.Severity[0].Score
		}
		if vi.Severity == "" {
			vi.Severity = "Unknown"
		}

		// Extract affected range and fixed version from the first ECOSYSTEM range.
		if len(v.Affected) > 0 {
			for _, r := range v.Affected[0].Ranges {
				if r.Type != "ECOSYSTEM" {
					continue
				}
				var introduced, fixed string
				for _, e := range r.Events {
					if e.Introduced != "" {
						introduced = e.Introduced
					}
					if e.Fixed != "" {
						fixed = e.Fixed
					}
				}
				if introduced != "" {
					vi.AffectedRange = ">=" + introduced
					if fixed != "" {
						vi.AffectedRange += ", <" + fixed
					}
				}
				if fixed != "" {
					vi.FixedIn = fixed
				}
				break
			}
		}

		// Use first reference URL.
		if len(v.References) > 0 {
			vi.URL = v.References[0].URL
		}

		vulns = append(vulns, vi)
	}
	return vulns, nil
}
```

- [ ] **Step 4: Write failing security handler test**

```go
// api/internal/handler/security_test.go
package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/osv"
)

func TestSecurityHandlerGet(t *testing.T) {
	osvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"vulns":[{"id":"GHSA-test","summary":"Test vuln","severity":[{"type":"CVSS_V3","score":"HIGH"}],"references":[{"url":"https://example.com"}]}]}`)
	}))
	defer osvSrv.Close()

	memCache := cache.NewMemoryCache(newTestSQLiteCache(t), 100)
	osvClient := osv.NewClient(osv.WithBaseURL(osvSrv.URL))
	h := handler.NewSecurityHandler(osvClient, memCache)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/security", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/packages/requests/security", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp handler.SecurityResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Package != "requests" {
		t.Errorf("package = %q, want requests", resp.Package)
	}
	if len(resp.Vulns) != 1 {
		t.Errorf("vulns count = %d, want 1", len(resp.Vulns))
	}
}

func TestSecurityHandlerInvalidName(t *testing.T) {
	memCache := cache.NewMemoryCache(newTestSQLiteCache(t), 100)
	osvClient := osv.NewClient()
	h := handler.NewSecurityHandler(osvClient, memCache)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/security", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/packages/../etc/security", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Error("expected non-200 for invalid package name")
	}
}
```

Note: `newTestSQLiteCache` is a helper already used in `packages_test.go` — use the same pattern (create a temp SQLite cache).

- [ ] **Step 5: Implement security handler**

```go
// api/internal/handler/security.go
package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/osv"
	"github.com/pypx/api/internal/pypi"
)

const securityTTL = 24 * time.Hour

// SecurityResponse is the response for GET /api/packages/{name}/security.
type SecurityResponse struct {
	Package   string         `json:"package"`
	Vulns     []osv.VulnInfo `json:"vulns"`
	CheckedAt string         `json:"checked_at"`
}

// SecurityHandler serves vulnerability data from OSV.dev.
type SecurityHandler struct {
	osv   *osv.Client
	cache cache.Cacher
}

// NewSecurityHandler creates a new SecurityHandler.
func NewSecurityHandler(osvClient *osv.Client, c cache.Cacher) *SecurityHandler {
	return &SecurityHandler{osv: osvClient, cache: c}
}

// Get handles GET /api/packages/{name}/security.
func (h *SecurityHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := pypi.ValidateName(name); err != nil {
		http.Error(w, "invalid package name", http.StatusBadRequest)
		return
	}

	cacheKey := "security:" + strings.ToLower(name)

	if data, fresh, err := h.cache.Get(cacheKey, securityTTL); err == nil && data != nil && fresh {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("Content-Type", "application/json")
		w.Write(data) //nolint:errcheck
		return
	}

	vulns, err := h.osv.FetchVulns(name)
	if err != nil {
		// Serve stale cache on error rather than failing the request.
		if data, _, cacheErr := h.cache.Get(cacheKey, 0); cacheErr == nil && data != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write(data) //nolint:errcheck
			return
		}
		http.Error(w, "failed to fetch security data", http.StatusBadGateway)
		return
	}

	if vulns == nil {
		vulns = []osv.VulnInfo{}
	}

	resp := SecurityResponse{
		Package:   name,
		Vulns:     vulns,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}

	encoded, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	h.cache.Set(cacheKey, encoded, securityTTL) //nolint:errcheck

	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Type", "application/json")
	w.Write(encoded) //nolint:errcheck
}
```

- [ ] **Step 6: Run all OSV + security tests**

```bash
cd api && go test ./internal/osv/... ./internal/handler/... -run "TestFetchVulns|TestSecurityHandler" -v
```

Expected: `PASS`

- [ ] **Step 7: Commit**

```bash
cd api && git add internal/osv/ internal/handler/security.go internal/handler/security_test.go
git commit -m "feat(api): add OSV security handler (GET /api/packages/{name}/security)"
```

---

## Task 6: Conda-forge Client

**Files:**
- Create: `api/internal/conda/client.go`
- Create: `api/internal/conda/client_test.go`

- [ ] **Step 1: Write failing test**

```go
// api/internal/conda/client_test.go
package conda_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pypx/api/internal/conda"
)

func TestFetchCondaInfo_Available(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/package/conda-forge/numpy" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"name":"numpy","latest_version":"1.26.4"}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := conda.NewClient(conda.WithBaseURL(srv.URL))
	info, err := c.FetchCondaInfo("numpy")
	if err != nil {
		t.Fatalf("FetchCondaInfo() error: %v", err)
	}
	if !info.Available {
		t.Error("Available should be true")
	}
	if info.Version != "1.26.4" {
		t.Errorf("Version = %q, want 1.26.4", info.Version)
	}
}

func TestFetchCondaInfo_NotAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := conda.NewClient(conda.WithBaseURL(srv.URL))
	info, err := c.FetchCondaInfo("some-obscure-package")
	if err != nil {
		t.Fatalf("FetchCondaInfo() error: %v", err)
	}
	if info.Available {
		t.Error("Available should be false for 404")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
cd api && go test ./internal/conda/... -v
```

Expected: `FAIL — no Go files`

- [ ] **Step 3: Implement conda client**

```go
// api/internal/conda/client.go
package conda

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// CondaForgeInfo describes a package's conda-forge availability.
type CondaForgeInfo struct {
	Available bool   `json:"available"`
	Version   string `json:"version,omitempty"`
	URL       string `json:"url,omitempty"`
}

// anacondaResponse is the partial JSON shape from the Anaconda API.
type anacondaResponse struct {
	Name          string `json:"name"`
	LatestVersion string `json:"latest_version"`
}

// Client queries the conda-forge channel via the Anaconda API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the Anaconda API base URL (for testing).
func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = url }
}

// NewClient creates a new conda Client.
func NewClient(opts ...Option) *Client {
	c := &Client{
		baseURL:    "https://api.anaconda.org",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// FetchCondaInfo checks whether name exists on conda-forge.
// Returns a CondaForgeInfo with Available=false (not an error) if the package
// is not found on conda-forge.
func (c *Client) FetchCondaInfo(name string) (CondaForgeInfo, error) {
	url := fmt.Sprintf("%s/package/conda-forge/%s", c.baseURL, name)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return CondaForgeInfo{}, fmt.Errorf("conda: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return CondaForgeInfo{Available: false}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return CondaForgeInfo{}, fmt.Errorf("conda: unexpected status %d", resp.StatusCode)
	}

	var raw anacondaResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return CondaForgeInfo{}, fmt.Errorf("conda: decode response: %w", err)
	}

	return CondaForgeInfo{
		Available: true,
		Version:   raw.LatestVersion,
		URL:       fmt.Sprintf("https://anaconda.org/conda-forge/%s", name),
	}, nil
}
```

- [ ] **Step 4: Run to verify it passes**

```bash
cd api && go test ./internal/conda/... -v
```

Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
cd api && git add internal/conda/
git commit -m "feat(api): add conda-forge availability client"
```

---

## Task 7: Type Support Stubs Check

**Files:**
- Create: `api/internal/pypi/stubs.go`
- Create: `api/internal/pypi/stubs_test.go`

- [ ] **Step 1: Write failing test**

```go
// api/internal/pypi/stubs_test.go
package pypi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pypx/api/internal/pypi"
)

func TestCheckTypeSupport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/pypi/types-requests/json":
			w.WriteHeader(http.StatusOK)
		case "/pypi/requests-stubs/json":
			w.WriteHeader(http.StatusNotFound)
		case "/pypi/types-numpy/json":
			w.WriteHeader(http.StatusNotFound)
		case "/pypi/numpy-stubs/json":
			w.WriteHeader(http.StatusOK)
		case "/pypi/types-notstubs/json", "/pypi/notstubs-stubs/json":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	tests := []struct {
		name        string
		pkg         string
		wantStatus  string
		wantStubPkg string
	}{
		{"types- prefix found", "requests", "stubs", "types-requests"},
		{"-stubs suffix found", "numpy", "stubs", "numpy-stubs"},
		{"no stubs found", "notstubs", "untyped", ""},
	}

	c := pypi.NewClient(pypi.WithBaseURL(srv.URL))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pypi.CheckTypeSupport(c, tt.pkg)
			if got.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", got.Status, tt.wantStatus)
			}
			if got.StubsPackage != tt.wantStubPkg {
				t.Errorf("StubsPackage = %q, want %q", got.StubsPackage, tt.wantStubPkg)
			}
		})
	}
}
```

- [ ] **Step 2: Check that `pypi.NewClient` accepts `WithBaseURL` option**

Look at `api/internal/pypi/client.go`. If `WithBaseURL` doesn't exist, add it to the existing `Option` block (after the existing `NewClient` function, around line 80):

```go
// WithBaseURL overrides the PyPI API base URL (for testing).
func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = url }
}
```

- [ ] **Step 3: Run to verify it fails**

```bash
cd api && go test ./internal/pypi/... -run TestCheckTypeSupport -v
```

Expected: `FAIL — undefined: pypi.TypeSupport`

- [ ] **Step 4: Implement type support check**

```go
// api/internal/pypi/stubs.go
package pypi

import (
	"fmt"
	"net/http"
)

// TypeSupport describes whether a package ships type information.
type TypeSupport struct {
	// Status is one of "typed", "stubs", or "untyped".
	// "typed"  — the package ships a py.typed marker (not yet checked here).
	// "stubs"  — a separate types-{name} or {name}-stubs package exists.
	// "untyped" — no type information found.
	Status       string `json:"status"`
	StubsPackage string `json:"stubs_package,omitempty"`
}

// CheckTypeSupport checks PyPI for stub packages for the given package name.
// It checks for types-{name} first, then {name}-stubs.
// Returns TypeSupport with Status "stubs" if found, "untyped" otherwise.
func CheckTypeSupport(c *Client, name string) TypeSupport {
	candidates := []string{
		"types-" + name,
		name + "-stubs",
	}
	for _, candidate := range candidates {
		if packageExists(c, candidate) {
			return TypeSupport{Status: "stubs", StubsPackage: candidate}
		}
	}
	return TypeSupport{Status: "untyped"}
}

// packageExists returns true if the named package exists on PyPI.
func packageExists(c *Client, name string) bool {
	url := fmt.Sprintf("%s/pypi/%s/json", c.baseURL, name)
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return false
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
```

- [ ] **Step 5: Run to verify it passes**

```bash
cd api && go test ./internal/pypi/... -run TestCheckTypeSupport -v
```

Expected: `PASS`

- [ ] **Step 6: Commit**

```bash
cd api && git add internal/pypi/stubs.go internal/pypi/stubs_test.go internal/pypi/client.go
git commit -m "feat(api): add type support stubs check"
```

---

## Task 8: Extras Handler (Type Support + Conda)

**Files:**
- Create: `api/internal/handler/extras.go`
- Create: `api/internal/handler/extras_test.go`

- [ ] **Step 1: Write failing test**

```go
// api/internal/handler/extras_test.go
package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/conda"
	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/pypi"
)

func TestExtrasHandlerGet(t *testing.T) {
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// types-requests exists, requests-stubs does not
		if r.URL.Path == "/pypi/types-requests/json" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer pypiSrv.Close()

	condaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/package/conda-forge/requests" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"name":"requests","latest_version":"2.32.3"}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer condaSrv.Close()

	memCache := cache.NewMemoryCache(newTestSQLiteCache(t), 100)
	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	condaClient := conda.NewClient(conda.WithBaseURL(condaSrv.URL))
	h := handler.NewExtrasHandler(pypiClient, condaClient, memCache)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/extras", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/packages/requests/extras", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp handler.ExtrasResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.TypeSupport.Status != "stubs" {
		t.Errorf("TypeSupport.Status = %q, want stubs", resp.TypeSupport.Status)
	}
	if resp.TypeSupport.StubsPackage != "types-requests" {
		t.Errorf("StubsPackage = %q, want types-requests", resp.TypeSupport.StubsPackage)
	}
	if resp.CondaForge == nil || !resp.CondaForge.Available {
		t.Error("CondaForge.Available should be true")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
cd api && go test ./internal/handler/... -run TestExtrasHandlerGet -v
```

Expected: `FAIL — undefined: handler.ExtrasResponse`

- [ ] **Step 3: Implement extras handler**

```go
// api/internal/handler/extras.go
package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/conda"
	"github.com/pypx/api/internal/pypi"
)

const extrasTTL = 24 * time.Hour

// ExtrasResponse is the response for GET /api/packages/{name}/extras.
type ExtrasResponse struct {
	Package     string            `json:"package"`
	TypeSupport pypi.TypeSupport  `json:"type_support"`
	CondaForge  *conda.CondaForgeInfo `json:"conda_forge"`
}

// ExtrasHandler serves type support and conda-forge data.
type ExtrasHandler struct {
	pypi  *pypi.Client
	conda *conda.Client
	cache cache.Cacher
}

// NewExtrasHandler creates a new ExtrasHandler.
func NewExtrasHandler(pypiClient *pypi.Client, condaClient *conda.Client, c cache.Cacher) *ExtrasHandler {
	return &ExtrasHandler{pypi: pypiClient, conda: condaClient, cache: c}
}

// Get handles GET /api/packages/{name}/extras.
func (h *ExtrasHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := pypi.ValidateName(name); err != nil {
		http.Error(w, "invalid package name", http.StatusBadRequest)
		return
	}

	cacheKey := "extras:" + strings.ToLower(name)

	if data, fresh, err := h.cache.Get(cacheKey, extrasTTL); err == nil && data != nil && fresh {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("Content-Type", "application/json")
		w.Write(data) //nolint:errcheck
		return
	}

	// Fetch type support and conda info in parallel.
	var (
		typeSupport pypi.TypeSupport
		condaInfo   conda.CondaForgeInfo
		condaErr    error
		wg          sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		typeSupport = pypi.CheckTypeSupport(h.pypi, name)
	}()
	go func() {
		defer wg.Done()
		condaInfo, condaErr = h.conda.FetchCondaInfo(name)
	}()
	wg.Wait()

	resp := ExtrasResponse{
		Package:     name,
		TypeSupport: typeSupport,
	}
	if condaErr == nil {
		resp.CondaForge = &condaInfo
	}

	encoded, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	h.cache.Set(cacheKey, encoded, extrasTTL) //nolint:errcheck

	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Type", "application/json")
	w.Write(encoded) //nolint:errcheck
}
```

- [ ] **Step 4: Run to verify it passes**

```bash
cd api && go test ./internal/handler/... -run TestExtrasHandlerGet -v
```

Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
cd api && git add internal/handler/extras.go internal/handler/extras_test.go
git commit -m "feat(api): add extras handler for type support and conda-forge"
```

---

## Task 9: Extend PackageResponse and ChangelogResponse

**Files:**
- Modify: `api/internal/handler/packages.go`
- Modify: `api/internal/handler/changelog.go`
- Modify: `api/internal/handler/packages_test.go`

- [ ] **Step 1: Extend PackageResponse with new fields**

In `api/internal/handler/packages.go`, update the `PackageResponse` struct. Add four new fields after `Dependencies` (line 49):

```go
// Existing end of struct:
Dependencies   enrichment.DependencyTree    `json:"dependencies"`

// Add:
PlatformCoverage enrichment.PlatformCoverage `json:"platform_coverage"`
ReleaseCadence   enrichment.ReleaseCadence   `json:"release_cadence"`
Maintainers      []enrichment.Maintainer     `json:"maintainers"`
DocURL           string                       `json:"doc_url"`
```

- [ ] **Step 2: Wire new enrichments in buildPackageResponse**

In `api/internal/handler/packages.go`, update `buildPackageResponse` to populate the new fields. The function currently returns at the end (around line 347). Update the return statement:

```go
func buildPackageResponse(r *pypi.PyPIResponse) PackageResponse {
	info := r.Info

	files := make([]FileInfo, 0, len(r.URLs))
	for _, f := range r.URLs {
		files = append(files, FileInfo{
			Filename:    f.Filename,
			Size:        f.Size,
			PackageType: f.PackageType,
			PythonVer:   f.PythonVer,
			UploadTime:  f.UploadTime,
		})
	}

	var descHTML string
	if strings.Contains(info.DescriptionType, "text/markdown") {
		descHTML, _ = markdown.Render(info.Description)
	}

	return PackageResponse{
		Name:             info.Name,
		Version:          info.Version,
		Summary:          info.Summary,
		Description:      info.Description,
		DescType:         info.DescriptionType,
		DescriptionHTML:  descHTML,
		License:          normalizeLicense(info),
		Author:           info.Author,
		AuthorEmail:      info.AuthorEmail,
		HomePage:         info.HomePage,
		RequiresPython:   info.RequiresPython,
		RequiresDist:     info.RequiresDist,
		ProjectURLs:      info.ProjectURLs,
		Classifiers:      info.Classifiers,
		LatestFiles:      files,
		InstallSize:      enrichment.ExtractInstallSize(r.URLs),
		ModuleFormat:     enrichment.ExtractModuleFormat(r.URLs),
		PythonVersions:   enrichment.ExtractPythonVersions(info.RequiresPython),
		Dependencies:     enrichment.ParseDependencies(info.RequiresDist),
		PlatformCoverage: enrichment.ExtractPlatformCoverage(r.URLs),
		ReleaseCadence:   enrichment.ComputeReleaseCadence(r.Releases),
		Maintainers:      enrichment.ParseMaintainers(info),
		DocURL:           enrichment.ExtractDocURL(info.ProjectURLs),
	}
}
```

- [ ] **Step 3: Extend ChangelogResponse with RepoInfo**

In `api/internal/handler/changelog.go`, add `RepoInfo` to the struct (after `Entries`):

```go
type ChangelogResponse struct {
	Package  string        `json:"package"`
	Source   string        `json:"source"`
	RepoURL  string        `json:"repo_url"`
	Entries  []gh.Release  `json:"entries"`
	RepoInfo *gh.RepoInfo  `json:"repo_info,omitempty"`
}
```

- [ ] **Step 4: Fetch repo info in the changelog handler**

In `api/internal/handler/changelog.go`, inside the `Get` method, after the `releases` fetch (after line 90), add:

```go
// Fetch repo health signals alongside releases.
var repoInfo *gh.RepoInfo
if ok {
    repoInfo, err = h.github.FetchRepoInfo(owner, repo)
    if err != nil {
        // Non-fatal — log and continue without repo info.
        log.Printf("changelog: failed to fetch repo info for %s/%s: %v", owner, repo, err)
        repoInfo = nil
    }
}
```

And update the response construction to include `RepoInfo`:

```go
resp := ChangelogResponse{
    Package:  pypiResp.Info.Name,
    Source:   source,
    RepoURL:  repoURL,
    Entries:  releases,
    RepoInfo: repoInfo,
}
```

Add `"log"` to the import block in `changelog.go` if not already present.

- [ ] **Step 5: Run the full handler test suite**

```bash
cd api && go test ./internal/handler/... -v
```

Expected: `PASS` (all existing tests plus new ones)

- [ ] **Step 6: Commit**

```bash
cd api && git add internal/handler/packages.go internal/handler/changelog.go internal/handler/packages_test.go
git commit -m "feat(api): extend PackageResponse and ChangelogResponse with new enrichments"
```

---

## Task 10: Wire New Routes in main.go

**Files:**
- Modify: `api/cmd/server/main.go`

- [ ] **Step 1: Add new imports and clients**

In `api/cmd/server/main.go`, add the new packages to the import block:

```go
"github.com/pypx/api/internal/conda"
"github.com/pypx/api/internal/osv"
```

- [ ] **Step 2: Instantiate new clients and handlers**

After the `statsClient` / `statsHandler` block (around line 55), add:

```go
osvClient := osv.NewClient()
securityHandler := handler.NewSecurityHandler(osvClient, c)

condaClient := conda.NewClient()
extrasHandler := handler.NewExtrasHandler(pypiClient, condaClient, c)
```

- [ ] **Step 3: Register new routes**

After the existing routes (after `r.Get("/api/packages/{name}/stats", statsHandler.Get)`, around line 88), add:

```go
r.Get("/api/packages/{name}/security", securityHandler.Get)
r.Get("/api/packages/{name}/extras", extrasHandler.Get)
```

- [ ] **Step 4: Build to verify it compiles**

```bash
cd api && go build ./...
```

Expected: exits 0, no errors.

- [ ] **Step 5: Commit**

```bash
cd api && git add cmd/server/main.go
git commit -m "feat(api): wire security and extras endpoints"
```

---

## Task 11: TypeScript Types + useApi Composable

**Files:**
- Modify: `web/app/types/api.ts`
- Modify: `web/app/composables/useApi.ts`

- [ ] **Step 1: Extend api.ts with all new interfaces**

Replace the full contents of `web/app/types/api.ts`:

```typescript
export interface PackageData {
  name: string;
  version: string;
  summary: string;
  description: string;
  description_content_type: string;
  description_html: string;
  license: string;
  author: string;
  author_email: string;
  home_page: string;
  requires_python: string;
  requires_dist: string[];
  project_urls: Record<string, string>;
  classifiers: string[];
  latest_files: FileInfo[];
  install_size: number;
  module_format: string;
  python_versions: PythonVersionInfo;
  dependencies: DependencyTree;
  platform_coverage: PlatformCoverage;
  release_cadence: ReleaseCadence;
  maintainers: Maintainer[];
  doc_url: string;
}

export interface FileInfo {
  filename: string;
  size: number;
  package_type: string;
  python_version: string;
  upload_time: string;
}

export interface PythonVersionInfo {
  constraint: string;
  min_version: string;
}

export interface Dependency {
  name: string;
  constraint: string;
}

export interface DependencyTree {
  required: Dependency[];
  extras: Record<string, Dependency[]>;
}

export interface PlatformCoverage {
  pure_python: boolean;
  linux_x86_64: boolean;
  linux_arm64: boolean;
  macos_x86_64: boolean;
  macos_arm64: boolean;
  windows_x86_64: boolean;
  musl: boolean;
}

export interface QuarterCount {
  quarter: string;
  count: number;
}

export interface ReleaseCadence {
  releases_last_12mo: number;
  avg_days_between_releases: number;
  last_released_at: string;
  quarterly_counts: QuarterCount[];
}

export interface Maintainer {
  name: string;
  email: string;
}

export interface VersionInfo {
  version: string;
  install_size: number;
  module_format: string;
  upload_time: string;
  files: FileInfo[];
}

export interface StatsData {
  package: string;
  period: string;
  date_range?: { from: string; to: string };
  overall: DataPoint[];
  python_versions: DataPoint[];
  systems: DataPoint[];
}

export interface DataPoint {
  category: string;
  downloads: number;
}

export interface SearchResult {
  name: string;
  summary: string;
  downloads: number;
}

export interface RepoOwner {
  login: string;
  avatar_url: string;
  display_name: string;
  url: string;
  is_org: boolean;
}

export interface RepoInfo {
  stars: number;
  forks: number;
  open_issues: number;
  last_pushed_at: string;
  owner: RepoOwner;
}

export interface ChangelogEntry {
  version: string;
  tag_name: string;
  title: string;
  body: string;
  body_html: string;
  published_at: string;
  url: string;
}

export interface ChangelogData {
  package: string;
  source: string;
  repo_url: string;
  entries: ChangelogEntry[];
  repo_info?: RepoInfo;
}

export interface VulnInfo {
  id: string;
  summary: string;
  severity: string;
  affected_range: string;
  fixed_in?: string;
  url: string;
}

export interface SecurityData {
  package: string;
  vulns: VulnInfo[];
  checked_at: string;
}

export interface TypeSupport {
  status: 'typed' | 'stubs' | 'untyped';
  stubs_package?: string;
}

export interface CondaForgeInfo {
  available: boolean;
  version?: string;
  url?: string;
}

export interface ExtrasData {
  package: string;
  type_support: TypeSupport;
  conda_forge?: CondaForgeInfo;
}
```

- [ ] **Step 2: Add fetchSecurity and fetchExtras to useApi.ts**

In `web/app/composables/useApi.ts`, add the new imports and methods:

```typescript
import type {
  PackageData,
  VersionInfo,
  DependencyTree,
  StatsData,
  SearchResult,
  ChangelogData,
  SecurityData,
  ExtrasData,
} from "~/types/api";

export function useApi() {
  const config = useRuntimeConfig();
  const baseURL = config.public.apiBase;

  async function fetchPackage(name: string): Promise<PackageData> {
    return $fetch<PackageData>(`${baseURL}/packages/${name}`);
  }

  async function fetchVersions(name: string): Promise<VersionInfo[]> {
    return $fetch<VersionInfo[]>(`${baseURL}/packages/${name}/versions`);
  }

  async function fetchDependencies(name: string): Promise<DependencyTree> {
    return $fetch<DependencyTree>(`${baseURL}/packages/${name}/dependencies`);
  }

  async function fetchStats(name: string, period?: string): Promise<StatsData> {
    return $fetch<StatsData>(`${baseURL}/packages/${name}/stats`, {
      params: period ? { period } : undefined,
    });
  }

  async function searchPackages(query: string, limit = 20): Promise<SearchResult[]> {
    return $fetch<SearchResult[]>(`${baseURL}/search`, {
      params: { q: query, limit },
    });
  }

  async function fetchChangelog(name: string): Promise<ChangelogData> {
    return $fetch<ChangelogData>(`${baseURL}/packages/${name}/changelog`);
  }

  async function fetchSecurity(name: string): Promise<SecurityData> {
    return $fetch<SecurityData>(`${baseURL}/packages/${name}/security`);
  }

  async function fetchExtras(name: string): Promise<ExtrasData> {
    return $fetch<ExtrasData>(`${baseURL}/packages/${name}/extras`);
  }

  return {
    fetchPackage,
    fetchVersions,
    fetchDependencies,
    fetchStats,
    searchPackages,
    fetchChangelog,
    fetchSecurity,
    fetchExtras,
  };
}
```

- [ ] **Step 3: Commit**

```bash
git add web/app/types/api.ts web/app/composables/useApi.ts
git commit -m "feat(web): add TypeScript types and API methods for package enrichment"
```

---

## Task 12: Platform Coverage Component

**Files:**
- Create: `web/app/components/PackagePlatforms.vue`

- [ ] **Step 1: Create the component**

```vue
<!-- web/app/components/PackagePlatforms.vue -->
<script setup lang="ts">
import type { PlatformCoverage } from "~/types/api";

const props = defineProps<{
  coverage: PlatformCoverage;
}>();

interface Platform {
  key: keyof PlatformCoverage;
  label: string;
  short: string;
}

const platforms: Platform[] = [
  { key: "pure_python", label: "Pure Python", short: "py" },
  { key: "linux_x86_64", label: "Linux x86_64", short: "linux" },
  { key: "linux_arm64", label: "Linux ARM64", short: "arm64" },
  { key: "macos_x86_64", label: "macOS Intel", short: "mac-x86" },
  { key: "macos_arm64", label: "macOS Apple Silicon", short: "mac-arm" },
  { key: "windows_x86_64", label: "Windows x86_64", short: "win" },
  { key: "musl", label: "musl/Alpine", short: "musl" },
];

const supported = computed(() =>
  platforms.filter((p) => props.coverage[p.key])
);
const hasAnyCoverage = computed(() => supported.value.length > 0);
</script>

<template>
  <div v-if="hasAnyCoverage">
    <div class="text-xs font-medium text-neutral-400 uppercase tracking-wide mb-2">
      Platforms
    </div>
    <div class="flex flex-wrap gap-1.5">
      <span
        v-for="p in supported"
        :key="p.key"
        :title="p.label"
        class="inline-flex items-center px-2 py-0.5 rounded text-xs font-mono bg-neutral-800 text-neutral-300 ring-1 ring-neutral-700"
      >
        {{ p.short }}
      </span>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Commit**

```bash
git add web/app/components/PackagePlatforms.vue
git commit -m "feat(web): add platform coverage component"
```

---

## Task 13: Maintainers Component

**Files:**
- Create: `web/app/components/PackageMaintainers.vue`

- [ ] **Step 1: Create the component**

```vue
<!-- web/app/components/PackageMaintainers.vue -->
<script setup lang="ts">
import type { Maintainer, RepoInfo } from "~/types/api";

const props = defineProps<{
  maintainers: Maintainer[];
  repoInfo?: RepoInfo;
}>();

// Build display list: prefer GitHub org owner if available, then maintainers.
const displayMaintainers = computed(() => {
  return props.maintainers.slice(0, 5); // cap at 5
});

const hasAny = computed(
  () => displayMaintainers.value.length > 0 || props.repoInfo?.owner
);
</script>

<template>
  <div v-if="hasAny">
    <div class="text-xs font-medium text-neutral-400 uppercase tracking-wide mb-2">
      Maintainers
    </div>

    <!-- GitHub org/owner badge -->
    <div v-if="repoInfo?.owner" class="mb-2">
      <a
        :href="repoInfo.owner.url"
        target="_blank"
        rel="noopener noreferrer"
        class="inline-flex items-center gap-2 text-sm text-neutral-300 hover:text-white transition-colors"
      >
        <img
          :src="repoInfo.owner.avatar_url"
          :alt="repoInfo.owner.login"
          class="w-5 h-5 rounded-full"
        />
        <span class="font-medium">
          {{ repoInfo.owner.display_name || repoInfo.owner.login }}
        </span>
        <span
          v-if="repoInfo.owner.is_org"
          class="text-xs px-1.5 py-0.5 rounded bg-neutral-800 text-neutral-400 ring-1 ring-neutral-700"
        >
          org
        </span>
      </a>
    </div>

    <!-- Individual maintainers -->
    <ul v-if="displayMaintainers.length" class="space-y-1">
      <li
        v-for="m in displayMaintainers"
        :key="m.email || m.name"
        class="text-sm text-neutral-400"
      >
        <span v-if="m.name" class="text-neutral-300">{{ m.name }}</span>
        <span v-if="m.name && m.email" class="text-neutral-600"> · </span>
        <a
          v-if="m.email"
          :href="`mailto:${m.email}`"
          class="text-neutral-500 hover:text-neutral-300 transition-colors"
        >
          {{ m.email }}
        </a>
      </li>
    </ul>
  </div>
</template>
```

- [ ] **Step 2: Commit**

```bash
git add web/app/components/PackageMaintainers.vue
git commit -m "feat(web): add maintainers component with GitHub org display"
```

---

## Task 14: Update Badge Row

**Files:**
- Modify: `web/app/components/PackageBadges.vue`

Read the current `PackageBadges.vue` first to understand the existing badge markup pattern, then add the following badges.

- [ ] **Step 1: Read the current badge component**

```bash
cat web/app/components/PackageBadges.vue
```

- [ ] **Step 2: Add type support, conda, and security badges**

The component receives `pkg` as a prop. Add `extras` and `security` props and three new badges after the existing ones. The exact placement depends on the current markup — insert the new props definition and badge elements following the same pattern as the existing badges.

Add to the `<script setup>`:

```typescript
import type { PackageData, ExtrasData, SecurityData } from "~/types/api";

const props = defineProps<{
  pkg: PackageData;
  extras?: ExtrasData | null;
  security?: SecurityData | null;
}>();

const vulnCount = computed(() => props.security?.vulns?.length ?? 0);
const typeStatus = computed(() => props.extras?.type_support?.status);
const condaAvailable = computed(() => props.extras?.conda_forge?.available);
```

Add these badges inside the badge row (after the existing dep count badge):

```vue
<!-- Type support badge -->
<span
  v-if="typeStatus && typeStatus !== 'untyped'"
  class="inline-flex items-center gap-1 px-2 py-1 rounded text-xs font-medium ring-1"
  :class="
    typeStatus === 'typed'
      ? 'bg-blue-950 text-blue-300 ring-blue-800'
      : 'bg-neutral-800 text-neutral-300 ring-neutral-700'
  "
>
  <span v-if="typeStatus === 'typed'">typed</span>
  <span v-else>stubs</span>
</span>

<!-- Conda badge -->
<a
  v-if="condaAvailable && extras?.conda_forge?.url"
  :href="extras.conda_forge.url"
  target="_blank"
  rel="noopener noreferrer"
  class="inline-flex items-center gap-1 px-2 py-1 rounded text-xs font-medium ring-1 bg-green-950 text-green-300 ring-green-800 hover:bg-green-900 transition-colors"
>
  conda
</a>

<!-- Security badge -->
<span
  v-if="security"
  class="inline-flex items-center gap-1 px-2 py-1 rounded text-xs font-medium ring-1"
  :class="
    vulnCount === 0
      ? 'bg-neutral-800 text-neutral-400 ring-neutral-700'
      : 'bg-red-950 text-red-300 ring-red-800'
  "
>
  <span v-if="vulnCount === 0">secure</span>
  <span v-else>{{ vulnCount }} {{ vulnCount === 1 ? 'CVE' : 'CVEs' }}</span>
</span>
```

- [ ] **Step 3: Commit**

```bash
git add web/app/components/PackageBadges.vue
git commit -m "feat(web): add type support, conda, and security badges"
```

---

## Task 15: Update Overview Component

**Files:**
- Modify: `web/app/components/PackageOverview.vue`

Read the current `PackageOverview.vue` first, then add the new sidebar sections.

- [ ] **Step 1: Read the current overview component**

```bash
cat web/app/components/PackageOverview.vue
```

- [ ] **Step 2: Add new props**

Extend the component's `defineProps` to accept `repoInfo` and `docUrl`. Example:

```typescript
import type { PackageData, RepoInfo } from "~/types/api";

const props = defineProps<{
  pkg: PackageData;
  repoInfo?: RepoInfo | null;
}>();
```

- [ ] **Step 3: Add GitHub health signals section to the sidebar**

Below the existing project links section, insert:

```vue
<!-- GitHub health signals -->
<div v-if="repoInfo" class="pt-3 border-t border-neutral-800">
  <div class="text-xs font-medium text-neutral-400 uppercase tracking-wide mb-2">
    GitHub
  </div>
  <div class="flex flex-wrap gap-x-4 gap-y-1 text-sm text-neutral-400">
    <span v-if="repoInfo.stars">
      <span class="text-neutral-300">{{ repoInfo.stars.toLocaleString() }}</span> stars
    </span>
    <span v-if="repoInfo.forks">
      <span class="text-neutral-300">{{ repoInfo.forks.toLocaleString() }}</span> forks
    </span>
    <span v-if="repoInfo.open_issues !== undefined">
      <span class="text-neutral-300">{{ repoInfo.open_issues.toLocaleString() }}</span> open issues
    </span>
  </div>
  <div v-if="repoInfo.last_pushed_at" class="text-xs text-neutral-500 mt-1">
    last commit {{ useTimeAgo(new Date(repoInfo.last_pushed_at)).value }}
  </div>
</div>

<!-- Doc link button -->
<div v-if="pkg.doc_url" class="pt-3 border-t border-neutral-800">
  <a
    :href="pkg.doc_url"
    target="_blank"
    rel="noopener noreferrer"
    class="inline-flex items-center gap-1.5 text-sm text-blue-400 hover:text-blue-300 transition-colors"
  >
    Documentation →
  </a>
</div>

<!-- Release cadence -->
<div
  v-if="pkg.release_cadence?.releases_last_12mo > 0"
  class="pt-3 border-t border-neutral-800"
>
  <div class="text-xs font-medium text-neutral-400 uppercase tracking-wide mb-1">
    Release Cadence
  </div>
  <div class="text-sm text-neutral-400">
    <span class="text-neutral-300">{{ pkg.release_cadence.releases_last_12mo }}</span>
    releases in the past year
    <span v-if="pkg.release_cadence.avg_days_between_releases > 0">
      · avg {{ Math.round(pkg.release_cadence.avg_days_between_releases) }} days apart
    </span>
  </div>
</div>

<!-- Platform coverage -->
<div class="pt-3 border-t border-neutral-800">
  <PackagePlatforms :coverage="pkg.platform_coverage" />
</div>

<!-- Maintainers -->
<div class="pt-3 border-t border-neutral-800">
  <PackageMaintainers :maintainers="pkg.maintainers" :repo-info="repoInfo" />
</div>
```

Note: `useTimeAgo` is from `@vueuse/core` which is available via `@vueuse/nuxt`. Import it at the top of `<script setup>`: `import { useTimeAgo } from '@vueuse/core'`.

- [ ] **Step 4: Commit**

```bash
git add web/app/components/PackageOverview.vue
git commit -m "feat(web): add GitHub signals, doc link, cadence, platforms, and maintainers to overview"
```

---

## Task 16: Wire Parallel Fetches on Package Page

**Files:**
- Modify: `web/app/pages/packages/[name].vue`

Read the current page component first, then add the parallel fetches.

- [ ] **Step 1: Read the current package page**

```bash
cat web/app/pages/packages/[name].vue
```

- [ ] **Step 2: Add security and extras parallel fetches**

In the `<script setup>` block, alongside the existing `useAsyncData` for the package, add two non-blocking fetches. They should not block SSR (use `server: false` or fetch client-side after hydration):

```typescript
const api = useApi();
const route = useRoute();
const name = route.params.name as string;

// Main package data (SSR-rendered)
const { data: pkg } = await useAsyncData(`pkg-${name}`, () =>
  api.fetchPackage(name)
);

// Parallel non-blocking fetches (client-side, don't block SSR)
const { data: security } = useAsyncData(
  `security-${name}`,
  () => api.fetchSecurity(name),
  { server: false, default: () => null }
);

const { data: extras } = useAsyncData(
  `extras-${name}`,
  () => api.fetchExtras(name),
  { server: false, default: () => null }
);

// Pass changelog data (already fetched by PackageVersions) to Overview for repoInfo.
// The changelog is fetched inside PackageVersions — use a shared state or prop-drill
// repoInfo from the changelog fetch. Simplest approach: fetch changelog here too
// and extract repoInfo.
const { data: changelog } = useAsyncData(
  `changelog-${name}`,
  () => api.fetchChangelog(name),
  { server: false, default: () => null }
);

const repoInfo = computed(() => changelog.value?.repo_info ?? null);
```

- [ ] **Step 3: Pass new props to child components**

Update the template to pass the new props:

```vue
<!-- Pass extras and security to badge row -->
<PackageBadges :pkg="pkg" :extras="extras" :security="security" />

<!-- Pass repoInfo to overview -->
<PackageOverview :pkg="pkg" :repo-info="repoInfo" />
```

- [ ] **Step 4: Verify the app runs**

```bash
cd web && pnpm dev
```

Open `http://localhost:3000/packages/requests` and verify:
- The overview sidebar shows GitHub signals (stars, forks, issues, last commit)
- The doc link appears if requests has documentation
- Maintainers list shows
- Platform coverage shows
- Badge row shows "stubs" badge (requests has types-requests)
- Badge row shows "conda" badge
- Security badge shows (requests has had vulns historically)

- [ ] **Step 5: Commit**

```bash
git add web/app/pages/packages/[name].vue
git commit -m "feat(web): wire security, extras, and repo info fetches on package page"
```

---

## Task 17: Run Full Test Suite + Final Verification

- [ ] **Step 1: Run all Go tests**

```bash
cd api && go test ./... -v 2>&1 | tail -30
```

Expected: all `PASS`, no failures.

- [ ] **Step 2: Run frontend type check**

```bash
cd web && pnpm typecheck
```

Expected: no type errors.

- [ ] **Step 3: Spot-check 3 packages in the browser**

Test with packages that cover different states:
- `requests` — stubs available (types-requests), conda available, GitHub under psf org
- `numpy` — platform-specific wheels, conda available, stubs (numpy-stubs)
- `flask` — pure Python, Pallets org

Verify for each:
- Badge row shows correct type/conda/security badges
- Overview sidebar shows GitHub signals with last commit time
- Maintainers section shows org name (not just login)
- Platform coverage grid shows correct platforms
- Doc link button appears and points to correct URL
- Release cadence shows realistic numbers

- [ ] **Step 4: Final commit**

```bash
git add -A
git commit -m "feat: package richness phase 1 complete (GitHub signals, maintainers, type support, security, platforms, cadence, docs, conda)"
```
