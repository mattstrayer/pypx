package parser

import (
	"strings"
	"testing"

	"github.com/pypx/goopy/ast"
	"github.com/pypx/goopy/token"
)

func TestParseDeeplyNestedDoesNotOverflow(t *testing.T) {
	for _, open := range []string{"[", "(", "{"} {
		src := "x = " + strings.Repeat(open, 5000)
		_, errs := parseWithErrors(t, src)
		if len(errs) == 0 {
			t.Errorf("open=%q: expected a parse error from the depth cap, got none", open)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func parse(t *testing.T, src string) *ast.Module {
	t.Helper()
	p := New([]byte(src))
	mod := p.Parse()
	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Logf("parse error at %d:%d: %s", e.Pos.Line, e.Pos.Col, e.Msg)
		}
		t.Fatalf("unexpected parse errors")
	}
	return mod
}

func parseWithErrors(t *testing.T, src string) (*ast.Module, []Error) {
	t.Helper()
	p := New([]byte(src))
	mod := p.Parse()
	return mod, p.Errors()
}

func requireStmts(t *testing.T, mod *ast.Module, n int) {
	t.Helper()
	if len(mod.Body) != n {
		t.Fatalf("expected %d statements, got %d", n, len(mod.Body))
	}
}

func asFuncDef(t *testing.T, stmt ast.Stmt) *ast.FunctionDef {
	t.Helper()
	fd, ok := stmt.(*ast.FunctionDef)
	if !ok {
		t.Fatalf("expected FunctionDef, got %T", stmt)
	}
	return fd
}

func asClassDef(t *testing.T, stmt ast.Stmt) *ast.ClassDef {
	t.Helper()
	cd, ok := stmt.(*ast.ClassDef)
	if !ok {
		t.Fatalf("expected ClassDef, got %T", stmt)
	}
	return cd
}

func asImport(t *testing.T, stmt ast.Stmt) *ast.Import {
	t.Helper()
	imp, ok := stmt.(*ast.Import)
	if !ok {
		t.Fatalf("expected Import, got %T", stmt)
	}
	return imp
}

func asImportFrom(t *testing.T, stmt ast.Stmt) *ast.ImportFrom {
	t.Helper()
	imp, ok := stmt.(*ast.ImportFrom)
	if !ok {
		t.Fatalf("expected ImportFrom, got %T", stmt)
	}
	return imp
}

func asAssign(t *testing.T, stmt ast.Stmt) *ast.Assign {
	t.Helper()
	a, ok := stmt.(*ast.Assign)
	if !ok {
		t.Fatalf("expected Assign, got %T", stmt)
	}
	return a
}

func asAnnAssign(t *testing.T, stmt ast.Stmt) *ast.AnnAssign {
	t.Helper()
	a, ok := stmt.(*ast.AnnAssign)
	if !ok {
		t.Fatalf("expected AnnAssign, got %T", stmt)
	}
	return a
}

func asTypeAlias(t *testing.T, stmt ast.Stmt) *ast.TypeAliasDef {
	t.Helper()
	ta, ok := stmt.(*ast.TypeAliasDef)
	if !ok {
		t.Fatalf("expected TypeAliasDef, got %T", stmt)
	}
	return ta
}

func asExprStmt(t *testing.T, stmt ast.Stmt) *ast.ExprStmt {
	t.Helper()
	es, ok := stmt.(*ast.ExprStmt)
	if !ok {
		t.Fatalf("expected ExprStmt, got %T", stmt)
	}
	return es
}

func asConstant(t *testing.T, expr ast.Expr) *ast.Constant {
	t.Helper()
	c, ok := expr.(*ast.Constant)
	if !ok {
		t.Fatalf("expected Constant, got %T", expr)
	}
	return c
}

func asName(t *testing.T, expr ast.Expr) *ast.Name {
	t.Helper()
	n, ok := expr.(*ast.Name)
	if !ok {
		t.Fatalf("expected Name, got %T", expr)
	}
	return n
}

// ---------------------------------------------------------------------------
// Task 7: Function definitions
// ---------------------------------------------------------------------------

func TestFuncDef_Simple(t *testing.T) {
	mod := parse(t, "def foo(): pass\n")
	requireStmts(t, mod, 1)
	fd := asFuncDef(t, mod.Body[0])
	if fd.Name != "foo" {
		t.Errorf("name = %q, want foo", fd.Name)
	}
	if fd.IsAsync {
		t.Error("expected non-async")
	}
	if fd.Returns != nil {
		t.Error("expected no return annotation")
	}
}

func TestFuncDef_Parameters(t *testing.T) {
	mod := parse(t, "def foo(a, b, c): pass\n")
	requireStmts(t, mod, 1)
	fd := asFuncDef(t, mod.Body[0])

	args := fd.Args
	if len(args.Args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(args.Args))
	}
	want := []string{"a", "b", "c"}
	for i, a := range args.Args {
		if a.Name != want[i] {
			t.Errorf("arg %d: %q, want %q", i, a.Name, want[i])
		}
	}
}

