package textfmt

import (
	"fmt"
	"slices"
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
	Aliases    []DocAliasInput
}

// DocAliasInput is a public name that points at a symbol documented elsewhere.
type DocAliasInput struct {
	Name   string
	Target string // Dotted module and name of the documented symbol.
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
	Attributes []DocAttributeInput
}

// DocAttributeInput is a class attribute or property.
type DocAttributeInput struct {
	Name        string
	Type        string
	Default     string
	Description string
	IsProperty  bool
	Writable    bool
}

// Kind returns "property" or "attribute".
func (a DocAttributeInput) Kind() string {
	if a.IsProperty {
		return "property"
	}
	return "attribute"
}

// Signature renders the attribute on one line: "x: int = 1", or
// "property x: int [read-only]".
func (a DocAttributeInput) Signature() string {
	s := a.Name
	if a.Type != "" {
		s += ": " + a.Type
	}
	if a.Default != "" {
		s += " = " + a.Default
	}
	if a.IsProperty {
		s = "property " + s
		if !a.Writable {
			s += " [read-only]"
		}
	}
	return lineBreaks.Replace(s)
}

// lineBreaks removes the characters that a multi-line string value would put
// into a one-line signature.
var lineBreaks = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ")

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
// A prefix also follows aliases. The block for an alias's target shows the target's
// path, as FormatSymbol does.
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
	matched := writeMatchingSymbols(&b, in, prefix)

	if prefix != "" {
		direct := matched
		seen := map[string]bool{}
		for _, mod := range in.Modules {
			for _, a := range mod.Aliases {
				path := mod.Name + "." + a.Name
				var target string
				switch {
				case direct && path == prefix:
					continue // A definition beats an alias with the same path.
				case matchesPrefix(path, prefix):
					target = a.Target
				case strings.HasPrefix(prefix, path+"."):
					target = a.Target + strings.TrimPrefix(prefix, path)
				default:
					continue
				}
				if matchesPrefix(target, prefix) || seen[target] {
					continue
				}
				seen[target] = true
				if writeMatchingSymbols(&b, in, target) {
					matched = true
				}
			}
		}
	}

	if !matched && prefix != "" {
		return fmt.Sprintf("# no symbols matching prefix=%s\n", prefix)
	}
	return b.String()
}

// writeMatchingSymbols writes the block of each symbol that matches prefix, under
// its module header, and reports whether it wrote any.
func writeMatchingSymbols(b *strings.Builder, in *DocsInput, prefix string) bool {
	matched := false

	for _, mod := range in.Modules {
		modHeaderWritten := false
		writeModuleHeader := func() {
			if !modHeaderWritten {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				fmt.Fprintf(b, "## %s (module)\n\n", mod.Name)
				modHeaderWritten = true
			}
		}

		for _, fn := range mod.Functions {
			path := mod.Name + "." + fn.Name
			if !matchesPrefix(path, prefix) {
				continue
			}
			writeModuleHeader()
			writeSymbolBlock(b, path, fn)
			matched = true
		}

		for _, cls := range slices.Concat(mod.Classes, mod.Exceptions) {
			path := mod.Name + "." + cls.Name
			classMatches := matchesPrefix(path, prefix)
			anyMethodMatches := false
			for _, m := range cls.Methods {
				if matchesPrefix(path+"."+m.Name, prefix) {
					anyMethodMatches = true
					break
				}
			}
			if !classMatches && !anyMethodMatches && !anyAttributeMatches(path, cls, prefix) {
				continue
			}
			writeModuleHeader()
			if classMatches {
				writeSymbolBlock(b, path, cls)
				matched = true
			}
			for _, m := range cls.Methods {
				mp := path + "." + m.Name
				if matchesPrefix(mp, prefix) {
					writeSymbolBlock(b, mp, m)
					matched = true
				}
			}
			if !classMatches && writeMatchingAttributes(b, path, cls, prefix) {
				matched = true
			}
		}
	}

	return matched
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

func anyAttributeMatches(path string, sym DocSymbolInput, prefix string) bool {
	for _, a := range sym.Attributes {
		if matchesPrefix(path+"."+a.Name, prefix) {
			return true
		}
	}
	return false
}

// writeMatchingAttributes writes a block for each attribute that matches prefix,
// and reports whether it wrote any.
func writeMatchingAttributes(b *strings.Builder, path string, sym DocSymbolInput, prefix string) bool {
	wrote := false
	for _, a := range sym.Attributes {
		if ap := path + "." + a.Name; matchesPrefix(ap, prefix) {
			writeAttributeBlock(b, ap, a)
			wrote = true
		}
	}
	return wrote
}

// writeAttributeBlock renders one attribute as a symbol of its own.
func writeAttributeBlock(b *strings.Builder, dottedPath string, a DocAttributeInput) {
	fmt.Fprintf(b, "### %s — %s\n", dottedPath, a.Signature())
	if a.Description != "" {
		b.WriteString(a.Description)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
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
	if len(sym.Attributes) > 0 {
		b.WriteString("\nAttributes:\n")
		for _, a := range sym.Attributes {
			b.WriteString("  ")
			b.WriteString(a.Name)
			if a.Type != "" {
				fmt.Fprintf(b, " (%s)", a.Type)
			}
			if a.Default != "" {
				fmt.Fprintf(b, " = %s", lineBreaks.Replace(a.Default))
			}
			if a.IsProperty {
				if a.Writable {
					b.WriteString(" [property]")
				} else {
					b.WriteString(" [property, read-only]")
				}
			}
			if a.Description != "" {
				fmt.Fprintf(b, ": %s", strings.ReplaceAll(a.Description, "\n", "\n    "))
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
//   - Attribute: "module.Class.attr" or "module.ExceptionClass.attr"
//   - Exception: "module.ExceptionClass", with methods as for a class
//
// A class lookup returns only the class block, not its methods. Methods are
// addressed individually by their dotted path. A method wins over an attribute
// with the same name.
//
// A path that starts with an alias resolves to the alias target, and the block
// shows the target's path.
func FormatSymbol(in *DocsInput, dotted string) (string, bool) {
	if out, ok := formatDirectSymbol(in, dotted); ok {
		return out, true
	}
	for _, mod := range in.Modules {
		for _, a := range mod.Aliases {
			path := mod.Name + "." + a.Name
			if dotted == path || strings.HasPrefix(dotted, path+".") {
				if out, ok := formatDirectSymbol(in, a.Target+strings.TrimPrefix(dotted, path)); ok {
					return out, true
				}
			}
		}
	}
	return "", false
}

// formatAttribute renders the attribute of sym called name.
func formatAttribute(sym DocSymbolInput, name, dotted string) (string, bool) {
	for _, a := range sym.Attributes {
		if a.Name == name {
			var b strings.Builder
			writeAttributeBlock(&b, dotted, a)
			return b.String(), true
		}
	}
	return "", false
}

// formatDirectSymbol looks dotted up among documented symbols only.
func formatDirectSymbol(in *DocsInput, dotted string) (string, bool) {
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

		// Class, exception, or one of their members under this module?
		for _, cls := range slices.Concat(mod.Classes, mod.Exceptions) {
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
				if out, ok := formatAttribute(cls, methodName, dotted); ok {
					return out, true
				}
			}
		}
	}
	return "", false
}
