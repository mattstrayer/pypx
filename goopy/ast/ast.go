package ast

import (
	"github.com/pypx/goopy/token"
)

// Node is the interface that all AST nodes implement.
type Node interface {
	Pos() token.Pos
	End() token.Pos
}

// Stmt is the interface for all statement nodes.
type Stmt interface {
	Node
	stmtNode()
}

// Expr is the interface for all expression nodes.
type Expr interface {
	Node
	exprNode()
}

// =======================
// Statement Nodes
// =======================

// Module is the root node of a Python module.
type Module struct {
	NamePos token.Pos
	Body    []Stmt
}

func (m *Module) Pos() token.Pos {
	return m.NamePos
}

func (m *Module) End() token.Pos {
	if len(m.Body) == 0 {
		return m.NamePos
	}
	return m.Body[len(m.Body)-1].End()
}

func (m *Module) stmtNode() {}

// FunctionDef is a function definition statement.
type FunctionDef struct {
	NamePos token.Pos
	Name    string
	Args    *Arguments
	Body    []Stmt
	// Returns annotation: *TypeExpr
	Decorator []Expr
	// Returns *TypeExpr
}

func (f *FunctionDef) Pos() token.Pos {
	return f.NamePos
}

func (f *FunctionDef) End() token.Pos {
	if len(f.Body) == 0 {
		return f.NamePos
	}
	return f.Body[len(f.Body)-1].End()
}

func (f *FunctionDef) stmtNode() {}

// ClassDef is a class definition statement.
type ClassDef struct {
	NamePos token.Pos
	Name    string
	Bases   []Expr
	Body    []Stmt
	// Decorator list
}

func (c *ClassDef) Pos() token.Pos {
	return c.NamePos
}

func (c *ClassDef) End() token.Pos {
	if len(c.Body) == 0 {
		return c.NamePos
	}
	return c.Body[len(c.Body)-1].End()
}

func (c *ClassDef) stmtNode() {}

// Assign is an assignment statement (x = value).
type Assign struct {
	AssignPos token.Pos
	Targets   []Expr
	Value     Expr
}

func (a *Assign) Pos() token.Pos {
	return a.AssignPos
}

func (a *Assign) End() token.Pos {
	if a.Value != nil {
		return a.Value.End()
	}
	return a.AssignPos
}

func (a *Assign) stmtNode() {}

// AnnAssign is an annotated assignment statement (x: int = value).
type AnnAssign struct {
	AnnPos     token.Pos
	Target     Expr
	Annotation Expr
	Value      Expr
	Simple     bool
}

func (a *AnnAssign) Pos() token.Pos {
	return a.AnnPos
}

func (a *AnnAssign) End() token.Pos {
	if a.Value != nil {
		return a.Value.End()
	}
	return a.Annotation.End()
}

func (a *AnnAssign) stmtNode() {}

// Import is an import statement (import x, y as z).
type Import struct {
	ImportPos token.Pos
	Names     []*ImportAlias
}

func (i *Import) Pos() token.Pos {
	return i.ImportPos
}

func (i *Import) End() token.Pos {
	if len(i.Names) == 0 {
		return i.ImportPos
	}
	// Approximate end based on last name
	return token.Pos{
		Line:   i.ImportPos.Line,
		Col:    i.ImportPos.Col + 100,
		Offset: i.ImportPos.Offset + 100,
	}
}

func (i *Import) stmtNode() {}

// ImportFrom is a from...import statement (from module import x, y).
type ImportFrom struct {
	FromPos   token.Pos
	Module    string
	Names     []*ImportAlias
	Level     int // relative import level (0 = absolute, >0 = relative)
}

func (i *ImportFrom) Pos() token.Pos {
	return i.FromPos
}

func (i *ImportFrom) End() token.Pos {
	if len(i.Names) == 0 {
		return i.FromPos
	}
	return token.Pos{
		Line:   i.FromPos.Line,
		Col:    i.FromPos.Col + 100,
		Offset: i.FromPos.Offset + 100,
	}
}

func (i *ImportFrom) stmtNode() {}

// If is an if statement.
type If struct {
	IfPos  token.Pos
	Test   Expr
	Body   []Stmt
	Orelse []Stmt
}

func (i *If) Pos() token.Pos {
	return i.IfPos
}

