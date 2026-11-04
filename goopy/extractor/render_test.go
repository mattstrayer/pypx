package extractor

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"slices"
	"testing"

	"github.com/pypx/goopy/ast"
	"github.com/pypx/goopy/model"
	"github.com/pypx/goopy/parser"
)

// renderCases holds source expressions by the node kind they exercise. An empty
// want means the rendering equals the source. Every want parses in CPython to the
// same tree as its source, and TestRenderCasesMatchCPython checks that when python3
// is available.
var renderCases = map[string][]struct{ src, want string }{
	"literal": {
		{src: `"a"`}, {src: `'a'`}, {src: `b"a"`}, {src: `f"{a!r}"`}, {src: `"a" "b"`},
		{src: "1_000"}, {src: "0x10"}, {src: "1.5e3"}, {src: "2j"},
		{src: "None"}, {src: "True"}, {src: "..."},
	},
	"tuple": {
		{src: "()"}, {src: "(a,)"}, {src: "(a, b)"}, {src: "((a, b), c)"},
		{src: "a, b", want: "(a, b)"}, {src: "a,", want: "(a,)"},
		{src: `(".netrc", "_netrc")`}, {src: "(*a, b)"},
		{src: "[(a, b), (c,)]"}, {src: "{(a, b): (c, d)}"}, {src: "f((a, b), ())"},
	},
	"subscript": {
		{src: "Dict[str, int]"}, {src: "Tuple[int, ...]"}, {src: "Callable[[int], str]"},
		{src: "a[b]"}, {src: "a[b,]"}, {src: "a[()]"}, {src: "a[(b, c), d]"},
		{src: "a[(b, c)]", want: "a[b, c]"},
		{src: "a[1:2]"}, {src: "a[:]"}, {src: "a[::2]"}, {src: "a[1:2, ::3]"},
		{src: "a[*b]"}, {src: "a[b][c]"}, {src: "(a + b)[c]"},
		{src: "Dict[str, Tuple[int, str]]"}, {src: "Annotated[int, (1, 2)]"},
	},
	"comprehension": {
		{src: "[a for a in b]"}, {src: "{a for a in b}"}, {src: "(a for a in b)"},
		{src: "{v: k for k, v in c.items()}"}, {src: "{**a for a in b}"},
		{src: "[*a for a in b]"},
		{src: "[a async for a in b]"}, {src: "[a for a in b if c if d]"},
		{src: "[a for b in c for a in b]"},
		{src: "[(a, b) for a, b in c]"}, {src: "[a for a, in b]"}, {src: "[a for a, *b in c]"},
		{src: "[a for (a, b) in c]", want: "[a for a, b in c]"},
		{src: "[a for a.b in c]"}, {src: "[a for a[0] in c]"},
		{src: "[a if b else c for a in d]"},
		{src: "[a for a in (b if c else d)]"},
		{src: "[a for a in (lambda: b)()]"},
		{src: "[a for a in (b, c)]"},
		{src: "[(a := b) for b in c]"},
		{src: "[a for a in [b for b in c]]"},
		{src: "{a: [b for b in a] for a in c}"},
	},
	"call": {
		{src: "f()"}, {src: "f(a, b=c)"}, {src: "f(*a, **b)"}, {src: "f(*a, b, c=d, **e)"},
		{src: "f(a for a in b)"}, {src: "f((a for a in b), c)"}, {src: "f((a for a in b), c=d)"},
		{src: "f(*a or b)", want: "f(*(a or b))"},
		{src: "(lambda: a)()"}, {src: "(a or b)()"}, {src: "f(a)(b).c"},
		{src: "f(lambda a: a)"}, {src: "f(a=lambda: b)"},
	},
	"attribute": {
		{src: "a.b.c"}, {src: `"a".join(b)`}, {src: "(a + b).c"}, {src: "(a or b).c"},
		{src: "(a if b else c).d"}, {src: "(-a).b"}, {src: "(await a).b"}, {src: "(lambda: a).b"},
		{src: "(1).real"}, {src: "1.5.real"}, {src: "[a].b"},
		{src: "(a, b).c"}, {src: "{a: b}.c"}, {src: "(a for a in b).c"},
	},
	"arithmetic": {
		{src: "a + b * c"}, {src: "(a + b) * c"}, {src: "a - (b - c)"}, {src: "a - b - c"},
		{src: "(a - b) - c", want: "a - b - c"},
		{src: "a * (b + c) / d"}, {src: "a % (b, c)"}, {src: `"%s" % (a, b)`},
		{src: "a @ b"}, {src: "a // b"}, {src: "a << (b + c)", want: "a << b + c"}, {src: "(a << b) + c"},
		{src: "a | b & c"}, {src: "(a | b) & c"}, {src: "a ^ (b | c)"},
		{src: "a ** b ** c"}, {src: "(a ** b) ** c"}, {src: "(-a) ** b"}, {src: "-a ** b"},
		{src: "10 ** -6"},
		{src: "(a + b) ** c"}, {src: "a ** (b + c)"},
	},
	"unary": {
		{src: "-a"}, {src: "~a"}, {src: "+a"}, {src: "--a"}, {src: "-(a + b)"},
		{src: "not a"}, {src: "not not a"}, {src: "not (a or b)"}, {src: "not a == b"},
		{src: "(not a) == b"}, {src: "-(not a)"},
	},
	"comparison": {
		{src: "a == b"}, {src: "a is not b"}, {src: "a not in b"}, {src: "a in b"},
		{src: "a < b < c"}, {src: "(a < b) < c"}, {src: "a < (b < c)"}, {src: "(a < b) < c < d"},
		{src: "(a in b) == c"}, {src: "a + 1 < b * 2"},
	},
	"boolean": {
		{src: "a or b or c"}, {src: "(a or b) or c"}, {src: "a or (b or c)"},
		{src: "a and b and c"}, {src: "(a and b) and c"}, {src: "a and (b and c)"},
		{src: "a or b and c"}, {src: "(a or b) and c"}, {src: "a and not b"},
		{src: "(a or b or c) and d"},
	},
	"ternary": {
		{src: "a if b else c"}, {src: "(a if b else c) if d else e"},
		{src: "a if (b if c else d) else e"}, {src: "a if b else c if d else e"},
		{src: "(a if b else c) + d"}, {src: "a or (b if c else d)"},
	},
	"lambda": {
		{src: "lambda: a"}, {src: "lambda a, b=1: a"}, {src: "lambda *a, **b: a"},
		{src: "lambda: (a, b)"}, {src: "lambda: (yield)"},
		{src: "(lambda: a) or b"}, {src: "lambda: a or b"}, {src: "lambda: lambda: a"},
	},
	"display": {
		{src: "[]"}, {src: "{}"}, {src: "[a, *b]"}, {src: "{a, *b}"},
		{src: "{**a, 'k': 1}"}, {src: "{**(a or b)}"}, {src: "{a: b if c else d}"},
		{src: "[a if b else c, lambda: d]"},
	},
	"await": {
		{src: "await a.b"}, {src: "await a(b)"}, {src: "await (a + b)"},
		{src: "(await a) + b", want: "await a + b"},
		{src: "(await a) ** b", want: "await a ** b"},
		{src: "-(await a)", want: "-await a"},
	},
	"yield": {
		{src: "(yield)"}, {src: "(yield a)"}, {src: "(yield a, b)"}, {src: "(yield from a)"},
		{src: "f((yield))"}, {src: "(yield) + 1"},
	},
	"named": {
		{src: "(b := 1)"}, {src: "(b := c + 1)"}, {src: "[b := 1, 2]", want: "[(b := 1), 2]"},
		{src: "f(b := 1)", want: "f((b := 1))"}, {src: "(b := 1) + 2"}, {src: "(a, (b := 1))"},
	},
	"starred": {
		{src: "[*a, *b]"}, {src: "[*(a or b)]"}, {src: "f(*a.b)"},
	},
}

