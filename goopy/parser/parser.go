// Package parser implements a recursive-descent parser for Python source code.
// It consumes a token stream from the lexer and produces an AST.
//
// The parser is designed to extract API documentation: it fully models function
// definitions, class definitions, imports, assignments, type aliases, and if
// statements. Constructs that don't affect the public API (for/while/try/with/match)
// are parsed shallowly as PassThrough nodes, but their bodies are still recursed
// into so nested def/class declarations can be discovered.
package parser

import (
	"fmt"
	"strings"

	"github.com/pypx/goopy/ast"
	"github.com/pypx/goopy/lexer"
	"github.com/pypx/goopy/token"
)

// Error represents a single parse error with its position.
type Error struct {
	Pos token.Pos
	Msg string
}

func (e Error) Error() string {
	return fmt.Sprintf("%d:%d: %s", e.Pos.Line, e.Pos.Col, e.Msg)
}

// maxExprDepth bounds expression-parser recursion to prevent a crafted
// deeply-nested input from overflowing the goroutine stack (a fatal,
// non-recoverable Go runtime error). 1000 is far below the crash threshold
// (~hundreds of thousands of frames) yet far above any real Python source.
const maxExprDepth = 1000

// Parser holds the state for parsing a Python source file.
type Parser struct {
	lex    *lexer.Lexer
	tok    token.Token // current (lookahead) token
	errors []Error
	src    []byte
	depth  int // current expression-recursion depth (guards against stack overflow)

	prev     token.Type // type of the most recently consumed token
	consumed int        // number of tokens consumed, used to detect a stalled loop
}

// New creates a parser for the given Python source bytes.
// It initializes the lexer and primes the first lookahead token.
func New(src []byte) *Parser {
	p := &Parser{
		lex: lexer.New(src),
		src: src,
	}
	p.next() // prime the lookahead
	return p
}

// Errors returns all parse errors collected during parsing.
func (p *Parser) Errors() []Error {
	return p.errors
}

// Parse parses the entire source as a Python module and returns the root AST node.
func (p *Parser) Parse() *ast.Module {
	mod := &ast.Module{Position: p.tok.Pos}

	// Skip leading newlines.
	for p.tok.Type == token.NEWLINE {
		p.next()
	}

	for p.tok.Type != token.EOF {
		stmt := p.parseStmtRecovering()
		if stmt != nil {
			mod.Body = append(mod.Body, stmt)
		}
		// Skip stray newlines between statements.
		for p.tok.Type == token.NEWLINE {
			p.next()
		}
	}

	mod.EndPos = p.tok.Pos
	return mod
}

// ---------------------------------------------------------------------------
// Token helpers
// ---------------------------------------------------------------------------

// next advances to the next token.
func (p *Parser) next() {
	p.prev = p.tok.Type
	p.consumed++
	p.tok = p.lex.Next()
}

// atLineEnd returns true at a token that ends the logical line or changes block
// nesting. The lexer emits these tokens only outside brackets, so they always mean
// real structure. Only the statement and block parsers may consume them.
func (p *Parser) atLineEnd() bool {
	switch p.tok.Type {
	case token.NEWLINE, token.INDENT, token.DEDENT, token.EOF:
		return true
	}
	return false
}

// expect consumes the current token if it matches typ. Otherwise it records an
// error and skips the unexpected token, unless that token is structural.
// Returns the consumed token (or the unexpected token on failure).
func (p *Parser) expect(typ token.Type) token.Token {
	if p.tok.Type == typ {
		t := p.tok
		p.next()
		return t
	}
	t := p.tok
	p.errorf("expected %s, got %s", typ, p.tok.Type)
	if !p.atLineEnd() {
		p.next()
	}
	return t
}

// expectColon consumes the colon that ends a compound statement header. If the
// header has unexpected tokens, it records an error and skips to the colon, so
// that the block body stays attached to this statement.
func (p *Parser) expectColon() {
	if p.tok.Type != token.COLON {
		p.errorf("expected %s, got %s", token.COLON, p.tok.Type)
		p.skipHeader()
	}
	if p.tok.Type == token.COLON {
		p.next()
	}
}

// expectName is a convenience for expecting a NAME token and returning its literal.
// Also accepts soft keywords (type, match, case) which can be used as identifiers.
func (p *Parser) expectName() string {
	if p.isSoftKeyword() {
		t := p.tok
		p.next()
		return t.Lit
	}
	t := p.expect(token.NAME)
	return t.Lit
}

// isSoftKeyword returns true for tokens that are keywords in some contexts
// but valid identifiers in others (Python 3.10+ match/case, 3.12+ type).
func (p *Parser) isSoftKeyword() bool {
	return p.tok.Type == token.TYPE || p.tok.Type == token.MATCH || p.tok.Type == token.CASE
}

