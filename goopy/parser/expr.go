package parser

import (
	"strings"

	"github.com/pypx/goopy/ast"
	"github.com/pypx/goopy/token"
)

// ---------------------------------------------------------------------------
// Expression parsing — recursive descent following Python's operator precedence
//
// Precedence (lowest to highest):
//   or → and → not → comparison → bitor → bitxor → bitand → shift →
//   arith → term → unary → power → atom_expr (with trailers)
// ---------------------------------------------------------------------------

// parseExpr is the entry point for expression parsing.
func (p *Parser) parseExpr() ast.Expr {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > maxExprDepth {
		return p.depthError()
	}
	expr := p.parseOrExpr()
	if expr == nil {
		return nil
	}

	// Handle ternary if expression: body if test else orelse.
	if p.tok.Type == token.IF {
		p.next()
		test := p.parseOrExpr()
		p.expect(token.ELSE)
		orelse := p.parseExpr()
		return &ast.IfExpr{
			Position: expr.Pos(),
			EndPos:   orelse.End(),
			Test:     test,
			Body:     expr,
			Orelse:   orelse,
		}
	}

	return expr
}

// depthError records that an expression nests too deep and drops the line.
func (p *Parser) depthError() ast.Expr {
	p.errorf("expression nesting too deep (max %d)", maxExprDepth)
	p.syncStmt()
	pos := p.tok.Pos
	return &ast.Constant{
		Position: pos,
		EndPos:   pos,
		Value:    "?",
		Kind:     "error",
		Lit:      "?",
	}
}

func (p *Parser) parseOrExpr() ast.Expr {
	left := p.parseAndExpr()
	for p.tok.Type == token.OR {
		op := p.tok.Type
		p.next()
		right := p.parseAndExpr()
		left = &ast.BinOp{
			Position: left.Pos(),
			EndPos:   right.End(),
			Left:     left,
			Op:       op,
			Right:    right,
		}
	}
	return left
}

func (p *Parser) parseAndExpr() ast.Expr {
	left := p.parseNotExpr()
	for p.tok.Type == token.AND {
		op := p.tok.Type
		p.next()
		right := p.parseNotExpr()
		left = &ast.BinOp{
			Position: left.Pos(),
			EndPos:   right.End(),
			Left:     left,
			Op:       op,
			Right:    right,
		}
	}
	return left
}

func (p *Parser) parseNotExpr() ast.Expr {
	if p.tok.Type == token.NOT {
		p.depth++
		defer func() { p.depth-- }()
		if p.depth > maxExprDepth {
			return p.depthError()
		}
		pos := p.tok.Pos
		p.next()
		operand := p.parseNotExpr()
		return &ast.UnaryOp{
			Position: pos,
			EndPos:   operand.End(),
			Op:       token.NOT,
			Operand:  operand,
		}
	}
	return p.parseComparison()
}

func (p *Parser) parseComparison() ast.Expr {
	left := p.parseBitOrExpr()
	for p.isCompOp() {
		op := p.consumeCompOp()
		right := p.parseBitOrExpr()
		left = &ast.BinOp{
			Position: left.Pos(),
			EndPos:   right.End(),
			Left:     left,
			Op:       op,
			Right:    right,
		}
	}
	return left
}

// isCompOp returns true if the current token starts a comparison operator.
func (p *Parser) isCompOp() bool {
	switch p.tok.Type {
	case token.EQ, token.NEQ, token.LT, token.GT, token.LTE, token.GTE, token.IN, token.IS:
		return true
	case token.NOT:
		// 'not in'
		return true
	}
	return false
}

// consumeCompOp consumes a comparison operator and returns its token type.
// Handles composite operators like 'not in' and 'is not'.
func (p *Parser) consumeCompOp() token.Type {
	switch p.tok.Type {
	case token.NOT:
		p.next() // consume 'not'
		p.expect(token.IN)
		return token.NOT // represent 'not in' as NOT
	case token.IS:
		p.next()
		if p.tok.Type == token.NOT {
			p.next()
			return token.IS // represent 'is not' as IS (simplified)
		}
		return token.IS
	default:
		op := p.tok.Type
		p.next()
		return op
	}
}

func (p *Parser) parseBitOrExpr() ast.Expr {
	left := p.parseBitXorExpr()
	for p.tok.Type == token.PIPE {
		op := p.tok.Type
		p.next()
		right := p.parseBitXorExpr()
		left = &ast.BinOp{
			Position: left.Pos(),
			EndPos:   right.End(),
			Left:     left,
			Op:       op,
			Right:    right,
		}
	}
	return left
}

