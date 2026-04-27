// api/internal/github/file_source.go
package github

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

// FileSource fetches and parses a CHANGELOG/CHANGES/HISTORY file from the repo.
type FileSource struct {
	Client *Client
	Owner  string
	Repo   string
}

func (s *FileSource) Name() string { return "github_changelog_file" }

func (s *FileSource) Fetch(ctx context.Context) ([]changelog.Entry, error) {
	content, filename, err := s.Client.FetchRawFile(ctx, s.Owner, s.Repo, candidateFilenames)
	if err != nil || content == "" {
		return nil, err
	}
	entries := changelog.Parse(content)
	for i := range entries {
		// Construct a URL pointing to the file on GitHub so users can view it.
		entries[i].URL = fmt.Sprintf("https://github.com/%s/%s/blob/HEAD/%s", s.Owner, s.Repo, filename)
	}
	return entries, nil
}
