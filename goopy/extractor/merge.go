package extractor

import (
	"slices"
	"strings"

	"github.com/pypx/goopy/model"
)

// PrepareStub applies the rules for stubs to a module parsed from a .pyi file.
//
// "from x import a" keeps a private to the stub. Only "from x import a as a",
// "from . import a" and "from x import *" re-export.
//
// "T = TypeVar(...)" and its relatives are not API. They stay reachable for imports,
// but they are public only if a __all__ lists them.
func PrepareStub(m *model.Module) {
	listed := func(name string) bool {
		return m.Exports != nil && slices.Contains(m.Exports.Names, name)
	}
	public := m.Attributes[:0:0]
	for _, a := range m.Attributes {
		a.StubTypeParam = isTypeParamValue(a.Value)
		if a.StubTypeParam && !listed(a.Name) {
			m.Hidden.Attributes = append(m.Hidden.Attributes, a)
		} else {
			public = append(public, a)
		}
	}
	m.Attributes = public
	for _, a := range m.Hidden.Attributes {
		a.StubTypeParam = isTypeParamValue(a.Value)
	}
	for _, imp := range m.ImportFroms {
		switch {
		case imp.Name == "*":
		case imp.Name == "":
			imp.NotExported = imp.Alias != imp.Module
		case imp.Level > 0 && imp.Module == "":
			imp.NotExported = imp.Alias != "" && imp.Alias != imp.Name
		default:
			imp.NotExported = imp.Alias != imp.Name
		}
	}
}

// isTypeParamValue reports whether an attribute's value creates a type parameter.
func isTypeParamValue(value string) bool {
	callee, _, isCall := strings.Cut(value, "(")
	if !isCall {
		return false
	}
	switch callee[strings.LastIndex(callee, ".")+1:] {
	case "TypeVar", "ParamSpec", "TypeVarTuple":
		return true
	}
	return false
}

// MergeStub merges a module's .pyi into its .py. The .py decides names and
// docstrings, and the stub decides types. Definitions only the stub has are added.
// A stub-only name the .py may import from elsewhere goes to base.StubOnly, because
// only the package resolver can tell if that import finds a definition.
func MergeStub(base, stub *model.Module) {
	if base.Docstring == nil {
		base.Docstring = stub.Docstring
	}

	bound := importsName(base)
	base.ImportFroms = append(base.ImportFroms, stub.ImportFroms...)
	mergeExports(base, stub)

	var own model.Definitions
	dest := func(name string) *model.Definitions {
		if bound(name) {
			return &base.StubOnly
		}
		return &own
	}
	for _, group := range []*model.Definitions{
		{Functions: stub.Functions, Classes: stub.Classes, Attributes: stub.Attributes, TypeAliases: stub.TypeAliases},
		&stub.Hidden,
	} {
		for _, f := range group.Functions {
			if fn := findFunction(base, f.Name); fn != nil {
				MergeFunction(fn, f)
			} else if !definesCode(base, f.Name) {
				d := dest(f.Name)
				d.Functions = append(d.Functions, f)
			}
		}
		for _, c := range group.Classes {
			if cls := findClass(base, c.Name); cls != nil {
				MergeClass(cls, c)
			} else if !definesCode(base, c.Name) {
				d := dest(c.Name)
				d.Classes = append(d.Classes, c)
			}
		}
		for _, a := range group.Attributes {
			if !definesCode(base, a.Name) {
				d := dest(a.Name)
				d.Attributes = append(d.Attributes, a)
			}
		}
		for _, t := range group.TypeAliases {
			if !definesCode(base, t.Name) {
				d := dest(t.Name)
				d.TypeAliases = append(d.TypeAliases, t)
			}
		}
	}
	AddStubDefinitions(base, own)
}

