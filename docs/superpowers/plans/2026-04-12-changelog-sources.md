# Changelog Sources Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the changelog endpoint to show data for packages that don't publish GitHub Releases, by adding a priority-ordered parallel fallback chain: GitHub/GitLab Releases → CHANGELOG file → git tags + commits.

**Architecture:** A new `internal/changelog` package defines a `Source` interface and a `Registry` that fires all sources concurrently and returns the highest-priority non-empty result. GitHub and GitLab each implement three sources (releases, file, tags). The existing handler is refactored to build a source list based on the detected platform and delegate to the registry.

**Tech Stack:** Go (net/http, encoding/base64, sync, regexp), Vue 3 (Composition API, computed), Tailwind CSS

---

## File Map

**New files:**
- `api/internal/changelog/source.go` — `Source` interface, `Entry`, `Result` types
- `api/internal/changelog/parser.go` — CHANGELOG file parser
- `api/internal/changelog/parser_test.go` — parser tests
- `api/internal/changelog/registry.go` — parallel fetch + priority selection
- `api/internal/changelog/registry_test.go` — registry tests
- `api/internal/github/releases_source.go` — `GitHubReleasesSource`
- `api/internal/github/file_source.go` — `GitHubChangelogFileSource`
- `api/internal/github/tags_source.go` — `GitHubTagsSource`
- `api/internal/gitlab/client.go` — GitLab API client + URL extraction
- `api/internal/gitlab/client_test.go` — GitLab client tests
- `api/internal/gitlab/releases_source.go` — `GitLabReleasesSource`
- `api/internal/gitlab/file_source.go` — `GitLabChangelogFileSource`
- `api/internal/gitlab/tags_source.go` — `GitLabTagsSource`

**Modified files:**
- `api/internal/github/client.go` — add `FetchRawFile`, `FetchTags`, `FetchCompare`
- `api/internal/github/client_test.go` — tests for new methods
- `api/internal/handler/changelog.go` — use registry, add gitlab client
- `api/internal/handler/changelog_test.go` — update + add tests for new sources
- `api/cmd/server/main.go` — inject gitlab client into handler
- `web/app/components/PackageVersions.vue` — source badge

---

## Task 1: Define changelog types

**Files:**
- Create: `api/internal/changelog/source.go`

- [ ] **Step 1: Create the types file**

```go
// api/internal/changelog/source.go
package changelog

import "context"

// Entry is the canonical changelog entry type used across all sources.
// JSON tags match the existing frontend ChangelogEntry interface.
type Entry struct {
	Version     string `json:"version"`
	TagName     string `json:"tag_name"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	BodyHTML    string `json:"body_html"`
	PublishedAt string `json:"published_at"`
	URL         string `json:"url"`
}

// Source is implemented by each changelog data provider.
// Implementations are constructed with all parameters they need (owner, repo, etc.)
// so Fetch takes only a context.
type Source interface {
	// Name returns the source identifier used in the API response "source" field.
	Name() string
	// Fetch retrieves changelog entries. Returns empty slice (not error) when no
	// data is available from this source.
	Fetch(ctx context.Context) ([]Entry, error)
}

// Result holds the winning source name and its entries.
type Result struct {
	Source  string
	Entries []Entry
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd api && go build ./internal/changelog/...
```

Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
git add api/internal/changelog/source.go
git commit -m "feat(changelog): define Source interface and Entry types"
```

---

## Task 2: CHANGELOG file parser (TDD)

**Files:**
- Create: `api/internal/changelog/parser_test.go`
- Create: `api/internal/changelog/parser.go`

- [ ] **Step 1: Write the failing tests**

```go
// api/internal/changelog/parser_test.go
package changelog_test

import (
	"strings"
	"testing"

	"github.com/pypx/api/internal/changelog"
)

func TestParse_KeepAChangelog(t *testing.T) {
	input := `# Changelog

## [1.2.0] - 2024-03-15
### Added
- New feature A

## [1.1.0] - 2024-01-10
### Fixed
- Bug fix B
`
	entries := changelog.Parse(input)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Version != "1.2.0" {
		t.Errorf("entry[0].Version = %q, want 1.2.0", entries[0].Version)
	}
	if entries[0].PublishedAt != "2024-03-15" {
		t.Errorf("entry[0].PublishedAt = %q, want 2024-03-15", entries[0].PublishedAt)
	}
	if !strings.Contains(entries[0].Body, "New feature A") {
		t.Errorf("entry[0].Body missing expected content, got: %q", entries[0].Body)
	}
	if entries[1].Version != "1.1.0" {
		t.Errorf("entry[1].Version = %q, want 1.1.0", entries[1].Version)
	}
}

func TestParse_GitHubChangelogGenerator(t *testing.T) {
	// ## [v1.17.0](https://github.com/user/repo/tree/v1.17.0) (2026-03-13)
	input := `## [v1.17.0](https://github.com/user/repo/tree/v1.17.0) (2026-03-13)
Some changes here.

## [v1.16.0](https://github.com/user/repo/tree/v1.16.0) (2025-12-01)
Other changes.
`
	entries := changelog.Parse(input)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Version != "1.17.0" {
		t.Errorf("entry[0].Version = %q, want 1.17.0", entries[0].Version)
	}
	if entries[0].PublishedAt != "2026-03-13" {
		t.Errorf("entry[0].PublishedAt = %q, want 2026-03-13", entries[0].PublishedAt)
	}
}

func TestParse_OpenAIStyle(t *testing.T) {
	// ## 2.31.0 (2026-04-08)
	input := `## 2.31.0 (2026-04-08)
Changed foo.

## 2.30.0 (2026-01-15)
Fixed bar.
`
	entries := changelog.Parse(input)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Version != "2.31.0" {
		t.Errorf("entry[0].Version = %q, want 2.31.0", entries[0].Version)
	}
	if entries[0].PublishedAt != "2026-04-08" {
		t.Errorf("entry[0].PublishedAt = %q, want 2026-04-08", entries[0].PublishedAt)
	}
}

func TestParse_BareMarkdownHeading(t *testing.T) {
	// ## 1.0.0  (no date)
	input := `## 1.0.0
Some content.

## 0.9.0
Old content.
`
	entries := changelog.Parse(input)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Version != "1.0.0" {
		t.Errorf("entry[0].Version = %q, want 1.0.0", entries[0].Version)
	}
	if entries[0].PublishedAt != "" {
		t.Errorf("entry[0].PublishedAt = %q, want empty", entries[0].PublishedAt)
	}
}

func TestParse_RSTVersionUnderline(t *testing.T) {
	// RST style: Version X.Y.Z\n-----------
	input := `Version 1.16.0
--------------

Released 2021-04-04

Some RST content here.

Version 1.15.0
--------------

Older content.
`
	entries := changelog.Parse(input)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Version != "1.16.0" {
		t.Errorf("entry[0].Version = %q, want 1.16.0", entries[0].Version)
	}
}

func TestParse_RSTBareUnderline(t *testing.T) {
	// RST style: 1.0.0\n-----
	input := `1.0.0
-----

Some changes.

0.9.0
-----

Older changes.
`
	entries := changelog.Parse(input)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Version != "1.0.0" {
		t.Errorf("entry[0].Version = %q, want 1.0.0", entries[0].Version)
	}
}

func TestParse_SkipsUnreleased(t *testing.T) {
	input := `## [Unreleased]
Some upcoming changes.

## [1.0.0] - 2024-01-01
Initial release.

## [0.9.0] - 2023-12-01
Beta.
`
	entries := changelog.Parse(input)
	// [Unreleased] should be skipped
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (Unreleased skipped), got %d", len(entries))
	}
	if entries[0].Version != "1.0.0" {
		t.Errorf("entry[0].Version = %q, want 1.0.0", entries[0].Version)
	}
}

func TestParse_StrictMode_TooFewSections(t *testing.T) {
	// Only 1 version section → strict mode returns empty
	input := `## [1.0.0] - 2024-01-01
Only one section.
`
	entries := changelog.Parse(input)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries (strict mode), got %d", len(entries))
	}
}

func TestParse_StrictMode_NoVersionSections(t *testing.T) {
	input := `# My Project

Just a README with no changelog sections.
`
	entries := changelog.Parse(input)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestParse_VPrefixStripped(t *testing.T) {
	input := `## v1.2.0 - 2024-03-15
Content A.

## v1.1.0 - 2024-01-10
Content B.
`
	entries := changelog.Parse(input)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Version != "1.2.0" {
		t.Errorf("entry[0].Version = %q, want 1.2.0 (v prefix stripped)", entries[0].Version)
	}
	if entries[0].TagName != "v1.2.0" {
		t.Errorf("entry[0].TagName = %q, want v1.2.0 (original)", entries[0].TagName)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd api && go test ./internal/changelog/... -v -run TestParse
```

Expected: compilation failure — `changelog.Parse` not defined.

- [ ] **Step 3: Implement the parser**

```go
// api/internal/changelog/parser.go
package changelog

import (
	"regexp"
	"strings"
)

// headingPattern holds a compiled regex and which capture group holds version/date.
type headingPattern struct {
	re           *regexp.Regexp
	versionGroup int
	dateGroup    int // 0 = no date captured
}

// versionRE validates that a string looks like a version (semver or calver).
// Must have at least one dot and start with a digit.
var versionRE = regexp.MustCompile(`^\d+\.\d[\d.a-zA-Z\-]*$`)

// noiseVersionRE matches non-version identifiers we should skip (e.g. "Unreleased").
var noiseVersionRE = regexp.MustCompile(`(?i)^unreleased$`)

// Ordered list of Markdown heading patterns, tried in priority order.
var mdPatterns = []headingPattern{
	// ## [1.0.0] - 2023-06-20  (Keep a Changelog, git-cliff, release-please)
	{
		re:           regexp.MustCompile(`^#{1,3}\s+\[v?([0-9]+[0-9a-zA-Z.\-]*)\]\s*-\s*(\d{4}-\d{2}-\d{2})`),
		versionGroup: 1,
		dateGroup:    2,
	},
	// ## [v1.17.0](url) (2026-03-13)  (github-changelog-generator)
	{
		re:           regexp.MustCompile(`^#{1,3}\s+\[v?([0-9]+[0-9a-zA-Z.\-]*)\](?:\([^)]*\))?\s*\((\d{4}-\d{2}-\d{2})\)`),
		versionGroup: 1,
		dateGroup:    2,
	},
	// ## 2.31.0 (2026-04-08)  (OpenAI SDK, requests HISTORY.md)
	{
		re:           regexp.MustCompile(`^#{1,3}\s+v?([0-9]+[0-9a-zA-Z.\-]*)\s+\((\d{4}-\d{2}-\d{2})\)\s*$`),
		versionGroup: 1,
		dateGroup:    2,
	},
	// ## [1.0.0]  (bare bracket, no date)
	{
		re:           regexp.MustCompile(`^#{1,3}\s+\[v?([0-9]+[0-9a-zA-Z.\-]*)\]\s*$`),
		versionGroup: 1,
		dateGroup:    0,
	},
	// ## v1.0.0 - 2024-01-01  or  ## v1.0.0  (bare heading with optional date after dash)
	{
		re:           regexp.MustCompile(`^#{1,3}\s+v?([0-9]+\.[0-9][0-9a-zA-Z.\-]*)\s*(?:-\s*(\d{4}-\d{2}-\d{2}))?\s*$`),
		versionGroup: 1,
		dateGroup:    2,
	},
}

