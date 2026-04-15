// Package extractor walks a Python AST and produces structured documentation
// model types. It converts parsed syntax trees into [model.Module] values
// suitable for JSON serialization or further processing.
package extractor

import (
	"strings"

	"github.com/pypx/goopy/ast"
	"github.com/pypx/goopy/docstring"
	"github.com/pypx/goopy/model"
	"github.com/pypx/goopy/token"
)

// Extractor walks an AST and produces documentation model types.
type Extractor struct{}

// New creates a new Extractor.
func New() *Extractor { return &Extractor{} }

// ExtractModule extracts structured documentation from a parsed AST module.
func (e *Extractor) ExtractModule(name string, mod *ast.Module) *model.Module {
	m := &model.Module{Name: name}

	exports := extractAllExports(mod.Body)

	// Check for module-level docstring (first statement).
	if len(mod.Body) > 0 {
		if ds := extractDocstring(mod.Body[0]); ds != "" {
			m.Docstring = docstring.Parse(ds)
		}
	}

	e.extractStmts(mod.Body, m, exports)

	return m
}

// extractStmts walks a slice of statements, populating m with any definitions
// found. It recurses into PassThrough and If bodies so that nested definitions
// (e.g. inside try/except or if TYPE_CHECKING blocks) are not silently dropped.
func (e *Extractor) extractStmts(stmts []ast.Stmt, m *model.Module, exports []string) {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.FunctionDef:
			if !isPublic(s.Name, exports) {
				continue
			}
			fn := e.extractFunction(s, false)
			m.Functions = append(m.Functions, fn)

		case *ast.ClassDef:
			if !isPublic(s.Name, exports) {
				continue
			}
			cls := e.extractClass(s, exports)
			m.Classes = append(m.Classes, cls)

		case *ast.AnnAssign:
			attr := e.extractAttribute(s)
			if attr != nil && isPublic(attr.Name, exports) {
				m.Attributes = append(m.Attributes, attr)
			}

		case *ast.Assign:
			// Extract __all__ is handled separately; skip it here.
			// Also extract simple module-level assignments as attributes.
			for _, target := range s.Targets {
				if n, ok := target.(*ast.Name); ok {
					if n.Name == "__all__" {
						continue
					}
					if !isPublic(n.Name, exports) {
						continue
					}
					attr := &model.Attribute{
						Name:  n.Name,
						Value: exprToString(s.Value),
					}
					m.Attributes = append(m.Attributes, attr)
				}
			}

		case *ast.TypeAliasDef:
			if !isPublic(s.Name, exports) {
				continue
			}
			ta := &model.TypeAlias{
				Name:  s.Name,
				Value: exprToTypeExpr(s.Value),
			}
			m.TypeAliases = append(m.TypeAliases, ta)

		case *ast.Import:
			for _, alias := range s.Names {
				n := alias.Name
				if alias.Alias != "" {
					n = alias.Alias
				}
				m.Imports = append(m.Imports, &model.TypeRef{Name: n})
			}

		case *ast.ImportFrom:
			for _, alias := range s.Names {
				n := alias.Name
				if alias.Alias != "" {
					n = alias.Alias
				}
				m.Imports = append(m.Imports, &model.TypeRef{Name: n})
			}

		case *ast.PassThrough:
			// Recurse into try/except, for, while, with, match, etc.
			e.extractStmts(s.Body, m, exports)

		case *ast.If:
			// Recurse into both branches (handles if TYPE_CHECKING, etc.).
			e.extractStmts(s.Body, m, exports)
			e.extractStmts(s.Orelse, m, exports)
		}
	}
}

// extractFunction converts an AST FunctionDef into a model Function.
func (e *Extractor) extractFunction(fd *ast.FunctionDef, isMethod bool) *model.Function {
	fn := &model.Function{
		Name:     fd.Name,
		IsAsync:  fd.IsAsync,
		IsMethod: isMethod,
	}

	// Decorators.
	for _, dec := range fd.Decorators {
		fn.Decorators = append(fn.Decorators, exprToString(dec))
	}

	// Parameters.
	if fd.Args != nil {
		fn.Parameters = extractParameters(fd.Args)
		// If this is a method, strip 'self' or 'cls' from the first parameter.
		if isMethod && len(fn.Parameters) > 0 {
			first := fn.Parameters[0].Name
			if first == "self" || first == "cls" {
				fn.Parameters = fn.Parameters[1:]
			}
		}
	}

	// Return type.
	if fd.Returns != nil {
		fn.Returns = exprToTypeExpr(fd.Returns)
	}

	// Docstring from first statement in body.
	if len(fd.Body) > 0 {
		if ds := extractDocstring(fd.Body[0]); ds != "" {
			parsed := docstring.Parse(ds)
			fn.Docstring = parsed
			// Attach doc params to parameters.
			attachDocParams(fn.Parameters, parsed.Params)
		}
	}

	return fn
}

