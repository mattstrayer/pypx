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
//	...
//
// Results are sorted alphabetically by path for determinism. The header line
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

	for _, mod := range in.Modules {
		for _, fn := range mod.Functions {
			path := mod.Name + "." + fn.Name
			if matchesQ(path) && matchesKind("function") {
				entries = append(entries, entry{path, "function", fn.Signature})
			}
		}
		for _, cls := range mod.Classes {
			path := mod.Name + "." + cls.Name
			if matchesQ(path) && matchesKind("class") {
				entries = append(entries, entry{path, "class", cls.Signature})
			}
			for _, m := range cls.Methods {
				mp := path + "." + m.Name
				if matchesQ(mp) && matchesKind("method") {
					entries = append(entries, entry{mp, "method", m.Signature})
				}
			}
		}
		for _, exc := range mod.Exceptions {
			path := mod.Name + "." + exc.Name
			if matchesQ(path) && matchesKind("exception") {
				entries = append(entries, entry{path, "exception", exc.Signature})
			}
		}
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })

	max := limit
	if len(entries) < max {
		max = len(entries)
	}
	for i := 0; i < max; i++ {
		e := entries[i]
		sig := strings.ReplaceAll(e.sig, "\t", " ")
		fmt.Fprintf(&b, "%s\t%s\t%s\n", e.path, e.kind, sig)
	}
	return b.String()
}
