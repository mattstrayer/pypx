package ast

import (
	"testing"

	"github.com/pypx/goopy/token"
)

func TestNodeInterface(t *testing.T) {
	// Test that Module implements Node
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	mod := &Module{
		NamePos: pos,
	}

	if mod.Pos() != pos {
		t.Errorf("Module.Pos() = %v, want %v", mod.Pos(), pos)
	}

	// End should be after all body statements
	if mod.End().Offset < pos.Offset {
		t.Errorf("Module.End().Offset should be >= Pos().Offset")
	}
}

func TestFunctionDef(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	fn := &FunctionDef{
		NamePos: pos,
		Name:    "example",
		Args: &Arguments{
			PosOnlyArgs: []*Arg{},
			Args:        []*Arg{},
			KwOnlyArgs:  []*Arg{},
		},
	}

	if fn.Pos() != pos {
		t.Errorf("FunctionDef.Pos() = %v, want %v", fn.Pos(), pos)
	}

	if fn.Name != "example" {
		t.Errorf("FunctionDef.Name = %q, want %q", fn.Name, "example")
	}

	// Verify stmtNode marker
	fn.stmtNode()
}

func TestClassDef(t *testing.T) {
	pos := token.Pos{Line: 5, Col: 0, Offset: 100}
	cls := &ClassDef{
		NamePos: pos,
		Name:    "MyClass",
		Bases:   []Expr{},
		Body:    []Stmt{},
	}

	if cls.Pos() != pos {
		t.Errorf("ClassDef.Pos() = %v, want %v", cls.Pos(), pos)
	}

	if cls.Name != "MyClass" {
		t.Errorf("ClassDef.Name = %q, want %q", cls.Name, "MyClass")
	}

	// Verify stmtNode marker
	cls.stmtNode()
}

func TestArguments(t *testing.T) {
	args := &Arguments{
		PosOnlyArgs: []*Arg{
			{Name: "a", Annotation: nil},
			{Name: "b", Annotation: nil},
		},
		Args: []*Arg{
			{Name: "c", Annotation: nil},
		},
		VarArg: &Arg{Name: "args"},
		KwOnlyArgs: []*Arg{
			{Name: "x", Annotation: nil},
		},
		VarKwArg: &Arg{Name: "kwargs"},
	}

	if len(args.PosOnlyArgs) != 2 {
		t.Errorf("Arguments.PosOnlyArgs len = %d, want 2", len(args.PosOnlyArgs))
	}

	if len(args.Args) != 1 {
		t.Errorf("Arguments.Args len = %d, want 1", len(args.Args))
	}

	if args.VarArg.Name != "args" {
		t.Errorf("Arguments.VarArg.Name = %q, want %q", args.VarArg.Name, "args")
	}

	if len(args.KwOnlyArgs) != 1 {
		t.Errorf("Arguments.KwOnlyArgs len = %d, want 1", len(args.KwOnlyArgs))
	}

	if args.VarKwArg.Name != "kwargs" {
		t.Errorf("Arguments.VarKwArg.Name = %q, want %q", args.VarKwArg.Name, "kwargs")
	}
}

func TestModule(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	mod := &Module{
		NamePos: pos,
		Body: []Stmt{
			&PassThrough{
				PassPos: token.Pos{Line: 2, Col: 0, Offset: 10},
			},
			&PassThrough{
				PassPos: token.Pos{Line: 3, Col: 0, Offset: 20},
			},
		},
	}

	if len(mod.Body) != 2 {
		t.Errorf("Module.Body len = %d, want 2", len(mod.Body))
	}

	// Verify that different statement types can be in Body
	for _, stmt := range mod.Body {
		stmt.stmtNode() // verify stmtNode marker
	}
}

func TestName(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 5, Offset: 5}
	name := &Name{
		NamePos: pos,
		Name:    "foo",
	}

	if name.Pos() != pos {
		t.Errorf("Name.Pos() = %v, want %v", name.Pos(), pos)
	}

	if name.Name != "foo" {
		t.Errorf("Name.Name = %q, want %q", name.Name, "foo")
	}

	// Verify exprNode marker
	name.exprNode()
}

