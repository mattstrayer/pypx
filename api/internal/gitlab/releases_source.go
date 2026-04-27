package gitlab

import (
	"context"
	"github.com/pypx/api/internal/changelog"
)

type ReleasesSource struct {
	Client      *Client
	ProjectPath string
}

func (s *ReleasesSource) Name() string { return "gitlab_releases" }

func (s *ReleasesSource) Fetch(ctx context.Context) ([]changelog.Entry, error) {
	releases, err := s.Client.FetchReleases(ctx, s.ProjectPath)
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
