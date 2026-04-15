package docstring

import (
	"strings"

	"github.com/pypx/goopy/model"
)

// parseSphinx parses a Sphinx/reST-style docstring.
//
// Supported field syntax:
//
//	:param name: description
//	:param type name: description   (inline type)
//	:type name: type
//	:returns: description
//	:return: description
//	:rtype: type
//	:raises ExcType: description
//
// Continuation lines (indented, not starting with `:`) are appended to the
// previous field's description.
func parseSphinx(raw string) *model.Docstring {
	doc := &model.Docstring{
		Text:  raw,
		Style: model.DocstringSphinx,
	}

	// paramsByName holds params keyed by name for :type merging.
	paramsByName := map[string]*model.DocParam{}

	lines := strings.Split(raw, "\n")

	// fieldKind tracks what the last directive was for continuation lines.
	type fieldKind int
	const (
		kindNone    fieldKind = iota
		kindParam             // last field was a :param:
		kindReturns           // last field was :returns:/:return:
		kindRaises            // last field was :raises:
	)

	lastKind := kindNone
	var lastParam *model.DocParam
	var lastRaises *model.DocRaises

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Continuation line: not empty, not a new directive, and previous field exists.
		if trimmed != "" && !strings.HasPrefix(trimmed, ":") && lastKind != kindNone {
			switch lastKind {
			case kindParam:
				if lastParam != nil {
					if lastParam.Description == "" {
						lastParam.Description = trimmed
					} else {
						lastParam.Description += " " + trimmed
					}
				}
			case kindReturns:
				if doc.Returns == nil {
					doc.Returns = &model.DocReturn{}
				}
				if doc.Returns.Description == "" {
					doc.Returns.Description = trimmed
				} else {
					doc.Returns.Description += " " + trimmed
				}
			case kindRaises:
				if lastRaises != nil {
					if lastRaises.Description == "" {
						lastRaises.Description = trimmed
					} else {
						lastRaises.Description += " " + trimmed
					}
				}
			}
			continue
		}

		// Reset continuation tracking on blank lines or non-directive text before fields.
		if trimmed == "" {
			lastKind = kindNone
			lastParam = nil
			lastRaises = nil
			continue
		}

		// Parse directive lines: must start and contain a closing colon.
		if !strings.HasPrefix(trimmed, ":") {
			// Plain text line — reset continuation only if we haven't started fields.
			lastKind = kindNone
			lastParam = nil
			lastRaises = nil
			continue
		}

		// Find the closing `:` of the directive name.
		rest := trimmed[1:] // strip leading `:`
		colonIdx := strings.Index(rest, ":")
		if colonIdx < 0 {
			// Malformed — skip.
			lastKind = kindNone
			continue
		}

		directive := rest[:colonIdx]        // e.g. "param x", "type x", "returns", "rtype", "raises ValueError"
		description := strings.TrimSpace(rest[colonIdx+1:]) // text after closing `:`

		// Dispatch by directive prefix.
		switch {
		case strings.HasPrefix(directive, "param "):
			// ":param name: desc"  or  ":param type name: desc"
			paramPart := strings.TrimPrefix(directive, "param ")
			name, typ := parseSphinxParamDirective(paramPart)
			p := getOrCreateParam(paramsByName, name)
			if typ != "" && p.Type == "" {
				p.Type = typ
			}
			if description != "" {
				if p.Description == "" {
					p.Description = description
				} else {
					p.Description += " " + description
				}
			}
			// Append to doc.Params only on first encounter.
			if _, exists := paramsByName[name]; !exists {
				doc.Params = append(doc.Params, p)
			}
			paramsByName[name] = p
			lastKind = kindParam
			lastParam = p

		case strings.HasPrefix(directive, "type "):
			// ":type name: typename"
			name := strings.TrimSpace(strings.TrimPrefix(directive, "type "))
			p := getOrCreateParam(paramsByName, name)
			p.Type = description
			if _, exists := paramsByName[name]; !exists {
				doc.Params = append(doc.Params, p)
				paramsByName[name] = p
			}
			lastKind = kindNone // :type lines don't have multi-line continuations

		case directive == "returns" || directive == "return":
			if doc.Returns == nil {
				doc.Returns = &model.DocReturn{}
			}
			if description != "" {
				if doc.Returns.Description == "" {
					doc.Returns.Description = description
				} else {
					doc.Returns.Description += " " + description
				}
			}
			lastKind = kindReturns
			lastParam = nil
			lastRaises = nil

		case directive == "rtype":
			if doc.Returns == nil {
				doc.Returns = &model.DocReturn{}
			}
			doc.Returns.Type = description
			lastKind = kindNone

		case strings.HasPrefix(directive, "raises ") || strings.HasPrefix(directive, "raise "):
			excType := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(directive, "raises "), "raise "))
			r := &model.DocRaises{Type: excType, Description: description}
			doc.Raises = append(doc.Raises, r)
			lastKind = kindRaises
			lastParam = nil
			lastRaises = r

		default:
			// Unknown directive — reset continuation.
			lastKind = kindNone
			lastParam = nil
			lastRaises = nil
		}
	}

	return doc
}

// parseSphinxParamDirective splits the part after ":param " into (name, type).
// Handles both ":param name:" (no type) and ":param type name:" (inline type).
func parseSphinxParamDirective(s string) (name, typ string) {
	s = strings.TrimSpace(s)
	parts := strings.Fields(s)
	if len(parts) == 1 {
		// ":param name:"
		return parts[0], ""
	}
	if len(parts) >= 2 {
		// ":param type name:" — type is all but last token, name is last token.
		name = parts[len(parts)-1]
		typ = strings.Join(parts[:len(parts)-1], " ")
		return name, typ
	}
	return s, ""
}

// getOrCreateParam returns the existing DocParam for name, or creates a new one.
// NOTE: This does NOT add it to doc.Params — callers manage that.
func getOrCreateParam(m map[string]*model.DocParam, name string) *model.DocParam {
	if p, ok := m[name]; ok {
		return p
	}
	return &model.DocParam{Name: name}
}
