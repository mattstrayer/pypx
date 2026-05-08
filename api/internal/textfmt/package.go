package textfmt

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pypx/api/internal/enrichment"
)

// PackageInput is the data FormatPackage needs. Mirrors handler.PackageResponse
// but keeps textfmt independent of handler to avoid an import cycle.
type PackageInput struct {
	Name             string
	Version          string
	Summary          string
	License          string
	Author           string
	HomePage         string
	RequiresPython   string
	InstallSize      int64
	ModuleFormat     string
	DocURL           string
	ProjectURLs      map[string]string
	PythonVersions   enrichment.PythonVersionInfo
	Dependencies     enrichment.DependencyTree
	PlatformCoverage enrichment.PlatformCoverage
	ReleaseCadence   enrichment.ReleaseCadence
	Maintainers      []enrichment.Maintainer
}

// FormatPackage renders a PackageInput as agent-friendly plain text.
// Lines are stable key:value pairs; lists use one item per line under
// section headers prefixed with "##". Output is deterministic — extras
// keys are sorted alphabetically.
func FormatPackage(p *PackageInput) string {
	var b strings.Builder

	WriteKV(&b, "name", p.Name)
	WriteKV(&b, "version", p.Version)
	WriteKV(&b, "summary", p.Summary)
	WriteKV(&b, "license", p.License)
	WriteKV(&b, "author", p.Author)
	WriteKV(&b, "homepage", p.HomePage)
	WriteKV(&b, "requires_python", p.RequiresPython)
	WriteKV(&b, "python_min", p.PythonVersions.MinVersion)
	if p.InstallSize > 0 {
		WriteKV(&b, "install_size", HumanBytes(p.InstallSize))
	}
	WriteKV(&b, "module_format", p.ModuleFormat)
	WriteKV(&b, "doc_url", p.DocURL)

	if repo := repoURL(p.ProjectURLs); repo != "" {
		WriteKV(&b, "repo_url", repo)
	}

	if c := p.ReleaseCadence; c.LastReleasedAt != "" {
		WriteKV(&b, "last_released_at", c.LastReleasedAt)
		fmt.Fprintf(&b, "releases_last_12mo: %d\n", c.ReleasesLast12Mo)
		if c.AvgDaysBetweenReleases > 0 {
			fmt.Fprintf(&b, "avg_days_between_releases: %.1f\n", c.AvgDaysBetweenReleases)
		}
	}

	platforms := platformList(p.PlatformCoverage)
	if len(platforms) > 0 {
		WriteKV(&b, "platforms", strings.Join(platforms, ","))
	}

	if len(p.Dependencies.Required) > 0 {
		b.WriteString("\n## dependencies\n")
		for _, d := range p.Dependencies.Required {
			if d.Constraint == "" {
				fmt.Fprintf(&b, "%s\n", d.Name)
			} else {
				fmt.Fprintf(&b, "%s %s\n", d.Name, d.Constraint)
			}
		}
	}

	if len(p.Dependencies.Extras) > 0 {
		b.WriteString("\n## extras\n")
		keys := make([]string, 0, len(p.Dependencies.Extras))
		for k := range p.Dependencies.Extras {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, name := range keys {
			deps := p.Dependencies.Extras[name]
			fmt.Fprintf(&b, "[%s]\n", name)
			for _, d := range deps {
				if d.Constraint == "" {
					fmt.Fprintf(&b, "  %s\n", d.Name)
				} else {
					fmt.Fprintf(&b, "  %s %s\n", d.Name, d.Constraint)
				}
			}
		}
	}

	if len(p.Maintainers) > 0 {
		b.WriteString("\n## maintainers\n")
		for _, m := range p.Maintainers {
			switch {
			case m.Name != "" && m.Email != "":
				fmt.Fprintf(&b, "%s <%s>\n", m.Name, m.Email)
			case m.Name != "":
				fmt.Fprintf(&b, "%s\n", m.Name)
			case m.Email != "":
				fmt.Fprintf(&b, "%s\n", m.Email)
			}
		}
	}

	return b.String()
}

func repoURL(urls map[string]string) string {
	lower := make(map[string]string, len(urls))
	for k, v := range urls {
		lower[strings.ToLower(k)] = v
	}
	for _, key := range []string{"source", "source code", "repository", "homepage", "github"} {
		if v, ok := lower[key]; ok && v != "" {
			return v
		}
	}
	return ""
}

func platformList(cov enrichment.PlatformCoverage) []string {
	out := []string{}
	if cov.PurePython {
		out = append(out, "any")
	}
	if cov.LinuxX86 {
		out = append(out, "linux_x86_64")
	}
	if cov.LinuxARM64 {
		out = append(out, "linux_arm64")
	}
	if cov.Musl {
		out = append(out, "linux_musl")
	}
	if cov.MacOSX86 {
		out = append(out, "macos_x86_64")
	}
	if cov.MacOSARM64 {
		out = append(out, "macos_arm64")
	}
	if cov.Windows {
		out = append(out, "windows_x86_64")
	}
	return out
}
