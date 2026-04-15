// Package docstring detects and parses Python docstrings in Google, NumPy,
// Sphinx/reST, and plain styles into structured documentation.
package docstring

import (
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

// splitSummary extracts the first paragraph (summary) from a docstring.
// Returns summary and remaining text.
func splitSummary(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	parts := strings.SplitN(raw, "\n\n", 2)
	summary := strings.TrimSpace(parts[0])
	rest := ""
	if len(parts) > 1 {
		rest = strings.TrimSpace(parts[1])
	}
	return summary, rest
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

func containsGoogleSections(raw string) bool {
	for _, s := range googleSectionHeaders {
		if strings.Contains(raw, s) {
			return true
		}
	}
	return false
}

var googleSectionHeaders = []string{
	"Args:", "Arguments:", "Returns:", "Return:", "Raises:",
	"Yields:", "Yield:", "Examples:", "Example:", "Note:", "Notes:",
	"Attributes:", "Todo:", "References:", "Parameters:", "Params:",
}

func allDashes(s string) bool {
	for _, c := range s {
		if c != '-' {
			return false
		}
	}
	return true
}
