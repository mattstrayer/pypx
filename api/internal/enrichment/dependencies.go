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

// inlineConstraintRe matches the first version operator in an inline specifier
// like "botocore<1.43.0,>=1.42.88" (no parentheses).
var inlineConstraintRe = regexp.MustCompile(`[><=!~]`)

// parseDep extracts the package name and version constraint from a dep specifier
// like "charset-normalizer (<4,>=2)" or "click (>=7.0)" or "requests" or
// the inline PEP 508 form "botocore<1.43.0,>=1.42.88".
func parseDep(s string) Dependency {
	// Handle the PEP 508 direct-reference form "name @ url" first. A normalized
	// package name can never contain '@', and the env-marker ';' part was
	// already split off by the caller.
	if atIdx := strings.Index(s, "@"); atIdx > 0 {
		return Dependency{
			Name:       stripExtras(strings.TrimSpace(s[:atIdx])),
			Constraint: strings.TrimSpace(s[atIdx+1:]),
		}
	}

	openIdx := strings.Index(s, "(")
	if openIdx == -1 {
		// Try inline constraint form: split on the first version operator character.
		if loc := inlineConstraintRe.FindStringIndex(s); loc != nil {
			return Dependency{
				Name:       stripExtras(strings.TrimSpace(s[:loc[0]])),
				Constraint: strings.TrimSpace(s[loc[0]:]),
			}
		}
		return Dependency{Name: stripExtras(strings.TrimSpace(s)), Constraint: ""}
	}

	name := stripExtras(strings.TrimSpace(s[:openIdx]))
	closeIdx := strings.LastIndex(s, ")")
	constraint := ""
	if closeIdx > openIdx {
		constraint = strings.TrimSpace(s[openIdx+1 : closeIdx])
	}

	return Dependency{Name: name, Constraint: constraint}
}

// stripExtras removes a trailing [extras] group from a dependency name:
// "celery[redis]" -> "celery".
func stripExtras(name string) string {
	if i := strings.Index(name, "["); i >= 0 && strings.HasSuffix(name, "]") {
		return strings.TrimSpace(name[:i])
	}
	return name
}