func TestAttribute(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	attr := &Attribute{
		ValuePos: pos,
		Value: &Name{
			NamePos: pos,
			Name:    "obj",
		},
		Attr: "method",
	}

	if attr.Pos() != pos {
		t.Errorf("Attribute.Pos() = %v, want %v", attr.Pos(), pos)
	}

	if attr.Attr != "method" {
		t.Errorf("Attribute.Attr = %q, want %q", attr.Attr, "method")
	}

	// Verify exprNode marker
	attr.exprNode()
}

func TestSubscript(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	subscript := &Subscript{
		ValuePos: pos,
		Value: &Name{
			NamePos: pos,
			Name:    "list",
		},
		Index: &Constant{
			ConstPos: pos,
			Value:    0,
		},
	}

	if subscript.Pos() != pos {
		t.Errorf("Subscript.Pos() = %v, want %v", subscript.Pos(), pos)
	}

	// Verify exprNode marker
	subscript.exprNode()
}

func TestBinOp(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	binop := &BinOp{
		LeftPos: pos,
		Left: &Constant{
			ConstPos: pos,
			Value:    1,
		},
		Op: token.PLUS,
		Right: &Constant{
			ConstPos: token.Pos{Line: 1, Col: 4, Offset: 4},
			Value:    2,
		},
	}

	if binop.Pos() != pos {
		t.Errorf("BinOp.Pos() = %v, want %v", binop.Pos(), pos)
	}

	if binop.Op != token.PLUS {
		t.Errorf("BinOp.Op = %v, want %v", binop.Op, token.PLUS)
	}

	// Verify exprNode marker
	binop.exprNode()
}

func TestUnaryOp(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	unary := &UnaryOp{
		OpPos: pos,
		Op:    token.NOT,
		Operand: &Name{
			NamePos: pos,
			Name:    "x",
		},
	}

	if unary.Pos() != pos {
		t.Errorf("UnaryOp.Pos() = %v, want %v", unary.Pos(), pos)
	}

	if unary.Op != token.NOT {
		t.Errorf("UnaryOp.Op = %v, want %v", unary.Op, token.NOT)
	}

	// Verify exprNode marker
	unary.exprNode()
}

func TestTuple(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	tuple := &Tuple{
		TuplePos: pos,
		Elts: []Expr{
			&Constant{ConstPos: pos, Value: 1},
			&Constant{ConstPos: pos, Value: 2},
		},
	}

	if tuple.Pos() != pos {
		t.Errorf("Tuple.Pos() = %v, want %v", tuple.Pos(), pos)
	}

	if len(tuple.Elts) != 2 {
		t.Errorf("Tuple.Elts len = %d, want 2", len(tuple.Elts))
	}

	// Verify exprNode marker
	tuple.exprNode()
}

func TestList(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	list := &List{
		ListPos: pos,
		Elts: []Expr{
			&Constant{ConstPos: pos, Value: "a"},
			&Constant{ConstPos: pos, Value: "b"},
		},
	}

	if list.Pos() != pos {
		t.Errorf("List.Pos() = %v, want %v", list.Pos(), pos)
	}

	if len(list.Elts) != 2 {
		t.Errorf("List.Elts len = %d, want 2", len(list.Elts))
	}

	// Verify exprNode marker
	list.exprNode()
}

func TestDict(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	dict := &Dict{
		DictPos: pos,
		Keys: []Expr{
			&Constant{ConstPos: pos, Value: "k1"},
		},
		Values: []Expr{
			&Constant{ConstPos: pos, Value: "v1"},
		},
	}

	if dict.Pos() != pos {
		t.Errorf("Dict.Pos() = %v, want %v", dict.Pos(), pos)
	}

	if len(dict.Keys) != 1 || len(dict.Values) != 1 {
		t.Errorf("Dict keys/values count mismatch")
	}

	// Verify exprNode marker
	dict.exprNode()
}

