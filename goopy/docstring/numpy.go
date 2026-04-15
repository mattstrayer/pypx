package docstring

import (
	"strings"

	"github.com/pypx/goopy/model"
)

// parseNumPy parses a NumPy-style docstring.
//
// NumPy sections are delimited by a header line followed by a line of dashes.
// Parameters: "name : type" header, indented description lines.
// Returns: type on first line, indented description lines → single *DocReturn.
// Raises: exception type on first line, indented description lines → []*DocRaises.
func parseNumPy(raw string) *model.Docstring {
	doc := &model.Docstring{
		Text:  raw,
		Style: model.DocstringNumpy,
	}

	lines := strings.Split(raw, "\n")
	sections := splitNumpySections(lines)

	for name, body := range sections {
		normalized := strings.ToLower(strings.TrimSpace(name))
		switch normalized {
		case "parameters", "params":
			doc.Params = parseNumpyParams(body)
		case "returns", "return":
			doc.Returns = parseNumpyReturn(body)
		case "raises", "raise":
			doc.Raises = parseNumpyRaises(body)
		}
	}

	return doc
}

// splitNumpySections finds all NumPy-style section headers (header + dashes)
// and returns a map of section name → lines of body content.
func splitNumpySections(lines []string) map[string][]string {
	result := make(map[string][]string)

	i := 0
	for i < len(lines) {
		// A section header is a non-empty line whose next non-empty sibling is all dashes.
		if i+1 < len(lines) {
			header := strings.TrimSpace(lines[i])
			dashes := strings.TrimSpace(lines[i+1])
			if header != "" && len(dashes) >= 3 && allDashes(dashes) {
				sectionName := header
				i += 2 // skip header and dashes line

				// Collect body lines until the next section header.
				var body []string
				for i < len(lines) {
					// Peek ahead: is this the start of a new section?
					if i+1 < len(lines) {
						nextHeader := strings.TrimSpace(lines[i])
						nextDashes := strings.TrimSpace(lines[i+1])
						if nextHeader != "" && len(nextDashes) >= 3 && allDashes(nextDashes) {
							break
						}
					}
					body = append(body, lines[i])
					i++
				}
				result[sectionName] = body
				continue
			}
		}
		i++
	}

	return result
}

// parseNumpyParams parses the body of a Parameters section.
// Each entry starts with "name : type" (unindented or minimally indented),
// followed by indented description lines.
func parseNumpyParams(lines []string) []*model.DocParam {
	var params []*model.DocParam

	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// A parameter header is a non-empty line that is NOT indented more than
		// the base indent level. We consider a line a param header if it is
		// non-empty and the first character after the leading whitespace is not
		// a space/tab beyond what a header would have. The simplest heuristic:
		// param headers have no leading whitespace (after stripping the common
		// indent), or they contain " : " which signals "name : type".
		if trimmed == "" {
			i++
			continue
		}

		// Detect indent of this line.
		indent := leadingSpaces(line)

		// Look ahead to see if the next non-empty line is more indented
		// (meaning this line is a header, not a description continuation).
		isHeader := false
		for j := i + 1; j < len(lines); j++ {
			next := lines[j]
			nextTrimmed := strings.TrimSpace(next)
			if nextTrimmed == "" {
				continue
			}
			if leadingSpaces(next) > indent {
				isHeader = true
			}
			break
		}

		// Also treat it as a header if it's the last non-empty block.
		if !isHeader {
			// Check if there are no more non-empty lines after this one.
			hasMore := false
			for j := i + 1; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) != "" {
					hasMore = true
					break
				}
			}
			if !hasMore {
				isHeader = true
			}
		}

		if isHeader {
			param := parseNumpyParamHeader(trimmed)
			i++
			// Collect description lines (more indented).
			var descLines []string
			for i < len(lines) {
				next := lines[i]
				nextTrimmed := strings.TrimSpace(next)
				if nextTrimmed == "" {
					i++
					continue
				}
				if leadingSpaces(next) <= indent {
					break
				}
				descLines = append(descLines, nextTrimmed)
				i++
			}
			param.Description = strings.Join(descLines, " ")
			params = append(params, param)
		} else {
			i++
		}
	}

	return params
}

// parseNumpyParamHeader parses a "name : type" or "name" header line.
func parseNumpyParamHeader(s string) *model.DocParam {
	param := &model.DocParam{}
	parts := strings.SplitN(s, " : ", 2)
	param.Name = strings.TrimSpace(parts[0])
	if len(parts) == 2 {
		param.Type = strings.TrimSpace(parts[1])
	}
	return param
}

// parseNumpyReturn parses the body of a Returns section into a single *DocReturn.
// The first non-empty line is the type; subsequent indented lines are the description.
func parseNumpyReturn(lines []string) *model.DocReturn {
	ret := &model.DocReturn{}

	i := 0
	// Skip leading blank lines.
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) {
		return ret
	}

	// First non-empty line is the type (may have no indent, or same indent as description).
	typeIndent := leadingSpaces(lines[i])
	ret.Type = strings.TrimSpace(lines[i])
	i++

	// Remaining non-empty lines that are more indented form the description.
	var descLines []string
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			i++
			continue
		}
		if leadingSpaces(lines[i]) > typeIndent {
			descLines = append(descLines, trimmed)
		}
		i++
	}
	ret.Description = strings.Join(descLines, " ")

	return ret
}

// parseNumpyRaises parses the body of a Raises section into []*DocRaises.
// Structure mirrors Parameters: exception type on header line, indented description.
func parseNumpyRaises(lines []string) []*model.DocRaises {
	var raises []*model.DocRaises

	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			i++
			continue
		}

		indent := leadingSpaces(line)

		// Determine if this is a header by checking the next non-empty line's indent.
		isHeader := false
		for j := i + 1; j < len(lines); j++ {
			next := lines[j]
			if strings.TrimSpace(next) == "" {
				continue
			}
			if leadingSpaces(next) > indent {
				isHeader = true
			}
			break
		}

		if !isHeader {
			hasMore := false
			for j := i + 1; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) != "" {
					hasMore = true
					break
				}
			}
			if !hasMore {
				isHeader = true
			}
		}

		if isHeader {
			r := &model.DocRaises{Type: trimmed}
			i++
			var descLines []string
			for i < len(lines) {
				next := lines[i]
				nextTrimmed := strings.TrimSpace(next)
				if nextTrimmed == "" {
					i++
					continue
				}
				if leadingSpaces(next) <= indent {
					break
				}
				descLines = append(descLines, nextTrimmed)
				i++
			}
			r.Description = strings.Join(descLines, " ")
			raises = append(raises, r)
		} else {
			i++
		}
	}

	return raises
}

// leadingSpaces counts the number of leading space/tab characters in s.
func leadingSpaces(s string) int {
	count := 0
	for _, c := range s {
		if c == ' ' || c == '\t' {
			count++
		} else {
			break
		}
	}
	return count
}
