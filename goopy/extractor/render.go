package extractor

import (
	"strings"

	"github.com/pypx/goopy/ast"
	"github.com/pypx/goopy/token"
)

// precedence orders expressions by how tightly they bind, loosest first. The AST
// does not record parentheses, so the renderer writes them from this order, as
// CPython's ast.unparse does: an operand that binds looser than its position
// allows gets parentheses.
type precedence int

const (
	precNamed precedence = iota // :=
	precTuple                   // a, b
	precYield                   // yield, yield from
	precTest                    // if-else, lambda
	precOr
	precAnd
	precNot
	precCmp
	precBitOr
	precBitXor
	precBitAnd
	precShift
	precArith
	precTerm
	precFactor // unary +, -, ~
	precPower  // **
	precAwait
	precAtom
)

// exprToString renders an AST expression as source text. Strings and numbers stay
// as written. The result parses back to the same expression.
func exprToString(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	if n, ok := expr.(*ast.Name); ok {
		return n.Name
	}
	var b strings.Builder
	writeExpr(&b, expr, precTest)
	return b.String()
}

// writeExpr writes expr where the context needs at least atLeast, with parentheses if
// expr binds looser than that. A nil expr writes nothing.
func writeExpr(b *strings.Builder, expr ast.Expr, atLeast precedence) {
	if expr == nil {
		return
	}
	if exprPrecedence(expr) < atLeast {
		b.WriteByte('(')
		writeBare(b, expr)
		b.WriteByte(')')
		return
	}
	writeBare(b, expr)
}

// exprPrecedence returns how tightly expr binds on its own.
func exprPrecedence(expr ast.Expr) precedence {
	switch e := expr.(type) {
	case *ast.NamedExpr:
		return precNamed
	case *ast.Tuple:
		if len(e.Elts) == 0 {
			return precAtom // "()" is the empty tuple whatever surrounds it
		}
		return precTuple
	case *ast.Yield:
		return precYield
	case *ast.Lambda, *ast.IfExpr:
		return precTest
	case *ast.BinOp:
		return binOpPrecedence(e.Op)
	case *ast.UnaryOp:
		if e.Op == token.NOT {
			return precNot
		}
		return precFactor
	case *ast.Await:
		return precAwait
	default:
		return precAtom
	}
}

func binOpPrecedence(op token.Type) precedence {
	switch op {
	case token.OR:
		return precOr
	case token.AND:
		return precAnd
	case token.EQ, token.NEQ, token.LT, token.GT, token.LTE, token.GTE, token.IN, token.IS:
		return precCmp
	case token.PIPE:
		return precBitOr
	case token.CARET:
		return precBitXor
	case token.AMPER:
		return precBitAnd
	case token.LSHIFT, token.RSHIFT:
		return precShift
	case token.PLUS, token.MINUS:
		return precArith
	case token.DSTAR:
		return precPower
	default: // * / // % @
		return precTerm
	}
}

