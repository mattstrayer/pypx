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

// parseExprFrom continues parsing an expression from a partially parsed left operand.
// Used when we've already consumed a NAME token for lookahead purposes.
func (p *Parser) parseExprFrom(left ast.Expr) ast.Expr {
	// Continue with any binary operators that might follow.
	return p.parseBinOpFrom(left)
}

// parseBinOpFrom handles binary operators starting from a pre-parsed left side,
// beginning at the 'or' precedence level.
func (p *Parser) parseBinOpFrom(left ast.Expr) ast.Expr {
	// We need to thread the left operand through the precedence chain.
	// The simplest approach: check for pipe (union type) since that's
	// the most common case in class base lists.
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
	for p.at(token.STAR, token.SLASH, token.DSLASH, token.PERCENT) {
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
	base := p.parseAtomExpr()
	if p.tok.Type == token.DSTAR {
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
	var args []ast.Expr
	var keywords []*ast.Keyword

	for p.tok.Type != token.RPAREN && p.tok.Type != token.EOF {
		// Bail out if we hit a statement keyword — the paren was never closed.
		// But FOR is valid inside calls (generator expressions), so exclude it.
		if p.isStmtStart() && p.tok.Type != token.FOR {
			break
		}

		// Check for keyword argument: name=expr
		// Also accept soft keywords (type, match, case) as keyword arg names.
		if p.tok.Type == token.NAME || p.isSoftKeyword() {
			savedTok := p.tok
			p.next()
			if p.tok.Type == token.ASSIGN {
				p.next()
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
			// Not keyword — reconstruct and parse as regular arg.
			nameExpr := &ast.Name{
				Position: savedTok.Pos,
				EndPos:   token.Pos{Line: savedTok.Pos.Line, Col: savedTok.Pos.Col + len(savedTok.Lit), Offset: savedTok.Pos.Offset + len(savedTok.Lit)},
				Name:     savedTok.Lit,
			}
			expr := p.parseTrailers(nameExpr)
			expr = p.parseExprFrom(expr)
			args = append(args, expr)
			// Generator expression: func(name for ...)
			if p.tok.Type == token.FOR {
				p.skipUntil(token.RPAREN)
				break
			}
			if p.tok.Type == token.COMMA {
				p.next()
			}
			continue
		}

		// Check for **kwargs in call.
		if p.tok.Type == token.DSTAR {
			p.next()
			val := p.parseExpr()
			keywords = append(keywords, &ast.Keyword{Value: val})
			if p.tok.Type == token.COMMA {
				p.next()
			}
			continue
		}

		// Check for *args in call.
		if p.tok.Type == token.STAR {
			pos := p.tok.Pos
			p.next()
			val := p.parseExpr()
			args = append(args, &ast.Starred{
				Position: pos,
				EndPos:   val.End(),
				Value:    val,
			})
			if p.tok.Type == token.COMMA {
				p.next()
			}
			continue
		}

		arg := p.parseExpr()
		if arg != nil {
			args = append(args, arg)
		}
		// Generator expression in call: func(expr for ...)
		if p.tok.Type == token.FOR {
			p.skipUntil(token.RPAREN)
			break
		}
		if p.tok.Type == token.COMMA {
			p.next()
		}
	}

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

// parseSubscriptContents parses the contents inside [...].
// For comma-separated items (e.g., dict[str, int]), returns a Tuple.
// For slice syntax (e.g., value[:end]), skips to ] since we don't deeply
// model slice expressions for doc extraction.
func (p *Parser) parseSubscriptContents() ast.Expr {
	// Slice with no start: [:end] or [::step]
	if p.tok.Type == token.COLON {
		p.skipUntil(token.RBRACK)
		return &ast.Constant{
			Position: p.tok.Pos,
			EndPos:   p.tok.Pos,
			Value:    "...",
			Kind:     "ellipsis",
			Lit:      "...",
		}
	}

	first := p.parseExpr()

	// Slice with start: [start:end] or [start:end:step]
	if p.tok.Type == token.COLON {
		p.skipUntil(token.RBRACK)
		return first
	}

	if p.tok.Type != token.COMMA {
		return first
	}

	// Multiple items — build a Tuple.
	elts := []ast.Expr{first}
	for p.tok.Type == token.COMMA {
		p.next()
		if p.tok.Type == token.RBRACK {
			break
		}
		elts = append(elts, p.parseExpr())
	}

	return &ast.Tuple{
		Position: first.Pos(),
		EndPos:   elts[len(elts)-1].End(),
		Elts:     elts,
	}
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
	default:
		// Unrecognized token — record error and advance to prevent infinite loops.
		p.errorf("unexpected token %s in expression", p.tok.Type)
		pos := p.tok.Pos
		if p.tok.Type != token.EOF {
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
		c == 'f' || c == 'F' || c == 'u' || c == 'U'
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

	first := p.parseExpr()

	// Generator expression: (expr for ...)
	if p.tok.Type == token.FOR {
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
			elts = append(elts, p.parseExpr())
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

	first := p.parseExpr()

	// List comprehension: [expr for ...]
	if p.tok.Type == token.FOR {
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
		elts = append(elts, p.parseExpr())
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

	first := p.parseExpr()

	if p.tok.Type == token.COLON {
		// Dict literal or dict comprehension.
		p.next()
		firstVal := p.parseExpr()

		// Dict comprehension: {k: v for ...}
		if p.tok.Type == token.FOR {
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

		keys := []ast.Expr{first}
		vals := []ast.Expr{firstVal}
		for p.tok.Type == token.COMMA {
			p.next()
			if p.tok.Type == token.RBRACE {
				break
			}
			k := p.parseExpr()
			p.expect(token.COLON)
			v := p.parseExpr()
			keys = append(keys, k)
			vals = append(vals, v)
		}
		endPos := p.tok.Pos
		p.expect(token.RBRACE)
		return &ast.Dict{
			Position: pos,
			EndPos:   endPos,
			Keys:     keys,
			Values:   vals,
		}
	}

	// Set comprehension: {expr for ...}
	if p.tok.Type == token.FOR {
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
		elts = append(elts, p.parseExpr())
	}
	endPos := p.tok.Pos
	p.expect(token.RBRACE)
	return &ast.Set{
		Position: pos,
		EndPos:   endPos,
		Elts:     elts,
	}
}

// parseLambda parses 'lambda params: expr'. We treat it as a passthrough
// by returning the whole thing as an opaque expression.
func (p *Parser) parseLambda() ast.Expr {
	pos := p.tok.Pos
	p.next() // consume 'lambda'

	// Skip to the colon — we don't deeply model lambda params.
	for !p.at(token.COLON, token.NEWLINE, token.EOF) {
		p.next()
	}
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