func TestFuncDef_Annotations(t *testing.T) {
	mod := parse(t, "def foo(x: int, y: str) -> bool: pass\n")
	requireStmts(t, mod, 1)
	fd := asFuncDef(t, mod.Body[0])

	// Check parameter annotations.
	if len(fd.Args.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(fd.Args.Args))
	}
	for _, a := range fd.Args.Args {
		if a.Annotation == nil {
			t.Errorf("expected annotation for %s", a.Name)
		}
	}

	// Check return annotation.
	if fd.Returns == nil {
		t.Fatal("expected return annotation")
	}
	ret := asName(t, fd.Returns)
	if ret.Name != "bool" {
		t.Errorf("return annotation = %q, want bool", ret.Name)
	}
}

func TestFuncDef_Defaults(t *testing.T) {
	mod := parse(t, "def foo(x=1, y=2): pass\n")
	requireStmts(t, mod, 1)
	fd := asFuncDef(t, mod.Body[0])

	if len(fd.Args.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(fd.Args.Args))
	}
	if len(fd.Args.Defaults) != 2 {
		t.Fatalf("expected 2 defaults, got %d", len(fd.Args.Defaults))
	}
	d0 := asConstant(t, fd.Args.Defaults[0])
	if d0.Value != "1" {
		t.Errorf("default[0] = %q, want 1", d0.Value)
	}
}

func TestFuncDef_PosOnly(t *testing.T) {
	mod := parse(t, "def foo(a, b, /, c): pass\n")
	requireStmts(t, mod, 1)
	fd := asFuncDef(t, mod.Body[0])

	if len(fd.Args.PosOnlyArgs) != 2 {
		t.Fatalf("expected 2 pos-only args, got %d", len(fd.Args.PosOnlyArgs))
	}
	if fd.Args.PosOnlyArgs[0].Name != "a" || fd.Args.PosOnlyArgs[1].Name != "b" {
		t.Errorf("pos-only args = %v, want [a, b]", fd.Args.PosOnlyArgs)
	}
	if len(fd.Args.Args) != 1 || fd.Args.Args[0].Name != "c" {
		t.Errorf("regular args = %v, want [c]", fd.Args.Args)
	}
}

func TestFuncDef_KwOnly(t *testing.T) {
	mod := parse(t, "def foo(a, *, b, c): pass\n")
	requireStmts(t, mod, 1)
	fd := asFuncDef(t, mod.Body[0])

	if len(fd.Args.Args) != 1 || fd.Args.Args[0].Name != "a" {
		t.Errorf("regular args = %v, want [a]", fd.Args.Args)
	}
	if fd.Args.VarArg != nil {
		t.Error("expected no vararg (bare *)")
	}
	if len(fd.Args.KwOnlyArgs) != 2 {
		t.Fatalf("expected 2 kw-only args, got %d", len(fd.Args.KwOnlyArgs))
	}
	if fd.Args.KwOnlyArgs[0].Name != "b" || fd.Args.KwOnlyArgs[1].Name != "c" {
		t.Errorf("kw-only args = %v, want [b, c]", fd.Args.KwOnlyArgs)
	}
}

func TestFuncDef_Varargs(t *testing.T) {
	mod := parse(t, "def foo(*args, **kwargs): pass\n")
	requireStmts(t, mod, 1)
	fd := asFuncDef(t, mod.Body[0])

	if fd.Args.VarArg == nil || fd.Args.VarArg.Name != "args" {
		t.Errorf("vararg = %v, want *args", fd.Args.VarArg)
	}
	if fd.Args.VarKwArg == nil || fd.Args.VarKwArg.Name != "kwargs" {
		t.Errorf("varkwarg = %v, want **kwargs", fd.Args.VarKwArg)
	}
}

func TestFuncDef_Async(t *testing.T) {
	mod := parse(t, "async def foo(): pass\n")
	requireStmts(t, mod, 1)
	fd := asFuncDef(t, mod.Body[0])
	if !fd.IsAsync {
		t.Error("expected async function")
	}
	if fd.Name != "foo" {
		t.Errorf("name = %q, want foo", fd.Name)
	}
}

func TestFuncDef_Docstring(t *testing.T) {
	src := `def foo():
    """This is a docstring."""
    pass
`
	mod := parse(t, src)
	requireStmts(t, mod, 1)
	fd := asFuncDef(t, mod.Body[0])

	if len(fd.Body) < 1 {
		t.Fatal("expected body statements")
	}
	es := asExprStmt(t, fd.Body[0])
	c := asConstant(t, es.Value)
	if c.Kind != "str" {
		t.Errorf("docstring kind = %q, want str", c.Kind)
	}
	if c.Value != "This is a docstring." {
		t.Errorf("docstring value = %q", c.Value)
	}
}

func TestFuncDef_Decorated(t *testing.T) {
	src := `@staticmethod
def foo(): pass
`
	mod := parse(t, src)
	requireStmts(t, mod, 1)
	fd := asFuncDef(t, mod.Body[0])
	if len(fd.Decorators) != 1 {
		t.Fatalf("expected 1 decorator, got %d", len(fd.Decorators))
	}
	dec := asName(t, fd.Decorators[0])
	if dec.Name != "staticmethod" {
		t.Errorf("decorator = %q, want staticmethod", dec.Name)
	}
}

