package handler

import (
	"strings"
	"testing"

	"github.com/pypx/goopy/model"
)

func TestIsConstant(t *testing.T) {
	tests := []struct {
		name, typ, value string
		want             bool
	}{
		// ALL_CAPS names, whatever the value.
		{"DEFAULT_POOLSIZE", "", "10", true},
		{"MAX2", "int", "2", true},
		{"_PRIVATE_LISTED", "", "1", true},
		{"NO_VALUE", "int", "", true},
		{"LOGGER", "", "logging.getLogger(__name__)", true},
		{"T", "", "", true},

		// TypeAlias annotations, whatever the name.
		{"IncEx", "TypeAlias", "'set[int]'", true},
		{"lower_alias", "typing.TypeAlias", "int", true},
		{"te_alias", "te.TypeAlias", "int", true},
		{"quoted", `"TypeAlias"`, "int", true},

		// CamelCase names for a typing generic.
		{"ColorType", "", "Union[ColorTuple, str]", true},
		{"Handler", "", "t.Callable[[int], str]", true},
		{"Base64Bytes", "", "typing.Annotated[bytes, Encoder]", true},
		{"Mode", "", "Literal['r', 'w']", true},
		{"Headers", "", "dict[str, str]", true},
		{"Later", "", "collections.abc.Awaitable[int]", true},

		// CamelCase names for anything else.
		{"Client", "", "make_client()", false},
		{"Alias", "", "OtherClass", false},
		{"First", "", "ITEMS[0]", false},
		{"Pair", "", "(int, str)", false},
		{"Any", "", "", false},

		// Names that are neither.
		{"value", "", "[37] * 100", false},
		{"app", "", "typer.Typer()", false},
		{"lowercase_union", "", "Union[int, str]", false},
		{"T_route", "", "t.Callable[..., str]", false},

		// Dunders.
		{"__version__", "str", `"1.0"`, false},
		{"__ALL__", "", "[]", false},
		{"__TYPE__", "TypeAlias", "int", false},

		// Type parameters, even with a name or annotation that would pass.
		{"T", "", `TypeVar("T")`, false},
		{"T_CO", "", `t.TypeVar("T_CO", covariant=True)`, false},
		{"T_teardown", "", `t.TypeVar("T_teardown", bound=TeardownCallable)`, false},
		{"P", "", `typing.ParamSpec("P")`, false},
		{"TS", "", `TypeVarTuple("TS")`, false},
		{"USER_ID", "", `NewType("USER_ID", int)`, false},
		{"UserId", "TypeAlias", `NewType("UserId", int)`, false},
		{"Cb", "", `Callable[[int], str]`, true},
	}
	for _, tt := range tests {
		if got := isConstant(tt.name, tt.typ, tt.value); got != tt.want {
			t.Errorf("isConstant(%q, %q, %q) = %v, want %v", tt.name, tt.typ, tt.value, got, tt.want)
		}
	}
}

