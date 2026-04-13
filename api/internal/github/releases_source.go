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