// extractClass converts an AST ClassDef into a model Class.
func (e *Extractor) extractClass(cd *ast.ClassDef, moduleExports []string) *model.Class {
	cls := &model.Class{
		Name: cd.Name,
	}

	// Decorators.
	for _, dec := range cd.Decorators {
		cls.Decorators = append(cls.Decorators, exprToString(dec))
	}

	// Base classes.
	for _, base := range cd.Bases {
		cls.BaseClasses = append(cls.BaseClasses, &model.TypeRef{
			Name: exprToString(base),
			Type: exprToTypeExpr(base),
		})
	}

	// Docstring from first statement in body.
	if len(cd.Body) > 0 {
		if ds := extractDocstring(cd.Body[0]); ds != "" {
			cls.Docstring = docstring.Parse(ds)
		}
	}

	// Walk class body for methods and attributes.
	for _, stmt := range cd.Body {
		switch s := stmt.(type) {
		case *ast.FunctionDef:
			// Check for @property decorator.
			if hasDecorator(s.Decorators, "property") {
				attr := e.extractPropertyAttribute(s)
				cls.Attributes = append(cls.Attributes, attr)
				continue
			}

			// Skip __init__ and private methods from the methods list.
			if s.Name == "__init__" {
				// But extract attributes from __init__ body.
				cls.Attributes = append(cls.Attributes, e.extractInitAttributes(s)...)
				continue
			}
			if strings.HasPrefix(s.Name, "_") && !strings.HasPrefix(s.Name, "__") {
				continue
			}
			// Also skip private dunder methods that aren't public API,
			// but keep standard dunders like __len__, __repr__, etc.

			fn := e.extractFunction(s, true)
			cls.Methods = append(cls.Methods, fn)

		case *ast.AnnAssign:
			attr := e.extractAttribute(s)
			if attr != nil {
				cls.Attributes = append(cls.Attributes, attr)
			}

		case *ast.Assign:
			for _, target := range s.Targets {
				if n, ok := target.(*ast.Name); ok {
					if strings.HasPrefix(n.Name, "_") && !strings.HasPrefix(n.Name, "__") {
						continue
					}
					attr := &model.Attribute{
						Name:  n.Name,
						Value: exprToString(s.Value),
					}
					cls.Attributes = append(cls.Attributes, attr)
				}
			}
		}
	}

	return cls
}

// extractPropertyAttribute converts a @property-decorated method into an Attribute.
func (e *Extractor) extractPropertyAttribute(fd *ast.FunctionDef) *model.Attribute {
	attr := &model.Attribute{
		Name:       fd.Name,
		IsProperty: true,
	}
	if fd.Returns != nil {
		attr.Type = exprToTypeExpr(fd.Returns)
	}
	if len(fd.Body) > 0 {
		if ds := extractDocstring(fd.Body[0]); ds != "" {
			attr.Docstring = docstring.Parse(ds)
		}
	}
	return attr
}

// extractInitAttributes extracts self.x = ... assignments from __init__ body.
func (e *Extractor) extractInitAttributes(fd *ast.FunctionDef) []*model.Attribute {
	var attrs []*model.Attribute
	for _, stmt := range fd.Body {
		switch s := stmt.(type) {
		case *ast.Assign:
			for _, target := range s.Targets {
				if a, ok := target.(*ast.Attribute); ok {
					if selfName, ok := a.Value.(*ast.Name); ok && selfName.Name == "self" {
						if strings.HasPrefix(a.Attr, "_") && !strings.HasPrefix(a.Attr, "__") {
							continue
						}
						attrs = append(attrs, &model.Attribute{
							Name:  a.Attr,
							Value: exprToString(s.Value),
						})
					}
				}
			}
		case *ast.AnnAssign:
			if a, ok := s.Target.(*ast.Attribute); ok {
				if selfName, ok := a.Value.(*ast.Name); ok && selfName.Name == "self" {
					if strings.HasPrefix(a.Attr, "_") && !strings.HasPrefix(a.Attr, "__") {
						continue
					}
					attr := &model.Attribute{
						Name: a.Attr,
						Type: exprToTypeExpr(s.Annotation),
					}
					if s.Value != nil {
						attr.Value = exprToString(s.Value)
					}
					attrs = append(attrs, attr)
				}
			}
		}
	}
	return attrs
}

// extractAttribute converts an AnnAssign statement into an Attribute.
func (e *Extractor) extractAttribute(s *ast.AnnAssign) *model.Attribute {
	n, ok := s.Target.(*ast.Name)
	if !ok {
		return nil
	}
	attr := &model.Attribute{
		Name: n.Name,
		Type: exprToTypeExpr(s.Annotation),
	}
	if s.Value != nil {
		attr.Value = exprToString(s.Value)
	}
	return attr
}

