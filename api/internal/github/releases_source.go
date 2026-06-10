package github

import (
	"context"
	"fmt"

	"github.com/pypx/api/internal/changelog"
)

// NewReleasesSource returns a changelog.Source that fetches GitHub Releases.
func NewReleasesSource(c *Client, owner, repo string) changelog.Source {
	return &changelog.ReleasesAdapter{
		SourceName: "github_releases",
		FetchFn: func(ctx context.Context) ([]changelog.ReleaseData, error) {
			releases, err := c.FetchReleases(ctx, owner, repo)
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

// candidateFilenames is the ordered list of changelog filenames to try.
var candidateFilenames = []string{
	"CHANGELOG.md", "CHANGELOG.rst", "CHANGELOG.txt", "CHANGELOG",
	"CHANGES.md", "CHANGES.rst", "CHANGES.txt", "CHANGES",
	"HISTORY.md", "HISTORY.rst", "HISTORY.txt", "HISTORY",
}

// NewFileSource returns a changelog.Source that fetches a CHANGELOG file from GitHub.
func NewFileSource(c *Client, owner, repo string) changelog.Source {
	return &changelog.FileAdapter{
		SourceName: "github_changelog_file",
		FetchFn: func(ctx context.Context) (string, string, error) {
			return c.FetchRawFile(ctx, owner, repo, candidateFilenames)
		},
		URLFmt:     fmt.Sprintf("https://github.com/%s/%s/blob/HEAD/{file}", owner, repo),
		ProjectRef: owner + "/" + repo,
	}
}

// NewTagsSource returns a changelog.Source that builds entries from commits
// between consecutive git tags on GitHub.
func NewTagsSource(c *Client, owner, repo string) changelog.Source {
	return &changelog.TagsAdapter{
		SourceName: "github_tags",
		FetchTagsFn: func(ctx context.Context) ([]changelog.TagData, error) {
			tags, err := c.FetchTags(ctx, owner, repo)
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
			messages, headDate, err := c.FetchCompare(ctx, owner, repo, older, newer)
			if err != nil {
				return changelog.CompareResult{}, err
			}
			return changelog.CompareResult{Messages: messages, HeadDate: headDate}, nil
		},
		URLFmt:     fmt.Sprintf("https://github.com/%s/%s/compare/{older}...{newer}", owner, repo),
		ProjectRef: owner + "/" + repo,
	}
}
