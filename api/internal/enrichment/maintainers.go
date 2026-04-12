package enrichment

import (
	"regexp"
	"strings"

	"github.com/pypx/api/internal/pypi"
)

// Maintainer represents a single package author or maintainer.
type Maintainer struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

var angleEmailRe = regexp.MustCompile(`^(.*?)\s*<([^>]+)>\s*$`)

// ParseMaintainers extracts a structured list of maintainers from PyPI
// package metadata. It prefers the author_email / maintainer_email fields
// (RFC 2822 "Name <addr>" format), falling back to the plain name fields.
func ParseMaintainers(info pypi.PackageInfo) []Maintainer {
	// Try author_email first (most packages put names+emails here).
	if info.AuthorEmail != "" {
		m := parseMaintainersFromEmailField(info.AuthorEmail, info.Author)
		if len(m) > 0 {
			return m
		}
	}

	// Fall back to plain author name.
	if info.Author != "" {
		var result []Maintainer
		for _, name := range splitNames(info.Author) {
			result = append(result, Maintainer{Name: name})
		}
		return result
	}

	// Try maintainer fields.
	if info.MaintainerEmail != "" {
		m := parseMaintainersFromEmailField(info.MaintainerEmail, info.Maintainer)
		if len(m) > 0 {
			return m
		}
	}

	if info.Maintainer != "" {
		var result []Maintainer
		for _, name := range splitNames(info.Maintainer) {
			result = append(result, Maintainer{Name: name})
		}
		return result
	}

	return nil
}

// parseMaintainersFromEmailField parses a comma-separated list of
// "Name <email>" entries. nameField is used as a fallback when the email
// entry contains no name portion.
func parseMaintainersFromEmailField(emailField, nameField string) []Maintainer {
	entries := splitEmailList(emailField)
	names := splitNames(nameField)

	var result []Maintainer
	for i, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		m := parseOneEntry(entry)
		// If the email field didn't include a name, use the positional name.
		if m.Name == "" && i < len(names) {
			m.Name = names[i]
		}
		if m.Name != "" || m.Email != "" {
			result = append(result, m)
		}
	}
	return result
}

// splitEmailList splits on commas that are not inside angle brackets.
func splitEmailList(s string) []string {
	var parts []string
	depth, start := 0, 0
	for i, ch := range s {
		switch ch {
		case '<':
			depth++
		case '>':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, s[start:])
}

// splitNames splits a plain name field on commas or semicolons.
func splitNames(s string) []string {
	if s == "" {
		return nil
	}
	var names []string
	for _, n := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' }) {
		n = strings.TrimSpace(n)
		if n != "" {
			names = append(names, n)
		}
	}
	return names
}

func parseOneEntry(s string) Maintainer {
	if m := angleEmailRe.FindStringSubmatch(s); m != nil {
		return Maintainer{
			Name:  strings.TrimSpace(m[1]),
			Email: strings.TrimSpace(m[2]),
		}
	}
	if strings.Contains(s, "@") {
		return Maintainer{Email: s}
	}
	return Maintainer{Name: s}
}

// docURLKeys are checked in priority order against project_urls keys
// (case-insensitive) to find the documentation URL.
var docURLKeys = []string{"documentation", "docs", "doc"}

// ExtractDocURL returns the documentation URL from a project_urls map,
// or an empty string if no documentation link is found.
func ExtractDocURL(projectURLs map[string]string) string {
	if len(projectURLs) == 0 {
		return ""
	}
	lower := make(map[string]string, len(projectURLs))
	for k, v := range projectURLs {
		lower[strings.ToLower(k)] = v
	}
	for _, key := range docURLKeys {
		if url, ok := lower[key]; ok {
			return url
		}
	}
	return ""
}