// parsedSection holds a detected version heading and the body lines that follow it.
type parsedSection struct {
	version string // normalized (v-prefix stripped)
	tagName string // original string from heading
	date    string // YYYY-MM-DD or ""
	body    []string
}

// Parse parses a CHANGELOG file into entries. Returns empty slice if fewer than
// 2 version sections are found (strict mode).
func Parse(content string) []Entry {
	lines := strings.Split(content, "\n")

	// Try Markdown patterns first; fall back to RST if no sections found.
	sections := parseMarkdown(lines)
	if len(sections) < 2 {
		sections = parseRST(lines)
	}
	if len(sections) < 2 {
		return nil
	}

	entries := make([]Entry, 0, len(sections))
	for _, s := range sections {
		body := strings.TrimSpace(strings.Join(s.body, "\n"))
		entries = append(entries, Entry{
			Version:     s.version,
			TagName:     s.tagName,
			Title:       s.tagName,
			Body:        body,
			PublishedAt: s.date,
		})
	}
	return entries
}

func parseMarkdown(lines []string) []parsedSection {
	var sections []parsedSection
	var current *parsedSection

	flush := func() {
		if current != nil {
			sections = append(sections, *current)
			current = nil
		}
	}

	for _, line := range lines {
		if version, date, tag, ok := matchMDHeading(line); ok {
			flush()
			if noiseVersionRE.MatchString(version) {
				// skip [Unreleased] etc. — set current to a sentinel we discard
				current = nil
				continue
			}
			current = &parsedSection{version: version, tagName: tag, date: date}
			continue
		}
		if current != nil {
			current.body = append(current.body, line)
		}
	}
	flush()
	return sections
}

// matchMDHeading tries each Markdown pattern against a line.
// Returns (normalizedVersion, date, originalTag, ok).
func matchMDHeading(line string) (version, date, tag string, ok bool) {
	for _, p := range mdPatterns {
		m := p.re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		raw := m[p.versionGroup]
		if !versionRE.MatchString(strings.TrimPrefix(raw, "v")) {
			// heading matched pattern but version string isn't version-like
			// (could be [Unreleased])
			if noiseVersionRE.MatchString(raw) {
				return raw, "", raw, true // caller handles noise check
			}
			continue
		}
		normalized := strings.TrimPrefix(raw, "v")
		d := ""
		if p.dateGroup > 0 && p.dateGroup < len(m) {
			d = m[p.dateGroup]
		}
		return normalized, d, raw, true
	}
	return "", "", "", false
}

// rstVersionLine matches "Version X.Y.Z" or bare "X.Y.Z" RST heading lines.
var rstVersionPrefixed = regexp.MustCompile(`^Version\s+(v?[0-9]+\.[0-9][0-9a-zA-Z.\-]*)\s*$`)
var rstVersionBare = regexp.MustCompile(`^(v?[0-9]+\.[0-9][0-9a-zA-Z.\-]*)\s*$`)
var rstUnderline = regexp.MustCompile(`^[-=~^#*+]{3,}\s*$`)
var rstDateLine = regexp.MustCompile(`(?:Released|Date:)\s*(\d{4}-\d{2}-\d{2})`)

func parseRST(lines []string) []parsedSection {
	var sections []parsedSection
	var current *parsedSection

	flush := func() {
		if current != nil {
			sections = append(sections, *current)
			current = nil
		}
	}

	for i, line := range lines {
		// Detect RST heading: line followed by underline of dashes/equals
		if i+1 < len(lines) && rstUnderline.MatchString(lines[i+1]) {
			var raw string
			if m := rstVersionPrefixed.FindStringSubmatch(line); m != nil {
				raw = m[1]
			} else if m := rstVersionBare.FindStringSubmatch(line); m != nil {
				raw = m[1]
			}
			if raw != "" {
				normalized := strings.TrimPrefix(raw, "v")
				flush()
				current = &parsedSection{version: normalized, tagName: raw}
				continue
			}
		}
		if current != nil {
			// Look for a date line within the first few lines of the body
			if len(current.body) < 4 {
				if m := rstDateLine.FindStringSubmatch(line); m != nil {
					current.date = m[1]
					continue
				}
			}
			current.body = append(current.body, line)
		}
	}
	flush()
	return sections
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd api && go test ./internal/changelog/... -v -run TestParse
```

Expected: all `TestParse_*` tests pass.

- [ ] **Step 5: Commit**

```bash
git add api/internal/changelog/
git commit -m "feat(changelog): implement CHANGELOG file parser with strict mode"
```

---

## Task 3: Parallel Registry (TDD)

**Files:**
- Create: `api/internal/changelog/registry_test.go`
- Create: `api/internal/changelog/registry.go`

- [ ] **Step 1: Write the failing tests**

```go
// api/internal/changelog/registry_test.go
package changelog_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pypx/api/internal/changelog"
)