// at returns true if the current token is one of the given types.
func (p *Parser) at(types ...token.Type) bool {
	for _, t := range types {
		if p.tok.Type == t {
			return true
		}
	}
	return false
}

// errorf records a parse error at the current token position.
func (p *Parser) errorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	p.errors = append(p.errors, Error{Pos: p.tok.Pos, Msg: msg})
}

// skipNewlines consumes any NEWLINE tokens.
func (p *Parser) skipNewlines() {
	for p.tok.Type == token.NEWLINE {
		p.next()
	}
}

// ---------------------------------------------------------------------------
// Error recovery
// ---------------------------------------------------------------------------

// isStmtStart returns true if the current token begins a new statement.
// Used to detect unclosed parentheses during error recovery.
// Excludes soft keywords (type, match, case) which can appear as identifiers.
func (p *Parser) isStmtStart() bool {
	if p.isSoftKeyword() {
		return false
	}
	switch p.tok.Type {
	case token.DEF, token.CLASS, token.ASYNC, token.IMPORT, token.FROM,
		token.IF, token.FOR, token.WHILE, token.TRY, token.WITH,
		token.RETURN, token.RAISE, token.PASS, token.DEDENT:
		return true
	}
	return false
}

// syncStmt skips tokens until we reach the end of the logical line.
func (p *Parser) syncStmt() {
	for !p.atLineEnd() {
		p.next()
	}
}

// skipHeader advances to the colon that ends a compound statement header. It
// ignores colons inside brackets and colons that belong to a lambda, and it never
// passes the end of the logical line.
func (p *Parser) skipHeader() {
	depth, lambdas := 0, 0
	for !p.atLineEnd() {
		switch p.tok.Type {
		case token.LPAREN, token.LBRACK, token.LBRACE:
			depth++
		case token.RPAREN, token.RBRACK, token.RBRACE:
			depth--
		case token.LAMBDA:
			if depth <= 0 {
				lambdas++
			}
		case token.COLON:
			if depth <= 0 {
				if lambdas == 0 {
					return
				}
				lambdas--
			}
		}
		p.next()
	}
}

// skipUntil skips tokens until the given closing delimiter is reached at depth 0.
// It tracks bracket nesting so it won't stop at a delimiter inside a nested group.
func (p *Parser) skipUntil(close token.Type) {
	depth := 0
	for !p.atLineEnd() {
		switch p.tok.Type {
		case token.LPAREN, token.LBRACK, token.LBRACE:
			depth++
		case token.RPAREN, token.RBRACK, token.RBRACE:
			if depth == 0 && p.tok.Type == close {
				return
			}
			depth--
		}
		p.next()
	}
}

// ---------------------------------------------------------------------------
// Statement parsing
// ---------------------------------------------------------------------------

// parseStmtRecovering parses one statement and contains any error inside it. After
// an error it drops the rest of the logical line, so that the block continues. It
// also guarantees progress, so that the statement loops always end.
func (p *Parser) parseStmtRecovering() ast.Stmt {
	errs, consumed := len(p.errors), p.consumed
	stmt := p.parseStmt()
	if len(p.errors) > errs && !p.atStmtBoundary() {
		p.syncStmt()
		if p.tok.Type == token.NEWLINE {
			p.next()
		}
	}
	if p.consumed == consumed {
		p.next()
	}
	return stmt
}

// atStmtBoundary returns true if the parser already stands at the start of a
// statement: the previous statement ended its line or its block, or it was
// abandoned at a statement keyword because of an unclosed bracket.
func (p *Parser) atStmtBoundary() bool {
	switch p.prev {
	case token.NEWLINE, token.INDENT, token.DEDENT:
		return true
	}
	return p.isStmtStart()
}

// parseStmt parses a single statement, dispatching based on the current token.
func (p *Parser) parseStmt() ast.Stmt {
	switch p.tok.Type {
	case token.INDENT:
		// An indent with no header, or after a header that failed. Keep the
		// body as a nested block so that its DEDENT cannot close the enclosing block.
		pos := p.tok.Pos
		p.errorf("unexpected indent")
		body := p.parseIndentedBody()
		return &ast.PassThrough{Kind: "error", Body: body, Position: pos, EndPos: p.tok.Pos}
	case token.AT:
		return p.parseDecorated()
	case token.DEF:
		return p.parseFuncDef(nil)
	case token.ASYNC:
		return p.parseAsync(nil)
	case token.CLASS:
		return p.parseClassDef(nil)
	case token.IF:
		return p.parseIf()
	case token.MATCH:
		if p.isMatchStmt() {
			return p.parseMatch()
		}
		return p.parseSimpleStmt()
	case token.FOR, token.WHILE, token.TRY, token.WITH:
		return p.parsePassThrough()
	default:
		return p.parseSmallStmt()
	}
}

