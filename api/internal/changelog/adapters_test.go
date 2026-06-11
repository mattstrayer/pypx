package changelog_test

import (
	"context"
	"errors"
	"testing"

	"github.com/pypx/api/internal/changelog"
)

// ---- ReleasesAdapter tests ----

func TestReleasesAdapter_Name(t *testing.T) {
	a := &changelog.ReleasesAdapter{
		SourceName: "github_releases",
		FetchFn: func(_ context.Context) ([]changelog.ReleaseData, error) {
			return nil, nil
		},
	}
	if got := a.Name(); got != "github_releases" {
		t.Fatalf("Name() = %q, want %q", got, "github_releases")
	}
}

func TestReleasesAdapter_HappyPath(t *testing.T) {
	a := &changelog.ReleasesAdapter{
		SourceName: "github_releases",
		FetchFn: func(_ context.Context) ([]changelog.ReleaseData, error) {
			return []changelog.ReleaseData{
				{Version: "1.0.0", TagName: "v1.0.0", Title: "First release", Body: "body", PublishedAt: "2024-01-01", URL: "https://example.com"},
			}, nil
		},
	}
	entries, err := a.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	e := entries[0]
	if e.Version != "1.0.0" || e.TagName != "v1.0.0" || e.Title != "First release" || e.Body != "body" || e.PublishedAt != "2024-01-01" || e.URL != "https://example.com" {
		t.Errorf("entry fields mismatch: %+v", e)
	}
}

func TestReleasesAdapter_ErrorPropagation(t *testing.T) {
	want := errors.New("upstream error")
	a := &changelog.ReleasesAdapter{
		SourceName: "github_releases",
		FetchFn: func(_ context.Context) ([]changelog.ReleaseData, error) {
			return nil, want
		},
	}
	entries, err := a.Fetch(context.Background())
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if entries != nil {
		t.Fatalf("expected nil entries on error, got %v", entries)
	}
}

func TestReleasesAdapter_EmptyInput(t *testing.T) {
	a := &changelog.ReleasesAdapter{
		SourceName: "github_releases",
		FetchFn: func(_ context.Context) ([]changelog.ReleaseData, error) {
			return []changelog.ReleaseData{}, nil
		},
	}
	entries, err := a.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

// ---- FileAdapter tests ----

func TestFileAdapter_Name(t *testing.T) {
	a := &changelog.FileAdapter{
		SourceName: "github_changelog_file",
		FetchFn: func(_ context.Context) (string, string, error) {
			return "", "", nil
		},
	}
	if got := a.Name(); got != "github_changelog_file" {
		t.Fatalf("Name() = %q, want %q", got, "github_changelog_file")
	}
}

func TestFileAdapter_HappyPath(t *testing.T) {
	a := &changelog.FileAdapter{
		SourceName: "github_changelog_file",
		FetchFn: func(_ context.Context) (string, string, error) {
			// Parser requires at least 2 sections to return entries.
			return "## 1.1.0 - 2024-02-01\n\nAdded feature.\n\n## 1.0.0 - 2024-01-01\n\nInitial release.\n", "CHANGELOG.md", nil
		},
		URLFmt:     "https://github.com/{ref}/blob/HEAD/{file}",
		ProjectRef: "owner/repo",
	}
	entries, err := a.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one entry")
	}
	for _, e := range entries {
		if e.URL != "https://github.com/owner/repo/blob/HEAD/CHANGELOG.md" {
			t.Errorf("URL = %q, want expected URL", e.URL)
		}
	}
}

func TestFileAdapter_ErrorPropagation(t *testing.T) {
	want := errors.New("fetch error")
	a := &changelog.FileAdapter{
		SourceName: "github_changelog_file",
		FetchFn: func(_ context.Context) (string, string, error) {
			return "", "", want
		},
	}
	entries, err := a.Fetch(context.Background())
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if entries != nil {
		t.Fatalf("expected nil entries on error, got %v", entries)
	}
}

func TestFileAdapter_EmptyContent(t *testing.T) {
	a := &changelog.FileAdapter{
		SourceName: "github_changelog_file",
		FetchFn: func(_ context.Context) (string, string, error) {
			return "", "", nil
		},
	}
	entries, err := a.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries != nil {
		t.Fatalf("expected nil entries for empty content, got %v", entries)
	}
}

