package rst

import (
	"fmt"
	"html"
	"strings"
)

// Render converts RST source to HTML.
// Returns an error only for nil-safe guard; callers may ignore the error and
// fall back to raw text if the output looks wrong.
func Render(src string) (string, error) {
	if src == "" {
		return "", nil
	}
	return renderDoc(src), nil
}

// headingLevel maps underline characters to HTML heading levels.
func headingLevel(ch byte) int {
	switch ch {
	case '=':
		return 1
	case '-':
		return 2
	case '~':
		return 3
	case '^':
		return 4
	default:
		return 5
	}
}

// isUnderline returns true when underline is all the same non-space character
// and at least as long as the text line above it.
func isUnderline(underline, above string) bool {
	u := strings.TrimRight(underline, "\n")
	if len(u) < len(above) || len(u) == 0 {
		return false
	}
	ch := u[0]
	if ch == ' ' || ch == '\t' {
		return false
	}
	for _, c := range []byte(u) {
		if c != ch {
			return false
		}
	}
	return true
}

// applyInline processes inline RST markup and returns escaped HTML.
func applyInline(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		// Role: :role:`text`
		if s[i] == ':' {
			end := strings.IndexByte(s[i+1:], ':')
			if end >= 0 {
				afterColon := i + 1 + end + 1
				if afterColon < len(s) && s[afterColon] == '`' {
					tickEnd := strings.IndexByte(s[afterColon+1:], '`')
					if tickEnd >= 0 {
						text := s[afterColon+1 : afterColon+1+tickEnd]
						b.WriteString("<code>")
						b.WriteString(html.EscapeString(text))
						b.WriteString("</code>")
						i = afterColon + 1 + tickEnd + 1
						continue
					}
				}
			}
		}
		// Bold: **text**
		if i+1 < len(s) && s[i] == '*' && s[i+1] == '*' {
			end := strings.Index(s[i+2:], "**")
			if end >= 0 {
				b.WriteString("<strong>")
				b.WriteString(html.EscapeString(s[i+2 : i+2+end]))
				b.WriteString("</strong>")
				i = i + 4 + end
				continue
			}
		}
		// Italic: *text* (not **)
		if s[i] == '*' && (i+1 >= len(s) || s[i+1] != '*') {
			end := strings.IndexByte(s[i+1:], '*')
			if end >= 0 && end > 0 {
				b.WriteString("<em>")
				b.WriteString(html.EscapeString(s[i+1 : i+1+end]))
				b.WriteString("</em>")
				i = i + 2 + end
				continue
			}
		}
		// Inline code: `text`
		if s[i] == '`' {
			end := strings.IndexByte(s[i+1:], '`')
			if end >= 0 {
				b.WriteString("<code>")
				b.WriteString(html.EscapeString(s[i+1 : i+1+end]))
				b.WriteString("</code>")
				i = i + 2 + end
				continue
			}
		}
		// HTML-escape raw characters.
		switch s[i] {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&#34;")
		default:
			b.WriteByte(s[i])
		}
		i++
	}
	return b.String()
}

// collectIndentedBody reads lines that are indented (or blank within the block)
// starting at lines[start]. Returns the dedented lines and the next line index.
func collectIndentedBody(lines []string, start int) ([]string, int) {
	var body []string
	i := start
	// Skip leading blank lines.
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) {
		return nil, i
	}
	// Determine indentation from first non-blank line.
	indent := len(lines[i]) - len(strings.TrimLeft(lines[i], " \t"))
	if indent == 0 {
		return nil, i
	}
	for i < len(lines) {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			// Allow blank lines within body only if followed by indented line.
			if i+1 < len(lines) {
				nextIndent := len(lines[i+1]) - len(strings.TrimLeft(lines[i+1], " \t"))
				if nextIndent >= indent {
					body = append(body, "")
					i++
					continue
				}
			}
			break
		}
		lineIndent := len(line) - len(strings.TrimLeft(line, " \t"))
		if lineIndent < indent {
			break
		}
		dedented := line
		if len(line) >= indent {
			dedented = line[indent:]
		}
		body = append(body, dedented)
		i++
	}
	return body, i
}