// parseSmallStmt parses one statement that can share a line with others: anything
// except a compound statement. A one-line block allows only these, so this path has
// no recursion through blocks, and a long line cannot grow the stack.
func (p *Parser) parseSmallStmt() ast.Stmt {
	switch p.tok.Type {
	case token.NAME:
		// 'lazy' is a soft keyword only directly before an import statement.
		if p.tok.Lit == "lazy" {
			switch p.lex.Clone().Next().Type {
			case token.IMPORT:
				p.next()
				return p.parseImport()
			case token.FROM:
				p.next()
				return p.parseImportFrom()
			}
		}
		return p.parseSimpleStmt()
	case token.IMPORT:
		return p.parseImport()
	case token.FROM:
		return p.parseImportFrom()
	case token.TYPE:
		return p.parseTypeOrSimple()
	case token.PASS:
		return p.parsePass()
	case token.RETURN, token.RAISE, token.YIELD, token.DEL,
		token.GLOBAL, token.NONLOCAL, token.ASSERT:
		return p.parseSimpleKeywordStmt()
	default:
		return p.parseSimpleStmt()
	}
}

// parseDecorated parses one or more decorators followed by a def or class.
func (p *Parser) parseDecorated() ast.Stmt {
	var decorators []ast.Expr

	for p.tok.Type == token.AT {
		p.next() // consume '@'
		dec := p.parseNamedExpr()
		decorators = append(decorators, dec)
		// Consume the NEWLINE after the decorator.
		if p.tok.Type == token.NEWLINE {
			p.next()
		}
		p.skipNewlines()
	}

	switch p.tok.Type {
	case token.DEF:
		return p.parseFuncDef(decorators)
	case token.ASYNC:
		return p.parseAsync(decorators)
	case token.CLASS:
		return p.parseClassDef(decorators)
	default:
		p.errorf("expected def or class after decorator, got %s", p.tok.Type)
		p.syncStmt()
		return nil
	}
}

// parseAsync parses 'async def ...', 'async for ...', or 'async with ...'.
func (p *Parser) parseAsync(decorators []ast.Expr) ast.Stmt {
	pos := p.tok.Pos
	p.next() // consume 'async'

	switch p.tok.Type {
	case token.DEF:
		fn := p.parseFuncDef(decorators)
		if fd, ok := fn.(*ast.FunctionDef); ok {
			fd.IsAsync = true
		}
		return fn

	case token.FOR:
		p.next() // consume 'for'
		body := p.parseLoopBlocks()
		endPos := pos
		if len(body) > 0 {
			endPos = body[len(body)-1].End()
		}
		return &ast.PassThrough{
			Kind:     "async for",
			Body:     body,
			Position: pos,
			EndPos:   endPos,
		}

	case token.WITH:
		p.next() // consume 'with'
		body := p.parsePassThroughBlock()
		endPos := pos
		if len(body) > 0 {
			endPos = body[len(body)-1].End()
		}
		return &ast.PassThrough{
			Kind:     "async with",
			Body:     body,
			Position: pos,
			EndPos:   endPos,
		}

	default:
		p.errorf("expected def, for, or with after async, got %s", p.tok.Type)
		p.syncStmt()
		return nil
	}
}

// parseFuncDef parses 'def name(params) -> return_type: block'.
func (p *Parser) parseFuncDef(decorators []ast.Expr) ast.Stmt {
	pos := p.tok.Pos
	if len(decorators) > 0 {
		pos = decorators[0].Pos()
	}
	p.next() // consume 'def'

	name := p.expectName()

	// Optional type parameters [T, U, ...].
	var typeParams []*ast.TypeParam
	if p.tok.Type == token.LBRACK {
		typeParams = p.parseTypeParams()
	}

	// Parameters.
	p.expect(token.LPAREN)
	args := p.parseParameters()

	// If the parameter list hit a statement keyword (unclosed paren), abandon this def.
	if p.isStmtStart() {
		return nil
	}

	p.expect(token.RPAREN)

	// Optional return type annotation.
	var returns ast.Expr
	if p.tok.Type == token.ARROW {
		p.next()
		returns = p.parseExpr()
	}

	p.expectColon()
	body := p.parseBlock()

	endPos := pos
	if len(body) > 0 {
		endPos = body[len(body)-1].End()
	}

	return &ast.FunctionDef{
		Name:       name,
		Args:       args,
		Body:       body,
		Decorators: decorators,
		Returns:    returns,
		TypeParams: typeParams,
		Position:   pos,
		EndPos:     endPos,
	}
}

