package textfmt

import (
	"fmt"
	"strings"

	"github.com/pypx/api/internal/search"
)

// FormatSearch renders search results as TSV: name<tab>downloads<tab>summary.
// One result per line, header line on top so agents can skip or detect.
// Tabs and newlines in summary are replaced with spaces to keep TSV intact.
func FormatSearch(results []search.PackageEntry) string {
	var b strings.Builder
	b.WriteString("# name\tdownloads\tsummary\n")
	for _, r := range results {
		summary := strings.ReplaceAll(r.Summary, "\t", " ")
		summary = strings.ReplaceAll(summary, "\n", " ")
		fmt.Fprintf(&b, "%s\t%d\t%s\n", r.Name, r.Downloads, summary)
	}
	return b.String()
}
