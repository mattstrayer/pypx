package extractor

import (
	"slices"
	"testing"

	"github.com/pypx/goopy/model"
)

func TestMergeFunction_Decorators(t *testing.T) {
	tests := []struct {
		name string
		base string
		stub string
		want []string
	}{
		{
			name: "stub only",
			base: "def f(a): ...\n",
			stub: "@final\n@deprecated('old')\ndef f(a): ...\n",
			want: []string{"final", "deprecated('old')"},
		},
		{
			name: "source first, then stub",
			base: "@lru_cache\ndef f(a): ...\n",
			stub: "@final\ndef f(a): ...\n",
			want: []string{"lru_cache", "final"},
		},
		{
			name: "same name once, source form stays",
			base: "@typing.final\n@deprecated('source')\ndef f(a): ...\n",
			stub: "@final\n@deprecated('stub')\n@override\ndef f(a): ...\n",
			want: []string{"typing.final", "deprecated('source')", "override"},
		},
		{
			name: "no decorators",
			base: "def f(a): ...\n",
			stub: "def f(a): ...\n",
			want: nil,
		},
		{
			name: "overload is not a decorator, and one overload's deprecation stays with it",
			base: "def f(a): ...\n",
			stub: "@overload\n@final\n@deprecated('x')\ndef f(a: int) -> int: ...\n@overload\ndef f(a: str) -> str: ...\n",
			want: []string{"final"},
		},
		{
			name: "override on the first overload applies to the function",
			base: "def f(a): ...\n",
			stub: "@overload\n@override\ndef f(a: int) -> int: ...\n@overload\ndef f(a: str) -> str: ...\n",
			want: []string{"override"},
		},
		{
			name: "a decorator on every overload applies to the function",
			base: "def f(a): ...\n",
			stub: "@overload\n@deprecated('x')\ndef f(a: int) -> int: ...\n@typing.overload\n@deprecated('y')\ndef f(a: str) -> str: ...\n",
			want: []string{"deprecated('x')"},
		},
		{
			name: "a stub implementation after overloads decides",
			base: "def f(a): ...\n",
			stub: "@overload\n@deprecated('x')\ndef f(a: int) -> int: ...\n@final\ndef f(a): ...\n",
			want: []string{"final"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := parseModule(t, tt.base)
			MergeStub(base, parseModule(t, tt.stub))
			if got := base.Functions[0].Decorators; !slices.Equal(got, tt.want) {
				t.Errorf("Decorators = %q, want %q", got, tt.want)
			}
		})
	}
}

// The overloads still reach the function when their decorators are merged.
func TestMergeFunction_DecoratorsKeepOverloads(t *testing.T) {
	base := parseModule(t, "def f(a): ...\n")
	MergeStub(base, parseModule(t, "@overload\n@final\ndef f(a: int) -> int: ...\n@overload\ndef f(a: str) -> str: ...\n"))
	f := base.Functions[0]
	if len(f.Overloads) != 2 {
		t.Fatalf("Overloads = %+v, want 2", f.Overloads)
	}
	if f.Parameters[0].Type != nil {
		t.Errorf("a = %+v, an overload says nothing about the implementation", f.Parameters[0])
	}
}

func TestMergeClass_Decorators(t *testing.T) {
	base := parseModule(t, "@dataclass\nclass C:\n    def m(self): ...\n    @classmethod\n    def n(cls): ...\n    def __init__(self): ...\n    class Inner: ...\n")
	stub := parseModule(t, `@final
@dataclasses.dataclass(frozen=True)
class C:
    @final
    def m(self): ...
    @classmethod
    def n(cls): ...
    @deprecated('old')
    def __init__(self): ...
    @final
    class Inner: ...
`)
	MergeStub(base, stub)
	c := base.Classes[0]
	if want := []string{"dataclass", "final"}; !slices.Equal(c.Decorators, want) {
		t.Errorf("class Decorators = %q, want %q", c.Decorators, want)
	}
	if want := []string{"final"}; !slices.Equal(c.Methods[0].Decorators, want) {
		t.Errorf("m Decorators = %q, want %q", c.Methods[0].Decorators, want)
	}
	if want := []string{"classmethod"}; !slices.Equal(c.Methods[1].Decorators, want) {
		t.Errorf("n Decorators = %q, want %q", c.Methods[1].Decorators, want)
	}
	if want := []string{"deprecated('old')"}; !slices.Equal(c.Constructor.Decorators, want) {
		t.Errorf("constructor Decorators = %q, want %q", c.Constructor.Decorators, want)
	}
	if want := []string{"final"}; !slices.Equal(c.Classes[0].Decorators, want) {
		t.Errorf("Inner Decorators = %q, want %q", c.Classes[0].Decorators, want)
	}
}

// A merge must not write through the stub's slice, which other aliases can share.
func TestAddDecorators_DoesNotAlias(t *testing.T) {
	decs := make([]string, 1, 4)
	decs[0] = "a"
	got := addDecorators(decs, []string{"b"})
	decs = append(decs, "c")
	if got[1] != "b" || decs[1] != "c" {
		t.Errorf("got %q and %q share an array", got, decs)
	}
}

func TestFunctionDecorators_NoOverloadAndNilStub(t *testing.T) {
	fn := &model.Function{Decorators: []string{"typing.overload", "final", "deprecated('x')"}, Overloads: []*model.Function{{Decorators: []string{"overload"}}}}
	if got, want := FunctionDecorators(fn, nil), []string{"final"}; !slices.Equal(got, want) {
		t.Errorf("FunctionDecorators = %q, want %q", got, want)
	}
	plain := &model.Function{Decorators: []string{"a"}}
	stub := &model.Function{Decorators: []string{"b", "x.a"}}
	if got, want := FunctionDecorators(plain, stub), []string{"a", "b"}; !slices.Equal(got, want) {
		t.Errorf("FunctionDecorators = %q, want %q", got, want)
	}
}

func TestClassDecorators(t *testing.T) {
	cls := &model.Class{Decorators: []string{"a"}}
	if got, want := ClassDecorators(cls, nil), []string{"a"}; !slices.Equal(got, want) {
		t.Errorf("no stub: %q", got)
	}
	if got, want := ClassDecorators(cls, &model.Class{Decorators: []string{"b"}}), []string{"a", "b"}; !slices.Equal(got, want) {
		t.Errorf("stub: %q", got)
	}
}