func TestSet(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	set := &Set{
		SetPos: pos,
		Elts: []Expr{
			&Constant{ConstPos: pos, Value: 1},
			&Constant{ConstPos: pos, Value: 2},
		},
	}

	if set.Pos() != pos {
		t.Errorf("Set.Pos() = %v, want %v", set.Pos(), pos)
	}

	if len(set.Elts) != 2 {
		t.Errorf("Set.Elts len = %d, want 2", len(set.Elts))
	}

	// Verify exprNode marker
	set.exprNode()
}

func TestConstant(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 5, Offset: 5}
	const1 := &Constant{
		ConstPos: pos,
		Value:    42,
	}

	if const1.Pos() != pos {
		t.Errorf("Constant.Pos() = %v, want %v", const1.Pos(), pos)
	}

	if const1.Value != 42 {
		t.Errorf("Constant.Value = %v, want 42", const1.Value)
	}

	// Verify exprNode marker
	const1.exprNode()
}

func TestCall(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	call := &Call{
		FuncPos: pos,
		Func: &Name{
			NamePos: pos,
			Name:    "foo",
		},
		Args: []Expr{
			&Constant{ConstPos: pos, Value: 1},
		},
		Keywords: []*Keyword{
			{
				Arg:   "x",
				Value: &Constant{ConstPos: pos, Value: 2},
			},
		},
	}

	if call.Pos() != pos {
		t.Errorf("Call.Pos() = %v, want %v", call.Pos(), pos)
	}

	if len(call.Args) != 1 {
		t.Errorf("Call.Args len = %d, want 1", len(call.Args))
	}

	if len(call.Keywords) != 1 {
		t.Errorf("Call.Keywords len = %d, want 1", len(call.Keywords))
	}

	// Verify exprNode marker
	call.exprNode()
}

func TestIfExpr(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	ifexpr := &IfExpr{
		IfPos: pos,
		Test: &Name{
			NamePos: pos,
			Name:    "x",
		},
		Body: &Constant{ConstPos: pos, Value: 1},
		Orelse: &Constant{ConstPos: pos, Value: 2},
	}

	if ifexpr.Pos() != pos {
		t.Errorf("IfExpr.Pos() = %v, want %v", ifexpr.Pos(), pos)
	}

	// Verify exprNode marker
	ifexpr.exprNode()
}

func TestImportAlias(t *testing.T) {
	alias := &ImportAlias{
		Name: "module",
		Asname: "mod",
	}

	if alias.Name != "module" {
		t.Errorf("ImportAlias.Name = %q, want %q", alias.Name, "module")
	}

	if alias.Asname != "mod" {
		t.Errorf("ImportAlias.Asname = %q, want %q", alias.Asname, "mod")
	}
}

func TestTypeParam(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	param := &TypeParam{
		ParamPos: pos,
		Name:     "T",
		Bound:    nil,
	}

	if param.Name != "T" {
		t.Errorf("TypeParam.Name = %q, want %q", param.Name, "T")
	}

	if param.Pos() != pos {
		t.Errorf("TypeParam.Pos() = %v, want %v", param.Pos(), pos)
	}
}

func TestAssign(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	assign := &Assign{
		AssignPos: pos,
		Targets: []Expr{
			&Name{NamePos: pos, Name: "x"},
		},
		Value: &Constant{ConstPos: pos, Value: 42},
	}

	if assign.Pos() != pos {
		t.Errorf("Assign.Pos() = %v, want %v", assign.Pos(), pos)
	}

	if len(assign.Targets) != 1 {
		t.Errorf("Assign.Targets len = %d, want 1", len(assign.Targets))
	}

	// Verify stmtNode marker
	assign.stmtNode()
}

func TestImport(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	imp := &Import{
		ImportPos: pos,
		Names: []*ImportAlias{
			{Name: "os"},
			{Name: "sys"},
		},
	}

	if imp.Pos() != pos {
		t.Errorf("Import.Pos() = %v, want %v", imp.Pos(), pos)
	}

	if len(imp.Names) != 2 {
		t.Errorf("Import.Names len = %d, want 2", len(imp.Names))
	}

	// Verify stmtNode marker
	imp.stmtNode()
}
