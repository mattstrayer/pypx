package textfmt

import (
	"fmt"
	"strings"
)

// CompareInput is the data needed by FormatCompare.
type CompareInput struct {
	Skipped  []SkippedPackage
	Packages []ComparePackageInput
}

// SkippedPackage records a package that could not be included in the compare.
type SkippedPackage struct {
	Name   string
	Reason string // "not found" | "fetch error"
}

// ComparePackageInput is the per-package data shown as a column.
type ComparePackageInput struct {
	Name             string
	Version          string
	Summary          string
	License          string
	PythonMin        string
	InstallSize      int64
	ModuleFormat     string
	LastReleasedDate string // YYYY-MM-DD
	ReleasesLast12Mo int
	DepCount         int
	Downloads30d     int64
	VulnCount        int
	Typed            string // "yes" | "stubs" | "no"
	RepoURL          string
	DocURL           string
}

// cellMax is the visible-character cap for any single cell. Cells longer than
// this are truncated to (cellMax-1) chars + "…".
const cellMax = 28

// FormatCompare renders the compare table. Leading "# skipped: ..." comment
// lines are emitted for each entry in Skipped. If Packages is empty, only the
// comments are emitted (no table header).
func FormatCompare(in *CompareInput) string {
	var b strings.Builder
	for _, s := range in.Skipped {
		fmt.Fprintf(&b, "# skipped: %s (%s)\n", s.Name, s.Reason)
	}
	if len(in.Packages) == 0 {
		return b.String()
	}

	// Build the rows.
	rows := []struct {
		label  string
		values []string
	}{
		{"version", make([]string, len(in.Packages))},
		{"summary", make([]string, len(in.Packages))},
		{"license", make([]string, len(in.Packages))},
		{"python_min", make([]string, len(in.Packages))},
		{"install_size", make([]string, len(in.Packages))},
		{"module_format", make([]string, len(in.Packages))},
		{"last_released_at", make([]string, len(in.Packages))},
		{"releases_last_12mo", make([]string, len(in.Packages))},
		{"dep_count", make([]string, len(in.Packages))},
		{"downloads_30d", make([]string, len(in.Packages))},
		{"vuln_count", make([]string, len(in.Packages))},
		{"typed", make([]string, len(in.Packages))},
		{"repo_url", make([]string, len(in.Packages))},
		{"doc_url", make([]string, len(in.Packages))},
	}

	for i, p := range in.Packages {
		rows[0].values[i] = p.Version
		rows[1].values[i] = p.Summary
		rows[2].values[i] = p.License
		rows[3].values[i] = p.PythonMin
		if p.InstallSize > 0 {
			rows[4].values[i] = HumanBytes(p.InstallSize)
		}
		rows[5].values[i] = p.ModuleFormat
		rows[6].values[i] = p.LastReleasedDate
		if p.ReleasesLast12Mo > 0 {
			rows[7].values[i] = fmt.Sprintf("%d", p.ReleasesLast12Mo)
		}
		if p.DepCount > 0 {
			rows[8].values[i] = fmt.Sprintf("%d", p.DepCount)
		}
		if p.Downloads30d > 0 {
			rows[9].values[i] = humanCount(p.Downloads30d)
		}
		rows[10].values[i] = fmt.Sprintf("%d", p.VulnCount)
		rows[11].values[i] = p.Typed
		rows[12].values[i] = p.RepoURL
		rows[13].values[i] = p.DocURL
	}

	// Header row + separator.
	// Each cell is " <text> " (1 space pad each side). Separator dashes mirror
	// that with cellMax+2 dashes per column.
	b.WriteString("| metric             |")
	for _, p := range in.Packages {
		fmt.Fprintf(&b, " %s |", truncCell(p.Name))
	}
	b.WriteByte('\n')

	b.WriteString("|--------------------|")
	dashes := strings.Repeat("-", cellMax+2)
	for range in.Packages {
		b.WriteString(dashes + "|")
	}
	b.WriteByte('\n')

	for _, row := range rows {
		fmt.Fprintf(&b, "| %-18s |", row.label)
		for _, v := range row.values {
			fmt.Fprintf(&b, " %s |", truncCell(v))
		}
		b.WriteByte('\n')
	}

	return b.String()
}

// truncCell truncates s to cellMax visible characters. Strings longer than
// cellMax become (cellMax-1) chars + "…". An empty string passes through.
func truncCell(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= cellMax {
		return s
	}
	return string(runes[:cellMax-1]) + "…"
}

// humanCount formats a download count with a short suffix.
// Examples: 45 → "45"; 1234 → "1.2K"; 1500000 → "1.5M"; 712_000_000 → "712M".
func humanCount(n int64) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%d", n)
	case n < 1_000_000:
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	case n < 1_000_000_000:
		return fmt.Sprintf("%dM", n/1_000_000)
	default:
		return fmt.Sprintf("%.1fB", float64(n)/1_000_000_000)
	}
}
