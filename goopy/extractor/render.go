package extractor

import (
	"strings"

	"github.com/pypx/goopy/ast"
	"github.com/pypx/goopy/token"
)

// exprToString renders an AST expression back to its source text representation.
func exprToString(expr ast.Expr) string {
	if expr == nil {
		return ""
	}

	switch e := expr.(type) {
	case *ast.Name:
		return e.Name
	case *ast.Constant:
		return e.Lit
	case *ast.Attribute:
		return exprToString(e.Value) + "." + e.Attr
	case *ast.Subscript:
		return exprToString(e.Value) + "[" + exprToString(e.Slice) + "]"
	case *ast.BinOp:
		op := operatorString(e.Op)
		return exprToString(e.Left) + " " + op + " " + exprToString(e.Right)
	case *ast.UnaryOp:
		return operatorString(e.Op) + exprToString(e.Operand)
	case *ast.Tuple:
		parts := make([]string, len(e.Elts))
		for i, elt := range e.Elts {
			parts[i] = exprToString(elt)
		}
		return strings.Join(parts, ", ")
	case *ast.List:
		parts := make([]string, len(e.Elts))
		for i, elt := range e.Elts {
			parts[i] = exprToString(elt)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case *ast.Call:
		s := exprToString(e.Func) + "("
		var args []string
		for _, arg := range e.Args {
			args = append(args, exprToString(arg))
		}
		for _, kw := range e.Keywords {
			if kw.Arg != "" {
				args = append(args, kw.Arg+"="+exprToString(kw.Value))
			} else {
				args = append(args, "**"+exprToString(kw.Value))
			}
		}
		return s + strings.Join(args, ", ") + ")"
	case *ast.Starred:
		return "*" + exprToString(e.Value)
	case *ast.IfExpr:
		return exprToString(e.Body) + " if " + exprToString(e.Test) + " else " + exprToString(e.Orelse)
	case *ast.Dict:
		var parts []string
		for i := range e.Keys {
			parts = append(parts, exprToString(e.Keys[i])+": "+exprToString(e.Values[i]))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case *ast.Set:
		parts := make([]string, len(e.Elts))
		for i, elt := range e.Elts {
			parts[i] = exprToString(elt)
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		return "?"
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
