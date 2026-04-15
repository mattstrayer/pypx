# goopy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a pure Go library that parses Python source code and extracts structured API documentation (functions, classes, exceptions, docstrings, type annotations) from PyPI wheels, replacing the Python docs-worker sidecar.

**Architecture:** Two-pass parser (lexer emits INDENT/DEDENT tokens, recursive-descent parser builds AST), followed by an extractor that walks the AST to produce a rich documentation model. Docstring parser handles Google, NumPy, and Sphinx styles. Wheel package downloads and extracts `.py` files from PyPI.

**Tech Stack:** Go 1.26, stdlib only (no third-party dependencies). Module at `goopy/` in the repo root with its own `go.mod`.

**Spec:** `docs/superpowers/specs/2026-04-14-goopy-design.md`

---

## Phase 1: Foundation (Token, AST, Model types)

### Task 1: Initialize Go module and define token types

**Files:**
- Create: `goopy/go.mod`
- Create: `goopy/token/token.go`
- Create: `goopy/token/token_test.go`

- [ ] **Step 1: Initialize the Go module**

```bash
mkdir -p goopy/token
cd goopy && go mod init github.com/pypx/goopy
```

- [ ] **Step 2: Write token type tests**

Create `goopy/token/token_test.go`:

```go
package token

import "testing"

func TestTokenString(t *testing.T) {
	tests := []struct {
		tok  Type
		want string
	}{
		{DEF, "DEF"},
		{CLASS, "CLASS"},
		{NAME, "NAME"},
		{STRING, "STRING"},
		{INDENT, "INDENT"},
		{DEDENT, "DEDENT"},
		{NEWLINE, "NEWLINE"},
		{ARROW, "ARROW"},
		{ELLIPSIS, "ELLIPSIS"},
		{EOF, "EOF"},
	}
	for _, tt := range tests {
		if got := tt.tok.String(); got != tt.want {
			t.Errorf("Type(%d).String() = %q, want %q", tt.tok, got, tt.want)
		}
	}
}

func TestLookupKeyword(t *testing.T) {
	tests := []struct {
		ident string
		want  Type
		isKw  bool
	}{
		{"def", DEF, true},
		{"class", CLASS, true},
		{"import", IMPORT, true},
		{"from", FROM, true},
		{"as", AS, true},
		{"if", IF, true},
		{"elif", ELIF, true},
		{"else", ELSE, true},
		{"async", ASYNC, true},
		{"return", RETURN, true},
		{"raise", RAISE, true},
		{"pass", PASS, true},
		{"None", NONE, true},
		{"True", TRUE, true},
		{"False", FALSE, true},
		{"type", TYPE, true},
		{"for", FOR, true},
		{"while", WHILE, true},
		{"try", TRY, true},
		{"except", EXCEPT, true},
		{"finally", FINALLY, true},
		{"with", WITH, true},
		{"match", MATCH, true},
		{"case", CASE, true},
		{"myvar", NAME, false},
		{"Class", NAME, false},
		{"DEF", NAME, false},
	}
	for _, tt := range tests {
		got, isKw := LookupKeyword(tt.ident)
		if got != tt.want || isKw != tt.isKw {
			t.Errorf("LookupKeyword(%q) = (%v, %v), want (%v, %v)",
				tt.ident, got, isKw, tt.want, tt.isKw)
		}
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd goopy && go test ./token/...
```

Expected: compilation errors — types not defined yet.

- [ ] **Step 4: Implement token types**

Create `goopy/token/token.go`:

```go
package token

// Type represents a token type in the Python lexer.
type Type int

const (
	// Special
	EOF Type = iota
	ILLEGAL
	NEWLINE
	INDENT
	DEDENT

	// Literals
	NAME
	NUMBER
	STRING

	// Keywords
	DEF
	CLASS
	IMPORT
	FROM
	AS
	IF
	ELIF
	ELSE
	ASYNC
	RETURN
	RAISE
	PASS
	NONE
	TRUE
	FALSE
	TYPE
	FOR
	WHILE
	TRY
	EXCEPT
	FINALLY
	WITH
	MATCH
	CASE
	YIELD
	DEL
	GLOBAL
	NONLOCAL
	ASSERT
	LAMBDA
	AND
	OR
	NOT
	IN
	IS

	// Operators
	ARROW    // ->
	COLON    // :
	ASSIGN   // =
	PIPE     // |
	STAR     // *
	DSTAR    // **
	DOT      // .
	ELLIPSIS // ...
	SLASH    // /
	AT       // @
	COMMA    // ,
	SEMI     // ;
	PLUS     // +
	MINUS    // -
	WALRUS   // :=
	EQ       // ==
	NEQ      // !=
	LT       // <
	GT       // >
	LTE      // <=
	GTE      // >=
	TILDE    // ~
	AMPER    // &
	CARET    // ^
	PERCENT  // %
	LSHIFT   // <<
	RSHIFT   // >>
	DSLASH   // //

	// Delimiters
	LPAREN // (
	RPAREN // )
	LBRACK // [
	RBRACK // ]
	LBRACE // {
	RBRACE // }
)

var typeNames = map[Type]string{
	EOF: "EOF", ILLEGAL: "ILLEGAL", NEWLINE: "NEWLINE",
	INDENT: "INDENT", DEDENT: "DEDENT",
	NAME: "NAME", NUMBER: "NUMBER", STRING: "STRING",
	DEF: "DEF", CLASS: "CLASS", IMPORT: "IMPORT", FROM: "FROM",
	AS: "AS", IF: "IF", ELIF: "ELIF", ELSE: "ELSE",
	ASYNC: "ASYNC", RETURN: "RETURN", RAISE: "RAISE", PASS: "PASS",
	NONE: "NONE", TRUE: "TRUE", FALSE: "FALSE", TYPE: "TYPE",
	FOR: "FOR", WHILE: "WHILE", TRY: "TRY", EXCEPT: "EXCEPT",
	FINALLY: "FINALLY", WITH: "WITH", MATCH: "MATCH", CASE: "CASE",
	YIELD: "YIELD", DEL: "DEL", GLOBAL: "GLOBAL", NONLOCAL: "NONLOCAL",
	ASSERT: "ASSERT", LAMBDA: "LAMBDA",
	AND: "AND", OR: "OR", NOT: "NOT", IN: "IN", IS: "IS",
	ARROW: "ARROW", COLON: "COLON", ASSIGN: "ASSIGN", PIPE: "PIPE",
	STAR: "STAR", DSTAR: "DSTAR", DOT: "DOT", ELLIPSIS: "ELLIPSIS",
	SLASH: "SLASH", AT: "AT", COMMA: "COMMA", SEMI: "SEMI",
	PLUS: "PLUS", MINUS: "MINUS", WALRUS: "WALRUS",
	EQ: "EQ", NEQ: "NEQ", LT: "LT", GT: "GT", LTE: "LTE", GTE: "GTE",
	TILDE: "TILDE", AMPER: "AMPER", CARET: "CARET", PERCENT: "PERCENT",
	LSHIFT: "LSHIFT", RSHIFT: "RSHIFT", DSLASH: "DSLASH",
	LPAREN: "LPAREN", RPAREN: "RPAREN",
	LBRACK: "LBRACK", RBRACK: "RBRACK",
	LBRACE: "LBRACE", RBRACE: "RBRACE",
}

// String returns the name of the token type.
func (t Type) String() string {
	if name, ok := typeNames[t]; ok {
		return name
	}
	return "UNKNOWN"
}

var keywords = map[string]Type{
	"def": DEF, "class": CLASS, "import": IMPORT, "from": FROM,
	"as": AS, "if": IF, "elif": ELIF, "else": ELSE,
	"async": ASYNC, "return": RETURN, "raise": RAISE, "pass": PASS,
	"None": NONE, "True": TRUE, "False": FALSE, "type": TYPE,
	"for": FOR, "while": WHILE, "try": TRY, "except": EXCEPT,
	"finally": FINALLY, "with": WITH, "match": MATCH, "case": CASE,
	"yield": YIELD, "del": DEL, "global": GLOBAL, "nonlocal": NONLOCAL,
	"assert": ASSERT, "lambda": LAMBDA,
	"and": AND, "or": OR, "not": NOT, "in": IN, "is": IS,
}

// LookupKeyword returns the keyword token type for ident, or NAME if not a keyword.
func LookupKeyword(ident string) (Type, bool) {
	if tok, ok := keywords[ident]; ok {
		return tok, true
	}
	return NAME, false
}

// Pos represents a position in source code.
type Pos struct {
	Line   int // 1-based line number
	Col    int // 0-based column (byte offset within line)
	Offset int // byte offset from start of source
}

// Token is a single lexical token with its position and literal value.
type Token struct {
	Type    Type
	Lit     string // literal text (for NAME, NUMBER, STRING); empty for keywords/operators
	Pos     Pos
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd goopy && go test ./token/... -v
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add goopy/go.mod goopy/token/
git commit -m "feat(goopy): add token types and keyword lookup"
```

---

### Task 2: Define AST node types

**Files:**
- Create: `goopy/ast/ast.go`
- Create: `goopy/ast/ast_test.go`

- [ ] **Step 1: Write AST tests**

Create `goopy/ast/ast_test.go`:

```go
package ast

import (
	"testing"

	"github.com/pypx/goopy/token"
)

func TestNodePositions(t *testing.T) {
	fn := &FunctionDef{
		Name:     "hello",
		IsAsync:  false,
		Position: token.Pos{Line: 1, Col: 0, Offset: 0},
		EndPos:   token.Pos{Line: 3, Col: 0, Offset: 30},
	}
	if fn.Pos() != fn.Position {
		t.Errorf("FunctionDef.Pos() = %v, want %v", fn.Pos(), fn.Position)
	}
	if fn.End() != fn.EndPos {
		t.Errorf("FunctionDef.End() = %v, want %v", fn.End(), fn.EndPos)
	}
}

func TestModuleBody(t *testing.T) {
	mod := &Module{
		Body: []Stmt{
			&FunctionDef{Name: "foo"},
			&ClassDef{Name: "Bar"},
		},
	}
	if len(mod.Body) != 2 {
		t.Fatalf("Module.Body has %d stmts, want 2", len(mod.Body))
	}
	fn, ok := mod.Body[0].(*FunctionDef)
	if !ok {
		t.Fatal("Body[0] is not *FunctionDef")
	}
	if fn.Name != "foo" {
		t.Errorf("FunctionDef.Name = %q, want %q", fn.Name, "foo")
	}
	cls, ok := mod.Body[1].(*ClassDef)
	if !ok {
		t.Fatal("Body[1] is not *ClassDef")
	}
	if cls.Name != "Bar" {
		t.Errorf("ClassDef.Name = %q, want %q", cls.Name, "Bar")
	}
}

func TestArgumentsAllParams(t *testing.T) {
	args := &Arguments{
		PosOnlyArgs: []Arg{{Name: "a"}},
		Args:        []Arg{{Name: "b"}},
		VarArg:      &Arg{Name: "args"},
		KwOnlyArgs:  []Arg{{Name: "c"}},
		VarKwArg:    &Arg{Name: "kwargs"},
	}
	if args.PosOnlyArgs[0].Name != "a" {
		t.Error("PosOnlyArgs[0] wrong")
	}
	if args.VarArg.Name != "args" {
		t.Error("VarArg wrong")
	}
	if args.VarKwArg.Name != "kwargs" {
		t.Error("VarKwArg wrong")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd goopy && go test ./ast/... -v
```

Expected: compilation errors.

- [ ] **Step 3: Implement AST node types**

Create `goopy/ast/ast.go`:

```go
package ast

import "github.com/pypx/goopy/token"

// Node is the interface implemented by all AST nodes.
type Node interface {
	Pos() token.Pos
	End() token.Pos
}

// Stmt is the interface implemented by all statement nodes.
type Stmt interface {
	Node
	stmtNode()
}

// Expr is the interface implemented by all expression nodes.
type Expr interface {
	Node
	exprNode()
}

// Module is the top-level AST node representing an entire Python file.
type Module struct {
	Body []Stmt
}

func (m *Module) Pos() token.Pos {
	if len(m.Body) > 0 {
		return m.Body[0].Pos()
	}
	return token.Pos{}
}
func (m *Module) End() token.Pos {
	if len(m.Body) > 0 {
		return m.Body[len(m.Body)-1].End()
	}
	return token.Pos{}
}

// --- Statements ---

type FunctionDef struct {
	Name       string
	Args       *Arguments
	Body       []Stmt
	Decorators []Expr
	Returns    Expr
	IsAsync    bool
	TypeParams []TypeParam
	Position   token.Pos
	EndPos     token.Pos
}

func (n *FunctionDef) Pos() token.Pos { return n.Position }
func (n *FunctionDef) End() token.Pos { return n.EndPos }
func (n *FunctionDef) stmtNode()      {}

type ClassDef struct {
	Name       string
	Bases      []Expr
	Keywords   []Keyword
	Body       []Stmt
	Decorators []Expr
	TypeParams []TypeParam
	Position   token.Pos
	EndPos     token.Pos
}

func (n *ClassDef) Pos() token.Pos { return n.Position }
func (n *ClassDef) End() token.Pos { return n.EndPos }
func (n *ClassDef) stmtNode()      {}

type Assign struct {
	Targets  []Expr
	Value    Expr
	Position token.Pos
}

func (n *Assign) Pos() token.Pos { return n.Position }
func (n *Assign) End() token.Pos { return n.Value.End() }
func (n *Assign) stmtNode()      {}

type AnnAssign struct {
	Target     Expr
	Annotation Expr
	Value      Expr // nil if no assignment
	Position   token.Pos
}

func (n *AnnAssign) Pos() token.Pos { return n.Position }
func (n *AnnAssign) End() token.Pos {
	if n.Value != nil {
		return n.Value.End()
	}
	return n.Annotation.End()
}
func (n *AnnAssign) stmtNode() {}

type Import struct {
	Names    []ImportAlias
	Position token.Pos
}

func (n *Import) Pos() token.Pos { return n.Position }
func (n *Import) End() token.Pos { return n.Position } // approximate
func (n *Import) stmtNode()      {}

type ImportFrom struct {
	Module   string
	Names    []ImportAlias
	Level    int // number of leading dots for relative imports
	Position token.Pos
}

func (n *ImportFrom) Pos() token.Pos { return n.Position }
func (n *ImportFrom) End() token.Pos { return n.Position } // approximate
func (n *ImportFrom) stmtNode()      {}

type ImportAlias struct {
	Name  string
	Alias string // empty if no "as"
}

type If struct {
	Test     Expr
	Body     []Stmt
	OrElse   []Stmt
	Position token.Pos
	EndPos   token.Pos
}

func (n *If) Pos() token.Pos { return n.Position }
func (n *If) End() token.Pos { return n.EndPos }
func (n *If) stmtNode()      {}

type ExprStmt struct {
	Value    Expr
	Position token.Pos
}

func (n *ExprStmt) Pos() token.Pos { return n.Position }
func (n *ExprStmt) End() token.Pos { return n.Value.End() }
func (n *ExprStmt) stmtNode()      {}

// PassThrough represents statements we parse shallowly (for, while, try, with, match).
// We recurse into Body to find nested def/class but don't model control flow.
type PassThrough struct {
	Kind     string // "for", "while", "try", "with", "match", etc.
	Body     []Stmt
	Position token.Pos
	EndPos   token.Pos
}

func (n *PassThrough) Pos() token.Pos { return n.Position }
func (n *PassThrough) End() token.Pos { return n.EndPos }
func (n *PassThrough) stmtNode()      {}

// TypeAliasDef represents PEP 695: type X = ...
type TypeAliasDef struct {
	Name       string
	Value      Expr
	TypeParams []TypeParam
	Position   token.Pos
	EndPos     token.Pos
}

func (n *TypeAliasDef) Pos() token.Pos { return n.Position }
func (n *TypeAliasDef) End() token.Pos { return n.EndPos }
func (n *TypeAliasDef) stmtNode()      {}

// --- Arguments ---

type Arguments struct {
	PosOnlyArgs []Arg
	Args        []Arg
	VarArg      *Arg   // *args
	KwOnlyArgs  []Arg
	VarKwArg    *Arg   // **kwargs
	Defaults    []Expr // defaults for Args (right-aligned)
	KwDefaults  []Expr // defaults for KwOnlyArgs (1:1, nil = no default)
}

type Arg struct {
	Name       string
	Annotation Expr
	Position   token.Pos
}

// --- Expressions ---

type Name struct {
	ID       string
	Position token.Pos
	EndPos   token.Pos
}

func (n *Name) Pos() token.Pos { return n.Position }
func (n *Name) End() token.Pos { return n.EndPos }
func (n *Name) exprNode()      {}

type Attribute struct {
	Value    Expr
	Attr     string
	Position token.Pos
	EndPos   token.Pos
}

func (n *Attribute) Pos() token.Pos { return n.Position }
func (n *Attribute) End() token.Pos { return n.EndPos }
func (n *Attribute) exprNode()      {}

type Subscript struct {
	Value    Expr
	Slice    Expr
	Position token.Pos
	EndPos   token.Pos
}

func (n *Subscript) Pos() token.Pos { return n.Position }
func (n *Subscript) End() token.Pos { return n.EndPos }
func (n *Subscript) exprNode()      {}

type BinOp struct {
	Left     Expr
	Op       string
	Right    Expr
	Position token.Pos
	EndPos   token.Pos
}

func (n *BinOp) Pos() token.Pos { return n.Position }
func (n *BinOp) End() token.Pos { return n.EndPos }
func (n *BinOp) exprNode()      {}

type UnaryOp struct {
	Op       string
	Operand  Expr
	Position token.Pos
	EndPos   token.Pos
}

func (n *UnaryOp) Pos() token.Pos { return n.Position }
func (n *UnaryOp) End() token.Pos { return n.EndPos }
func (n *UnaryOp) exprNode()      {}

type Tuple struct {
	Elts     []Expr
	Position token.Pos
	EndPos   token.Pos
}

func (n *Tuple) Pos() token.Pos { return n.Position }
func (n *Tuple) End() token.Pos { return n.EndPos }
func (n *Tuple) exprNode()      {}

type List struct {
	Elts     []Expr
	Position token.Pos
	EndPos   token.Pos
}

func (n *List) Pos() token.Pos { return n.Position }
func (n *List) End() token.Pos { return n.EndPos }
func (n *List) exprNode()      {}

type Dict struct {
	Keys     []Expr // nil key = **unpack
	Values   []Expr
	Position token.Pos
	EndPos   token.Pos
}

func (n *Dict) Pos() token.Pos { return n.Position }
func (n *Dict) End() token.Pos { return n.EndPos }
func (n *Dict) exprNode()      {}

type Set struct {
	Elts     []Expr
	Position token.Pos
	EndPos   token.Pos
}

func (n *Set) Pos() token.Pos { return n.Position }
func (n *Set) End() token.Pos { return n.EndPos }
func (n *Set) exprNode()      {}

type Constant struct {
	Value    string // string representation: "42", `"hello"`, "True", "None"
	Kind     string // "str", "int", "float", "bool", "none", "ellipsis"
	Position token.Pos
	EndPos   token.Pos
}

func (n *Constant) Pos() token.Pos { return n.Position }
func (n *Constant) End() token.Pos { return n.EndPos }
func (n *Constant) exprNode()      {}

type Call struct {
	Func     Expr
	Args     []Expr
	Keywords []Keyword
	Position token.Pos
	EndPos   token.Pos
}

func (n *Call) Pos() token.Pos { return n.Position }
func (n *Call) End() token.Pos { return n.EndPos }
func (n *Call) exprNode()      {}

type Keyword struct {
	Arg      string // empty for **kwargs
	Value    Expr
	Position token.Pos
}

type Starred struct {
	Value    Expr
	Position token.Pos
	EndPos   token.Pos
}

func (n *Starred) Pos() token.Pos { return n.Position }
func (n *Starred) End() token.Pos { return n.EndPos }
func (n *Starred) exprNode()      {}

type IfExpr struct {
	Body     Expr
	Test     Expr
	OrElse   Expr
	Position token.Pos
	EndPos   token.Pos
}

func (n *IfExpr) Pos() token.Pos { return n.Position }
func (n *IfExpr) End() token.Pos { return n.EndPos }
func (n *IfExpr) exprNode()      {}

// --- Type Parameters (PEP 695) ---

type TypeParam struct {
	Name        string
	Kind        string // "typevar", "paramspec", "typevartuple"
	Bound       Expr
	Default     Expr
	Constraints []Expr
	Position    token.Pos
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd goopy && go test ./ast/... -v
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add goopy/ast/
git commit -m "feat(goopy): add AST node types"
```