// AddStubDefinitions adds definitions from a stub to m, under m's rules for public
// names. A function, class or type alias replaces an attribute of the same name,
// because "f = make()" says less than the stub's "def f(a: int) -> str". An
// attribute that already exists takes the stub's type.
func AddStubDefinitions(m *model.Module, defs model.Definitions) {
	var exports map[string]struct{}
	if ex := m.Exports; ex != nil && !ex.Dynamic && len(ex.Refs) == 0 {
		exports = make(map[string]struct{}, len(ex.Names))
		for _, n := range ex.Names {
			exports[n] = struct{}{}
		}
	}
	for _, f := range defs.Functions {
		if pub := replaceAttribute(m, f.Name); isPublicAfterReplace(pub, f.Name, exports) {
			m.Functions = append(m.Functions, f)
		} else {
			m.Hidden.Functions = append(m.Hidden.Functions, f)
		}
	}
	for _, c := range defs.Classes {
		if pub := replaceAttribute(m, c.Name); isPublicAfterReplace(pub, c.Name, exports) {
			m.Classes = append(m.Classes, c)
		} else {
			m.Hidden.Classes = append(m.Hidden.Classes, c)
		}
	}
	for _, t := range defs.TypeAliases {
		if pub := replaceAttribute(m, t.Name); isPublicAfterReplace(pub, t.Name, exports) {
			m.TypeAliases = append(m.TypeAliases, t)
		} else {
			m.Hidden.TypeAliases = append(m.Hidden.TypeAliases, t)
		}
	}
	for _, a := range defs.Attributes {
		if existing := findAttribute(m, a.Name); existing != nil {
			MergeAttribute(existing, a)
		} else if isPublic(a.Name, exports) && (exports != nil || !a.StubTypeParam) {
			m.Attributes = append(m.Attributes, a)
		} else {
			m.Hidden.Attributes = append(m.Hidden.Attributes, a)
		}
	}
}

// MergeFunction applies a stub's signature to fn. Parameters match by name, and a
// stub parameter fn does not have is ignored. A stub that only has overloads says
// nothing about the implementation's own parameters, so it adds overloads and the
// decorators that apply to all of them.
func MergeFunction(fn, stub *model.Function) {
	if fn.Docstring == nil && stub.Docstring != nil {
		fn.Docstring = stub.Docstring
		attachDocParams(fn.Parameters, fn.Docstring.Params)
	}
	fn.Decorators = addDecorators(fn.Decorators, wholeDecorators(stub))
	if isOverload(stub) {
		first := *stub
		first.Overloads = nil
		fn.Overloads = append([]*model.Function{&first}, stub.Overloads...)
		return
	}
	if len(stub.Overloads) > 0 {
		fn.Overloads = stub.Overloads
	}
	if stub.Returns != nil {
		fn.Returns = stub.Returns
	}
	for _, p := range fn.Parameters {
		for _, sp := range stub.Parameters {
			if sp.Name == p.Name && sp.Type != nil {
				p.Type = sp.Type
			}
		}
	}
}

// FunctionDecorators returns the decorators that apply to all of fn, then the ones
// stub adds. It never returns @overload. stub may be nil. It serves a stub that was
// not merged into fn, as from a separate stub package.
func FunctionDecorators(fn, stub *model.Function) []string {
	if stub == nil {
		return wholeDecorators(fn)
	}
	return addDecorators(wholeDecorators(fn), wholeDecorators(stub))
}

// ClassDecorators returns cls's decorators, then the ones stub adds. stub may be nil.
// It serves a stub that was not merged into cls, as from a separate stub package.
func ClassDecorators(cls, stub *model.Class) []string {
	if stub == nil {
		return cls.Decorators
	}
	return addDecorators(cls.Decorators, stub.Decorators)
}

// wholeDecorators returns fn's decorators without @overload. A decorator on an
// overload can describe that overload alone, as @deprecated does, so it counts only
// if every overload has it. @final and @override count from one overload, because
// the typing rules put them on the first overload in a stub.
func wholeDecorators(fn *model.Function) []string {
	if !isOverload(fn) {
		return fn.Decorators
	}
	var out []string
	for _, dec := range fn.Decorators {
		switch name := DecoratorName(dec); {
		case name == "overload":
		case name == "final", name == "override",
			!slices.ContainsFunc(fn.Overloads, func(o *model.Function) bool { return !hasDecoratorNamed(o.Decorators, name) }):
			out = append(out, dec)
		}
	}
	return out
}

