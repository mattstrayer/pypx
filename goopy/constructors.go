package goopy

import (
	"slices"
	"strings"

	"github.com/pypx/goopy/extractor"
	"github.com/pypx/goopy/model"
)

// classRef is a class and the module that defines it, which its base names are
// read against.
type classRef struct {
	module string
	cls    *model.Class
}

// resolveConstructors gives each top-level class with no constructor the one
// Python would run for it, by walking its bases inside this wheel. A class with
// several real bases, or a base outside the wheel, stays empty: its MRO is not
// known here. Nested classes stay empty too, because their bases can name members
// of the enclosing class. All lookups run before any change, so module order has
// no effect.
func (r *resolver) resolveConstructors(parsed []*parsedModule) {
	type found struct {
		cls  *model.Class
		ctor *model.Function
	}
	var results []found
	for _, pm := range parsed {
		for _, cls := range slices.Concat(pm.mod.Classes, pm.mod.Hidden.Classes) {
			if cls.Constructor != nil {
				continue
			}
			ref := classRef{pm.mod.Name, cls}
			if ctor := r.constructorOf(ref, map[*model.Class]bool{}); ctor != nil {
				results = append(results, found{cls, ctor})
			}
		}
	}
	for _, f := range results {
		extractor.InheritConstructor(f.cls, f.ctor)
	}
}

// baseOf returns the one real base of a class, or nil if it has none. ok is false
// if the base cannot be known.
func (r *resolver) baseOf(ref classRef) (base *classRef, ok bool) {
	var real []string
	for _, b := range ref.cls.BaseClasses {
		if !extractor.IsInertBase(b.Name) {
			real = append(real, b.Name)
		}
	}
	if len(real) == 0 {
		return nil, true
	}
	if len(real) > 1 {
		return nil, false
	}
	name, _, _ := strings.Cut(real[0], "[")
	module, attr := ref.module, name
	if i := strings.LastIndex(name, "."); i >= 0 {
		module = r.refTarget(r.modules[ref.module], name[:i])
		attr = name[i+1:]
	}
	d, found := r.lookup(module, attr, map[[2]string]bool{}, 0)
	if !found || d.cls == nil {
		return nil, false
	}
	return &classRef{d.module, d.cls}, true
}

// constructorOf follows the single-inheritance MRO to the first class that has
// or generates __init__. It returns nil if there is none or it cannot be known.
// seen stops a cycle in the bases.
func (r *resolver) constructorOf(ref classRef, seen map[*model.Class]bool) *model.Function {
	if ref.cls.Constructor != nil {
		return ref.cls.Constructor
	}
	if seen[ref.cls] {
		return nil
	}
	seen[ref.cls] = true

	if d := ref.cls.Dataclass; d != nil && d.Init {
		fields, ok := r.dataclassFields(ref, map[*model.Class]bool{})
		if !ok {
			return nil
		}
		return extractor.DataclassConstructor(fields)
	}
	base, ok := r.baseOf(ref)
	if !ok || base == nil {
		return nil
	}
	return r.constructorOf(*base, seen)
}

// dataclassFields returns the fields @dataclass collects for a class: inherited
// first, with a redeclared field keeping its place. A plain class in the chain
// adds none, but its bases still count. ok is false if a base cannot be known.
func (r *resolver) dataclassFields(ref classRef, seen map[*model.Class]bool) (fields []*model.DataclassField, ok bool) {
	if seen[ref.cls] {
		return nil, false
	}
	seen[ref.cls] = true

	base, ok := r.baseOf(ref)
	if !ok {
		return nil, false
	}
	if base != nil {
		inherited, ok := r.dataclassFields(*base, seen)
		if !ok {
			return nil, false
		}
		fields = slices.Clone(inherited)
	}
	if d := ref.cls.Dataclass; d != nil {
		for _, f := range d.Fields {
			i := slices.IndexFunc(fields, func(g *model.DataclassField) bool { return g.Param.Name == f.Param.Name })
			if i >= 0 {
				fields[i] = f
			} else {
				fields = append(fields, f)
			}
		}
	}
	return fields, true
}
