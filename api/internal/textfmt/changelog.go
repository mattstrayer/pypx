package textfmt

import (
	"fmt"
	"strings"

	"github.com/pypx/api/internal/changelog"
)

// ChangelogInput is the data FormatChangelog needs.
type ChangelogInput struct {
	Package string
	Source  string
	RepoURL string
	Entries []changelog.Entry
}

// FormatChangelog renders a package changelog as a single markdown document.
// Each entry becomes a "## <version> — <date>" heading followed by the body
// markdown. Output is grep-friendly: agents can extract a single version
// with `awk '/^## 0\.27\.0/,/^## /'`.
func FormatChangelog(in *ChangelogInput) string {
	var b strings.Builder
	WriteKV(&b, "package", in.Package)
	WriteKV(&b, "source", in.Source)
	WriteKV(&b, "repo_url", in.RepoURL)
	b.WriteByte('\n')
	for _, e := range in.Entries {
		switch {
		case e.Version != "" && e.PublishedAt != "":
			fmt.Fprintf(&b, "## %s — %s\n", e.Version, e.PublishedAt)
		case e.Version != "":
			fmt.Fprintf(&b, "## %s\n", e.Version)
		default:
			b.WriteString("## (unknown)\n")
		}
		body := strings.TrimSpace(e.Body)
		if body != "" {
			b.WriteString(body)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}
