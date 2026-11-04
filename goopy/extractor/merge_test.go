package extractor

import (
	"testing"

	"github.com/pypx/goopy/model"
	"github.com/pypx/goopy/parser"
)

func parseModule(t *testing.T, src string) *model.Module {
	t.Helper()
	return New().ExtractModule("m", parser.New([]byte(src)).Parse())
}

func TestMergeStub_StubDecidesTypes(t *testing.T) {
	base := parseModule(t, "def f(a: int, b=1) -> int: ...\nX: int = 1\n")
	stub := parseModule(t, "def f(a: str, b: float = ...) -> str: ...\nX: str\n")
	MergeStub(base, stub)
	f := base.Functions[0]
	if f.Parameters[0].Type.Name != "str" || f.Parameters[1].Type.Name != "float" || f.Returns.Name != "str" {
		t.Errorf("f = %+v, want the stub's types", f)
	}
	if f.Parameters[1].Default != "1" {
		t.Errorf("b default = %q, want the .py value", f.Parameters[1].Default)
	}
	if a := base.Attributes[0]; a.Type.Name != "str" || a.Value != "1" {
		t.Errorf("X = %+v", a)
	}
}

func TestMergeStub_KindConflicts(t *testing.T) {
	base := parseModule(t, "class C:\n    x = 1\n    @property\n    def p(self): ...\n    def m(self): ...\nclass K: ...\ndef f(): ...\n")
	stub := parseModule(t, "class C:\n    def x(self) -> int: ...\n    def p(self) -> int: ...\n    m: int\nK: int\nclass f: ...\n")
	MergeStub(base, stub)
	if len(base.Functions) != 1 || len(base.Classes) != 2 || len(base.Attributes) != 0 {
		t.Errorf("module has %d functions, %d classes, %d attributes, want 1, 2, 0", len(base.Functions), len(base.Classes), len(base.Attributes))
	}
	c := base.Classes[0]
	names := map[string]int{}
	for _, m := range c.Methods {
		names[m.Name]++
	}
	if names["x"] != 1 || names["m"] != 1 || names["p"] != 0 {
		t.Errorf("methods = %v, want x and m", names)
	}
	if len(c.Attributes) != 1 || c.Attributes[0].Name != "p" {
		t.Errorf("attributes = %+v, want the property p", c.Attributes)
	}
}

func TestMergeStub_Exports(t *testing.T) {
	tests := []struct {
		name, base, stub string
		wantNames        []string
		wantDynamic      bool
		wantNil          bool
	}{
		{"neither", "", "", nil, false, true},
		{"readable .py wins", "__all__ = ['a']\n", "__all__ = ['b']\n", []string{"a"}, false, false},
		{"stub adds to no .py list", "", "__all__ = ['b']\n", []string{"b"}, true, false},
		{"stub adds to dynamic .py list", "__all__ = ['a'] + f()\n", "__all__ = ['b']\n", []string{"a", "b"}, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, stub := parseModule(t, tt.base), parseModule(t, tt.stub)
			MergeStub(base, stub)
			ex := base.Exports
			if tt.wantNil {
				if ex != nil {
					t.Errorf("Exports = %+v, want nil", ex)
				}
				return
			}
			if ex == nil || ex.Dynamic != tt.wantDynamic || len(ex.Names) != len(tt.wantNames) {
				t.Fatalf("Exports = %+v, want names %v, dynamic %v", ex, tt.wantNames, tt.wantDynamic)
			}
			for i, n := range tt.wantNames {
				if ex.Names[i] != n {
					t.Errorf("Names = %v, want %v", ex.Names, tt.wantNames)
				}
			}
		})
	}
}

func TestMergeStub_ImportedNamesWait(t *testing.T) {
	base := parseModule(t, "from .a import f\nfrom .b import *\n")
	stub := parseModule(t, "def f(n: int) -> int: ...\n")
	MergeStub(base, stub)
	if len(base.Functions) != 0 || len(base.StubOnly.Functions) != 1 {
		t.Errorf("Functions = %d, StubOnly = %d, want 0 and 1", len(base.Functions), len(base.StubOnly.Functions))
	}
}

func TestMarkStubImports(t *testing.T) {
	m := parseModule(t, `
from x import a
from x import b as b
from x import c as d
from x import *
from . import e
from . import f as g
import y
import z as z
`)
	MarkStubImports(m)
	want := []bool{true, false, true, false, false, true, true, false}
	if len(m.ImportFroms) != len(want) {
		t.Fatalf("%d imports, want %d", len(m.ImportFroms), len(want))
	}
	for i, imp := range m.ImportFroms {
		if imp.NotExported != want[i] {
			t.Errorf("import %d %+v: NotExported = %v, want %v", i, imp, imp.NotExported, want[i])
		}
	}
}