func TestFirstParagraph(t *testing.T) {
	tests := []struct{ name, text, want string }{
		{"empty", "", ""},
		{"one line", "Lexers for Spice.", "Lexers for Spice."},
		{"stops at a blank line", "First line\nsecond line.\n\nNext paragraph.", "First line\nsecond line."},
		{"blank line with spaces", "First.\n   \nNext.", "First."},
		{"windows line ends", "First\r\nsecond.\r\n\r\nNext.", "First\nsecond."},
		{"leading blank lines", "\n\nFirst.\n\nNext.", "First."},
		{"title with underline", "requests.adapters\n~~~~~~~~~~~~~~~~~\n\nThis module has adapters.\n\nMore.", "This module has adapters."},
		{"title with overline", "=====\nTitle\n=====\n\nBody.", "Body."},
		{"title then text with no blank line", "Title\n-----\nBody.\n\nMore.", "Body."},
		{"title only", "Title\n=====", ""},
		{"rule inside a paragraph stays", "One\ntwo\n---\nthree.", "One\ntwo\n---\nthree."},
		{"two dashes are not a rule", "--\nBody.", "--\nBody."},
		{"mixed punctuation is not a rule", "-=-\nBody.", "-=-\nBody."},
		{"directive", ".. currentmodule:: numpy.fft\n\nThe module.\n\nMore.", "The module."},
		{"directive with options", ".. dialect:: mssql\n    :name: SQL Server\n\nThe dialect.", "The dialect."},
		{"directive with body", ".. testsetup::\n\n    from a import b\n\n    c = 1\n\nThe module.", "The module."},
		{"two directives", ".. a::\n\n.. b::\n\nBody.", "Body."},
		{"directive only", ".. testsetup::\n\n    from a import b\n", ""},
		{"directive line inside a paragraph stays", "One.\n.. two::\n\nThree.", "One.\n.. two::"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstParagraph(tt.text); got != tt.want {
				t.Errorf("firstParagraph(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestConvertToDocsResponse_Constants(t *testing.T) {
	long := strings.Repeat("x", maxDefaultRunes+10)
	pkg := &model.Package{Modules: []*model.Module{{
		Name:      "pkg",
		Docstring: &model.Docstring{Text: "pkg\n===\n\nThe summary.\n\nThe details."},
		Functions: []*model.Function{{Name: "f"}},
		Attributes: []*model.Attribute{
			{Name: "LIMIT", Type: &model.TypeExpr{Raw: "int"}, Value: "10"},
			{Name: "LONG", Value: long},
			{Name: "Alias", Value: "Union[int, str]"},
			{Name: "__version__", Value: `"1"`},
			{Name: "T", Value: `TypeVar("T")`},
			{Name: "app", Value: "App()"},
		},
		TypeAliases: []*model.TypeAlias{
			{Name: "Point", Value: &model.TypeExpr{Raw: "tuple[int, int]"}},
			{Name: "lower"},
		},
	}}}
	resp := convertToDocsResponse("pkg", "1", pkg, stubIndex{})
	mod := resp.Modules[0]
	if mod.Docstring != "The summary." {
		t.Errorf("Docstring = %q", mod.Docstring)
	}
	names := []string{}
	for _, c := range mod.Constants {
		names = append(names, c.Name)
	}
	if got, want := strings.Join(names, " "), "LIMIT LONG Alias Point lower"; got != want {
		t.Fatalf("constants = %q, want %q", got, want)
	}
	if c := mod.Constants[0]; c.Type != "int" || c.Default != "10" || c.DefaultDigest != "" {
		t.Errorf("LIMIT = %+v", c)
	}
	if c := mod.Constants[1]; c.Default != truncateDefault(long) || c.DefaultDigest == "" {
		t.Errorf("LONG = %+v, want a cut value and a digest", c)
	}
	if c := mod.Constants[3]; c.Type != "TypeAlias" || c.Default != "tuple[int, int]" {
		t.Errorf("Point = %+v", c)
	}
	if c := mod.Constants[4]; c.Type != "TypeAlias" || c.Default != "" {
		t.Errorf("lower = %+v", c)
	}
}

func TestConvertToDocsResponse_OnlyConstantsAvailable(t *testing.T) {
	pkg := &model.Package{Modules: []*model.Module{
		{Name: "pkg.values", Attributes: []*model.Attribute{{Name: "LIMIT", Value: "10"}}},
		{Name: "pkg.types", TypeAliases: []*model.TypeAlias{{Name: "Point"}}},
	}}
	resp := convertToDocsResponse("pkg", "1", pkg, stubIndex{})
	if !resp.Available || len(resp.Modules) != 2 {
		t.Errorf("resp = %+v, want both modules available", resp)
	}
}

// A docstring is no reason to keep a module: the package would count as available
// with nothing to show.
func TestConvertToDocsResponse_DocstringOnlyModules(t *testing.T) {
	doc := func(text string) *model.Docstring { return &model.Docstring{Text: text} }

	t.Run("a top-level module stays and a submodule goes", func(t *testing.T) {
		resp := convertToDocsResponse("pkg", "1", &model.Package{Modules: []*model.Module{
			{Name: "pkg", Docstring: doc("About pkg.")},
			{Name: "pkg.core", Functions: []*model.Function{{Name: "f"}}},
			{Name: "pkg.notes", Docstring: doc("Only words.")},
		}}, stubIndex{})
		var names []string
		for _, m := range resp.Modules {
			names = append(names, m.Name)
		}
		if want := "pkg,pkg.core"; strings.Join(names, ",") != want {
			t.Errorf("modules = %v, want %s", names, want)
		}
		if !resp.Available {
			t.Error("Available = false, want true")
		}
	})

	t.Run("a summary alone does not make a package available", func(t *testing.T) {
		resp := convertToDocsResponse("pkg", "1", &model.Package{Modules: []*model.Module{
			{Name: "pkg", Docstring: doc("About pkg.")},
		}}, stubIndex{})
		if resp.Available {
			t.Error("Available = true, want false")
		}
		if len(resp.Modules) != 1 || resp.Modules[0].Docstring != "About pkg." {
			t.Errorf("modules = %+v", resp.Modules)
		}
	})
}

func TestFlattenSymbols_Constants(t *testing.T) {
	sig := func(c DocConstant) map[string]string {
		return flattenSymbols(DocsResponse{Modules: []DocModule{{Name: "m", Constants: []DocConstant{c}}}})
	}
	if got := sig(DocConstant{Name: "X", Type: "int", Default: "1"})["m.X"]; got != "X: int = 1" {
		t.Errorf("signature = %q", got)
	}
	// Two values that differ only past the cut have one Default and differ in digest.
	a := sig(DocConstant{Name: "X", Default: "cut…", DefaultDigest: "aa"})["m.X"]
	b := sig(DocConstant{Name: "X", Default: "cut…", DefaultDigest: "bb"})["m.X"]
	if a == b {
		t.Errorf("signatures match past the cut: %q", a)
	}
}

func TestFlattenSymbols_AliasOfConstantFollowsTarget(t *testing.T) {
	got := flattenSymbols(DocsResponse{Modules: []DocModule{
		{Name: "m", Aliases: []DocAlias{{Name: "X", Target: "m.a.X"}}},
		{Name: "m.a", Constants: []DocConstant{{Name: "X", Default: "1"}}},
	}})
	if got["m.X"] != "X = 1" || got["m.a.X"] != "X = 1" {
		t.Errorf("rows = %v", got)
	}
}

func TestFlattenSymbols_DefinitionBeatsConstant(t *testing.T) {
	got := flattenSymbols(DocsResponse{Modules: []DocModule{{
		Name:      "m",
		Functions: []DocSymbol{{Name: "X", Signature: "def X()"}},
		Constants: []DocConstant{{Name: "X", Default: "1"}},
	}}})
	if got["m.X"] != "def X()" {
		t.Errorf("m.X = %q", got["m.X"])
	}
}

func TestDocsInputFrom_CarriesConstantsAndDocstring(t *testing.T) {
	in := docsInputFrom(DocsResponse{Modules: []DocModule{{
		Name:      "m",
		Docstring: "Summary.",
		Constants: []DocConstant{{Name: "X", Type: "int", Default: "1"}},
	}}})
	mod := in.Modules[0]
	if mod.Docstring != "Summary." || len(mod.Constants) != 1 || mod.Constants[0].Signature() != "X: int = 1" {
		t.Errorf("module = %+v", mod)
	}
}
