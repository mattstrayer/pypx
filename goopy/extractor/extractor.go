// Package extractor walks a Python AST and produces structured documentation
// model types. It converts parsed syntax trees into [model.Module] values
// suitable for JSON serialization or further processing.
package extractor

import (
	"slices"
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

	m.Exports = extractAllExports(mod.Body)
	// A partial __all__ cannot filter here. The package resolver filters once it
	// knows the referenced lists.
	var exports map[string]struct{}
	if m.Exports != nil && len(m.Exports.Refs) == 0 && !m.Exports.Dynamic {
		exports = make(map[string]struct{}, len(m.Exports.Names))
		for _, n := range m.Exports.Names {
			exports[n] = struct{}{}
		}
	}

	// Check for module-level docstring (first statement).
	if len(mod.Body) > 0 {
		if ds := extractDocstring(mod.Body[0]); ds != "" {
			m.Docstring = docstring.Parse(ds)
		}
	}

	e.extractStmts(mod.Body, m, exports)

	// The resolver indexes these lists, so one name must be one definition here.
	collapseDefinitions(&m.Functions, &m.Classes, &m.Attributes, &m.TypeAliases)
	collapseDefinitions(&m.Hidden.Functions, &m.Hidden.Classes, &m.Hidden.Attributes, &m.Hidden.TypeAliases)

	return m
}

// collapseDefinitions leaves one definition per name in each list.
func collapseDefinitions(fns *[]*model.Function, classes *[]*model.Class, attrs *[]*model.Attribute, aliases *[]*model.TypeAlias) {
	*fns = collapseFunctions(*fns)
	*classes = collapseLast(*classes, func(c *model.Class) string { return c.Name })
	*attrs = collapseAttributes(*attrs)
	*aliases = collapseLast(*aliases, func(t *model.TypeAlias) string { return t.Name })
}