func TestFuncDef_KwOnlyWithDefaults(t *testing.T) {
	mod := parse(t, "def foo(*, a, b=2): pass\n")
	requireStmts(t, mod, 1)
	fd := asFuncDef(t, mod.Body[0])

	if len(fd.Args.KwOnlyArgs) != 2 {
		t.Fatalf("expected 2 kw-only args, got %d", len(fd.Args.KwOnlyArgs))
	}
	if len(fd.Args.KwDefaults) != 2 {
		t.Fatalf("expected 2 kw-defaults, got %d", len(fd.Args.KwDefaults))
	}
	if fd.Args.KwDefaults[0] != nil {
		t.Error("expected nil default for 'a'")
	}
	if fd.Args.KwDefaults[1] == nil {
		t.Fatal("expected default for 'b'")
	}
	d := asConstant(t, fd.Args.KwDefaults[1])
	if d.Value != "2" {
		t.Errorf("default = %q, want 2", d.Value)
	}
}

func TestFuncDef_ComplexSignature(t *testing.T) {
	// All five kinds of parameters.
	mod := parse(t, "def foo(a, b, /, c, d=4, *args, e, f=6, **kwargs): pass\n")
	requireStmts(t, mod, 1)
	fd := asFuncDef(t, mod.Body[0])

	if len(fd.Args.PosOnlyArgs) != 2 {
		t.Errorf("pos-only: got %d, want 2", len(fd.Args.PosOnlyArgs))
	}
	if len(fd.Args.Args) != 2 {
		t.Errorf("args: got %d, want 2", len(fd.Args.Args))
	}
	if fd.Args.VarArg == nil || fd.Args.VarArg.Name != "args" {
		t.Error("expected *args")
	}
	if len(fd.Args.KwOnlyArgs) != 2 {
		t.Errorf("kw-only: got %d, want 2", len(fd.Args.KwOnlyArgs))
	}
	if fd.Args.VarKwArg == nil || fd.Args.VarKwArg.Name != "kwargs" {
		t.Error("expected **kwargs")
	}
	// Defaults: d=4 is a regular default.
	if len(fd.Args.Defaults) != 1 {
		t.Errorf("defaults: got %d, want 1", len(fd.Args.Defaults))
	}
	// KwDefaults: e has no default (nil), f has default 6.
	if len(fd.Args.KwDefaults) != 2 {
		t.Errorf("kw-defaults: got %d, want 2", len(fd.Args.KwDefaults))
	}
}

// ---------------------------------------------------------------------------
// Task 8: Class definitions
// ---------------------------------------------------------------------------

func TestClassDef_Simple(t *testing.T) {
	mod := parse(t, "class Foo: pass\n")
	requireStmts(t, mod, 1)
	cd := asClassDef(t, mod.Body[0])
	if cd.Name != "Foo" {
		t.Errorf("name = %q, want Foo", cd.Name)
	}
}

func TestClassDef_WithBases(t *testing.T) {
	mod := parse(t, "class Foo(Bar, Baz): pass\n")
	requireStmts(t, mod, 1)
	cd := asClassDef(t, mod.Body[0])
	if len(cd.Bases) != 2 {
		t.Fatalf("expected 2 bases, got %d", len(cd.Bases))
	}
	b0 := asName(t, cd.Bases[0])
	b1 := asName(t, cd.Bases[1])
	if b0.Name != "Bar" || b1.Name != "Baz" {
		t.Errorf("bases = [%s, %s], want [Bar, Baz]", b0.Name, b1.Name)
	}
}

func TestClassDef_WithMetaclass(t *testing.T) {
	mod := parse(t, "class Foo(metaclass=ABCMeta): pass\n")
	requireStmts(t, mod, 1)
	cd := asClassDef(t, mod.Body[0])
	if len(cd.Keywords) != 1 {
		t.Fatalf("expected 1 keyword, got %d", len(cd.Keywords))
	}
	if cd.Keywords[0].Arg != "metaclass" {
		t.Errorf("keyword arg = %q, want metaclass", cd.Keywords[0].Arg)
	}
	val := asName(t, cd.Keywords[0].Value)
	if val.Name != "ABCMeta" {
		t.Errorf("keyword value = %q, want ABCMeta", val.Name)
	}
}

func TestClassDef_WithMethods(t *testing.T) {
	src := `class Foo:
    def bar(self): pass
    def baz(self): pass
`
	mod := parse(t, src)
	requireStmts(t, mod, 1)
	cd := asClassDef(t, mod.Body[0])
	if len(cd.Body) != 2 {
		t.Fatalf("expected 2 body statements, got %d", len(cd.Body))
	}
	m0 := asFuncDef(t, cd.Body[0])
	m1 := asFuncDef(t, cd.Body[1])
	if m0.Name != "bar" || m1.Name != "baz" {
		t.Errorf("methods = [%s, %s], want [bar, baz]", m0.Name, m1.Name)
	}
}

func TestClassDef_Decorated(t *testing.T) {
	src := `@dataclass
class Foo:
    x: int
`
	mod := parse(t, src)
	requireStmts(t, mod, 1)
	cd := asClassDef(t, mod.Body[0])
	if len(cd.Decorators) != 1 {
		t.Fatalf("expected 1 decorator, got %d", len(cd.Decorators))
	}
	dec := asName(t, cd.Decorators[0])
	if dec.Name != "dataclass" {
		t.Errorf("decorator = %q, want dataclass", dec.Name)
	}
}

