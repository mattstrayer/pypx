package handler

import (
	"slices"
	"testing"

	"github.com/pypx/goopy"
	"github.com/pypx/goopy/model"
)

// extractModule runs the real extractor, so the kinds under test are the ones a
// wheel gives, not ones a test set by hand.
func extractModule(t *testing.T, py, pyi string) *model.Module {
	t.Helper()
	files := map[string][]byte{"m/__init__.py": []byte(py)}
	if pyi != "" {
		files["m/__init__.pyi"] = []byte(pyi)
	}
	pkg := goopy.ExtractPackage(t.Context(), "m", files, []string{"m"})
	if len(pkg.Modules) == 0 {
		t.Fatal("no modules")
	}
	return pkg.Modules[0]
}

func TestSignature_KeepsMarkers(t *testing.T) {
	tests := []struct {
		src  string
		want string
	}{
		{"def f(op, other, **kwargs): pass", "def f(op, other, **kwargs)"},
		{"def f(*args): pass", "def f(*args)"},
		{"def f(*args, a, **kw): pass", "def f(*args, a, **kw)"},
		{"def f(a, *, b): pass", "def f(a, *, b)"},
		{"def f(*, a=1, b: int = 2): pass", "def f(*, a = 1, b: int = 2)"},
		{"def f(a, /): pass", "def f(a, /)"},
		{"def f(a, /, b): pass", "def f(a, /, b)"},
		{"def f(a, /, *, b): pass", "def f(a, /, *, b)"},
		{"def f(a, /, *args, b, **kw): pass", "def f(a, /, *args, b, **kw)"},
		{"def f(a, b=1, /, c=2, *, d, e=3, **kw) -> int: pass", "def f(a, b = 1, /, c = 2, *, d, e = 3, **kw) -> int"},
		{"def f(*args: int, **kw: str): pass", "def f(*args: int, **kw: str)"},
		{"async def f(a, *, b): pass", "async def f(a, *, b)"},
		{"def f(): pass", "def f()"},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			sym := convertFunction(extractModule(t, tt.src+"\n", "").Functions[0], nil)
			if sym.Signature != tt.want {
				t.Errorf("Signature = %q, want %q", sym.Signature, tt.want)
			}
		})
	}
}

// The bound parameter leaves the list, and a "/" with nothing before it goes too.
func TestSignature_Methods(t *testing.T) {
	src := `class C:
    def __init__(self, a, *, b): pass
    def plain(self, a, /, b, *c, **d): pass
    def only_self(self, /): pass
    def after_self(self, /, a): pass
    def star(*args, **kw): pass
    @classmethod
    def make(cls, a, /, *, b): pass
    @staticmethod
    def st(a, /, *, b): pass
`
	cls := extractModule(t, src, "").Classes[0]
	want := map[string]string{
		"plain":      "def plain(a, /, b, *c, **d)",
		"only_self":  "def only_self()",
		"after_self": "def after_self(a)",
		"star":       "def star(*args, **kw)",
		"make":       "def make(a, /, *, b)",
		"st":         "def st(a, /, *, b)",
	}
	for _, m := range cls.Methods {
		if got := convertFunction(m, nil).Signature; got != want[m.Name] {
			t.Errorf("%s: %q, want %q", m.Name, got, want[m.Name])
		}
	}
	if got, want := convertFunction(cls.Constructor, nil).Signature, "def __init__(a, *, b)"; got != want {
		t.Errorf("constructor: %q, want %q", got, want)
	}
}

// Names and kinds come from the .py, and the stub adds the types. A stub-only
// function has its own kinds.
func TestSignature_Stubs(t *testing.T) {
	mod := extractModule(t, "def f(a, *args, b=1, **kw): pass\n",
		"def f(a: int, *args: str, b: int = ..., **kw: bool) -> None: ...\ndef only(x: int, /, *, y: str) -> None: ...\n")
	want := map[string]string{
		"f":    "def f(a: int, *args: str, b: int = 1, **kw: bool) -> None",
		"only": "def only(x: int, /, *, y: str) -> None",
	}
	for _, fn := range mod.Functions {
		if got := convertFunction(fn, nil).Signature; got != want[fn.Name] {
			t.Errorf("%s: %q, want %q", fn.Name, got, want[fn.Name])
		}
	}
}

// A stub package fills types without merging, and the markers still come out.
func TestSignature_StubPackageFill(t *testing.T) {
	fn := extractModule(t, "def f(a, *args, **kw): pass\n", "").Functions[0]
	stub := extractModule(t, "def f(a: int, *args: str, **kw: bool) -> None: pass\n", "").Functions[0]
	if got, want := convertFunction(fn, stub).Signature, "def f(a: int, *args: str, **kw: bool) -> None"; got != want {
		t.Errorf("Signature = %q, want %q", got, want)
	}
}

// One builder renders both, so a parameter list reads the same in each.
func TestSignature_MatchesOverloadForm(t *testing.T) {
	src := "def f(a: int, /, b: str = 'x', *c: int, d: bool, **e: int) -> None: pass\n"
	fn := extractModule(t, src, "").Functions[0]
	if got, want := convertFunction(fn, nil).Signature, overloadSignature("f", fn); got != want {
		t.Errorf("main %q, overload form %q", got, want)
	}
}

