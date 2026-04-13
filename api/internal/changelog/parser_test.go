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
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (Unreleased skipped), got %d", len(entries))
	}
	if entries[0].Version != "1.0.0" {
		t.Errorf("entry[0].Version = %q, want 1.0.0", entries[0].Version)
	}
}

func TestParse_StrictMode_TooFewSections(t *testing.T) {
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
