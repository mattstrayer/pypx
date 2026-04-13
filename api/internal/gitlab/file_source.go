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
	content, filename, err := s.Client.FetchRawFile(s.ProjectPath, candidateFilenames)
	if err != nil || content == "" {
		return nil, err
	}
	entries := changelog.Parse(content)
	for i := range entries {
		entries[i].URL = fmt.Sprintf("https://gitlab.com/%s/-/blob/HEAD/%s", s.ProjectPath, filename)
	}
	return entries, nil
}