// A marker-only change is an API change. A diff could not see one before.
func TestDiff_SeesMarkerOnlyChange(t *testing.T) {
	signature := func(src string) map[string]string {
		mod := extractModule(t, src, "")
		return flattenSymbols(convertToDocsResponse("m", "1", &model.Package{Modules: []*model.Module{mod}}, stubIndex{}))
	}
	for _, tt := range []struct{ from, to string }{
		{"def f(a, b): pass\n", "def f(a, *, b): pass\n"},
		{"def f(a, b): pass\n", "def f(a, /, b): pass\n"},
		{"def f(a): pass\n", "def f(*a): pass\n"},
		{"def f(a): pass\n", "def f(**a): pass\n"},
	} {
		from, to := signature(tt.from)["m.f"], signature(tt.to)["m.f"]
		if from == "" || from == to {
			t.Errorf("%q to %q: rows %q and %q", tt.from, tt.to, from, to)
		}
	}
	if a, b := signature("def f(a, *, b): pass\n")["m.f"], signature("def f(a, *, b): pass\n")["m.f"]; a != b {
		t.Errorf("no change, rows %q and %q", a, b)
	}
}

func TestDecorators_FromStubInPackage(t *testing.T) {
	mod := extractModule(t, `def f(): pass
class C:
    def m(self): pass
    class Inner: pass
`, `@final
def f(): ...
@final
class C:
    @classmethod
    @deprecated('x')
    def m(cls): ...
    @final
    class Inner: ...
`)
	dm := convertToDocsResponse("m", "1", &model.Package{Modules: []*model.Module{mod}}, stubIndex{}).Modules[0]
	if want := []string{"final"}; !slices.Equal(dm.Functions[0].Decorators, want) {
		t.Errorf("f = %q", dm.Functions[0].Decorators)
	}
	c := dm.Classes[0]
	if want := []string{"final"}; !slices.Equal(c.Decorators, want) {
		t.Errorf("C = %q", c.Decorators)
	}
	if want := []string{"deprecated"}; !slices.Equal(c.Methods[0].Decorators, want) {
		t.Errorf("C.m = %q", c.Methods[0].Decorators)
	}
	if c.Methods[0].MethodKind != "classmethod" {
		t.Errorf("C.m kind = %q", c.Methods[0].MethodKind)
	}
	if want := []string{"final"}; !slices.Equal(c.Classes[0].Decorators, want) {
		t.Errorf("C.Inner = %q", c.Classes[0].Decorators)
	}
}

// A separate stub package has no merge step, so conversion applies its decorators.
func TestDecorators_FromStubPackage(t *testing.T) {
	src := extractModule(t, `def f(): pass
def g(): pass
class C:
    def m(self): pass
    class Inner: pass
`, "")
	stubPkg := &model.Package{Modules: []*model.Module{extractModule(t, `@final
def f(): pass
@overload
@deprecated('one')
def g(a: int): pass
@overload
def g(a: str): pass
@final
class C:
    @staticmethod
    @final
    def m(): pass
    @final
    class Inner: pass
`, "")}}
	dm := convertToDocsResponse("m", "1", &model.Package{Modules: []*model.Module{src}}, buildStubIndex(stubPkg)).Modules[0]
	byName := func(name string) DocSymbol {
		i := slices.IndexFunc(dm.Functions, func(s DocSymbol) bool { return s.Name == name })
		return dm.Functions[i]
	}
	if got := byName("f").Decorators; !slices.Equal(got, []string{"final"}) {
		t.Errorf("f = %q", got)
	}
	if got := byName("g").Decorators; len(got) != 0 {
		t.Errorf("g = %q, the deprecation belongs to one overload", got)
	}
	c := dm.Classes[0]
	if !slices.Equal(c.Decorators, []string{"final"}) || !slices.Equal(c.Classes[0].Decorators, []string{"final"}) {
		t.Errorf("C = %q, Inner = %q", c.Decorators, c.Classes[0].Decorators)
	}
	if m := c.Methods[0]; !slices.Equal(m.Decorators, []string{"final"}) || m.MethodKind != "staticmethod" {
		t.Errorf("C.m = %q kind %q", m.Decorators, m.MethodKind)
	}
}

// A re-exported class finds its stub under the path that defines it.
func TestDecorators_StubPackageOrigin(t *testing.T) {
	stubs := buildStubIndex(&model.Package{Modules: []*model.Module{{
		Name:    "m._core",
		Classes: []*model.Class{{Name: "C", Decorators: []string{"final"}}},
	}}})
	sym := convertClass(&model.Class{Name: "C", Origin: "m._core"}, stubs, "m")
	if !slices.Equal(sym.Decorators, []string{"final"}) {
		t.Errorf("Decorators = %q", sym.Decorators)
	}
}

// An @overload with no implementation shows only what applies to the whole function.
func TestDecorators_OverloadHolder(t *testing.T) {
	fn := extractModule(t, "", "@overload\n@final\n@deprecated('x')\ndef f(a: int): ...\n@overload\ndef f(a: str): ...\n").Functions[0]
	sym := convertFunction(fn, nil)
	if !slices.Equal(sym.Decorators, []string{"final"}) {
		t.Errorf("Decorators = %q", sym.Decorators)
	}
	if len(sym.Overloads) != 2 {
		t.Errorf("Overloads = %q", sym.Overloads)
	}
}