// parseClassDef parses 'class name(bases): block'.
func (p *Parser) parseClassDef(decorators []ast.Expr) ast.Stmt {
	pos := p.tok.Pos
	if len(decorators) > 0 {
		pos = decorators[0].Pos()
	}
	p.next() // consume 'class'

	name := p.expectName()

	// Optional type parameters.
	var typeParams []*ast.TypeParam
	if p.tok.Type == token.LBRACK {
		typeParams = p.parseTypeParams()
	}

	// Optional base classes / keywords.
	var bases []ast.Expr
	var keywords []*ast.Keyword
	if p.tok.Type == token.LPAREN {
		p.next()
		bases, keywords = p.parseArgList()

		// If the arg list hit a statement keyword (unclosed paren), abandon this class.
		if p.isStmtStart() {
			p.errorf("unclosed parenthesis in class bases")
			return nil
		}

		p.expect(token.RPAREN)
	}

	p.expectColon()
	body := p.parseBlock()

	endPos := pos
	if len(body) > 0 {
		endPos = body[len(body)-1].End()
	}

	return &ast.ClassDef{
		Name:       name,
		Bases:      bases,
		Keywords:   keywords,
		Body:       body,
		Decorators: decorators,
		TypeParams: typeParams,
		Position:   pos,
		EndPos:     endPos,
	}
}

// parseBlock parses a colon-delimited block: NEWLINE INDENT stmt+ DEDENT.
// Also handles single-line blocks (e.g., "def foo(): pass").
func (p *Parser) parseBlock() []ast.Stmt {
	// Single-line block (no NEWLINE INDENT).
	if p.tok.Type != token.NEWLINE {
		var body []ast.Stmt
		for {
			if stmt := p.parseSmallStmt(); stmt != nil {
				body = append(body, stmt)
			}
			// A ';' that endSimpleStmt consumed means this line has more statements.
			if p.prev != token.SEMI || p.atLineEnd() {
				return body
			}
		}
	}

	p.next() // consume NEWLINE
	p.skipNewlines()

	if p.tok.Type != token.INDENT {
		// Empty block or malformed — don't error, just return empty.
		return nil
	}
	return p.parseIndentedBody()
}

// parseIndentedBody parses INDENT stmt+ DEDENT. The caller stands at the INDENT.
func (p *Parser) parseIndentedBody() []ast.Stmt {
	p.next() // consume INDENT

	var body []ast.Stmt
	for p.tok.Type != token.DEDENT && p.tok.Type != token.EOF {
		stmt := p.parseStmtRecovering()
		if stmt != nil {
			body = append(body, stmt)
		}
		p.skipNewlines()
	}

	if p.tok.Type == token.DEDENT {
		p.next()
	}

	return body
}

// endSimpleStmt consumes the end of a simple statement: a NEWLINE, or a ';' with
// the NEWLINE that can follow it. Any other token is an error. The parser then
// stands at the next statement, on this line or the next one.
func (p *Parser) endSimpleStmt() {
	switch p.tok.Type {
	case token.NEWLINE:
		p.next()
	case token.SEMI:
		p.next()
		if p.tok.Type == token.NEWLINE {
			p.next()
		}
	case token.DEDENT, token.EOF:
	default:
		p.errorf("expected end of statement, got %s", p.tok.Type)
	}
}

// parsePass parses a 'pass' statement.
func (p *Parser) parsePass() ast.Stmt {
	pos := p.tok.Pos
	p.next() // consume 'pass'
	p.endSimpleStmt()
	return &ast.PassThrough{
		Kind:     "pass",
		Position: pos,
		EndPos:   pos,
	}
}

// parseImport parses 'import dotted_as_names'.
func (p *Parser) parseImport() ast.Stmt {
	pos := p.tok.Pos
	p.next() // consume 'import'

	var names []*ast.ImportAlias
	for {
		alias := p.parseDottedAsName()
		names = append(names, alias)
		if p.tok.Type != token.COMMA {
			break
		}
		p.next() // consume ','
	}

	endPos := p.tok.Pos
	p.endSimpleStmt()

	return &ast.Import{
		Position: pos,
		EndPos:   endPos,
		Names:    names,
	}
}

// parseDottedAsName parses "dotted_name ('as' NAME)?".
func (p *Parser) parseDottedAsName() *ast.ImportAlias {
	name := p.parseDottedName()
	alias := ""
	if p.tok.Type == token.AS {
		p.next()
		alias = p.expectName()
	}
	return &ast.ImportAlias{Name: name, Alias: alias}
}

// parseDottedName parses "NAME ('.' NAME)*" and returns the joined string.
func (p *Parser) parseDottedName() string {
	name := p.expectName()
	for p.tok.Type == token.DOT {
		p.next()
		name += "." + p.expectName()
	}
	return name
}

