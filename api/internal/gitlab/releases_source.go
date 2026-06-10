package gitlab

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

// NewReleasesSource returns a changelog.Source that fetches GitLab Releases.
func NewReleasesSource(c *Client, projectPath string) changelog.Source {
	return &changelog.ReleasesAdapter{
		SourceName: "gitlab_releases",
		FetchFn: func(ctx context.Context) ([]changelog.ReleaseData, error) {
			releases, err := c.FetchReleases(ctx, projectPath)
			if err != nil {
				return nil, err
			}
			data := make([]changelog.ReleaseData, 0, len(releases))
			for _, r := range releases {
				data = append(data, changelog.ReleaseData{
					Version:     r.Version,
					TagName:     r.TagName,
					Title:       r.Title,
					Body:        r.Body,
					PublishedAt: r.PublishedAt,
					URL:         r.URL,
				})
			}
			return data, nil
		},
	}
}

// NewFileSource returns a changelog.Source that fetches a CHANGELOG file from GitLab.
func NewFileSource(c *Client, projectPath string) changelog.Source {
	return &changelog.FileAdapter{
		SourceName: "gitlab_changelog_file",
		FetchFn: func(ctx context.Context) (string, string, error) {
			return c.FetchRawFile(ctx, projectPath, candidateFilenames)
		},
		URLFmt:     fmt.Sprintf("https://gitlab.com/%s/-/blob/HEAD/{file}", projectPath),
		ProjectRef: projectPath,
	}
}

// NewTagsSource returns a changelog.Source that builds entries from commits
// between consecutive git tags on GitLab.
func NewTagsSource(c *Client, projectPath string) changelog.Source {
	return &changelog.TagsAdapter{
		SourceName: "gitlab_tags",
		FetchTagsFn: func(ctx context.Context) ([]changelog.TagData, error) {
			tags, err := c.FetchTags(ctx, projectPath)
			if err != nil {
				return nil, err
			}
			out := make([]changelog.TagData, len(tags))
			for i, t := range tags {
				out[i] = changelog.TagData{Name: t.Name}
			}
			return out, nil
		},
		FetchCompareFn: func(ctx context.Context, older, newer string) (changelog.CompareResult, error) {
			messages, headDate, err := c.FetchCompare(ctx, projectPath, older, newer)
			if err != nil {
				return changelog.CompareResult{}, err
			}
			return changelog.CompareResult{Messages: messages, HeadDate: headDate}, nil
		},
		URLFmt:     fmt.Sprintf("https://gitlab.com/%s/-/compare/{older}...{newer}", projectPath),
		ProjectRef: projectPath,
	}
}
