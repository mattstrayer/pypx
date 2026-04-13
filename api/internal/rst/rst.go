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

// subDef holds a resolved substitution definition (e.g. .. |name| image:: url).
type subDef struct {
	src    string // image URL
	target string // optional link URL
	alt    string // optional alt text
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

// safeURL returns true when the URL scheme is not a script protocol.
func safeURL(u string) bool {
	lower := strings.ToLower(strings.TrimSpace(u))
	return !strings.HasPrefix(lower, "javascript:") && !strings.HasPrefix(lower, "vbscript:")
}

// applyInline processes inline RST markup and returns escaped HTML.
// subs is the map of substitution definitions (may be nil).
func applyInline(s string, subs map[string]subDef) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		// Substitution reference: |name|
		if s[i] == '|' && len(subs) > 0 {
			pipeEnd := strings.IndexByte(s[i+1:], '|')
			if pipeEnd >= 0 {
				name := s[i+1 : i+1+pipeEnd]
				if sub, ok := subs[name]; ok && safeURL(sub.src) {
					alt := html.EscapeString(sub.alt)
					img := fmt.Sprintf(`<img src="%s" alt="%s">`, html.EscapeString(sub.src), alt)
					if sub.target != "" && safeURL(sub.target) {
						fmt.Fprintf(&b, `<a href="%s">%s</a>`, html.EscapeString(sub.target), img)
					} else {
						b.WriteString(img)
					}
					i = i + 2 + pipeEnd
					continue
				}
			}
		}

		// Role: :role:`text`
		if s[i] == ':' && i+1 < len(s) {
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

		// Backtick: either `text <url>`_ hyperlink or `code` inline.
		if s[i] == '`' {
			end := strings.IndexByte(s[i+1:], '`')
			if end >= 0 {
				content := s[i+1 : i+1+end]
				afterTick := i + 2 + end
				// Hyperlink: `text <url>`_
				ltIdx := strings.LastIndex(content, " <")
				if ltIdx >= 0 && strings.HasSuffix(content, ">") &&
					afterTick < len(s) && s[afterTick] == '_' {
					text := strings.TrimSpace(content[:ltIdx])
					url := content[ltIdx+2 : len(content)-1]
					if safeURL(url) {
						fmt.Fprintf(&b, `<a href="%s">%s</a>`, html.EscapeString(url), html.EscapeString(text))
					} else {
						b.WriteString(html.EscapeString(text))
					}
					i = afterTick + 1
				} else {
					b.WriteString("<code>")
					b.WriteString(html.EscapeString(content))
					b.WriteString("</code>")
					i = afterTick
				}
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
			// Peek ahead past blank lines to find the next non-blank line.
			// If it's still indented, keep the blank line(s) in the body.
			j := i + 1
			for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
				j++
			}
			if j < len(lines) {
				nextIndent := len(lines[j]) - len(strings.TrimLeft(lines[j], " \t"))
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

// collectSubs does a pre-pass over lines to collect substitution definitions
// of the form:  .. |name| image:: url  (with optional :target: / :alt: body).
func collectSubs(lines []string) map[string]subDef {
	subs := make(map[string]subDef)
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, ".. |") {
			continue
		}
		rest := trimmed[3:] // "|name| directive:: args"
		// Find closing pipe.
		pipeEnd := strings.IndexByte(rest[1:], '|')
		if pipeEnd < 0 {
			continue
		}
		name := rest[1 : 1+pipeEnd]
		after := strings.TrimSpace(rest[2+pipeEnd:]) // "directive:: args"
		colonIdx := strings.Index(after, "::")
		if colonIdx < 0 {
			continue
		}
		directive := strings.TrimSpace(after[:colonIdx])
		if directive != "image" {
			continue // only image substitutions produce inline content
		}
		src := strings.TrimSpace(after[colonIdx+2:])
		sub := subDef{src: src}
		// Parse options from indented body (:target:, :alt:, etc.)
		body, _ := collectIndentedBody(lines, i+1)
		for _, opt := range body {
			opt = strings.TrimSpace(opt)
			if strings.HasPrefix(opt, ":target:") {
				sub.target = strings.TrimSpace(opt[8:])
			} else if strings.HasPrefix(opt, ":alt:") {
				sub.alt = strings.TrimSpace(opt[5:])
			}
		}
		subs[name] = sub
	}
	return subs
}

func renderDoc(src string) string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	lines := strings.Split(src, "\n")

	// Pre-pass: collect substitution definitions so inline references can resolve them.
	subs := collectSubs(lines)

	// Convenience wrapper so we don't repeat subs at every call site.
	inline := func(s string) string { return applyInline(s, subs) }

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

		// Directive: .. directive:: args  (includes substitution definitions)
		if strings.HasPrefix(trimmed, ".. ") {
			rest := trimmed[3:] // after ".. "
			colonIdx := strings.Index(rest, "::")
			if colonIdx < 0 {
				// Not a directive with :: (e.g. hyperlink targets like ".. _label:").
				// Skip the line and any indented body to avoid an infinite loop.
				_, next := collectIndentedBody(lines, i+1)
				if next == i+1 {
					i++
				} else {
					i = next
				}
				continue
			}
			directive := strings.TrimSpace(rest[:colonIdx])
			args := strings.TrimSpace(rest[colonIdx+2:])
			body, next := collectIndentedBody(lines, i+1)
			i = next

			// Substitution definition: .. |name| image:: url — already collected; skip.
			if len(directive) > 2 && directive[0] == '|' {
				continue
			}

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
					inline(strings.Join(body, " ")))

			case "warning", "caution", "danger", "attention":
				fmt.Fprintf(&out, "<div class=\"rst-warning\">%s</div>\n",
					inline(strings.Join(body, " ")))

			case "image":
				if safeURL(args) {
					alt := ""
					for _, opt := range body {
						opt = strings.TrimSpace(opt)
						if strings.HasPrefix(opt, ":alt:") {
							alt = strings.TrimSpace(opt[5:])
						}
					}
					fmt.Fprintf(&out, "<img src=\"%s\" alt=\"%s\">\n",
						html.EscapeString(args), html.EscapeString(alt))
				}

			case "toctree", "contents", "include", "literalinclude":
				// Omit — internal Sphinx directives.

			default:
				// Unknown directive — render body as paragraph if non-empty.
				if len(body) > 0 {
					fmt.Fprintf(&out, "<p>%s</p>\n", inline(strings.Join(body, " ")))
				}
			}
			continue
		}

		// Heading: current line followed by underline.
		if i+1 < len(lines) && isUnderline(lines[i+1], trimmed) {
			level := headingLevel(strings.TrimSpace(lines[i+1])[0])
			fmt.Fprintf(&out, "<h%d>%s</h%d>\n", level, inline(trimmed), level)
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
					fmt.Fprintf(&out, "<li>%s</li>\n", inline(t[2:]))
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
				fmt.Fprintf(&out, "<p>%s:</p>\n", inline(text))
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
			fmt.Fprintf(&out, "<p>%s</p>\n", inline(strings.Join(paraLines, " ")))
		}
	}

	return out.String()
}