// extractParameters converts AST Arguments into model Parameters.
func extractParameters(args *ast.Arguments) []*model.Parameter {
	var params []*model.Parameter

	// Positional-only args.
	defaultOffset := len(args.PosOnlyArgs) + len(args.Args) - len(args.Defaults)
	idx := 0

	for _, arg := range args.PosOnlyArgs {
		p := &model.Parameter{
			Name: arg.Name,
			Kind: model.ParamPositionalOnly,
		}
		if arg.Annotation != nil {
			p.Type = exprToTypeExpr(arg.Annotation)
		}
		if di := idx - defaultOffset; di >= 0 && di < len(args.Defaults) {
			p.Default = exprToString(args.Defaults[di])
		}
		params = append(params, p)
		idx++
	}

	// Regular args.
	for _, arg := range args.Args {
		p := &model.Parameter{
			Name: arg.Name,
			Kind: model.ParamPositionalOrKeyword,
		}
		if arg.Annotation != nil {
			p.Type = exprToTypeExpr(arg.Annotation)
		}
		if di := idx - defaultOffset; di >= 0 && di < len(args.Defaults) {
			p.Default = exprToString(args.Defaults[di])
		}
		params = append(params, p)
		idx++
	}

	// *args.
	if args.VarArg != nil {
		p := &model.Parameter{
			Name: args.VarArg.Name,
			Kind: model.ParamVarPositional,
		}
		if args.VarArg.Annotation != nil {
			p.Type = exprToTypeExpr(args.VarArg.Annotation)
		}
		params = append(params, p)
	}

	// Keyword-only args.
	for i, arg := range args.KwOnlyArgs {
		p := &model.Parameter{
			Name: arg.Name,
			Kind: model.ParamKeywordOnly,
		}
		if arg.Annotation != nil {
			p.Type = exprToTypeExpr(arg.Annotation)
		}
		if i < len(args.KwDefaults) && args.KwDefaults[i] != nil {
			p.Default = exprToString(args.KwDefaults[i])
		}
		params = append(params, p)
	}

	// **kwargs.
	if args.VarKwArg != nil {
		p := &model.Parameter{
			Name: args.VarKwArg.Name,
			Kind: model.ParamVarKeyword,
		}
		if args.VarKwArg.Annotation != nil {
			p.Type = exprToTypeExpr(args.VarKwArg.Annotation)
		}
		params = append(params, p)
	}

	return params
}

// attachDocParams matches parsed docstring params to function parameters.
func attachDocParams(params []*model.Parameter, docParams []*model.DocParam) {
	if len(docParams) == 0 {
		return
	}
	byName := make(map[string]*model.DocParam, len(docParams))
	for _, dp := range docParams {
		byName[dp.Name] = dp
	}
	for _, p := range params {
		if dp, ok := byName[p.Name]; ok {
			p.DocParam = dp
		}
	}
}

// extractDocstring checks if stmt is an expression statement containing a string
// constant (i.e. a docstring). Returns the unquoted string content.
func extractDocstring(stmt ast.Stmt) string {
	es, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return ""
	}
	c, ok := es.Value.(*ast.Constant)
	if !ok || c.Kind != "str" {
		return ""
	}
	return c.Value
}

// extractAllExports finds __all__ = [...] in module body and returns the list of names.
// Returns nil if __all__ is not defined.
func extractAllExports(body []ast.Stmt) []string {
	for _, stmt := range body {
		assign, ok := stmt.(*ast.Assign)
		if !ok {
			continue
		}
		for _, target := range assign.Targets {
			n, ok := target.(*ast.Name)
			if !ok || n.Name != "__all__" {
				continue
			}
			var elts []ast.Expr
			switch v := assign.Value.(type) {
			case *ast.List:
				elts = v.Elts
			case *ast.Tuple:
				elts = v.Elts
			default:
				continue
			}
			var exports []string
			for _, elt := range elts {
				c, ok := elt.(*ast.Constant)
				if ok && c.Kind == "str" {
					exports = append(exports, c.Value)
				}
			}
			return exports
		}
	}
	return nil
}

// isPublic determines if a name should be included in the output.
// If exports is non-nil (__all__ exists), the name must be in exports.
// Otherwise, names starting with _ are considered private.
func isPublic(name string, exports []string) bool {
	if exports != nil {
		for _, e := range exports {
			if e == name {
				return true
			}
		}
		return false
	}
	return !strings.HasPrefix(name, "_")
}

// hasDecorator checks if a function has a specific decorator name.
func hasDecorator(decorators []ast.Expr, name string) bool {
	for _, dec := range decorators {
		switch d := dec.(type) {
		case *ast.Name:
			if d.Name == name {
				return true
			}
		case *ast.Attribute:
			if d.Attr == name {
				return true
			}
		case *ast.Call:
			// e.g., @property or @functools.cached_property
			if n, ok := d.Func.(*ast.Name); ok && n.Name == name {
				return true
			}
			if a, ok := d.Func.(*ast.Attribute); ok && a.Attr == name {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// AST expression to TypeExpr conversion
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// AST expression to string conversion
// ---------------------------------------------------------------------------

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