func TestExprToString(t *testing.T) {
	for kind, cases := range renderCases {
		t.Run(kind, func(t *testing.T) {
			for _, c := range cases {
				want := c.want
				if want == "" {
					want = c.src
				}
				if got := renderExpr(t, c.src); got != want {
					t.Errorf("%s: got %q, want %q", c.src, got, want)
				}
				// Rendering what we rendered must change nothing.
				if got := renderExpr(t, want); got != want {
					t.Errorf("%s: second pass got %q", want, got)
				}
			}
		})
	}
}

func renderExpr(t *testing.T, src string) string {
	t.Helper()
	p := parser.New([]byte("x = " + src + "\n"))
	mod := p.Parse()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("%s: parse errors: %v", src, errs)
	}
	return exprToString(mod.Body[0].(*ast.Assign).Value)
}

// TestRenderCasesMatchCPython checks with CPython's ast module that each rendering
// means what its source means, so a wrong expectation cannot hide in the table.
func TestRenderCasesMatchCPython(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}
	var pairs [][2]string
	for _, cases := range renderCases {
		for _, c := range cases {
			if c.want != "" {
				pairs = append(pairs, [2]string{c.src, c.want})
			}
			pairs = append(pairs, [2]string{c.src, renderExpr(t, c.src)})
		}
	}
	input, err := json.Marshal(pairs)
	if err != nil {
		t.Fatal(err)
	}
	// PEP 798 needs Python 3.15, so a source this python3 cannot parse is skipped.
	const script = `
import ast, json, sys
def tree(s):
    return ast.dump(ast.parse("async def f():\n x = " + s + "\n"))
for src, got in json.load(sys.stdin):
    try:
        want = tree(src)
    except SyntaxError:
        continue
    try:
        same = tree(got) == want
    except SyntaxError:
        same = False
    if not same:
        print(src, "->", got)
`
	cmd := exec.Command("python3", "-I", "-c", script)
	cmd.Stdin = bytes.NewReader(input)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("python3: %v\n%s", err, out)
	}
	if len(out) > 0 {
		t.Errorf("renderings that differ from their source in CPython:\n%s", out)
	}
}