func (p *Parser) parseBitXorExpr() ast.Expr {
	left := p.parseBitAndExpr()
	for p.tok.Type == token.CARET {
		op := p.tok.Type
		p.next()
		right := p.parseBitAndExpr()
		left = &ast.BinOp{
			Position: left.Pos(),
			EndPos:   right.End(),
			Left:     left,
			Op:       op,
			Right:    right,
		}
	}
	return left
}

func (p *Parser) parseBitAndExpr() ast.Expr {
	left := p.parseShiftExpr()
	for p.tok.Type == token.AMPER {
		op := p.tok.Type
		p.next()
		right := p.parseShiftExpr()
		left = &ast.BinOp{
			Position: left.Pos(),
			EndPos:   right.End(),
			Left:     left,
			Op:       op,
			Right:    right,
		}
	}
	return left
}

func (p *Parser) parseShiftExpr() ast.Expr {
	left := p.parseArithExpr()
	for p.tok.Type == token.LSHIFT || p.tok.Type == token.RSHIFT {
		op := p.tok.Type
		p.next()
		right := p.parseArithExpr()
		left = &ast.BinOp{
			Position: left.Pos(),
			EndPos:   right.End(),
			Left:     left,
			Op:       op,
			Right:    right,
		}
	}
	return left
}

func (p *Parser) parseArithExpr() ast.Expr {
	left := p.parseTerm()
	for p.tok.Type == token.PLUS || p.tok.Type == token.MINUS {
		op := p.tok.Type
		p.next()
		right := p.parseTerm()
		left = &ast.BinOp{
			Position: left.Pos(),
			EndPos:   right.End(),
			Left:     left,
			Op:       op,
			Right:    right,
		}
	}
	return left
}

func (p *Parser) parseTerm() ast.Expr {
	left := p.parseUnary()
	for p.at(token.STAR, token.SLASH, token.DSLASH, token.PERCENT, token.AT) {
		op := p.tok.Type
		p.next()
		right := p.parseUnary()
		left = &ast.BinOp{
			Position: left.Pos(),
			EndPos:   right.End(),
			Left:     left,
			Op:       op,
			Right:    right,
		}
	}
	return left
}

func (p *Parser) parseUnary() ast.Expr {
	if p.at(token.PLUS, token.MINUS, token.TILDE) {
		p.depth++
		defer func() { p.depth-- }()
		if p.depth > maxExprDepth {
			return p.depthError()
		}
		pos := p.tok.Pos
		op := p.tok.Type
		p.next()
		operand := p.parseUnary()
		return &ast.UnaryOp{
			Position: pos,
			EndPos:   operand.End(),
			Op:       op,
			Operand:  operand,
		}
	}
	return p.parsePower()
}

func (p *Parser) parsePower() ast.Expr {
	var base ast.Expr
	if p.tok.Type == token.AWAIT {
		pos := p.tok.Pos
		p.next()
		val := p.parseAtomExpr()
		base = &ast.Await{Position: pos, EndPos: val.End(), Value: val}
	} else {
		base = p.parseAtomExpr()
	}
	if p.tok.Type == token.DSTAR {
		p.depth++
		defer func() { p.depth-- }()
		if p.depth > maxExprDepth {
			return p.depthError()
		}
		op := p.tok.Type
		p.next()
		exp := p.parseUnary()
		return &ast.BinOp{
			Position: base.Pos(),
			EndPos:   exp.End(),
			Left:     base,
			Op:       op,
			Right:    exp,
		}
	}
	return base
}

func (p *Parser) parseAtomExpr() ast.Expr {
	atom := p.parseAtom()
	if atom == nil {
		return nil
	}
	return p.parseTrailers(atom)
}

// parseTrailers parses trailing '.attr', '(args)', and '[subscript]' on an expression.
func (p *Parser) parseTrailers(expr ast.Expr) ast.Expr {
	for {
		switch p.tok.Type {
		case token.DOT:
			p.next()
			name := p.expectName()
			expr = &ast.Attribute{
				Position: expr.Pos(),
				EndPos:   p.tok.Pos,
				Value:    expr,
				Attr:     name,
			}

		case token.LPAREN:
			expr = p.parseCallTrailer(expr)

		case token.LBRACK:
			p.next()
			slice := p.parseSubscriptContents()
			endPos := p.tok.Pos
			p.expect(token.RBRACK)
			expr = &ast.Subscript{
				Position: expr.Pos(),
				EndPos:   endPos,
				Value:    expr,
				Slice:    slice,
			}

		default:
			return expr
		}
	}
}

