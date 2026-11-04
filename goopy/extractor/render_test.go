package extractor

import (
	"testing"

	"github.com/pypx/goopy/ast"
	"github.com/pypx/goopy/parser"
)

func TestExprToString_NewNodes(t *testing.T) {
	cases := map[string]string{
		"{**a, 'k': 1}":  "{**a, 'k': 1}",
		"(b := 1)":       "b := 1",
		"y[1:2, ::3]":    "y[1:2, ::3]",
		"y[:]":           "y[:]",
		"await a.b":      "await a.b",
		"(yield)":        "yield",
		"(yield a)":      "yield a",
		"(yield from a)": "yield from a",
		"a @ b":          "a @ b",
		"a in b":         "a in b",
	}
	for src, want := range cases {
		mod := parser.New([]byte("x = " + src + "\n")).Parse()
		got := exprToString(mod.Body[0].(*ast.Assign).Value)
		if got != want {
			t.Errorf("%s: got %q, want %q", src, got, want)
		}
	}
}

func TestComparisonOperatorsKeepBothWords(t *testing.T) {
	m := extract("def f(a=x is not y, b=x not in y, c=x is y, d=x in y, e=not x, g=not x in y): pass\n")
	want := []string{"x is not y", "x not in y", "x is y", "x in y", "not x", "not x in y"}
	params := m.Functions[0].Parameters
	if len(params) != len(want) {
		t.Fatalf("got %d params, want %d", len(params), len(want))
	}
	for i, w := range want {
		if params[i].Default != w {
			t.Errorf("param %s: got %q, want %q", params[i].Name, params[i].Default, w)
		}
	}
}