// parseImportFrom parses 'from ... import ...'.
func (p *Parser) parseImportFrom() ast.Stmt {
	pos := p.tok.Pos
	p.next() // consume 'from'

	// Count leading dots for relative imports.
	// The lexer groups dots greedily, so '....' is ELLIPSIS DOT.
	level := 0
	for p.at(token.DOT, token.ELLIPSIS) {
		if p.tok.Type == token.DOT {
			level++
		} else {
			level += 3
		}
		p.next()
	}

	// Optional module name.
	module := ""
	if p.tok.Type == token.NAME || p.isSoftKeyword() {
		module = p.parseDottedName()
	}

	p.expect(token.IMPORT)

	var names []*ast.ImportAlias

	if p.tok.Type == token.STAR {
		// from module import *
		names = append(names, &ast.ImportAlias{Name: "*"})
		p.next()
	} else if p.tok.Type == token.LPAREN {
		// from module import (name1, name2, ...)
		p.next()
		names = p.parseImportNames()
		p.expect(token.RPAREN)
	} else {
		names = p.parseImportNames()
	}
	if len(names) == 0 {
		p.errorf("expected import name, got %s", p.tok.Type)
	}

	endPos := p.tok.Pos
	p.endSimpleStmt()

	return &ast.ImportFrom{
		Position: pos,
		EndPos:   endPos,
		Module:   module,
		Names:    names,
		Level:    level,
	}
}

// parseImportNames parses a comma-separated list of import aliases.
func (p *Parser) parseImportNames() []*ast.ImportAlias {
	var names []*ast.ImportAlias
	for {
		if p.tok.Type == token.RPAREN || p.tok.Type == token.NEWLINE || p.tok.Type == token.EOF {
			break
		}
		name := p.expectName()
		alias := ""
		if p.tok.Type == token.AS {
			p.next()
			alias = p.expectName()
		}
		names = append(names, &ast.ImportAlias{Name: name, Alias: alias})
		if p.tok.Type != token.COMMA {
			break
		}
		p.next() // consume ','
		p.skipNewlines()
	}
	return names
}

// parseTypeOrSimple disambiguates 'type' as a type alias keyword vs. a regular name.
// In Python 3.12+, 'type X = ...' is a type alias. But 'type' can also be a regular
// identifier (e.g., 'type = "foo"' or 'type(x).y'). Only an alias has a name next.
func (p *Parser) parseTypeOrSimple() ast.Stmt {
	switch p.lex.Clone().Next().Type {
	case token.NAME, token.TYPE, token.MATCH, token.CASE:
	default:
		return p.parseSimpleStmt()
	}

	pos := p.tok.Pos
	p.next() // consume 'type'
	name := p.expectName()

	// Optional type parameters.
	var typeParams []*ast.TypeParam
	if p.tok.Type == token.LBRACK {
		typeParams = p.parseTypeParams()
	}

	p.expect(token.ASSIGN)
	value := p.parseExpr()
	endPos := value.End()
	p.endSimpleStmt()
	return &ast.TypeAliasDef{
		Position:   pos,
		EndPos:     endPos,
		Name:       name,
		TypeParams: typeParams,
		Value:      value,
	}
}

// parseIf parses 'if expr: block (elif expr: block)* (else: block)?'.
func (p *Parser) parseIf() ast.Stmt {
	pos := p.tok.Pos
	p.next() // consume 'if'

	test := p.parseNamedExpr()
	p.expectColon()
	body := p.parseBlock()

	var orelse []ast.Stmt
	p.skipNewlines()

	if p.tok.Type == token.ELIF {
		// elif is parsed as a nested if in the orelse.
		elifStmt := p.parseElif()
		if elifStmt != nil {
			orelse = []ast.Stmt{elifStmt}
		}
	} else if p.tok.Type == token.ELSE {
		p.next() // consume 'else'
		p.expectColon()
		orelse = p.parseBlock()
	}

	endPos := pos
	if len(orelse) > 0 {
		endPos = orelse[len(orelse)-1].End()
	} else if len(body) > 0 {
		endPos = body[len(body)-1].End()
	}

	return &ast.If{
		Position: pos,
		EndPos:   endPos,
		Test:     test,
		Body:     body,
		Orelse:   orelse,
	}
}