// parseCallTrailer parses '(' arglist ')' as a Call node.
func (p *Parser) parseCallTrailer(fn ast.Expr) ast.Expr {
	p.next() // consume '('
	args, keywords := p.parseArgList()

	endPos := p.tok.Pos
	p.expect(token.RPAREN)

	return &ast.Call{
		Position: fn.Pos(),
		EndPos:   endPos,
		Func:     fn,
		Args:     args,
		Keywords: keywords,
	}
}

// parseArgList parses the arguments of a call or a class definition, up to the
// closing parenthesis. Each argument is a full expression, a keyword argument, or
// a * or ** unpack. A generator expression argument is skimmed.
func (p *Parser) parseArgList() ([]ast.Expr, []*ast.Keyword) {
	var args []ast.Expr
	var keywords []*ast.Keyword

	for p.tok.Type != token.RPAREN && !p.atLineEnd() {
		// Bail out if we hit a statement keyword: the paren was never closed.
		if p.isStmtStart() {
			break
		}

		switch p.tok.Type {
		case token.DSTAR:
			p.next()
			keywords = append(keywords, &ast.Keyword{Value: p.parseExpr()})
		case token.STAR:
			pos := p.tok.Pos
			p.next()
			val := p.parseExpr()
			args = append(args, &ast.Starred{Position: pos, EndPos: val.End(), Value: val})
		default:
			arg := p.parseNamedExpr()
			if n, ok := arg.(*ast.Name); ok && p.tok.Type == token.ASSIGN {
				p.next()
				keywords = append(keywords, &ast.Keyword{Arg: n.Name, Value: p.parseExpr()})
			} else {
				args = append(args, arg)
			}
		}

		if p.at(token.FOR, token.ASYNC) {
			p.skipUntil(token.RPAREN)
			break
		}
		if p.tok.Type != token.COMMA {
			break
		}
		p.next()
	}

	return args, keywords
}

// parseNamedExpr parses expr [':=' expr].
func (p *Parser) parseNamedExpr() ast.Expr {
	x := p.parseExpr()
	if p.tok.Type != token.WALRUS {
		return x
	}
	p.next()
	value := p.parseExpr()
	return &ast.NamedExpr{Position: x.Pos(), EndPos: value.End(), Target: x, Value: value}
}

// parseExprList parses an unparenthesised tuple: expr (',' expr)* [','].
// A single expression with no comma stays as that expression.
func (p *Parser) parseExprList() ast.Expr {
	first := p.parseExpr()
	if p.tok.Type != token.COMMA {
		return first
	}
	elts := []ast.Expr{first}
	for p.tok.Type == token.COMMA {
		p.next()
		if p.atLineEnd() || p.at(token.ASSIGN, token.SEMI, token.COLON, token.RPAREN, token.RBRACK, token.RBRACE) {
			break
		}
		elts = append(elts, p.parseExpr())
	}
	return &ast.Tuple{Position: first.Pos(), EndPos: elts[len(elts)-1].End(), Elts: elts}
}

// parseSubscriptContents parses the contents inside [...]. Several comma-separated
// items, such as dict[str, int] or y[1, :], become a Tuple.
func (p *Parser) parseSubscriptContents() ast.Expr {
	first := p.parseSliceItem()
	if p.tok.Type != token.COMMA {
		return first
	}

	elts := []ast.Expr{first}
	for p.tok.Type == token.COMMA {
		p.next()
		if p.tok.Type == token.RBRACK {
			break
		}
		elts = append(elts, p.parseSliceItem())
	}

	return &ast.Tuple{
		Position: first.Pos(),
		EndPos:   elts[len(elts)-1].End(),
		Elts:     elts,
	}
}