func TestClassDef_Docstring(t *testing.T) {
	src := `class Foo:
    """Foo docstring."""
    pass
`
	mod := parse(t, src)
	requireStmts(t, mod, 1)
	cd := asClassDef(t, mod.Body[0])
	if len(cd.Body) < 1 {
		t.Fatal("expected body statements")
	}
	es := asExprStmt(t, cd.Body[0])
	c := asConstant(t, es.Value)
	if c.Kind != "str" {
		t.Errorf("kind = %q, want str", c.Kind)
	}
	if c.Value != "Foo docstring." {
		t.Errorf("value = %q", c.Value)
	}
}

func TestClassDef_EllipsisBody(t *testing.T) {
	mod := parse(t, "class Foo: ...\n")
	requireStmts(t, mod, 1)
	cd := asClassDef(t, mod.Body[0])
	if len(cd.Body) != 1 {
		t.Fatalf("expected 1 body statement, got %d", len(cd.Body))
	}
	es := asExprStmt(t, cd.Body[0])
	c := asConstant(t, es.Value)
	if c.Kind != "ellipsis" {
		t.Errorf("kind = %q, want ellipsis", c.Kind)
	}
}

// ---------------------------------------------------------------------------
// Task 9: Imports, assignments, type aliases
// ---------------------------------------------------------------------------

func TestImport_Simple(t *testing.T) {
	mod := parse(t, "import os\n")
	requireStmts(t, mod, 1)
	imp := asImport(t, mod.Body[0])
	if len(imp.Names) != 1 {
		t.Fatalf("expected 1 name, got %d", len(imp.Names))
	}
	if imp.Names[0].Name != "os" {
		t.Errorf("name = %q, want os", imp.Names[0].Name)
	}
}

func TestImport_Alias(t *testing.T) {
	mod := parse(t, "import numpy as np\n")
	requireStmts(t, mod, 1)
	imp := asImport(t, mod.Body[0])
	if imp.Names[0].Alias != "np" {
		t.Errorf("alias = %q, want np", imp.Names[0].Alias)
	}
}

func TestImportFrom_Simple(t *testing.T) {
	mod := parse(t, "from os.path import join, exists\n")
	requireStmts(t, mod, 1)
	imp := asImportFrom(t, mod.Body[0])
	if imp.Module != "os.path" {
		t.Errorf("module = %q, want os.path", imp.Module)
	}
	if len(imp.Names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(imp.Names))
	}
	if imp.Names[0].Name != "join" || imp.Names[1].Name != "exists" {
		t.Errorf("names = %v", imp.Names)
	}
}

func TestImportFrom_Relative(t *testing.T) {
	mod := parse(t, "from ..utils import helper\n")
	requireStmts(t, mod, 1)
	imp := asImportFrom(t, mod.Body[0])
	if imp.Level != 2 {
		t.Errorf("level = %d, want 2", imp.Level)
	}
	if imp.Module != "utils" {
		t.Errorf("module = %q, want utils", imp.Module)
	}
}

func TestImportFrom_Star(t *testing.T) {
	mod := parse(t, "from os import *\n")
	requireStmts(t, mod, 1)
	imp := asImportFrom(t, mod.Body[0])
	if len(imp.Names) != 1 || imp.Names[0].Name != "*" {
		t.Error("expected star import")
	}
}

func TestAssign_Simple(t *testing.T) {
	mod := parse(t, "x = 42\n")
	requireStmts(t, mod, 1)
	a := asAssign(t, mod.Body[0])
	if len(a.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(a.Targets))
	}
	tgt := asName(t, a.Targets[0])
	if tgt.Name != "x" {
		t.Errorf("target = %q, want x", tgt.Name)
	}
	val := asConstant(t, a.Value)
	if val.Value != "42" || val.Kind != "int" {
		t.Errorf("value = %q (%s), want 42 (int)", val.Value, val.Kind)
	}
}

func TestAnnAssign(t *testing.T) {
	mod := parse(t, "x: int = 42\n")
	requireStmts(t, mod, 1)
	a := asAnnAssign(t, mod.Body[0])
	tgt := asName(t, a.Target)
	if tgt.Name != "x" {
		t.Errorf("target = %q, want x", tgt.Name)
	}
	ann := asName(t, a.Annotation)
	if ann.Name != "int" {
		t.Errorf("annotation = %q, want int", ann.Name)
	}
	if !a.Simple {
		t.Error("expected Simple=true")
	}
}

func TestAssign_AllList(t *testing.T) {
	mod := parse(t, `__all__ = ["foo", "bar"]`+"\n")
	requireStmts(t, mod, 1)
	a := asAssign(t, mod.Body[0])
	tgt := asName(t, a.Targets[0])
	if tgt.Name != "__all__" {
		t.Errorf("target = %q, want __all__", tgt.Name)
	}
	lst, ok := a.Value.(*ast.List)
	if !ok {
		t.Fatalf("expected List, got %T", a.Value)
	}
	if len(lst.Elts) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(lst.Elts))
	}
	e0 := asConstant(t, lst.Elts[0])
	e1 := asConstant(t, lst.Elts[1])
	if e0.Value != "foo" || e1.Value != "bar" {
		t.Errorf("elements = [%q, %q], want [foo, bar]", e0.Value, e1.Value)
	}
}