// extractStmts walks a slice of statements, populating m with any definitions
// found. It recurses into PassThrough and If bodies so that nested definitions
// (e.g. inside try/except or if TYPE_CHECKING blocks) are not silently dropped.
func (e *Extractor) extractStmts(stmts []ast.Stmt, m *model.Module, exports map[string]struct{}) {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.FunctionDef:
			fn := e.extractFunction(s, false)
			if isPublic(s.Name, exports) {
				m.Functions = append(m.Functions, fn)
			} else {
				m.Hidden.Functions = append(m.Hidden.Functions, fn)
			}

		case *ast.ClassDef:
			cls := e.extractClass(s, exports)
			if isPublic(s.Name, exports) {
				m.Classes = append(m.Classes, cls)
			} else {
				m.Hidden.Classes = append(m.Hidden.Classes, cls)
			}

		case *ast.AnnAssign:
			attr := e.extractAttribute(s)
			if attr == nil || attr.Name == "__all__" {
				continue
			}
			if isPublic(attr.Name, exports) {
				m.Attributes = append(m.Attributes, attr)
			} else {
				m.Hidden.Attributes = append(m.Hidden.Attributes, attr)
			}

		case *ast.Assign:
			// Extract __all__ is handled separately; skip it here.
			// Also extract simple module-level assignments as attributes.
			addAttr := func(name, value string) {
				if name == "__all__" {
					return
				}
				attr := &model.Attribute{Name: name, Value: value}
				if isPublic(name, exports) {
					m.Attributes = append(m.Attributes, attr)
				} else {
					m.Hidden.Attributes = append(m.Hidden.Attributes, attr)
				}
			}
			for _, target := range s.Targets {
				switch t := target.(type) {
				case *ast.Name:
					addAttr(t.Name, exprToString(s.Value))
				case *ast.Tuple:
					// "a, b = 1, 2" pairs values with names. For any other value
					// the parts are unknown, so the attributes have no value.
					values, _ := s.Value.(*ast.Tuple)
					for i, elt := range t.Elts {
						n, ok := elt.(*ast.Name)
						if !ok {
							continue
						}
						value := ""
						if values != nil && len(values.Elts) == len(t.Elts) {
							value = exprToString(values.Elts[i])
						}
						addAttr(n.Name, value)
					}
				}
			}

		case *ast.TypeAliasDef:
			ta := &model.TypeAlias{
				Name:  s.Name,
				Value: exprToTypeExpr(s.Value),
			}
			if isPublic(s.Name, exports) {
				m.TypeAliases = append(m.TypeAliases, ta)
			} else {
				m.Hidden.TypeAliases = append(m.Hidden.TypeAliases, ta)
			}

		case *ast.Import:
			for _, alias := range s.Names {
				n := alias.Name
				if alias.Alias != "" {
					n = alias.Alias
				}
				m.Imports = append(m.Imports, &model.TypeRef{Name: n})
				m.ImportFroms = append(m.ImportFroms, &model.Import{Module: alias.Name, Alias: alias.Alias})
			}

		case *ast.ImportFrom:
			for _, alias := range s.Names {
				n := alias.Name
				if alias.Alias != "" {
					n = alias.Alias
				}
				m.Imports = append(m.Imports, &model.TypeRef{Name: n})
				m.ImportFroms = append(m.ImportFroms, &model.Import{
					Module: s.Module,
					Level:  s.Level,
					Name:   alias.Name,
					Alias:  alias.Alias,
				})
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
		// A method's first parameter is the bound object, whatever its name. In
		// def f(*args) or def f(*, a) no parameter holds only that object.
		if isMethod && !hasDecorator(fd.Decorators, "staticmethod") && len(fn.Parameters) > 0 {
			if k := fn.Parameters[0].Kind; k == model.ParamPositionalOnly || k == model.ParamPositionalOrKeyword {
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
	e.extractClassStmts(cd.Body, cls, moduleExports)

	// One name is one member. __init__ goes through the same collapse, so the
	// constructor is the implementation with its overloads attached.
	cls.Methods = collapseFunctions(cls.Methods)
	cls.Attributes = collapseAttributes(cls.Attributes)
	cls.Classes = collapseLast(cls.Classes, func(c *model.Class) string { return c.Name })
	for i, m := range cls.Methods {
		if m.Name == "__init__" {
			cls.Constructor = m
			cls.Methods = slices.Delete(cls.Methods, i, i+1)
			break
		}
	}

	cls.Dataclass = extractDataclass(cd)
	if d := cls.Dataclass; d != nil {
		if cls.Docstring != nil {
			for _, f := range d.Fields {
				attachClassDocParam(f.Param, cls.Docstring)
			}
		}
		// A real base may add fields, and only the package resolver can see it.
		if cls.Constructor == nil && d.Init && !slices.ContainsFunc(cd.Bases, func(b ast.Expr) bool { return !IsInertBase(exprToString(b)) }) {
			cls.Constructor = DataclassConstructor(d.Fields)
		}
	}
	if cls.Constructor != nil && cls.Docstring != nil {
		attachClassDocParams(cls)
	}

	return cls
}

// attachClassDocParams lets class docstring params fill gaps in the constructor's
// parameters. The __init__ docstring wins.
func attachClassDocParams(cls *model.Class) {
	for _, p := range cls.Constructor.Parameters {
		attachClassDocParam(p, cls.Docstring)
	}
}

func attachClassDocParam(p *model.Parameter, doc *model.Docstring) {
	if p.DocParam != nil {
		return
	}
	for _, dp := range doc.Params {
		if dp.Name == p.Name {
			p.DocParam = dp
			return
		}
	}
}

// InheritConstructor gives cls its own copy of ctor, the constructor Python would
// run for it, so documenting cls does not change the class that defines ctor.
func InheritConstructor(cls *model.Class, ctor *model.Function) {
	c := *ctor
	c.Parameters = make([]*model.Parameter, len(ctor.Parameters))
	for i, p := range ctor.Parameters {
		cp := *p
		c.Parameters[i] = &cp
	}
	cls.Constructor = &c
	if cls.Docstring != nil {
		attachClassDocParams(cls)
	}
}

// extractClassStmts walks statements of a class body, populating cls. It recurses
// into If and PassThrough bodies like extractStmts. __init__ lands in cls.Methods
// until extractClass moves it to cls.Constructor.
func (e *Extractor) extractClassStmts(stmts []ast.Stmt, cls *model.Class, moduleExports map[string]struct{}) {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.FunctionDef:
			if isPrivateMember(s.Name) {
				continue
			}
			if accessor, ok := propertyAccessor(s.Decorators); ok {
				e.applyPropertyAccessor(cls, s, accessor)
				continue
			}
			if slices.ContainsFunc(propertyDecorators, func(name string) bool { return hasDecorator(s.Decorators, name) }) {
				cls.Attributes = append(cls.Attributes, e.extractPropertyAttribute(s))
				continue
			}
			if s.Name == "__init__" {
				cls.Attributes = append(cls.Attributes, e.extractInitAttributes(s)...)
			}
			cls.Methods = append(cls.Methods, e.extractFunction(s, true))

		case *ast.AnnAssign:
			attr := e.extractAttribute(s)
			if attr != nil && !isPrivateMember(attr.Name) {
				cls.Attributes = append(cls.Attributes, attr)
			}

		case *ast.Assign:
			for _, target := range s.Targets {
				if n, ok := target.(*ast.Name); ok {
					if isPrivateMember(n.Name) {
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
			// Same privacy rule as methods.
			if isPrivateMember(s.Name) {
				continue
			}
			cls.Classes = append(cls.Classes, e.extractClass(s, moduleExports))

		case *ast.PassThrough:
			e.extractClassStmts(s.Body, cls, moduleExports)

		case *ast.If:
			e.extractClassStmts(s.Body, cls, moduleExports)
			e.extractClassStmts(s.Orelse, cls, moduleExports)
		}
	}
}

// isPrivateMember reports whether a class member is private: _name or the
// name-mangled __name, but not a dunder such as __len__. Python mangles a name
// that starts with two underscores unless it also ends with two.
func isPrivateMember(name string) bool {
	isDunder := strings.HasPrefix(name, "__") && strings.HasSuffix(name, "__")
	return strings.HasPrefix(name, "_") && !isDunder
}

// propertyDecorators lists the decorators that turn a method into an attribute.
var propertyDecorators = []string{"property", "cached_property", "abstractproperty"}

// propertyAccessor returns the accessor kind ("setter", "getter", "deleter") of
// a @x.setter style decorator.
func propertyAccessor(decorators []ast.Expr) (string, bool) {
	for _, dec := range decorators {
		if a, ok := dec.(*ast.Attribute); ok {
			switch a.Attr {
			case "setter", "getter", "deleter":
				return a.Attr, true
			}
		}
	}
	return "", false
}

// applyPropertyAccessor records a @x.setter style method on cls. It never adds a
// method: the accessor belongs to the attribute of the same name.
func (e *Extractor) applyPropertyAccessor(cls *model.Class, fd *ast.FunctionDef, accessor string) {
	for _, attr := range slices.Backward(cls.Attributes) {
		if attr.Name == fd.Name {
			attr.IsProperty = true
			attr.Writable = attr.Writable || accessor == "setter"
			// The accessor is a class-level definition, and __init__ only sets it.
			if attr.Instance {
				attr.Instance, attr.Value = false, ""
			}
			return
		}
	}
	// A custom descriptor such as @hybrid_property stays where it is, as a method.
	if slices.ContainsFunc(cls.Methods, func(m *model.Function) bool { return m.Name == fd.Name }) {
		return
	}
	// The property is not in this body, as in @Base.x.setter. The method still
	// binds a property of its own name.
	attr := e.extractPropertyAttribute(fd)
	attr.Writable = accessor == "setter"
	if accessor != "getter" {
		attr.Type = nil
	}
	cls.Attributes = append(cls.Attributes, attr)
}

// isOverload reports whether fn is decorated with overload or typing.overload.
func isOverload(fn *model.Function) bool {
	return slices.ContainsFunc(fn.Decorators, func(d string) bool {
		return d == "overload" || strings.HasSuffix(d, ".overload")
	})
}

// collapseFunctions leaves one function per name, at the position of the name's
// first definition. The primary is the implementation, or the first overload if
// there is no implementation. Among several implementations (version branches)
// it is the last with a docstring, otherwise the last.
func collapseFunctions(fns []*model.Function) []*model.Function {
	type group struct{ impls, overloads []*model.Function }
	groups := make(map[string]*group, len(fns))
	var names []string
	for _, fn := range fns {
		g, ok := groups[fn.Name]
		if !ok {
			g = &group{}
			groups[fn.Name] = g
			names = append(names, fn.Name)
		}
		if isOverload(fn) {
			g.overloads = append(g.overloads, fn)
		} else {
			g.impls = append(g.impls, fn)
		}
	}
	out := fns[:0:0]
	for _, name := range names {
		g := groups[name]
		var primary *model.Function
		if len(g.impls) > 0 {
			primary = g.impls[len(g.impls)-1]
			for _, fn := range slices.Backward(g.impls) {
				if hasDocstring(fn) {
					primary = fn
					break
				}
			}
		} else {
			primary, g.overloads = g.overloads[0], g.overloads[1:]
		}
		primary.Overloads = g.overloads
		if primary.Docstring == nil {
			if i := slices.IndexFunc(g.overloads, hasDocstring); i >= 0 {
				primary.Docstring = g.overloads[i].Docstring
				attachDocParams(primary.Parameters, primary.Docstring.Params)
			}
		}
		out = append(out, primary)
	}
	return out
}

func hasDocstring(fn *model.Function) bool { return fn.Docstring != nil }

// collapseLast leaves one item per name: the last, at the position of the name's
// first definition.
func collapseLast[T any](items []T, name func(T) string) []T {
	index := make(map[string]int, len(items))
	out := items[:0:0]
	for _, it := range items {
		if i, ok := index[name(it)]; ok {
			out[i] = it
			continue
		}
		index[name(it)] = len(out)
		out = append(out, it)
	}
	return out
}

// collapseAttributes leaves one attribute per name. The last definition wins, but
// definitions of a name describe one attribute: "x: int" and a later "self.x = x"
// keep the type, and "self.x = x" does not stop a property from being a property.
// A class-level definition beats __init__ in either source order, because its value
// is the real default. The result is Instance only if every definition is.
func collapseAttributes(attrs []*model.Attribute) []*model.Attribute {
	index := make(map[string]int, len(attrs))
	out := attrs[:0:0]
	for _, a := range attrs {
		i, ok := index[a.Name]
		if !ok {
			index[a.Name] = len(out)
			out = append(out, a)
			continue
		}
		winner, other := a, out[i]
		if (other.IsProperty && !a.IsProperty) || (a.Instance && !other.Instance) {
			winner, other = other, a
		}
		if winner.Type == nil {
			winner.Type = other.Type
		}
		if winner.Docstring == nil {
			winner.Docstring = other.Docstring
		}
		out[i] = winner
	}
	return out
}

// DataclassConstructor builds the __init__ that @dataclass generates from fields,
// inherited fields first. Python places keyword-only fields after all others.
func DataclassConstructor(fields []*model.DataclassField) *model.Function {
	fn := &model.Function{Name: "__init__", IsMethod: true}
	var kwOnly []*model.Parameter
	for _, f := range fields {
		switch {
		case !f.Init:
		case f.Param.Kind == model.ParamKeywordOnly:
			kwOnly = append(kwOnly, f.Param)
		default:
			fn.Parameters = append(fn.Parameters, f.Param)
		}
	}
	fn.Parameters = append(fn.Parameters, kwOnly...)
	return fn
}

// extractDataclass reads the decorator and fields of a @dataclass class. Returns
// nil for other classes.
func extractDataclass(cd *ast.ClassDef) *model.Dataclass {
	var dec ast.Expr
	for _, d := range cd.Decorators {
		target := d
		if c, ok := d.(*ast.Call); ok {
			target = c.Func
		}
		switch t := target.(type) {
		case *ast.Name:
			if t.Name == "dataclass" {
				dec = d
			}
		case *ast.Attribute:
			if t.Attr == "dataclass" {
				dec = d
			}
		}
	}
	if dec == nil {
		return nil
	}
	dc := &model.Dataclass{Init: true}
	decKind := model.ParamPositionalOrKeyword
	if c, ok := dec.(*ast.Call); ok {
		if v := callKeyword(c, "init"); v != nil && exprToString(v) == "False" {
			dc.Init = false
		}
		if v := callKeyword(c, "kw_only"); v != nil && exprToString(v) == "True" {
			decKind = model.ParamKeywordOnly
		}
	}

	afterSentinel := false
	for _, stmt := range cd.Body {
		s, ok := stmt.(*ast.AnnAssign)
		if !ok {
			continue
		}
		n, ok := s.Target.(*ast.Name)
		if !ok {
			continue
		}
		ann := exprToString(s.Annotation)
		if ann == "ClassVar" || strings.HasPrefix(ann, "ClassVar[") || strings.HasSuffix(ann, ".ClassVar") || strings.Contains(ann, ".ClassVar[") {
			continue
		}
		if ann == "KW_ONLY" || strings.HasSuffix(ann, ".KW_ONLY") {
			afterSentinel = true
			continue
		}

		kind := decKind
		if afterSentinel {
			kind = model.ParamKeywordOnly
		}
		p := &model.Parameter{Name: n.Name}
		init := true
		if s.Value != nil {
			p.Default = exprToString(s.Value)
			if c, ok := s.Value.(*ast.Call); ok && isFieldCall(c) {
				if v := callKeyword(c, "init"); v != nil && exprToString(v) == "False" {
					init = false
				}
				if v := callKeyword(c, "kw_only"); v != nil {
					if exprToString(v) == "True" {
						kind = model.ParamKeywordOnly
					} else if exprToString(v) == "False" {
						kind = model.ParamPositionalOrKeyword
					}
				}
			}
		}
		p.Kind = kind

		p.Type = exprToTypeExpr(s.Annotation)
		if sub, ok := s.Annotation.(*ast.Subscript); ok {
			if base := exprToString(sub.Value); base == "InitVar" || strings.HasSuffix(base, ".InitVar") {
				p.Type = exprToTypeExpr(sub.Slice)
			}
		}

		dc.Fields = append(dc.Fields, &model.DataclassField{Param: p, Init: init})
	}
	return dc
}

// callKeyword returns the value of keyword argument name in call c, or nil.
func callKeyword(c *ast.Call, name string) ast.Expr {
	for _, kw := range c.Keywords {
		if kw.Arg == name {
			return kw.Value
		}
	}
	return nil
}

// isFieldCall reports whether c is field(...) or dataclasses.field(...).
func isFieldCall(c *ast.Call) bool {
	switch f := c.Func.(type) {
	case *ast.Name:
		return f.Name == "field"
	case *ast.Attribute:
		return f.Attr == "field"
	}
	return false
}

// IsInertBase reports whether a base class adds neither dataclass fields nor a
// constructor: object, ABC, Protocol and Generic[...], with optional module
// prefixes. s is the base as written in the source.
func IsInertBase(s string) bool {
	if i := strings.Index(s, "["); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimPrefix(s, "typing.")
	s = strings.TrimPrefix(s, "typing_extensions.")
	switch s {
	case "object", "ABC", "abc.ABC", "Protocol", "Generic":
		return true
	}
	return false
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
						if isPrivateMember(a.Attr) {
							continue
						}
						attrs = append(attrs, &model.Attribute{
							Name:     a.Attr,
							Value:    exprToString(s.Value),
							Instance: true,
						})
					}
				}
			}
		case *ast.AnnAssign:
			if a, ok := s.Target.(*ast.Attribute); ok {
				if selfName, ok := a.Value.(*ast.Name); ok && selfName.Name == "self" {
					if isPrivateMember(a.Attr) {
						continue
					}
					attr := &model.Attribute{
						Name:     a.Attr,
						Type:     exprToTypeExpr(s.Annotation),
						Instance: true,
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

// extractAllExports reads __all__ from the module body, in statement order. It
// follows the same blocks as extractStmts (try/except, if, for, with).
// Returns nil if the module never touches __all__.
//
// At module level "__all__ = x" replaces the list, "+=" adds and "-=" removes.
// Inside a block a plain assignment adds, because the condition is unknown and
// every branch counts.
func extractAllExports(body []ast.Stmt) *model.Exports {
	var ex *model.Exports
	var walk func(stmts []ast.Stmt, nested bool)
	add := func(value ast.Expr) {
		if ex == nil {
			ex = &model.Exports{}
		}
		addExportExpr(ex, value)
	}
	// assign handles a plain assignment. "__all__ = __all__ + x" keeps the old
	// list, because addExportExpr ignores __all__ itself.
	assign := func(value ast.Expr, nested bool) {
		if !nested && !refersToAll(value) {
			ex = nil
		}
		add(value)
	}
	walk = func(stmts []ast.Stmt, nested bool) {
		for _, stmt := range stmts {
			switch s := stmt.(type) {
			case *ast.Assign:
				for _, target := range s.Targets {
					n, ok := target.(*ast.Name)
					if !ok || n.Name != "__all__" {
						continue
					}
					switch {
					case !s.Augmented:
						assign(s.Value, nested)
					case s.Op == token.PLUSEQ || s.Op == token.PIPEEQ:
						add(s.Value)
					case s.Op == token.MINUSEQ:
						if ex != nil {
							removeExportExpr(ex, s.Value)
						}
					default:
						// Any other operator cannot be read statically.
						if ex == nil {
							ex = &model.Exports{}
						}
						ex.Dynamic = true
					}
				}
			case *ast.AnnAssign:
				if n, ok := s.Target.(*ast.Name); ok && n.Name == "__all__" && s.Value != nil {
					assign(s.Value, nested)
				}
			case *ast.ExprStmt:
				// __all__.extend(...) and __all__.append(...).
				c, ok := s.Value.(*ast.Call)
				if !ok || len(c.Args) != 1 {
					continue
				}
				f, ok := c.Func.(*ast.Attribute)
				if !ok {
					continue
				}
				if n, ok := f.Value.(*ast.Name); !ok || n.Name != "__all__" {
					continue
				}
				switch f.Attr {
				case "extend":
					add(c.Args[0])
				case "append":
					add(&ast.List{Elts: c.Args})
				case "remove":
					if ex != nil {
						removeExportExpr(ex, &ast.List{Elts: c.Args})
					}
				}
			case *ast.If:
				walk(s.Body, true)
				walk(s.Orelse, true)
			case *ast.PassThrough:
				walk(s.Body, true)
			}
		}
	}
	walk(body, false)
	return ex
}

// refersToAll reports whether a __all__ expression reads __all__ itself.
func refersToAll(e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.Name:
		return v.Name == "__all__"
	case *ast.BinOp:
		return refersToAll(v.Left) || refersToAll(v.Right)
	case *ast.List:
		return slices.ContainsFunc(v.Elts, refersToAll)
	case *ast.Tuple:
		return slices.ContainsFunc(v.Elts, refersToAll)
	case *ast.Starred:
		return refersToAll(v.Value)
	}
	return false
}

// removeExportExpr removes string literals from ex. Removing anything else marks
// ex as Dynamic, because the result cannot be read statically.
func removeExportExpr(ex *model.Exports, e ast.Expr) {
	var elts []ast.Expr
	switch v := e.(type) {
	case *ast.List:
		elts = v.Elts
	case *ast.Tuple:
		elts = v.Elts
	case *ast.Set:
		elts = v.Elts
	default:
		ex.Dynamic = true
		return
	}
	for _, elt := range elts {
		c, ok := elt.(*ast.Constant)
		if !ok || c.Kind != "str" {
			ex.Dynamic = true
			continue
		}
		ex.Names = slices.DeleteFunc(ex.Names, func(n string) bool { return n == c.Value })
	}
}

// addExportExpr adds the content of one __all__ expression to ex: string
// literals in a list or tuple, "+" between such lists, and other.__all__
// references. Anything else marks ex as Dynamic.
func addExportExpr(ex *model.Exports, e ast.Expr) {
	switch v := e.(type) {
	case *ast.List:
		addExportElts(ex, v.Elts)
	case *ast.Tuple:
		addExportElts(ex, v.Elts)
	case *ast.Set:
		addExportElts(ex, v.Elts)
	case *ast.Name:
		if v.Name != "__all__" {
			ex.Dynamic = true
		}
	case *ast.BinOp:
		if v.Op != token.PLUS {
			ex.Dynamic = true
			return
		}
		addExportExpr(ex, v.Left)
		addExportExpr(ex, v.Right)
	case *ast.Attribute:
		if v.Attr != "__all__" {
			ex.Dynamic = true
			return
		}
		if !isDottedName(v.Value) {
			ex.Dynamic = true
			return
		}
		if ref := exprToString(v.Value); !slices.Contains(ex.Refs, ref) {
			ex.Refs = append(ex.Refs, ref)
		}
	default:
		ex.Dynamic = true
	}
}

func addExportElts(ex *model.Exports, elts []ast.Expr) {
	for _, elt := range elts {
		switch c := elt.(type) {
		case *ast.Constant:
			if c.Kind != "str" {
				ex.Dynamic = true
			} else if !slices.Contains(ex.Names, c.Value) {
				ex.Names = append(ex.Names, c.Value)
			}
		case *ast.Starred:
			addExportExpr(ex, c.Value)
		default:
			ex.Dynamic = true
		}
	}
}

// isDottedName reports whether e is a name or a chain of attributes on a name.
func isDottedName(e ast.Expr) bool {
	for {
		switch v := e.(type) {
		case *ast.Name:
			return true
		case *ast.Attribute:
			e = v.Value
		default:
			return false
		}
	}
}

// isPublic determines if a name should be included in the output.
// If exports is non-nil (__all__ exists), the name must be in exports.
// Otherwise, names starting with _ are considered private.
func isPublic(name string, exports map[string]struct{}) bool {
	if exports != nil {
		_, ok := exports[name]
		return ok
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