// writeBare writes expr with no parentheses around the whole of it.
func writeBare(b *strings.Builder, expr ast.Expr) {
	switch e := expr.(type) {
	case *ast.Name:
		b.WriteString(e.Name)
	case *ast.Constant:
		b.WriteString(e.Lit)
	case *ast.Attribute:
		if c, ok := e.Value.(*ast.Constant); ok && c.Kind == "int" {
			// "1.real" reads as a float followed by a name.
			b.WriteString("(" + c.Lit + ")")
		} else {
			writeExpr(b, e.Value, precAtom)
		}
		b.WriteString("." + e.Attr)
	case *ast.Subscript:
		writeExpr(b, e.Value, precAtom)
		b.WriteByte('[')
		// A tuple of subscripts needs no parentheses: Dict[str, int].
		if t, ok := e.Slice.(*ast.Tuple); ok && len(t.Elts) > 0 {
			writeTupleItems(b, t.Elts)
		} else {
			writeExpr(b, e.Slice, precTest)
		}
		b.WriteByte(']')
	case *ast.Slice:
		writeExpr(b, e.Lower, precTest)
		b.WriteByte(':')
		writeExpr(b, e.Upper, precTest)
		if e.Step != nil {
			b.WriteByte(':')
			writeExpr(b, e.Step, precTest)
		}
	case *ast.Lambda:
		b.WriteString("lambda")
		if e.Args != "" {
			b.WriteString(" " + e.Args)
		}
		b.WriteString(": ")
		writeExpr(b, e.Body, precTest)
	case *ast.BinOp:
		writeBinOp(b, e)
	case *ast.UnaryOp:
		b.WriteString(operatorString(e.Op))
		writeExpr(b, e.Operand, exprPrecedence(e))
	case *ast.Tuple:
		if len(e.Elts) == 0 {
			b.WriteString("()")
			return
		}
		// writeExpr adds parentheses everywhere but a tuple-level position.
		writeTupleItems(b, e.Elts)
	case *ast.List:
		b.WriteByte('[')
		writeItems(b, e.Elts)
		b.WriteByte(']')
	case *ast.Set:
		b.WriteByte('{')
		writeItems(b, e.Elts)
		b.WriteByte('}')
	case *ast.Dict:
		b.WriteByte('{')
		for i := range e.Keys {
			if i > 0 {
				b.WriteString(", ")
			}
			writeDictEntry(b, e.Keys[i], e.Values[i])
		}
		b.WriteByte('}')
	case *ast.ListComp:
		b.WriteByte('[')
		writeExpr(b, e.Elt, precTest)
		writeComprehensions(b, e.Generators)
		b.WriteByte(']')
	case *ast.SetComp:
		b.WriteByte('{')
		writeExpr(b, e.Elt, precTest)
		writeComprehensions(b, e.Generators)
		b.WriteByte('}')
	case *ast.DictComp:
		b.WriteByte('{')
		writeDictEntry(b, e.Key, e.Value)
		writeComprehensions(b, e.Generators)
		b.WriteByte('}')
	case *ast.GeneratorExp:
		b.WriteByte('(')
		writeGenerator(b, e)
		b.WriteByte(')')
	case *ast.Call:
		writeCall(b, e)
	case *ast.Starred:
		b.WriteByte('*')
		writeExpr(b, e.Value, precBitOr)
	case *ast.IfExpr:
		writeExpr(b, e.Body, precOr)
		b.WriteString(" if ")
		writeExpr(b, e.Test, precOr)
		b.WriteString(" else ")
		writeExpr(b, e.Orelse, precTest)
	case *ast.Await:
		b.WriteString("await ")
		writeExpr(b, e.Value, precAtom)
	case *ast.Yield:
		b.WriteString("yield")
		if e.From {
			b.WriteString(" from")
		}
		if e.Value != nil {
			b.WriteByte(' ')
			writeExpr(b, e.Value, precTuple)
		}
	case *ast.NamedExpr:
		writeExpr(b, e.Target, precAtom)
		b.WriteString(" := ")
		writeExpr(b, e.Value, precTest)
	default:
		b.WriteByte('?')
	}
}

// writeBinOp writes a binary operator, a comparison or a boolean operator. Every
// one but ** groups to the left, so its right operand needs a tighter binding.
func writeBinOp(b *strings.Builder, e *ast.BinOp) {
	prec := binOpPrecedence(e.Op)
	left, right := prec, prec+1
	switch prec {
	case precOr, precAnd, precCmp:
		// These form runs rather than nest, so a left operand of the same kind
		// continues the run only if the parser says so.
		if !e.Chained {
			left = prec + 1
		}
	case precPower:
		// The right side of ** may be a unary operator: 10 ** -6.
		left, right = prec+1, precFactor
	}
	writeExpr(b, e.Left, left)
	b.WriteByte(' ')
	b.WriteString(binOpString(e))
	b.WriteByte(' ')
	writeExpr(b, e.Right, right)
}