func TestTypeAlias(t *testing.T) {
	mod := parse(t, "type Vector = list[float]\n")
	requireStmts(t, mod, 1)
	ta := asTypeAlias(t, mod.Body[0])
	if ta.Name != "Vector" {
		t.Errorf("name = %q, want Vector", ta.Name)
	}
	// Value should be a Subscript (list[float]).
	sub, ok := ta.Value.(*ast.Subscript)
	if !ok {
		t.Fatalf("expected Subscript, got %T", ta.Value)
	}
	base := asName(t, sub.Value)
	if base.Name != "list" {
		t.Errorf("base = %q, want list", base.Name)
	}
}

// ---------------------------------------------------------------------------
// Task 10: Complex type annotations
// ---------------------------------------------------------------------------

func TestAnnotation_Generic(t *testing.T) {
	mod := parse(t, "x: dict[str, int]\n")
	requireStmts(t, mod, 1)
	a := asAnnAssign(t, mod.Body[0])
	sub, ok := a.Annotation.(*ast.Subscript)
	if !ok {
		t.Fatalf("expected Subscript, got %T", a.Annotation)
	}
	base := asName(t, sub.Value)
	if base.Name != "dict" {
		t.Errorf("base = %q, want dict", base.Name)
	}
	// Slice should be a Tuple with 2 elements.
	tup, ok := sub.Slice.(*ast.Tuple)
	if !ok {
		t.Fatalf("expected Tuple slice, got %T", sub.Slice)
	}
	if len(tup.Elts) != 2 {
		t.Fatalf("expected 2 slice elements, got %d", len(tup.Elts))
	}
}

func TestAnnotation_Union(t *testing.T) {
	mod := parse(t, "x: int | str\n")
	requireStmts(t, mod, 1)
	a := asAnnAssign(t, mod.Body[0])
	binop, ok := a.Annotation.(*ast.BinOp)
	if !ok {
		t.Fatalf("expected BinOp, got %T", a.Annotation)
	}
	if binop.Op != token.PIPE {
		t.Errorf("op = %s, want PIPE", binop.Op)
	}
}

func TestAnnotation_Dotted(t *testing.T) {
	mod := parse(t, "x: os.PathLike\n")
	requireStmts(t, mod, 1)
	a := asAnnAssign(t, mod.Body[0])
	attr, ok := a.Annotation.(*ast.Attribute)
	if !ok {
		t.Fatalf("expected Attribute, got %T", a.Annotation)
	}
	if attr.Attr != "PathLike" {
		t.Errorf("attr = %q, want PathLike", attr.Attr)
	}
}

func TestAnnotation_NestedGeneric(t *testing.T) {
	mod := parse(t, "x: Callable[[int, str], Awaitable[dict[str, list[int]]]]\n")
	requireStmts(t, mod, 1)
	a := asAnnAssign(t, mod.Body[0])

	// Top-level should be a Subscript: Callable[...]
	sub, ok := a.Annotation.(*ast.Subscript)
	if !ok {
		t.Fatalf("expected Subscript, got %T", a.Annotation)
	}
	base := asName(t, sub.Value)
	if base.Name != "Callable" {
		t.Errorf("base = %q, want Callable", base.Name)
	}
}

func TestAnnotation_OptionalWithDefault(t *testing.T) {
	mod := parse(t, "def foo(x: Optional[str] = None): pass\n")
	requireStmts(t, mod, 1)
	fd := asFuncDef(t, mod.Body[0])
	if len(fd.Args.Args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(fd.Args.Args))
	}
	a := fd.Args.Args[0]
	if a.Name != "x" {
		t.Errorf("name = %q, want x", a.Name)
	}
	// Annotation should be Optional[str] — a Subscript.
	sub, ok := a.Annotation.(*ast.Subscript)
	if !ok {
		t.Fatalf("expected Subscript annotation, got %T", a.Annotation)
	}
	base := asName(t, sub.Value)
	if base.Name != "Optional" {
		t.Errorf("annotation base = %q, want Optional", base.Name)
	}
	// Default should be None.
	if len(fd.Args.Defaults) != 1 {
		t.Fatalf("expected 1 default, got %d", len(fd.Args.Defaults))
	}
	def := asConstant(t, fd.Args.Defaults[0])
	if def.Kind != "none" {
		t.Errorf("default kind = %q, want none", def.Kind)
	}
}

// ---------------------------------------------------------------------------
// Task 11: Error recovery
// ---------------------------------------------------------------------------

func TestErrorRecovery_MalformedFuncThenValid(t *testing.T) {
	src := `def broken(
def foo(): pass
`
	mod, errs := parseWithErrors(t, src)
	if len(errs) == 0 {
		t.Fatal("expected parse errors for malformed function")
	}

	// The valid function should still be parsed.
	var found bool
	for _, stmt := range mod.Body {
		if fd, ok := stmt.(*ast.FunctionDef); ok && fd.Name == "foo" {
			found = true
			break
		}
	}
	if !found {
		t.Error("valid function 'foo' was not parsed after malformed function")
	}
}