// parseSliceItem parses one subscript item: a named expression or a slice.
func (p *Parser) parseSliceItem() ast.Expr {
	pos := p.tok.Pos
	var lower ast.Expr
	if p.tok.Type != token.COLON {
		lower = p.parseNamedExpr()
		if p.tok.Type != token.COLON {
			return lower
		}
	}
	s := &ast.Slice{Position: pos, Lower: lower}
	p.next() // consume ':'
	if !p.atSliceItemEnd() && p.tok.Type != token.COLON {
		s.Upper = p.parseExpr()
	}
	if p.tok.Type == token.COLON {
		p.next()
		if !p.atSliceItemEnd() {
			s.Step = p.parseExpr()
		}
	}
	s.EndPos = p.tok.Pos
	return s
}

// atSliceItemEnd returns true at a token that ends a subscript item.
func (p *Parser) atSliceItemEnd() bool {
	return p.at(token.COMMA, token.RBRACK) || p.atLineEnd()
}

// ---------------------------------------------------------------------------
// Atom parsing
// ---------------------------------------------------------------------------

func (p *Parser) parseAtom() ast.Expr {
	switch p.tok.Type {
	case token.NAME, token.TYPE, token.MATCH, token.CASE:
		// TYPE, MATCH, CASE are soft keywords in Python 3.12+ — they can
		// appear as regular identifiers in expression context.
		return p.parseName()
	case token.NUMBER:
		return p.parseNumber()
	case token.STRING:
		return p.parseString()
	case token.NONE:
		return p.parseNone()
	case token.TRUE:
		return p.parseBool("True")
	case token.FALSE:
		return p.parseBool("False")
	case token.ELLIPSIS:
		return p.parseEllipsis()
	case token.LPAREN:
		return p.parseParenExpr()
	case token.LBRACK:
		return p.parseListExpr()
	case token.LBRACE:
		return p.parseDictOrSetExpr()
	case token.STAR:
		// Starred expression.
		pos := p.tok.Pos
		p.next()
		val := p.parseExpr()
		return &ast.Starred{
			Position: pos,
			EndPos:   val.End(),
			Value:    val,
		}
	case token.LAMBDA:
		return p.parseLambda()
	case token.YIELD:
		return p.parseYield()
	default:
		// Unrecognized token: record an error and skip it, unless it is structural.
		// The statement loops guarantee progress at structural tokens.
		p.errorf("unexpected token %s in expression", p.tok.Type)
		pos := p.tok.Pos
		if !p.atLineEnd() {
			p.next()
		}
		return &ast.Constant{
			Position: pos,
			EndPos:   pos,
			Value:    "?",
			Kind:     "error",
			Lit:      "?",
		}
	}
}

func (p *Parser) parseName() ast.Expr {
	t := p.tok
	p.next()
	return &ast.Name{
		Position: t.Pos,
		EndPos:   token.Pos{Line: t.Pos.Line, Col: t.Pos.Col + len(t.Lit), Offset: t.Pos.Offset + len(t.Lit)},
		Name:     t.Lit,
	}
}

func (p *Parser) parseNumber() ast.Expr {
	t := p.tok
	p.next()

	kind := "int"
	lit := t.Lit
	if strings.ContainsAny(lit, ".eEjJ") {
		kind = "float"
	}

	return &ast.Constant{
		Position: t.Pos,
		EndPos:   token.Pos{Line: t.Pos.Line, Col: t.Pos.Col + len(lit), Offset: t.Pos.Offset + len(lit)},
		Value:    lit,
		Kind:     kind,
		Lit:      lit,
	}
}

// parseString parses one or more adjacent string tokens into a single Constant.
// Python concatenates adjacent string literals: "a" "b" becomes "ab".
func (p *Parser) parseString() ast.Expr {
	t := p.tok
	p.next()

	lit := t.Lit
	value := stripQuotes(lit)

	// Concatenate adjacent strings.
	for p.tok.Type == token.STRING {
		lit += " " + p.tok.Lit
		value += stripQuotes(p.tok.Lit)
		p.next()
	}

	return &ast.Constant{
		Position: t.Pos,
		EndPos:   token.Pos{Line: t.Pos.Line, Col: t.Pos.Col + len(lit), Offset: t.Pos.Offset + len(lit)},
		Value:    value,
		Kind:     "str",
		Lit:      lit,
	}
}

// stripQuotes removes the quote delimiters and optional prefix from a string literal.
func stripQuotes(s string) string {
	// Remove prefix (r, b, f, u, rb, br, etc.)
	i := 0
	for i < len(s) && isStringPrefixChar(s[i]) {
		i++
	}
	s = s[i:]

	// Remove triple quotes or single quotes.
	if len(s) >= 6 && (s[:3] == `"""` || s[:3] == `'''`) {
		return s[3 : len(s)-3]
	}
	if len(s) >= 2 {
		return s[1 : len(s)-1]
	}
	return s
}

