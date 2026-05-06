package textfmt

import (
	"fmt"
	"strings"
)

// ExtrasInput is the data FormatExtras needs. Mirrors handler.ExtrasResponse
// but keeps textfmt independent of the handler and pypi/conda/github packages.
type ExtrasInput struct {
	Package        string
	TypeStatus     string // "typed" | "stubs" | "untyped"
	StubPackage    string // empty if not applicable (only set when TypeStatus == "stubs")
	CondaAvailable bool
	CondaLatest    string // empty if not on conda-forge
	HasRepo        bool   // true iff repo info is present
	RepoStars      int
	RepoForks      int
	RepoOpenIssues int
	RepoLicense    string
	RepoHomepage   string
}

// FormatExtras renders extras data as agent-friendly plain text.
// The conda_forge section is omitted when CondaAvailable is false.
// The repo section is omitted when HasRepo is false.
func FormatExtras(in *ExtrasInput) string {
	var b strings.Builder

	WriteKV(&b, "package", in.Package)
	WriteKV(&b, "type_status", in.TypeStatus)
	if in.TypeStatus == "stubs" && in.StubPackage != "" {
		WriteKV(&b, "stub_package", in.StubPackage)
	}

	if in.CondaAvailable {
		b.WriteString("\n## conda_forge\n")
		b.WriteString("available: true\n")
		WriteKV(&b, "latest", in.CondaLatest)
	}

	if in.HasRepo {
		b.WriteString("\n## repo\n")
		fmt.Fprintf(&b, "stars: %d\n", in.RepoStars)
		fmt.Fprintf(&b, "forks: %d\n", in.RepoForks)
		fmt.Fprintf(&b, "open_issues: %d\n", in.RepoOpenIssues)
		WriteKV(&b, "license", in.RepoLicense)
		WriteKV(&b, "homepage", in.RepoHomepage)
	}

	return b.String()
}
