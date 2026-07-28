package textfmt

import (
	"fmt"
	"strings"
)

// StatPoint is a single labeled data point (a week bucket, a Python version,
// or an OS) with its download count.
type StatPoint struct {
	Label     string
	Downloads int64
}

// StatsInput is the data needed by FormatStats. The handler maps
// handler.CombinedStats into this shape; textfmt does not import handler.
type StatsInput struct {
	Package  string
	Period   string
	DateFrom string
	DateTo   string

	Overall        []StatPoint
	PythonVersions []StatPoint
	Systems        []StatPoint
}

// FormatStats renders download stats as plain text. The date_range line is
// omitted when DateFrom is empty. Each section (weekly downloads, python
// versions, systems) is omitted when its slice is empty. If all three
// sections are empty, a "# no download data" line is emitted instead.
func FormatStats(s StatsInput) string {
	var b strings.Builder

	fmt.Fprintf(&b, "package: %s\n", s.Package)
	fmt.Fprintf(&b, "period: %s\n", s.Period)
	if s.DateFrom != "" {
		fmt.Fprintf(&b, "date_range: %s → %s\n", s.DateFrom, s.DateTo)
	}
	b.WriteByte('\n')

	if len(s.Overall) == 0 && len(s.PythonVersions) == 0 && len(s.Systems) == 0 {
		b.WriteString("# no download data\n")
		return b.String()
	}

	writeStatSection(&b, "## weekly downloads", s.Overall)
	writeStatSection(&b, "## python versions", s.PythonVersions)
	writeStatSection(&b, "## systems", s.Systems)

	return strings.TrimRight(b.String(), "\n") + "\n"
}

// writeStatSection writes a section header followed by one "label\tdownloads"
// line per point, plus a trailing blank line. No-op when points is empty.
func writeStatSection(b *strings.Builder, header string, points []StatPoint) {
	if len(points) == 0 {
		return
	}
	b.WriteString(header)
	b.WriteByte('\n')
	for _, p := range points {
		fmt.Fprintf(b, "%s\t%d\n", p.Label, p.Downloads)
	}
	b.WriteByte('\n')
}