func (i *If) End() token.Pos {
	if len(i.Orelse) > 0 {
		return i.Orelse[len(i.Orelse)-1].End()
	}
	if len(i.Body) > 0 {
		return i.Body[len(i.Body)-1].End()
	}
	return i.IfPos
}

func (i *If) stmtNode() {}

// ExprStmt is an expression statement (bare expression).
type ExprStmt struct {
	Value Expr
}

func (e *ExprStmt) Pos() token.Pos {
	return e.Value.Pos()
}

func (e *ExprStmt) End() token.Pos {
	return e.Value.End()
}

func (e *ExprStmt) stmtNode() {}

// PassThrough is a pass statement.
type PassThrough struct {
	PassPos token.Pos
}

func (p *PassThrough) Pos() token.Pos {
	return p.PassPos
}

func (p *PassThrough) End() token.Pos {
	return p.PassPos
}

func (p *PassThrough) stmtNode() {}

// TypeAliasDef is a type alias definition (type X = int).
type TypeAliasDef struct {
	TypePos token.Pos
	Name    string
	TypeParams []*TypeParam
	Value   Expr
}

func (t *TypeAliasDef) Pos() token.Pos {
	return t.TypePos
}

func (t *TypeAliasDef) End() token.Pos {
	if t.Value != nil {
		return t.Value.End()
	}
	return t.TypePos
}

func (t *TypeAliasDef) stmtNode() {}

// =======================
// Expression Nodes
// =======================

// Name is a simple name/identifier expression.
type Name struct {
	NamePos token.Pos
	Name    string
}

func (n *Name) Pos() token.Pos {
	return n.NamePos
}

func (n *Name) End() token.Pos {
	return token.Pos{
		Line:   n.NamePos.Line,
		Col:    n.NamePos.Col + len(n.Name),
		Offset: n.NamePos.Offset + len(n.Name),
	}
}

func (n *Name) exprNode() {}

// Attribute is an attribute access expression (obj.attr).
type Attribute struct {
	ValuePos token.Pos
	Value    Expr
	Attr     string
}

func (a *Attribute) Pos() token.Pos {
	return a.ValuePos
}

func (a *Attribute) End() token.Pos {
	return token.Pos{
		Line:   a.Value.End().Line,
		Col:    a.Value.End().Col + 1 + len(a.Attr),
		Offset: a.Value.End().Offset + 1 + len(a.Attr),
	}
}

func (a *Attribute) exprNode() {}

// Subscript is a subscript expression (obj[index]).
type Subscript struct {
	ValuePos token.Pos
	Value    Expr
	Index    Expr
}

func (s *Subscript) Pos() token.Pos {
	return s.ValuePos
}

func (s *Subscript) End() token.Pos {
	return s.Index.End()
}

func (s *Subscript) exprNode() {}

// BinOp is a binary operation (left op right).
type BinOp struct {
	LeftPos token.Pos
	Left    Expr
	Op      token.Type
	Right   Expr
}

func (b *BinOp) Pos() token.Pos {
	return b.LeftPos
}

func (b *BinOp) End() token.Pos {
	return b.Right.End()
}

func (b *BinOp) exprNode() {}

// UnaryOp is a unary operation (op operand).
type UnaryOp struct {
	OpPos   token.Pos
	Op      token.Type
	Operand Expr
}

func (u *UnaryOp) Pos() token.Pos {
	return u.OpPos
}

func (u *UnaryOp) End() token.Pos {
	return u.Operand.End()
}

func (u *UnaryOp) exprNode() {}

// Tuple is a tuple expression (a, b, c).
type Tuple struct {
	TuplePos token.Pos
	Elts     []Expr
}

func (t *Tuple) Pos() token.Pos {
	return t.TuplePos
}

func (t *Tuple) End() token.Pos {
	if len(t.Elts) == 0 {
		return t.TuplePos
	}
	return t.Elts[len(t.Elts)-1].End()
}

func (t *Tuple) exprNode() {}

// List is a list expression [a, b, c].
type List struct {
	ListPos token.Pos
	Elts    []Expr
}

func (l *List) Pos() token.Pos {
	return l.ListPos
}

func (l *List) End() token.Pos {
	if len(l.Elts) == 0 {
		return l.ListPos
	}
	return l.Elts[len(l.Elts)-1].End()
}

func (l *List) exprNode() {}