// stubSource is a test implementation of Source.
type stubSource struct {
	name    string
	entries []changelog.Entry
	err     error
	delay   time.Duration
}

func (s *stubSource) Name() string { return s.name }
func (s *stubSource) Fetch(ctx context.Context) ([]changelog.Entry, error) {
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return s.entries, s.err
}

func TestRegistry_ReturnsHighestPriorityNonEmpty(t *testing.T) {
	// Priority = position in slice (index 0 = highest priority).
	// Source at index 0 is empty, source at index 1 has entries.
	// Expect entries from index 1.
	sources := []changelog.Source{
		&stubSource{name: "empty", entries: nil},
		&stubSource{name: "has_data", entries: []changelog.Entry{{Version: "1.0.0"}}},
		&stubSource{name: "also_data", entries: []changelog.Entry{{Version: "2.0.0"}}},
	}
	reg := changelog.NewRegistry(sources...)
	result := reg.Fetch(context.Background())

	if result.Source != "has_data" {
		t.Errorf("result.Source = %q, want has_data", result.Source)
	}
	if len(result.Entries) != 1 || result.Entries[0].Version != "1.0.0" {
		t.Errorf("unexpected entries: %+v", result.Entries)
	}
}

func TestRegistry_HigherPriorityWinsEvenIfSlower(t *testing.T) {
	// Index 0 has delay but data; index 1 is fast but lower priority.
	// Both run in parallel; index 0 should still win because it has data and higher priority.
	sources := []changelog.Source{
		&stubSource{name: "slow_winner", entries: []changelog.Entry{{Version: "1.0.0"}}, delay: 50 * time.Millisecond},
		&stubSource{name: "fast_loser", entries: []changelog.Entry{{Version: "2.0.0"}}},
	}
	reg := changelog.NewRegistry(sources...)
	result := reg.Fetch(context.Background())

	if result.Source != "slow_winner" {
		t.Errorf("result.Source = %q, want slow_winner", result.Source)
	}
}

func TestRegistry_AllEmpty_ReturnsNone(t *testing.T) {
	sources := []changelog.Source{
		&stubSource{name: "a", entries: nil},
		&stubSource{name: "b", entries: nil},
	}
	reg := changelog.NewRegistry(sources...)
	result := reg.Fetch(context.Background())

	if result.Source != "none" {
		t.Errorf("result.Source = %q, want none", result.Source)
	}
	if len(result.Entries) != 0 {
		t.Errorf("expected empty entries, got %+v", result.Entries)
	}
}

func TestRegistry_ErrorSourceSkipped(t *testing.T) {
	sources := []changelog.Source{
		&stubSource{name: "erroring", err: errors.New("network failure")},
		&stubSource{name: "good", entries: []changelog.Entry{{Version: "1.0.0"}}},
	}
	reg := changelog.NewRegistry(sources...)
	result := reg.Fetch(context.Background())

	if result.Source != "good" {
		t.Errorf("result.Source = %q, want good", result.Source)
	}
}

func TestRegistry_NoSources_ReturnsNone(t *testing.T) {
	reg := changelog.NewRegistry()
	result := reg.Fetch(context.Background())
	if result.Source != "none" {
		t.Errorf("result.Source = %q, want none", result.Source)
	}
}

