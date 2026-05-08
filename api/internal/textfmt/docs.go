package textfmt

import (
	"fmt"
	"strings"
)

// DocsInput mirrors handler.DocsResponse. Defined here so textfmt does not
// need to import handler (avoids an import cycle). Fields use the same JSON
// names and shapes; the handler converts before calling these formatters.
type DocsInput struct {
	Package     string
	Version     string
	Available   bool
	StubPackage string
	Modules     []DocModuleInput
}

type DocModuleInput struct {
	Name       string
	Functions  []DocSymbolInput
	Classes    []DocSymbolInput
	Exceptions []DocSymbolInput
}

type DocSymbolInput struct {
	Name       string
	Kind       string // "function" | "class" | "method" | "exception"
	Signature  string
	Docstring  string
	Parameters []DocParamInput
	Returns    *DocReturnInput
	Raises     []DocRaiseInput
	Methods    []DocSymbolInput
}

type DocParamInput struct {
	Name        string
	Type        string
	Description string
	Kind        string
	Default     string
}

type DocReturnInput struct {
	Type        string
	Description string
}

type DocRaiseInput struct {
	Type        string
	Description string
}

// FormatDocs renders DocsInput as agent-friendly plain text. Optional prefix
// filters output to symbols whose dotted path starts with prefix (or equals
// it). Empty prefix means full dump. If prefix is non-empty and no symbol
// matches, returns a single "# no symbols matching prefix=..." line.
//
// Output structure:
//
//	## <module> (module)
//
//	### <module>.<symbol> — <signature>
//	<docstring>
//	(parameters, returns, raises blocks if present)
func FormatDocs(in *DocsInput, prefix string) string {
	var b strings.Builder
	matched := false

	for _, mod := range in.Modules {
		modHeaderWritten := false
		writeModuleHeader := func() {
			if !modHeaderWritten {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				fmt.Fprintf(&b, "## %s (module)\n\n", mod.Name)
				modHeaderWritten = true
			}
		}

		for _, fn := range mod.Functions {
			path := mod.Name + "." + fn.Name
			if !matchesPrefix(path, prefix) {
				continue
			}
			writeModuleHeader()
			writeSymbolBlock(&b, path, fn)
			matched = true
		}

		for _, cls := range mod.Classes {
			path := mod.Name + "." + cls.Name
			classMatches := matchesPrefix(path, prefix)
			anyMethodMatches := false
			for _, m := range cls.Methods {
				if matchesPrefix(path+"."+m.Name, prefix) {
					anyMethodMatches = true
					break
				}
			}
			if !classMatches && !anyMethodMatches {
				continue
			}
			writeModuleHeader()
			if classMatches {
				writeSymbolBlock(&b, path, cls)
				matched = true
			}
			for _, m := range cls.Methods {
				mp := path + "." + m.Name
				if matchesPrefix(mp, prefix) {
					writeSymbolBlock(&b, mp, m)
					matched = true
				}
			}
		}

		for _, exc := range mod.Exceptions {
			path := mod.Name + "." + exc.Name
			if !matchesPrefix(path, prefix) {
				continue
			}
			writeModuleHeader()
			writeSymbolBlock(&b, path, exc)
			matched = true
		}
	}

	if !matched && prefix != "" {
		return fmt.Sprintf("# no symbols matching prefix=%s\n", prefix)
	}
	return b.String()
}

// matchesPrefix returns true if prefix is empty, equals path, or path starts
// with prefix+".".
func matchesPrefix(path, prefix string) bool {
	if prefix == "" {
		return true
	}
	if path == prefix {
		return true
	}
	return strings.HasPrefix(path, prefix+".")
}

// writeSymbolBlock renders a single symbol's heading + body to b.
func writeSymbolBlock(b *strings.Builder, dottedPath string, sym DocSymbolInput) {
	if sym.Signature != "" {
		fmt.Fprintf(b, "### %s — %s\n", dottedPath, sym.Signature)
	} else {
		fmt.Fprintf(b, "### %s\n", dottedPath)
	}
	if sym.Docstring != "" {
		b.WriteString(sym.Docstring)
		b.WriteByte('\n')
	}
	if len(sym.Parameters) > 0 {
		b.WriteString("\nParameters:\n")
		for _, p := range sym.Parameters {
			b.WriteString("  ")
			b.WriteString(p.Name)
			if p.Type != "" {
				fmt.Fprintf(b, " (%s)", p.Type)
			}
			if p.Default != "" {
				fmt.Fprintf(b, " = %s", p.Default)
			}
			if p.Description != "" {
				fmt.Fprintf(b, ": %s", p.Description)
			}
			b.WriteByte('\n')
		}
	}
	if sym.Returns != nil {
		b.WriteString("\nReturns:")
		if sym.Returns.Type != "" {
			fmt.Fprintf(b, " %s", sym.Returns.Type)
		}
		if sym.Returns.Description != "" {
			fmt.Fprintf(b, " — %s", sym.Returns.Description)
		}
		b.WriteByte('\n')
	}
	if len(sym.Raises) > 0 {
		b.WriteString("\nRaises:\n")
		for _, r := range sym.Raises {
			b.WriteString("  ")
			b.WriteString(r.Type)
			if r.Description != "" {
				fmt.Fprintf(b, ": %s", r.Description)
			}
			b.WriteByte('\n')
		}
	}
	b.WriteByte('\n')
}

// FormatSymbol returns the rendered block for a single symbol identified by
// its dotted path. Returns ok=false if no matching symbol is found.
//
// Lookup rules:
//   - Module-qualified function: "module.fn"
//   - Class: "module.Class"
//   - Method: "module.Class.method"
//   - Exception: "module.ExceptionClass"
//
// A class lookup returns only the class block, not its methods. Methods are
// addressed individually by their dotted path.
func FormatSymbol(in *DocsInput, dotted string) (string, bool) {
	for _, mod := range in.Modules {
		if !strings.HasPrefix(dotted, mod.Name+".") && dotted != mod.Name {
			continue
		}
		// Strip module prefix from the search target.
		rest := strings.TrimPrefix(dotted, mod.Name+".")
		if rest == "" {
			continue // module-only path; not a symbol
		}

		// Function under this module?
		for _, fn := range mod.Functions {
			if rest == fn.Name {
				var b strings.Builder
				writeSymbolBlock(&b, dotted, fn)
				return b.String(), true
			}
		}

		// Class or class.method under this module?
		for _, cls := range mod.Classes {
			if rest == cls.Name {
				var b strings.Builder
				writeSymbolBlock(&b, dotted, cls)
				return b.String(), true
			}
			methodPrefix := cls.Name + "."
			if strings.HasPrefix(rest, methodPrefix) {
				methodName := strings.TrimPrefix(rest, methodPrefix)
				for _, m := range cls.Methods {
					if methodName == m.Name {
						var b strings.Builder
						writeSymbolBlock(&b, dotted, m)
						return b.String(), true
					}
				}
			}
		}

		// Exception under this module?
		for _, exc := range mod.Exceptions {
			if rest == exc.Name {
				var b strings.Builder
				writeSymbolBlock(&b, dotted, exc)
				return b.String(), true
			}
		}
	}
	return "", false
}
