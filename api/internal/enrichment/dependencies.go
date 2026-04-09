package enrichment

import (
	"regexp"
	"strings"
)

// Dependency represents a single package dependency with an optional version constraint.
type Dependency struct {
	Name       string `json:"name"`
	Constraint string `json:"constraint"`
}

// DependencyTree holds required dependencies and extras-grouped dependencies.
type DependencyTree struct {
	Required []Dependency            `json:"required"`
	Extras   map[string][]Dependency `json:"extras"`
}

var extraMarkerRe = regexp.MustCompile(`extra\s*==\s*"([^"]+)"`)

// ParseDependencies parses a PEP 508 requires_dist list into a DependencyTree.
// Dependencies with an `extra == "name"` marker are grouped under Extras;
// all others are placed in Required. Both slices are always non-nil.
func ParseDependencies(requiresDist []string) DependencyTree {
	tree := DependencyTree{
		Required: []Dependency{},
		Extras:   map[string][]Dependency{},
	}

	for _, raw := range requiresDist {
		parts := strings.SplitN(raw, ";", 2)
		depPart := strings.TrimSpace(parts[0])
		dep := parseDep(depPart)

		if len(parts) == 2 {
			marker := parts[1]
			if m := extraMarkerRe.FindStringSubmatch(marker); m != nil {
				extraName := m[1]
				tree.Extras[extraName] = append(tree.Extras[extraName], dep)
				continue
			}
		}

		tree.Required = append(tree.Required, dep)
	}

	return tree
}

// parseDep extracts the package name and version constraint from a dep specifier
// like "charset-normalizer (<4,>=2)" or "click (>=7.0)" or "requests".
func parseDep(s string) Dependency {
	openIdx := strings.Index(s, "(")
	if openIdx == -1 {
		return Dependency{Name: strings.TrimSpace(s), Constraint: ""}
	}

	name := strings.TrimSpace(s[:openIdx])
	closeIdx := strings.LastIndex(s, ")")
	constraint := ""
	if closeIdx > openIdx {
		constraint = strings.TrimSpace(s[openIdx+1 : closeIdx])
	}

	return Dependency{Name: name, Constraint: constraint}
}