// ---- TagsAdapter tests ----

func TestTagsAdapter_Name(t *testing.T) {
	a := &changelog.TagsAdapter{
		SourceName: "github_tags",
		FetchTagsFn: func(_ context.Context) ([]changelog.TagData, error) {
			return nil, nil
		},
		FetchCompareFn: func(_ context.Context, older, newer string) (changelog.CompareResult, error) {
			return changelog.CompareResult{}, nil
		},
	}
	if got := a.Name(); got != "github_tags" {
		t.Fatalf("Name() = %q, want %q", got, "github_tags")
	}
}

func TestTagsAdapter_HappyPath(t *testing.T) {
	a := &changelog.TagsAdapter{
		SourceName: "github_tags",
		FetchTagsFn: func(_ context.Context) ([]changelog.TagData, error) {
			return []changelog.TagData{{Name: "v1.1.0"}, {Name: "v1.0.0"}}, nil
		},
		FetchCompareFn: func(_ context.Context, older, newer string) (changelog.CompareResult, error) {
			return changelog.CompareResult{
				Messages: []string{"fix something", "add feature"},
				HeadDate: "2024-02-01",
			}, nil
		},
		URLFmt:     "https://github.com/{ref}/compare/{older}...{newer}",
		ProjectRef: "owner/repo",
	}
	entries, err := a.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	e := entries[0]
	if e.Version != "1.1.0" {
		t.Errorf("Version = %q, want 1.1.0", e.Version)
	}
	if e.TagName != "v1.1.0" {
		t.Errorf("TagName = %q, want v1.1.0", e.TagName)
	}
	if e.URL != "https://github.com/owner/repo/compare/v1.0.0...v1.1.0" {
		t.Errorf("URL = %q", e.URL)
	}
	if e.PublishedAt != "2024-02-01" {
		t.Errorf("PublishedAt = %q, want 2024-02-01", e.PublishedAt)
	}
}

func TestTagsAdapter_ErrorPropagation(t *testing.T) {
	want := errors.New("tags error")
	a := &changelog.TagsAdapter{
		SourceName: "github_tags",
		FetchTagsFn: func(_ context.Context) ([]changelog.TagData, error) {
			return nil, want
		},
		FetchCompareFn: func(_ context.Context, older, newer string) (changelog.CompareResult, error) {
			return changelog.CompareResult{}, nil
		},
	}
	entries, err := a.Fetch(context.Background())
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if entries != nil {
		t.Fatalf("expected nil entries on error, got %v", entries)
	}
}

func TestTagsAdapter_FewerThanTwoTags(t *testing.T) {
	a := &changelog.TagsAdapter{
		SourceName: "github_tags",
		FetchTagsFn: func(_ context.Context) ([]changelog.TagData, error) {
			return []changelog.TagData{{Name: "v1.0.0"}}, nil
		},
		FetchCompareFn: func(_ context.Context, older, newer string) (changelog.CompareResult, error) {
			return changelog.CompareResult{}, nil
		},
	}
	entries, err := a.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries with < 2 tags, got %d", len(entries))
	}
}

func TestTagsAdapter_CompareErrorSkipsPair(t *testing.T) {
	compareErr := errors.New("compare failed")
	a := &changelog.TagsAdapter{
		SourceName: "github_tags",
		FetchTagsFn: func(_ context.Context) ([]changelog.TagData, error) {
			return []changelog.TagData{{Name: "v1.2.0"}, {Name: "v1.1.0"}, {Name: "v1.0.0"}}, nil
		},
		FetchCompareFn: func(_ context.Context, older, newer string) (changelog.CompareResult, error) {
			if older == "v1.1.0" && newer == "v1.2.0" {
				return changelog.CompareResult{}, compareErr
			}
			return changelog.CompareResult{Messages: []string{"commit"}, HeadDate: "2024-01-01"}, nil
		},
		URLFmt:     "https://github.com/{ref}/compare/{older}...{newer}",
		ProjectRef: "owner/repo",
	}
	entries, err := a.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only the second pair (v1.0.0 → v1.1.0) succeeded.
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].TagName != "v1.1.0" {
		t.Errorf("TagName = %q, want v1.1.0", entries[0].TagName)
	}
}