---

### Task 3: Define public model types

**Files:**
- Create: `goopy/model/model.go`
- Create: `goopy/model/typeexpr.go`
- Create: `goopy/model/model_test.go`

- [ ] **Step 1: Write model tests**

Create `goopy/model/model_test.go`:

```go
package model

import (
	"encoding/json"
	"testing"
)

func TestTypeExprJSON(t *testing.T) {
	expr := TypeExpr{
		Kind: TypeGeneric,
		Name: "dict",
		Args: []TypeExpr{
			{Kind: TypeName, Name: "str", Raw: "str"},
			{Kind: TypeName, Name: "int", Raw: "int"},
		},
		Raw: "dict[str, int]",
	}

	data, err := json.Marshal(expr)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got TypeExpr
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Kind != TypeGeneric {
		t.Errorf("Kind = %q, want %q", got.Kind, TypeGeneric)
	}
	if got.Name != "dict" {
		t.Errorf("Name = %q, want %q", got.Name, "dict")
	}
	if len(got.Args) != 2 {
		t.Fatalf("Args len = %d, want 2", len(got.Args))
	}
	if got.Args[0].Name != "str" {
		t.Errorf("Args[0].Name = %q, want %q", got.Args[0].Name, "str")
	}
	if got.Raw != "dict[str, int]" {
		t.Errorf("Raw = %q, want %q", got.Raw, "dict[str, int]")
	}
}

func TestParamKindValues(t *testing.T) {
	kinds := []ParamKind{
		ParamPositionalOnly, ParamPositionalOrKw, ParamVarPositional,
		ParamKeywordOnly, ParamVarKeyword,
	}
	seen := map[ParamKind]bool{}
	for _, k := range kinds {
		if seen[k] {
			t.Errorf("duplicate ParamKind: %q", k)
		}
		seen[k] = true
		if k == "" {
			t.Error("empty ParamKind")
		}
	}
}

func TestDocstringStyleValues(t *testing.T) {
	styles := []DocstringStyle{StyleGoogle, StyleNumPy, StyleSphinx, StylePlain}
	for _, s := range styles {
		if s == "" {
			t.Error("empty DocstringStyle")
		}
	}
}

func TestModuleJSON(t *testing.T) {
	mod := Module{
		Name:      "mymodule",
		Docstring: "A test module.",
		Functions: []Function{
			{
				Name:      "hello",
				Signature: "def hello(name: str) -> str",
				Parameters: []Parameter{
					{
						Name: "name",
						Type: &TypeExpr{Kind: TypeName, Name: "str", Raw: "str"},
						Kind: ParamPositionalOrKw,
					},
				},
				Returns: &TypeRef{
					Type: &TypeExpr{Kind: TypeName, Name: "str", Raw: "str"},
				},
			},
		},
		Classes:    []Class{},
		Exceptions: []Class{},
	}

	data, err := json.Marshal(mod)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got Module
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Name != "mymodule" {
		t.Errorf("Name = %q, want %q", got.Name, "mymodule")
	}
	if len(got.Functions) != 1 {
		t.Fatalf("Functions len = %d, want 1", len(got.Functions))
	}
	if got.Functions[0].Parameters[0].Type.Name != "str" {
		t.Errorf("param type = %q, want %q", got.Functions[0].Parameters[0].Type.Name, "str")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd goopy && go test ./model/... -v
```

Expected: compilation errors.

- [ ] **Step 3: Implement model types**

Create `goopy/model/model.go`:

```go
package model

// Package is the top-level output from goopy: documentation for an entire Python package.
type Package struct {
	Name    string   `json:"name"`
	Modules []Module `json:"modules"`
}

// Module represents a single Python module's documented API.
type Module struct {
	Name        string      `json:"name"`
	Docstring   string      `json:"docstring"`
	Functions   []Function  `json:"functions"`
	Classes     []Class     `json:"classes"`
	Exceptions  []Class     `json:"exceptions"`
	Constants   []Attribute `json:"constants"`
	TypeAliases []TypeAlias `json:"type_aliases"`
	Exports     []string    `json:"exports"`
}

// Function represents a documented Python function or method.
type Function struct {
	Name          string      `json:"name"`
	Signature     string      `json:"signature"`
	Docstring     *Docstring  `json:"docstring"`
	Parameters    []Parameter `json:"parameters"`
	Returns       *TypeRef    `json:"returns"`
	Decorators    []string    `json:"decorators"`
	IsAsync       bool        `json:"is_async"`
	IsProperty    bool        `json:"is_property"`
	IsStatic      bool        `json:"is_static"`
	IsClassMethod bool        `json:"is_classmethod"`
	IsAbstract    bool        `json:"is_abstract"`
}

// Class represents a documented Python class.
type Class struct {
	Name        string      `json:"name"`
	Signature   string      `json:"signature"`
	Docstring   *Docstring  `json:"docstring"`
	Parameters  []Parameter `json:"parameters"`
	Bases       []string    `json:"bases"`
	Decorators  []string    `json:"decorators"`
	Methods     []Function  `json:"methods"`
	Properties  []Function  `json:"properties"`
	Attributes  []Attribute `json:"attributes"`
	IsAbstract  bool        `json:"is_abstract"`
	IsDataclass bool        `json:"is_dataclass"`
}

// Parameter represents a function/method parameter.
type Parameter struct {
	Name        string    `json:"name"`
	Type        *TypeExpr `json:"type,omitempty"`
	Default     string    `json:"default,omitempty"`
	Description string    `json:"description,omitempty"`
	Kind        ParamKind `json:"kind"`
}

// ParamKind classifies how a parameter is passed.
type ParamKind string

const (
	ParamPositionalOnly ParamKind = "positional_only"
	ParamPositionalOrKw ParamKind = "positional_or_keyword"
	ParamVarPositional  ParamKind = "var_positional"
	ParamKeywordOnly    ParamKind = "keyword_only"
	ParamVarKeyword     ParamKind = "var_keyword"
)

// Attribute represents a module-level constant or class attribute.
type Attribute struct {
	Name      string    `json:"name"`
	Type      *TypeExpr `json:"type,omitempty"`
	Value     string    `json:"value,omitempty"`
	Docstring string    `json:"docstring,omitempty"`
}

// TypeAlias represents a Python type alias (e.g., type X = int | str).
type TypeAlias struct {
	Name  string    `json:"name"`
	Value *TypeExpr `json:"value"`
}

// TypeRef is a type annotation with an optional description (used for return types).
type TypeRef struct {
	Type        *TypeExpr `json:"type"`
	Description string    `json:"description,omitempty"`
}

// Docstring is a parsed Python docstring with structured sections.
type Docstring struct {
	Raw         string         `json:"raw"`
	Summary     string         `json:"summary"`
	Description string         `json:"description,omitempty"`
	Params      []DocParam     `json:"params,omitempty"`
	Returns     []DocReturn    `json:"returns,omitempty"`
	Raises      []DocRaises    `json:"raises,omitempty"`
	Examples    []string       `json:"examples,omitempty"`
	Notes       string         `json:"notes,omitempty"`
	Style       DocstringStyle `json:"style"`
}

// DocParam is a parameter documented in a docstring.
type DocParam struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Description string `json:"description"`
}

// DocReturn is a return value documented in a docstring.
type DocReturn struct {
	Type        string `json:"type,omitempty"`
	Description string `json:"description"`
}

// DocRaises is an exception documented in a docstring.
type DocRaises struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// DocstringStyle identifies the docstring convention used.
type DocstringStyle string

const (
	StyleGoogle DocstringStyle = "google"
	StyleNumPy  DocstringStyle = "numpy"
	StyleSphinx DocstringStyle = "sphinx"
	StylePlain  DocstringStyle = "plain"
)
```

Create `goopy/model/typeexpr.go`:

```go
package model

// TypeExpr is a structured representation of a Python type annotation.
// Every node retains the original source text in Raw.
type TypeExpr struct {
	Kind     TypeExprKind `json:"kind"`
	Name     string       `json:"name,omitempty"`
	Args     []TypeExpr   `json:"args,omitempty"`
	Elements []TypeExpr   `json:"elements,omitempty"`
	Returns  *TypeExpr    `json:"returns,omitempty"`
	Value    string       `json:"value,omitempty"`
	Raw      string       `json:"raw"`
}

// TypeExprKind classifies the kind of type expression.
type TypeExprKind string

const (
	TypeName      TypeExprKind = "name"       // str, int, MyClass
	TypeGeneric   TypeExprKind = "generic"    // list[int], dict[str, Any]
	TypeUnion     TypeExprKind = "union"      // str | int, Union[str, int]
	TypeOptional  TypeExprKind = "optional"   // Optional[str]
	TypeCallable  TypeExprKind = "callable"   // Callable[[int, str], bool]
	TypeTuple     TypeExprKind = "tuple"      // tuple[int, str, ...]
	TypeLiteral   TypeExprKind = "literal"    // Literal["a", "b", 3]
	TypeNone      TypeExprKind = "none"       // None
	TypeEllipsis  TypeExprKind = "ellipsis"   // ...
	TypeUnpack    TypeExprKind = "unpack"     // *tuple[int, ...]
	TypeParamSpec TypeExprKind = "paramspec"  // ParamSpec, **P
)
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd goopy && go test ./model/... -v
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add goopy/model/
git commit -m "feat(goopy): add public model and TypeExpr types"
```

---

## Phase 2: Lexer

### Task 4: Basic lexer — identifiers, keywords, operators, newlines

**Files:**
- Create: `goopy/lexer/lexer.go`
- Create: `goopy/lexer/lexer_test.go`

- [ ] **Step 1: Write basic lexer tests**

Create `goopy/lexer/lexer_test.go`:

```go
package lexer

import (
	"testing"

	"github.com/pypx/goopy/token"
)

func tokenize(src string) []token.Token {
	l := New([]byte(src))
	var tokens []token.Token
	for {
		tok := l.Next()
		tokens = append(tokens, tok)
		if tok.Type == token.EOF {
			break
		}
	}
	return tokens
}

func tokenTypes(tokens []token.Token) []token.Type {
	types := make([]token.Type, len(tokens))
	for i, t := range tokens {
		types[i] = t.Type
	}
	return types
}

func expectTypes(t *testing.T, src string, want []token.Type) {
	t.Helper()
	tokens := tokenize(src)
	got := tokenTypes(tokens)
	if len(got) != len(want) {
		t.Fatalf("tokenize(%q): got %d tokens %v, want %d tokens %v", src, len(got), got, len(want), want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("tokenize(%q): token[%d] = %v, want %v", src, i, got[i], want[i])
		}
	}
}

func TestEmpty(t *testing.T) {
	expectTypes(t, "", []token.Type{token.NEWLINE, token.EOF})
}

func TestSimpleDef(t *testing.T) {
	expectTypes(t, "def foo():\n    pass\n", []token.Type{
		token.DEF, token.NAME, token.LPAREN, token.RPAREN, token.COLON, token.NEWLINE,
		token.INDENT, token.PASS, token.NEWLINE,
		token.DEDENT, token.EOF,
	})
}

func TestKeywords(t *testing.T) {
	expectTypes(t, "class if else import from as\n", []token.Type{
		token.CLASS, token.IF, token.ELSE, token.IMPORT, token.FROM, token.AS, token.NEWLINE,
		token.EOF,
	})
}

func TestOperators(t *testing.T) {
	expectTypes(t, "a -> b : c = d | e\n", []token.Type{
		token.NAME, token.ARROW, token.NAME, token.COLON, token.NAME,
		token.ASSIGN, token.NAME, token.PIPE, token.NAME, token.NEWLINE,
		token.EOF,
	})
}

func TestEllipsis(t *testing.T) {
	expectTypes(t, "...\n", []token.Type{token.ELLIPSIS, token.NEWLINE, token.EOF})
}

func TestNumbers(t *testing.T) {
	tokens := tokenize("42 3.14 0xff 1_000\n")
	nums := 0
	for _, tok := range tokens {
		if tok.Type == token.NUMBER {
			nums++
		}
	}
	if nums != 4 {
		t.Errorf("expected 4 NUMBER tokens, got %d", nums)
	}
}

func TestCommentIgnored(t *testing.T) {
	expectTypes(t, "x = 1  # comment\n", []token.Type{
		token.NAME, token.ASSIGN, token.NUMBER, token.NEWLINE, token.EOF,
	})
}

func TestStarDstar(t *testing.T) {
	expectTypes(t, "*args **kwargs\n", []token.Type{
		token.STAR, token.NAME, token.DSTAR, token.NAME, token.NEWLINE, token.EOF,
	})
}

func TestDecorator(t *testing.T) {
	expectTypes(t, "@property\ndef x():\n    pass\n", []token.Type{
		token.AT, token.NAME, token.NEWLINE,
		token.DEF, token.NAME, token.LPAREN, token.RPAREN, token.COLON, token.NEWLINE,
		token.INDENT, token.PASS, token.NEWLINE,
		token.DEDENT, token.EOF,
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd goopy && go test ./lexer/... -v
```

Expected: compilation errors.

- [ ] **Step 3: Implement the lexer**

Create `goopy/lexer/lexer.go`:

