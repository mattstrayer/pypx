package textfmt

import (
	"fmt"
	"sort"
	"strings"
)

// FormatSymbols emits a TSV index of symbols matching the optional
// case-insensitive substring query against the dotted path. kind filters by
// symbol kind (empty means all kinds). limit caps the result count (use a
// positive value).
//
// Output:
//
//	# path<TAB>kind<TAB>signature
//	httpx.Client<TAB>class<TAB>class Client(BaseClient)
//	pandas.DataFrame<TAB>alias<TAB>-> pandas.core.frame.DataFrame
//	...
//
// Results are sorted alphabetically by path, then kind, for determinism. The header line
// is always emitted (agents may skip lines starting with "#").
func FormatSymbols(in *DocsInput, q, kind string, limit int) string {
	var b strings.Builder
	b.WriteString("# path\tkind\tsignature\n")
	if limit <= 0 {
		return b.String()
	}

	type entry struct {
		path, kind, sig string
	}
	var entries []entry

	qLower := strings.ToLower(q)
	matchesQ := func(path string) bool {
		if q == "" {
			return true
		}
		return strings.Contains(strings.ToLower(path), qLower)
	}
	matchesKind := func(k string) bool {
		if kind == "" {
			return true
		}
		return k == kind
	}
	addAttributes := func(path string, sym DocSymbolInput) {
		for _, a := range sym.Attributes {
			ap := path + "." + a.Name
			if matchesQ(ap) && matchesKind(a.Kind()) {
				entries = append(entries, entry{ap, a.Kind(), a.Signature()})
			}
		}
	}

	for _, mod := range in.Modules {
		for _, fn := range mod.Functions {
			path := mod.Name + "." + fn.Name
			if matchesQ(path) && matchesKind("function") {
				entries = append(entries, entry{path, "function", fn.Signature})
			}
		}
		var addClass func(path, symKind string, cls DocSymbolInput)
		addClass = func(path, symKind string, cls DocSymbolInput) {
			if matchesQ(path) && matchesKind(symKind) {
				entries = append(entries, entry{path, symKind, cls.Signature})
			}
			for _, m := range cls.Methods {
				mp := path + "." + m.Name
				if matchesQ(mp) && matchesKind("method") {
					entries = append(entries, entry{mp, "method", m.Signature})
				}
			}
			addAttributes(path, cls)
			for _, n := range cls.Classes {
				addClass(path+"."+n.Name, "class", n)
			}
		}
		for _, cls := range mod.Classes {
			addClass(mod.Name+"."+cls.Name, "class", cls)
		}
		for _, exc := range mod.Exceptions {
			addClass(mod.Name+"."+exc.Name, "exception", exc)
		}
		for _, c := range mod.Constants {
			path := mod.Name + "." + c.Name
			if matchesQ(path) && matchesKind("constant") {
				entries = append(entries, entry{path, "constant", c.Signature()})
			}
		}
		for _, a := range mod.Aliases {
			path := mod.Name + "." + a.Name
			if matchesQ(path) && matchesKind("alias") {
				entries = append(entries, entry{path, "alias", "-> " + a.Target})
			}
		}
	}

	// A path can repeat, as a method and an attribute of the same name. Those order
	// by kind, and equal kinds keep their source order.
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].path != entries[j].path {
			return entries[i].path < entries[j].path
		}
		return entries[i].kind < entries[j].kind
	})

	max := limit
	if len(entries) < max {
		max = len(entries)
	}
	for i := 0; i < max; i++ {
		e := entries[i]
		sig := strings.ReplaceAll(lineBreaks.Replace(e.sig), "\t", " ")
		fmt.Fprintf(&b, "%s\t%s\t%s\n", e.path, e.kind, sig)
	}
	return b.String()
}