func TestErrorRecovery_MalformedClassThenValid(t *testing.T) {
	src := `class Broken(
class Good: pass
`
	mod, errs := parseWithErrors(t, src)
	if len(errs) == 0 {
		t.Fatal("expected parse errors for malformed class")
	}

	var found bool
	for _, stmt := range mod.Body {
		if cd, ok := stmt.(*ast.ClassDef); ok && cd.Name == "Good" {
			found = true
			break
		}
	}
	if !found {
		t.Error("valid class 'Good' was not parsed after malformed class")
	}
}

func TestErrorRecovery_NoErrorsOnValidCode(t *testing.T) {
	src := `import os
from pathlib import Path

class Foo:
    """Docstring."""

    def __init__(self, x: int = 0):
        self.x = x

    def method(self) -> str:
        return "hello"

def bar(a, b, /, *, c, d=4, **kwargs) -> bool:
    pass
`
	_, errs := parseWithErrors(t, src)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Logf("unexpected error at %d:%d: %s", e.Pos.Line, e.Pos.Col, e.Msg)
		}
		t.Fatalf("expected no errors, got %d", len(errs))
	}
}

// ---------------------------------------------------------------------------
// Additional tests for completeness
// ---------------------------------------------------------------------------

func TestIfStmt(t *testing.T) {
	src := `if True:
    x = 1
elif False:
    x = 2
else:
    x = 3
`
	mod := parse(t, src)
	requireStmts(t, mod, 1)
	ifStmt, ok := mod.Body[0].(*ast.If)
	if !ok {
		t.Fatalf("expected If, got %T", mod.Body[0])
	}
	if len(ifStmt.Body) != 1 {
		t.Errorf("if body: expected 1 stmt, got %d", len(ifStmt.Body))
	}
	if len(ifStmt.Orelse) != 1 {
		t.Fatalf("expected 1 elif chain, got %d", len(ifStmt.Orelse))
	}
	elif, ok := ifStmt.Orelse[0].(*ast.If)
	if !ok {
		t.Fatalf("expected nested If for elif, got %T", ifStmt.Orelse[0])
	}
	if len(elif.Orelse) != 1 {
		t.Errorf("else body: expected 1 stmt, got %d", len(elif.Orelse))
	}
}

func TestPassThrough_For(t *testing.T) {
	src := `for x in range(10):
    def inner(): pass
`
	mod := parse(t, src)
	requireStmts(t, mod, 1)
	pt, ok := mod.Body[0].(*ast.PassThrough)
	if !ok {
		t.Fatalf("expected PassThrough, got %T", mod.Body[0])
	}
	if pt.Kind != "for" {
		t.Errorf("kind = %q, want for", pt.Kind)
	}
	// The nested function should be in the body.
	var found bool
	for _, stmt := range pt.Body {
		if fd, ok := stmt.(*ast.FunctionDef); ok && fd.Name == "inner" {
			found = true
		}
	}
	if !found {
		t.Error("nested function 'inner' not found in for body")
	}
}

func TestPassThrough_Try(t *testing.T) {
	src := `try:
    def in_try(): pass
except:
    def in_except(): pass
finally:
    def in_finally(): pass
`
	mod := parse(t, src)
	requireStmts(t, mod, 1)
	pt, ok := mod.Body[0].(*ast.PassThrough)
	if !ok {
		t.Fatalf("expected PassThrough, got %T", mod.Body[0])
	}
	if pt.Kind != "try" {
		t.Errorf("kind = %q, want try", pt.Kind)
	}

	// All three nested defs should be in the body.
	names := map[string]bool{}
	for _, stmt := range pt.Body {
		if fd, ok := stmt.(*ast.FunctionDef); ok {
			names[fd.Name] = true
		}
	}
	for _, want := range []string{"in_try", "in_except", "in_finally"} {
		if !names[want] {
			t.Errorf("nested function %q not found in try body", want)
		}
	}
}

func TestConstant_Number(t *testing.T) {
	tests := []struct {
		src  string
		kind string
		val  string
	}{
		{"x = 42\n", "int", "42"},
		{"x = 3.14\n", "float", "3.14"},
		{"x = 1e10\n", "float", "1e10"},
		{"x = 0xFF\n", "int", "0xFF"},
	}

	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			mod := parse(t, tt.src)
			a := asAssign(t, mod.Body[0])
			c := asConstant(t, a.Value)
			if c.Kind != tt.kind {
				t.Errorf("kind = %q, want %q", c.Kind, tt.kind)
			}
			if c.Value != tt.val {
				t.Errorf("value = %q, want %q", c.Value, tt.val)
			}
		})
	}
}

func TestConstant_Booleans(t *testing.T) {
	mod := parse(t, "x = True\ny = False\nz = None\n")
	requireStmts(t, mod, 3)

	a0 := asAssign(t, mod.Body[0])
	c0 := asConstant(t, a0.Value)
	if c0.Kind != "bool" || c0.Value != "True" {
		t.Errorf("True: kind=%q value=%q", c0.Kind, c0.Value)
	}

	a1 := asAssign(t, mod.Body[1])
	c1 := asConstant(t, a1.Value)
	if c1.Kind != "bool" || c1.Value != "False" {
		t.Errorf("False: kind=%q value=%q", c1.Kind, c1.Value)
	}

	a2 := asAssign(t, mod.Body[2])
	c2 := asConstant(t, a2.Value)
	if c2.Kind != "none" || c2.Value != "None" {
		t.Errorf("None: kind=%q value=%q", c2.Kind, c2.Value)
	}
}

