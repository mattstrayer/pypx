package textfmt

import (
	"fmt"
	"strings"

	"github.com/pypx/api/internal/osv"
)

// SecurityInput is the data FormatSecurity needs.
type SecurityInput struct {
	Package   string
	CheckedAt string
	Vulns     []osv.VulnInfo
}

// FormatSecurity renders security advisories as one entry per block.
// Empty vuln list still produces a header so agents can confirm the check ran.
func FormatSecurity(in *SecurityInput) string {
	var b strings.Builder
	WriteKV(&b, "package", in.Package)
	WriteKV(&b, "checked_at", in.CheckedAt)
	fmt.Fprintf(&b, "vuln_count: %d\n", len(in.Vulns))
	for _, v := range in.Vulns {
		b.WriteByte('\n')
		WriteKV(&b, "id", v.ID)
		WriteKV(&b, "severity", v.Severity)
		WriteKV(&b, "affected", v.AffectedRange)
		WriteKV(&b, "fixed_in", v.FixedIn)
		WriteKV(&b, "summary", v.Summary)
		WriteKV(&b, "url", v.URL)
	}
	return b.String()
}
