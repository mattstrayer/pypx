package textfmt

import (
	"fmt"
	"strings"

	"github.com/pypx/api/internal/changelog"
)

// DiffInput is the data needed by FormatDiff. Each major section is either
// populated OR carries an Unavailable string. When Unavailable != "", the
// populated fields for that section are ignored.
type DiffInput struct {
	Package string
	From    string
	To      string

	Changelog            []changelog.Entry
	ChangelogUnavailable string

	DepChanges            DepDiff
	DepChangesUnavailable string

	APIChanges            APIDiff
	APIChangesUnavailable string
}

// DepDiff is the dependency-level diff between two package versions.
type DepDiff struct {
	Added   []string
	Removed []string
	Bumped  []DepBump
}

// DepBump records a dependency whose version constraint changed.
type DepBump struct {
	Name           string
	FromConstraint string
	ToConstraint   string
}

// APIDiff is the symbol-level diff between two package versions.
type APIDiff struct {
	Added   []string
	Removed []string
	Changed []APIChange
	// Truncated counts: how many additional items were elided beyond the cap.
	AddedTruncated   int
	RemovedTruncated int
	ChangedTruncated int
}

// APIChange records a symbol whose signature differs between versions.
type APIChange struct {
	Path    string
	FromSig string
	ToSig   string
}

// FormatDiff renders the diff as markdown. Sections appear in fixed order:
// header, changelog, dependency changes, api changes.
func FormatDiff(in *DiffInput) string {
	var b strings.Builder
	WriteKV(&b, "package", in.Package)
	WriteKV(&b, "from", in.From)
	WriteKV(&b, "to", in.To)

	// ## changelog
	b.WriteString("\n## changelog\n")
	if in.ChangelogUnavailable != "" {
		fmt.Fprintf(&b, "# unavailable: %s\n", in.ChangelogUnavailable)
	} else if len(in.Changelog) == 0 {
		b.WriteString("# no entries in range\n")
	} else {
		for _, e := range in.Changelog {
			switch {
			case e.Version != "" && e.PublishedAt != "":
				fmt.Fprintf(&b, "\n## %s — %s\n", e.Version, e.PublishedAt)
			case e.Version != "":
				fmt.Fprintf(&b, "\n## %s\n", e.Version)
			default:
				b.WriteString("\n## (unknown)\n")
			}
			body := strings.TrimSpace(e.Body)
			if body != "" {
				b.WriteString(body)
				b.WriteByte('\n')
			}
		}
	}

	// ## dependency changes
	b.WriteString("\n## dependency changes\n")
	if in.DepChangesUnavailable != "" {
		fmt.Fprintf(&b, "# unavailable: %s\n", in.DepChangesUnavailable)
	} else {
		dd := in.DepChanges
		if len(dd.Added) == 0 && len(dd.Removed) == 0 && len(dd.Bumped) == 0 {
			b.WriteString("# no changes\n")
		} else {
			for _, n := range dd.Added {
				fmt.Fprintf(&b, "+ added: %s\n", n)
			}
			for _, n := range dd.Removed {
				fmt.Fprintf(&b, "- removed: %s\n", n)
			}
			for _, bp := range dd.Bumped {
				fmt.Fprintf(&b, "~ bumped: %s (%s → %s)\n", bp.Name, bp.FromConstraint, bp.ToConstraint)
			}
		}
	}

	// ## api changes
	b.WriteString("\n## api changes\n")
	if in.APIChangesUnavailable != "" {
		fmt.Fprintf(&b, "# unavailable: %s\n", in.APIChangesUnavailable)
	} else {
		ad := in.APIChanges
		if len(ad.Added) == 0 && len(ad.Removed) == 0 && len(ad.Changed) == 0 &&
			ad.AddedTruncated == 0 && ad.RemovedTruncated == 0 && ad.ChangedTruncated == 0 {
			b.WriteString("# no changes\n")
		} else {
			for _, p := range ad.Added {
				fmt.Fprintf(&b, "+ added: %s\n", p)
			}
			if ad.AddedTruncated > 0 {
				fmt.Fprintf(&b, "# truncated: %d more added\n", ad.AddedTruncated)
			}
			for _, p := range ad.Removed {
				fmt.Fprintf(&b, "- removed: %s\n", p)
			}
			if ad.RemovedTruncated > 0 {
				fmt.Fprintf(&b, "# truncated: %d more removed\n", ad.RemovedTruncated)
			}
			for _, c := range ad.Changed {
				fmt.Fprintf(&b, "~ changed: %s  (was: %s; now: %s)\n", c.Path, c.FromSig, c.ToSig)
			}
			if ad.ChangedTruncated > 0 {
				fmt.Fprintf(&b, "# truncated: %d more changed\n", ad.ChangedTruncated)
			}
		}
	}

	return b.String()
}
