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
	tags, err := s.Client.FetchTags(ctx, s.ProjectPath)
	if err != nil || len(tags) < 2 {
		return nil, err
	}

	entries := make([]changelog.Entry, 0, len(tags)-1)
	for i := 0; i < len(tags)-1; i++ {
		newer := tags[i]
		older := tags[i+1]

		messages, headDate, err := s.Client.FetchCompare(ctx, s.ProjectPath, older.Name, newer.Name)
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