// addDecorators returns decs in order, then each decorator in more whose name decs
// does not have. @typing.final and @final count as one, and the first stays.
func addDecorators(decs, more []string) []string {
	out := slices.Clip(decs)
	for _, dec := range more {
		if !hasDecoratorNamed(out, DecoratorName(dec)) {
			out = append(out, dec)
		}
	}
	return out
}

func hasDecoratorNamed(decs []string, name string) bool {
	return slices.ContainsFunc(decs, func(dec string) bool { return DecoratorName(dec) == name })
}

// DecoratorName returns a decorator's name without its arguments or module path:
// functools.lru_cache(maxsize=128) gives lru_cache.
func DecoratorName(dec string) string {
	head, _, _ := strings.Cut(dec, "(")
	return head[strings.LastIndex(head, ".")+1:]
}

// MergeClass merges a stub's class into cls, member by member.
func MergeClass(cls, stub *model.Class) {
	if cls.Docstring == nil && stub.Docstring != nil {
		cls.Docstring = stub.Docstring
	}
	cls.Decorators = addDecorators(cls.Decorators, stub.Decorators)
	if len(cls.BaseClasses) == 0 {
		cls.BaseClasses = stub.BaseClasses
	}
	if cls.Dataclass == nil {
		cls.Dataclass = stub.Dataclass
	}
	switch {
	case stub.Constructor == nil:
	case cls.Constructor == nil:
		cls.Constructor = stub.Constructor
	default:
		MergeFunction(cls.Constructor, stub.Constructor)
	}
	if cls.Constructor != nil && cls.Docstring != nil {
		attachClassDocParams(cls)
	}

	// A member a stub adds never takes the place of a method, nested class or
	// property, only of a plain attribute.
	methodAt := func(name string) int {
		return slices.IndexFunc(cls.Methods, func(f *model.Function) bool { return f.Name == name })
	}
	classAt := func(name string) int {
		return slices.IndexFunc(cls.Classes, func(c *model.Class) bool { return c.Name == name })
	}
	attrAt := func(name string) int {
		return slices.IndexFunc(cls.Attributes, func(a *model.Attribute) bool { return a.Name == name })
	}
	takeName := func(name string) bool {
		i := attrAt(name)
		if i < 0 {
			return true
		}
		if cls.Attributes[i].IsProperty {
			return false
		}
		cls.Attributes = slices.Delete(cls.Attributes, i, i+1)
		return true
	}
	for _, m := range stub.Methods {
		switch i := methodAt(m.Name); {
		case i >= 0:
			MergeFunction(cls.Methods[i], m)
		case classAt(m.Name) < 0 && takeName(m.Name):
			cls.Methods = append(cls.Methods, m)
		}
	}
	for _, c := range stub.Classes {
		switch i := classAt(c.Name); {
		case i >= 0:
			MergeClass(cls.Classes[i], c)
		case methodAt(c.Name) < 0 && takeName(c.Name):
			cls.Classes = append(cls.Classes, c)
		}
	}
	for _, a := range stub.Attributes {
		switch i := attrAt(a.Name); {
		case i >= 0:
			MergeAttribute(cls.Attributes[i], a)
		case methodAt(a.Name) < 0 && classAt(a.Name) < 0:
			cls.Attributes = append(cls.Attributes, a)
		}
	}
}

// MergeAttribute takes the stub's type and adds what attr does not have.
func MergeAttribute(attr, stub *model.Attribute) {
	if stub.Type != nil {
		attr.Type = stub.Type
	}
	if attr.Value == "" && stub.Value != "..." {
		attr.Value = stub.Value
	}
	if attr.Docstring == nil {
		attr.Docstring = stub.Docstring
	}
	attr.IsProperty = attr.IsProperty || stub.IsProperty
	attr.Writable = attr.Writable || stub.Writable
}