func TestRegistry_RunsInParallel(t *testing.T) {
	// Three sources each taking 100ms. Sequential would take 300ms+.
	// Parallel should complete in ~100ms.
	sources := []changelog.Source{
		&stubSource{name: "a", entries: nil, delay: 100 * time.Millisecond},
		&stubSource{name: "b", entries: nil, delay: 100 * time.Millisecond},
		&stubSource{name: "c", entries: []changelog.Entry{{Version: "1.0.0"}}, delay: 100 * time.Millisecond},
	}
	reg := changelog.NewRegistry(sources...)
	start := time.Now()
	result := reg.Fetch(context.Background())
	elapsed := time.Since(start)

	if elapsed > 250*time.Millisecond {
		t.Errorf("fetch took %v, expected parallel execution (~100ms)", elapsed)
	}
	if result.Source != "c" {
		t.Errorf("result.Source = %q, want c", result.Source)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd api && go test ./internal/changelog/... -v -run TestRegistry
```

Expected: compilation failure — `changelog.NewRegistry` not defined.

- [ ] **Step 3: Implement the registry**

```go
// api/internal/changelog/registry.go
package changelog

import (
	"context"
	"sort"
	"sync"
)

// Registry fires all sources concurrently and returns the highest-priority
// (lowest index) non-empty result.
type Registry struct {
	sources []Source
}

// NewRegistry constructs a Registry. Sources are provided in priority order:
// index 0 is highest priority.
func NewRegistry(sources ...Source) *Registry {
	return &Registry{sources: sources}
}

type outcome struct {
	priority int
	result   Result
	err      error
}

// Fetch runs all sources in parallel, waits for all to finish, then returns
// the highest-priority source that produced non-empty entries.
// Never returns an error — individual source errors are logged/dropped.
func (r *Registry) Fetch(ctx context.Context) Result {
	if len(r.sources) == 0 {
		return Result{Source: "none"}
	}

	ch := make(chan outcome, len(r.sources))
	var wg sync.WaitGroup

	for i, src := range r.sources {
		wg.Add(1)
		go func(priority int, s Source) {
			defer wg.Done()
			entries, err := s.Fetch(ctx)
			ch <- outcome{priority: priority, result: Result{Source: s.Name(), Entries: entries}, err: err}
		}(i, src)
	}

	// Close channel once all goroutines finish.
	go func() {
		wg.Wait()
		close(ch)
	}()

	// Collect all outcomes.
	outcomes := make([]outcome, 0, len(r.sources))
	for o := range ch {
		outcomes = append(outcomes, o)
	}

	// Sort by priority (lowest index wins).
	sort.Slice(outcomes, func(i, j int) bool {
		return outcomes[i].priority < outcomes[j].priority
	})

	// Return first non-empty, non-error result.
	for _, o := range outcomes {
		if o.err == nil && len(o.result.Entries) > 0 {
			return o.result
		}
	}
	return Result{Source: "none"}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd api && go test ./internal/changelog/... -v
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add api/internal/changelog/
git commit -m "feat(changelog): implement parallel registry with priority-based selection"
```

---

## Task 4: Extend GitHub client with new methods

**Files:**
- Modify: `api/internal/github/client.go`
- Modify: `api/internal/github/client_test.go`

- [ ] **Step 1: Write failing tests for the new methods**

Append these test functions to `api/internal/github/client_test.go`:

```go
func TestFetchRawFile_ReturnsContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/contents/CHANGELOG.md" {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, "## 1.0.0\nSome content")
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := github.NewClient(github.WithBaseURL(srv.URL))
	content, filename, err := c.FetchRawFile("owner", "repo", []string{"CHANGELOG.md", "CHANGES.md"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filename != "CHANGELOG.md" {
		t.Errorf("filename = %q, want CHANGELOG.md", filename)
	}
	if content != "## 1.0.0\nSome content" {
		t.Errorf("content = %q", content)
	}
}

func TestFetchRawFile_TriesFallbackNames(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/contents/CHANGES.md" {
			fmt.Fprint(w, "## 1.0.0\nfound it")
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := github.NewClient(github.WithBaseURL(srv.URL))
	content, filename, err := c.FetchRawFile("owner", "repo", []string{"CHANGELOG.md", "CHANGES.md"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filename != "CHANGES.md" {
		t.Errorf("filename = %q, want CHANGES.md", filename)
	}
	_ = content
}

func TestFetchRawFile_NoneFound_ReturnsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := github.NewClient(github.WithBaseURL(srv.URL))
	content, filename, err := c.FetchRawFile("owner", "repo", []string{"CHANGELOG.md"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "" || filename != "" {
		t.Errorf("expected empty result, got content=%q filename=%q", content, filename)
	}
}

func TestFetchTags_ReturnsSortedVersionTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/tags" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `[
				{"name":"v1.2.0","commit":{"sha":"abc"}},
				{"name":"not-a-version","commit":{"sha":"xyz"}},
				{"name":"v1.1.0","commit":{"sha":"def"}}
			]`)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := github.NewClient(github.WithBaseURL(srv.URL))
	tags, err := c.FetchTags("owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "not-a-version" should be filtered out
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d: %+v", len(tags), tags)
	}
	if tags[0].Name != "v1.2.0" {
		t.Errorf("tags[0].Name = %q, want v1.2.0", tags[0].Name)
	}
}

func TestFetchCompare_ReturnsCommitMessages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/compare/v1.1.0...v1.2.0" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{
				"commits": [
					{"commit":{"message":"Fix unicode handling","author":{"date":"2024-03-15T10:00:00Z"}}},
					{"commit":{"message":"Merge pull request #1","author":{"date":"2024-03-15T11:00:00Z"}}},
					{"commit":{"message":"Add new feature","author":{"date":"2024-03-15T12:00:00Z"}}}
				]
			}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := github.NewClient(github.WithBaseURL(srv.URL))
	messages, headDate, err := c.FetchCompare("owner", "repo", "v1.1.0", "v1.2.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "Merge pull request #1" should be filtered as noise
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages (merge filtered), got %d: %v", len(messages), messages)
	}
	if messages[0] != "Fix unicode handling" {
		t.Errorf("messages[0] = %q", messages[0])
	}
	if headDate != "2024-03-15T12:00:00Z" {
		t.Errorf("headDate = %q, want 2024-03-15T12:00:00Z", headDate)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd api && go test ./internal/github/... -v -run "TestFetchRawFile|TestFetchTags|TestFetchCompare"
```

Expected: compilation failure — new methods not yet defined.

- [ ] **Step 3: Add new types and methods to `api/internal/github/client.go`**

Add these new types near the top of the file (after the existing public types):

```go
// Tag represents a git tag with version-like name.
type Tag struct {
	Name string // original tag name (e.g. "v1.2.0")
	SHA  string // commit SHA
}
```

Add these new private variables after the existing `priorityKeys` var:

```go
// tagVersionRE matches tag names that look like version strings.
var tagVersionRE = regexp.MustCompile(`^v?\d+\.\d[\d.a-zA-Z\-]*$`)

// noiseCommitRE matches commit messages that should be excluded from changelogs.
var noiseCommitRE = regexp.MustCompile(`(?i)^(merge\b|bump version|version bump|chore[:(]|release[:(]|prepare release|update changelog)`)
```

Add these new methods to the `Client`:

```go
// FetchRawFile fetches the raw content of the first file in the candidates list
// that exists in the repo. Returns ("", "", nil) if none are found.
func (c *Client) FetchRawFile(owner, repo string, candidates []string) (content, filename string, err error) {
	for _, name := range candidates {
		url := fmt.Sprintf("%s/repos/%s/%s/contents/%s", c.baseURL, owner, repo, name)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			continue
		}
		// Request raw content directly — no base64 decoding needed.
		req.Header.Set("Accept", "application/vnd.github.raw+json")
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			continue
		}
		if resp.StatusCode != http.StatusOK {
			continue
		}
		var buf strings.Builder
		_, err = io.Copy(&buf, resp.Body)
		if err != nil {
			continue
		}
		return buf.String(), name, nil
	}
	return "", "", nil
}

// FetchTags returns all version-like tags for the repo, preserving their original order
// from the API (typically newest-first from GitHub).
func (c *Client) FetchTags(owner, repo string) ([]Tag, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/tags?per_page=50", c.baseURL, owner, repo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	var raw []struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	tags := make([]Tag, 0, len(raw))
	for _, t := range raw {
		if tagVersionRE.MatchString(t.Name) {
			tags = append(tags, Tag{Name: t.Name, SHA: t.Commit.SHA})
		}
	}
	return tags, nil
}

// FetchCompare fetches the commits between base and head tags.
// Returns filtered commit messages and the date of the HEAD commit.
// Returns (nil, "", nil) on 404.
func (c *Client) FetchCompare(owner, repo, base, head string) (messages []string, headDate string, err error) {
	url := fmt.Sprintf("%s/repos/%s/%s/compare/%s...%s", c.baseURL, owner, repo, base, head)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, "", nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("compare API returned %d", resp.StatusCode)
	}

	var payload struct {
		Commits []struct {
			Commit struct {
				Message string `json:"message"`
				Author  struct {
					Date string `json:"date"`
				} `json:"author"`
			} `json:"commit"`
		} `json:"commits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, "", err
	}

	const maxCommits = 30
	messages = make([]string, 0, len(payload.Commits))
	for _, c := range payload.Commits {
		// Use first line of commit message only.
		msg := strings.SplitN(c.Commit.Message, "\n", 2)[0]
		if !noiseCommitRE.MatchString(msg) {
			messages = append(messages, msg)
			if len(messages) >= maxCommits {
				break
			}
		}
	}

	// headDate = date of the last commit in the list (GitHub orders oldest-first).
	if n := len(payload.Commits); n > 0 {
		headDate = payload.Commits[n-1].Commit.Author.Date
	}
	return messages, headDate, nil
}
```

Also add `"io"` to the import block in `client.go`.

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd api && go test ./internal/github/... -v -run "TestFetchRawFile|TestFetchTags|TestFetchCompare"
```

Expected: all new tests pass. Verify existing tests still pass:

```bash
cd api && go test ./internal/github/... -v
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add api/internal/github/client.go api/internal/github/client_test.go
git commit -m "feat(github): add FetchRawFile, FetchTags, FetchCompare methods"
```

---

## Task 5: GitHub sources

**Files:**
- Create: `api/internal/github/releases_source.go`
- Create: `api/internal/github/file_source.go`
- Create: `api/internal/github/tags_source.go`

No separate test files needed — the sources are thin wrappers over the client methods tested in Task 4. Integration is tested via the handler test in Task 7.

- [ ] **Step 1: Create ReleasesSource**

```go
// api/internal/github/releases_source.go
package github

import (
	"context"

	"github.com/pypx/api/internal/changelog"
)

// ReleasesSource fetches changelog entries from GitHub Releases.
// This mirrors the existing handler logic and is the highest-priority source.
type ReleasesSource struct {
	Client *Client
	Owner  string
	Repo   string
}

func (s *ReleasesSource) Name() string { return "github_releases" }

func (s *ReleasesSource) Fetch(_ context.Context) ([]changelog.Entry, error) {
	releases, err := s.Client.FetchReleases(s.Owner, s.Repo)
	if err != nil {
		return nil, err
	}
	entries := make([]changelog.Entry, 0, len(releases))
	for _, r := range releases {
		entries = append(entries, changelog.Entry{
			Version:     r.Version,
			TagName:     r.TagName,
			Title:       r.Title,
			Body:        r.Body,
			PublishedAt: r.PublishedAt,
			URL:         r.URL,
		})
	}
	return entries, nil
}
```

- [ ] **Step 2: Create FileSource**

```go
// api/internal/github/file_source.go
package github

import (
	"context"
	"fmt"

	"github.com/pypx/api/internal/changelog"
)

// candidateFilenames is the ordered list of changelog filenames to try.
var candidateFilenames = []string{
	"CHANGELOG.md", "CHANGELOG.rst", "CHANGELOG.txt", "CHANGELOG",
	"CHANGES.md", "CHANGES.rst", "CHANGES.txt", "CHANGES",
	"HISTORY.md", "HISTORY.rst", "HISTORY.txt", "HISTORY",
}

// FileSource fetches and parses a CHANGELOG/CHANGES/HISTORY file from the repo.
type FileSource struct {
	Client *Client
	Owner  string
	Repo   string
}

func (s *FileSource) Name() string { return "github_changelog_file" }

func (s *FileSource) Fetch(_ context.Context) ([]changelog.Entry, error) {
	content, _, err := s.Client.FetchRawFile(s.Owner, s.Repo, candidateFilenames)
	if err != nil || content == "" {
		return nil, err
	}
	entries := changelog.Parse(content)
	for i := range entries {
		// Construct a URL pointing to the file on GitHub so users can view it.
		entries[i].URL = fmt.Sprintf("https://github.com/%s/%s/blob/HEAD/CHANGELOG.md", s.Owner, s.Repo)
	}
	return entries, nil
}
```

- [ ] **Step 3: Create TagsSource**

```go
// api/internal/github/tags_source.go
package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/pypx/api/internal/changelog"
)

// TagsSource builds changelog entries from commits between consecutive git tags.
type TagsSource struct {
	Client *Client
	Owner  string
	Repo   string
}

func (s *TagsSource) Name() string { return "github_tags" }

func (s *TagsSource) Fetch(ctx context.Context) ([]changelog.Entry, error) {
	tags, err := s.Client.FetchTags(s.Owner, s.Repo)
	if err != nil || len(tags) < 2 {
		return nil, err
	}

	// tags is ordered newest-first from GitHub.
	// For each pair (tags[i], tags[i+1]), compare tags[i+1]...tags[i].
	entries := make([]changelog.Entry, 0, len(tags)-1)
	for i := 0; i < len(tags)-1; i++ {
		newer := tags[i]
		older := tags[i+1]

		messages, headDate, err := s.Client.FetchCompare(s.Owner, s.Repo, older.Name, newer.Name)
		if err != nil {
			// Skip this pair on error rather than failing the whole source.
			continue
		}

		var body string
		if len(messages) > 0 {
			var sb strings.Builder
			for _, m := range messages {
				sb.WriteString("- ")
				sb.WriteString(m)
				sb.WriteString("\n")
			}
			body = sb.String()
		}

		// Strip leading v for normalized version.
		version := strings.TrimPrefix(newer.Name, "v")

		entries = append(entries, changelog.Entry{
			Version:     version,
			TagName:     newer.Name,
			Title:       newer.Name,
			Body:        body,
			PublishedAt: headDate,
			URL:         fmt.Sprintf("https://github.com/%s/%s/compare/%s...%s", s.Owner, s.Repo, older.Name, newer.Name),
		})
	}
	return entries, nil
}
```

- [ ] **Step 4: Verify the package compiles**

```bash
cd api && go build ./internal/github/...
```

Expected: no output.

- [ ] **Step 5: Commit**

```bash
git add api/internal/github/releases_source.go api/internal/github/file_source.go api/internal/github/tags_source.go
git commit -m "feat(github): implement ReleasesSource, FileSource, TagsSource"
```

---

## Task 6: GitLab client and sources

**Files:**
- Create: `api/internal/gitlab/client.go`
- Create: `api/internal/gitlab/client_test.go`
- Create: `api/internal/gitlab/releases_source.go`
- Create: `api/internal/gitlab/file_source.go`
- Create: `api/internal/gitlab/tags_source.go`

- [ ] **Step 1: Write failing tests for the GitLab client**

```go
// api/internal/gitlab/client_test.go
package gitlab_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pypx/api/internal/gitlab"
)

func TestExtractGitLabRepo(t *testing.T) {
	tests := []struct {
		name        string
		projectURLs map[string]string
		wantPath    string
		wantOK      bool
	}{
		{
			name:        "source key with gitlab URL",
			projectURLs: map[string]string{"Source": "https://gitlab.com/user/myproject"},
			wantPath:    "user/myproject",
			wantOK:      true,
		},
		{
			name:        "repository key case insensitive",
			projectURLs: map[string]string{"repository": "https://gitlab.com/group/sub/project"},
			wantPath:    "group/sub/project",
			wantOK:      true,
		},
		{
			name:        "github URL not matched",
			projectURLs: map[string]string{"Source": "https://github.com/user/repo"},
			wantPath:    "",
			wantOK:      false,
		},
		{
			name:        "no URLs",
			projectURLs: map[string]string{},
			wantPath:    "",
			wantOK:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, _, ok := gitlab.ExtractGitLabRepo(tt.projectURLs)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if path != tt.wantPath {
				t.Errorf("path = %q, want %q", path, tt.wantPath)
			}
		})
	}
}

func TestGitLabClient_FetchReleases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/projects/user%2Frepo/releases" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `[
				{"tag_name":"v1.0.0","name":"Release 1.0.0","description":"Some notes","released_at":"2024-01-01T00:00:00Z"}
			]`)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := gitlab.NewClient(gitlab.WithBaseURL(srv.URL))
	releases, err := c.FetchReleases("user/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(releases) != 1 {
		t.Fatalf("expected 1 release, got %d", len(releases))
	}
	if releases[0].Version != "1.0.0" {
		t.Errorf("Version = %q, want 1.0.0", releases[0].Version)
	}
}

func TestGitLabClient_FetchRawFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/projects/user%2Frepo/repository/files/CHANGELOG.md/raw" {
			fmt.Fprint(w, "## 1.0.0\ncontent")
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := gitlab.NewClient(gitlab.WithBaseURL(srv.URL))
	content, filename, err := c.FetchRawFile("user/repo", []string{"CHANGELOG.md", "CHANGES.md"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filename != "CHANGELOG.md" {
		t.Errorf("filename = %q, want CHANGELOG.md", filename)
	}
	_ = content
}

func TestGitLabClient_FetchTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/projects/user%2Frepo/repository/tags" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `[
				{"name":"v1.1.0","commit":{"id":"abc"}},
				{"name":"not-a-version","commit":{"id":"xyz"}},
				{"name":"v1.0.0","commit":{"id":"def"}}
			]`)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := gitlab.NewClient(gitlab.WithBaseURL(srv.URL))
	tags, err := c.FetchTags("user/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 version tags, got %d", len(tags))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd api && go test ./internal/gitlab/... -v
```

Expected: compilation failure — package does not exist.

- [ ] **Step 3: Implement the GitLab client**

```go
// api/internal/gitlab/client.go
package gitlab

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var gitlabURLPattern = regexp.MustCompile(
	`(?i)gitlab\.com/([A-Za-z0-9_.\-/]+?)(?:\.git|/?$)`,
)

var priorityKeys = []string{
	"source",
	"repository",
	"source code",
	"gitlab",
	"homepage",
	"code",
}

// ExtractGitLabRepo extracts the GitLab project path from PyPI project_urls.
// Returns (projectPath, repoURL, ok). projectPath is like "user/repo" or "group/sub/repo".
func ExtractGitLabRepo(projectURLs map[string]string) (projectPath, repoURL string, ok bool) {
	lower := make(map[string]string, len(projectURLs))
	for k, v := range projectURLs {
		lower[strings.ToLower(k)] = v
	}

	var candidates []string
	for _, key := range priorityKeys {
		if v, exists := lower[key]; exists {
			candidates = append(candidates, v)
		}
	}
	// Also scan all values as fallback.
	for _, v := range projectURLs {
		candidates = append(candidates, v)
	}

	for _, rawURL := range candidates {
		if m := gitlabURLPattern.FindStringSubmatch(rawURL); m != nil {
			path := strings.TrimSuffix(m[1], "/")
			// Require at least one slash (namespace/project).
			if !strings.Contains(path, "/") {
				continue
			}
			return path, "https://gitlab.com/" + path, true
		}
	}
	return "", "", false
}

// Release holds a normalized GitLab release.
type Release struct {
	Version     string
	TagName     string
	Title       string
	Body        string
	PublishedAt string
	URL         string
}

// Tag represents a git tag.
type Tag struct {
	Name string
	SHA  string
}

// Client is a minimal GitLab API client.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type Option func(*Client)

func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = u }
}

func WithToken(tok string) Option {
	return func(c *Client) { c.token = tok }
}

func NewClient(opts ...Option) *Client {
	c := &Client{
		baseURL:    "https://gitlab.com",
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

func (c *Client) encodedPath(projectPath string) string {
	return url.PathEscape(projectPath)
}

func (c *Client) get(path string) (*http.Response, error) {
	req, err := http.NewRequest("GET", c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("PRIVATE-TOKEN", c.token)
	}
	return c.httpClient.Do(req)
}

var tagVersionRE = regexp.MustCompile(`^v?\d+\.\d[\d.a-zA-Z\-]*$`)

// FetchReleases fetches GitLab releases for the given project path.
func (c *Client) FetchReleases(projectPath string) ([]Release, error) {
	resp, err := c.get(fmt.Sprintf("/api/v4/projects/%s/releases", c.encodedPath(projectPath)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gitlab releases API returned %d", resp.StatusCode)
	}

	var raw []struct {
		TagName     string `json:"tag_name"`
		Name        string `json:"name"`
		Description string `json:"description"`
		ReleasedAt  string `json:"released_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	releases := make([]Release, 0, len(raw))
	for _, r := range raw {
		version := strings.TrimPrefix(r.TagName, "v")
		releases = append(releases, Release{
			Version:     version,
			TagName:     r.TagName,
			Title:       r.Name,
			Body:        r.Description,
			PublishedAt: r.ReleasedAt,
			URL:         fmt.Sprintf("https://gitlab.com/%s/-/releases/%s", projectPath, r.TagName),
		})
	}
	return releases, nil
}

// FetchRawFile fetches the first file in candidates that exists in the repo.
func (c *Client) FetchRawFile(projectPath string, candidates []string) (content, filename string, err error) {
	for _, name := range candidates {
		path := fmt.Sprintf("/api/v4/projects/%s/repository/files/%s/raw?ref=HEAD",
			c.encodedPath(projectPath), url.PathEscape(name))
		resp, err := c.get(path)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			continue
		}
		if resp.StatusCode != http.StatusOK {
			continue
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}
		return string(body), name, nil
	}
	return "", "", nil
}

// FetchTags returns version-like tags for the project, preserving API order (newest-first).
func (c *Client) FetchTags(projectPath string) ([]Tag, error) {
	resp, err := c.get(fmt.Sprintf("/api/v4/projects/%s/repository/tags?per_page=50", c.encodedPath(projectPath)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	var raw []struct {
		Name   string `json:"name"`
		Commit struct {
			ID string `json:"id"`
		} `json:"commit"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	tags := make([]Tag, 0, len(raw))
	for _, t := range raw {
		if tagVersionRE.MatchString(t.Name) {
			tags = append(tags, Tag{Name: t.Name, SHA: t.Commit.ID})
		}
	}
	return tags, nil
}

var noiseCommitRE = regexp.MustCompile(`(?i)^(merge\b|bump version|version bump|chore[:(]|release[:(]|prepare release|update changelog)`)

// FetchCompare returns filtered commit messages and the date of the HEAD commit
// between base and head refs.
func (c *Client) FetchCompare(projectPath, base, head string) (messages []string, headDate string, err error) {
	path := fmt.Sprintf("/api/v4/projects/%s/repository/compare?from=%s&to=%s",
		c.encodedPath(projectPath), url.QueryEscape(base), url.QueryEscape(head))
	resp, err := c.get(path)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, "", nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("gitlab compare API returned %d", resp.StatusCode)
	}

	var payload struct {
		Commits []struct {
			Message       string `json:"message"`
			AuthoredDate  string `json:"authored_date"`
		} `json:"commits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, "", err
	}

	const maxCommits = 30
	messages = make([]string, 0, len(payload.Commits))
	for _, cm := range payload.Commits {
		msg := strings.SplitN(cm.Message, "\n", 2)[0]
		if !noiseCommitRE.MatchString(msg) {
			messages = append(messages, msg)
			if len(messages) >= maxCommits {
				break
			}
		}
	}

	if n := len(payload.Commits); n > 0 {
		headDate = payload.Commits[n-1].AuthoredDate
	}
	return messages, headDate, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd api && go test ./internal/gitlab/... -v
```

Expected: all tests pass.

- [ ] **Step 5: Implement GitLab sources**

```go
// api/internal/gitlab/releases_source.go
package gitlab

import (
	"context"
	"github.com/pypx/api/internal/changelog"
)

type ReleasesSource struct {
	Client      *Client
	ProjectPath string
	RepoURL     string
}

func (s *ReleasesSource) Name() string { return "gitlab_releases" }

func (s *ReleasesSource) Fetch(_ context.Context) ([]changelog.Entry, error) {
	releases, err := s.Client.FetchReleases(s.ProjectPath)
	if err != nil {
		return nil, err
	}
	entries := make([]changelog.Entry, 0, len(releases))
	for _, r := range releases {
		entries = append(entries, changelog.Entry{
			Version:     r.Version,
			TagName:     r.TagName,
			Title:       r.Title,
			Body:        r.Body,
			PublishedAt: r.PublishedAt,
			URL:         r.URL,
		})
	}
	return entries, nil
}
```

```go
// api/internal/gitlab/file_source.go
package gitlab

import (
	"context"
	"fmt"
	"github.com/pypx/api/internal/changelog"
)

var candidateFilenames = []string{
	"CHANGELOG.md", "CHANGELOG.rst", "CHANGELOG.txt", "CHANGELOG",
	"CHANGES.md", "CHANGES.rst", "CHANGES.txt", "CHANGES",
	"HISTORY.md", "HISTORY.rst", "HISTORY.txt", "HISTORY",
}

type FileSource struct {
	Client      *Client
	ProjectPath string
}

func (s *FileSource) Name() string { return "gitlab_changelog_file" }

func (s *FileSource) Fetch(_ context.Context) ([]changelog.Entry, error) {
	content, _, err := s.Client.FetchRawFile(s.ProjectPath, candidateFilenames)
	if err != nil || content == "" {
		return nil, err
	}
	entries := changelog.Parse(content)
	for i := range entries {
		entries[i].URL = fmt.Sprintf("https://gitlab.com/%s/-/blob/HEAD/CHANGELOG.md", s.ProjectPath)
	}
	return entries, nil
}
```

```go
// api/internal/gitlab/tags_source.go
package gitlab

import (
	"context"
	"fmt"
	"strings"

	"github.com/pypx/api/internal/changelog"
)

type TagsSource struct {
	Client      *Client
	ProjectPath string
}

func (s *TagsSource) Name() string { return "gitlab_tags" }

func (s *TagsSource) Fetch(ctx context.Context) ([]changelog.Entry, error) {
	tags, err := s.Client.FetchTags(s.ProjectPath)
	if err != nil || len(tags) < 2 {
		return nil, err
	}

	entries := make([]changelog.Entry, 0, len(tags)-1)
	for i := 0; i < len(tags)-1; i++ {
		newer := tags[i]
		older := tags[i+1]

		messages, headDate, err := s.Client.FetchCompare(s.ProjectPath, older.Name, newer.Name)
		if err != nil {
			continue
		}

		var body string
		if len(messages) > 0 {
			var sb strings.Builder
			for _, m := range messages {
				sb.WriteString("- ")
				sb.WriteString(m)
				sb.WriteString("\n")
			}
			body = sb.String()
		}

		version := strings.TrimPrefix(newer.Name, "v")
		entries = append(entries, changelog.Entry{
			Version:     version,
			TagName:     newer.Name,
			Title:       newer.Name,
			Body:        body,
			PublishedAt: headDate,
			URL:         fmt.Sprintf("https://gitlab.com/%s/-/compare/%s...%s", s.ProjectPath, older.Name, newer.Name),
		})
	}
	return entries, nil
}
```

- [ ] **Step 6: Verify all GitLab code compiles**

```bash
cd api && go build ./internal/gitlab/...
```

Expected: no output.

- [ ] **Step 7: Commit**

```bash
git add api/internal/gitlab/
git commit -m "feat(gitlab): add GitLab client and changelog sources"
```

---

## Task 7: Refactor the changelog handler

**Files:**
- Modify: `api/internal/handler/changelog.go`
- Modify: `api/internal/handler/changelog_test.go`
- Modify: `api/cmd/server/main.go`

- [ ] **Step 1: Write new handler tests covering the fallback sources**

Replace the content of `api/internal/handler/changelog_test.go` with:

```go
package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	gh "github.com/pypx/api/internal/github"
	"github.com/pypx/api/internal/gitlab"
	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/pypi"
)

// pypiResponse builds a minimal PyPI JSON response with the given project URL.
func pypiResponse(sourceURL string) string {
	return `{"info":{"name":"testpkg","version":"1.0.0","summary":"","description":"",` +
		`"description_content_type":"","license":"","author":"","author_email":"",` +
		`"home_page":"","requires_python":"","requires_dist":null,` +
		`"project_urls":{"Source":"` + sourceURL + `"}},"releases":{},"urls":[]}`
}

func setupChangelogRouter(ghBase, glBase string) (*chi.Mux, *cache.MemoryCache) {
	sqliteCache, _ := cache.New(":memory:")
	c := cache.NewMemoryCache(sqliteCache, 100)
	pypiClient := pypi.NewClient()
	pkgHandler := handler.NewPackageHandler(pypiClient, c)
	ghClient := gh.NewClient(gh.WithBaseURL(ghBase))
	glClient := gitlab.NewClient(gitlab.WithBaseURL(glBase))
	changelogHandler := handler.NewChangelogHandler(ghClient, glClient, c, pkgHandler)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/changelog", changelogHandler.Get)
	return r, c
}

func TestChangelogGet_GitHubReleases(t *testing.T) {
	ghSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/releases":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[{"tag_name":"v1.0.0","name":"Release 1.0","body":"## Changes\n- Fix foo","published_at":"2024-01-01T00:00:00Z","html_url":"https://github.com/owner/repo/releases/tag/v1.0.0"}]`))
		case "/repos/owner/repo":
			w.Write([]byte(`{"stargazers_count":10,"forks_count":2,"open_issues_count":1,"pushed_at":"2024-01-01T00:00:00Z","owner":{"login":"owner","type":"User","avatar_url":""}}`))
		case "/users/owner":
			w.Write([]byte(`{"name":"Owner Name"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ghSrv.Close()

	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(pypiResponse("https://github.com/owner/repo")))
	}))
	defer pypiSrv.Close()

	// Override pypi base URL (requires pypi.WithBaseURL — see note below)
	// For this test, use a custom pypi client pointing at pypiSrv.
	sqliteCache, _ := cache.New(":memory:")
	c := cache.NewMemoryCache(sqliteCache, 100)
	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	pkgHandler := handler.NewPackageHandler(pypiClient, c)
	ghClient := gh.NewClient(gh.WithBaseURL(ghSrv.URL))
	glClient := gitlab.NewClient()
	changelogHandler := handler.NewChangelogHandler(ghClient, glClient, c, pkgHandler)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/changelog", changelogHandler.Get)

	req := httptest.NewRequest("GET", "/api/packages/testpkg/changelog", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Source  string `json:"source"`
		Entries []struct {
			Version string `json:"version"`
		} `json:"entries"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.Source != "github_releases" {
		t.Errorf("source = %q, want github_releases", resp.Source)
	}
	if len(resp.Entries) == 0 {
		t.Error("expected at least one entry")
	}
}

func TestChangelogGet_FallsBackToChangelogFile(t *testing.T) {
	ghSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/releases":
			// No releases
			w.Write([]byte(`[]`))
		case "/repos/owner/repo/contents/CHANGELOG.md":
			w.Write([]byte("## [1.0.0] - 2024-01-01\nFixed stuff.\n\n## [0.9.0] - 2023-12-01\nBeta."))
		case "/repos/owner/repo/tags":
			w.Write([]byte(`[]`))
		case "/repos/owner/repo":
			w.Write([]byte(`{"stargazers_count":5,"forks_count":1,"open_issues_count":0,"pushed_at":"2024-01-01T00:00:00Z","owner":{"login":"owner","type":"User","avatar_url":""}}`))
		case "/users/owner":
			w.Write([]byte(`{"name":"Owner"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ghSrv.Close()

	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(pypiResponse("https://github.com/owner/repo")))
	}))
	defer pypiSrv.Close()

	sqliteCache, _ := cache.New(":memory:")
	c := cache.NewMemoryCache(sqliteCache, 100)
	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	pkgHandler := handler.NewPackageHandler(pypiClient, c)
	ghClient := gh.NewClient(gh.WithBaseURL(ghSrv.URL))
	glClient := gitlab.NewClient()
	changelogHandler := handler.NewChangelogHandler(ghClient, glClient, c, pkgHandler)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/changelog", changelogHandler.Get)

	req := httptest.NewRequest("GET", "/api/packages/testpkg/changelog", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Source  string `json:"source"`
		Entries []struct{ Version string `json:"version"` } `json:"entries"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Source != "github_changelog_file" {
		t.Errorf("source = %q, want github_changelog_file", resp.Source)
	}
	if len(resp.Entries) == 0 {
		t.Error("expected entries from CHANGELOG file")
	}
}

func TestChangelogGet_FallsBackToTags(t *testing.T) {
	ghSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/releases":
			w.Write([]byte(`[]`))
		case "/repos/owner/repo/tags":
			w.Write([]byte(`[{"name":"v1.1.0","commit":{"sha":"abc"}},{"name":"v1.0.0","commit":{"sha":"def"}}]`))
		case "/repos/owner/repo/compare/v1.0.0...v1.1.0":
			w.Write([]byte(`{"commits":[{"commit":{"message":"Add feature","author":{"date":"2024-03-01T00:00:00Z"}}}]}`))
		case "/repos/owner/repo":
			w.Write([]byte(`{"stargazers_count":0,"forks_count":0,"open_issues_count":0,"pushed_at":"2024-01-01T00:00:00Z","owner":{"login":"owner","type":"User","avatar_url":""}}`))
		case "/users/owner":
			w.Write([]byte(`{"name":"Owner"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ghSrv.Close()

	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(pypiResponse("https://github.com/owner/repo")))
	}))
	defer pypiSrv.Close()

	sqliteCache, _ := cache.New(":memory:")
	c := cache.NewMemoryCache(sqliteCache, 100)
	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	pkgHandler := handler.NewPackageHandler(pypiClient, c)
	ghClient := gh.NewClient(gh.WithBaseURL(ghSrv.URL))
	glClient := gitlab.NewClient()
	changelogHandler := handler.NewChangelogHandler(ghClient, glClient, c, pkgHandler)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/changelog", changelogHandler.Get)

	req := httptest.NewRequest("GET", "/api/packages/testpkg/changelog", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp struct {
		Source  string `json:"source"`
		Entries []struct{ Version string `json:"version"` } `json:"entries"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Source != "github_tags" {
		t.Errorf("source = %q, want github_tags", resp.Source)
	}
}

func TestChangelogGet_NoRepoURL_ReturnsEmpty(t *testing.T) {
	pypiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// No Source URL in project_urls
		w.Write([]byte(`{"info":{"name":"testpkg","version":"1.0.0","summary":"","description":"","description_content_type":"","license":"","author":"","author_email":"","home_page":"","requires_python":"","requires_dist":null,"project_urls":{}},"releases":{},"urls":[]}`))
	}))
	defer pypiSrv.Close()

	sqliteCache, _ := cache.New(":memory:")
	c := cache.NewMemoryCache(sqliteCache, 100)
	pypiClient := pypi.NewClient(pypi.WithBaseURL(pypiSrv.URL))
	pkgHandler := handler.NewPackageHandler(pypiClient, c)
	ghClient := gh.NewClient()
	glClient := gitlab.NewClient()
	changelogHandler := handler.NewChangelogHandler(ghClient, glClient, c, pkgHandler)

	r := chi.NewRouter()
	r.Get("/api/packages/{name}/changelog", changelogHandler.Get)

	req := httptest.NewRequest("GET", "/api/packages/testpkg/changelog", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp struct {
		Entries []struct{} `json:"entries"`
		Source  string     `json:"source"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Entries) != 0 {
		t.Errorf("expected empty entries for package without repo URL")
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd api && go test ./internal/handler/... -v -run TestChangelog
```

Expected: compilation failure — `NewChangelogHandler` signature mismatch.

- [ ] **Step 3: Rewrite `api/internal/handler/changelog.go`**

```go
// api/internal/handler/changelog.go
package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/changelog"
	gh "github.com/pypx/api/internal/github"
	"github.com/pypx/api/internal/gitlab"
	"github.com/pypx/api/internal/markdown"
)

const changelogTTL = 7 * 24 * time.Hour

// ChangelogResponse is the JSON shape returned to the frontend.
// The Entries field uses changelog.Entry so all sources share one type.
type ChangelogResponse struct {
	Package  string             `json:"package"`
	Source   string             `json:"source"`
	RepoURL  string             `json:"repo_url"`
	Entries  []changelog.Entry  `json:"entries"`
	RepoInfo *gh.RepoInfo       `json:"repo_info,omitempty"`
}

// ChangelogHandler handles GET /api/packages/{name}/changelog.
type ChangelogHandler struct {
	github *gh.Client
	gitlab *gitlab.Client
	cache  cache.Cacher
	pkg    *PackageHandler
}

func NewChangelogHandler(ghClient *gh.Client, glClient *gitlab.Client, c cache.Cacher, pkgHandler *PackageHandler) *ChangelogHandler {
	return &ChangelogHandler{
		github: ghClient,
		gitlab: glClient,
		cache:  c,
		pkg:    pkgHandler,
	}
}

func (h *ChangelogHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(chi.URLParam(r, "name"))
	if name == "" {
		http.Error(w, "package name required", http.StatusBadRequest)
		return
	}

	cacheKey := "changelog:" + name

	// Serve from cache if fresh.
	if data, fresh, err := h.cache.Get(cacheKey, changelogTTL); err == nil && fresh && data != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=604800")
		w.Write(data)
		return
	}

	// Fetch PyPI package info to get project URLs.
	pkg, err := h.pkg.FetchPackage(r.Context(), name)
	if err != nil {
		http.Error(w, "package not found", http.StatusNotFound)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	resp := h.buildResponse(ctx, name, pkg.Info.ProjectURLs)

	data, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := h.cache.Set(cacheKey, data, changelogTTL); err != nil {
		log.Printf("changelog cache set error: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=604800")
	w.Write(data)
}

// buildResponse constructs a ChangelogResponse using the parallel registry.
func (h *ChangelogHandler) buildResponse(ctx context.Context, pkgName string, projectURLs map[string]string) ChangelogResponse {
	// Detect GitHub.
	if owner, repo, ok := gh.ExtractGitHubRepo(projectURLs); ok {
		repoURL := "https://github.com/" + owner + "/" + repo
		sources := []changelog.Source{
			&gh.ReleasesSource{Client: h.github, Owner: owner, Repo: repo},
			&gh.FileSource{Client: h.github, Owner: owner, Repo: repo},
			&gh.TagsSource{Client: h.github, Owner: owner, Repo: repo},
		}
		reg := changelog.NewRegistry(sources...)
		result := reg.Fetch(ctx)

		// Render markdown bodies to HTML.
		entries := renderHTML(result.Entries)

		// Fetch repo metadata (stars, forks, etc.) for the sidebar.
		repoInfo, _ := h.github.FetchRepoInfo(owner, repo)

		return ChangelogResponse{
			Package:  pkgName,
			Source:   result.Source,
			RepoURL:  repoURL,
			Entries:  entries,
			RepoInfo: repoInfo,
		}
	}

	// Detect GitLab.
	if projectPath, repoURL, ok := gitlab.ExtractGitLabRepo(projectURLs); ok {
		sources := []changelog.Source{
			&gitlab.ReleasesSource{Client: h.gitlab, ProjectPath: projectPath, RepoURL: repoURL},
			&gitlab.FileSource{Client: h.gitlab, ProjectPath: projectPath},
			&gitlab.TagsSource{Client: h.gitlab, ProjectPath: projectPath},
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

	// No recognizable repo URL.
	return ChangelogResponse{
		Package: pkgName,
		Source:  "none",
		Entries: []changelog.Entry{},
	}
}

// renderHTML renders Body markdown to BodyHTML for each entry in-place.
func renderHTML(entries []changelog.Entry) []changelog.Entry {
	for i := range entries {
		if entries[i].Body != "" {
			html, err := markdown.Render(entries[i].Body)
			if err == nil {
				entries[i].BodyHTML = html
			}
		}
	}
	return entries
}
```

- [ ] **Step 4: Update `api/cmd/server/main.go`**

Find the section that initializes the changelog handler and add the gitlab client:

```go
// Add after ghClient initialization:
glClient := gitlab.NewClient()
// If GITLAB_TOKEN env var is set, pass it:
// if tok := os.Getenv("GITLAB_TOKEN"); tok != "" {
//     glClient = gitlab.NewClient(gitlab.WithToken(tok))
// }

// Update the NewChangelogHandler call:
changelogHandler := handler.NewChangelogHandler(ghClient, glClient, c, pkgHandler)
```

Also add to imports:
```go
"github.com/pypx/api/internal/gitlab"
```

- [ ] **Step 5: Run all handler tests**

```bash
cd api && go test ./internal/handler/... -v -run TestChangelog
```

Expected: all changelog tests pass.

- [ ] **Step 6: Run the full test suite**

```bash
cd api && go test ./...
```

Expected: all tests pass.

- [ ] **Step 7: Commit**

```bash
git add api/internal/handler/changelog.go api/internal/handler/changelog_test.go api/cmd/server/main.go
git commit -m "feat(handler): refactor changelog handler to use parallel source registry"
```

---

## Task 8: Frontend source badge

**Files:**
- Modify: `web/app/components/PackageVersions.vue`

- [ ] **Step 1: Add `sourceBadgeLabel` computed and badge template**

Open `web/app/components/PackageVersions.vue`. Find the `<script setup>` block and add this computed after the existing `changelogMap` computed:

```typescript
const sourceBadgeLabel = computed(() => {
  const labels: Record<string, string> = {
    github_changelog_file: 'via CHANGELOG',
    github_tags: 'via git tags',
    gitlab_releases: 'via GitLab',
    gitlab_changelog_file: 'via CHANGELOG',
    gitlab_tags: 'via git tags',
  }
  return labels[changelog.value?.source ?? ''] ?? ''
})

const showSourceBadge = computed(() =>
  sourceBadgeLabel.value !== '' && (changelog.value?.entries?.length ?? 0) > 0
)
```

- [ ] **Step 2: Add the badge to the template**

Find the versions table in the template. Directly above the `<table>` element, add:

```html
<!-- Source badge: shown when changelog data comes from a non-default source -->
<div v-if="showSourceBadge" class="flex items-center justify-end mb-2">
  <span class="flex items-center gap-1 bg-zinc-800 border border-zinc-700 text-zinc-400 text-[10px] px-1.5 py-0.5 rounded font-mono">
    <!-- Tag icon for tag-based sources -->
    <svg
      v-if="changelog?.source?.includes('tags')"
      xmlns="http://www.w3.org/2000/svg"
      class="w-2.5 h-2.5"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"/>
      <line x1="7" y1="7" x2="7.01" y2="7"/>
    </svg>
    <!-- Document icon for file-based sources -->
    <svg
      v-else
      xmlns="http://www.w3.org/2000/svg"
      class="w-2.5 h-2.5"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
      <polyline points="14 2 14 8 20 8"/>
    </svg>
    {{ sourceBadgeLabel }}
  </span>
</div>
```

- [ ] **Step 3: Start the dev server and verify the badge appears**

```bash
cd web && pnpm dev
```

Navigate to a package that uses git tags (e.g. `six` if the API is running, or any package). Verify:
- The badge appears above the versions table when source is not `github_releases`
- The badge does NOT appear for packages with GitHub Releases
- The badge does NOT appear when there are no changelog entries

- [ ] **Step 4: Commit**

```bash
git add web/app/components/PackageVersions.vue
git commit -m "feat(web): add source badge to versions table for non-release changelog data"
```

---

## Notes for implementors

**`pypi.WithBaseURL`:** The handler tests assume `pypi.NewClient` accepts a `WithBaseURL` option (same pattern as the GitHub client). If this option doesn't exist yet, add it to `api/internal/pypi/client.go` following the same `Option func(*Client)` pattern used in `api/internal/github/client.go`.

**`pkg.Info.ProjectURLs`:** The handler calls `h.pkg.FetchPackage(ctx, name)` and accesses `pkg.Info.ProjectURLs`. Verify the exact field path by checking `api/internal/handler/packages.go` — adjust if the field name differs.

**Context timeout:** The 15-second timeout in `buildResponse` covers the entire registry fetch. Individual source failures are dropped (not propagated), so a single slow or failing source doesn't break the response.

**GitLab token:** The implementation reads `GITHUB_TOKEN` already. Add `GITLAB_TOKEN` environment variable support in `main.go` following the same pattern.
