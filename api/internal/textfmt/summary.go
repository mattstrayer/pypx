package textfmt

import (
	"fmt"
	"strings"

	"github.com/pypx/api/internal/osv"
)

// SummaryInput composes a tight agent briefing from package + security data.
// Security may be nil when the security check was skipped or failed.
type SummaryInput struct {
	Package  *PackageInput
	Security *SecurityInput
}

// FormatSummary renders a one-screen briefing (≤2KB target). Includes top-5
// dependencies and security count/max severity, omits maintainers, full
// dependency list, extras, and platform coverage detail.
func FormatSummary(in *SummaryInput) string {
	var b strings.Builder
	if in == nil || in.Package == nil {
		return ""
	}
	p := in.Package

	WriteKV(&b, "name", p.Name)
	WriteKV(&b, "version", p.Version)
	WriteKV(&b, "summary", p.Summary)
	WriteKV(&b, "license", p.License)
	WriteKV(&b, "requires_python", p.RequiresPython)
	if p.InstallSize > 0 {
		WriteKV(&b, "install_size", HumanBytes(p.InstallSize))
	}
	if c := p.ReleaseCadence; c.LastReleasedAt != "" {
		WriteKV(&b, "last_released_at", c.LastReleasedAt)
		fmt.Fprintf(&b, "releases_last_12mo: %d\n", c.ReleasesLast12Mo)
	}
	if repo := repoURL(p.ProjectURLs); repo != "" {
		WriteKV(&b, "repo_url", repo)
	}
	WriteKV(&b, "doc_url", p.DocURL)

	if len(p.Dependencies.Required) > 0 {
		b.WriteString("\n## top_dependencies\n")
		max := 5
		if len(p.Dependencies.Required) < max {
			max = len(p.Dependencies.Required)
		}
		for _, d := range p.Dependencies.Required[:max] {
			if d.Constraint == "" {
				fmt.Fprintf(&b, "%s\n", d.Name)
			} else {
				fmt.Fprintf(&b, "%s %s\n", d.Name, d.Constraint)
			}
		}
	}

	if in.Security != nil {
		b.WriteString("\n## security\n")
		fmt.Fprintf(&b, "vuln_count: %d\n", len(in.Security.Vulns))
		if len(in.Security.Vulns) > 0 {
			if sev := maxSeverity(in.Security.Vulns); sev != "UNKNOWN" {
				WriteKV(&b, "max_severity", sev)
			}
		}
	}

	return b.String()
}

// maxSeverity returns the highest severity label among the provided vulns.
// Ranking: CRITICAL > HIGH > MEDIUM > LOW > UNKNOWN. Empty/unrecognized
// labels are treated as UNKNOWN.
// If severity starts with "CVSS:", derives a label from impact metrics:
//   - If C:H or I:H or A:H → "HIGH"
//   - Else if C:L or I:L or A:L → "MEDIUM"
//   - Else → "LOW"
func maxSeverity(vulns []osv.VulnInfo) string {
	rank := map[string]int{"CRITICAL": 4, "HIGH": 3, "MEDIUM": 2, "LOW": 1}
	best := 0
	bestLabel := "UNKNOWN"
	for _, v := range vulns {
		var label string
		if strings.HasPrefix(v.Severity, "CVSS:") {
			// Derive label from CVSS vector impact metrics.
			// Check for high impact (C:H, I:H, or A:H as separate metrics).
			hasHighImpact := strings.Contains(v.Severity, "/C:H") || strings.Contains(v.Severity, "/I:H") || strings.Contains(v.Severity, "/A:H")
			if hasHighImpact {
				label = "HIGH"
			} else {
				// Check for low impact (C:L, I:L, or A:L as separate metrics).
				hasLowImpact := strings.Contains(v.Severity, "/C:L") || strings.Contains(v.Severity, "/I:L") || strings.Contains(v.Severity, "/A:L")
				if hasLowImpact {
					label = "MEDIUM"
				} else {
					label = "LOW"
				}
			}
		} else {
			label = strings.ToUpper(v.Severity)
		}
		r, ok := rank[label]
		if !ok {
			continue
		}
		if r > best {
			best = r
			bestLabel = label
		}
	}
	return bestLabel
}
