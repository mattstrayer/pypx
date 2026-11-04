package goopy

import (
	"sync"

	"github.com/pypx/goopy/model"
)

// typeFlattener reduces every type expression to its Kind and Raw text. The parsed
// trees take more memory than anything else in a package with many annotations: 47%
// of the live heap at the peak of pulumi-aws.
//
// Equal expressions then share one value, because such a package repeats a few
// annotations hundreds of thousands of times. Nothing changes a TypeExpr after
// parsing. Merges and the API only read one or replace the pointer to it.
type typeFlattener struct {
	mu    sync.Mutex
	types map[typeKey]*model.TypeExpr
}

type typeKey struct {
	kind model.TypeExprKind
	raw  string
}

func newTypeFlattener() *typeFlattener {
	return &typeFlattener{types: map[typeKey]*model.TypeExpr{}}
}

// module flattens every type expression in mod. It is safe for concurrent use.
func (tf *typeFlattener) module(mod *model.Module) {
	tf.definitions(model.Definitions{
		Functions:   mod.Functions,
		Classes:     mod.Classes,
		Attributes:  mod.Attributes,
		TypeAliases: mod.TypeAliases,
	})
	tf.definitions(mod.Hidden)
	tf.definitions(mod.StubOnly)
	for _, ref := range mod.Imports {
		ref.Type = tf.share(ref.Type)
	}
}

func (tf *typeFlattener) definitions(d model.Definitions) {
	for _, f := range d.Functions {
		tf.function(f)
	}
	for _, c := range d.Classes {
		tf.class(c)
	}
	for _, a := range d.Attributes {
		a.Type = tf.share(a.Type)
	}
	for _, t := range d.TypeAliases {
		t.Value = tf.share(t.Value)
	}
}

func (tf *typeFlattener) function(f *model.Function) {
	if f == nil {
		return
	}
	for _, p := range f.Parameters {
		p.Type = tf.share(p.Type)
	}
	f.Returns = tf.share(f.Returns)
	for _, o := range f.Overloads {
		tf.function(o)
	}
}

func (tf *typeFlattener) class(c *model.Class) {
	for _, b := range c.BaseClasses {
		b.Type = tf.share(b.Type)
	}
	for _, m := range c.Methods {
		tf.function(m)
	}
	for _, a := range c.Attributes {
		a.Type = tf.share(a.Type)
	}
	for _, n := range c.Classes {
		tf.class(n)
	}
	tf.function(c.Constructor)
}

// share returns the one flattened value for t.
func (tf *typeFlattener) share(t *model.TypeExpr) *model.TypeExpr {
	if t == nil {
		return nil
	}
	key := typeKey{t.Kind, t.Raw}
	tf.mu.Lock()
	defer tf.mu.Unlock()
	if shared, ok := tf.types[key]; ok {
		return shared
	}
	shared := &model.TypeExpr{Kind: t.Kind, Raw: t.Raw}
	tf.types[key] = shared
	return shared
}
