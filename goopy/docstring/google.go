package docstring

import (
	"strings"

	"github.com/pypx/goopy/model"
)

// googleSectionHeaders is the canonical list of recognized Google-style section names.
var googleSectionHeaders = []string{
	"Args:", "Arguments:", "Returns:", "Return:", "Raises:",
	"Yields:", "Yield:", "Examples:", "Example:", "Note:", "Notes:",
	"Attributes:", "Todo:", "References:", "Parameters:", "Params:",
}

// googleHeaderSet is a pre-built set of recognized Google-style section headers,
// initialized once at package load time to avoid repeated allocations.
var googleHeaderSet = func() map[string]bool {
	m := make(map[string]bool, len(googleSectionHeaders))
	for _, h := range googleSectionHeaders {
		m[h] = true
	}
	return m
}()

// parseGoogle parses a Google-style docstring.
func parseGoogle(raw string) *model.Docstring {
	doc := &model.Docstring{
		Text:  raw,
		Style: model.DocstringGoogle,
	}

	// Split into lines for section-based parsing.
	lines := strings.Split(raw, "\n")

	// Find section boundaries: map from section header line index to section name.
	type sectionSpan struct {
		name  string
		start int // line index after the header line
		end   int // exclusive
	}

	var sections []sectionSpan

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if googleHeaderSet[trimmed] {
			sections = append(sections, sectionSpan{
				name:  trimmed,
				start: i + 1,
			})
		}
	}

	// Fill in end boundaries.
	for i := range sections {
		if i+1 < len(sections) {
			// End just before the next section header line.
			sections[i].end = sections[i+1].start - 1
		} else {
			sections[i].end = len(lines)
		}
	}

	for _, sec := range sections {
		sectionLines := lines[sec.start:sec.end]
		switch sec.name {
		case "Args:", "Arguments:", "Parameters:", "Params:":
			doc.Params = parseGoogleParams(sectionLines)
		case "Returns:", "Return:":
			doc.Returns = parseGoogleReturn(sectionLines)
		case "Raises:":
			doc.Raises = parseGoogleRaises(sectionLines)
		}
	}

	return doc
}

// parseGoogleParams parses the lines of an Args/Parameters section.
// Each entry starts with an unindented-within-section item:
//
//	name: description
//	name (type): description
//
// Continuation lines (more deeply indented) are joined to the previous item.
func parseGoogleParams(lines []string) []*model.DocParam {
	var params []*model.DocParam

	// Determine the base indentation from the first non-empty line.
	baseIndent := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		baseIndent = countLeadingSpaces(l)
		break
	}
	if baseIndent < 0 {
		return nil
	}

	var current *model.DocParam
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := countLeadingSpaces(line)
		content := strings.TrimSpace(line)

		if indent == baseIndent {
			// New parameter entry.
			if current != nil {
				params = append(params, current)
			}
			current = parseGoogleParamLine(content)
		} else if indent > baseIndent && current != nil {
			// Continuation of previous param description.
			if current.Description == "" {
				current.Description = content
			} else {
				current.Description += " " + content
			}
		}
	}
	if current != nil {
		params = append(params, current)
	}
	return params
}

// parseGoogleParamLine parses a single "name: desc" or "name (type): desc" line.
func parseGoogleParamLine(s string) *model.DocParam {
	param := &model.DocParam{}

	// Check for optional type annotation: "name (type): desc"
	if parenStart := strings.Index(s, " ("); parenStart != -1 {
		parenEnd := strings.Index(s, "):")
		if parenEnd != -1 && parenEnd > parenStart {
			param.Name = strings.TrimSpace(s[:parenStart])
			param.Type = strings.TrimSpace(s[parenStart+2 : parenEnd])
			param.Description = strings.TrimSpace(s[parenEnd+2:])
			return param
		}
	}

	// Plain "name: desc"
	if colonIdx := strings.Index(s, ":"); colonIdx != -1 {
		param.Name = strings.TrimSpace(s[:colonIdx])
		param.Description = strings.TrimSpace(s[colonIdx+1:])
	} else {
		param.Name = s
	}
	return param
}

// parseGoogleReturn parses the Returns section into a single *model.DocReturn.
// Format: "type: description" or just a description with no colon.
func parseGoogleReturn(lines []string) *model.DocReturn {
	var parts []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			parts = append(parts, strings.TrimSpace(l))
		}
	}
	if len(parts) == 0 {
		return nil
	}

	// Join all non-empty lines as the full return text.
	full := strings.Join(parts, " ")

	// Try to split "type: description".
	if colonIdx := strings.Index(full, ":"); colonIdx != -1 {
		typePart := strings.TrimSpace(full[:colonIdx])
		descPart := strings.TrimSpace(full[colonIdx+1:])
		// Only treat as type:desc if typePart looks like a type (no spaces, or simple).
		if !strings.Contains(typePart, " ") {
			return &model.DocReturn{Type: typePart, Description: descPart}
		}
	}
	return &model.DocReturn{Description: full}
}

// parseGoogleRaises parses the Raises section.
// Format: "ExceptionType: description"
func parseGoogleRaises(lines []string) []*model.DocRaises {
	var raises []*model.DocRaises

	baseIndent := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		baseIndent = countLeadingSpaces(l)
		break
	}
	if baseIndent < 0 {
		return nil
	}

	var current *model.DocRaises
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := countLeadingSpaces(line)
		content := strings.TrimSpace(line)

		if indent == baseIndent {
			if current != nil {
				raises = append(raises, current)
			}
			current = &model.DocRaises{}
			if colonIdx := strings.Index(content, ":"); colonIdx != -1 {
				current.Type = strings.TrimSpace(content[:colonIdx])
				current.Description = strings.TrimSpace(content[colonIdx+1:])
			} else {
				current.Type = content
			}
		} else if indent > baseIndent && current != nil {
			if current.Description == "" {
				current.Description = content
			} else {
				current.Description += " " + content
			}
		}
	}
	if current != nil {
		raises = append(raises, current)
	}
	return raises
}

func countLeadingSpaces(s string) int {
	count := 0
	for _, c := range s {
		if c == ' ' {
			count++
		} else if c == '\t' {
			count += 4
		} else {
			break
		}
	}
	return count
}
