package changelog

import "context"

// Entry is the canonical changelog entry type used across all sources.
// JSON tags match the existing frontend ChangelogEntry interface.
type Entry struct {
	Version     string `json:"version"`
	TagName     string `json:"tag_name"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	BodyHTML    string `json:"body_html"`
	PublishedAt string `json:"published_at"`
	URL         string `json:"url"`
}

// Source is implemented by each changelog data provider.
// Implementations are constructed with all parameters they need (owner, repo, etc.)
// so Fetch takes only a context.
type Source interface {
	// Name returns the source identifier used in the API response "source" field.
	Name() string
	// Fetch retrieves changelog entries. Returns empty slice (not error) when no
	// data is available from this source.
	Fetch(ctx context.Context) ([]Entry, error)
}

// Result holds the winning source name and its entries.
type Result struct {
	Source  string
	Entries []Entry
}