```go
package lexer

import (
	"github.com/pypx/goopy/token"
)

// Lexer tokenizes Python source code, emitting INDENT/DEDENT tokens
// for Python's indentation-based block structure.
type Lexer struct {
	src    []byte
	pos    int // current byte offset
	line   int // 1-based
	col    int // 0-based
	tokens []token.Token // buffered tokens (for DEDENT bursts)

	indentStack []int // stack of indentation levels
	bracketDepth int  // depth of (), [], {} nesting
	atLineStart  bool // whether we're at the start of a logical line
}

// New creates a new lexer for the given source bytes.
func New(src []byte) *Lexer {
	return &Lexer{
		src:         src,
		line:        1,
		col:         0,
		indentStack: []int{0},
		atLineStart: true,
	}
}

// Next returns the next token.
func (l *Lexer) Next() token.Token {
	// Return buffered tokens first (DEDENT bursts).
	if len(l.tokens) > 0 {
		tok := l.tokens[0]
		l.tokens = l.tokens[1:]
		return tok
	}

	// At line start (outside brackets), handle indentation.
	if l.atLineStart && l.bracketDepth == 0 {
		l.handleIndentation()
		if len(l.tokens) > 0 {
			tok := l.tokens[0]
			l.tokens = l.tokens[1:]
			return tok
		}
	}

	l.skipSpacesAndTabs()

	if l.pos >= len(l.src) {
		return l.eofTokens()
	}

	pos := l.curPos()
	ch := l.src[l.pos]

	// Skip comments.
	if ch == '#' {
		l.skipToEOL()
		return l.emitNewline(pos)
	}

	// Newline.
	if ch == '\n' {
		l.advance()
		if l.bracketDepth > 0 {
			// Inside brackets, newlines are ignored.
			return l.Next()
		}
		l.atLineStart = true
		return token.Token{Type: token.NEWLINE, Pos: pos}
	}

	// Carriage return.
	if ch == '\r' {
		l.advance()
		if l.pos < len(l.src) && l.src[l.pos] == '\n' {
			l.advance()
		}
		if l.bracketDepth > 0 {
			return l.Next()
		}
		l.atLineStart = true
		return token.Token{Type: token.NEWLINE, Pos: pos}
	}

	// Line continuation.
	if ch == '\\' && l.pos+1 < len(l.src) && (l.src[l.pos+1] == '\n' || l.src[l.pos+1] == '\r') {
		l.advance() // skip backslash
		if l.src[l.pos] == '\r' {
			l.advance()
		}
		if l.pos < len(l.src) && l.src[l.pos] == '\n' {
			l.advance()
		}
		return l.Next()
	}

	// String literals.
	if ch == '\'' || ch == '"' || ((ch == 'r' || ch == 'R' || ch == 'b' || ch == 'B' ||
		ch == 'f' || ch == 'F' || ch == 'u' || ch == 'U') && l.peekIsStringStart()) {
		return l.readString(pos)
	}

	// Numbers.
	if ch >= '0' && ch <= '9' {
		return l.readNumber(pos)
	}

	// Identifiers and keywords.
	if isIdentStart(ch) {
		return l.readIdent(pos)
	}

	// Operators and delimiters.
	return l.readOperator(pos)
}

func (l *Lexer) curPos() token.Pos {
	return token.Pos{Line: l.line, Col: l.col, Offset: l.pos}
}

func (l *Lexer) advance() {
	if l.pos < len(l.src) {
		if l.src[l.pos] == '\n' {
			l.line++
			l.col = 0
		} else {
			l.col++
		}
		l.pos++
	}
}

func (l *Lexer) peek() byte {
	if l.pos+1 < len(l.src) {
		return l.src[l.pos+1]
	}
	return 0
}

func (l *Lexer) skipSpacesAndTabs() {
	for l.pos < len(l.src) && (l.src[l.pos] == ' ' || l.src[l.pos] == '\t') {
		l.advance()
	}
}

func (l *Lexer) skipToEOL() {
	for l.pos < len(l.src) && l.src[l.pos] != '\n' && l.src[l.pos] != '\r' {
		l.advance()
	}
}

func (l *Lexer) emitNewline(pos token.Pos) token.Token {
	// After a comment, we need to consume the newline.
	if l.pos < len(l.src) {
		if l.src[l.pos] == '\r' {
			l.advance()
		}
		if l.pos < len(l.src) && l.src[l.pos] == '\n' {
			l.advance()
		}
	}
	if l.bracketDepth > 0 {
		return l.Next()
	}
	l.atLineStart = true
	return token.Token{Type: token.NEWLINE, Pos: pos}
}

func (l *Lexer) handleIndentation() {
	// Measure indentation of current line.
	indent := 0
	startPos := l.pos
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		if ch == ' ' {
			indent++
			l.advance()
		} else if ch == '\t' {
			indent += 8 - (indent % 8) // tab stops at multiples of 8
			l.advance()
		} else {
			break
		}
	}

	// Blank line or comment-only line: skip entirely.
	if l.pos >= len(l.src) || l.src[l.pos] == '\n' || l.src[l.pos] == '\r' || l.src[l.pos] == '#' {
		// Don't emit INDENT/DEDENT for blank/comment lines.
		_ = startPos
		return
	}

	l.atLineStart = false
	current := l.indentStack[len(l.indentStack)-1]

	if indent > current {
		l.indentStack = append(l.indentStack, indent)
		l.tokens = append(l.tokens, token.Token{Type: token.INDENT, Pos: l.curPos()})
	} else if indent < current {
		for len(l.indentStack) > 1 && l.indentStack[len(l.indentStack)-1] > indent {
			l.indentStack = l.indentStack[:len(l.indentStack)-1]
			l.tokens = append(l.tokens, token.Token{Type: token.DEDENT, Pos: l.curPos()})
		}
		if l.indentStack[len(l.indentStack)-1] != indent {
			l.tokens = append(l.tokens, token.Token{
				Type: token.ILLEGAL,
				Lit:  "inconsistent indentation",
				Pos:  l.curPos(),
			})
		}
	}
}

func (l *Lexer) eofTokens() token.Token {
	pos := l.curPos()
	// Emit DEDENT for each remaining indent level.
	for len(l.indentStack) > 1 {
		l.indentStack = l.indentStack[:len(l.indentStack)-1]
		l.tokens = append(l.tokens, token.Token{Type: token.DEDENT, Pos: pos})
	}
	l.tokens = append(l.tokens, token.Token{Type: token.EOF, Pos: pos})

	tok := l.tokens[0]
	l.tokens = l.tokens[1:]
	return tok
}

func (l *Lexer) readIdent(pos token.Pos) token.Token {
	start := l.pos
	for l.pos < len(l.src) && isIdentCont(l.src[l.pos]) {
		l.advance()
	}
	lit := string(l.src[start:l.pos])

	if kwType, isKw := token.LookupKeyword(lit); isKw {
		return token.Token{Type: kwType, Lit: lit, Pos: pos}
	}
	return token.Token{Type: token.NAME, Lit: lit, Pos: pos}
}

func (l *Lexer) readNumber(pos token.Pos) token.Token {
	start := l.pos
	// Handle 0x, 0o, 0b prefixes.
	if l.src[l.pos] == '0' && l.pos+1 < len(l.src) {
		next := l.src[l.pos+1]
		if next == 'x' || next == 'X' || next == 'o' || next == 'O' || next == 'b' || next == 'B' {
			l.advance()
			l.advance()
			for l.pos < len(l.src) && (isHexDigit(l.src[l.pos]) || l.src[l.pos] == '_') {
				l.advance()
			}
			return token.Token{Type: token.NUMBER, Lit: string(l.src[start:l.pos]), Pos: pos}
		}
	}
	for l.pos < len(l.src) && (isDigit(l.src[l.pos]) || l.src[l.pos] == '_') {
		l.advance()
	}
	// Decimal point.
	if l.pos < len(l.src) && l.src[l.pos] == '.' {
		l.advance()
		for l.pos < len(l.src) && (isDigit(l.src[l.pos]) || l.src[l.pos] == '_') {
			l.advance()
		}
	}
	// Exponent.
	if l.pos < len(l.src) && (l.src[l.pos] == 'e' || l.src[l.pos] == 'E') {
		l.advance()
		if l.pos < len(l.src) && (l.src[l.pos] == '+' || l.src[l.pos] == '-') {
			l.advance()
		}
		for l.pos < len(l.src) && (isDigit(l.src[l.pos]) || l.src[l.pos] == '_') {
			l.advance()
		}
	}
	// Complex suffix.
	if l.pos < len(l.src) && (l.src[l.pos] == 'j' || l.src[l.pos] == 'J') {
		l.advance()
	}
	return token.Token{Type: token.NUMBER, Lit: string(l.src[start:l.pos]), Pos: pos}
}

func (l *Lexer) peekIsStringStart() bool {
	// Check if current char is a string prefix followed by a quote.
	// Current char is already known to be r/R/b/B/f/F/u/U.
	i := l.pos + 1
	// Allow two-char prefixes: rb, br, rf, fr, etc.
	if i < len(l.src) {
		ch := l.src[i]
		if ch == 'r' || ch == 'R' || ch == 'b' || ch == 'B' || ch == 'f' || ch == 'F' {
			i++
		}
	}
	if i < len(l.src) {
		return l.src[i] == '\'' || l.src[i] == '"'
	}
	return false
}

func (l *Lexer) readString(pos token.Pos) token.Token {
	start := l.pos
	isFString := false

	// Skip prefixes.
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		if ch == 'f' || ch == 'F' {
			isFString = true
			l.advance()
		} else if ch == 'r' || ch == 'R' || ch == 'b' || ch == 'B' || ch == 'u' || ch == 'U' {
			l.advance()
		} else {
			break
		}
	}

	if l.pos >= len(l.src) {
		return token.Token{Type: token.ILLEGAL, Lit: "unexpected end in string prefix", Pos: pos}
	}

	quote := l.src[l.pos]
	triple := false

	// Check for triple quotes.
	if l.pos+2 < len(l.src) && l.src[l.pos+1] == quote && l.src[l.pos+2] == quote {
		triple = true
		l.advance()
		l.advance()
		l.advance()
	} else {
		l.advance()
	}

	braceDepth := 0
	for l.pos < len(l.src) {
		ch := l.src[l.pos]

		if isFString && ch == '{' && l.pos+1 < len(l.src) && l.src[l.pos+1] != '{' {
			braceDepth++
			l.advance()
			continue
		}
		if isFString && ch == '{' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '{' {
			l.advance()
			l.advance()
			continue
		}
		if isFString && ch == '}' && braceDepth > 0 {
			braceDepth--
			l.advance()
			continue
		}
		if isFString && ch == '}' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '}' {
			l.advance()
			l.advance()
			continue
		}

		if ch == '\\' {
			l.advance()
			if l.pos < len(l.src) {
				l.advance()
			}
			continue
		}

		if ch == quote && braceDepth == 0 {
			if triple {
				if l.pos+2 < len(l.src) && l.src[l.pos+1] == quote && l.src[l.pos+2] == quote {
					l.advance()
					l.advance()
					l.advance()
					return token.Token{Type: token.STRING, Lit: string(l.src[start:l.pos]), Pos: pos}
				}
				l.advance()
				continue
			}
			l.advance()
			return token.Token{Type: token.STRING, Lit: string(l.src[start:l.pos]), Pos: pos}
		}

		l.advance()
	}

	return token.Token{Type: token.ILLEGAL, Lit: "unterminated string", Pos: pos}
}

func (l *Lexer) readOperator(pos token.Pos) token.Token {
	ch := l.src[l.pos]
	l.advance()

	switch ch {
	case '(':
		l.bracketDepth++
		return token.Token{Type: token.LPAREN, Pos: pos}
	case ')':
		l.bracketDepth--
		return token.Token{Type: token.RPAREN, Pos: pos}
	case '[':
		l.bracketDepth++
		return token.Token{Type: token.LBRACK, Pos: pos}
	case ']':
		l.bracketDepth--
		return token.Token{Type: token.RBRACK, Pos: pos}
	case '{':
		l.bracketDepth++
		return token.Token{Type: token.LBRACE, Pos: pos}
	case '}':
		l.bracketDepth--
		return token.Token{Type: token.RBRACE, Pos: pos}
	case ':':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return token.Token{Type: token.WALRUS, Pos: pos}
		}
		return token.Token{Type: token.COLON, Pos: pos}
	case '=':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return token.Token{Type: token.EQ, Pos: pos}
		}
		return token.Token{Type: token.ASSIGN, Pos: pos}
	case '-':
		if l.pos < len(l.src) && l.src[l.pos] == '>' {
			l.advance()
			return token.Token{Type: token.ARROW, Pos: pos}
		}
		return token.Token{Type: token.MINUS, Pos: pos}
	case '+':
		return token.Token{Type: token.PLUS, Pos: pos}
	case '*':
		if l.pos < len(l.src) && l.src[l.pos] == '*' {
			l.advance()
			return token.Token{Type: token.DSTAR, Pos: pos}
		}
		return token.Token{Type: token.STAR, Pos: pos}
	case '/':
		if l.pos < len(l.src) && l.src[l.pos] == '/' {
			l.advance()
			return token.Token{Type: token.DSLASH, Pos: pos}
		}
		return token.Token{Type: token.SLASH, Pos: pos}
	case '.':
		// Check for ellipsis.
		if l.pos+1 < len(l.src) && l.src[l.pos] == '.' && l.src[l.pos+1] == '.' {
			l.advance()
			l.advance()
			return token.Token{Type: token.ELLIPSIS, Pos: pos}
		}
		// Check for float starting with dot (e.g., .5)
		if l.pos < len(l.src) && isDigit(l.src[l.pos]) {
			// Rewind and read as number — handle .5 style floats
			l.pos--
			l.col--
			return l.readDotNumber(pos)
		}
		return token.Token{Type: token.DOT, Pos: pos}
	case '|':
		return token.Token{Type: token.PIPE, Pos: pos}
	case '@':
		return token.Token{Type: token.AT, Pos: pos}
	case ',':
		return token.Token{Type: token.COMMA, Pos: pos}
	case ';':
		return token.Token{Type: token.SEMI, Pos: pos}
	case '~':
		return token.Token{Type: token.TILDE, Pos: pos}
	case '&':
		return token.Token{Type: token.AMPER, Pos: pos}
	case '^':
		return token.Token{Type: token.CARET, Pos: pos}
	case '%':
		return token.Token{Type: token.PERCENT, Pos: pos}
	case '!':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return token.Token{Type: token.NEQ, Pos: pos}
		}
		return token.Token{Type: token.ILLEGAL, Lit: "!", Pos: pos}
	case '<':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return token.Token{Type: token.LTE, Pos: pos}
		}
		if l.pos < len(l.src) && l.src[l.pos] == '<' {
			l.advance()
			return token.Token{Type: token.LSHIFT, Pos: pos}
		}
		return token.Token{Type: token.LT, Pos: pos}
	case '>':
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.advance()
			return token.Token{Type: token.GTE, Pos: pos}
		}
		if l.pos < len(l.src) && l.src[l.pos] == '>' {
			l.advance()
			return token.Token{Type: token.RSHIFT, Pos: pos}
		}
		return token.Token{Type: token.GT, Pos: pos}
	default:
		return token.Token{Type: token.ILLEGAL, Lit: string(ch), Pos: pos}
	}
}

func (l *Lexer) readDotNumber(pos token.Pos) token.Token {
	start := l.pos
	l.advance() // skip the dot
	for l.pos < len(l.src) && (isDigit(l.src[l.pos]) || l.src[l.pos] == '_') {
		l.advance()
	}
	if l.pos < len(l.src) && (l.src[l.pos] == 'e' || l.src[l.pos] == 'E') {
		l.advance()
		if l.pos < len(l.src) && (l.src[l.pos] == '+' || l.src[l.pos] == '-') {
			l.advance()
		}
		for l.pos < len(l.src) && (isDigit(l.src[l.pos]) || l.src[l.pos] == '_') {
			l.advance()
		}
	}
	return token.Token{Type: token.NUMBER, Lit: string(l.src[start:l.pos]), Pos: pos}
}

func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isIdentCont(ch byte) bool {
	return isIdentStart(ch) || (ch >= '0' && ch <= '9')
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isHexDigit(ch byte) bool {
	return isDigit(ch) || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd goopy && go test ./lexer/... -v
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add goopy/lexer/
git commit -m "feat(goopy): implement Python lexer with INDENT/DEDENT"
```

---

### Task 5: Lexer — indentation edge cases

**Files:**
- Modify: `goopy/lexer/lexer_test.go`

- [ ] **Step 1: Write indentation edge case tests**

Add to `goopy/lexer/lexer_test.go`:

