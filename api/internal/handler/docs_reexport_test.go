package handler

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/pypx/goopy/model"
)

func stubFn(ret string) *model.Function {
	return &model.Function{Name: "stub", Returns: &model.TypeExpr{Raw: ret}}
}

func TestStubForFunction_LookupOrder(t *testing.T) {
	stubs := stubIndex{
		"pkg.published":  stubFn("published"),
		"pkg._x.defined": stubFn("origin"),
		"pkg._x.other":   stubFn("origin-other"),
		"pkg._x.old":     stubFn("origin-renamed"),
	}
	tests := []struct {
		name string
		fn   *model.Function
		want string
	}{
		{"published key wins", &model.Function{Name: "published", Origin: "pkg._x"}, "published"},
		{"origin key", &model.Function{Name: "defined", Origin: "pkg._x"}, "origin"},
		{"renamed uses OriginName", &model.Function{Name: "new", Origin: "pkg._x", OriginName: "old"}, "origin-renamed"},
		{"direct definition has no origin lookup", &model.Function{Name: "defined"}, ""},
		{"no stub", &model.Function{Name: "missing", Origin: "pkg._x"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stubForFunction(stubs, "pkg", tt.fn)
			switch {
			case tt.want == "" && got != nil:
				t.Errorf("got stub %+v, want none", got)
			case tt.want != "" && (got == nil || got.Returns.Raw != tt.want):
				t.Errorf("got %+v, want return %q", got, tt.want)
			}
		})
	}
}

func TestConvertClass_StubFallbackToOrigin(t *testing.T) {
	stubs := stubIndex{
		"pandas.core.frame.DataFrame.head": stubFn("DataFrame"),
		"pkg.New.direct":                   stubFn("direct"),
		"pkg._x.Old.renamed":               stubFn("renamed"),
	}
	cls := &model.Class{
		Name:    "DataFrame",
		Origin:  "pandas.core.frame",
		Methods: []*model.Function{{Name: "head", IsMethod: true}},
	}
	sym := convertClass(cls, stubs, "pandas")
	if sym.Origin != "pandas.core.frame" {
		t.Errorf("Origin = %q", sym.Origin)
	}
	if sym.Methods[0].Returns == nil || sym.Methods[0].Returns.Type != "DataFrame" {
		t.Errorf("method returns = %+v, want stub by origin key", sym.Methods[0].Returns)
	}

	renamed := &model.Class{
		Name: "New", Origin: "pkg._x", OriginName: "Old",
		Methods: []*model.Function{{Name: "direct"}, {Name: "renamed"}},
	}
	sym = convertClass(renamed, stubs, "pkg")
	if sym.Methods[0].Returns == nil || sym.Methods[0].Returns.Type != "direct" {
		t.Errorf("direct = %+v", sym.Methods[0].Returns)
	}
	if sym.Methods[1].Returns == nil || sym.Methods[1].Returns.Type != "renamed" {
		t.Errorf("renamed = %+v", sym.Methods[1].Returns)
	}
}

func TestBuildStubIndex_Aliases(t *testing.T) {
	head := &model.Function{Name: "head"}
	read := &model.Function{Name: "read"}
	pkg := &model.Package{Modules: []*model.Module{
		{Name: "pkg", Aliases: []*model.Alias{
			{Name: "DF", Target: "pkg.frame.DataFrame"},
			{Name: "read", Target: "pkg.io.read"},
			{Name: "gone", Target: "pkg.io.gone"},
		}},
		{Name: "pkg.frame", Classes: []*model.Class{{Name: "DataFrame", Methods: []*model.Function{head}}}},
		{Name: "pkg.io", Functions: []*model.Function{read}},
	}}
	idx := buildStubIndex(pkg)
	if idx["pkg.DF.head"] != head {
		t.Error("alias of a class must index its methods")
	}
	if idx["pkg.read"] != read {
		t.Error("alias of a function must index the function")
	}
	if _, ok := idx["pkg.gone"]; ok {
		t.Error("alias with no target must not be indexed")
	}
}