// parseElif parses 'elif expr: block ...' as a nested If node.
func (p *Parser) parseElif() ast.Stmt {
	pos := p.tok.Pos
	p.next() // consume 'elif'

	test := p.parseNamedExpr()
	p.expectColon()
	body := p.parseBlock()

	var orelse []ast.Stmt
	p.skipNewlines()

	if p.tok.Type == token.ELIF {
		elifStmt := p.parseElif()
		if elifStmt != nil {
			orelse = []ast.Stmt{elifStmt}
		}
	} else if p.tok.Type == token.ELSE {
		p.next()
		p.expectColon()
		orelse = p.parseBlock()
	}

	endPos := pos
	if len(orelse) > 0 {
		endPos = orelse[len(orelse)-1].End()
	} else if len(body) > 0 {
		endPos = body[len(body)-1].End()
	}

	return &ast.If{
		Position: pos,
		EndPos:   endPos,
		Test:     test,
		Body:     body,
		Orelse:   orelse,
	}
}

// parsePassThrough handles compound statements we don't deeply model:
// for, while, try, with, match. It skips to the block, recurses into it
// to find nested defs/classes, and wraps everything in a PassThrough node.
func (p *Parser) parsePassThrough() ast.Stmt {
	pos := p.tok.Pos
	kind := strings.ToLower(p.tok.Lit)
	p.next() // consume the keyword

	var body []ast.Stmt

	switch kind {
	case "try":
		body = p.parseTryBlocks(pos)
	case "for", "while":
		body = p.parseLoopBlocks()
	default:
		body = p.parsePassThroughBlock()
	}

	endPos := pos
	if len(body) > 0 {
		endPos = body[len(body)-1].End()
	}

	return &ast.PassThrough{
		Kind:     kind,
		Body:     body,
		Position: pos,
		EndPos:   endPos,
	}
}

// parsePassThroughBlock skips the header up to its ':' then parses the block.
func (p *Parser) parsePassThroughBlock() []ast.Stmt {
	p.skipHeader()
	p.expectColon()
	return p.parseBlock()
}

// isMatchStmt decides whether the soft keyword 'match' at the current token starts
// a match statement. The whole statement grammar decides this: only a match
// statement has a logical line that ends in ':' and is followed by an indented 'case'.
func (p *Parser) isMatchStmt() bool {
	lex := p.lex.Clone()
	t := lex.Next()
	if t.Type == token.COLON {
		return false // 'match: T' is an annotation of a variable named match.
	}
	prev := token.EOF
	for t.Type != token.NEWLINE {
		if t.Type == token.EOF || t.Type == token.INDENT || t.Type == token.DEDENT {
			return false
		}
		prev = t.Type
		t = lex.Next()
	}
	return prev == token.COLON && lex.Next().Type == token.INDENT && lex.Next().Type == token.CASE
}

// parseMatch parses a match statement. isMatchStmt has checked its shape. Subjects
// and patterns are skipped, and the bodies of all cases are recursed into.
func (p *Parser) parseMatch() ast.Stmt {
	pos := p.tok.Pos
	p.next() // consume 'match'
	p.skipHeader()
	p.expectColon()
	p.skipNewlines()
	p.next() // consume INDENT

	var body []ast.Stmt
	for p.tok.Type != token.DEDENT && p.tok.Type != token.EOF {
		if p.tok.Type == token.CASE {
			p.next()
			body = append(body, p.parsePassThroughBlock()...)
		} else {
			p.errorf("expected case, got %s", p.tok.Type)
			if stmt := p.parseStmtRecovering(); stmt != nil {
				body = append(body, stmt)
			}
		}
		p.skipNewlines()
	}
	if p.tok.Type == token.DEDENT {
		p.next()
	}

	endPos := pos
	if len(body) > 0 {
		endPos = body[len(body)-1].End()
	}
	return &ast.PassThrough{Kind: "match", Body: body, Position: pos, EndPos: endPos}
}

// parseLoopBlocks parses the body of a for or while loop and its optional else.
func (p *Parser) parseLoopBlocks() []ast.Stmt {
	body := p.parsePassThroughBlock()
	p.skipNewlines()
	if p.tok.Type == token.ELSE {
		p.next()
		body = append(body, p.parsePassThroughBlock()...)
	}
	return body
}

// parseTryBlocks handles try: ... except: ... else: ... finally: ... blocks.
func (p *Parser) parseTryBlocks(pos token.Pos) []ast.Stmt {
	allBody := p.parsePassThroughBlock()
	p.skipNewlines()

	// Handle except/else/finally blocks.
	for p.at(token.EXCEPT, token.ELSE, token.FINALLY) {
		p.next() // consume keyword
		allBody = append(allBody, p.parsePassThroughBlock()...)
		p.skipNewlines()
	}

	return allBody
}