```go
func TestNestedIndentation(t *testing.T) {
	src := "if True:\n    if True:\n        pass\n"
	expectTypes(t, src, []token.Type{
		token.IF, token.TRUE, token.COLON, token.NEWLINE,
		token.INDENT, token.IF, token.TRUE, token.COLON, token.NEWLINE,
		token.INDENT, token.PASS, token.NEWLINE,
		token.DEDENT, token.DEDENT, token.EOF,
	})
}

func TestMultipleDedent(t *testing.T) {
	src := "def a():\n    def b():\n        pass\nx = 1\n"
	expectTypes(t, src, []token.Type{
		token.DEF, token.NAME, token.LPAREN, token.RPAREN, token.COLON, token.NEWLINE,
		token.INDENT,
		token.DEF, token.NAME, token.LPAREN, token.RPAREN, token.COLON, token.NEWLINE,
		token.INDENT, token.PASS, token.NEWLINE,
		token.DEDENT, token.DEDENT,
		token.NAME, token.ASSIGN, token.NUMBER, token.NEWLINE,
		token.EOF,
	})
}

func TestBlankLinesIgnored(t *testing.T) {
	src := "def foo():\n\n    pass\n"
	expectTypes(t, src, []token.Type{
		token.DEF, token.NAME, token.LPAREN, token.RPAREN, token.COLON, token.NEWLINE,
		token.INDENT, token.PASS, token.NEWLINE,
		token.DEDENT, token.EOF,
	})
}

func TestCommentOnlyLinesIgnored(t *testing.T) {
	src := "def foo():\n    # a comment\n    pass\n"
	expectTypes(t, src, []token.Type{
		token.DEF, token.NAME, token.LPAREN, token.RPAREN, token.COLON, token.NEWLINE,
		token.INDENT, token.PASS, token.NEWLINE,
		token.DEDENT, token.EOF,
	})
}

func TestBracketsCancelNewlines(t *testing.T) {
	src := "x = (\n    1,\n    2\n)\n"
	expectTypes(t, src, []token.Type{
		token.NAME, token.ASSIGN, token.LPAREN,
		token.NUMBER, token.COMMA,
		token.NUMBER,
		token.RPAREN, token.NEWLINE,
		token.EOF,
	})
}

func TestExplicitLineContinuation(t *testing.T) {
	src := "x = 1 + \\\n    2\n"
	expectTypes(t, src, []token.Type{
		token.NAME, token.ASSIGN, token.NUMBER, token.PLUS,
		token.NUMBER, token.NEWLINE,
		token.EOF,
	})
}

func TestEOFDedents(t *testing.T) {
	src := "def foo():\n    pass"
	// No trailing newline — should still get DEDENT and EOF.
	tokens := tokenize(src)
	types := tokenTypes(tokens)
	// Should end with NEWLINE, DEDENT, EOF.
	last3 := types[len(types)-3:]
	if last3[0] != token.NEWLINE || last3[1] != token.DEDENT || last3[2] != token.EOF {
		t.Errorf("expected [...NEWLINE DEDENT EOF], got %v", last3)
	}
}
```

- [ ] **Step 2: Run tests and fix any failures**

```bash
cd goopy && go test ./lexer/... -v
```

Fix any failures. Common issues: blank line handling, EOF without trailing newline, comment-only lines emitting spurious INDENT/DEDENT.

- [ ] **Step 3: Commit**

```bash
git add goopy/lexer/
git commit -m "test(goopy): add lexer indentation edge case tests"
```

---

### Task 6: Lexer — string literal edge cases

**Files:**
- Modify: `goopy/lexer/lexer_test.go`

- [ ] **Step 1: Write string edge case tests**

Add to `goopy/lexer/lexer_test.go`:

```go
func TestTripleQuotedString(t *testing.T) {
	src := `x = """hello
world"""`+ "\n"
	tokens := tokenize(src)
	found := false
	for _, tok := range tokens {
		if tok.Type == token.STRING && tok.Lit == `"""hello\nworld"""` {
			found = true
		}
		// Also accept the raw form with actual newline.
		if tok.Type == token.STRING {
			found = true
		}
	}
	if !found {
		t.Error("no STRING token found for triple-quoted string")
	}
}

func TestSingleQuotedString(t *testing.T) {
	src := `x = 'hello'` + "\n"
	tokens := tokenize(src)
	found := false
	for _, tok := range tokens {
		if tok.Type == token.STRING && tok.Lit == `'hello'` {
			found = true
		}
	}
	if !found {
		t.Error("no STRING token for single-quoted string")
	}
}

func TestRawString(t *testing.T) {
	src := `x = r"raw\nstring"` + "\n"
	tokens := tokenize(src)
	found := false
	for _, tok := range tokens {
		if tok.Type == token.STRING {
			found = true
		}
	}
	if !found {
		t.Error("no STRING token for raw string")
	}
}

func TestFString(t *testing.T) {
	src := `x = f"hello {name}"` + "\n"
	tokens := tokenize(src)
	found := false
	for _, tok := range tokens {
		if tok.Type == token.STRING {
			found = true
		}
	}
	if !found {
		t.Error("no STRING token for f-string")
	}
}

func TestFStringNestedBraces(t *testing.T) {
	src := `x = f"result: {d['key']}"` + "\n"
	tokens := tokenize(src)
	strCount := 0
	for _, tok := range tokens {
		if tok.Type == token.STRING {
			strCount++
		}
	}
	// Should be exactly 1 STRING token (the whole f-string).
	if strCount != 1 {
		t.Errorf("expected 1 STRING token for f-string with nested braces, got %d", strCount)
	}
}

func TestByteString(t *testing.T) {
	src := `x = b"bytes"` + "\n"
	tokens := tokenize(src)
	found := false
	for _, tok := range tokens {
		if tok.Type == token.STRING {
			found = true
		}
	}
	if !found {
		t.Error("no STRING token for byte string")
	}
}

func TestStringEscapes(t *testing.T) {
	src := `x = "hello\"world"` + "\n"
	tokens := tokenize(src)
	found := false
	for _, tok := range tokens {
		if tok.Type == token.STRING {
			found = true
		}
	}
	if !found {
		t.Error("no STRING token for string with escaped quote")
	}
}
```

- [ ] **Step 2: Run tests and fix any failures**

```bash
cd goopy && go test ./lexer/... -v
```

- [ ] **Step 3: Commit**

```bash
git add goopy/lexer/
git commit -m "test(goopy): add lexer string literal edge case tests"
```

---

## Phase 3: Parser

### Task 7: Parser — function definitions

**Files:**
- Create: `goopy/parser/parser.go`
- Create: `goopy/parser/parser_test.go`

- [ ] **Step 1: Write parser tests for function definitions**

Create `goopy/parser/parser_test.go`:

```go
package parser

import (
	"testing"

	"github.com/pypx/goopy/ast"
)

func parse(src string) *ast.Module {
	p := New([]byte(src))
	return p.Parse()
}

func TestSimpleFunction(t *testing.T) {
	mod := parse("def foo():\n    pass\n")
	if len(mod.Body) != 1 {
		t.Fatalf("Body has %d stmts, want 1", len(mod.Body))
	}
	fn, ok := mod.Body[0].(*ast.FunctionDef)
	if !ok {
		t.Fatalf("Body[0] is %T, want *ast.FunctionDef", mod.Body[0])
	}
	if fn.Name != "foo" {
		t.Errorf("Name = %q, want %q", fn.Name, "foo")
	}
	if fn.Args == nil {
		t.Fatal("Args is nil")
	}
	if len(fn.Args.Args) != 0 {
		t.Errorf("Args.Args len = %d, want 0", len(fn.Args.Args))
	}
}

func TestFunctionWithParams(t *testing.T) {
	mod := parse("def foo(a, b, c):\n    pass\n")
	fn := mod.Body[0].(*ast.FunctionDef)
	if len(fn.Args.Args) != 3 {
		t.Fatalf("Args.Args len = %d, want 3", len(fn.Args.Args))
	}
	names := []string{"a", "b", "c"}
	for i, arg := range fn.Args.Args {
		if arg.Name != names[i] {
			t.Errorf("Args.Args[%d].Name = %q, want %q", i, arg.Name, names[i])
		}
	}
}

func TestFunctionWithAnnotations(t *testing.T) {
	mod := parse("def foo(x: int, y: str) -> bool:\n    pass\n")
	fn := mod.Body[0].(*ast.FunctionDef)
	if len(fn.Args.Args) != 2 {
		t.Fatalf("Args.Args len = %d, want 2", len(fn.Args.Args))
	}
	// x: int
	ann, ok := fn.Args.Args[0].Annotation.(*ast.Name)
	if !ok {
		t.Fatalf("Args[0].Annotation is %T, want *ast.Name", fn.Args.Args[0].Annotation)
	}
	if ann.ID != "int" {
		t.Errorf("Args[0] annotation = %q, want %q", ann.ID, "int")
	}
	// -> bool
	ret, ok := fn.Returns.(*ast.Name)
	if !ok {
		t.Fatalf("Returns is %T, want *ast.Name", fn.Returns)
	}
	if ret.ID != "bool" {
		t.Errorf("Returns = %q, want %q", ret.ID, "bool")
	}
}

func TestFunctionWithDefaults(t *testing.T) {
	mod := parse("def foo(x=1, y=2):\n    pass\n")
	fn := mod.Body[0].(*ast.FunctionDef)
	if len(fn.Args.Defaults) != 2 {
		t.Fatalf("Defaults len = %d, want 2", len(fn.Args.Defaults))
	}
}

func TestFunctionPosOnly(t *testing.T) {
	mod := parse("def foo(a, b, /, c):\n    pass\n")
	fn := mod.Body[0].(*ast.FunctionDef)
	if len(fn.Args.PosOnlyArgs) != 2 {
		t.Errorf("PosOnlyArgs len = %d, want 2", len(fn.Args.PosOnlyArgs))
	}
	if len(fn.Args.Args) != 1 {
		t.Errorf("Args len = %d, want 1", len(fn.Args.Args))
	}
}

func TestFunctionKwOnly(t *testing.T) {
	mod := parse("def foo(a, *, b, c):\n    pass\n")
	fn := mod.Body[0].(*ast.FunctionDef)
	if len(fn.Args.Args) != 1 {
		t.Errorf("Args len = %d, want 1", len(fn.Args.Args))
	}
	if len(fn.Args.KwOnlyArgs) != 2 {
		t.Errorf("KwOnlyArgs len = %d, want 2", len(fn.Args.KwOnlyArgs))
	}
}

func TestFunctionVarArgs(t *testing.T) {
	mod := parse("def foo(*args, **kwargs):\n    pass\n")
	fn := mod.Body[0].(*ast.FunctionDef)
	if fn.Args.VarArg == nil || fn.Args.VarArg.Name != "args" {
		t.Errorf("VarArg = %v, want args", fn.Args.VarArg)
	}
	if fn.Args.VarKwArg == nil || fn.Args.VarKwArg.Name != "kwargs" {
		t.Errorf("VarKwArg = %v, want kwargs", fn.Args.VarKwArg)
	}
}

func TestAsyncFunction(t *testing.T) {
	mod := parse("async def foo():\n    pass\n")
	fn := mod.Body[0].(*ast.FunctionDef)
	if !fn.IsAsync {
		t.Error("IsAsync = false, want true")
	}
}

func TestDocstring(t *testing.T) {
	mod := parse("def foo():\n    \"\"\"Hello world.\"\"\"\n    pass\n")
	fn := mod.Body[0].(*ast.FunctionDef)
	if len(fn.Body) < 1 {
		t.Fatal("Body is empty")
	}
	stmt, ok := fn.Body[0].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("Body[0] is %T, want *ast.ExprStmt", fn.Body[0])
	}
	c, ok := stmt.Value.(*ast.Constant)
	if !ok {
		t.Fatalf("ExprStmt.Value is %T, want *ast.Constant", stmt.Value)
	}
	if c.Kind != "str" {
		t.Errorf("Constant.Kind = %q, want %q", c.Kind, "str")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd goopy && go test ./parser/... -v
```

Expected: compilation errors.

- [ ] **Step 3: Implement the parser**

Create `goopy/parser/parser.go`. This is the largest single file — the recursive-descent parser. The implementation should include:

- `Parser` struct holding the lexer, current token, and error list
- `Parse()` → `*ast.Module` entry point
- `parseStmt()` dispatching to `parseFunctionDef`, `parseClassDef`, `parseImport`, `parseAssign`, etc.
- `parseFunctionDef()` handling `async def NAME(args) -> return: block`
- `parseArguments()` handling all five parameter kinds with annotations and defaults
- `parseExpr()` recursive expression parser for annotations, defaults, decorators
- `parseBlock()` consuming `NEWLINE INDENT stmt+ DEDENT`
- Error recovery: `synchronize()` skipping to next `NEWLINE`/`DEDENT`

```go
package parser

import (
	"github.com/pypx/goopy/ast"
	"github.com/pypx/goopy/lexer"
	"github.com/pypx/goopy/token"
)

// Parser is a recursive-descent parser for Python source code.
type Parser struct {
	lex    *lexer.Lexer
	tok    token.Token
	errors []Error
}

// Error represents a parse error with position.
type Error struct {
	Pos token.Pos
	Msg string
}

// New creates a new parser for the given source.
func New(src []byte) *Parser {
	p := &Parser{lex: lexer.New(src)}
	p.next() // prime the first token
	return p
}

// Parse parses the entire source and returns a Module AST.
func (p *Parser) Parse() *ast.Module {
	mod := &ast.Module{}
	for p.tok.Type != token.EOF {
		if p.tok.Type == token.NEWLINE {
			p.next()
			continue
		}
		stmt := p.parseStmt()
		if stmt != nil {
			mod.Body = append(mod.Body, stmt)
		}
	}
	return mod
}

// Errors returns all parse errors encountered.
func (p *Parser) Errors() []Error {
	return p.errors
}

func (p *Parser) next() {
	p.tok = p.lex.Next()
}

func (p *Parser) expect(t token.Type) token.Token {
	tok := p.tok
	if p.tok.Type != t {
		p.error("expected %v, got %v", t, p.tok.Type)
	} else {
		p.next()
	}
	return tok
}

func (p *Parser) error(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	p.errors = append(p.errors, Error{Pos: p.tok.Pos, Msg: msg})
}

func (p *Parser) parseStmt() ast.Stmt {
	switch p.tok.Type {
	case token.AT:
		return p.parseDecorated()
	case token.DEF:
		return p.parseFunctionDef(nil)
	case token.ASYNC:
		return p.parseAsyncDef()
	case token.CLASS:
		return p.parseClassDef(nil)
	case token.IMPORT:
		return p.parseImport()
	case token.FROM:
		return p.parseImportFrom()
	case token.IF:
		return p.parseIf()
	case token.TYPE:
		return p.parseTypeAlias()
	case token.FOR, token.WHILE, token.TRY, token.WITH, token.MATCH:
		return p.parsePassThrough()
	default:
		return p.parseSimpleStmt()
	}
}

// The full implementation continues with each parse method...
// parseFunctionDef, parseClassDef, parseArguments, parseExpr, etc.
// See the spec for grammar rules that each method implements.
```

**Note to implementer:** The parser is ~400-600 lines. Implement each grammar rule from the spec. Key methods:

- `parseFunctionDef(decorators []ast.Expr)` — parse `def NAME(args) -> ret: block`
- `parseClassDef(decorators []ast.Expr)` — parse `class NAME(bases): block`
- `parseArguments()` — handle `a, b, /, c, *args, d, **kwargs` with annotations and defaults
- `parseExpr()` — handle `atom trailer* ('|' atom trailer*)*` for union types
- `parseAtom()` — NAME, NUMBER, STRING, None/True/False, `(expr)`, `[exprs]`, `{exprs}`, `...`
- `parseTrailer(expr)` — `.name`, `(args)`, `[slice]`
- `parseBlock()` — `NEWLINE INDENT stmt+ DEDENT`
- `parseDecorated()` — `('@' expr NEWLINE)+ (funcdef | classdef)`
- `parsePassThrough()` — shallow parse of for/while/try/with/match, recurse into blocks

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd goopy && go test ./parser/... -v
```

- [ ] **Step 5: Commit**

```bash
git add goopy/parser/
git commit -m "feat(goopy): implement recursive-descent Python parser"
```

---

### Task 8: Parser — class definitions

**Files:**
- Modify: `goopy/parser/parser_test.go`
- Modify: `goopy/parser/parser.go` (if needed)

- [ ] **Step 1: Write class definition tests**

Add to `goopy/parser/parser_test.go`:

```go
func TestSimpleClass(t *testing.T) {
	mod := parse("class Foo:\n    pass\n")
	if len(mod.Body) != 1 {
		t.Fatalf("Body has %d stmts, want 1", len(mod.Body))
	}
	cls, ok := mod.Body[0].(*ast.ClassDef)
	if !ok {
		t.Fatalf("Body[0] is %T, want *ast.ClassDef", mod.Body[0])
	}
	if cls.Name != "Foo" {
		t.Errorf("Name = %q, want %q", cls.Name, "Foo")
	}
}

func TestClassWithBases(t *testing.T) {
	mod := parse("class Foo(Bar, Baz):\n    pass\n")
	cls := mod.Body[0].(*ast.ClassDef)
	if len(cls.Bases) != 2 {
		t.Fatalf("Bases len = %d, want 2", len(cls.Bases))
	}
	b0, ok := cls.Bases[0].(*ast.Name)
	if !ok || b0.ID != "Bar" {
		t.Errorf("Bases[0] = %v, want Bar", cls.Bases[0])
	}
}