// The renderer must not wrap a tuple that the subscript syntax already delimits, in
// any place that holds a type.
func TestAnnotationsGainNoParentheses(t *testing.T) {
	m := extract("x: Dict[str, int] = {}\ndef f(a: Tuple[int, str], *b: Dict[str, Tuple[int, ...]]) -> Callable[[int, str], Dict[str, int]]: pass\n")
	if got := m.Attributes[0].Type.Raw; got != "Dict[str, int]" {
		t.Errorf("attribute: got %q", got)
	}
	f := m.Functions[0]
	if got := f.Parameters[0].Type.Raw; got != "Tuple[int, str]" {
		t.Errorf("param a: got %q", got)
	}
	if got := f.Parameters[1].Type.Raw; got != "Dict[str, Tuple[int, ...]]" {
		t.Errorf("param b: got %q", got)
	}
	if got := f.Returns.Raw; got != "Callable[[int, str], Dict[str, int]]" {
		t.Errorf("returns: got %q", got)
	}
}

func TestValuesKeepTheirMeaning(t *testing.T) {
	m := extract(`NETRC_FILES = (".netrc", "_netrc")
COLORS_BY_VALUE = {v: k for k, v in COLORS_BY_NAME.items()}
EMPTY = ()
ONE = (1,)
SIZE = (a + b) * c
def f(a=(), b=(1,), c=(1, 2), d=[i for i in x], e=(i for i in x)): pass
`)
	for name, want := range map[string]string{
		"NETRC_FILES":     `(".netrc", "_netrc")`,
		"COLORS_BY_VALUE": "{v: k for k, v in COLORS_BY_NAME.items()}",
		"EMPTY":           "()",
		"ONE":             "(1,)",
		"SIZE":            "(a + b) * c",
	} {
		i := slices.IndexFunc(m.Attributes, func(a *model.Attribute) bool { return a.Name == name })
		if i < 0 {
			t.Errorf("%s: not found", name)
			continue
		}
		if got := m.Attributes[i].Value; got != want {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}
	want := []string{"()", "(1,)", "(1, 2)", "[i for i in x]", "(i for i in x)"}
	for i, p := range m.Functions[0].Parameters {
		if p.Default != want[i] {
			t.Errorf("param %s: got %q, want %q", p.Name, p.Default, want[i])
		}
	}
}

func TestComprehensionIsNotReadAsExportList(t *testing.T) {
	for _, src := range []string{
		`__all__ = ["a" for _ in b]`,
		`__all__ = ("a" for _ in b)`,
		`__all__ = {"a" for _ in b}`,
	} {
		m := extract(src + "\n")
		if m.Exports == nil || !m.Exports.Dynamic || len(m.Exports.Names) != 0 {
			t.Errorf("%s: got %+v, want a dynamic list with no names", src, m.Exports)
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