func renderDoc(src string) string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	lines := strings.Split(src, "\n")
	var out strings.Builder
	i := 0

	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Blank line.
		if trimmed == "" {
			i++
			continue
		}

		// Directive: .. directive:: args
		if strings.HasPrefix(trimmed, ".. ") {
			rest := trimmed[3:] // after ".. "
			colonIdx := strings.Index(rest, "::")
			if colonIdx >= 0 {
				directive := strings.TrimSpace(rest[:colonIdx])
				args := strings.TrimSpace(rest[colonIdx+2:])
				body, next := collectIndentedBody(lines, i+1)
				i = next

				switch directive {
				case "code-block", "code", "sourcecode":
					lang := args
					code := strings.Join(body, "\n")
					if lang != "" {
						fmt.Fprintf(&out, "<pre><code class=\"language-%s\">%s</code></pre>\n",
							html.EscapeString(lang), html.EscapeString(code))
					} else {
						fmt.Fprintf(&out, "<pre><code>%s</code></pre>\n", html.EscapeString(code))
					}

				case "note":
					fmt.Fprintf(&out, "<div class=\"rst-note\">%s</div>\n",
						applyInline(strings.Join(body, " ")))

				case "warning", "caution", "danger", "attention":
					fmt.Fprintf(&out, "<div class=\"rst-warning\">%s</div>\n",
						applyInline(strings.Join(body, " ")))

				case "image":
					fmt.Fprintf(&out, "<img src=\"%s\" alt=\"\">\n", html.EscapeString(args))

				case "toctree", "contents", "include", "literalinclude":
					// Omit — internal Sphinx directives.

				default:
					// Unknown directive — render body as paragraph if non-empty.
					if len(body) > 0 {
						fmt.Fprintf(&out, "<p>%s</p>\n", applyInline(strings.Join(body, " ")))
					}
				}
				continue
			}
		}

		// Heading: current line followed by underline.
		if i+1 < len(lines) && isUnderline(lines[i+1], trimmed) {
			level := headingLevel(strings.TrimSpace(lines[i+1])[0])
			fmt.Fprintf(&out, "<h%d>%s</h%d>\n", level, applyInline(trimmed), level)
			i += 2
			continue
		}

		// Bullet list.
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			out.WriteString("<ul>\n")
			for i < len(lines) {
				t := strings.TrimSpace(lines[i])
				if t == "" {
					i++
					break
				}
				if strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") {
					fmt.Fprintf(&out, "<li>%s</li>\n", applyInline(t[2:]))
					i++
				} else {
					break
				}
			}
			out.WriteString("</ul>\n")
			continue
		}

		// Literal block: paragraph ending with ::.
		if strings.HasSuffix(trimmed, "::") {
			text := strings.TrimSuffix(trimmed, "::")
			text = strings.TrimRight(text, " ")
			if text != "" {
				fmt.Fprintf(&out, "<p>%s:</p>\n", applyInline(text))
			}
			body, next := collectIndentedBody(lines, i+1)
			i = next
			if len(body) > 0 {
				fmt.Fprintf(&out, "<pre><code>%s</code></pre>\n",
					html.EscapeString(strings.Join(body, "\n")))
			}
			continue
		}

		// Regular paragraph: collect until blank line or heading underline.
		var paraLines []string
		for i < len(lines) {
			t := strings.TrimSpace(lines[i])
			if t == "" {
				i++
				break
			}
			if strings.HasPrefix(t, ".. ") {
				break
			}
			if i+1 < len(lines) && isUnderline(lines[i+1], t) {
				break
			}
			if strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") {
				break
			}
			paraLines = append(paraLines, t)
			i++
		}
		if len(paraLines) > 0 {
			fmt.Fprintf(&out, "<p>%s</p>\n", applyInline(strings.Join(paraLines, " ")))
		}
	}

	return out.String()
}
