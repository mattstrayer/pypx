package handler

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/pypx/goopy/model"
)

func TestServedDecorators(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"none", nil, nil},
		{"bare", []string{"final", "override"}, []string{"final", "override"}},
		{"dotted", []string{"functools.lru_cache", "abc.abstractmethod", "typing.final"}, []string{"lru_cache", "abstractmethod", "final"}},
		{"arguments", []string{`deprecated("use g(), not f.old")`, "functools.lru_cache(maxsize=a.b)"}, []string{"deprecated", "lru_cache"}},
		{"dataclass with arguments", []string{"dataclasses.dataclass(frozen=True)"}, []string{"dataclass"}},
		{"context managers", []string{"contextlib.contextmanager", "asynccontextmanager"}, []string{"contextmanager", "asynccontextmanager"}},
		{"abstract forms", []string{"abc.abstractclassmethod", "abstractstaticmethod"}, []string{"abstractmethod"}},
		{"project helpers", []string{"_generative", "util.memoized_property", "click.option(1)"}, nil},
		{"binding is not served", []string{"classmethod", "staticmethod"}, nil},
		{"overload is not served", []string{"typing.overload"}, nil},
		{"cache only from functools", []string{"cache", "functools.cache", "reflection.cache"}, []string{"cache"}},
		{"reflection.cache alone", []string{"reflection.cache"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := servedDecorators(tt.in); !slices.Equal(got, tt.want) {
				t.Errorf("servedDecorators(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// One fact gets one label: the binding stays in MethodKind.
func TestConvertFunction_BindingAndDecorators(t *testing.T) {
	m := &model.Function{Name: "make", IsMethod: true, Decorators: []string{"abc.abstractclassmethod", "typing.final"}}
	sym := convertFunction(m, nil)
	if sym.MethodKind != "classmethod" {
		t.Errorf("MethodKind = %q", sym.MethodKind)
	}
	if want := []string{"abstractmethod", "final"}; !slices.Equal(sym.Decorators, want) {
		t.Errorf("Decorators = %q, want %q", sym.Decorators, want)
	}
}

func TestConvertClass_Decorators(t *testing.T) {
	sym := convertClass(&model.Class{Name: "C", Decorators: []string{"dataclasses.dataclass(frozen=True)", "typing.final"}}, nil, "m")
	if want := []string{"dataclass", "final"}; !slices.Equal(sym.Decorators, want) {
		t.Errorf("Decorators = %q, want %q", sym.Decorators, want)
	}
	viaStub := convertClass(&model.Class{Name: "C", Dataclass: &model.Dataclass{}}, nil, "m")
	if want := []string{"dataclass"}; !slices.Equal(viaStub.Decorators, want) {
		t.Errorf("Decorators with Dataclass set = %q, want %q", viaStub.Decorators, want)
	}
}

func TestConvertFunction_Overloads(t *testing.T) {
	iterable := &model.TypeExpr{Raw: "Iterable[_T]"}
	fn := &model.Function{
		Name:       "first",
		Parameters: []*model.Parameter{{Name: "iterable", Kind: model.ParamPositionalOrKeyword}, {Name: "default", Kind: model.ParamPositionalOrKeyword}},
		Overloads: []*model.Function{
			{Name: "first", Decorators: []string{"overload"}, Parameters: []*model.Parameter{{Name: "iterable", Type: iterable, Kind: model.ParamPositionalOrKeyword}}, Returns: &model.TypeExpr{Raw: "_T"}},
			{Name: "first", Decorators: []string{"typing.overload"}, IsAsync: true, Parameters: []*model.Parameter{
				{Name: "a", Kind: model.ParamPositionalOnly},
				{Name: "b", Kind: model.ParamPositionalOrKeyword, Default: "1"},
				{Name: "c", Kind: model.ParamKeywordOnly, Type: &model.TypeExpr{Raw: "int"}},
				{Name: "kw", Kind: model.ParamVarKeyword},
			}},
			{Name: "first", Decorators: []string{"overload"}, Parameters: []*model.Parameter{
				{Name: "args", Kind: model.ParamVarPositional},
				{Name: "c", Kind: model.ParamKeywordOnly},
			}},
			{Name: "first", Decorators: []string{"overload"}, Parameters: []*model.Parameter{{Name: "only", Kind: model.ParamPositionalOnly}}},
		},
	}
	sym := convertFunction(fn, nil)
	if sym.Signature != "def first(iterable, default)" {
		t.Errorf("main signature = %q, want the implementation", sym.Signature)
	}
	want := []string{
		"def first(iterable: Iterable[_T]) -> _T",
		"async def first(a, /, b = 1, *, c: int, **kw)",
		"def first(*args, c)",
		"def first(only, /)",
	}
	if !slices.Equal(sym.Overloads, want) {
		t.Errorf("Overloads = %q, want %q", sym.Overloads, want)
	}
}

func TestConvertFunction_OverloadsUsePublishedName(t *testing.T) {
	fn := &model.Function{Name: "renamed", Overloads: []*model.Function{{Name: "original", Decorators: []string{"overload"}}}}
	if got := convertFunction(fn, nil).Overloads; !slices.Equal(got, []string{"def renamed()"}) {
		t.Errorf("Overloads = %q", got)
	}
}

// With no implementation the first overload becomes the function, so it counts too.
// An overload alone is the main signature, and a fold would only repeat it.
func TestConvertFunction_OverloadWithoutImplementation(t *testing.T) {
	second := &model.Function{Name: "f", Decorators: []string{"overload"}, Parameters: []*model.Parameter{{Name: "b"}}}
	fn := &model.Function{Name: "f", Decorators: []string{"overload"}, Parameters: []*model.Parameter{{Name: "a"}}, Overloads: []*model.Function{second}}
	if got, want := convertFunction(fn, nil).Overloads, []string{"def f(a)", "def f(b)"}; !slices.Equal(got, want) {
		t.Errorf("Overloads = %q, want %q", got, want)
	}
	alone := &model.Function{Name: "f", Decorators: []string{"overload"}}
	if got := convertFunction(alone, nil).Overloads; got != nil {
		t.Errorf("lone overload: Overloads = %q", got)
	}
}

func TestConvertFunction_StubOverloads(t *testing.T) {
	overload := func(param string) *model.Function {
		return &model.Function{Name: "f", Decorators: []string{"overload"}, Parameters: []*model.Parameter{{Name: param}}}
	}
	fn := &model.Function{Name: "f"}

	stub := overload("a")
	stub.Overloads = []*model.Function{overload("b")}
	if got, want := convertFunction(fn, stub).Overloads, []string{"def f(a)", "def f(b)"}; !slices.Equal(got, want) {
		t.Errorf("overload-only stub: Overloads = %q, want %q", got, want)
	}

	impl := &model.Function{Name: "f", Overloads: []*model.Function{overload("c")}}
	if got, want := convertFunction(fn, impl).Overloads, []string{"def f(c)"}; !slices.Equal(got, want) {
		t.Errorf("stub with implementation: Overloads = %q, want %q", got, want)
	}

	own := &model.Function{Name: "f", Overloads: []*model.Function{overload("own")}}
	if got, want := convertFunction(own, impl).Overloads, []string{"def f(own)"}; !slices.Equal(got, want) {
		t.Errorf("source overloads win: Overloads = %q, want %q", got, want)
	}

	if got := convertFunction(fn, &model.Function{Name: "f"}).Overloads; got != nil {
		t.Errorf("plain stub: Overloads = %q", got)
	}
}

func TestConvertClass_ConstructorOverloads(t *testing.T) {
	sym := convertClass(&model.Class{Name: "C", Constructor: &model.Function{
		Name:      "__init__",
		Overloads: []*model.Function{{Name: "__init__", Decorators: []string{"overload"}, Parameters: []*model.Parameter{{Name: "x", Type: &model.TypeExpr{Raw: "int"}}}}},
	}}, nil, "m")
	if want := []string{"def __init__(x: int)"}; !slices.Equal(sym.Overloads, want) {
		t.Errorf("Overloads = %q, want %q", sym.Overloads, want)
	}
}

func TestConvertClass_NestedClasses(t *testing.T) {
	inner := &model.Class{
		Name:        "Inner",
		BaseClasses: []*model.TypeRef{{Name: "enum.Enum"}},
		Decorators:  []string{"typing.final"},
		Methods:     []*model.Function{{Name: "go", IsMethod: true, Decorators: []string{"classmethod"}}},
		Attributes:  []*model.Attribute{{Name: "x", Value: "1"}},
		Classes:     []*model.Class{{Name: "Deep"}},
	}
	outer := &model.Class{Name: "Outer", Classes: []*model.Class{
		inner,
		{Name: "_Private"},
		{Name: "__Mangled"},
	}}

	sym := convertClass(outer, nil, "m")
	if len(sym.Classes) != 1 {
		t.Fatalf("Classes = %+v, want Inner only", sym.Classes)
	}
	got := sym.Classes[0]
	if got.Name != "Inner" || got.Kind != "class" || got.Signature != "class Inner(enum.Enum)" {
		t.Errorf("Inner = %+v", got)
	}
	if got.Attributes == nil || len(got.Attributes) != 1 {
		t.Errorf("Inner.Attributes = %+v", got.Attributes)
	}
	if len(got.Methods) != 1 || got.Methods[0].MethodKind != "classmethod" {
		t.Errorf("Inner.Methods = %+v", got.Methods)
	}
	if !slices.Equal(got.Decorators, []string{"final"}) {
		t.Errorf("Inner.Decorators = %q", got.Decorators)
	}
	if len(got.Classes) != 1 || got.Classes[0].Name != "Deep" {
		t.Errorf("Inner.Classes = %+v", got.Classes)
	}
}

func TestConvertClass_NestedStubs(t *testing.T) {
	nested := func(returns string) *model.Class {
		fn := &model.Function{Name: "go", IsMethod: true}
		if returns != "" {
			fn.Returns = &model.TypeExpr{Raw: returns}
		}
		return &model.Class{Name: "Inner", Methods: []*model.Function{fn}}
	}
	stubs := buildStubIndex(&model.Package{Modules: []*model.Module{
		{Name: "m", Classes: []*model.Class{{Name: "Outer", Classes: []*model.Class{nested("Published")}}}},
		{Name: "m._core", Classes: []*model.Class{{Name: "Copied", Classes: []*model.Class{nested("AtOrigin")}}}},
	}})

	published := convertClass(&model.Class{Name: "Outer", Classes: []*model.Class{nested("")}}, stubs, "m")
	if r := published.Classes[0].Methods[0].Returns; r == nil || r.Type != "Published" {
		t.Errorf("published path: Returns = %+v", r)
	}

	copied := convertClass(&model.Class{Name: "Copied", Origin: "m._core", Classes: []*model.Class{nested("")}}, stubs, "m")
	if r := copied.Classes[0].Methods[0].Returns; r == nil || r.Type != "AtOrigin" {
		t.Errorf("origin path: Returns = %+v", r)
	}
}

// A nested exception keeps kind class, and it does not move its parent to Exceptions.
func TestConvertToDocsResponse_NestedExceptionStaysInParent(t *testing.T) {
	resp := convertToDocsResponse("p", "1", &model.Package{Modules: []*model.Module{{
		Name: "m",
		Classes: []*model.Class{{
			Name:    "Outer",
			Classes: []*model.Class{{Name: "InnerError", BaseClasses: []*model.TypeRef{{Name: "Exception"}}}},
		}},
	}}}, nil)
	dm := resp.Modules[0]
	if len(dm.Exceptions) != 0 || len(dm.Classes) != 1 {
		t.Fatalf("Exceptions = %+v, Classes = %+v", dm.Exceptions, dm.Classes)
	}
	if kind := dm.Classes[0].Classes[0].Kind; kind != "class" {
		t.Errorf("nested kind = %q", kind)
	}
}

func TestDocSymbol_NewFieldsOmittedWhenEmpty(t *testing.T) {
	encoded, err := json.Marshal(DocSymbol{Name: "f", Kind: "function"})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"classes", "decorators", "overloads"} {
		if _, ok := fields[name]; ok {
			t.Errorf("%s served for a symbol with none: %s", name, encoded)
		}
	}
}

func TestEncodeDocs_RoundTripsMembers(t *testing.T) {
	resp := DocsResponse{Package: "p", Version: "1", Available: true, Modules: []DocModule{{
		Name: "m",
		Classes: []DocSymbol{{
			Name: "Outer", Kind: "class", Signature: "class Outer",
			Classes:   []DocSymbol{{Name: "Inner", Kind: "class", Signature: "class Inner", Decorators: []string{"final"}}},
			Overloads: []string{"def __init__(x)"},
		}},
	}}}
	encoded, err := encodeDocs(resp)
	if err != nil {
		t.Fatal(err)
	}
	var back DocsResponse
	if err := json.Unmarshal(encoded, &back); err != nil {
		t.Fatal(err)
	}
	outer := back.Modules[0].Classes[0]
	if len(outer.Classes) != 1 || !slices.Equal(outer.Classes[0].Decorators, []string{"final"}) || !slices.Equal(outer.Overloads, []string{"def __init__(x)"}) {
		t.Errorf("round trip = %+v", outer)
	}
}

func TestFlattenSymbols_NestedClasses(t *testing.T) {
	got := flattenSymbols(DocsResponse{Modules: []DocModule{{
		Name: "m",
		Classes: []DocSymbol{{
			Name: "Outer", Signature: "class Outer",
			Classes: []DocSymbol{{
				Name: "Inner", Signature: "class Inner(Base)",
				Methods:    []DocSymbol{{Name: "go", Signature: "def go()", Overloads: []string{"def go(a)"}}},
				Attributes: []DocAttribute{{Name: "x", Type: "int"}},
				Classes:    []DocSymbol{{Name: "Deep", Signature: "class Deep"}},
			}},
		}},
		Aliases: []DocAlias{{Name: "Alias", Target: "m.Outer"}},
	}}})
	want := map[string]string{
		"m.Outer":            "class Outer",
		"m.Outer.Inner":      "class Inner(Base)",
		"m.Outer.Inner.go":   "def go() [overloads: def go(a)]",
		"m.Outer.Inner.x":    "x: int",
		"m.Outer.Inner.Deep": "class Deep",
		"m.Alias":            "class Outer",
		"m.Alias.Inner":      "class Inner(Base)",
		"m.Alias.Inner.go":   "def go() [overloads: def go(a)]",
		"m.Alias.Inner.x":    "x: int",
		"m.Alias.Inner.Deep": "class Deep",
	}
	for path, sig := range want {
		if got[path] != sig {
			t.Errorf("%s = %q, want %q", path, got[path], sig)
		}
	}
	if len(got) != len(want) {
		t.Errorf("rows = %v", got)
	}
}

func TestFlattenSymbols_OverloadOnlyChange(t *testing.T) {
	flatten := func(overloads ...string) map[string]string {
		return flattenSymbols(DocsResponse{Modules: []DocModule{{
			Name:      "m",
			Functions: []DocSymbol{{Name: "f", Signature: "def f(a)", Overloads: overloads}},
		}}})
	}
	if plain := flatten()["m.f"]; plain != "def f(a)" {
		t.Errorf("no overloads: %q", plain)
	}
	if flatten("def f(a: int)")["m.f"] == flatten("def f(a: str)")["m.f"] {
		t.Error("a change to an overload alone must change the row")
	}
}

func TestDocSymbolFrom_CarriesMembers(t *testing.T) {
	in := docSymbolFrom(DocSymbol{
		Name: "C", MethodKind: "classmethod", Decorators: []string{"final"}, Overloads: []string{"def f()"},
		Classes: []DocSymbol{{Name: "Inner"}},
	})
	if in.MethodKind != "classmethod" || !slices.Equal(in.Decorators, []string{"final"}) ||
		!slices.Equal(in.Overloads, []string{"def f()"}) || len(in.Classes) != 1 || in.Classes[0].Name != "Inner" {
		t.Errorf("docSymbolFrom = %+v", in)
	}
}
