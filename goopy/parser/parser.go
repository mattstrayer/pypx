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
	return e.Msg
}

// Parser holds the state for parsing a Python source file.
type Parser struct {
	lex    *lexer.Lexer
	tok    token.Token // current (lookahead) token
	errors []Error
	src    []byte
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
		stmt := p.parseStmt()
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
	p.tok = p.lex.Next()
}

// expect consumes the current token if it matches typ, otherwise records an error
// and advances past the unexpected token to prevent infinite loops.
// Returns the consumed token (or the unexpected token on failure).
func (p *Parser) expect(typ token.Type) token.Token {
	if p.tok.Type == typ {
		t := p.tok
		p.next()
		return t
	}
	t := p.tok
	p.errorf("expected %s, got %s", typ, p.tok.Type)
	if p.tok.Type != token.EOF {
		p.next()
	}
	return t
}

// expectName is a convenience for expecting a NAME token and returning its literal.
func (p *Parser) expectName() string {
	t := p.expect(token.NAME)
	return t.Lit
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
	msg := formatMsg(format, args...)
	p.errors = append(p.errors, Error{Pos: p.tok.Pos, Msg: msg})
}

// formatMsg is a minimal sprintf that avoids importing fmt in the hot path.
// It supports %s only, which is all we need for error messages.
func formatMsg(format string, args ...any) string {
	var b strings.Builder
	argIdx := 0
	for i := 0; i < len(format); i++ {
		if format[i] == '%' && i+1 < len(format) && format[i+1] == 's' {
			if argIdx < len(args) {
				switch v := args[argIdx].(type) {
				case string:
					b.WriteString(v)
				case token.Type:
					b.WriteString(v.String())
				default:
					b.WriteString("?")
				}
				argIdx++
			}
			i++ // skip 's'
		} else {
			b.WriteByte(format[i])
		}
	}
	return b.String()
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
func (p *Parser) isStmtStart() bool {
	switch p.tok.Type {
	case token.DEF, token.CLASS, token.ASYNC, token.IMPORT, token.FROM,
		token.IF, token.FOR, token.WHILE, token.TRY, token.WITH, token.MATCH,
		token.RETURN, token.RAISE, token.PASS, token.DEDENT:
		return true
	}
	return false
}

// syncStmt skips tokens until we reach a statement boundary (NEWLINE, DEDENT, or EOF).
func (p *Parser) syncStmt() {
	for !p.at(token.NEWLINE, token.DEDENT, token.EOF) {
		p.next()
	}
}

// syncExpr skips tokens until we reach an expression boundary.
func (p *Parser) syncExpr() {
	for !p.at(token.COMMA, token.RPAREN, token.RBRACK, token.COLON, token.NEWLINE, token.EOF) {
		p.next()
	}
}

// skipUntil skips tokens until the given closing delimiter is reached at depth 0.
// It tracks bracket nesting so it won't stop at a delimiter inside a nested group.
func (p *Parser) skipUntil(close token.Type) {
	depth := 0
	for p.tok.Type != token.EOF {
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

// parseStmt parses a single statement, dispatching based on the current token.
func (p *Parser) parseStmt() ast.Stmt {
	switch p.tok.Type {
	case token.AT:
		return p.parseDecorated()
	case token.DEF:
		return p.parseFuncDef(nil)
	case token.ASYNC:
		return p.parseAsync(nil)
	case token.CLASS:
		return p.parseClassDef(nil)
	case token.IMPORT:
		return p.parseImport()
	case token.FROM:
		return p.parseImportFrom()
	case token.TYPE:
		return p.parseTypeOrSimple()
	case token.IF:
		return p.parseIf()
	case token.PASS:
		return p.parsePass()
	case token.FOR, token.WHILE, token.TRY, token.WITH, token.MATCH:
		return p.parsePassThrough()
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
		dec := p.parseExpr()
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
		body := p.parsePassThroughBlock()
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

	p.expect(token.COLON)
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
		bases, keywords = p.parseClassArgs()

		// If the arg list hit a statement keyword (unclosed paren), abandon this class.
		if p.isStmtStart() {
			return nil
		}

		p.expect(token.RPAREN)
	}

	p.expect(token.COLON)
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

// parseClassArgs parses the argument list inside class Foo(...).
// It distinguishes positional bases from keyword arguments (e.g., metaclass=ABCMeta).
func (p *Parser) parseClassArgs() ([]ast.Expr, []*ast.Keyword) {
	var bases []ast.Expr
	var keywords []*ast.Keyword

	for p.tok.Type != token.RPAREN && p.tok.Type != token.EOF {
		// Bail out if we hit a statement keyword — the paren was never closed.
		if p.isStmtStart() {
			p.errorf("unclosed parenthesis in class bases")
			break
		}

		// Check for keyword argument: name=expr
		if p.tok.Type == token.NAME {
			// Peek: is the next token '='?
			savedTok := p.tok
			p.next()
			if p.tok.Type == token.ASSIGN {
				// keyword argument
				p.next() // consume '='
				val := p.parseExpr()
				keywords = append(keywords, &ast.Keyword{
					Arg:   savedTok.Lit,
					Value: val,
				})
				if p.tok.Type == token.COMMA {
					p.next()
				}
				continue
			}
			// Not a keyword arg — back up. We already consumed the NAME,
			// so we need to parse the rest as a trailer on that name.
			expr := &ast.Name{
				Position: savedTok.Pos,
				EndPos:   token.Pos{Line: savedTok.Pos.Line, Col: savedTok.Pos.Col + len(savedTok.Lit), Offset: savedTok.Pos.Offset + len(savedTok.Lit)},
				Name:     savedTok.Lit,
			}
			// Parse any trailers (dots, subscripts, calls).
			result := p.parseTrailers(expr)
			// Handle binary operators that might follow (like |).
			result = p.parseExprFrom(result)
			bases = append(bases, result)
			if p.tok.Type == token.COMMA {
				p.next()
			}
			continue
		}

		expr := p.parseExpr()
		bases = append(bases, expr)
		if p.tok.Type == token.COMMA {
			p.next()
		}
	}

	return bases, keywords
}

// parseBlock parses a colon-delimited block: NEWLINE INDENT stmt+ DEDENT.
// Also handles single-line blocks (e.g., "def foo(): pass").
func (p *Parser) parseBlock() []ast.Stmt {
	// Single-line block (no NEWLINE INDENT).
	if p.tok.Type != token.NEWLINE {
		stmt := p.parseSimpleStmtInline()
		if stmt != nil {
			return []ast.Stmt{stmt}
		}
		return nil
	}

	p.next() // consume NEWLINE
	p.skipNewlines()

	if p.tok.Type != token.INDENT {
		// Empty block or malformed — don't error, just return empty.
		return nil
	}
	p.next() // consume INDENT

	var body []ast.Stmt
	for p.tok.Type != token.DEDENT && p.tok.Type != token.EOF {
		stmt := p.parseStmt()
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

// parseSimpleStmtInline parses a simple statement on the same line (e.g., pass, ..., expr).
func (p *Parser) parseSimpleStmtInline() ast.Stmt {
	if p.tok.Type == token.PASS {
		return p.parsePass()
	}
	return p.parseSimpleStmt()
}

// parsePass parses a 'pass' statement.
func (p *Parser) parsePass() ast.Stmt {
	pos := p.tok.Pos
	p.next() // consume 'pass'
	// Consume trailing newline if present.
	if p.tok.Type == token.NEWLINE {
		p.next()
	}
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
	// Consume trailing newline.
	if p.tok.Type == token.NEWLINE {
		p.next()
	}

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
	level := 0
	for p.tok.Type == token.DOT {
		level++
		p.next()
	}
	// Handle '...' (ELLIPSIS) as three dots.
	for p.tok.Type == token.ELLIPSIS {
		level += 3
		p.next()
	}

	// Optional module name.
	module := ""
	if p.tok.Type == token.NAME {
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

	endPos := p.tok.Pos
	if p.tok.Type == token.NEWLINE {
		p.next()
	}

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
// identifier (e.g., 'type = "foo"' or 'type(x)').
func (p *Parser) parseTypeOrSimple() ast.Stmt {
	pos := p.tok.Pos
	savedTok := p.tok
	p.next() // consume 'type'

	// If followed by NAME, it's a type alias statement.
	if p.tok.Type == token.NAME {
		name := p.expectName()

		// Optional type parameters.
		var typeParams []*ast.TypeParam
		if p.tok.Type == token.LBRACK {
			typeParams = p.parseTypeParams()
		}

		if p.tok.Type == token.ASSIGN {
			p.next() // consume '='
			value := p.parseExpr()
			endPos := value.End()
			if p.tok.Type == token.NEWLINE {
				p.next()
			}
			return &ast.TypeAliasDef{
				Position:   pos,
				EndPos:     endPos,
				Name:       name,
				TypeParams: typeParams,
				Value:      value,
			}
		}
	}

	// Fall back to treating 'type' as a name expression in a simple statement.
	nameExpr := &ast.Name{
		Position: savedTok.Pos,
		EndPos:   token.Pos{Line: savedTok.Pos.Line, Col: savedTok.Pos.Col + len(savedTok.Lit), Offset: savedTok.Pos.Offset + len(savedTok.Lit)},
		Name:     "type",
	}
	return p.finishSimpleStmt(nameExpr)
}

// parseIf parses 'if expr: block (elif expr: block)* (else: block)?'.
func (p *Parser) parseIf() ast.Stmt {
	pos := p.tok.Pos
	p.next() // consume 'if'

	test := p.parseExpr()
	p.expect(token.COLON)
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
		p.expect(token.COLON)
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

	test := p.parseExpr()
	p.expect(token.COLON)
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
		p.expect(token.COLON)
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

// parsePassThroughBlock skips tokens until ':' then parses the block.
func (p *Parser) parsePassThroughBlock() []ast.Stmt {
	// Skip to the colon.
	for !p.at(token.COLON, token.NEWLINE, token.EOF) {
		p.next()
	}
	if p.tok.Type == token.COLON {
		p.next()
		return p.parseBlock()
	}
	// If we hit NEWLINE without colon, skip it and move on.
	if p.tok.Type == token.NEWLINE {
		p.next()
	}
	return nil
}

// parseTryBlocks handles try: ... except: ... else: ... finally: ... blocks.
func (p *Parser) parseTryBlocks(pos token.Pos) []ast.Stmt {
	var allBody []ast.Stmt

	// Skip to first ':'
	for !p.at(token.COLON, token.NEWLINE, token.EOF) {
		p.next()
	}
	if p.tok.Type == token.COLON {
		p.next()
		allBody = append(allBody, p.parseBlock()...)
	}

	p.skipNewlines()

	// Handle except/else/finally blocks.
	for p.at(token.EXCEPT, token.ELSE, token.FINALLY) {
		p.next() // consume keyword
		// Skip to colon.
		for !p.at(token.COLON, token.NEWLINE, token.EOF) {
			p.next()
		}
		if p.tok.Type == token.COLON {
			p.next()
			allBody = append(allBody, p.parseBlock()...)
		}
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

	// Consume the rest of the line.
	for !p.at(token.NEWLINE, token.EOF, token.DEDENT) {
		p.next()
	}
	if p.tok.Type == token.NEWLINE {
		p.next()
	}

	return &ast.PassThrough{
		Kind:     kind,
		Position: pos,
		EndPos:   pos,
	}
}

// parseSimpleStmt parses an expression statement, assignment, or annotated assignment.
func (p *Parser) parseSimpleStmt() ast.Stmt {
	expr := p.parseExpr()
	if expr == nil {
		p.errorf("expected statement, got %s", p.tok.Type)
		p.syncStmt()
		if p.tok.Type == token.NEWLINE {
			p.next()
		}
		return nil
	}

	return p.finishSimpleStmt(expr)
}

// finishSimpleStmt completes a simple statement after the initial expression is parsed.
// This handles assignment (=), annotated assignment (:), or bare expression statement.
func (p *Parser) finishSimpleStmt(expr ast.Expr) ast.Stmt {
	switch p.tok.Type {
	case token.ASSIGN:
		// Assignment: target = value
		p.next()
		value := p.parseExpr()
		endPos := p.tok.Pos
		if p.tok.Type == token.NEWLINE {
			p.next()
		}
		return &ast.Assign{
			Position: expr.Pos(),
			EndPos:   endPos,
			Targets:  []ast.Expr{expr},
			Value:    value,
		}

	case token.PLUSEQ, token.MINUSEQ, token.STAREQ, token.SLASHEQ,
		token.DSLASHEQ, token.PERCENTEQ, token.DSTAREQ,
		token.AMPEREQ, token.PIPEEQ, token.CARETEQ,
		token.RSHIFTEQ, token.LSHIFTEQ:
		// Augmented assignment: target op= value
		p.next()
		value := p.parseExpr()
		endPos := p.tok.Pos
		if p.tok.Type == token.NEWLINE {
			p.next()
		}
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
			value = p.parseExpr()
		}
		endPos := p.tok.Pos
		if p.tok.Type == token.NEWLINE {
			p.next()
		}
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
		if p.tok.Type == token.NEWLINE {
			p.next()
		}
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

	for p.tok.Type != token.RPAREN && p.tok.Type != token.EOF {
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
			if p.tok.Type == token.NAME {
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

// parseTypeParams parses type parameters: '[' name (',' name)* ']'.
func (p *Parser) parseTypeParams() []*ast.TypeParam {
	p.next() // consume '['
	var params []*ast.TypeParam

	for p.tok.Type != token.RBRACK && p.tok.Type != token.EOF {
		tp := &ast.TypeParam{Position: p.tok.Pos}
		tp.Name = p.expectName()

		// Optional bound: ': expr' or default via '=' (simplified).
		if p.tok.Type == token.COLON {
			p.next()
			tp.Bound = p.parseExpr()
		}

		tp.EndPos = p.tok.Pos
		params = append(params, tp)

		if p.tok.Type == token.COMMA {
			p.next()
		}
	}

	p.expect(token.RBRACK)
	return params
}
