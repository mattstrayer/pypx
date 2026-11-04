package docstring

import "strings"

// cleandoc mirrors Python's inspect.cleandoc: it expands tabs, strips leading
// whitespace from the first line, removes the common indent from the remaining
// lines, and drops leading and trailing blank lines.
func cleandoc(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	lines := strings.Split(raw, "\n")
	for i, l := range lines {
		lines[i] = expandTabs(l, 8)
	}

	margin := -1
	for _, l := range lines[1:] {
		content := strings.TrimLeft(l, " ")
		if content == "" {
			continue
		}
		if indent := len(l) - len(content); margin < 0 || indent < margin {
			margin = indent
		}
	}

	lines[0] = strings.TrimLeft(lines[0], " ")
	if margin > 0 {
		for i := 1; i < len(lines); i++ {
			if len(lines[i]) >= margin {
				lines[i] = lines[i][margin:]
			} else {
				lines[i] = ""
			}
		}
	}

	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	return strings.Join(lines, "\n")
}

// expandTabs replaces tabs with spaces up to the next multiple of tabSize,
// like Python's str.expandtabs.
func expandTabs(s string, tabSize int) string {
	if !strings.Contains(s, "\t") {
		return s
	}
	var b strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			n := tabSize - col%tabSize
			b.WriteString(strings.Repeat(" ", n))
			col += n
			continue
		}
		b.WriteRune(r)
		col++
	}
	return b.String()
}
