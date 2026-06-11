// Package docstring detects and parses Python docstrings in Google, NumPy,
// Sphinx/reST, and plain styles into structured documentation.
package docstring

import (
	"regexp"
	"strings"

	"github.com/pypx/goopy/model"
)

// Detect identifies the docstring style from raw text.
func Detect(raw string) model.DocstringStyle {
	if containsSphinxFields(raw) {
		return model.DocstringSphinx
	}
	if containsNumpySections(raw) {
		return model.DocstringNumpy
	}
	if containsGoogleSections(raw) {
		return model.DocstringGoogle
	}
	return model.DocstringPlain
}

// Parse detects the style and dispatches to the appropriate parser.
func Parse(raw string) *model.Docstring {
	style := Detect(raw)
	switch style {
	case model.DocstringGoogle:
		return parseGoogle(raw)
	case model.DocstringNumpy:
		return parseNumPy(raw)
	case model.DocstringSphinx:
		return parseSphinx(raw)
	default:
		return parsePlain(raw)
	}
}

func parsePlain(raw string) *model.Docstring {
	return &model.Docstring{
		Text:  strings.TrimSpace(raw),
		Style: model.DocstringPlain,
	}
}

func containsSphinxFields(raw string) bool {
	return strings.Contains(raw, ":param ") || strings.Contains(raw, ":type ") ||
		strings.Contains(raw, ":returns:") || strings.Contains(raw, ":rtype:")
}

func containsNumpySections(raw string) bool {
	lines := strings.Split(raw, "\n")
	for i := 0; i < len(lines)-1; i++ {
		trimmed := strings.TrimSpace(lines[i])
		nextTrimmed := strings.TrimSpace(lines[i+1])
		if trimmed != "" && len(nextTrimmed) >= 3 && allDashes(nextTrimmed) {
			return true
		}
	}
	return false
}

// googleSectionRe matches a Google-style section header at the start of a line
// (with optional leading whitespace), followed only by optional trailing whitespace.
// The (?m) flag makes ^ match the start of each line.
var googleSectionRe = regexp.MustCompile(
	`(?m)^\s*(Args|Arguments|Returns|Return|Raises|Yields|Yield|Examples|Example|Note|Notes|Attributes|Todo|References|Parameters|Params):\s*$`,
)

func containsGoogleSections(raw string) bool {
	return googleSectionRe.MatchString(raw)
}

func allDashes(s string) bool {
	for _, c := range s {
		if c != '-' {
			return false
		}
	}
	return true
}