func TestMultipleDecorators(t *testing.T) {
	src := `@app.route("/")
@requires_auth
def index(): pass
`
	mod := parse(t, src)
	requireStmts(t, mod, 1)
	fd := asFuncDef(t, mod.Body[0])
	if len(fd.Decorators) != 2 {
		t.Fatalf("expected 2 decorators, got %d", len(fd.Decorators))
	}
}

func TestTypeAlias_WithTypeParams(t *testing.T) {
	mod := parse(t, "type Pair[T, U] = tuple[T, U]\n")
	requireStmts(t, mod, 1)
	ta := asTypeAlias(t, mod.Body[0])
	if ta.Name != "Pair" {
		t.Errorf("name = %q, want Pair", ta.Name)
	}
	if len(ta.TypeParams) != 2 {
		t.Fatalf("expected 2 type params, got %d", len(ta.TypeParams))
	}
	if ta.TypeParams[0].Name != "T" || ta.TypeParams[1].Name != "U" {
		t.Errorf("type params = [%s, %s], want [T, U]", ta.TypeParams[0].Name, ta.TypeParams[1].Name)
	}
}

func TestImportFrom_Parenthesized(t *testing.T) {
	src := "from os.path import (\n    join,\n    exists,\n)\n"
	mod := parse(t, src)
	requireStmts(t, mod, 1)
	imp := asImportFrom(t, mod.Body[0])
	if len(imp.Names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(imp.Names))
	}
}

func TestImportFrom_WithAlias(t *testing.T) {
	mod := parse(t, "from collections import OrderedDict as OD\n")
	requireStmts(t, mod, 1)
	imp := asImportFrom(t, mod.Body[0])
	if imp.Names[0].Name != "OrderedDict" || imp.Names[0].Alias != "OD" {
		t.Errorf("import alias: name=%q alias=%q", imp.Names[0].Name, imp.Names[0].Alias)
	}
}

func TestDictExpr(t *testing.T) {
	mod := parse(t, `x = {"a": 1, "b": 2}`+"\n")
	requireStmts(t, mod, 1)
	a := asAssign(t, mod.Body[0])
	d, ok := a.Value.(*ast.Dict)
	if !ok {
		t.Fatalf("expected Dict, got %T", a.Value)
	}
	if len(d.Keys) != 2 || len(d.Values) != 2 {
		t.Errorf("dict: %d keys, %d values", len(d.Keys), len(d.Values))
	}
}

func TestSetExpr(t *testing.T) {
	mod := parse(t, "x = {1, 2, 3}\n")
	requireStmts(t, mod, 1)
	a := asAssign(t, mod.Body[0])
	s, ok := a.Value.(*ast.Set)
	if !ok {
		t.Fatalf("expected Set, got %T", a.Value)
	}
	if len(s.Elts) != 3 {
		t.Errorf("set: %d elements, want 3", len(s.Elts))
	}
}

func TestTupleExpr(t *testing.T) {
	mod := parse(t, "x = (1, 2, 3)\n")
	requireStmts(t, mod, 1)
	a := asAssign(t, mod.Body[0])
	tup, ok := a.Value.(*ast.Tuple)
	if !ok {
		t.Fatalf("expected Tuple, got %T", a.Value)
	}
	if len(tup.Elts) != 3 {
		t.Errorf("tuple: %d elements, want 3", len(tup.Elts))
	}
}

func TestCallExpr_WithKeyword(t *testing.T) {
	mod := parse(t, "foo(1, key=val)\n")
	requireStmts(t, mod, 1)
	es := asExprStmt(t, mod.Body[0])
	call, ok := es.Value.(*ast.Call)
	if !ok {
		t.Fatalf("expected Call, got %T", es.Value)
	}
	if len(call.Args) != 1 {
		t.Errorf("args: %d, want 1", len(call.Args))
	}
	if len(call.Keywords) != 1 {
		t.Fatalf("keywords: %d, want 1", len(call.Keywords))
	}
	if call.Keywords[0].Arg != "key" {
		t.Errorf("keyword arg = %q, want key", call.Keywords[0].Arg)
	}
}

func TestClassDef_WithBasesAndKeyword(t *testing.T) {
	mod := parse(t, "class Foo(Bar, metaclass=ABCMeta): pass\n")
	requireStmts(t, mod, 1)
	cd := asClassDef(t, mod.Body[0])
	if len(cd.Bases) != 1 {
		t.Errorf("bases: %d, want 1", len(cd.Bases))
	}
	if len(cd.Keywords) != 1 {
		t.Errorf("keywords: %d, want 1", len(cd.Keywords))
	}
}