// mergeExports decides __all__ for a merged module. A __all__ in the .py that can
// be read wins. Otherwise neither file is the full list: the .py's public names
// and whatever the stub's __all__ lists both count, which Dynamic expresses.
func mergeExports(base, stub *model.Module) {
	if stub.Exports == nil || (base.Exports != nil && !base.Exports.Dynamic) {
		return
	}
	merged := &model.Exports{Dynamic: true}
	if base.Exports != nil {
		merged.Names = slices.Clone(base.Exports.Names)
		merged.Refs = slices.Clone(base.Exports.Refs)
	}
	for _, n := range stub.Exports.Names {
		if !slices.Contains(merged.Names, n) {
			merged.Names = append(merged.Names, n)
		}
	}
	for _, r := range stub.Exports.Refs {
		if !slices.Contains(merged.Refs, r) {
			merged.Refs = append(merged.Refs, r)
		}
	}
	base.Exports = merged
}

// importsName returns a test for names m's imports may bind. A star import may
// bind any name.
func importsName(m *model.Module) func(string) bool {
	star := false
	names := make(map[string]bool, len(m.ImportFroms))
	for _, imp := range m.ImportFroms {
		switch {
		case imp.Name == "*":
			star = true
		case imp.Alias != "":
			names[imp.Alias] = true
		case imp.Name != "":
			names[imp.Name] = true
		default:
			first, _, _ := strings.Cut(imp.Module, ".")
			names[first] = true
		}
	}
	return func(name string) bool { return star || names[name] }
}

// definesCode reports whether m has a function, class or type alias with this name.
func definesCode(m *model.Module, name string) bool {
	return findFunction(m, name) != nil || findClass(m, name) != nil ||
		slices.ContainsFunc(m.TypeAliases, func(t *model.TypeAlias) bool { return t.Name == name }) ||
		slices.ContainsFunc(m.Hidden.TypeAliases, func(t *model.TypeAlias) bool { return t.Name == name })
}

func findFunction(m *model.Module, name string) *model.Function {
	match := func(f *model.Function) bool { return f.Name == name }
	if i := slices.IndexFunc(m.Functions, match); i >= 0 {
		return m.Functions[i]
	}
	if i := slices.IndexFunc(m.Hidden.Functions, match); i >= 0 {
		return m.Hidden.Functions[i]
	}
	return nil
}

func findClass(m *model.Module, name string) *model.Class {
	match := func(c *model.Class) bool { return c.Name == name }
	if i := slices.IndexFunc(m.Classes, match); i >= 0 {
		return m.Classes[i]
	}
	if i := slices.IndexFunc(m.Hidden.Classes, match); i >= 0 {
		return m.Hidden.Classes[i]
	}
	return nil
}

func findAttribute(m *model.Module, name string) *model.Attribute {
	match := func(a *model.Attribute) bool { return a.Name == name }
	if i := slices.IndexFunc(m.Attributes, match); i >= 0 {
		return m.Attributes[i]
	}
	if i := slices.IndexFunc(m.Hidden.Attributes, match); i >= 0 {
		return m.Hidden.Attributes[i]
	}
	return nil
}

// replaceAttribute removes the attribute with this name from m. It returns true
// if the attribute was public, false if it was hidden, and nil if m had none.
func replaceAttribute(m *model.Module, name string) *bool {
	match := func(a *model.Attribute) bool { return a.Name == name }
	if i := slices.IndexFunc(m.Attributes, match); i >= 0 {
		m.Attributes = slices.Delete(m.Attributes, i, i+1)
		pub := true
		return &pub
	}
	if i := slices.IndexFunc(m.Hidden.Attributes, match); i >= 0 {
		m.Hidden.Attributes = slices.Delete(m.Hidden.Attributes, i, i+1)
		pub := false
		return &pub
	}
	return nil
}

// isPublicAfterReplace keeps the place of a replaced attribute. A new name follows
// the module's rules.
func isPublicAfterReplace(replaced *bool, name string, exports map[string]struct{}) bool {
	if replaced != nil {
		return *replaced
	}
	return isPublic(name, exports)
}