// Dict is a dict expression {k1: v1, k2: v2}.
type Dict struct {
	DictPos token.Pos
	Keys    []Expr
	Values  []Expr
}

func (d *Dict) Pos() token.Pos {
	return d.DictPos
}

func (d *Dict) End() token.Pos {
	if len(d.Values) == 0 {
		return d.DictPos
	}
	return d.Values[len(d.Values)-1].End()
}

func (d *Dict) exprNode() {}

// Set is a set expression {a, b, c}.
type Set struct {
	SetPos token.Pos
	Elts   []Expr
}

func (s *Set) Pos() token.Pos {
	return s.SetPos
}

func (s *Set) End() token.Pos {
	if len(s.Elts) == 0 {
		return s.SetPos
	}
	return s.Elts[len(s.Elts)-1].End()
}

func (s *Set) exprNode() {}

// Constant is a constant value (number, string, bool, None).
type Constant struct {
	ConstPos token.Pos
	Value    interface{}
}

func (c *Constant) Pos() token.Pos {
	return c.ConstPos
}

func (c *Constant) End() token.Pos {
	// Approximate end based on string representation
	switch v := c.Value.(type) {
	case string:
		return token.Pos{
			Line:   c.ConstPos.Line,
			Col:    c.ConstPos.Col + len(v) + 2, // +2 for quotes
			Offset: c.ConstPos.Offset + len(v) + 2,
		}
	case int, float64:
		return token.Pos{
			Line:   c.ConstPos.Line,
			Col:    c.ConstPos.Col + 5,
			Offset: c.ConstPos.Offset + 5,
		}
	}
	return c.ConstPos
}

func (c *Constant) exprNode() {}

// Call is a function call expression func(args, keywords).
type Call struct {
	FuncPos  token.Pos
	Func     Expr
	Args     []Expr
	Keywords []*Keyword
}

func (c *Call) Pos() token.Pos {
	return c.FuncPos
}

func (c *Call) End() token.Pos {
	if len(c.Keywords) > 0 {
		// Approximate: after the last keyword
		return token.Pos{
			Line:   c.FuncPos.Line,
			Col:    c.FuncPos.Col + 50,
			Offset: c.FuncPos.Offset + 50,
		}
	}
	if len(c.Args) > 0 {
		return c.Args[len(c.Args)-1].End()
	}
	return c.FuncPos
}

func (c *Call) exprNode() {}

// Starred is a starred expression (*expr).
type Starred struct {
	StarPos token.Pos
	Value   Expr
}

func (s *Starred) Pos() token.Pos {
	return s.StarPos
}

func (s *Starred) End() token.Pos {
	return s.Value.End()
}

func (s *Starred) exprNode() {}

// IfExpr is a ternary if expression (body if test else orelse).
type IfExpr struct {
	IfPos  token.Pos
	Test   Expr
	Body   Expr
	Orelse Expr
}

func (i *IfExpr) Pos() token.Pos {
	return i.IfPos
}

func (i *IfExpr) End() token.Pos {
	return i.Orelse.End()
}

func (i *IfExpr) exprNode() {}

// =======================
// Supporting Types
// =======================

// Arguments represents function arguments (def f(args)).
type Arguments struct {
	PosOnlyArgs []*Arg
	Args        []*Arg
	VarArg      *Arg
	KwOnlyArgs  []*Arg
	VarKwArg    *Arg
	Defaults    []Expr
	KwDefaults  []Expr
}

// Arg represents a single function argument.
type Arg struct {
	ArgPos     token.Pos
	Name       string
	Annotation Expr
}

// Keyword represents a keyword argument in a call (k=v).
type Keyword struct {
	Arg   string
	Value Expr
}

// TypeParam represents a type parameter in a generic function/class.
type TypeParam struct {
	ParamPos token.Pos
	Name     string
	Bound    Expr
}

func (t *TypeParam) Pos() token.Pos {
	return t.ParamPos
}

func (t *TypeParam) End() token.Pos {
	if t.Bound != nil {
		return t.Bound.End()
	}
	return token.Pos{
		Line:   t.ParamPos.Line,
		Col:    t.ParamPos.Col + len(t.Name),
		Offset: t.ParamPos.Offset + len(t.Name),
	}
}

// ImportAlias represents a single import (module as name).
type ImportAlias struct {
	Name   string
	Asname string
}
