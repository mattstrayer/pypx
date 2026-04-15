package ast

import (
	"testing"

	"github.com/pypx/goopy/token"
)

func TestNodeInterface(t *testing.T) {
	// Test that Module implements Node
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	endPos := token.Pos{Line: 5, Col: 0, Offset: 50}
	mod := &Module{
		Position: pos,
		EndPos:   endPos,
	}

	if mod.Pos() != pos {
		t.Errorf("Module.Pos() = %v, want %v", mod.Pos(), pos)
	}

	if mod.End() != endPos {
		t.Errorf("Module.End() = %v, want %v", mod.End(), endPos)
	}
}

func TestFunctionDef(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	endPos := token.Pos{Line: 5, Col: 0, Offset: 50}
	fn := &FunctionDef{
		Position: pos,
		EndPos:   endPos,
		Name:     "example",
		Args: &Arguments{
			PosOnlyArgs: []*Arg{},
			Args:        []*Arg{},
			KwOnlyArgs:  []*Arg{},
		},
	}

	if fn.Pos() != pos {
		t.Errorf("FunctionDef.Pos() = %v, want %v", fn.Pos(), pos)
	}

	if fn.End() != endPos {
		t.Errorf("FunctionDef.End() = %v, want %v", fn.End(), endPos)
	}

	if fn.Name != "example" {
		t.Errorf("FunctionDef.Name = %q, want %q", fn.Name, "example")
	}

	// Verify stmtNode marker
	fn.stmtNode()
}

func TestFunctionDefWithReturnsAndAsync(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	endPos := token.Pos{Line: 10, Col: 0, Offset: 100}
	returnType := &Name{
		Position: token.Pos{Line: 1, Col: 20, Offset: 20},
		EndPos:   token.Pos{Line: 1, Col: 24, Offset: 24},
		Name:     "None",
	}
	fn := &FunctionDef{
		Position:   pos,
		EndPos:     endPos,
		Name:       "fetch_data",
		IsAsync:    true,
		Returns:    returnType,
		Decorators: []Expr{},
		TypeParams: []*TypeParam{},
		Args:       &Arguments{},
	}

	if !fn.IsAsync {
		t.Errorf("FunctionDef.IsAsync = false, want true")
	}

	if fn.Returns == nil {
		t.Errorf("FunctionDef.Returns = nil, want non-nil")
	}

	returnName, ok := fn.Returns.(*Name)
	if !ok {
		t.Errorf("FunctionDef.Returns is not *Name")
	} else if returnName.Name != "None" {
		t.Errorf("FunctionDef.Returns name = %q, want %q", returnName.Name, "None")
	}

	if len(fn.Decorators) != 0 {
		t.Errorf("FunctionDef.Decorators len = %d, want 0", len(fn.Decorators))
	}

	fn.stmtNode()
}