func isStringPrefixChar(c byte) bool {
	return c == 'r' || c == 'R' || c == 'b' || c == 'B' ||
		c == 'f' || c == 'F' || c == 'u' || c == 'U' ||
		c == 't' || c == 'T'
}

func (p *Parser) parseNone() ast.Expr {
	t := p.tok
	p.next()
	return &ast.Constant{
		Position: t.Pos,
		EndPos:   token.Pos{Line: t.Pos.Line, Col: t.Pos.Col + 4, Offset: t.Pos.Offset + 4},
		Value:    "None",
		Kind:     "none",
		Lit:      "None",
	}
}

func (p *Parser) parseBool(val string) ast.Expr {
	t := p.tok
	p.next()
	return &ast.Constant{
		Position: t.Pos,
		EndPos:   token.Pos{Line: t.Pos.Line, Col: t.Pos.Col + len(val), Offset: t.Pos.Offset + len(val)},
		Value:    val,
		Kind:     "bool",
		Lit:      val,
	}
}

func (p *Parser) parseEllipsis() ast.Expr {
	t := p.tok
	p.next()
	return &ast.Constant{
		Position: t.Pos,
		EndPos:   token.Pos{Line: t.Pos.Line, Col: t.Pos.Col + 3, Offset: t.Pos.Offset + 3},
		Value:    "...",
		Kind:     "ellipsis",
		Lit:      "...",
	}
}

// parseParenExpr parses '(' expr ')' or '(' ')' for empty tuple, or '(' expr ',' ... ')' for tuple.
func (p *Parser) parseParenExpr() ast.Expr {
	pos := p.tok.Pos
	p.next() // consume '('

	// Empty tuple.
	if p.tok.Type == token.RPAREN {
		endPos := p.tok.Pos
		p.next()
		return &ast.Tuple{
			Position: pos,
			EndPos:   endPos,
		}
	}

	first := p.parseNamedExpr()

	// Generator expression: (expr for ...)
	if p.at(token.FOR, token.ASYNC) {
		p.skipUntil(token.RPAREN)
		endPos := p.tok.Pos
		p.expect(token.RPAREN)
		return &ast.Tuple{Position: pos, EndPos: endPos, Elts: []ast.Expr{first}}
	}

	// Check for tuple: (a, b, ...) or (a,)
	if p.tok.Type == token.COMMA {
		elts := []ast.Expr{first}
		for p.tok.Type == token.COMMA {
			p.next()
			if p.tok.Type == token.RPAREN {
				break
			}
			elts = append(elts, p.parseNamedExpr())
		}
		endPos := p.tok.Pos
		p.expect(token.RPAREN)
		return &ast.Tuple{
			Position: pos,
			EndPos:   endPos,
			Elts:     elts,
		}
	}

	// Parenthesized expression.
	p.expect(token.RPAREN)
	return first
}

// parseListExpr parses '[' elts ']' or '[' expr 'for' ... ']' (list comprehension).
func (p *Parser) parseListExpr() ast.Expr {
	pos := p.tok.Pos
	p.next() // consume '['

	// Empty list.
	if p.tok.Type == token.RBRACK {
		endPos := p.tok.Pos
		p.next()
		return &ast.List{Position: pos, EndPos: endPos}
	}

	first := p.parseNamedExpr()

	// List comprehension: [expr for ...]
	if p.at(token.FOR, token.ASYNC) {
		p.skipUntil(token.RBRACK)
		endPos := p.tok.Pos
		p.expect(token.RBRACK)
		return &ast.List{Position: pos, EndPos: endPos, Elts: []ast.Expr{first}}
	}

	elts := []ast.Expr{first}
	for p.tok.Type == token.COMMA {
		p.next()
		if p.tok.Type == token.RBRACK {
			break
		}
		elts = append(elts, p.parseNamedExpr())
	}

	endPos := p.tok.Pos
	p.expect(token.RBRACK)

	return &ast.List{
		Position: pos,
		EndPos:   endPos,
		Elts:     elts,
	}
}

