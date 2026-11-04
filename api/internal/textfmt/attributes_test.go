package textfmt_test

import (
	"strings"
	"testing"

	"github.com/pypx/api/internal/textfmt"
)

func attributeDocs() *textfmt.DocsInput {
	return &textfmt.DocsInput{
		Package: "p",
		Modules: []textfmt.DocModuleInput{{
			Name: "p",
			Classes: []textfmt.DocSymbolInput{{
				Name:      "C",
				Kind:      "class",
				Signature: "class C",
				Methods:   []textfmt.DocSymbolInput{{Name: "both", Kind: "method", Signature: "def both()"}},
				Attributes: []textfmt.DocAttributeInput{
					{Name: "limit", Type: "int", Default: "1", Description: "First line.\nSecond line."},
					{Name: "size", Type: "int", IsProperty: true, Description: "The size."},
					{Name: "name", IsProperty: true, Writable: true},
					{Name: "both"},
				},
			}},
			Exceptions: []textfmt.DocSymbolInput{{
				Name:       "E",
				Kind:       "exception",
				Signature:  "class E(Exception)",
				Attributes: []textfmt.DocAttributeInput{{Name: "code", Type: "int"}},
			}},
			Aliases: []textfmt.DocAliasInput{{Name: "Alias", Target: "p.C"}},
		}},
	}
}

func TestFormatDocs_ListsAttributes(t *testing.T) {
	out := textfmt.FormatDocs(attributeDocs(), "")
	for _, want := range []string{
		"Attributes:\n",
		"  limit (int) = 1: First line.\n    Second line.\n",
		"  size (int) [property, read-only]: The size.\n",
		"  name [property]\n",
		"  code (int)\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "### p.C.size") {
		t.Errorf("a full dump lists attributes under the class only:\n%s", out)
	}
}

func TestFormatDocs_AttributePrefix(t *testing.T) {
	out := textfmt.FormatDocs(attributeDocs(), "p.C.size")
	if !strings.Contains(out, "### p.C.size — property size: int [read-only]\nThe size.\n") {
		t.Errorf("output:\n%s", out)
	}
	if strings.Contains(out, "### p.C —") || strings.Contains(out, "limit") {
		t.Errorf("the prefix should select one attribute:\n%s", out)
	}
	if out := textfmt.FormatDocs(attributeDocs(), "p.E.code"); !strings.Contains(out, "### p.E.code — code: int\n") {
		t.Errorf("exception attribute output:\n%s", out)
	}
}

func TestFormatSymbol_Attributes(t *testing.T) {
	for dotted, want := range map[string]string{
		"p.C.size":     "### p.C.size — property size: int [read-only]\nThe size.\n",
		"p.C.name":     "### p.C.name — property name\n",
		"p.E.code":     "### p.E.code — code: int\n",
		"p.C.both":     "### p.C.both — def both()\n",
		"p.Alias.size": "### p.C.size — property size: int [read-only]\n",
	} {
		out, ok := textfmt.FormatSymbol(attributeDocs(), dotted)
		if !ok || !strings.Contains(out, want) {
			t.Errorf("%s: ok=%v, out=%q, want %q", dotted, ok, out, want)
		}
	}
	if _, ok := textfmt.FormatSymbol(attributeDocs(), "p.C.missing"); ok {
		t.Error("an unknown attribute must not resolve")
	}
}

func TestFormatSymbols_Attributes(t *testing.T) {
	out := textfmt.FormatSymbols(attributeDocs(), "", "", 100)
	for _, want := range []string{
		"p.C.limit\tattribute\tlimit: int = 1\n",
		"p.C.size\tproperty\tproperty size: int [read-only]\n",
		"p.C.name\tproperty\tproperty name\n",
		"p.E.code\tattribute\tcode: int\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	props := textfmt.FormatSymbols(attributeDocs(), "", "property", 100)
	if strings.Contains(props, "p.C.limit") || !strings.Contains(props, "p.C.size") {
		t.Errorf("kind=property output:\n%s", props)
	}
	attrs := textfmt.FormatSymbols(attributeDocs(), "", "attribute", 100)
	if !strings.Contains(attrs, "p.C.limit") || strings.Contains(attrs, "p.C.size") {
		t.Errorf("kind=attribute output:\n%s", attrs)
	}
}

func TestAttributeSignature_OneLine(t *testing.T) {
	a := textfmt.DocAttributeInput{Name: "x", Default: "\"\"\"a\nb\"\"\""}
	if got := a.Signature(); strings.Contains(got, "\n") {
		t.Errorf("Signature = %q", got)
	}
}
