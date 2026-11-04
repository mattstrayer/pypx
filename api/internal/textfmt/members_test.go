package textfmt_test

import (
	"strings"
	"testing"

	"github.com/pypx/api/internal/textfmt"
)

func membersFixture() *textfmt.DocsInput {
	return &textfmt.DocsInput{
		Package: "p", Version: "1", Available: true,
		Modules: []textfmt.DocModuleInput{{
			Name: "m",
			Functions: []textfmt.DocSymbolInput{{
				Name: "first", Kind: "function", Signature: "def first(iterable, default)",
				Decorators: []string{"contextmanager"},
				Overloads:  []string{"def first(iterable: Iterable[_T]) -> _T", "def first(iterable, default: str = \"a\nb\") -> _T"},
			}},
			Classes: []textfmt.DocSymbolInput{{
				Name: "Outer", Kind: "class", Signature: "class Outer",
				Decorators: []string{"dataclass", "final"},
				Methods: []textfmt.DocSymbolInput{{
					Name: "make", Kind: "function", Signature: "def make()",
					MethodKind: "classmethod", Decorators: []string{"abstractmethod"},
				}},
				Classes: []textfmt.DocSymbolInput{{
					Name: "Inner", Kind: "class", Signature: "class Inner(Base)", Docstring: "Inner doc.",
					Methods:    []textfmt.DocSymbolInput{{Name: "go", Kind: "function", Signature: "def go()"}},
					Attributes: []textfmt.DocAttributeInput{{Name: "x", Type: "int"}},
					Classes: []textfmt.DocSymbolInput{{
						Name: "Deep", Kind: "class", Signature: "class Deep",
						Methods: []textfmt.DocSymbolInput{{Name: "run", Kind: "function", Signature: "def run()"}},
					}},
				}},
			}},
		}},
	}
}

func TestFormatDocsBadgesAfterSignature(t *testing.T) {
	out := textfmt.FormatDocs(membersFixture(), "")
	for _, line := range []string{
		"### m.first — def first(iterable, default) [contextmanager]\n",
		"### m.Outer — class Outer [dataclass, final]\n",
		"### m.Outer.make — def make() [classmethod, abstract]\n",
		"### m.Outer.Inner.go — def go()\n",
	} {
		if !strings.Contains(out, line) {
			t.Errorf("missing %q in:\n%s", line, out)
		}
	}
}

func TestFormatDocsOverloadLines(t *testing.T) {
	out := textfmt.FormatDocs(membersFixture(), "m.first")
	want := "### m.first — def first(iterable, default) [contextmanager]\n" +
		"Overload: def first(iterable: Iterable[_T]) -> _T\n" +
		"Overload: def first(iterable, default: str = \"a b\") -> _T\n"
	if !strings.Contains(out, want) {
		t.Errorf("missing %q in:\n%s", want, out)
	}
}

func TestFormatDocsNestedBlocks(t *testing.T) {
	out := textfmt.FormatDocs(membersFixture(), "")
	for _, heading := range []string{
		"### m.Outer.Inner — class Inner(Base)\n",
		"### m.Outer.Inner.go — def go()\n",
		"### m.Outer.Inner.Deep — class Deep\n",
		"### m.Outer.Inner.Deep.run — def run()\n",
	} {
		if strings.Count(out, heading) != 1 {
			t.Errorf("want %q once in:\n%s", heading, out)
		}
	}
}

func TestFormatDocsPrefixFindsNestedMembers(t *testing.T) {
	tests := []struct {
		prefix   string
		want     []string
		dontWant []string
	}{
		{"m.Outer.Inner", []string{"### m.Outer.Inner —", "### m.Outer.Inner.go", "### m.Outer.Inner.Deep.run"}, []string{"### m.Outer —", "m.Outer.make"}},
		{"m.Outer.Inner.Deep.run", []string{"## m (module)", "### m.Outer.Inner.Deep.run"}, []string{"### m.Outer.Inner —"}},
		{"m.Outer.Inner.x", []string{"### m.Outer.Inner.x — x: int"}, []string{"### m.Outer.Inner —"}},
	}
	for _, tt := range tests {
		out := textfmt.FormatDocs(membersFixture(), tt.prefix)
		for _, s := range tt.want {
			if !strings.Contains(out, s) {
				t.Errorf("prefix %s: missing %q in:\n%s", tt.prefix, s, out)
			}
		}
		for _, s := range tt.dontWant {
			if strings.Contains(out, s) {
				t.Errorf("prefix %s: unexpected %q in:\n%s", tt.prefix, s, out)
			}
		}
	}
	if out := textfmt.FormatDocs(membersFixture(), "m.Outer.Nope"); !strings.HasPrefix(out, "# no symbols matching") {
		t.Errorf("no match: %q", out)
	}
}

func TestFormatSymbolNested(t *testing.T) {
	tests := []struct{ path, want string }{
		{"m.Outer.Inner", "### m.Outer.Inner — class Inner(Base)\nInner doc."},
		{"m.Outer.Inner.go", "### m.Outer.Inner.go — def go()"},
		{"m.Outer.Inner.x", "### m.Outer.Inner.x — x: int"},
		{"m.Outer.Inner.Deep", "### m.Outer.Inner.Deep — class Deep"},
		{"m.Outer.Inner.Deep.run", "### m.Outer.Inner.Deep.run — def run()"},
		{"m.Outer.make", "### m.Outer.make — def make() [classmethod, abstract]"},
	}
	for _, tt := range tests {
		out, ok := textfmt.FormatSymbol(membersFixture(), tt.path)
		if !ok || !strings.HasPrefix(out, tt.want) {
			t.Errorf("FormatSymbol(%s) = %q, %v; want prefix %q", tt.path, out, ok, tt.want)
		}
	}
	if _, ok := textfmt.FormatSymbol(membersFixture(), "m.Outer.Inner.Nope"); ok {
		t.Error("found a member that does not exist")
	}
}

func TestFormatSymbolsNested(t *testing.T) {
	out := textfmt.FormatSymbols(membersFixture(), "", "", 100)
	for _, row := range []string{
		"m.Outer.Inner\tclass\tclass Inner(Base)\n",
		"m.Outer.Inner.go\tmethod\tdef go()\n",
		"m.Outer.Inner.x\tattribute\tx: int\n",
		"m.Outer.Inner.Deep\tclass\tclass Deep\n",
		"m.Outer.Inner.Deep.run\tmethod\tdef run()\n",
		// One row per function, and no badges.
		"m.first\tfunction\tdef first(iterable, default)\n",
		"m.Outer.make\tmethod\tdef make()\n",
		"m.Outer\tclass\tclass Outer\n",
	} {
		if !strings.Contains(out, row) {
			t.Errorf("missing %q in:\n%s", row, out)
		}
	}
	if strings.Contains(out, "Overload") {
		t.Errorf("symbols.txt shows overloads:\n%s", out)
	}
	classes := textfmt.FormatSymbols(membersFixture(), "", "class", 100)
	if strings.Contains(classes, "method") || !strings.Contains(classes, "m.Outer.Inner.Deep\t") {
		t.Errorf("kind=class:\n%s", classes)
	}
}