func TestClassWithMetaclass(t *testing.T) {
	mod := parse("class Foo(metaclass=ABCMeta):\n    pass\n")
	cls := mod.Body[0].(*ast.ClassDef)
	if len(cls.Keywords) != 1 {
		t.Fatalf("Keywords len = %d, want 1", len(cls.Keywords))
	}
	if cls.Keywords[0].Arg != "metaclass" {
		t.Errorf("Keywords[0].Arg = %q, want %q", cls.Keywords[0].Arg, "metaclass")
	}
}

func TestClassWithMethods(t *testing.T) {
	src := "class Foo:\n    def bar(self):\n        pass\n    def baz(self):\n        pass\n"
	mod := parse(src)
	cls := mod.Body[0].(*ast.ClassDef)
	methods := 0
	for _, stmt := range cls.Body {
		if _, ok := stmt.(*ast.FunctionDef); ok {
			methods++
		}
	}
	if methods != 2 {
		t.Errorf("found %d methods, want 2", methods)
	}
}

func TestDecoratedClass(t *testing.T) {
	src := "@dataclass\nclass Foo:\n    x: int = 0\n"
	mod := parse(src)
	cls := mod.Body[0].(*ast.ClassDef)
	if len(cls.Decorators) != 1 {
		t.Fatalf("Decorators len = %d, want 1", len(cls.Decorators))
	}
	dec, ok := cls.Decorators[0].(*ast.Name)
	if !ok || dec.ID != "dataclass" {
		t.Errorf("Decorators[0] = %v, want dataclass", cls.Decorators[0])
	}
}

func TestClassWithDocstring(t *testing.T) {
	src := "class Foo:\n    \"\"\"A foo class.\"\"\"\n    pass\n"
	mod := parse(src)
	cls := mod.Body[0].(*ast.ClassDef)
	if len(cls.Body) < 1 {
		t.Fatal("Body is empty")
	}
	stmt, ok := cls.Body[0].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("Body[0] is %T, want *ast.ExprStmt", cls.Body[0])
	}
	c, ok := stmt.Value.(*ast.Constant)
	if !ok || c.Kind != "str" {
		t.Error("expected string constant docstring")
	}
}
```

- [ ] **Step 2: Run tests**

```bash
cd goopy && go test ./parser/... -v
```

Fix any failures.

- [ ] **Step 3: Commit**

```bash
git add goopy/parser/
git commit -m "test(goopy): add parser class definition tests"
```

---

### Task 9: Parser — imports, assignments, type aliases

**Files:**
- Modify: `goopy/parser/parser_test.go`
- Modify: `goopy/parser/parser.go` (if needed)

- [ ] **Step 1: Write import and assignment tests**

Add to `goopy/parser/parser_test.go`:

```go
func TestImport(t *testing.T) {
	mod := parse("import os\n")
	imp, ok := mod.Body[0].(*ast.Import)
	if !ok {
		t.Fatalf("Body[0] is %T, want *ast.Import", mod.Body[0])
	}
	if len(imp.Names) != 1 || imp.Names[0].Name != "os" {
		t.Errorf("Names = %v, want [os]", imp.Names)
	}
}

func TestImportAs(t *testing.T) {
	mod := parse("import numpy as np\n")
	imp := mod.Body[0].(*ast.Import)
	if imp.Names[0].Name != "numpy" || imp.Names[0].Alias != "np" {
		t.Errorf("Names[0] = %v, want numpy as np", imp.Names[0])
	}
}

func TestImportFrom(t *testing.T) {
	mod := parse("from os.path import join, exists\n")
	imp, ok := mod.Body[0].(*ast.ImportFrom)
	if !ok {
		t.Fatalf("Body[0] is %T, want *ast.ImportFrom", mod.Body[0])
	}
	if imp.Module != "os.path" {
		t.Errorf("Module = %q, want %q", imp.Module, "os.path")
	}
	if len(imp.Names) != 2 {
		t.Fatalf("Names len = %d, want 2", len(imp.Names))
	}
}

func TestRelativeImport(t *testing.T) {
	mod := parse("from ..utils import helper\n")
	imp := mod.Body[0].(*ast.ImportFrom)
	if imp.Level != 2 {
		t.Errorf("Level = %d, want 2", imp.Level)
	}
	if imp.Module != "utils" {
		t.Errorf("Module = %q, want %q", imp.Module, "utils")
	}
}

func TestAssign(t *testing.T) {
	mod := parse("x = 42\n")
	asgn, ok := mod.Body[0].(*ast.Assign)
	if !ok {
		t.Fatalf("Body[0] is %T, want *ast.Assign", mod.Body[0])
	}
	if len(asgn.Targets) != 1 {
		t.Fatalf("Targets len = %d, want 1", len(asgn.Targets))
	}
}

func TestAnnotatedAssign(t *testing.T) {
	mod := parse("x: int = 42\n")
	ann, ok := mod.Body[0].(*ast.AnnAssign)
	if !ok {
		t.Fatalf("Body[0] is %T, want *ast.AnnAssign", mod.Body[0])
	}
	target, ok := ann.Target.(*ast.Name)
	if !ok || target.ID != "x" {
		t.Errorf("Target = %v, want x", ann.Target)
	}
	annType, ok := ann.Annotation.(*ast.Name)
	if !ok || annType.ID != "int" {
		t.Errorf("Annotation = %v, want int", ann.Annotation)
	}
}

func TestAllAssign(t *testing.T) {
	mod := parse(`__all__ = ["foo", "bar"]` + "\n")
	asgn, ok := mod.Body[0].(*ast.Assign)
	if !ok {
		t.Fatalf("Body[0] is %T, want *ast.Assign", mod.Body[0])
	}
	target, ok := asgn.Targets[0].(*ast.Name)
	if !ok || target.ID != "__all__" {
		t.Errorf("Target = %v, want __all__", asgn.Targets[0])
	}
	lst, ok := asgn.Value.(*ast.List)
	if !ok {
		t.Fatalf("Value is %T, want *ast.List", asgn.Value)
	}
	if len(lst.Elts) != 2 {
		t.Errorf("List len = %d, want 2", len(lst.Elts))
	}
}

func TestTypeAlias(t *testing.T) {
	mod := parse("type Vector = list[float]\n")
	ta, ok := mod.Body[0].(*ast.TypeAliasDef)
	if !ok {
		t.Fatalf("Body[0] is %T, want *ast.TypeAliasDef", mod.Body[0])
	}
	if ta.Name != "Vector" {
		t.Errorf("Name = %q, want %q", ta.Name, "Vector")
	}
}
```

- [ ] **Step 2: Run tests**

```bash
cd goopy && go test ./parser/... -v
```

- [ ] **Step 3: Commit**

```bash
git add goopy/parser/
git commit -m "test(goopy): add parser import, assignment, type alias tests"
```

---

### Task 10: Parser — complex type annotations

**Files:**
- Modify: `goopy/parser/parser_test.go`

- [ ] **Step 1: Write type annotation tests**

Add to `goopy/parser/parser_test.go`:

```go
func TestGenericAnnotation(t *testing.T) {
	mod := parse("def foo(x: dict[str, int]):\n    pass\n")
	fn := mod.Body[0].(*ast.FunctionDef)
	ann := fn.Args.Args[0].Annotation
	sub, ok := ann.(*ast.Subscript)
	if !ok {
		t.Fatalf("Annotation is %T, want *ast.Subscript", ann)
	}
	name, ok := sub.Value.(*ast.Name)
	if !ok || name.ID != "dict" {
		t.Errorf("Subscript.Value = %v, want dict", sub.Value)
	}
}

func TestUnionAnnotation(t *testing.T) {
	mod := parse("def foo(x: int | str):\n    pass\n")
	fn := mod.Body[0].(*ast.FunctionDef)
	ann := fn.Args.Args[0].Annotation
	binop, ok := ann.(*ast.BinOp)
	if !ok {
		t.Fatalf("Annotation is %T, want *ast.BinOp", ann)
	}
	if binop.Op != "|" {
		t.Errorf("Op = %q, want %q", binop.Op, "|")
	}
}

func TestDottedAnnotation(t *testing.T) {
	mod := parse("def foo(x: os.PathLike):\n    pass\n")
	fn := mod.Body[0].(*ast.FunctionDef)
	ann := fn.Args.Args[0].Annotation
	attr, ok := ann.(*ast.Attribute)
	if !ok {
		t.Fatalf("Annotation is %T, want *ast.Attribute", ann)
	}
	if attr.Attr != "PathLike" {
		t.Errorf("Attr = %q, want %q", attr.Attr, "PathLike")
	}
}

func TestNestedGenericAnnotation(t *testing.T) {
	mod := parse("def foo(x: Callable[[int, str], Awaitable[dict[str, list[int]]]]):\n    pass\n")
	fn := mod.Body[0].(*ast.FunctionDef)
	ann := fn.Args.Args[0].Annotation
	// Just verify it parsed without error and is a Subscript.
	_, ok := ann.(*ast.Subscript)
	if !ok {
		t.Fatalf("Annotation is %T, want *ast.Subscript", ann)
	}
}

func TestOptionalAnnotation(t *testing.T) {
	mod := parse("def foo(x: Optional[str] = None):\n    pass\n")
	fn := mod.Body[0].(*ast.FunctionDef)
	ann := fn.Args.Args[0].Annotation
	sub, ok := ann.(*ast.Subscript)
	if !ok {
		t.Fatalf("Annotation is %T, want *ast.Subscript", ann)
	}
	name, ok := sub.Value.(*ast.Name)
	if !ok || name.ID != "Optional" {
		t.Errorf("Subscript.Value = %v, want Optional", sub.Value)
	}
	if len(fn.Args.Defaults) != 1 {
		t.Errorf("Defaults len = %d, want 1", len(fn.Args.Defaults))
	}
}
```

- [ ] **Step 2: Run tests**

```bash
cd goopy && go test ./parser/... -v
```

- [ ] **Step 3: Commit**

```bash
git add goopy/parser/
git commit -m "test(goopy): add parser complex type annotation tests"
```

---

### Task 11: Parser — error recovery

**Files:**
- Modify: `goopy/parser/parser_test.go`
- Modify: `goopy/parser/parser.go` (if needed)

- [ ] **Step 1: Write error recovery tests**

Add to `goopy/parser/parser_test.go`:

```go
func TestErrorRecovery(t *testing.T) {
	// Malformed first function, valid second function.
	src := "def broken(:\n    pass\ndef good():\n    pass\n"
	p := New([]byte(src))
	mod := p.Parse()
	if len(p.Errors()) == 0 {
		t.Error("expected parse errors for malformed def")
	}
	// Should still find the good function.
	found := false
	for _, stmt := range mod.Body {
		if fn, ok := stmt.(*ast.FunctionDef); ok && fn.Name == "good" {
			found = true
		}
	}
	if !found {
		t.Error("error recovery failed: 'good' function not found")
	}
}

func TestErrorRecoveryClass(t *testing.T) {
	src := "class Bad(:\n    pass\nclass Good:\n    pass\n"
	p := New([]byte(src))
	mod := p.Parse()
	if len(p.Errors()) == 0 {
		t.Error("expected parse errors for malformed class")
	}
	found := false
	for _, stmt := range mod.Body {
		if cls, ok := stmt.(*ast.ClassDef); ok && cls.Name == "Good" {
			found = true
		}
	}
	if !found {
		t.Error("error recovery failed: 'Good' class not found")
	}
}

func TestNoErrorsOnValidCode(t *testing.T) {
	src := "def foo(x: int = 1, *, y: str) -> bool:\n    \"\"\"Docstring.\"\"\"\n    pass\n"
	p := New([]byte(src))
	_ = p.Parse()
	if len(p.Errors()) != 0 {
		t.Errorf("unexpected errors: %v", p.Errors())
	}
}
```

- [ ] **Step 2: Run tests**

```bash
cd goopy && go test ./parser/... -v
```

- [ ] **Step 3: Commit**

```bash
git add goopy/parser/
git commit -m "test(goopy): add parser error recovery tests"
```

---

## Phase 4: Docstring Parsing

### Task 12: Docstring style detection and plain parser

**Files:**
- Create: `goopy/docstring/docstring.go`
- Create: `goopy/docstring/docstring_test.go`

- [ ] **Step 1: Write detection and plain parser tests**

Create `goopy/docstring/docstring_test.go`:

```go
package docstring

import (
	"testing"

	"github.com/pypx/goopy/model"
)

func TestDetectGoogle(t *testing.T) {
	raw := "Short summary.\n\nArgs:\n    x: Description.\n"
	if got := Detect(raw); got != model.StyleGoogle {
		t.Errorf("Detect() = %q, want %q", got, model.StyleGoogle)
	}
}

func TestDetectNumPy(t *testing.T) {
	raw := "Short summary.\n\nParameters\n----------\nx : int\n    Description.\n"
	if got := Detect(raw); got != model.StyleNumPy {
		t.Errorf("Detect() = %q, want %q", got, model.StyleNumPy)
	}
}

func TestDetectSphinx(t *testing.T) {
	raw := "Short summary.\n\n:param x: Description.\n:type x: int\n"
	if got := Detect(raw); got != model.StyleSphinx {
		t.Errorf("Detect() = %q, want %q", got, model.StyleSphinx)
	}
}

func TestDetectPlain(t *testing.T) {
	raw := "Just a simple description."
	if got := Detect(raw); got != model.StylePlain {
		t.Errorf("Detect() = %q, want %q", got, model.StylePlain)
	}
}

func TestParsePlain(t *testing.T) {
	raw := "Short summary.\n\nExtended description\nspanning two lines."
	doc := Parse(raw)
	if doc.Summary != "Short summary." {
		t.Errorf("Summary = %q, want %q", doc.Summary, "Short summary.")
	}
	if doc.Description != "Extended description\nspanning two lines." {
		t.Errorf("Description = %q", doc.Description)
	}
	if doc.Style != model.StylePlain {
		t.Errorf("Style = %q, want %q", doc.Style, model.StylePlain)
	}
	if doc.Raw != raw {
		t.Errorf("Raw mismatch")
	}
}