func TestFuncDef_InBlock(t *testing.T) {
	src := `class Foo:
    def __init__(self, x: int, y: str = "hello") -> None:
        """Initialize Foo."""
        self.x = x
        self.y = y
`
	mod := parse(t, src)
	requireStmts(t, mod, 1)
	cd := asClassDef(t, mod.Body[0])
	if len(cd.Body) != 1 {
		t.Fatalf("expected 1 method, got %d", len(cd.Body))
	}
	fd := asFuncDef(t, cd.Body[0])
	if fd.Name != "__init__" {
		t.Errorf("name = %q, want __init__", fd.Name)
	}
	if len(fd.Args.Args) != 3 { // self, x, y
		t.Errorf("args: %d, want 3", len(fd.Args.Args))
	}
}

func TestAnnotation_ThreeWayUnion(t *testing.T) {
	mod := parse(t, "x: int | str | None\n")
	requireStmts(t, mod, 1)
	a := asAnnAssign(t, mod.Body[0])
	// Should be BinOp(BinOp(int, |, str), |, None)
	outer, ok := a.Annotation.(*ast.BinOp)
	if !ok {
		t.Fatalf("expected BinOp, got %T", a.Annotation)
	}
	if outer.Op != token.PIPE {
		t.Errorf("outer op = %s, want PIPE", outer.Op)
	}
	inner, ok := outer.Left.(*ast.BinOp)
	if !ok {
		t.Fatalf("expected inner BinOp, got %T", outer.Left)
	}
	if inner.Op != token.PIPE {
		t.Errorf("inner op = %s, want PIPE", inner.Op)
	}
}

func TestAsyncFor(t *testing.T) {
	src := "async for x in items:\n    def inner(): pass\n"
	mod := parse(t, src)
	requireStmts(t, mod, 1)
	pt, ok := mod.Body[0].(*ast.PassThrough)
	if !ok {
		t.Fatalf("expected PassThrough, got %T", mod.Body[0])
	}
	if pt.Kind != "async for" {
		t.Errorf("kind = %q, want %q", pt.Kind, "async for")
	}
}

func TestAsyncWith(t *testing.T) {
	src := "async with ctx() as c:\n    def inner(): pass\n"
	mod := parse(t, src)
	requireStmts(t, mod, 1)
	pt, ok := mod.Body[0].(*ast.PassThrough)
	if !ok {
		t.Fatalf("expected PassThrough, got %T", mod.Body[0])
	}
	if pt.Kind != "async with" {
		t.Errorf("kind = %q, want %q", pt.Kind, "async with")
	}
}

func TestAugmentedAssignment(t *testing.T) {
	src := "__all__ += [\"extra\"]\n"
	mod := parse(t, src)
	requireStmts(t, mod, 1)
	assign := asAssign(t, mod.Body[0])

	if len(assign.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(assign.Targets))
	}
	name, ok := assign.Targets[0].(*ast.Name)
	if !ok {
		t.Fatalf("expected Name target, got %T", assign.Targets[0])
	}
	if name.Name != "__all__" {
		t.Errorf("target name = %q, want %q", name.Name, "__all__")
	}
	if assign.Value == nil {
		t.Fatal("expected non-nil value")
	}
	_, ok = assign.Value.(*ast.List)
	if !ok {
		t.Fatalf("expected List value, got %T", assign.Value)
	}
}

func TestListComprehension(t *testing.T) {
	src := "x = [i for i in range(10)]\n"
	mod := parse(t, src)
	requireStmts(t, mod, 1)
	a := asAssign(t, mod.Body[0])
	_, ok := a.Value.(*ast.List)
	if !ok {
		t.Fatalf("expected List, got %T", a.Value)
	}
}

func TestDictComprehension(t *testing.T) {
	src := "x = {k: v for k, v in items.items()}\n"
	mod := parse(t, src)
	requireStmts(t, mod, 1)
	a := asAssign(t, mod.Body[0])
	d, ok := a.Value.(*ast.Dict)
	if !ok {
		t.Fatalf("expected Dict, got %T", a.Value)
	}
	if len(d.Keys) != 1 {
		t.Errorf("keys len = %d, want 1", len(d.Keys))
	}
}

func TestSetComprehension(t *testing.T) {
	src := "x = {i for i in items}\n"
	mod := parse(t, src)
	requireStmts(t, mod, 1)
	a := asAssign(t, mod.Body[0])
	_, ok := a.Value.(*ast.Set)
	if !ok {
		t.Fatalf("expected Set, got %T", a.Value)
	}
}

func TestGeneratorExpression(t *testing.T) {
	src := "x = tuple(i for i in range(10))\n"
	mod := parse(t, src)
	requireStmts(t, mod, 1)
}

func TestNestedComprehension(t *testing.T) {
	src := "x = [i for i in [j for j in range(10)]]\n"
	mod := parse(t, src)
	requireStmts(t, mod, 1)
	a := asAssign(t, mod.Body[0])
	_, ok := a.Value.(*ast.List)
	if !ok {
		t.Fatalf("expected List, got %T", a.Value)
	}
}

func TestComprehensionInDefaultValue(t *testing.T) {
	src := "def foo(x=[i for i in range(5)]):\n    pass\n"
	mod := parse(t, src)
	requireStmts(t, mod, 1)
	fd := asFuncDef(t, mod.Body[0])
	if fd.Name != "foo" {
		t.Errorf("name = %q, want foo", fd.Name)
	}
}