// parseSimpleKeywordStmt handles simple keyword statements (return, raise, yield, etc.)
// that we don't deeply model. Consumes until NEWLINE.
func (p *Parser) parseSimpleKeywordStmt() ast.Stmt {
	pos := p.tok.Pos
	kind := strings.ToLower(p.tok.Lit)
	p.next()

	// Consume the rest of the statement.
	for !p.atLineEnd() && p.tok.Type != token.SEMI {
		p.next()
	}
	p.endSimpleStmt()

	return &ast.PassThrough{
		Kind:     kind,
		Position: pos,
		EndPos:   pos,
	}
}

// parseSimpleStmt parses an expression statement, assignment, or annotated assignment.
func (p *Parser) parseSimpleStmt() ast.Stmt {
	expr := p.parseExprList()
	if expr == nil {
		p.errorf("expected statement, got %s", p.tok.Type)
		p.syncStmt()
		p.endSimpleStmt()
		return nil
	}

	return p.finishSimpleStmt(expr)
}

// finishSimpleStmt completes a simple statement after the initial expression is parsed.
// This handles assignment (=), annotated assignment (:), or bare expression statement.
func (p *Parser) finishSimpleStmt(expr ast.Expr) ast.Stmt {
	switch p.tok.Type {
	case token.ASSIGN:
		// Assignment: target = value, or chained: a = b = value
		targets := []ast.Expr{expr}
		p.next()
		value := p.parseExprList()
		// In a chain, every expression except the last is a target.
		for p.tok.Type == token.ASSIGN {
			targets = append(targets, value)
			p.next()
			value = p.parseExprList()
		}
		endPos := p.tok.Pos
		p.endSimpleStmt()
		return &ast.Assign{
			Position: expr.Pos(),
			EndPos:   endPos,
			Targets:  targets,
			Value:    value,
		}

	case token.PLUSEQ, token.MINUSEQ, token.STAREQ, token.SLASHEQ,
		token.DSLASHEQ, token.PERCENTEQ, token.DSTAREQ,
		token.AMPEREQ, token.PIPEEQ, token.CARETEQ,
		token.RSHIFTEQ, token.LSHIFTEQ, token.ATEQ:
		// Augmented assignment: target op= value
		p.next()
		value := p.parseExprList()
		endPos := p.tok.Pos
		p.endSimpleStmt()
		return &ast.Assign{
			Position: expr.Pos(),
			EndPos:   endPos,
			Targets:  []ast.Expr{expr},
			Value:    value,
		}

	case token.COLON:
		// Annotated assignment: target : annotation (= value)?
		p.next()
		annotation := p.parseExpr()
		var value ast.Expr
		if p.tok.Type == token.ASSIGN {
			p.next()
			value = p.parseExprList()
		}
		endPos := p.tok.Pos
		p.endSimpleStmt()
		// Simple is true when the target is a plain name (not subscript/attribute).
		_, simple := expr.(*ast.Name)
		return &ast.AnnAssign{
			Position:   expr.Pos(),
			EndPos:     endPos,
			Target:     expr,
			Annotation: annotation,
			Value:      value,
			Simple:     simple,
		}

	default:
		// Bare expression statement.
		p.endSimpleStmt()
		return &ast.ExprStmt{Value: expr}
	}
}

// ---------------------------------------------------------------------------
// Parameter parsing
// ---------------------------------------------------------------------------

