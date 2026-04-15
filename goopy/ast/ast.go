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
	Position token.Pos
	EndPos   token.Pos
	Body     []Stmt
}

func (m *Module) Pos() token.Pos { return m.Position }
func (m *Module) End() token.Pos { return m.EndPos }
func (m *Module) stmtNode()      {}

// FunctionDef is a function definition statement.
type FunctionDef struct {
	Name       string
	Args       *Arguments
	Body       []Stmt
	Decorators []Expr
	Returns    Expr
	IsAsync    bool
	TypeParams []*TypeParam
	Position   token.Pos
	EndPos     token.Pos
}

func (f *FunctionDef) Pos() token.Pos { return f.Position }
func (f *FunctionDef) End() token.Pos { return f.EndPos }
func (f *FunctionDef) stmtNode()      {}

// ClassDef is a class definition statement.
type ClassDef struct {
	Name       string
	Bases      []Expr
	Keywords   []*Keyword
	Body       []Stmt
	Decorators []Expr
	TypeParams []*TypeParam
	Position   token.Pos
	EndPos     token.Pos
}

func (c *ClassDef) Pos() token.Pos { return c.Position }
func (c *ClassDef) End() token.Pos { return c.EndPos }
func (c *ClassDef) stmtNode()      {}

// Assign is an assignment statement (x = value).
type Assign struct {
	Position token.Pos
	EndPos   token.Pos
	Targets  []Expr
	Value    Expr
}

func (a *Assign) Pos() token.Pos { return a.Position }
func (a *Assign) End() token.Pos { return a.EndPos }
func (a *Assign) stmtNode()      {}

// AnnAssign is an annotated assignment statement (x: int = value).
type AnnAssign struct {
	Position   token.Pos
	EndPos     token.Pos
	Target     Expr
	Annotation Expr
	Value      Expr
	Simple     bool
}

func (a *AnnAssign) Pos() token.Pos { return a.Position }
func (a *AnnAssign) End() token.Pos { return a.EndPos }
func (a *AnnAssign) stmtNode()      {}

// Import is an import statement (import x, y as z).
type Import struct {
	Position token.Pos
	EndPos   token.Pos
	Names    []*ImportAlias
}

func (i *Import) Pos() token.Pos { return i.Position }
func (i *Import) End() token.Pos { return i.EndPos }
func (i *Import) stmtNode()      {}

// ImportFrom is a from...import statement (from module import x, y).
type ImportFrom struct {
	Position token.Pos
	EndPos   token.Pos
	Module   string
	Names    []*ImportAlias
	Level    int // relative import level (0 = absolute, >0 = relative)
}

func (i *ImportFrom) Pos() token.Pos { return i.Position }
func (i *ImportFrom) End() token.Pos { return i.EndPos }
func (i *ImportFrom) stmtNode()      {}

// If is an if statement.
type If struct {
	Position token.Pos
	EndPos   token.Pos
	Test     Expr
	Body     []Stmt
	Orelse   []Stmt
}

func (i *If) Pos() token.Pos { return i.Position }
func (i *If) End() token.Pos { return i.EndPos }
func (i *If) stmtNode()      {}

// ExprStmt is an expression statement (bare expression).
type ExprStmt struct {
	Value Expr
}

func (e *ExprStmt) Pos() token.Pos { return e.Value.Pos() }
func (e *ExprStmt) End() token.Pos { return e.Value.End() }
func (e *ExprStmt) stmtNode()      {}

// PassThrough is a catch-all for statements parsed shallowly (for/while/try/with/match/pass etc.).
// Body holds inner statements so nested def/class declarations can still be discovered.
type PassThrough struct {
	Kind     string // "for", "while", "try", "with", "match", "pass", etc.
	Body     []Stmt
	Position token.Pos
	EndPos   token.Pos
}

func (p *PassThrough) Pos() token.Pos { return p.Position }
func (p *PassThrough) End() token.Pos { return p.EndPos }
func (p *PassThrough) stmtNode()      {}

// TypeAliasDef is a type alias definition (type X = int).
type TypeAliasDef struct {
	Position   token.Pos
	EndPos     token.Pos
	Name       string
	TypeParams []*TypeParam
	Value      Expr
}

func (t *TypeAliasDef) Pos() token.Pos { return t.Position }
func (t *TypeAliasDef) End() token.Pos { return t.EndPos }
func (t *TypeAliasDef) stmtNode()      {}

// =======================
// Expression Nodes
// =======================

// Name is a simple name/identifier expression.
type Name struct {
	Position token.Pos
	EndPos   token.Pos
	Name     string
}

func (n *Name) Pos() token.Pos { return n.Position }
func (n *Name) End() token.Pos { return n.EndPos }
func (n *Name) exprNode()      {}

