package extractor

import (
	"github.com/pypx/goopy/ast"
	"github.com/pypx/goopy/model"
	"github.com/pypx/goopy/token"
)

// exprToTypeExpr converts an AST expression to a model TypeExpr.
func exprToTypeExpr(expr ast.Expr) *model.TypeExpr {
	if expr == nil {
		return nil
	}

	switch e := expr.(type) {
	case *ast.Name:
		if e.Name == "None" {
			return &model.TypeExpr{Kind: model.TypeExprNone, Raw: "None"}
		}
		return &model.TypeExpr{Kind: model.TypeExprName, Name: e.Name, Raw: e.Name}

	case *ast.Constant:
		switch e.Kind {
		case "none":
			return &model.TypeExpr{Kind: model.TypeExprNone, Raw: "None"}
		case "ellipsis":
			return &model.TypeExpr{Kind: model.TypeExprEllipsis, Raw: "..."}
		default:
			return &model.TypeExpr{Kind: model.TypeExprLiteral, Value: e.Value, Raw: e.Lit}
		}

	case *ast.Attribute:
		// e.g., typing.Optional -> treat as the attr name for type resolution.
		raw := exprToString(expr)
		attrName := e.Attr
		// For typing.X, treat as just X for type analysis purposes.
		if n, ok := e.Value.(*ast.Name); ok && isTypingModule(n.Name) {
			return exprToTypeExpr(&ast.Name{Name: attrName})
		}
		return &model.TypeExpr{Kind: model.TypeExprName, Name: raw, Raw: raw}

	case *ast.Subscript:
		typeName := resolveTypeName(e.Value)
		raw := exprToString(expr)

		switch typeName {
		case "Optional":
			inner := exprToTypeExpr(e.Slice)
			return &model.TypeExpr{
				Kind:     model.TypeExprOptional,
				Elements: []*model.TypeExpr{inner},
				Raw:      raw,
			}

		case "Union":
			elements := sliceToTypeExprs(e.Slice)
			return &model.TypeExpr{
				Kind:     model.TypeExprUnion,
				Elements: elements,
				Raw:      raw,
			}

		case "Callable":
			// Callable[[arg_types], return_type]
			tuple, ok := e.Slice.(*ast.Tuple)
			if ok && len(tuple.Elts) == 2 {
				var argTypes []*model.TypeExpr
				if argList, ok := tuple.Elts[0].(*ast.List); ok {
					for _, arg := range argList.Elts {
						argTypes = append(argTypes, exprToTypeExpr(arg))
					}
				}
				retType := exprToTypeExpr(tuple.Elts[1])
				return &model.TypeExpr{
					Kind:    model.TypeExprCallable,
					Args:    argTypes,
					Returns: retType,
					Raw:     raw,
				}
			}
			// Fallback for malformed Callable.
			return &model.TypeExpr{Kind: model.TypeExprName, Name: raw, Raw: raw}

		case "Tuple":
			elements := sliceToTypeExprs(e.Slice)
			return &model.TypeExpr{
				Kind:     model.TypeExprTuple,
				Name:     "Tuple",
				Elements: elements,
				Raw:      raw,
			}

		case "Literal":
			elements := sliceToTypeExprs(e.Slice)
			return &model.TypeExpr{
				Kind:     model.TypeExprLiteral,
				Elements: elements,
				Raw:      raw,
			}

		default:
			// Generic type: list[int], Dict[str, int], etc.
			args := sliceToTypeExprs(e.Slice)
			return &model.TypeExpr{
				Kind: model.TypeExprGeneric,
				Name: typeName,
				Args: args,
				Raw:  raw,
			}
		}

	case *ast.BinOp:
		if e.Op == token.PIPE {
			elements := flattenUnion(expr)
			raw := exprToString(expr)
			return &model.TypeExpr{
				Kind:     model.TypeExprUnion,
				Elements: elements,
				Raw:      raw,
			}
		}
		// Other binary ops — just use raw string.
		raw := exprToString(expr)
		return &model.TypeExpr{Kind: model.TypeExprName, Name: raw, Raw: raw}

	case *ast.Tuple:
		elements := make([]*model.TypeExpr, len(e.Elts))
		for i, elt := range e.Elts {
			elements[i] = exprToTypeExpr(elt)
		}
		raw := exprToString(expr)
		return &model.TypeExpr{
			Kind:     model.TypeExprTuple,
			Elements: elements,
			Raw:      raw,
		}

	case *ast.Starred:
		inner := exprToTypeExpr(e.Value)
		raw := "*" + inner.Raw
		return &model.TypeExpr{
			Kind: model.TypeExprUnpack,
			Args: []*model.TypeExpr{inner},
			Raw:  raw,
		}

	case *ast.List:
		// A list in a type context (e.g., Callable[[int, str], bool] inner part).
		elements := make([]*model.TypeExpr, len(e.Elts))
		for i, elt := range e.Elts {
			elements[i] = exprToTypeExpr(elt)
		}
		raw := exprToString(expr)
		return &model.TypeExpr{
			Kind:     model.TypeExprTuple,
			Elements: elements,
			Raw:      raw,
		}

	default:
		raw := exprToString(expr)
		return &model.TypeExpr{Kind: model.TypeExprName, Name: raw, Raw: raw}
	}
}

// flattenUnion collects all elements of a chain of | operators.
func flattenUnion(expr ast.Expr) []*model.TypeExpr {
	binOp, ok := expr.(*ast.BinOp)
	if !ok || binOp.Op != token.PIPE {
		return []*model.TypeExpr{exprToTypeExpr(expr)}
	}
	var result []*model.TypeExpr
	result = append(result, flattenUnion(binOp.Left)...)
	result = append(result, flattenUnion(binOp.Right)...)
	return result
}

// sliceToTypeExprs converts a subscript slice expression to a slice of TypeExprs.
// Handles both single elements and tuples (comma-separated).
func sliceToTypeExprs(slice ast.Expr) []*model.TypeExpr {
	if tuple, ok := slice.(*ast.Tuple); ok {
		result := make([]*model.TypeExpr, len(tuple.Elts))
		for i, elt := range tuple.Elts {
			result[i] = exprToTypeExpr(elt)
		}
		return result
	}
	return []*model.TypeExpr{exprToTypeExpr(slice)}
}

// resolveTypeName extracts the simple type name from an expression, resolving
// typing module prefixes (e.g., typing.Optional -> "Optional").
func resolveTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Name:
		return e.Name
	case *ast.Attribute:
		if n, ok := e.Value.(*ast.Name); ok && isTypingModule(n.Name) {
			return e.Attr
		}
		return exprToString(expr)
	default:
		return exprToString(expr)
	}
}

// isTypingModule returns true if name is a known typing module name.
func isTypingModule(name string) bool {
	return name == "typing" || name == "typing_extensions" || name == "collections.abc"
}
