// Package changelog contains generic source adapters that collapse per-forge
// implementations into a single mapping loop each. Adding a third forge
// (Codeberg, sourcehut, etc.) only requires three ~10-line constructor
// functions that wrap the forge's client calls as closures — no new source
// types needed.
package changelog

import (
	"context"
	"strings"
)

// ReleaseData is the forge-agnostic release shape accepted by ReleasesAdapter.
// Each forge constructor maps its native release type into this struct.
type ReleaseData struct {
	Version     string
	TagName     string
	Title       string
	Body        string
	PublishedAt string
	URL         string
}

// ReleasesAdapter adapts any forge's release fetcher to the Source interface.
type ReleasesAdapter struct {
	SourceName string
	FetchFn    func(ctx context.Context) ([]ReleaseData, error)
}

func (a *ReleasesAdapter) Name() string { return a.SourceName }

func (a *ReleasesAdapter) Fetch(ctx context.Context) ([]Entry, error) {
	releases, err := a.FetchFn(ctx)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(releases))
	for _, r := range releases {
		entries = append(entries, Entry{
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

// FileAdapter adapts any forge's raw-file fetcher to the Source interface.
// FetchFn returns (content, filename, error). When content is empty the
// adapter returns an empty slice (same behaviour as the original sources).
type FileAdapter struct {
	SourceName string
	// FetchFn returns the raw text content and the resolved filename.
	FetchFn func(ctx context.Context) (content, filename string, err error)
	// URLFmt is a format string used to build the per-entry URL.
	// It receives (projectRef, filename) via fmt.Sprintf, e.g.:
	//   "https://github.com/%s/%s/blob/HEAD/%s"  → projectRef = "owner/repo"
	//   "https://gitlab.com/%s/-/blob/HEAD/%s"   → projectRef = "group/repo"
	URLFmt string
	// ProjectRef is the forge-specific path substituted into URLFmt.
	ProjectRef string
}

func (a *FileAdapter) Name() string { return a.SourceName }

func (a *FileAdapter) Fetch(ctx context.Context) ([]Entry, error) {
	content, filename, err := a.FetchFn(ctx)
	if err != nil || content == "" {
		return nil, err
	}
	entries := Parse(content)
	fileURL := strings.NewReplacer("{ref}", a.ProjectRef, "{file}", filename).Replace(a.URLFmt)
	for i := range entries {
		entries[i].URL = fileURL
	}
	return entries, nil
}

// TagData is the forge-agnostic tag shape accepted by TagsAdapter.
type TagData struct {
	Name string
}

// CompareResult holds the data returned by a forge's compare (diff) call.
type CompareResult struct {
	Messages []string
	// HeadDate is the publication date of the newer tag, as a string to match
	// the Entry.PublishedAt type.
	HeadDate string
}

// TagsAdapter builds changelog entries from commits between consecutive git
// tags. Each forge constructor supplies FetchTagsFn and FetchCompareFn
// closures together with a URLFmt / ProjectRef for the compare URL.
type TagsAdapter struct {
	SourceName     string
	FetchTagsFn    func(ctx context.Context) ([]TagData, error)
	FetchCompareFn func(ctx context.Context, older, newer string) (CompareResult, error)
	// URLFmt template: use {ref}, {older}, {newer} placeholders.
	// GitHub: "https://github.com/{ref}/compare/{older}...{newer}"
	// GitLab: "https://gitlab.com/{ref}/-/compare/{older}...{newer}"
	URLFmt     string
	ProjectRef string
}

func (a *TagsAdapter) Name() string { return a.SourceName }

func (a *TagsAdapter) Fetch(ctx context.Context) ([]Entry, error) {
	tags, err := a.FetchTagsFn(ctx)
	if err != nil || len(tags) < 2 {
		return nil, err
	}

	// tags is ordered newest-first (both GitHub and GitLab return newest-first).
	// For each pair (tags[i], tags[i+1]) compare tags[i+1]...tags[i].
	entries := make([]Entry, 0, len(tags)-1)
	for i := 0; i < len(tags)-1; i++ {
		newer := tags[i]
		older := tags[i+1]

		cmp, err := a.FetchCompareFn(ctx, older.Name, newer.Name)
		if err != nil {
			// Skip this pair on error rather than failing the whole source.
			continue
		}

		var body string
		if len(cmp.Messages) > 0 {
			var sb strings.Builder
			for _, m := range cmp.Messages {
				sb.WriteString("- ")
				sb.WriteString(m)
				sb.WriteString("\n")
			}
			body = sb.String()
		}

		version := strings.TrimPrefix(newer.Name, "v")
		compareURL := strings.NewReplacer(
			"{ref}", a.ProjectRef,
			"{older}", older.Name,
			"{newer}", newer.Name,
		).Replace(a.URLFmt)

		entries = append(entries, Entry{
			Version:     version,
			TagName:     newer.Name,
			Title:       newer.Name,
			Body:        body,
			PublishedAt: cmp.HeadDate,
			URL:         compareURL,
		})
	}
	return entries, nil
}
