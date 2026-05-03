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
	tags, err := s.Client.FetchTags(ctx, s.Owner, s.Repo)
	if err != nil || len(tags) < 2 {
		return nil, err
	}

	// tags is ordered newest-first from GitHub.
	// For each pair (tags[i], tags[i+1]), compare tags[i+1]...tags[i].
	entries := make([]changelog.Entry, 0, len(tags)-1)
	for i := 0; i < len(tags)-1; i++ {
		newer := tags[i]
		older := tags[i+1]

		messages, headDate, err := s.Client.FetchCompare(ctx, s.Owner, s.Repo, older.Name, newer.Name)
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