// parseParameters parses function parameters inside parentheses.
// Handles all five kinds: positional-only, regular, *args, keyword-only, **kwargs.
func (p *Parser) parseParameters() *ast.Arguments {
	args := &ast.Arguments{}

	if p.tok.Type == token.RPAREN {
		return args
	}

	// Phase 1: Collect all parameters linearly, then categorize.
	type paramInfo struct {
		name       string
		pos        token.Pos
		annotation ast.Expr
		default_   ast.Expr
		isStar     bool // bare * or *name
		starName   string
		isDStar    bool // **name
	}
	var params []paramInfo
	slashIdx := -1 // index after which we saw '/'

	for p.tok.Type != token.RPAREN && !p.atLineEnd() {
		// Bail out if we hit a statement keyword — the paren was never closed.
		if p.isStmtStart() {
			p.errorf("unclosed parenthesis in parameter list")
			break
		}

		if p.tok.Type == token.SLASH {
			// Positional-only separator.
			slashIdx = len(params)
			p.next()
			if p.tok.Type == token.COMMA {
				p.next()
			}
			continue
		}

		if p.tok.Type == token.DSTAR {
			// **kwargs
			p.next()
			pi := paramInfo{isDStar: true, pos: p.tok.Pos}
			pi.name = p.expectName()
			if p.tok.Type == token.COLON {
				p.next()
				pi.annotation = p.parseExpr()
			}
			params = append(params, pi)
			if p.tok.Type == token.COMMA {
				p.next()
			}
			continue
		}

		if p.tok.Type == token.STAR {
			p.next()
			pi := paramInfo{isStar: true, pos: p.tok.Pos}
			// Could be bare * (keyword-only separator) or *args.
			if p.tok.Type == token.NAME || p.isSoftKeyword() {
				pi.starName = p.expectName()
				pi.pos = p.tok.Pos
				if p.tok.Type == token.COLON {
					p.next()
					pi.annotation = p.parseExpr()
				}
			}
			params = append(params, pi)
			if p.tok.Type == token.COMMA {
				p.next()
			}
			continue
		}

		// Regular parameter.
		pi := paramInfo{pos: p.tok.Pos}
		pi.name = p.expectName()
		if p.tok.Type == token.COLON {
			p.next()
			pi.annotation = p.parseExpr()
		}
		if p.tok.Type == token.ASSIGN {
			p.next()
			pi.default_ = p.parseExpr()
		}
		params = append(params, pi)
		if p.tok.Type == token.COMMA {
			p.next()
		}
	}

	// Phase 2: Categorize parameters.
	starIdx := -1
	for i, pi := range params {
		if pi.isStar {
			starIdx = i
			break
		}
	}

	dstarIdx := -1
	for i, pi := range params {
		if pi.isDStar {
			dstarIdx = i
			break
		}
	}

	// Determine the range of regular positional params.
	regularEnd := len(params)
	if starIdx >= 0 {
		regularEnd = starIdx
	}
	if dstarIdx >= 0 && dstarIdx < regularEnd {
		regularEnd = dstarIdx
	}

	// Split regular params into pos-only and args based on slash.
	if slashIdx >= 0 {
		for i := 0; i < slashIdx && i < regularEnd; i++ {
			pi := params[i]
			args.PosOnlyArgs = append(args.PosOnlyArgs, &ast.Arg{
				ArgPos:     pi.pos,
				Name:       pi.name,
				Annotation: pi.annotation,
			})
			if pi.default_ != nil {
				args.Defaults = append(args.Defaults, pi.default_)
			}
		}
		for i := slashIdx; i < regularEnd; i++ {
			pi := params[i]
			args.Args = append(args.Args, &ast.Arg{
				ArgPos:     pi.pos,
				Name:       pi.name,
				Annotation: pi.annotation,
			})
			if pi.default_ != nil {
				args.Defaults = append(args.Defaults, pi.default_)
			}
		}
	} else {
		for i := 0; i < regularEnd; i++ {
			pi := params[i]
			args.Args = append(args.Args, &ast.Arg{
				ArgPos:     pi.pos,
				Name:       pi.name,
				Annotation: pi.annotation,
			})
			if pi.default_ != nil {
				args.Defaults = append(args.Defaults, pi.default_)
			}
		}
	}

	// Handle *args.
	if starIdx >= 0 {
		pi := params[starIdx]
		if pi.starName != "" {
			args.VarArg = &ast.Arg{
				ArgPos:     pi.pos,
				Name:       pi.starName,
				Annotation: pi.annotation,
			}
		}

		// Keyword-only args: everything after * until ** or end.
		kwEnd := len(params)
		if dstarIdx >= 0 {
			kwEnd = dstarIdx
		}
		for i := starIdx + 1; i < kwEnd; i++ {
			pi := params[i]
			args.KwOnlyArgs = append(args.KwOnlyArgs, &ast.Arg{
				ArgPos:     pi.pos,
				Name:       pi.name,
				Annotation: pi.annotation,
			})
			args.KwDefaults = append(args.KwDefaults, pi.default_) // nil if no default
		}
	}

	// Handle **kwargs.
	if dstarIdx >= 0 {
		pi := params[dstarIdx]
		args.VarKwArg = &ast.Arg{
			ArgPos:     pi.pos,
			Name:       pi.name,
			Annotation: pi.annotation,
		}
	}

	return args
}

// parseTypeParams parses type parameters: '[' param (',' param)* ']', where a param
// is [*|**] name [':' bound] ['=' default].
func (p *Parser) parseTypeParams() []*ast.TypeParam {
	p.next() // consume '['
	var params []*ast.TypeParam

	for p.tok.Type != token.RBRACK && !p.atLineEnd() {
		tp := &ast.TypeParam{Position: p.tok.Pos}
		if p.at(token.STAR, token.DSTAR) {
			p.next()
		}
		tp.Name = p.expectName()

		if p.tok.Type == token.COLON {
			p.next()
			tp.Bound = p.parseExpr()
		}
		if p.tok.Type == token.ASSIGN {
			p.next()
			tp.Default = p.parseExpr()
		}

		tp.EndPos = p.tok.Pos
		params = append(params, tp)

		if p.tok.Type != token.COMMA {
			break
		}
		p.next()
	}

	p.expect(token.RBRACK)
	return params
}
