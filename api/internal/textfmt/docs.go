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