// Attribute is an attribute access expression (obj.attr).
type Attribute struct {
	Position token.Pos
	EndPos   token.Pos
	Value    Expr
	Attr     string
}

func (a *Attribute) Pos() token.Pos { return a.Position }
func (a *Attribute) End() token.Pos { return a.EndPos }
func (a *Attribute) exprNode()      {}

// Subscript is a subscript expression (obj[slice]).
type Subscript struct {
	Position token.Pos
	EndPos   token.Pos
	Value    Expr
	Slice    Expr
}

func (s *Subscript) Pos() token.Pos { return s.Position }
func (s *Subscript) End() token.Pos { return s.EndPos }
func (s *Subscript) exprNode()      {}

// BinOp is a binary operation (left op right).
type BinOp struct {
	Position token.Pos
	EndPos   token.Pos
	Left     Expr
	Op       token.Type
	Right    Expr
}

func (b *BinOp) Pos() token.Pos { return b.Position }
func (b *BinOp) End() token.Pos { return b.EndPos }
func (b *BinOp) exprNode()      {}

// UnaryOp is a unary operation (op operand).
type UnaryOp struct {
	Position token.Pos
	EndPos   token.Pos
	Op       token.Type
	Operand  Expr
}

func (u *UnaryOp) Pos() token.Pos { return u.Position }
func (u *UnaryOp) End() token.Pos { return u.EndPos }
func (u *UnaryOp) exprNode()      {}

// Tuple is a tuple expression (a, b, c).
type Tuple struct {
	Position token.Pos
	EndPos   token.Pos
	Elts     []Expr
}

func (t *Tuple) Pos() token.Pos { return t.Position }
func (t *Tuple) End() token.Pos { return t.EndPos }
func (t *Tuple) exprNode()      {}

// List is a list expression [a, b, c].
type List struct {
	Position token.Pos
	EndPos   token.Pos
	Elts     []Expr
}

func (l *List) Pos() token.Pos { return l.Position }
func (l *List) End() token.Pos { return l.EndPos }
func (l *List) exprNode()      {}

// Dict is a dict expression {k1: v1, k2: v2}.
type Dict struct {
	Position token.Pos
	EndPos   token.Pos
	Keys     []Expr
	Values   []Expr
}

func (d *Dict) Pos() token.Pos { return d.Position }
func (d *Dict) End() token.Pos { return d.EndPos }
func (d *Dict) exprNode()      {}

// Set is a set expression {a, b, c}.
type Set struct {
	Position token.Pos
	EndPos   token.Pos
	Elts     []Expr
}

func (s *Set) Pos() token.Pos { return s.Position }
func (s *Set) End() token.Pos { return s.EndPos }
func (s *Set) exprNode()      {}

// Constant is a constant value (number, string, bool, None).
// Value holds the string representation, Kind identifies the type,
// and Lit preserves the raw source literal text.
type Constant struct {
	Position token.Pos
	EndPos   token.Pos
	Value    string // string representation: "42", "hello", "True"
	Kind     string // "str", "int", "float", "bool", "none", "ellipsis"
	Lit      string // raw source literal text (preserves quotes for strings)
}

func (c *Constant) Pos() token.Pos { return c.Position }
func (c *Constant) End() token.Pos { return c.EndPos }
func (c *Constant) exprNode()      {}

// Call is a function call expression func(args, keywords).
type Call struct {
	Position token.Pos
	EndPos   token.Pos
	Func     Expr
	Args     []Expr
	Keywords []*Keyword
}

func (c *Call) Pos() token.Pos { return c.Position }
func (c *Call) End() token.Pos { return c.EndPos }
func (c *Call) exprNode()      {}

// Starred is a starred expression (*expr).
type Starred struct {
	Position token.Pos
	EndPos   token.Pos
	Value    Expr
}

func (s *Starred) Pos() token.Pos { return s.Position }
func (s *Starred) End() token.Pos { return s.EndPos }
func (s *Starred) exprNode()      {}

// IfExpr is a ternary if expression (body if test else orelse).
type IfExpr struct {
	Position token.Pos
	EndPos   token.Pos
	Test     Expr
	Body     Expr
	Orelse   Expr
}

func (i *IfExpr) Pos() token.Pos { return i.Position }
func (i *IfExpr) End() token.Pos { return i.EndPos }
func (i *IfExpr) exprNode()      {}

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
	Position token.Pos
	EndPos   token.Pos
	Name     string
	Bound    Expr
}

func (t *TypeParam) Pos() token.Pos { return t.Position }
func (t *TypeParam) End() token.Pos { return t.EndPos }

// ImportAlias represents a single import (module as name).
type ImportAlias struct {
	Name  string
	Alias string
}