func TestConvertToDocsResponse_Aliases(t *testing.T) {
	pkg := &model.Package{Modules: []*model.Module{
		{Name: "pkg", Aliases: []*model.Alias{
			{Name: "fn", Target: "pkg.a.fn"},
			{Name: "Err", Target: "pkg.a.Err"},
			{Name: "CONST", Target: "pkg.a.CONST"},
			{Name: "Kind", Target: "pkg.a.Kind"},
			{Name: "ghost", Target: "pkg.a.ghost"},
		}},
		{Name: "pkg.a",
			Functions:   []*model.Function{{Name: "fn"}},
			Classes:     []*model.Class{{Name: "Err", BaseClasses: []*model.TypeRef{{Name: "Exception"}}}},
			Attributes:  []*model.Attribute{{Name: "CONST"}},
			TypeAliases: []*model.TypeAlias{{Name: "Kind"}},
		},
		{Name: "pkg.only", Aliases: []*model.Alias{{Name: "CONST", Target: "pkg.a.CONST"}}},
		{Name: "pkg.lost", Aliases: []*model.Alias{{Name: "ghost", Target: "pkg.a.ghost"}}},
	}}
	resp := convertToDocsResponse("pkg", "1", pkg, stubIndex{})
	if len(resp.Modules) != 3 || resp.Modules[2].Name != "pkg.only" {
		t.Fatalf("Modules = %+v, want pkg, pkg.a and pkg.only", resp.Modules)
	}
	want := []DocAlias{
		{Name: "fn", Target: "pkg.a.fn"},
		{Name: "Err", Target: "pkg.a.Err"},
		{Name: "CONST", Target: "pkg.a.CONST"},
		{Name: "Kind", Target: "pkg.a.Kind"},
	}
	if root := resp.Modules[0]; !slices.Equal(root.Aliases, want) {
		t.Errorf("root aliases = %+v, want %+v", root.Aliases, want)
	}
	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"aliases":[{"name":"fn","target":"pkg.a.fn"}`) {
		t.Errorf("aliases missing from JSON: %s", encoded)
	}
}

// A package that only re-exports attributes still has names to show.
func TestConvertToDocsResponse_OnlyAttributeAliasesAvailable(t *testing.T) {
	pkg := &model.Package{Modules: []*model.Module{
		{Name: "pkg", Aliases: []*model.Alias{{Name: "CONST", Target: "pkg.a.CONST"}}},
		{Name: "pkg.a", Attributes: []*model.Attribute{{Name: "CONST"}}},
	}}
	resp := convertToDocsResponse("pkg", "1", pkg, stubIndex{})
	if !resp.Available || len(resp.Modules) != 1 || resp.Modules[0].Name != "pkg" {
		t.Errorf("resp = %+v, want pkg available", resp)
	}
}

func TestConvertToDocsResponse_NoNamesNotAvailable(t *testing.T) {
	pkg := &model.Package{Modules: []*model.Module{
		{Name: "pkg", Aliases: []*model.Alias{{Name: "ghost", Target: "pkg.a.ghost"}}},
		{Name: "pkg.a", Attributes: []*model.Attribute{{Name: "CONST"}}},
	}}
	resp := convertToDocsResponse("pkg", "1", pkg, stubIndex{})
	if resp.Available || len(resp.Modules) != 0 {
		t.Errorf("resp = %+v, want not available", resp)
	}
}

func TestDocsInputFrom_CarriesOrigin(t *testing.T) {
	in := docsInputFrom(DocsResponse{Modules: []DocModule{{
		Name:      "pkg",
		Functions: []DocSymbol{{Name: "f", Origin: "pkg._x"}},
	}}})
	if got := in.Modules[0].Functions[0].Origin; got != "pkg._x" {
		t.Errorf("Origin = %q", got)
	}
}

func TestDocsInputFrom_CarriesAliases(t *testing.T) {
	in := docsInputFrom(DocsResponse{Modules: []DocModule{{
		Name:      "pkg",
		Functions: []DocSymbol{},
		Aliases:   []DocAlias{{Name: "a", Target: "pkg.b.a"}},
	}}})
	if len(in.Modules[0].Aliases) != 1 || in.Modules[0].Aliases[0].Target != "pkg.b.a" {
		t.Errorf("Aliases = %+v", in.Modules[0].Aliases)
	}
}