// parseDictOrSetExpr parses '{' ... '}', distinguishing dict from set.
func (p *Parser) parseDictOrSetExpr() ast.Expr {
	pos := p.tok.Pos
	p.next() // consume '{'

	// Empty dict.
	if p.tok.Type == token.RBRACE {
		endPos := p.tok.Pos
		p.next()
		return &ast.Dict{Position: pos, EndPos: endPos}
	}

	// Dict starting with an unpack, or an unpacking comprehension: {**x for ...}.
	if p.tok.Type == token.DSTAR {
		p.next()
		firstVal := p.parseExpr()
		if p.at(token.FOR, token.ASYNC) {
			p.skipUntil(token.RBRACE)
			endPos := p.tok.Pos
			p.expect(token.RBRACE)
			return &ast.Dict{Position: pos, EndPos: endPos, Keys: []ast.Expr{nil}, Values: []ast.Expr{firstVal}}
		}
		return p.parseDictItems(pos, nil, firstVal)
	}

	first := p.parseNamedExpr()

	if p.tok.Type == token.COLON {
		// Dict literal or dict comprehension.
		p.next()
		firstVal := p.parseExpr()

		// Dict comprehension: {k: v for ...}
		if p.at(token.FOR, token.ASYNC) {
			p.skipUntil(token.RBRACE)
			endPos := p.tok.Pos
			p.expect(token.RBRACE)
			return &ast.Dict{
				Position: pos,
				EndPos:   endPos,
				Keys:     []ast.Expr{first},
				Values:   []ast.Expr{firstVal},
			}
		}

		return p.parseDictItems(pos, first, firstVal)
	}

	// Set comprehension: {expr for ...}
	if p.at(token.FOR, token.ASYNC) {
		p.skipUntil(token.RBRACE)
		endPos := p.tok.Pos
		p.expect(token.RBRACE)
		return &ast.Set{Position: pos, EndPos: endPos, Elts: []ast.Expr{first}}
	}

	// Set literal.
	elts := []ast.Expr{first}
	for p.tok.Type == token.COMMA {
		p.next()
		if p.tok.Type == token.RBRACE {
			break
		}
		elts = append(elts, p.parseNamedExpr())
	}
	endPos := p.tok.Pos
	p.expect(token.RBRACE)
	return &ast.Set{
		Position: pos,
		EndPos:   endPos,
		Elts:     elts,
	}
}

// parseDictItems parses the rest of a dict display after its first entry, up to
// and including the closing brace. A nil key marks a ** unpack entry.
func (p *Parser) parseDictItems(pos token.Pos, firstKey, firstVal ast.Expr) ast.Expr {
	keys := []ast.Expr{firstKey}
	vals := []ast.Expr{firstVal}
	for p.tok.Type == token.COMMA {
		p.next()
		if p.tok.Type == token.RBRACE {
			break
		}
		if p.tok.Type == token.DSTAR {
			p.next()
			keys = append(keys, nil)
			vals = append(vals, p.parseExpr())
			continue
		}
		k := p.parseExpr()
		p.expect(token.COLON)
		keys = append(keys, k)
		vals = append(vals, p.parseExpr())
	}
	endPos := p.tok.Pos
	p.expect(token.RBRACE)
	return &ast.Dict{Position: pos, EndPos: endPos, Keys: keys, Values: vals}
}

// parseYield parses 'yield', 'yield exprlist' or 'yield from expr'.
func (p *Parser) parseYield() ast.Expr {
	y := &ast.Yield{Position: p.tok.Pos, EndPos: p.tok.Pos}
	p.next() // consume 'yield'
	switch {
	case p.tok.Type == token.FROM:
		p.next()
		y.From = true
		y.Value = p.parseExpr()
	case p.atLineEnd() || p.at(token.RPAREN, token.RBRACK, token.RBRACE, token.SEMI, token.ASSIGN, token.COLON, token.COMMA):
		return y
	default:
		y.Value = p.parseExprList()
	}
	y.EndPos = y.Value.End()
	return y
}

// parseLambda parses 'lambda params: expr'. We treat it as a passthrough
// by returning the whole thing as an opaque expression.
func (p *Parser) parseLambda() ast.Expr {
	pos := p.tok.Pos
	p.next() // consume 'lambda'

	// Skip to the colon — we don't deeply model lambda params.
	p.skipHeader()
	if p.tok.Type == token.COLON {
		p.next()
	}

	body := p.parseExpr()
	if body == nil {
		return &ast.Constant{
			Position: pos,
			EndPos:   pos,
			Value:    "...",
			Kind:     "ellipsis",
			Lit:      "...",
		}
	}
	return body
}