func TestParseSummaryOnly(t *testing.T) {
	raw := "Just a summary."
	doc := Parse(raw)
	if doc.Summary != "Just a summary." {
		t.Errorf("Summary = %q", doc.Summary)
	}
	if doc.Description != "" {
		t.Errorf("Description should be empty, got %q", doc.Description)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd goopy && go test ./docstring/... -v
```

- [ ] **Step 3: Implement detection and plain parser**

Create `goopy/docstring/docstring.go`:

```go
package docstring

import (
	"strings"

	"github.com/pypx/goopy/model"
)

// Detect identifies the docstring style.
func Detect(raw string) model.DocstringStyle {
	if containsSphinxFields(raw) {
		return model.StyleSphinx
	}
	if containsNumpySections(raw) {
		return model.StyleNumPy
	}
	if containsGoogleSections(raw) {
		return model.StyleGoogle
	}
	return model.StylePlain
}

// Parse detects the style and dispatches to the appropriate parser.
func Parse(raw string) *model.Docstring {
	style := Detect(raw)
	switch style {
	case model.StyleGoogle:
		return parseGoogle(raw)
	case model.StyleNumPy:
		return parseNumPy(raw)
	case model.StyleSphinx:
		return parseSphinx(raw)
	default:
		return parsePlain(raw)
	}
}

func parsePlain(raw string) *model.Docstring {
	summary, description := splitSummaryDescription(raw)
	return &model.Docstring{
		Raw:         raw,
		Summary:     summary,
		Description: description,
		Style:       model.StylePlain,
	}
}

func splitSummaryDescription(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	parts := strings.SplitN(raw, "\n\n", 2)
	summary := strings.TrimSpace(parts[0])
	description := ""
	if len(parts) > 1 {
		description = strings.TrimSpace(parts[1])
	}
	return summary, description
}

func containsSphinxFields(raw string) bool {
	return strings.Contains(raw, ":param ") || strings.Contains(raw, ":type ") ||
		strings.Contains(raw, ":returns:") || strings.Contains(raw, ":rtype:")
}

func containsNumpySections(raw string) bool {
	lines := strings.Split(raw, "\n")
	for i := 0; i < len(lines)-1; i++ {
		trimmed := strings.TrimSpace(lines[i])
		nextTrimmed := strings.TrimSpace(lines[i+1])
		if trimmed != "" && len(nextTrimmed) >= 3 && allDashes(nextTrimmed) {
			return true
		}
	}
	return false
}

func containsGoogleSections(raw string) bool {
	sections := []string{"Args:", "Arguments:", "Returns:", "Return:", "Raises:",
		"Yields:", "Yield:", "Examples:", "Example:", "Note:", "Notes:",
		"Attributes:", "Todo:", "References:"}
	for _, s := range sections {
		if strings.Contains(raw, s) {
			return true
		}
	}
	return false
}

func allDashes(s string) bool {
	for _, c := range s {
		if c != '-' {
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd goopy && go test ./docstring/... -v
```

- [ ] **Step 5: Commit**

```bash
git add goopy/docstring/
git commit -m "feat(goopy): add docstring style detection and plain parser"
```

---

### Task 13: Google-style docstring parser

**Files:**
- Create: `goopy/docstring/google.go`
- Modify: `goopy/docstring/docstring_test.go`

- [ ] **Step 1: Write Google-style parser tests**

Add to `goopy/docstring/docstring_test.go`:

```go
func TestParseGoogleBasic(t *testing.T) {
	raw := `Short summary.

Extended description here.

Args:
    x: The first param.
    y (int): The second param.

Returns:
    str: The result.

Raises:
    ValueError: If x is negative.

Examples:
    >>> foo(1, 2)
    3

Note:
    Something important.`

	doc := Parse(raw)
	if doc.Style != model.StyleGoogle {
		t.Errorf("Style = %q, want %q", doc.Style, model.StyleGoogle)
	}
	if doc.Summary != "Short summary." {
		t.Errorf("Summary = %q", doc.Summary)
	}
	if doc.Description != "Extended description here." {
		t.Errorf("Description = %q", doc.Description)
	}
	if len(doc.Params) != 2 {
		t.Fatalf("Params len = %d, want 2", len(doc.Params))
	}
	if doc.Params[0].Name != "x" {
		t.Errorf("Params[0].Name = %q, want %q", doc.Params[0].Name, "x")
	}
	if doc.Params[0].Description != "The first param." {
		t.Errorf("Params[0].Description = %q", doc.Params[0].Description)
	}
	if doc.Params[1].Name != "y" || doc.Params[1].Type != "int" {
		t.Errorf("Params[1] = %v", doc.Params[1])
	}
	if len(doc.Returns) != 1 {
		t.Fatalf("Returns len = %d, want 1", len(doc.Returns))
	}
	if doc.Returns[0].Type != "str" {
		t.Errorf("Returns[0].Type = %q, want %q", doc.Returns[0].Type, "str")
	}
	if len(doc.Raises) != 1 {
		t.Fatalf("Raises len = %d, want 1", len(doc.Raises))
	}
	if doc.Raises[0].Type != "ValueError" {
		t.Errorf("Raises[0].Type = %q", doc.Raises[0].Type)
	}
	if len(doc.Examples) != 1 {
		t.Fatalf("Examples len = %d, want 1", len(doc.Examples))
	}
	if doc.Notes != "Something important." {
		t.Errorf("Notes = %q", doc.Notes)
	}
}

func TestParseGoogleMultiLineParam(t *testing.T) {
	raw := `Summary.

Args:
    x: A very long description
        that spans multiple lines.
    y: Short.`

	doc := Parse(raw)
	if len(doc.Params) != 2 {
		t.Fatalf("Params len = %d, want 2", len(doc.Params))
	}
	if !strings.Contains(doc.Params[0].Description, "spans multiple lines") {
		t.Errorf("Params[0].Description = %q, want multi-line content", doc.Params[0].Description)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd goopy && go test ./docstring/... -v
```

- [ ] **Step 3: Implement Google-style parser**

Create `goopy/docstring/google.go`:

```go
package docstring

import (
	"strings"

	"github.com/pypx/goopy/model"
)

// parseGoogle parses a Google-style docstring.
func parseGoogle(raw string) *model.Docstring {
	summary, rest := splitSummaryDescription(raw)

	doc := &model.Docstring{
		Raw:     raw,
		Summary: summary,
		Style:   model.StyleGoogle,
	}

	// Split rest into sections.
	sections := splitGoogleSections(rest)

	for name, body := range sections {
		switch name {
		case "description":
			doc.Description = body
		case "args", "arguments", "parameters", "params":
			doc.Params = parseGoogleParams(body)
		case "returns", "return":
			doc.Returns = parseGoogleReturns(body)
		case "raises", "raise":
			doc.Raises = parseGoogleRaises(body)
		case "examples", "example":
			doc.Examples = []string{strings.TrimSpace(body)}
		case "note", "notes":
			doc.Notes = strings.TrimSpace(body)
		}
	}

	return doc
}

// splitGoogleSections splits docstring body into named sections.
// Returns a map of lowercase section name -> body text.
// Content before the first section header is keyed as "description".
func splitGoogleSections(text string) map[string]string {
	sections := map[string]string{}
	if text == "" {
		return sections
	}

	sectionHeaders := []string{
		"Args:", "Arguments:", "Parameters:", "Params:",
		"Returns:", "Return:",
		"Raises:", "Raise:",
		"Yields:", "Yield:",
		"Examples:", "Example:",
		"Note:", "Notes:",
		"Attributes:",
		"Todo:", "References:",
	}

	lines := strings.Split(text, "\n")
	currentSection := "description"
	var currentBody []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		foundHeader := ""
		for _, h := range sectionHeaders {
			if trimmed == h {
				foundHeader = strings.ToLower(strings.TrimSuffix(h, ":"))
				break
			}
		}

		if foundHeader != "" {
			// Save previous section.
			if body := strings.TrimSpace(strings.Join(currentBody, "\n")); body != "" {
				sections[currentSection] = body
			}
			currentSection = foundHeader
			currentBody = nil
		} else {
			currentBody = append(currentBody, line)
		}
	}

	// Save final section.
	if body := strings.TrimSpace(strings.Join(currentBody, "\n")); body != "" {
		sections[currentSection] = body
	}

	return sections
}

// parseGoogleParams parses the body of an Args/Parameters section.
func parseGoogleParams(body string) []model.DocParam {
	var params []model.DocParam
	lines := strings.Split(body, "\n")

	var current *model.DocParam
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Count leading spaces to determine if this is a new param or continuation.
		indent := len(line) - len(strings.TrimLeft(line, " \t"))

		if indent <= 4 && strings.Contains(trimmed, ":") {
			// New parameter.
			if current != nil {
				params = append(params, *current)
			}
			current = &model.DocParam{}
			colonIdx := strings.Index(trimmed, ":")
			namepart := strings.TrimSpace(trimmed[:colonIdx])
			desc := strings.TrimSpace(trimmed[colonIdx+1:])

			// Check for type in parens: name (type)
			if parenIdx := strings.Index(namepart, "("); parenIdx != -1 {
				current.Name = strings.TrimSpace(namepart[:parenIdx])
				current.Type = strings.TrimSpace(strings.TrimSuffix(namepart[parenIdx+1:], ")"))
			} else {
				current.Name = namepart
			}
			current.Description = desc
		} else if current != nil {
			// Continuation of previous param description.
			current.Description += " " + trimmed
		}
	}
	if current != nil {
		params = append(params, *current)
	}
	return params
}

// parseGoogleReturns parses the body of a Returns section.
func parseGoogleReturns(body string) []model.DocReturn {
	var returns []model.DocReturn
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return returns
	}

	// Format: "type: description" or just "description"
	if colonIdx := strings.Index(trimmed, ":"); colonIdx != -1 {
		typePart := strings.TrimSpace(trimmed[:colonIdx])
		desc := strings.TrimSpace(trimmed[colonIdx+1:])
		// Heuristic: if typePart looks like a type name (no spaces, starts with upper or is a builtin)
		if !strings.Contains(typePart, " ") {
			returns = append(returns, model.DocReturn{Type: typePart, Description: desc})
		} else {
			returns = append(returns, model.DocReturn{Description: trimmed})
		}
	} else {
		returns = append(returns, model.DocReturn{Description: trimmed})
	}
	return returns
}

// parseGoogleRaises parses the body of a Raises section.
func parseGoogleRaises(body string) []model.DocRaises {
	var raises []model.DocRaises
	lines := strings.Split(body, "\n")

	var current *model.DocRaises
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " \t"))

		if indent <= 4 && strings.Contains(trimmed, ":") {
			if current != nil {
				raises = append(raises, *current)
			}
			colonIdx := strings.Index(trimmed, ":")
			current = &model.DocRaises{
				Type:        strings.TrimSpace(trimmed[:colonIdx]),
				Description: strings.TrimSpace(trimmed[colonIdx+1:]),
			}
		} else if current != nil {
			current.Description += " " + trimmed
		}
	}
	if current != nil {
		raises = append(raises, *current)
	}
	return raises
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd goopy && go test ./docstring/... -v
```

- [ ] **Step 5: Commit**

```bash
git add goopy/docstring/
git commit -m "feat(goopy): add Google-style docstring parser"
```

---

### Task 14: NumPy-style docstring parser

**Files:**
- Create: `goopy/docstring/numpy.go`
- Modify: `goopy/docstring/docstring_test.go`

- [ ] **Step 1: Write NumPy-style parser tests**

Add to `goopy/docstring/docstring_test.go`:

```go
func TestParseNumPyBasic(t *testing.T) {
	raw := `Short summary.

Extended description.

Parameters
----------
x : int
    The first param.
y : str, optional
    The second param.

Returns
-------
bool
    The result.

Raises
------
ValueError
    If x is negative.

Notes
-----
Something important.

Examples
--------
>>> foo(1, 2)
3`

	doc := Parse(raw)
	if doc.Style != model.StyleNumPy {
		t.Errorf("Style = %q, want %q", doc.Style, model.StyleNumPy)
	}
	if doc.Summary != "Short summary." {
		t.Errorf("Summary = %q", doc.Summary)
	}
	if len(doc.Params) != 2 {
		t.Fatalf("Params len = %d, want 2", len(doc.Params))
	}
	if doc.Params[0].Name != "x" || doc.Params[0].Type != "int" {
		t.Errorf("Params[0] = %v", doc.Params[0])
	}
	if doc.Params[1].Name != "y" || doc.Params[1].Type != "str, optional" {
		t.Errorf("Params[1] = %v", doc.Params[1])
	}
	if len(doc.Returns) != 1 || doc.Returns[0].Type != "bool" {
		t.Errorf("Returns = %v", doc.Returns)
	}
	if len(doc.Raises) != 1 || doc.Raises[0].Type != "ValueError" {
		t.Errorf("Raises = %v", doc.Raises)
	}
}
```

- [ ] **Step 2: Implement NumPy-style parser**

Create `goopy/docstring/numpy.go`. The NumPy format uses section headers with underline dashes. Parameters use `name : type` format on the header line and indented description on following lines.

- [ ] **Step 3: Run tests to verify they pass**

```bash
cd goopy && go test ./docstring/... -v
```

- [ ] **Step 4: Commit**

```bash
git add goopy/docstring/
git commit -m "feat(goopy): add NumPy-style docstring parser"
```

---

### Task 15: Sphinx/reST-style docstring parser

**Files:**
- Create: `goopy/docstring/sphinx.go`
- Modify: `goopy/docstring/docstring_test.go`

- [ ] **Step 1: Write Sphinx-style parser tests**

Add to `goopy/docstring/docstring_test.go`:

```go
func TestParseSphinxBasic(t *testing.T) {
	raw := `Short summary.

Extended description.

:param x: The first param.
:type x: int
:param y: The second param.
:type y: str
:returns: The result.
:rtype: bool
:raises ValueError: If x is negative.`

	doc := Parse(raw)
	if doc.Style != model.StyleSphinx {
		t.Errorf("Style = %q, want %q", doc.Style, model.StyleSphinx)
	}
	if len(doc.Params) != 2 {
		t.Fatalf("Params len = %d, want 2", len(doc.Params))
	}
	if doc.Params[0].Name != "x" || doc.Params[0].Type != "int" {
		t.Errorf("Params[0] = %v", doc.Params[0])
	}
	if doc.Params[0].Description != "The first param." {
		t.Errorf("Params[0].Description = %q", doc.Params[0].Description)
	}
	if len(doc.Returns) != 1 || doc.Returns[0].Type != "bool" {
		t.Errorf("Returns = %v", doc.Returns)
	}
	if len(doc.Raises) != 1 || doc.Raises[0].Type != "ValueError" {
		t.Errorf("Raises = %v", doc.Raises)
	}
}

func TestParseSphinxInlineType(t *testing.T) {
	// Some Sphinx docs put the type inline: :param int x: description
	raw := `:param int x: The first param.`
	doc := Parse(raw)
	if len(doc.Params) != 1 {
		t.Fatalf("Params len = %d, want 1", len(doc.Params))
	}
	if doc.Params[0].Name != "x" || doc.Params[0].Type != "int" {
		t.Errorf("Params[0] = %v", doc.Params[0])
	}
}
```

- [ ] **Step 2: Implement Sphinx-style parser**

Create `goopy/docstring/sphinx.go`. Parse `:param name:`, `:type name:`, `:returns:`, `:rtype:`, `:raises Type:` field syntax.

- [ ] **Step 3: Run tests to verify they pass**

```bash
cd goopy && go test ./docstring/... -v
```

- [ ] **Step 4: Commit**

```bash
git add goopy/docstring/
git commit -m "feat(goopy): add Sphinx/reST-style docstring parser"
```

---

## Phase 5: Extractor

### Task 16: Extractor — basic function and class extraction

**Files:**
- Create: `goopy/extractor/extractor.go`
- Create: `goopy/extractor/extractor_test.go`

- [ ] **Step 1: Write basic extraction tests**

Create `goopy/extractor/extractor_test.go`:

```go
package extractor

import (
	"testing"

	"github.com/pypx/goopy/model"
	"github.com/pypx/goopy/parser"
)

func extract(src string) *model.Module {
	p := parser.New([]byte(src))
	mod := p.Parse()
	e := New()
	return e.ExtractModule("testmod", mod)
}

func TestExtractSimpleFunction(t *testing.T) {
	src := `def hello(name: str) -> str:
    """Say hello."""
    return f"Hello, {name}"
`
	mod := extract(src)
	if len(mod.Functions) != 1 {
		t.Fatalf("Functions len = %d, want 1", len(mod.Functions))
	}
	fn := mod.Functions[0]
	if fn.Name != "hello" {
		t.Errorf("Name = %q, want %q", fn.Name, "hello")
	}
	if fn.Docstring == nil || fn.Docstring.Summary != "Say hello." {
		t.Errorf("Docstring = %v", fn.Docstring)
	}
	if len(fn.Parameters) != 1 {
		t.Fatalf("Parameters len = %d, want 1", len(fn.Parameters))
	}
	if fn.Parameters[0].Name != "name" {
		t.Errorf("Parameters[0].Name = %q", fn.Parameters[0].Name)
	}
	if fn.Parameters[0].Type == nil || fn.Parameters[0].Type.Name != "str" {
		t.Errorf("Parameters[0].Type = %v", fn.Parameters[0].Type)
	}
	if fn.Returns == nil || fn.Returns.Type == nil || fn.Returns.Type.Name != "str" {
		t.Errorf("Returns = %v", fn.Returns)
	}
}

func TestExtractClass(t *testing.T) {
	src := `class Greeter:
    """A greeter class."""

    def __init__(self, name: str):
        self.name = name

    def greet(self) -> str:
        """Say hello."""
        return f"Hello, {self.name}"
`
	mod := extract(src)
	if len(mod.Classes) != 1 {
		t.Fatalf("Classes len = %d, want 1", len(mod.Classes))
	}
	cls := mod.Classes[0]
	if cls.Name != "Greeter" {
		t.Errorf("Name = %q", cls.Name)
	}
	if cls.Docstring == nil || cls.Docstring.Summary != "A greeter class." {
		t.Errorf("Docstring = %v", cls.Docstring)
	}
	// Parameters should come from __init__ (minus self).
	if len(cls.Parameters) != 1 || cls.Parameters[0].Name != "name" {
		t.Errorf("Parameters = %v", cls.Parameters)
	}
	// greet should be a method.
	if len(cls.Methods) != 1 || cls.Methods[0].Name != "greet" {
		t.Errorf("Methods = %v", cls.Methods)
	}
}

func TestExtractException(t *testing.T) {
	src := `class MyError(ValueError):
    """Custom error."""
    pass
`
	mod := extract(src)
	if len(mod.Exceptions) != 1 {
		t.Fatalf("Exceptions len = %d, want 1", len(mod.Exceptions))
	}
	if mod.Exceptions[0].Name != "MyError" {
		t.Errorf("Name = %q", mod.Exceptions[0].Name)
	}
	if len(mod.Classes) != 0 {
		t.Errorf("Classes should be empty, got %d", len(mod.Classes))
	}
}

func TestSkipPrivate(t *testing.T) {
	src := `def _private():
    pass

def public():
    pass
`
	mod := extract(src)
	if len(mod.Functions) != 1 || mod.Functions[0].Name != "public" {
		t.Errorf("Functions = %v", mod.Functions)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd goopy && go test ./extractor/... -v
```

- [ ] **Step 3: Implement the extractor**

Create `goopy/extractor/extractor.go`:

```go
package extractor

import (
	"strings"

	goopyast "github.com/pypx/goopy/ast"
	"github.com/pypx/goopy/docstring"
	"github.com/pypx/goopy/model"
)

// Extractor walks an AST and produces a documentation model.
type Extractor struct {
	source []byte // original source for raw text extraction
}

// New creates a new Extractor.
func New() *Extractor {
	return &Extractor{}
}

// ExtractModule extracts documentation from a parsed module AST.
func (e *Extractor) ExtractModule(name string, mod *goopyast.Module) *model.Module {
	result := &model.Module{Name: name}

	// Extract module docstring.
	if len(mod.Body) > 0 {
		if ds := extractDocstring(mod.Body[0]); ds != "" {
			result.Docstring = ds
		}
	}

	// Extract __all__ if present.
	exports := extractAllExports(mod.Body)
	result.Exports = exports

	for _, stmt := range mod.Body {
		switch s := stmt.(type) {
		case *goopyast.FunctionDef:
			if isPublic(s.Name, exports) {
				fn := e.extractFunction(s)
				result.Functions = append(result.Functions, fn)
			}
		case *goopyast.ClassDef:
			if isPublic(s.Name, exports) {
				cls := e.extractClass(s)
				if isExceptionClass(s) {
					result.Exceptions = append(result.Exceptions, cls)
				} else {
					result.Classes = append(result.Classes, cls)
				}
			}
		case *goopyast.AnnAssign:
			attr := e.extractAttribute(s)
			if attr != nil && isPublic(attr.Name, exports) {
				result.Constants = append(result.Constants, *attr)
			}
		case *goopyast.TypeAliasDef:
			if isPublic(s.Name, exports) {
				result.TypeAliases = append(result.TypeAliases, e.extractTypeAlias(s))
			}
		}
	}

	return result
}

func (e *Extractor) extractFunction(fn *goopyast.FunctionDef) model.Function {
	result := model.Function{
		Name:       fn.Name,
		IsAsync:    fn.IsAsync,
		Decorators: extractDecoratorNames(fn.Decorators),
	}

	// Semantic flags from decorators.
	for _, dec := range result.Decorators {
		switch dec {
		case "property":
			result.IsProperty = true
		case "staticmethod":
			result.IsStatic = true
		case "classmethod":
			result.IsClassMethod = true
		case "abstractmethod":
			result.IsAbstract = true
		}
	}

	// Parameters.
	if fn.Args != nil {
		result.Parameters = e.extractParameters(fn.Args)
	}

	// Return type.
	if fn.Returns != nil {
		result.Returns = &model.TypeRef{
			Type: exprToTypeExpr(fn.Returns),
		}
	}

	// Docstring.
	if len(fn.Body) > 0 {
		if ds := extractDocstring(fn.Body[0]); ds != "" {
			parsed := docstring.Parse(ds)
			result.Docstring = parsed
			// Merge docstring param descriptions into parameters.
			mergeParamDescriptions(result.Parameters, parsed.Params)
			// Merge docstring return description.
			if result.Returns != nil && len(parsed.Returns) > 0 {
				result.Returns.Description = parsed.Returns[0].Description
			}
		}
	}

	// Build signature string.
	result.Signature = buildSignature(fn, result.Parameters, result.Returns)

	return result
}

func (e *Extractor) extractClass(cls *goopyast.ClassDef) model.Class {
	result := model.Class{
		Name:       cls.Name,
		Bases:      extractBaseNames(cls.Bases),
		Decorators: extractDecoratorNames(cls.Decorators),
	}

	for _, dec := range result.Decorators {
		switch dec {
		case "dataclass":
			result.IsDataclass = true
		case "abstractmethod", "ABCMeta":
			result.IsAbstract = true
		}
	}

	// Walk class body.
	for _, stmt := range cls.Body {
		switch s := stmt.(type) {
		case *goopyast.FunctionDef:
			fn := e.extractFunction(s)
			if s.Name == "__init__" {
				// Hoist __init__ params as class params (skip self).
				if len(fn.Parameters) > 0 && fn.Parameters[0].Name == "self" {
					result.Parameters = fn.Parameters[1:]
				} else {
					result.Parameters = fn.Parameters
				}
			} else if !strings.HasPrefix(s.Name, "_") {
				if fn.IsProperty {
					result.Properties = append(result.Properties, fn)
				} else {
					result.Methods = append(result.Methods, fn)
				}
			}
		case *goopyast.AnnAssign:
			attr := e.extractAttribute(s)
			if attr != nil {
				result.Attributes = append(result.Attributes, *attr)
			}
		}
	}

	// Class docstring.
	if len(cls.Body) > 0 {
		if ds := extractDocstring(cls.Body[0]); ds != "" {
			result.Docstring = docstring.Parse(ds)
		}
	}

	// Build signature.
	result.Signature = buildClassSignature(cls.Name, result.Parameters, result.Bases)

	return result
}

// Helper functions: extractDocstring, isPublic, isExceptionClass,
// exprToTypeExpr, extractParameters, mergeParamDescriptions,
// buildSignature, buildClassSignature, extractDecoratorNames,
// extractBaseNames, extractAllExports, extractAttribute, extractTypeAlias
// ... (implement each helper)
```

**Note to implementer:** The extractor needs these helper functions fully implemented:
- `extractDocstring(stmt)` — check if stmt is `*ast.ExprStmt` containing `*ast.Constant{Kind: "str"}`
- `isPublic(name, exports)` — if exports is non-nil, check membership; otherwise check `!strings.HasPrefix(name, "_")`
- `isExceptionClass(cls)` — check base class names for Exception/Error/Warning/BaseException
- `exprToTypeExpr(expr)` — convert AST expression to `model.TypeExpr` tree
- `extractParameters(args)` — convert `ast.Arguments` to `[]model.Parameter` with correct `ParamKind`
- `buildSignature(fn, params, returns)` — build human-readable `"def name(params) -> ret"` string
- `mergeParamDescriptions(params, docParams)` — match by name, fill in descriptions

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd goopy && go test ./extractor/... -v
```

- [ ] **Step 5: Commit**

```bash
git add goopy/extractor/
git commit -m "feat(goopy): implement AST-to-model extractor"
```

---

### Task 17: Extractor — decorators, properties, __all__

**Files:**
- Modify: `goopy/extractor/extractor_test.go`
- Modify: `goopy/extractor/extractor.go` (if needed)

- [ ] **Step 1: Write advanced extraction tests**

Add to `goopy/extractor/extractor_test.go`:

```go
func TestExtractDecorators(t *testing.T) {
	src := `class Foo:
    @staticmethod
    def bar():
        pass

    @classmethod
    def baz(cls):
        pass

    @abstractmethod
    def qux(self):
        pass
`
	mod := extract(src)
	cls := mod.Classes[0]
	for _, m := range cls.Methods {
		switch m.Name {
		case "bar":
			if !m.IsStatic {
				t.Error("bar should be static")
			}
		case "baz":
			if !m.IsClassMethod {
				t.Error("baz should be classmethod")
			}
		case "qux":
			if !m.IsAbstract {
				t.Error("qux should be abstract")
			}
		}
	}
}

func TestExtractProperty(t *testing.T) {
	src := `class Foo:
    @property
    def name(self) -> str:
        """The name."""
        return self._name
`
	mod := extract(src)
	cls := mod.Classes[0]
	if len(cls.Properties) != 1 {
		t.Fatalf("Properties len = %d, want 1", len(cls.Properties))
	}
	if cls.Properties[0].Name != "name" || !cls.Properties[0].IsProperty {
		t.Errorf("Property = %v", cls.Properties[0])
	}
}

func TestExtractAllExports(t *testing.T) {
	src := `__all__ = ["public_func"]

def public_func():
    pass

def _private_func():
    pass

def other_func():
    pass
`
	mod := extract(src)
	if len(mod.Functions) != 1 || mod.Functions[0].Name != "public_func" {
		t.Errorf("Functions = %v (expected only public_func)", mod.Functions)
	}
	if len(mod.Exports) != 1 || mod.Exports[0] != "public_func" {
		t.Errorf("Exports = %v", mod.Exports)
	}
}

func TestExtractAsyncFunction(t *testing.T) {
	src := `async def fetch(url: str) -> bytes:
    """Fetch URL."""
    pass
`
	mod := extract(src)
	if len(mod.Functions) != 1 || !mod.Functions[0].IsAsync {
		t.Errorf("expected async function")
	}
}

func TestExtractDataclass(t *testing.T) {
	src := `@dataclass
class Point:
    x: float
    y: float
`
	mod := extract(src)
	cls := mod.Classes[0]
	if !cls.IsDataclass {
		t.Error("expected IsDataclass = true")
	}
	if len(cls.Attributes) != 2 {
		t.Errorf("Attributes len = %d, want 2", len(cls.Attributes))
	}
}
```

- [ ] **Step 2: Run tests**

```bash
cd goopy && go test ./extractor/... -v
```

- [ ] **Step 3: Commit**

```bash
git add goopy/extractor/
git commit -m "test(goopy): add extractor decorator, property, __all__ tests"
```

---

### Task 18: Extractor — TypeExpr conversion

**Files:**
- Modify: `goopy/extractor/extractor_test.go`
- Modify: `goopy/extractor/extractor.go` (if needed)

- [ ] **Step 1: Write TypeExpr conversion tests**

Add to `goopy/extractor/extractor_test.go`:

```go
func TestTypeExprSimple(t *testing.T) {
	src := `def foo(x: str) -> int:
    pass
`
	mod := extract(src)
	fn := mod.Functions[0]
	if fn.Parameters[0].Type == nil || fn.Parameters[0].Type.Kind != model.TypeName {
		t.Errorf("param type = %v, want TypeName", fn.Parameters[0].Type)
	}
	if fn.Parameters[0].Type.Name != "str" {
		t.Errorf("param type name = %q, want str", fn.Parameters[0].Type.Name)
	}
}

func TestTypeExprGeneric(t *testing.T) {
	src := `def foo(x: list[int]) -> dict[str, int]:
    pass
`
	mod := extract(src)
	fn := mod.Functions[0]
	paramType := fn.Parameters[0].Type
	if paramType.Kind != model.TypeGeneric {
		t.Fatalf("param type kind = %q, want generic", paramType.Kind)
	}
	if paramType.Name != "list" {
		t.Errorf("param type name = %q, want list", paramType.Name)
	}
	if len(paramType.Args) != 1 || paramType.Args[0].Name != "int" {
		t.Errorf("param type args = %v", paramType.Args)
	}
}

func TestTypeExprUnion(t *testing.T) {
	src := `def foo(x: int | str | None) -> bool:
    pass
`
	mod := extract(src)
	fn := mod.Functions[0]
	paramType := fn.Parameters[0].Type
	if paramType.Kind != model.TypeUnion {
		t.Fatalf("param type kind = %q, want union", paramType.Kind)
	}
}

func TestTypeExprOptional(t *testing.T) {
	src := `def foo(x: Optional[str]) -> bool:
    pass
`
	mod := extract(src)
	fn := mod.Functions[0]
	paramType := fn.Parameters[0].Type
	if paramType.Kind != model.TypeOptional {
		t.Fatalf("param type kind = %q, want optional", paramType.Kind)
	}
}

func TestTypeExprCallable(t *testing.T) {
	src := `def foo(cb: Callable[[int, str], bool]) -> None:
    pass
`
	mod := extract(src)
	fn := mod.Functions[0]
	paramType := fn.Parameters[0].Type
	if paramType.Kind != model.TypeCallable {
		t.Fatalf("param type kind = %q, want callable", paramType.Kind)
	}
}

func TestTypeExprNone(t *testing.T) {
	src := `def foo() -> None:
    pass
`
	mod := extract(src)
	fn := mod.Functions[0]
	if fn.Returns == nil || fn.Returns.Type.Kind != model.TypeNone {
		t.Errorf("Returns = %v, want TypeNone", fn.Returns)
	}
}
```

- [ ] **Step 2: Run tests**

```bash
cd goopy && go test ./extractor/... -v
```

- [ ] **Step 3: Commit**

```bash
git add goopy/extractor/
git commit -m "test(goopy): add TypeExpr conversion tests"
```

---

## Phase 6: Public API & Wheel Integration

### Task 19: Public API entry point

**Files:**
- Create: `goopy/goopy.go`
- Create: `goopy/goopy_test.go`

- [ ] **Step 1: Write public API tests**

Create `goopy/goopy_test.go`:

```go
package goopy

import (
	"testing"
)

func TestExtractModule(t *testing.T) {
	src := []byte(`"""My module."""

def hello(name: str) -> str:
    """Say hello.

    Args:
        name: The name to greet.

    Returns:
        str: A greeting string.
    """
    return f"Hello, {name}"

class Greeter:
    """A greeter."""

    def __init__(self, prefix: str = "Hello"):
        self.prefix = prefix

    def greet(self, name: str) -> str:
        """Greet someone."""
        return f"{self.prefix}, {name}"

class BadInputError(ValueError):
    """Raised on bad input."""
    pass
`)
	mod := ExtractModule("mymod", src)
	if mod.Name != "mymod" {
		t.Errorf("Name = %q", mod.Name)
	}
	if len(mod.Functions) != 1 {
		t.Fatalf("Functions len = %d, want 1", len(mod.Functions))
	}
	if mod.Functions[0].Name != "hello" {
		t.Errorf("Functions[0].Name = %q", mod.Functions[0].Name)
	}
	if len(mod.Classes) != 1 {
		t.Fatalf("Classes len = %d, want 1", len(mod.Classes))
	}
	if mod.Classes[0].Name != "Greeter" {
		t.Errorf("Classes[0].Name = %q", mod.Classes[0].Name)
	}
	if len(mod.Exceptions) != 1 {
		t.Fatalf("Exceptions len = %d, want 1", len(mod.Exceptions))
	}
	if mod.Exceptions[0].Name != "BadInputError" {
		t.Errorf("Exceptions[0].Name = %q", mod.Exceptions[0].Name)
	}
}

func TestExtractModuleErrors(t *testing.T) {
	// Malformed source should not panic, should return partial results.
	src := []byte("def broken(:\n    pass\ndef good():\n    pass\n")
	mod := ExtractModule("test", src)
	if mod == nil {
		t.Fatal("ExtractModule returned nil")
	}
	// Should have at least the good function.
	found := false
	for _, fn := range mod.Functions {
		if fn.Name == "good" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'good' function in partial results")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd goopy && go test ./... -v -count=1
```

- [ ] **Step 3: Implement public API**

Create `goopy/goopy.go`:

```go
// Package goopy extracts structured API documentation from Python source code.
package goopy

import (
	"github.com/pypx/goopy/extractor"
	"github.com/pypx/goopy/model"
	"github.com/pypx/goopy/parser"
)

// ExtractModule parses Python source code and returns structured documentation.
func ExtractModule(name string, src []byte) *model.Module {
	p := parser.New(src)
	mod := p.Parse()
	e := extractor.New()
	return e.ExtractModule(name, mod)
}

// ExtractPackage parses multiple Python source files and returns a Package.
// files maps relative paths (e.g., "mypackage/__init__.py") to source bytes.
func ExtractPackage(name string, files map[string][]byte, topLevelPkgs []string) *model.Package {
	pkg := &model.Package{Name: name}

	for _, pkgName := range topLevelPkgs {
		// Find all .py files belonging to this package.
		var modules []*model.Module
		for path, src := range files {
			if belongsToPackage(path, pkgName) {
				modName := pathToModuleName(path)
				mod := ExtractModule(modName, src)
				if hasContent(mod) {
					modules = append(modules, mod)
				}
			}
		}
		pkg.Modules = append(pkg.Modules, modules...)
	}

	return pkg
}

func belongsToPackage(path, pkgName string) bool {
	// Implementation: check if path starts with pkgName/
	return len(path) > len(pkgName) && path[:len(pkgName)] == pkgName &&
		(path[len(pkgName)] == '/')
}

func pathToModuleName(path string) string {
	// Convert "package/submod.py" to "package.submod"
	// Convert "package/__init__.py" to "package"
	name := path
	if len(name) > 3 && name[len(name)-3:] == ".py" {
		name = name[:len(name)-3]
	}
	if len(name) > 9 && name[len(name)-9:] == "/__init__" {
		name = name[:len(name)-9]
	}
	result := make([]byte, len(name))
	for i := range name {
		if name[i] == '/' {
			result[i] = '.'
		} else {
			result[i] = name[i]
		}
	}
	return string(result)
}

func hasContent(mod *model.Module) bool {
	return len(mod.Functions) > 0 || len(mod.Classes) > 0 ||
		len(mod.Exceptions) > 0 || len(mod.Constants) > 0 ||
		len(mod.TypeAliases) > 0
}
```

- [ ] **Step 4: Run all tests**

```bash
cd goopy && go test ./... -v -count=1
```

- [ ] **Step 5: Commit**

```bash
git add goopy/goopy.go goopy/goopy_test.go
git commit -m "feat(goopy): add public API entry point"
```

---

### Task 20: Wheel download and extraction

**Files:**
- Create: `goopy/wheel/wheel.go`
- Create: `goopy/wheel/wheel_test.go`

- [ ] **Step 1: Write wheel tests**

Create `goopy/wheel/wheel_test.go`:

```go
package wheel

import (
	"testing"
)

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"requests", "requests"},
		{"My-Package", "my_package"},
		{"my.package", "my_package"},
		{"My-Cool.Package", "my_cool_package"},
		{"UPPER", "upper"},
	}
	for _, tt := range tests {
		got := NormalizeName(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFindTopLevelFromMetadata(t *testing.T) {
	content := "requests\n"
	got := parseTopLevelTxt(content)
	if len(got) != 1 || got[0] != "requests" {
		t.Errorf("parseTopLevelTxt(%q) = %v, want [requests]", content, got)
	}
}

func TestFindTopLevelMultiple(t *testing.T) {
	content := "pkg1\npkg2\n"
	got := parseTopLevelTxt(content)
	if len(got) != 2 {
		t.Errorf("parseTopLevelTxt() = %v, want 2 entries", got)
	}
}

func TestSelectWheelURL(t *testing.T) {
	urls := []WheelFile{
		{Filename: "pkg-1.0-cp39-linux.whl", URL: "https://example.com/linux.whl"},
		{Filename: "pkg-1.0-py3-none-any.whl", URL: "https://example.com/any.whl"},
	}
	got := selectWheel(urls)
	if got != "https://example.com/any.whl" {
		t.Errorf("selectWheel() = %q, want any.whl URL", got)
	}
}

func TestSelectWheelFallback(t *testing.T) {
	urls := []WheelFile{
		{Filename: "pkg-1.0-cp39-linux.whl", URL: "https://example.com/linux.whl"},
	}
	got := selectWheel(urls)
	if got != "https://example.com/linux.whl" {
		t.Errorf("selectWheel() = %q, want fallback URL", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd goopy && go test ./wheel/... -v
```

- [ ] **Step 3: Implement wheel package**

Create `goopy/wheel/wheel.go`:

```go
package wheel

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	DefaultMaxSize = 50 * 1024 * 1024 // 50 MB
	pypiBaseURL    = "https://pypi.org"
)

// WheelFile represents a wheel distribution file from PyPI.
type WheelFile struct {
	Filename string
	URL      string
	Size     int64
}

// WheelContents holds the extracted .py files from a wheel.
type WheelContents struct {
	Files        map[string][]byte // relative path -> source bytes
	TopLevelPkgs []string
}

// Source fetches and extracts Python source files from PyPI wheels.
type Source struct {
	HTTPClient *http.Client
	MaxSize    int64
}

// NewSource creates a new wheel Source with defaults.
func NewSource() *Source {
	return &Source{
		HTTPClient: http.DefaultClient,
		MaxSize:    DefaultMaxSize,
	}
}

// Fetch downloads a wheel for the given package and version, returning .py file contents.
func (s *Source) Fetch(ctx context.Context, name, version string) (*WheelContents, error) {
	// 1. Get wheel URL from PyPI JSON API.
	wheels, err := s.fetchWheelURLs(ctx, name, version)
	if err != nil {
		return nil, fmt.Errorf("fetching wheel URLs: %w", err)
	}
	if len(wheels) == 0 {
		return nil, fmt.Errorf("no wheel files found for %s==%s", name, version)
	}

	url := selectWheel(wheels)

	// 2. Check size.
	size, err := s.headSize(ctx, url)
	if err == nil && size > s.MaxSize {
		return nil, fmt.Errorf("wheel too large: %d bytes (max %d)", size, s.MaxSize)
	}

	// 3. Download.
	data, err := s.download(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("downloading wheel: %w", err)
	}

	// 4. Extract .py files.
	return extractPyFiles(data, name)
}

func (s *Source) fetchWheelURLs(ctx context.Context, name, version string) ([]WheelFile, error) {
	url := fmt.Sprintf("%s/pypi/%s/%s/json", pypiBaseURL, name, version)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("PyPI returned %d", resp.StatusCode)
	}

	var pypiResp struct {
		URLs []struct {
			Filename    string `json:"filename"`
			URL         string `json:"url"`
			Size        int64  `json:"size"`
			PackageType string `json:"packagetype"`
		} `json:"urls"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pypiResp); err != nil {
		return nil, err
	}

	var wheels []WheelFile
	for _, u := range pypiResp.URLs {
		if u.PackageType == "bdist_wheel" {
			wheels = append(wheels, WheelFile{
				Filename: u.Filename,
				URL:      u.URL,
				Size:     u.Size,
			})
		}
	}
	return wheels, nil
}

func selectWheel(wheels []WheelFile) string {
	// Prefer none-any wheels.
	for _, w := range wheels {
		if strings.Contains(w.Filename, "none-any") {
			return w.URL
		}
	}
	return wheels[0].URL
}

func (s *Source) headSize(ctx context.Context, url string) (int64, error) {
	req, _ := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.ContentLength, nil
}

func (s *Source) download(ctx context.Context, url string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func extractPyFiles(data []byte, pkgName string) (*WheelContents, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("opening zip: %w", err)
	}

	contents := &WheelContents{
		Files: make(map[string][]byte),
	}

	for _, f := range r.File {
		if strings.Contains(f.Name, "__pycache__") {
			continue
		}

		// Read top_level.txt for package names.
		if strings.HasSuffix(f.Name, ".dist-info/top_level.txt") {
			rc, err := f.Open()
			if err == nil {
				data, _ := io.ReadAll(rc)
				rc.Close()
				contents.TopLevelPkgs = parseTopLevelTxt(string(data))
			}
			continue
		}

		if strings.HasSuffix(f.Name, ".py") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			src, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}
			contents.Files[f.Name] = src
		}
	}

	// Fallback: infer top-level packages from directory structure.
	if len(contents.TopLevelPkgs) == 0 {
		contents.TopLevelPkgs = inferTopLevel(contents.Files, pkgName)
	}

	return contents, nil
}