func binOpString(e *ast.BinOp) string {
	if e.Negated {
		if e.Op == token.IN {
			return "not in"
		}
		return "is not"
	}
	return operatorString(e.Op)
}

// writeCall writes a call. A generator expression as the only argument shares the
// parentheses of the call.
func writeCall(b *strings.Builder, e *ast.Call) {
	writeExpr(b, e.Func, precAtom)
	b.WriteByte('(')
	if g, ok := soleGenerator(e); ok {
		writeGenerator(b, g)
		b.WriteByte(')')
		return
	}
	writeItems(b, e.Args)
	for i, kw := range e.Keywords {
		if i > 0 || len(e.Args) > 0 {
			b.WriteString(", ")
		}
		if kw.Arg != "" {
			b.WriteString(kw.Arg + "=")
		} else {
			b.WriteString("**")
		}
		writeExpr(b, kw.Value, precTest)
	}
	b.WriteByte(')')
}

func soleGenerator(e *ast.Call) (*ast.GeneratorExp, bool) {
	if len(e.Args) != 1 || len(e.Keywords) != 0 {
		return nil, false
	}
	g, ok := e.Args[0].(*ast.GeneratorExp)
	return g, ok
}

func writeGenerator(b *strings.Builder, g *ast.GeneratorExp) {
	writeExpr(b, g.Elt, precTest)
	writeComprehensions(b, g.Generators)
}

// writeComprehensions writes the clauses that follow the element of a comprehension.
func writeComprehensions(b *strings.Builder, gens []*ast.Comprehension) {
	for _, c := range gens {
		if c.IsAsync {
			b.WriteString(" async")
		}
		b.WriteString(" for ")
		writeExpr(b, c.Target, precTuple)
		b.WriteString(" in ")
		writeExpr(b, c.Iter, precOr)
		for _, cond := range c.Ifs {
			b.WriteString(" if ")
			writeExpr(b, cond, precOr)
		}
	}
}

// writeDictEntry writes "key: value", or "**value" for a nil key.
func writeDictEntry(b *strings.Builder, key, value ast.Expr) {
	if key == nil {
		b.WriteString("**")
		writeExpr(b, value, precBitOr)
		return
	}
	writeExpr(b, key, precTest)
	b.WriteString(": ")
	writeExpr(b, value, precTest)
}

// writeItems writes a comma-separated list of expressions.
func writeItems(b *strings.Builder, items []ast.Expr) {
	for i, item := range items {
		if i > 0 {
			b.WriteString(", ")
		}
		writeExpr(b, item, precTest)
	}
}

// writeTupleItems writes the items of a tuple. A single item keeps its comma, which
// is what makes it a tuple.
func writeTupleItems(b *strings.Builder, items []ast.Expr) {
	writeItems(b, items)
	if len(items) == 1 {
		b.WriteByte(',')
	}
}

// operatorString returns the string representation of a token operator.
func operatorString(op token.Type) string {
	switch op {
	case token.PIPE:
		return "|"
	case token.PLUS:
		return "+"
	case token.MINUS:
		return "-"
	case token.STAR:
		return "*"
	case token.DSTAR:
		return "**"
	case token.SLASH:
		return "/"
	case token.DSLASH:
		return "//"
	case token.AT:
		return "@"
	case token.IN:
		return "in"
	case token.IS:
		return "is"
	case token.PERCENT:
		return "%"
	case token.AMPER:
		return "&"
	case token.CARET:
		return "^"
	case token.TILDE:
		return "~"
	case token.LSHIFT:
		return "<<"
	case token.RSHIFT:
		return ">>"
	case token.AND:
		return "and"
	case token.OR:
		return "or"
	case token.NOT:
		return "not "
	case token.EQ:
		return "=="
	case token.NEQ:
		return "!="
	case token.LT:
		return "<"
	case token.GT:
		return ">"
	case token.LTE:
		return "<="
	case token.GTE:
		return ">="
	default:
		return "?"
	}
}
