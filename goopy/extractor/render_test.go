package extractor

import (
	"testing"

	"github.com/pypx/goopy/ast"
	"github.com/pypx/goopy/parser"
)

func TestExprToString_NewNodes(t *testing.T) {
	cases := map[string]string{
		"{**a, 'k': 1}": "{**a, 'k': 1}",
		"(b := 1)":      "b := 1",
	}
	for src, want := range cases {
		mod := parser.New([]byte("x = " + src + "\n")).Parse()
		got := exprToString(mod.Body[0].(*ast.Assign).Value)
		if got != want {
			t.Errorf("%s: got %q, want %q", src, got, want)
		}
	}
}