func parseTopLevelTxt(content string) []string {
	var pkgs []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			pkgs = append(pkgs, line)
		}
	}
	return pkgs
}

func inferTopLevel(files map[string][]byte, pkgName string) []string {
	seen := map[string]bool{}
	for path := range files {
		parts := strings.SplitN(path, "/", 2)
		dir := parts[0]
		if strings.HasSuffix(dir, ".dist-info") || strings.HasSuffix(dir, ".data") {
			continue
		}
		seen[dir] = true
	}

	if len(seen) > 0 {
		var result []string
		for dir := range seen {
			result = append(result, dir)
		}
		return result
	}

	return []string{NormalizeName(pkgName)}
}

// NormalizeName converts a PyPI package name to a Python import name.
func NormalizeName(name string) string {
	result := strings.ToLower(name)
	result = strings.ReplaceAll(result, "-", "_")
	result = strings.ReplaceAll(result, ".", "_")
	return result
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd goopy && go test ./wheel/... -v
```

- [ ] **Step 5: Commit**

```bash
git add goopy/wheel/
git commit -m "feat(goopy): add wheel download and extraction"
```

---

### Task 21: ExtractFromPyPI convenience function

**Files:**
- Modify: `goopy/goopy.go`
- Modify: `goopy/goopy_test.go`

- [ ] **Step 1: Write integration test**

Add to `goopy/goopy_test.go`:

```go
func TestExtractFromPyPI_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	pkg, err := ExtractFromPyPI(ctx, "click", "8.1.8")
	if err != nil {
		t.Fatalf("ExtractFromPyPI: %v", err)
	}
	if pkg.Name != "click" {
		t.Errorf("Name = %q, want %q", pkg.Name, "click")
	}
	if len(pkg.Modules) == 0 {
		t.Error("no modules extracted")
	}

	// click should have some well-known functions.
	totalFuncs := 0
	for _, mod := range pkg.Modules {
		totalFuncs += len(mod.Functions)
	}
	if totalFuncs == 0 {
		t.Error("no functions found in click")
	}
}
```

- [ ] **Step 2: Implement ExtractFromPyPI**

Add to `goopy/goopy.go`:

```go
import (
	"context"

	"github.com/pypx/goopy/extractor"
	"github.com/pypx/goopy/model"
	"github.com/pypx/goopy/parser"
	"github.com/pypx/goopy/wheel"
)