func TestClassDef(t *testing.T) {
	pos := token.Pos{Line: 5, Col: 0, Offset: 100}
	endPos := token.Pos{Line: 20, Col: 0, Offset: 300}
	cls := &ClassDef{
		Position:   pos,
		EndPos:     endPos,
		Name:       "MyClass",
		Bases:      []Expr{},
		Keywords:   []*Keyword{},
		Body:       []Stmt{},
		Decorators: []Expr{},
		TypeParams: []*TypeParam{},
	}

	if cls.Pos() != pos {
		t.Errorf("ClassDef.Pos() = %v, want %v", cls.Pos(), pos)
	}

	if cls.End() != endPos {
		t.Errorf("ClassDef.End() = %v, want %v", cls.End(), endPos)
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
	endPos := token.Pos{Line: 30, Col: 0, Offset: 200}
	mod := &Module{
		Position: pos,
		EndPos:   endPos,
		Body: []Stmt{
			&PassThrough{
				Kind:     "pass",
				Position: token.Pos{Line: 2, Col: 0, Offset: 10},
				EndPos:   token.Pos{Line: 2, Col: 4, Offset: 14},
			},
			&PassThrough{
				Kind:     "pass",
				Position: token.Pos{Line: 3, Col: 0, Offset: 20},
				EndPos:   token.Pos{Line: 3, Col: 4, Offset: 24},
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

func TestPassThrough(t *testing.T) {
	pos := token.Pos{Line: 5, Col: 0, Offset: 50}
	endPos := token.Pos{Line: 8, Col: 0, Offset: 90}

	nested := &FunctionDef{
		Position: token.Pos{Line: 6, Col: 4, Offset: 60},
		EndPos:   token.Pos{Line: 7, Col: 0, Offset: 80},
		Name:     "inner",
		Args:     &Arguments{},
	}

	pt := &PassThrough{
		Kind:     "for",
		Position: pos,
		EndPos:   endPos,
		Body:     []Stmt{nested},
	}

	if pt.Kind != "for" {
		t.Errorf("PassThrough.Kind = %q, want %q", pt.Kind, "for")
	}

	if pt.Pos() != pos {
		t.Errorf("PassThrough.Pos() = %v, want %v", pt.Pos(), pos)
	}

	if pt.End() != endPos {
		t.Errorf("PassThrough.End() = %v, want %v", pt.End(), endPos)
	}

	if len(pt.Body) != 1 {
		t.Errorf("PassThrough.Body len = %d, want 1", len(pt.Body))
	}

	pt.stmtNode()
}

func TestName(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 5, Offset: 5}
	endPos := token.Pos{Line: 1, Col: 8, Offset: 8}
	name := &Name{
		Position: pos,
		EndPos:   endPos,
		Name:     "foo",
	}

	if name.Pos() != pos {
		t.Errorf("Name.Pos() = %v, want %v", name.Pos(), pos)
	}

	if name.End() != endPos {
		t.Errorf("Name.End() = %v, want %v", name.End(), endPos)
	}

	if name.Name != "foo" {
		t.Errorf("Name.Name = %q, want %q", name.Name, "foo")
	}

	// Verify exprNode marker
	name.exprNode()
}

func TestAttribute(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	endPos := token.Pos{Line: 1, Col: 10, Offset: 10}
	attr := &Attribute{
		Position: pos,
		EndPos:   endPos,
		Value: &Name{
			Position: pos,
			EndPos:   token.Pos{Line: 1, Col: 3, Offset: 3},
			Name:     "obj",
		},
		Attr: "method",
	}

	if attr.Pos() != pos {
		t.Errorf("Attribute.Pos() = %v, want %v", attr.Pos(), pos)
	}

	if attr.End() != endPos {
		t.Errorf("Attribute.End() = %v, want %v", attr.End(), endPos)
	}

	if attr.Attr != "method" {
		t.Errorf("Attribute.Attr = %q, want %q", attr.Attr, "method")
	}

	// Verify exprNode marker
	attr.exprNode()
}

func TestSubscript(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	endPos := token.Pos{Line: 1, Col: 7, Offset: 7}
	subscript := &Subscript{
		Position: pos,
		EndPos:   endPos,
		Value: &Name{
			Position: pos,
			EndPos:   token.Pos{Line: 1, Col: 4, Offset: 4},
			Name:     "list",
		},
		Slice: &Constant{
			Position: token.Pos{Line: 1, Col: 5, Offset: 5},
			EndPos:   token.Pos{Line: 1, Col: 6, Offset: 6},
			Value:    "0",
			Kind:     "int",
			Lit:      "0",
		},
	}

	if subscript.Pos() != pos {
		t.Errorf("Subscript.Pos() = %v, want %v", subscript.Pos(), pos)
	}

	if subscript.End() != endPos {
		t.Errorf("Subscript.End() = %v, want %v", subscript.End(), endPos)
	}

	// Verify exprNode marker
	subscript.exprNode()
}

func TestBinOp(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	endPos := token.Pos{Line: 1, Col: 5, Offset: 5}
	binop := &BinOp{
		Position: pos,
		EndPos:   endPos,
		Left: &Constant{
			Position: pos,
			EndPos:   token.Pos{Line: 1, Col: 1, Offset: 1},
			Value:    "1",
			Kind:     "int",
			Lit:      "1",
		},
		Op: token.PLUS,
		Right: &Constant{
			Position: token.Pos{Line: 1, Col: 4, Offset: 4},
			EndPos:   token.Pos{Line: 1, Col: 5, Offset: 5},
			Value:    "2",
			Kind:     "int",
			Lit:      "2",
		},
	}

	if binop.Pos() != pos {
		t.Errorf("BinOp.Pos() = %v, want %v", binop.Pos(), pos)
	}

	if binop.End() != endPos {
		t.Errorf("BinOp.End() = %v, want %v", binop.End(), endPos)
	}

	if binop.Op != token.PLUS {
		t.Errorf("BinOp.Op = %v, want %v", binop.Op, token.PLUS)
	}

	// Verify exprNode marker
	binop.exprNode()
}

func TestUnaryOp(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	endPos := token.Pos{Line: 1, Col: 5, Offset: 5}
	unary := &UnaryOp{
		Position: pos,
		EndPos:   endPos,
		Op:       token.NOT,
		Operand: &Name{
			Position: pos,
			EndPos:   endPos,
			Name:     "x",
		},
	}

	if unary.Pos() != pos {
		t.Errorf("UnaryOp.Pos() = %v, want %v", unary.Pos(), pos)
	}

	if unary.End() != endPos {
		t.Errorf("UnaryOp.End() = %v, want %v", unary.End(), endPos)
	}

	if unary.Op != token.NOT {
		t.Errorf("UnaryOp.Op = %v, want %v", unary.Op, token.NOT)
	}

	// Verify exprNode marker
	unary.exprNode()
}

func TestTuple(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	endPos := token.Pos{Line: 1, Col: 4, Offset: 4}
	tuple := &Tuple{
		Position: pos,
		EndPos:   endPos,
		Elts: []Expr{
			&Constant{Position: pos, EndPos: endPos, Value: "1", Kind: "int", Lit: "1"},
			&Constant{Position: pos, EndPos: endPos, Value: "2", Kind: "int", Lit: "2"},
		},
	}

	if tuple.Pos() != pos {
		t.Errorf("Tuple.Pos() = %v, want %v", tuple.Pos(), pos)
	}

	if tuple.End() != endPos {
		t.Errorf("Tuple.End() = %v, want %v", tuple.End(), endPos)
	}

	if len(tuple.Elts) != 2 {
		t.Errorf("Tuple.Elts len = %d, want 2", len(tuple.Elts))
	}

	// Verify exprNode marker
	tuple.exprNode()
}

func TestList(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	endPos := token.Pos{Line: 1, Col: 5, Offset: 5}
	list := &List{
		Position: pos,
		EndPos:   endPos,
		Elts: []Expr{
			&Constant{Position: pos, EndPos: endPos, Value: "a", Kind: "str", Lit: `"a"`},
			&Constant{Position: pos, EndPos: endPos, Value: "b", Kind: "str", Lit: `"b"`},
		},
	}

	if list.Pos() != pos {
		t.Errorf("List.Pos() = %v, want %v", list.Pos(), pos)
	}

	if list.End() != endPos {
		t.Errorf("List.End() = %v, want %v", list.End(), endPos)
	}

	if len(list.Elts) != 2 {
		t.Errorf("List.Elts len = %d, want 2", len(list.Elts))
	}

	// Verify exprNode marker
	list.exprNode()
}

func TestDict(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	endPos := token.Pos{Line: 1, Col: 10, Offset: 10}
	dict := &Dict{
		Position: pos,
		EndPos:   endPos,
		Keys: []Expr{
			&Constant{Position: pos, EndPos: endPos, Value: "k1", Kind: "str", Lit: `"k1"`},
		},
		Values: []Expr{
			&Constant{Position: pos, EndPos: endPos, Value: "v1", Kind: "str", Lit: `"v1"`},
		},
	}

	if dict.Pos() != pos {
		t.Errorf("Dict.Pos() = %v, want %v", dict.Pos(), pos)
	}

	if dict.End() != endPos {
		t.Errorf("Dict.End() = %v, want %v", dict.End(), endPos)
	}

	if len(dict.Keys) != 1 || len(dict.Values) != 1 {
		t.Errorf("Dict keys/values count mismatch")
	}

	// Verify exprNode marker
	dict.exprNode()
}

func TestSet(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	endPos := token.Pos{Line: 1, Col: 5, Offset: 5}
	set := &Set{
		Position: pos,
		EndPos:   endPos,
		Elts: []Expr{
			&Constant{Position: pos, EndPos: endPos, Value: "1", Kind: "int", Lit: "1"},
			&Constant{Position: pos, EndPos: endPos, Value: "2", Kind: "int", Lit: "2"},
		},
	}

	if set.Pos() != pos {
		t.Errorf("Set.Pos() = %v, want %v", set.Pos(), pos)
	}

	if set.End() != endPos {
		t.Errorf("Set.End() = %v, want %v", set.End(), endPos)
	}

	if len(set.Elts) != 2 {
		t.Errorf("Set.Elts len = %d, want 2", len(set.Elts))
	}

	// Verify exprNode marker
	set.exprNode()
}

func TestConstant(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 5, Offset: 5}
	endPos := token.Pos{Line: 1, Col: 7, Offset: 7}
	const1 := &Constant{
		Position: pos,
		EndPos:   endPos,
		Value:    "42",
		Kind:     "int",
		Lit:      "42",
	}

	if const1.Pos() != pos {
		t.Errorf("Constant.Pos() = %v, want %v", const1.Pos(), pos)
	}

	if const1.End() != endPos {
		t.Errorf("Constant.End() = %v, want %v", const1.End(), endPos)
	}

	if const1.Value != "42" {
		t.Errorf("Constant.Value = %q, want %q", const1.Value, "42")
	}

	if const1.Kind != "int" {
		t.Errorf("Constant.Kind = %q, want %q", const1.Kind, "int")
	}

	if const1.Lit != "42" {
		t.Errorf("Constant.Lit = %q, want %q", const1.Lit, "42")
	}

	// Verify exprNode marker
	const1.exprNode()
}

func TestCall(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	endPos := token.Pos{Line: 1, Col: 10, Offset: 10}
	call := &Call{
		Position: pos,
		EndPos:   endPos,
		Func: &Name{
			Position: pos,
			EndPos:   token.Pos{Line: 1, Col: 3, Offset: 3},
			Name:     "foo",
		},
		Args: []Expr{
			&Constant{Position: pos, EndPos: endPos, Value: "1", Kind: "int", Lit: "1"},
		},
		Keywords: []*Keyword{
			{
				Arg:   "x",
				Value: &Constant{Position: pos, EndPos: endPos, Value: "2", Kind: "int", Lit: "2"},
			},
		},
	}

	if call.Pos() != pos {
		t.Errorf("Call.Pos() = %v, want %v", call.Pos(), pos)
	}

	if call.End() != endPos {
		t.Errorf("Call.End() = %v, want %v", call.End(), endPos)
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
	endPos := token.Pos{Line: 1, Col: 15, Offset: 15}
	ifexpr := &IfExpr{
		Position: pos,
		EndPos:   endPos,
		Test: &Name{
			Position: pos,
			EndPos:   token.Pos{Line: 1, Col: 1, Offset: 1},
			Name:     "x",
		},
		Body:   &Constant{Position: pos, EndPos: endPos, Value: "1", Kind: "int", Lit: "1"},
		Orelse: &Constant{Position: pos, EndPos: endPos, Value: "2", Kind: "int", Lit: "2"},
	}

	if ifexpr.Pos() != pos {
		t.Errorf("IfExpr.Pos() = %v, want %v", ifexpr.Pos(), pos)
	}

	if ifexpr.End() != endPos {
		t.Errorf("IfExpr.End() = %v, want %v", ifexpr.End(), endPos)
	}

	// Verify exprNode marker
	ifexpr.exprNode()
}

func TestImportAlias(t *testing.T) {
	alias := &ImportAlias{
		Name:  "module",
		Alias: "mod",
	}

	if alias.Name != "module" {
		t.Errorf("ImportAlias.Name = %q, want %q", alias.Name, "module")
	}

	if alias.Alias != "mod" {
		t.Errorf("ImportAlias.Alias = %q, want %q", alias.Alias, "mod")
	}
}

func TestTypeParam(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	endPos := token.Pos{Line: 1, Col: 1, Offset: 1}
	param := &TypeParam{
		Position: pos,
		EndPos:   endPos,
		Name:     "T",
		Bound:    nil,
	}

	if param.Name != "T" {
		t.Errorf("TypeParam.Name = %q, want %q", param.Name, "T")
	}

	if param.Pos() != pos {
		t.Errorf("TypeParam.Pos() = %v, want %v", param.Pos(), pos)
	}

	if param.End() != endPos {
		t.Errorf("TypeParam.End() = %v, want %v", param.End(), endPos)
	}
}

func TestAssign(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	endPos := token.Pos{Line: 1, Col: 6, Offset: 6}
	assign := &Assign{
		Position: pos,
		EndPos:   endPos,
		Targets: []Expr{
			&Name{Position: pos, EndPos: token.Pos{Line: 1, Col: 1, Offset: 1}, Name: "x"},
		},
		Value: &Constant{Position: pos, EndPos: endPos, Value: "42", Kind: "int", Lit: "42"},
	}

	if assign.Pos() != pos {
		t.Errorf("Assign.Pos() = %v, want %v", assign.Pos(), pos)
	}

	if assign.End() != endPos {
		t.Errorf("Assign.End() = %v, want %v", assign.End(), endPos)
	}

	if len(assign.Targets) != 1 {
		t.Errorf("Assign.Targets len = %d, want 1", len(assign.Targets))
	}

	// Verify stmtNode marker
	assign.stmtNode()
}

func TestImport(t *testing.T) {
	pos := token.Pos{Line: 1, Col: 0, Offset: 0}
	endPos := token.Pos{Line: 1, Col: 15, Offset: 15}
	imp := &Import{
		Position: pos,
		EndPos:   endPos,
		Names: []*ImportAlias{
			{Name: "os"},
			{Name: "sys"},
		},
	}

	if imp.Pos() != pos {
		t.Errorf("Import.Pos() = %v, want %v", imp.Pos(), pos)
	}

	if imp.End() != endPos {
		t.Errorf("Import.End() = %v, want %v", imp.End(), endPos)
	}

	if len(imp.Names) != 2 {
		t.Errorf("Import.Names len = %d, want 2", len(imp.Names))
	}

	// Verify stmtNode marker
	imp.stmtNode()
}
