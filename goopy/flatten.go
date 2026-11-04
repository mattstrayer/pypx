package goopy

import "github.com/pypx/goopy/model"

// flattenTypes reduces every type expression in mod to its Kind and Raw text. The
// parsed trees take more memory than anything else in a package with many
// annotations: 47% of the live heap at the peak of pulumi-aws.
func flattenTypes(mod *model.Module) {
	flattenDefinitions(model.Definitions{
		Functions:   mod.Functions,
		Classes:     mod.Classes,
		Attributes:  mod.Attributes,
		TypeAliases: mod.TypeAliases,
	})
	flattenDefinitions(mod.Hidden)
	for _, ref := range mod.Imports {
		flattenType(ref.Type)
	}
}

func flattenDefinitions(d model.Definitions) {
	for _, f := range d.Functions {
		flattenFunction(f)
	}
	for _, c := range d.Classes {
		flattenClass(c)
	}
	for _, a := range d.Attributes {
		flattenType(a.Type)
	}
	for _, t := range d.TypeAliases {
		flattenType(t.Value)
	}
}

func flattenFunction(f *model.Function) {
	if f == nil {
		return
	}
	for _, p := range f.Parameters {
		flattenType(p.Type)
	}
	flattenType(f.Returns)
}

func flattenClass(c *model.Class) {
	for _, b := range c.BaseClasses {
		flattenType(b.Type)
	}
	for _, m := range c.Methods {
		flattenFunction(m)
	}
	for _, a := range c.Attributes {
		flattenType(a.Type)
	}
	for _, n := range c.Classes {
		flattenClass(n)
	}
	flattenFunction(c.Constructor)
}

func flattenType(t *model.TypeExpr) {
	if t != nil {
		*t = model.TypeExpr{Kind: t.Kind, Raw: t.Raw}
	}
}