// ExtractFromPyPI downloads a package wheel from PyPI and extracts documentation.
func ExtractFromPyPI(ctx context.Context, name, version string) (*model.Package, error) {
	src := wheel.NewSource()
	contents, err := src.Fetch(ctx, name, version)
	if err != nil {
		return nil, err
	}
	return ExtractPackage(name, contents.Files, contents.TopLevelPkgs), nil
}
```

- [ ] **Step 3: Run integration test**

```bash
cd goopy && go test ./... -v -run TestExtractFromPyPI_Integration -count=1
```

This downloads the actual click wheel from PyPI and parses it.

- [ ] **Step 4: Commit**

```bash
git add goopy/goopy.go goopy/goopy_test.go
git commit -m "feat(goopy): add ExtractFromPyPI convenience function"
```

---

## Phase 7: Ecosystem Tests

### Task 22: Ecosystem test runner

**Files:**
- Create: `goopy/ecosystem/packages.json`
- Create: `goopy/ecosystem/ecosystem_test.go`

- [ ] **Step 1: Create package list**

Create `goopy/ecosystem/packages.json`:

```json
{
  "packages": [
    {"name": "click", "version": "8.1.8"},
    {"name": "requests", "version": "2.32.3"},
    {"name": "flask", "version": "3.1.0"},
    {"name": "httpx", "version": "0.28.1"},
    {"name": "pydantic", "version": "2.10.6"},
    {"name": "fastapi", "version": "0.115.12"},
    {"name": "rich", "version": "13.9.4"},
    {"name": "typer", "version": "0.15.2"},
    {"name": "attrs", "version": "25.1.0"},
    {"name": "more-itertools", "version": "10.6.0"}
  ]
}
```

- [ ] **Step 2: Create ecosystem test runner**

Create `goopy/ecosystem/ecosystem_test.go`:

```go
package ecosystem

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/pypx/goopy"
)

type packageEntry struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type packageList struct {
	Packages []packageEntry `json:"packages"`
}

func TestEcosystem(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping ecosystem tests in short mode")
	}

	data, err := os.ReadFile("packages.json")
	if err != nil {
		t.Fatalf("reading packages.json: %v", err)
	}

	var list packageList
	if err := json.Unmarshal(data, &list); err != nil {
		t.Fatalf("parsing packages.json: %v", err)
	}

	for _, pkg := range list.Packages {
		t.Run(pkg.Name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			result, err := goopy.ExtractFromPyPI(ctx, pkg.Name, pkg.Version)
			if err != nil {
				t.Fatalf("ExtractFromPyPI(%s, %s): %v", pkg.Name, pkg.Version, err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if len(result.Modules) == 0 {
				t.Error("no modules extracted")
			}

			totalFuncs := 0
			totalClasses := 0
			for _, mod := range result.Modules {
				totalFuncs += len(mod.Functions)
				totalClasses += len(mod.Classes)
			}

			t.Logf("%s: %d modules, %d functions, %d classes",
				pkg.Name, len(result.Modules), totalFuncs, totalClasses)

			if totalFuncs == 0 && totalClasses == 0 {
				t.Errorf("no functions or classes found in %s", pkg.Name)
			}
		})
	}
}
```

- [ ] **Step 3: Run ecosystem tests**

```bash
cd goopy && go test ./ecosystem/... -v -count=1
```

This downloads and parses all 10 packages. Expect some to need fixes in the parser/extractor.

- [ ] **Step 4: Fix any failures discovered**

Iterate: fix parser/extractor bugs, re-run ecosystem tests until all pass.

- [ ] **Step 5: Commit**

```bash
git add goopy/ecosystem/
git commit -m "test(goopy): add ecosystem test runner with 10 packages"
```

---

## Phase 8: pypx Integration

### Deferred Technical Debt (from principal Go audit, 2026-04-15)

These items were identified during a principal-level Go audit and deferred to Phase 8. They should be addressed during or immediately after integration work.

- [x] **Fetcher interface** — `wheel.Fetcher` interface with configurable `BaseURL` on `Source`. `ExtractFromWheel()` accepts any Fetcher.

- [x] **Wheel package test coverage (25.7% → 86.5%)** — httptest-based tests for Fetch/fetchWheelURLs, crafted zip tests for extractPyFiles.

- [x] **Concurrency in ExtractPackage** — Bounded worker pool using `sync.WaitGroup` + channel. GOMAXPROCS workers for packages with 5+ modules. Race-detector clean.

- [x] **Benchmarks and fuzz tests** — BenchmarkLex (193 MB/s), BenchmarkParse (95 MB/s), BenchmarkParseAndExtract (72 MB/s). FuzzLex and FuzzParse targets (zero panics in 500K+ executions).

- [x] **CI workflow** — `.github/workflows/goopy-ecosystem.yml`: weekly cron + manual dispatch, unit tests with race detector, 60-package parity test against griffe.

---

### Task 23: Update DocsHandler to use goopy

**Files:**
- Modify: `api/go.mod` (add goopy dependency)
- Modify: `api/internal/handler/docs.go`
- Modify: `api/internal/handler/docs_test.go`

- [ ] **Step 1: Add goopy dependency to API module**

```bash
cd api && go mod edit -require github.com/pypx/goopy@v0.0.0
cd api && go mod edit -replace github.com/pypx/goopy=../goopy
cd api && go mod tidy
```

- [ ] **Step 2: Update DocsHandler**

Modify `api/internal/handler/docs.go`:

Replace the sidecar HTTP call with a direct `goopy.ExtractFromPyPI` call. Update the response types to use the richer model (add Methods, Properties, Attributes to `DocSymbol`, add structured Docstring fields). Keep backward compatibility by ensuring the existing JSON fields (`name`, `kind`, `signature`, `docstring`, `parameters`, `returns`) remain present.

Key changes:
- Remove `sidecarURL` and `httpClient` from `DocsHandler`
- Replace `sidecarResponse` struct with direct goopy model usage
- Convert `model.Package` → `DocsResponse` in the handler
- Update `NewDocsHandler` signature (remove sidecarURL parameter)

- [ ] **Step 3: Update tests**

Modify `api/internal/handler/docs_test.go` to work without the sidecar mock server. Tests should verify that the handler correctly converts goopy output to the API response format.

- [ ] **Step 4: Run API tests**

```bash
cd api && go test ./internal/handler/... -v -run TestDocs
```

- [ ] **Step 5: Commit**

```bash
git add api/go.mod api/go.sum api/internal/handler/docs.go api/internal/handler/docs_test.go
git commit -m "feat(api): replace Python sidecar with goopy for docs extraction"
```

---

### Task 24: Update main.go and remove sidecar wiring

**Files:**
- Modify: `api/cmd/server/main.go`
- Modify: `docker-compose.yml`

- [ ] **Step 1: Update main.go**

Remove the `DOCS_WORKER_URL` env var and sidecar URL from `NewDocsHandler` call in `api/cmd/server/main.go`.

- [ ] **Step 2: Update docker-compose.yml**

Remove the `docs-worker` service definition. Remove the `DOCS_WORKER_URL` environment variable from the `api` service.

- [ ] **Step 3: Run the API server locally and verify docs endpoint**

```bash
cd api && go run ./cmd/server &
curl -s http://localhost:8080/api/packages/click/docs | jq '.modules | length'
```

Expected: non-zero number of modules.

- [ ] **Step 4: Commit**

```bash
git add api/cmd/server/main.go docker-compose.yml
git commit -m "chore: remove docs-worker sidecar from deployment"
```

---

### Task 25: Remove docs-worker directory

**Files:**
- Delete: `docs-worker/` (entire directory)

- [ ] **Step 1: Verify nothing else references docs-worker**

```bash
grep -r "docs-worker\|docs_worker\|DOCS_WORKER" --include="*.go" --include="*.yml" --include="*.yaml" --include="*.ts" --include="*.vue" api/ web/ docker-compose.yml Caddyfile
```

Expected: no references remaining.

- [ ] **Step 2: Remove the directory**

```bash
git rm -r docs-worker/
```

- [ ] **Step 3: Update Caddyfile if it routes to docs-worker**

Check `Caddyfile` for any proxy rules pointing to the docs-worker service and remove them.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "chore: remove Python docs-worker (replaced by goopy)"
```

---

## Summary

| Phase | Tasks | What it produces |
|-------|-------|-----------------|
| 1: Foundation | Tasks 1-3 | Token types, AST nodes, public model types |
| 2: Lexer | Tasks 4-6 | Working Python lexer with INDENT/DEDENT |
| 3: Parser | Tasks 7-11 | Complete recursive-descent parser |
| 4: Docstring | Tasks 12-15 | Google, NumPy, Sphinx docstring parsers |
| 5: Extractor | Tasks 16-18 | AST-to-model extraction with TypeExpr |
| 6: Public API | Tasks 19-21 | goopy.ExtractModule, ExtractFromPyPI |
| 7: Ecosystem | Task 22 | 10-package ecosystem test suite |
| 8: Integration | Tasks 23-25 | pypx uses goopy, Python sidecar removed |

Each phase produces passing tests before moving to the next. The ecosystem tests in Phase 7 serve as the validation gate before integration in Phase 8.
