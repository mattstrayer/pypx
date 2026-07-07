// Package extractor walks a Python AST and produces structured documentation
// model types. It converts parsed syntax trees into [model.Module] values
// suitable for JSON serialization or further processing.
package extractor

import (
	"strings"

	"github.com/pypx/goopy/ast"
	"github.com/pypx/goopy/docstring"
	"github.com/pypx/goopy/model"
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
func (e *Extractor) extractStmts(stmts []ast.Stmt, m *model.Module, exports map[string]struct{}) {
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
func (e *Extractor) extractClass(cd *ast.ClassDef, moduleExports map[string]struct{}) *model.Class {
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
			if isPrivateName(s.Name) {
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
					if isPrivateName(n.Name) {
						continue
					}
					attr := &model.Attribute{
						Name:  n.Name,
						Value: exprToString(s.Value),
					}
					cls.Attributes = append(cls.Attributes, attr)
				}
			}

		case *ast.ClassDef:
			// Nested class (e.g. Django `class Meta`, pydantic v1 `class Config`).
			// Same privacy rule as methods: skip _Name but keep dunder-style names.
			if isPrivateName(s.Name) {
				continue
			}
			cls.Classes = append(cls.Classes, e.extractClass(s, moduleExports))
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
						if isPrivateName(a.Attr) {
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
					if isPrivateName(a.Attr) {
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

// extractAllExports finds __all__ = [...] in module body and returns a set of names.
// Returns nil if __all__ is not defined.
func extractAllExports(body []ast.Stmt) map[string]struct{} {
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
			exports := make(map[string]struct{}, len(elts))
			for _, elt := range elts {
				c, ok := elt.(*ast.Constant)
				if ok && c.Kind == "str" {
					exports[c.Value] = struct{}{}
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
func isPublic(name string, exports map[string]struct{}) bool {
	if exports != nil {
		_, ok := exports[name]
		return ok
	}
	return !isPrivateName(name)
}

// isDunder reports whether name is a Python dunder (e.g. __init__, __len__):
// leading AND trailing double underscore.
func isDunder(name string) bool {
	return strings.HasPrefix(name, "__") && strings.HasSuffix(name, "__")
}

// isPrivateName reports whether a bare identifier is private by Python
// convention: any leading underscore that is not a dunder. This covers both
// single-underscore (_x) and name-mangled (__x) members.
func isPrivateName(name string) bool {
	return strings.HasPrefix(name, "_") && !isDunder(name)
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

