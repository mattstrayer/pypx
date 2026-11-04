package goopy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/pypx/goopy/model"
)

func TestExtractPackageDeterministic(t *testing.T) {
	files := map[string][]byte{}
	for _, top := range []string{"alpha", "beta"} {
		files[top+"/__init__.py"] = []byte("def " + top + "_root(): pass\n")
		for i := 0; i < 19; i++ {
			sub := fmt.Sprintf("%s/sub%d", top, i%3)
			files[fmt.Sprintf("%s/m%02d.py", sub, i)] = []byte(fmt.Sprintf("def f%d(): pass\n", i))
		}
	}
	if len(files) != 40 {
		t.Fatalf("setup: %d files, want 40", len(files))
	}
	var first []byte
	for run := 0; run < 20; run++ {
		pkg := ExtractPackage(context.Background(), "test", files, []string{"beta", "alpha"})
		got, err := json.Marshal(pkg)
		if err != nil {
			t.Fatal(err)
		}
		if run == 0 {
			first = got
			continue
		}
		if !bytes.Equal(first, got) {
			t.Fatalf("run %d output differs from run 0", run)
		}
	}
}

func TestExtractPackage_DuplicateModuleNames(t *testing.T) {
	files := map[string][]byte{
		"foo/__init__.py": []byte("def from_pkg(): pass\n"),
		"foo.py":          []byte("def from_file(): pass\n"),
		"foo/a.py":        []byte("def a(): pass\n"),
	}
	for run := 0; run < 10; run++ {
		pkg := ExtractPackage(context.Background(), "foo", files, []string{"foo", "foo"})
		if len(pkg.Modules) != 2 {
			t.Fatalf("Modules len = %d, want 2", len(pkg.Modules))
		}
		if pkg.Modules[0].Name != "foo" || pkg.Modules[1].Name != "foo.a" {
			t.Fatalf("order = %q, %q", pkg.Modules[0].Name, pkg.Modules[1].Name)
		}
	}
}

func TestExtractPackageWorkerPanicKeepsOtherModules(t *testing.T) {
	orig := extractModuleFn
	defer func() { extractModuleFn = orig }()
	extractModuleFn = func(name string, src []byte) (*model.Module, []error) {
		if name == "pkg.m3" {
			panic("boom")
		}
		return orig(name, src)
	}
	files := map[string][]byte{}
	for i := 0; i < 12; i++ {
		files[fmt.Sprintf("pkg/m%d.py", i)] = []byte(fmt.Sprintf("def f%d(): pass\n", i))
	}
	pkg := ExtractPackage(context.Background(), "pkg", files, []string{"pkg"})
	if len(pkg.Modules) != 11 {
		t.Fatalf("Modules len = %d, want 11", len(pkg.Modules))
	}
}

func TestExtractPackageStatsCountParseErrors(t *testing.T) {
	files := map[string][]byte{
		"pkg/__init__.py": []byte("def ok(): pass\n"),
		"pkg/bad.py":      []byte("def good(): pass\ndef broken(:\n    pass\n"),
		"pkg/fine.py":     []byte("class C: pass\n"),
	}
	pkg := ExtractPackage(context.Background(), "pkg", files, []string{"pkg"})
	s := pkg.Stats
	if s.Files != 3 || s.Modules != 3 {
		t.Errorf("Files, Modules = %d, %d; want 3, 3", s.Files, s.Modules)
	}
	if s.FilesWithErrors != 1 {
		t.Errorf("FilesWithErrors = %d, want 1", s.FilesWithErrors)
	}
	if s.Errors < 1 {
		t.Errorf("Errors = %d, want at least 1", s.Errors)
	}
	if s.PanickedModules != 0 {
		t.Errorf("PanickedModules = %d, want 0", s.PanickedModules)
	}
	// The broken file still publishes what parsed before the error.
	var found bool
	for _, m := range pkg.Modules {
		if m.Name == "pkg.bad" && len(m.Functions) > 0 {
			found = true
		}
	}
	if !found {
		t.Error("pkg.bad lost its partial output")
	}
	if b, _ := json.Marshal(pkg); strings.Contains(string(b), "Stats") || strings.Contains(string(b), "Errors") {
		t.Errorf("Stats reached JSON: %s", b)
	}
}

func TestExtractPackageStatsCountPanics(t *testing.T) {
	orig := extractModuleFn
	defer func() { extractModuleFn = orig }()
	extractModuleFn = func(name string, src []byte) (*model.Module, []error) {
		if name == "pkg.m3" {
			panic("boom")
		}
		return orig(name, src)
	}
	files := map[string][]byte{}
	for i := 0; i < 12; i++ {
		files[fmt.Sprintf("pkg/m%d.py", i)] = []byte(fmt.Sprintf("def f%d(): pass\n", i))
	}
	pkg := ExtractPackage(context.Background(), "pkg", files, []string{"pkg"})
	if pkg.Stats.PanickedModules != 1 || pkg.Stats.Files != 12 {
		t.Errorf("Stats = %+v, want 1 panic over 12 files", pkg.Stats)
	}
}

// symbolNames lists every name a module publishes, for test comparisons.
func symbolNames(m *model.Module) []string {
	var out []string
	for _, f := range m.Functions {
		out = append(out, f.Name)
	}
	for _, c := range m.Classes {
		out = append(out, c.Name)
	}
	for _, a := range m.Attributes {
		out = append(out, a.Name)
	}
	for _, ta := range m.TypeAliases {
		out = append(out, ta.Name)
	}
	slices.Sort(out)
	return out
}

func findModule(t *testing.T, pkg *model.Package, name string) *model.Module {
	t.Helper()
	for _, m := range pkg.Modules {
		if m.Name == name {
			return m
		}
	}
	t.Fatalf("module %q not found", name)
	return nil
}

func TestExtractPackageReexports(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		tops  []string
		// module -> published names; every name there must match exactly.
		want map[string][]string
		// "module.name" -> Origin.
		origins map[string]string
		// "module.name" -> Target. Compared exactly across all modules.
		aliases map[string]string
	}{
		{
			name: "private definition is copied once, into the first export",
			files: map[string]string{
				"pkg/__init__.py": "from ._x import Y\nfrom ._x import Y as Z\n",
				"pkg/sub.py":      "from ._x import Y\n",
				"pkg/a.py":        "from ._x import Y as W\n",
				"pkg/_x.py":       "class Y: pass\n",
			},
			want:    map[string][]string{"pkg": {"Y"}, "pkg.a": nil, "pkg.sub": nil},
			origins: map[string]string{"pkg.Y": "pkg._x"},
			aliases: map[string]string{"pkg.Z": "pkg.Y", "pkg.a.W": "pkg.Y", "pkg.sub.Y": "pkg.Y"},
		},
		{
			name: "rename copied when no export keeps the name",
			files: map[string]string{
				"pkg/__init__.py": "from ._x import Y as Z\n",
				"pkg/sub.py":      "from ._x import Y\n",
				"pkg/_x.py":       "class Y: pass\n",
			},
			want:    map[string][]string{"pkg": {"Z"}, "pkg.sub": nil},
			origins: map[string]string{"pkg.Z": "pkg._x"},
			aliases: map[string]string{"pkg.sub.Y": "pkg.Z"},
		},
		{
			name: "copy goes to the shortest module path, not the first in sort order",
			files: map[string]string{
				"pkg/__init__.py":      "",
				"pkg/core/__init__.py": "",
				"pkg/core/records.py":  "from .._x import Y\n",
				"pkg/rec.py":           "from ._x import Y\n",
				"pkg/_x.py":            "class Y: pass\n",
			},
			want:    map[string][]string{"pkg.core.records": nil, "pkg.rec": {"Y"}},
			origins: map[string]string{"pkg.rec.Y": "pkg._x"},
			aliases: map[string]string{"pkg.core.records.Y": "pkg.rec.Y"},
		},
		{
			name: "modules of equal length fall back to sort order",
			files: map[string]string{
				"pkg/__init__.py": "",
				"pkg/zz.py":       "from ._x import Y\n",
				"pkg/aaaa.py":     "from ._x import Y\n",
				"pkg/_x.py":       "class Y: pass\n",
			},
			want:    map[string][]string{"pkg.aaaa": {"Y"}, "pkg.zz": nil},
			origins: map[string]string{"pkg.aaaa.Y": "pkg._x"},
			aliases: map[string]string{"pkg.zz.Y": "pkg.aaaa.Y"},
		},
		{
			name: "public definition stays in place",
			files: map[string]string{
				"pkg/__init__.py": "from .pub import a\n__all__ = [\"a\"]\n",
				"pkg/other.py":    "from .pub import a as b\n__all__ = [\"b\"]\n",
				"pkg/pub.py":      "def a(): pass\n",
			},
			want:    map[string][]string{"pkg": nil, "pkg.other": nil, "pkg.pub": {"a"}},
			aliases: map[string]string{"pkg.a": "pkg.pub.a", "pkg.other.b": "pkg.pub.a"},
		},
		{
			name: "definition hidden in a public module is copied",
			files: map[string]string{
				"pkg/__init__.py": "from .pub import _a, b\n__all__ = [\"_a\", \"b\"]\n",
				"pkg/pub.py":      "__all__ = [\"c\"]\ndef _a(): pass\ndef b(): pass\ndef c(): pass\n",
			},
			want:    map[string][]string{"pkg": {"_a", "b"}, "pkg.pub": {"c"}},
			origins: map[string]string{"pkg._a": "pkg.pub", "pkg.b": "pkg.pub"},
		},
		{
			name: "import wins over except fallback in a public module",
			files: map[string]string{
				"pkg/__init__.py": "try:\n    from ._x import Y\nexcept ImportError:\n    Y = None\n",
				"pkg/_x.py":       "class Y: pass\n",
			},
			want:    map[string][]string{"pkg": {"Y"}},
			origins: map[string]string{"pkg.Y": "pkg._x"},
		},
		{
			name: "import wins over else fallback with __all__",
			files: map[string]string{
				"pkg/__init__.py": "__all__ = [\"Y\"]\nif HAVE:\n    from ._x import Y\nelse:\n    Y = None\n",
				"pkg/_x.py":       "def Y(): pass\n",
			},
			want:    map[string][]string{"pkg": {"Y"}},
			origins: map[string]string{"pkg.Y": "pkg._x"},
		},
		{
			name: "fallback from a public module becomes an alias",
			files: map[string]string{
				"pkg/__init__.py": "try:\n    from .pub import Y\nexcept ImportError:\n    Y = None\n",
				"pkg/pub.py":      "class Y: pass\n",
			},
			want:    map[string][]string{"pkg": nil, "pkg.pub": {"Y"}},
			aliases: map[string]string{"pkg.Y": "pkg.pub.Y"},
		},
		{
			name: "fallback stays when the import leaves the wheel",
			files: map[string]string{
				"pkg/__init__.py": "try:\n    from other import Y\nexcept ImportError:\n    Y = None\n",
			},
			want: map[string][]string{"pkg": {"Y"}},
		},
		{
			name: "reassigned __all__ drops the old names",
			files: map[string]string{
				"pkg/__init__.py": "__all__ = [\"a\"]\n__all__ = [\"b\"]\nfrom ._x import a, b\n",
				"pkg/_x.py":       "def a(): pass\ndef b(): pass\n",
			},
			want: map[string][]string{"pkg": {"b"}},
		},
		{
			name: "relative",
			files: map[string]string{
				"pkg/__init__.py": "from ._x import Y\n",
				"pkg/_x.py":       "class Y: pass\n",
			},
			want:    map[string][]string{"pkg": {"Y"}},
			origins: map[string]string{"pkg.Y": "pkg._x"},
		},
		{
			name: "absolute",
			files: map[string]string{
				"pkg/__init__.py": "from pkg._x import Y\n",
				"pkg/_x.py":       "def Y(): pass\n",
			},
			want:    map[string][]string{"pkg": {"Y"}},
			origins: map[string]string{"pkg.Y": "pkg._x"},
		},
		{
			name: "alias",
			files: map[string]string{
				"pkg/__init__.py": "from ._x import foo as bar\n",
				"pkg/_x.py":       "def foo(): pass\n",
			},
			want:    map[string][]string{"pkg": {"bar"}},
			origins: map[string]string{"pkg.bar": "pkg._x"},
		},
		{
			name: "star with public names",
			files: map[string]string{
				"pkg/__init__.py": "from ._x import *\n",
				"pkg/_x.py":       "def a(): pass\ndef _b(): pass\nC = 1\n",
			},
			want:    map[string][]string{"pkg": {"C", "a"}},
			origins: map[string]string{"pkg.a": "pkg._x", "pkg.C": "pkg._x"},
		},
		{
			name: "star follows target __all__",
			files: map[string]string{
				"pkg/__init__.py": "from ._x import *\n",
				"pkg/_x.py":       "__all__ = [\"a\"]\ndef a(): pass\ndef b(): pass\n",
			},
			want: map[string][]string{"pkg": {"a"}},
		},
		{
			name: "chained",
			files: map[string]string{
				"pkg/__init__.py": "from ._a import Y as Z\n",
				"pkg/_a.py":       "from ._b import Y\n",
				"pkg/_b.py":       "from ._c import *\n",
				"pkg/_c.py":       "class Y: pass\n",
			},
			want:    map[string][]string{"pkg": {"Z"}},
			origins: map[string]string{"pkg.Z": "pkg._c"},
		},
		{
			name: "cycle",
			files: map[string]string{
				"pkg/__init__.py": "from ._a import Y, Z\n",
				"pkg/_a.py":       "from ._b import Y\nfrom ._b import *\nclass Z: pass\n",
				"pkg/_b.py":       "from ._a import Y\nfrom ._a import *\n",
			},
			want:    map[string][]string{"pkg": {"Z"}},
			origins: map[string]string{"pkg.Z": "pkg._a"},
		},
		{
			name: "__all__ only",
			files: map[string]string{
				"pkg/__init__.py": "from ._x import a, b\n__all__ = [\"a\"]\n",
				"pkg/_x.py":       "def a(): pass\ndef b(): pass\n",
			},
			want:    map[string][]string{"pkg": {"a"}},
			origins: map[string]string{"pkg.a": "pkg._x"},
		},
		{
			name: "__all__ lists a private name",
			files: map[string]string{
				"pkg/__init__.py": "from ._x import _a\n__all__ = [\"_a\"]\n",
				"pkg/_x.py":       "def _a(): pass\n",
			},
			want: map[string][]string{"pkg": {"_a"}},
		},
		{
			name: "private names stay hidden without __all__",
			files: map[string]string{
				"pkg/__init__.py": "from ._x import _a, b as _c\n",
				"pkg/_x.py":       "def _a(): pass\ndef b(): pass\n",
			},
			want: map[string][]string{"pkg": nil},
		},
		{
			name: "explicit import reaches name the target __all__ hides",
			files: map[string]string{
				"pkg/__init__.py": "from ._x import b\n",
				"pkg/_x.py":       "__all__ = [\"a\"]\ndef a(): pass\ndef b(): pass\n",
			},
			want: map[string][]string{"pkg": {"b"}},
		},
		{
			name: "__all__ aliases a public module",
			files: map[string]string{
				"pkg/__init__.py": "from .pub import a\n__all__ = [\"a\"]\n",
				"pkg/pub.py":      "def a(): pass\n",
			},
			want:    map[string][]string{"pkg": nil, "pkg.pub": {"a"}},
			origins: map[string]string{"pkg.pub.a": ""},
			aliases: map[string]string{"pkg.a": "pkg.pub.a"},
		},
		{
			name: "no __all__ does not copy from a public module",
			files: map[string]string{
				"pkg/__init__.py": "from .pub import a\nfrom .pub2 import *\n",
				"pkg/pub.py":      "def a(): pass\n",
				"pkg/pub2.py":     "def b(): pass\n",
			},
			want: map[string][]string{"pkg": nil, "pkg.pub": {"a"}, "pkg.pub2": {"b"}},
		},
		{
			name: "direct definition wins",
			files: map[string]string{
				"pkg/__init__.py": "from ._x import a\n__all__ = [\"a\"]\ndef a(): pass\n",
				"pkg/_x.py":       "def a(): pass\n",
			},
			want:    map[string][]string{"pkg": {"a"}},
			origins: map[string]string{"pkg.a": ""},
		},
		{
			name: "absolute import across top-level packages",
			files: map[string]string{
				"attrs/__init__.py": "from attr import define\nfrom attr._make import Factory\n__all__ = [\"define\", \"Factory\"]\n",
				"attr/__init__.py":  "from ._next_gen import define\n__all__ = [\"define\"]\n",
				"attr/_next_gen.py": "def define(): pass\n",
				"attr/_make.py":     "class Factory: pass\n",
			},
			tops: []string{"attr", "attrs"},
			want: map[string][]string{"attr": {"define"}, "attrs": {"Factory"}},
			origins: map[string]string{
				"attr.define":   "attr._next_gen",
				"attrs.Factory": "attr._make",
			},
			aliases: map[string]string{"attrs.define": "attr.define"},
		},
		{
			name: "import from another distribution finds nothing",
			files: map[string]string{
				"pkg/__init__.py": "from other import a\nfrom typing import Any\n__all__ = [\"a\", \"Any\"]\n",
			},
			want: map[string][]string{"pkg": nil},
		},
		{
			name: "parent relative import",
			files: map[string]string{
				"pkg/__init__.py":     "",
				"pkg/sub/__init__.py": "from .._x import a\nfrom ..sib import b\n__all__ = [\"a\", \"b\"]\n",
				"pkg/sub/mod.py":      "from .._x import a\nfrom ._y import c\n",
				"pkg/sub/_y.py":       "def c(): pass\n",
				"pkg/_x.py":           "def a(): pass\n",
				"pkg/sib.py":          "def b(): pass\n",
			},
			want: map[string][]string{"pkg.sub": {"a"}, "pkg.sub.mod": {"c"}, "pkg.sib": {"b"}},
			origins: map[string]string{
				"pkg.sub.a":     "pkg._x",
				"pkg.sub.mod.c": "pkg.sub._y",
			},
			aliases: map[string]string{"pkg.sub.b": "pkg.sib.b", "pkg.sub.mod.a": "pkg.sub.a"},
		},
		{
			name: "module __all__ references",
			files: map[string]string{
				"pkg/__init__.py": "from . import a\nfrom .a import *\nfrom ._b import *\nimport pkg.c as cc\nfrom .c import z\n__all__ = a.__all__ + _b.__all__ + cc.__all__\ndef hidden(): pass\n",
				"pkg/a.py":        "__all__ = [\"x\"]\ndef x(): pass\n",
				"pkg/_b.py":       "__all__ = [\"y\"]\ndef y(): pass\n",
				"pkg/c.py":        "__all__ = [\"z\"]\ndef z(): pass\n",
			},
			want:    map[string][]string{"pkg": {"y"}, "pkg.a": {"x"}, "pkg.c": {"z"}},
			aliases: map[string]string{"pkg.x": "pkg.a.x", "pkg.z": "pkg.c.z"},
		},
		{
			name: "__all__ += and local definition",
			files: map[string]string{
				"pkg/__init__.py": "__all__ = [\"a\"]\n__all__ += [\"b\"]\n__all__.append(\"_c\")\n__all__.extend([\"d\"])\nfrom ._x import a, _c\ndef b(): pass\ndef _c2(): pass\nclass d: pass\ndef not_listed(): pass\n",
				"pkg/_x.py":       "def a(): pass\ndef _c(): pass\n",
			},
			want: map[string][]string{"pkg": {"_c", "a", "b", "d"}},
		},
		{
			name: "unreadable __all__ keeps the underscore rule",
			files: map[string]string{
				"pkg/__init__.py": "__all__ = make() + [\"_a\"]\nfrom ._x import _a, b\ndef c(): pass\ndef _d(): pass\n",
				"pkg/_x.py":       "def _a(): pass\ndef b(): pass\n",
			},
			want: map[string][]string{"pkg": {"_a", "b", "c"}},
		},
		{
			name: "attribute and type alias",
			files: map[string]string{
				"pkg/__init__.py": "from ._x import A, T\n",
				"pkg/_x.py":       "A: int = 1\ntype T = int\n",
			},
			want:    map[string][]string{"pkg": {"A", "T"}},
			origins: map[string]string{"pkg.A": "pkg._x", "pkg.T": "pkg._x"},
		},
		{
			name: "import wins over a placeholder attribute in the source module",
			files: map[string]string{
				"pkg/__init__.py": "from ._x import Y\n",
				"pkg/_x.py":       "try:\n    from ._impl import Y\nexcept ImportError:\n    Y = None\n",
				"pkg/_impl.py":    "class Y: pass\n",
			},
			want:    map[string][]string{"pkg": {"Y"}},
			origins: map[string]string{"pkg.Y": "pkg._impl"},
		},
		{
			name: "relative import above the wheel finds nothing",
			files: map[string]string{
				"pkg/__init__.py": "from ...far import a\n__all__ = [\"a\"]\n",
				"pkg/_x.py":       "",
			},
			want: map[string][]string{"pkg": nil},
		},
		{
			name: "single-file distribution",
			files: map[string]string{
				"six.py": "from _six_impl import a\n__all__ = [\"a\"]\n",
			},
			want: map[string][]string{"six": nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := make(map[string][]byte, len(tt.files))
			for k, v := range tt.files {
				files[k] = []byte(v)
			}
			tops := tt.tops
			if tops == nil {
				tops = []string{"pkg"}
				if _, ok := files["six.py"]; ok {
					tops = []string{"six"}
				}
			}
			pkg := ExtractPackage(context.Background(), "test", files, tops)

			published := map[string]bool{}
			for _, m := range pkg.Modules {
				published[m.Name] = true
				if isPrivateModule(strings.ReplaceAll(m.Name, ".", "/") + ".py") {
					t.Errorf("private module %q was published", m.Name)
				}
				if m.ImportFroms != nil || m.Exports != nil || len(m.Hidden.Functions) != 0 {
					t.Errorf("module %q keeps resolver input", m.Name)
				}
			}
			for modName, wantNames := range tt.want {
				if wantNames == nil && !published[modName] {
					continue
				}
				m := findModule(t, pkg, modName)
				if got := symbolNames(m); !slices.Equal(got, wantNames) {
					t.Errorf("%s names = %q, want %q", modName, got, wantNames)
				}
			}
			for key, want := range tt.origins {
				i := strings.LastIndex(key, ".")
				if got, ok := symbolOrigin(findModule(t, pkg, key[:i]), key[i+1:]); !ok || got != want {
					t.Errorf("%s Origin = %q (found %v), want %q", key, got, ok, want)
				}
			}
			gotAliases := map[string]string{}
			for _, m := range pkg.Modules {
				for _, a := range m.Aliases {
					gotAliases[m.Name+"."+a.Name] = a.Target
				}
			}
			if len(gotAliases) != 0 || len(tt.aliases) != 0 {
				if !maps.Equal(gotAliases, tt.aliases) {
					t.Errorf("aliases = %v, want %v", gotAliases, tt.aliases)
				}
			}
		})
	}
}

func symbolOrigin(m *model.Module, name string) (string, bool) {
	for _, f := range m.Functions {
		if f.Name == name {
			return f.Origin, true
		}
	}
	for _, c := range m.Classes {
		if c.Name == name {
			return c.Origin, true
		}
	}
	for _, a := range m.Attributes {
		if a.Name == name {
			return a.Origin, true
		}
	}
	for _, ta := range m.TypeAliases {
		if ta.Name == name {
			return ta.Origin, true
		}
	}
	return "", false
}

// httpxShaped returns files shaped like httpx: an __init__.py with __all__,
// private implementation modules, and one public module.
func httpxShaped() map[string][]byte {
	files := map[string][]byte{
		"httpx/__init__.py": []byte(`"""HTTPX."""
from ._api import *
from ._client import AsyncClient, Client
from ._exceptions import HTTPError
from .sub import helper
__all__ = ["AsyncClient", "Client", "HTTPError", "get", "helper", "request"]
`),
		"httpx/_api.py": []byte(`__all__ = ["get", "request"]
def get(url: str) -> "Response":
    """Send GET."""
def request(method: str, url: str) -> "Response": pass
def not_exported(): pass
`),
		"httpx/_client.py": []byte(`class BaseClient: pass
class Client(BaseClient):
    """Sync client."""
    def get(self, url: str): pass
class AsyncClient(BaseClient): pass
`),
		"httpx/_exceptions.py": []byte("class HTTPError(Exception): pass\n"),
		"httpx/sub.py":         []byte("def helper(): pass\n"),
	}
	return files
}

func TestExtractPackage_HTTPXShaped(t *testing.T) {
	pkg := ExtractPackage(context.Background(), "httpx", httpxShaped(), []string{"httpx"})

	var names []string
	for _, m := range pkg.Modules {
		names = append(names, m.Name)
	}
	if !slices.Equal(names, []string{"httpx", "httpx.sub"}) {
		t.Fatalf("modules = %q", names)
	}
	root := pkg.Modules[0]
	want := []string{"AsyncClient", "Client", "HTTPError", "get", "request"}
	if got := symbolNames(root); !slices.Equal(got, want) {
		t.Errorf("root names = %q, want %q", got, want)
	}
	if len(root.Aliases) != 1 || *root.Aliases[0] != (model.Alias{Name: "helper", Target: "httpx.sub.helper"}) {
		t.Errorf("root aliases = %+v", root.Aliases)
	}
	for _, c := range root.Classes {
		if c.Name == "Client" {
			if c.Origin != "httpx._client" || len(c.Methods) != 1 || c.Docstring == nil {
				t.Errorf("Client = %+v", c)
			}
		}
	}
}

func TestExtractPackage_CopyDoesNotChangeSource(t *testing.T) {
	files := map[string][]byte{
		"pkg/__init__.py": []byte("from .pub import _a as b\n__all__ = [\"b\"]\n"),
		"pkg/pub.py":      []byte("def _a(): pass\ndef c(): pass\n"),
	}
	pkg := ExtractPackage(context.Background(), "pkg", files, []string{"pkg"})
	copied := findModule(t, pkg, "pkg").Functions[0]
	if copied.Name != "b" || copied.Origin != "pkg.pub" || copied.OriginName != "_a" {
		t.Errorf("copy = %+v", copied)
	}
}

func TestExtractPackage_OriginJSON(t *testing.T) {
	files := map[string][]byte{
		"pkg/__init__.py": []byte("from ._x import a\n__all__ = [\"a\", \"b\"]\ndef b(): pass\n"),
		"pkg/_x.py":       []byte("def a(): pass\n"),
	}
	pkg := ExtractPackage(context.Background(), "pkg", files, []string{"pkg"})
	got, err := json.Marshal(pkg)
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if !strings.Contains(s, `"origin":"pkg._x"`) {
		t.Errorf("origin missing from %s", s)
	}
	if strings.Count(s, `"origin"`) != 1 {
		t.Errorf("origin must be omitted on direct definitions: %s", s)
	}
	for _, field := range []string{"ImportFroms", "Hidden", "Exports", "OriginName"} {
		if strings.Contains(s, field) {
			t.Errorf("%s reached JSON: %s", field, s)
		}
	}
}

func TestExtractPackage_ParsesEachFileOnce(t *testing.T) {
	orig := extractModuleFn
	defer func() { extractModuleFn = orig }()

	for _, size := range []int{0, 8} { // Below and above parallelThreshold.
		files := httpxShaped()
		for i := range size {
			files[fmt.Sprintf("httpx/_extra%d.py", i)] = []byte("from ._api import *\nfrom ._client import *\n")
		}
		var mu sync.Mutex
		calls := map[string]int{}
		extractModuleFn = func(name string, src []byte) (*model.Module, []error) {
			mu.Lock()
			calls[name]++
			mu.Unlock()
			return orig(name, src)
		}
		ExtractPackage(context.Background(), "httpx", files, []string{"httpx"})
		if len(calls) != len(files) {
			t.Errorf("size %d: parsed %d modules, want %d", size, len(calls), len(files))
		}
		for name, n := range calls {
			if n != 1 {
				t.Errorf("size %d: %s parsed %d times", size, name, n)
			}
		}
	}
}

func TestExtractPackage_DepthCap(t *testing.T) {
	build := func(hops int) map[string][]byte {
		files := map[string][]byte{"pkg/__init__.py": []byte("from ._m0 import Y\n")}
		for i := 0; i < hops; i++ {
			files[fmt.Sprintf("pkg/_m%d.py", i)] = []byte(fmt.Sprintf("from ._m%d import Y\n", i+1))
		}
		files[fmt.Sprintf("pkg/_m%d.py", hops)] = []byte("class Y: pass\n")
		return files
	}
	near := ExtractPackage(context.Background(), "pkg", build(maxImportDepth-1), []string{"pkg"})
	if got := symbolNames(findModule(t, near, "pkg")); !slices.Equal(got, []string{"Y"}) {
		t.Errorf("within cap: %q", got)
	}
	far := ExtractPackage(context.Background(), "pkg", build(maxImportDepth+1), []string{"pkg"})
	if got := symbolNames(findModule(t, far, "pkg")); len(got) != 0 {
		t.Errorf("past cap: %q", got)
	}
}

func TestExtractPackage_ReexportsDeterministic(t *testing.T) {
	files := httpxShaped()
	for i := 0; i < 12; i++ {
		files[fmt.Sprintf("httpx/_m%d.py", i)] = []byte(fmt.Sprintf("from ._m%d import *\ndef f%d(): pass\n", (i+1)%12, i))
		files[fmt.Sprintf("httpx/pub%d.py", i)] = []byte(fmt.Sprintf("from ._m%d import *\n", i))
	}
	var first []byte
	for run := 0; run < 20; run++ {
		got, err := json.Marshal(ExtractPackage(context.Background(), "httpx", files, []string{"httpx"}))
		if err != nil {
			t.Fatal(err)
		}
		if run == 0 {
			first = got
		} else if !bytes.Equal(first, got) {
			t.Fatalf("run %d output differs from run 0", run)
		}
	}
}

func TestExtractModule(t *testing.T) {
	src := []byte("\"\"\"My module.\"\"\"\n\ndef hello(name: str) -> str:\n    \"\"\"Say hello.\n\n    Args:\n        name: The name to greet.\n\n    Returns:\n        str: A greeting string.\n    \"\"\"\n    return f\"Hello, {name}\"\n\nclass Greeter:\n    \"\"\"A greeter.\"\"\"\n\n    def __init__(self, prefix: str = \"Hello\"):\n        self.prefix = prefix\n\n    def greet(self, name: str) -> str:\n        \"\"\"Greet someone.\"\"\"\n        return f\"{self.prefix}, {name}\"\n\nclass BadInputError(ValueError):\n    \"\"\"Raised on bad input.\"\"\"\n    pass\n")

	mod, _ := ExtractModule("mymod", src)
	if mod.Name != "mymod" {
		t.Errorf("Name = %q", mod.Name)
	}
	if len(mod.Functions) != 1 {
		t.Fatalf("Functions len = %d, want 1", len(mod.Functions))
	}
	if mod.Functions[0].Name != "hello" {
		t.Errorf("Functions[0].Name = %q", mod.Functions[0].Name)
	}
	if len(mod.Classes) < 1 {
		t.Fatal("expected at least 1 class")
	}
	found := false
	for _, cls := range mod.Classes {
		if cls.Name == "Greeter" {
			found = true
		}
	}
	if !found {
		t.Error("Greeter class not found")
	}
}

func TestExtractModuleErrors(t *testing.T) {
	// Malformed source should not panic, should return partial results.
	src := []byte("def broken(:\n    pass\ndef good():\n    pass\n")
	mod, errs := ExtractModule("test", src)
	if mod == nil {
		t.Fatal("ExtractModule returned nil")
	}
	if len(errs) == 0 {
		t.Error("expected parse errors for malformed source")
	}
	found := false
	for _, fn := range mod.Functions {
		if fn.Name == "good" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'good' function in partial results")
	}
}

func TestPathToModuleName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"pkg/mod.py", "pkg.mod"},
		{"pkg/__init__.py", "pkg"},
		{"pkg/sub/deep.py", "pkg.sub.deep"},
	}
	for _, tt := range tests {
		got := pathToModuleName(tt.path)
		if got != tt.want {
			t.Errorf("pathToModuleName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestExtractPackage(t *testing.T) {
	files := map[string][]byte{
		"mypkg/__init__.py": []byte("\"\"\"My package.\"\"\"\ndef top_func(): pass\n"),
		"mypkg/sub.py":      []byte("def sub_func(): pass\n"),
		"other/foo.py":      []byte("def other_func(): pass\n"),
	}
	pkg := ExtractPackage(context.Background(), "test", files, []string{"mypkg"})
	if len(pkg.Modules) != 2 {
		t.Fatalf("Modules len = %d, want 2", len(pkg.Modules))
	}
}

func TestExtractPackage_SingleFileModule(t *testing.T) {
	files := map[string][]byte{
		"six.py": []byte("def add_metaclass(cls): pass\n"),
	}
	pkg := ExtractPackage(context.Background(), "six", files, []string{"six"})
	if len(pkg.Modules) != 1 {
		t.Fatalf("Modules len = %d, want 1", len(pkg.Modules))
	}
	if pkg.Modules[0].Name != "six" {
		t.Errorf("Modules[0].Name = %q, want %q", pkg.Modules[0].Name, "six")
	}
	if len(pkg.Modules[0].Functions) != 1 {
		t.Fatalf("Functions len = %d, want 1", len(pkg.Modules[0].Functions))
	}
	if pkg.Modules[0].Functions[0].Name != "add_metaclass" {
		t.Errorf("Functions[0].Name = %q, want %q", pkg.Modules[0].Functions[0].Name, "add_metaclass")
	}
}

func TestExtractPackage_SingleFileModule_DoubleCountGuard(t *testing.T) {
	// When both a directory package and a bare top-level file of the same
	// name exist, only the directory package should be extracted.
	files := map[string][]byte{
		"six/__init__.py": []byte("def add_metaclass(cls): pass\n"),
		"six.py":          []byte("def other_func(): pass\n"),
	}
	pkg := ExtractPackage(context.Background(), "six", files, []string{"six"})
	if len(pkg.Modules) != 1 {
		t.Fatalf("Modules len = %d, want 1", len(pkg.Modules))
	}
	if pkg.Modules[0].Name != "six" {
		t.Errorf("Modules[0].Name = %q, want %q", pkg.Modules[0].Name, "six")
	}
	if len(pkg.Modules[0].Functions) != 1 || pkg.Modules[0].Functions[0].Name != "add_metaclass" {
		t.Errorf("expected the directory package's add_metaclass, got %+v", pkg.Modules[0].Functions)
	}
}

func TestExtractPackage_DocstringOnlyModule(t *testing.T) {
	// A module with only a docstring (e.g., __init__.py) should not be filtered out.
	files := map[string][]byte{
		"mypkg/__init__.py": []byte(`"""My package docstring."""
`),
	}
	pkg := ExtractPackage(context.Background(), "mypkg", files, []string{"mypkg"})
	if len(pkg.Modules) != 1 {
		t.Fatalf("Modules len = %d, want 1 (docstring-only module should be included)", len(pkg.Modules))
	}
	if pkg.Modules[0].Docstring == nil {
		t.Error("expected module docstring to be set")
	}
}

func TestExtractPackage_CancelledContext(t *testing.T) {
	// Create enough files to trigger the goroutine pool path (>4 files)
	files := make(map[string][]byte)
	for i := 0; i < 10; i++ {
		path := fmt.Sprintf("mypkg/mod%d.py", i)
		files[path] = []byte(fmt.Sprintf(`
def func_%d():
    """Function %d"""
    pass
`, i, i))
	}

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Should return without panic or hang
	pkg := ExtractPackage(ctx, "mypkg", files, []string{"mypkg"})

	// Package should be non-nil (always returns a result)
	if pkg == nil {
		t.Fatal("expected non-nil package")
	}

	// With immediate cancellation, most/all modules should be skipped
	// (exact count depends on goroutine scheduling, so just verify it doesn't hang or panic)
	t.Logf("extracted %d modules with cancelled context", len(pkg.Modules))
}

func TestExtractPackage_SmallPackageCancelledContext(t *testing.T) {
	// <=4 files takes the non-goroutine path
	files := map[string][]byte{
		"mypkg/__init__.py": []byte(`"""Package docstring"""`),
		"mypkg/a.py":        []byte(`def a(): pass`),
		"mypkg/b.py":        []byte(`def b(): pass`),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	pkg := ExtractPackage(ctx, "mypkg", files, []string{"mypkg"})
	if pkg == nil {
		t.Fatal("expected non-nil package")
	}
	t.Logf("extracted %d modules with cancelled context (small package)", len(pkg.Modules))
}

func TestIsTestModule(t *testing.T) {
	cases := map[string]bool{
		"pandas/tests/plotting/frame/test_frame.py": true,
		"numpy/ma/tests/test_core.py":               true,
		"pkg/tests/__init__.py":                     true,
		"pkg/test/test_a.py":                        true,
		"pkg/test/helpers.py":                       true,
		"pkg/test/sub/deep.py":                      true,
		"pkg/test_utils.py":                         true,
		"pkg/utils_test.py":                         true,
		"pkg/conftest.py":                           true,
		"pkg/sub/conftest.py":                       true,
		"pkg/tests.py":                              true,
		"pytest/__init__.py":                        false,
		"pytest/main.py":                            false,
		"testing/__init__.py":                       false,
		"numpy/testing/__init__.py":                 false,
		"pkg/testing/utils.py":                      false,
		"pkg/core.py":                               false,
		"pkg/contest.py":                            false,
		"pkg/attest.py":                             false,
		"test/__init__.py":                          false,
		"django/test/__init__.py":                   false,
		"django/test/client.py":                     false,
		"werkzeug/test.py":                          false,
	}
	files := map[string][]byte{"pkg/test/test_a.py": nil}
	for path := range cases {
		files[path] = nil
	}
	// django/test has no test files, so it must stay public.
	for _, p := range []string{"django/test/__init__.py", "django/test/client.py"} {
		files[p] = nil
	}
	testDirs := testFileDirs(files)
	for path, want := range cases {
		if got := isTestModule(path, testDirs); got != want {
			t.Errorf("isTestModule(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestExtractPackage_SkipsTestModules(t *testing.T) {
	files := map[string][]byte{
		"pkg/__init__.py":        []byte("def a(): pass\n"),
		"pkg/tests/test_a.py":    []byte("def test_a(): pass\n"),
		"pkg/conftest.py":        []byte("def fix(): pass\n"),
		"pkg/testing/helpers.py": []byte("def helper(): pass\n"),
		"pkg/test/test_a.py":     []byte("def test_a(): pass\n"),
		"pkg/test/helpers.py":    []byte("def h(): pass\n"),
		"pkg/test.py":            []byte("def public(): pass\n"),
	}
	pkg := ExtractPackage(context.Background(), "pkg", files, []string{"pkg"})
	var names []string
	for _, m := range pkg.Modules {
		names = append(names, m.Name)
	}
	want := []string{"pkg", "pkg.test", "pkg.testing.helpers"}
	if !slices.Equal(names, want) {
		t.Errorf("modules = %v, want %v", names, want)
	}
}

func TestExtractPackage_KeepsPublicTestPackage(t *testing.T) {
	files := map[string][]byte{
		"django/__init__.py":      []byte("x = 1\n"),
		"django/test/__init__.py": []byte("from .client import Client\n"),
		"django/test/client.py":   []byte("class Client: pass\n"),
		"django/tests/test_x.py":  []byte("def test_x(): pass\n"),
	}
	pkg := ExtractPackage(context.Background(), "django", files, []string{"django"})
	var names []string
	for _, m := range pkg.Modules {
		names = append(names, m.Name)
	}
	want := []string{"django", "django.test", "django.test.client"}
	if !slices.Equal(names, want) {
		t.Errorf("modules = %v, want %v", names, want)
	}
}

func requireModules(t *testing.T, name, version string, want ...string) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	pkg, err := ExtractFromPyPI(context.Background(), name, version)
	if err != nil {
		t.Fatalf("ExtractFromPyPI: %v", err)
	}
	have := map[string]bool{}
	for _, m := range pkg.Modules {
		have[m.Name] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("%s %s: module %q missing (got %d modules)", name, version, w, len(pkg.Modules))
		}
	}
}

func TestExtractFromPyPI_Django(t *testing.T) {
	requireModules(t, "django", "6.1.3", "django.test", "django.test.client")
}

func TestExtractFromPyPI_Werkzeug(t *testing.T) {
	requireModules(t, "werkzeug", "3.1.8", "werkzeug.test")
}

func TestExtractFromPyPI_Setuptools(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	pkg, err := ExtractFromPyPI(context.Background(), "setuptools", "84.2.1")
	if err != nil {
		t.Fatalf("ExtractFromPyPI: %v", err)
	}
	if len(pkg.Modules) <= 50 {
		t.Errorf("setuptools modules = %d, want > 50", len(pkg.Modules))
	}
}

func TestExtractFromPyPI_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	pkg, err := ExtractFromPyPI(ctx, "click", "8.1.8")
	if err != nil {
		t.Fatalf("ExtractFromPyPI: %v", err)
	}
	if pkg.Name != "click" {
		t.Errorf("Name = %q, want %q", pkg.Name, "click")
	}
	if len(pkg.Modules) == 0 {
		t.Error("no modules extracted")
	}

	totalFuncs := 0
	for _, mod := range pkg.Modules {
		totalFuncs += len(mod.Functions)
	}
	if totalFuncs == 0 {
		t.Error("no functions found in click")
	}
}

// stubPackage extracts files under the single top-level package "pkg".
func stubPackage(t *testing.T, files map[string]string) map[string]*model.Module {
	t.Helper()
	in := make(map[string][]byte, len(files))
	for p, s := range files {
		in[p] = []byte(s)
	}
	mods := map[string]*model.Module{}
	for _, m := range ExtractPackage(context.Background(), "pkg", in, []string{"pkg"}).Modules {
		mods[m.Name] = m
	}
	return mods
}

func stubFunc(t *testing.T, m *model.Module, name string) *model.Function {
	t.Helper()
	if m == nil {
		t.Fatalf("module is missing, want function %s", name)
	}
	for _, f := range m.Functions {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("%s has no function %s", m.Name, name)
	return nil
}

func TestExtractPackage_MergesPyAndPyi(t *testing.T) {
	mods := stubPackage(t, map[string]string{
		"pkg/core.py": `
def get(url, timeout=None):
    """Fetch a URL."""

class Client:
    """A client."""
    def __init__(self, base):
        self.base = base
    def send(self, body):
        """Send a body."""
`,
		"pkg/core.pyi": `
def get(url: str, timeout: float | None = ..., extra: int = ...) -> bytes:
    """Stub text."""

def only_in_stub(n: int) -> int: ...

class Client:
    base: str
    def __init__(self, base: str) -> None: ...
    def send(self, body: bytes) -> int: ...
    def close(self) -> None: ...

class StubClass: ...
`,
	})
	m := mods["pkg.core"]
	get := stubFunc(t, m, "get")
	if get.Docstring == nil || get.Docstring.Text != "Fetch a URL." {
		t.Errorf("get docstring = %+v, want the .py text", get.Docstring)
	}
	if len(get.Parameters) != 2 {
		t.Errorf("get has %d parameters, want the 2 in the .py", len(get.Parameters))
	}
	if get.Parameters[0].Type == nil || get.Parameters[0].Type.Name != "str" {
		t.Errorf("url type = %+v, want str", get.Parameters[0].Type)
	}
	if get.Parameters[1].Default != "None" {
		t.Errorf("timeout default = %q, want the .py value", get.Parameters[1].Default)
	}
	if get.Returns == nil || get.Returns.Name != "bytes" {
		t.Errorf("get returns = %+v, want bytes", get.Returns)
	}
	if got := stubFunc(t, m, "only_in_stub"); got.Returns == nil {
		t.Error("only_in_stub has no return type")
	}

	names := map[string]*model.Class{}
	for _, c := range m.Classes {
		names[c.Name] = c
	}
	if len(m.Classes) != 2 || names["StubClass"] == nil {
		t.Fatalf("classes = %v, want Client and StubClass", names)
	}
	cl := names["Client"]
	if cl.Docstring == nil || cl.Docstring.Text != "A client." {
		t.Errorf("Client docstring = %+v", cl.Docstring)
	}
	if cl.Constructor == nil || cl.Constructor.Parameters[0].Type == nil {
		t.Errorf("constructor = %+v, want a typed parameter", cl.Constructor)
	}
	methods := map[string]*model.Function{}
	for _, f := range cl.Methods {
		methods[f.Name] = f
	}
	if methods["close"] == nil {
		t.Error("close from the stub is missing")
	}
	if s := methods["send"]; s == nil || s.Parameters[0].Type == nil || s.Returns == nil || s.Docstring == nil {
		t.Errorf("send = %+v, want types from the stub and the .py docstring", s)
	}
	if len(cl.Attributes) != 1 || cl.Attributes[0].Type == nil || cl.Attributes[0].Value != "base" {
		t.Errorf("attributes = %+v, want base with the stub's type and the .py's value", cl.Attributes)
	}
}

func TestExtractPackage_StubOnlyModule(t *testing.T) {
	// orjson's shape: the .py imports from a compiled module, the .pyi holds the API.
	mods := stubPackage(t, map[string]string{
		"pkg/__init__.py": `
from .pkg import *
from .pkg import __version__

__all__ = ("dumps", "loads", "OPT_X", "Fragment", "__version__")
`,
		"pkg/__init__.pyi": `
from typing import Any

__version__: str

def dumps(obj: Any, /, option: int | None = None) -> bytes: ...
def loads(obj: bytes) -> Any: ...
def hidden_helper() -> None: ...

class Fragment:
    contents: bytes

OPT_X: int
`,
	})
	m := mods["pkg"]
	if d := stubFunc(t, m, "dumps"); d.Returns == nil || d.Returns.Name != "bytes" {
		t.Errorf("dumps returns = %+v", d.Returns)
	}
	stubFunc(t, m, "loads")
	for _, f := range m.Functions {
		if f.Name == "hidden_helper" {
			t.Error("hidden_helper is not in the .py's __all__")
		}
	}
	if len(m.Classes) != 1 || m.Classes[0].Name != "Fragment" {
		t.Errorf("classes = %+v", m.Classes)
	}

	// A bare stub with no .py at all.
	mods = stubPackage(t, map[string]string{
		"pkg/__init__.pyi": "def run(n: int) -> int: ...\n",
		"pkg/sub.pyi":      "def go() -> None: ...\n",
	})
	stubFunc(t, mods["pkg"], "run")
	stubFunc(t, mods["pkg.sub"], "go")
}

func TestExtractPackage_StubOverloads(t *testing.T) {
	mods := stubPackage(t, map[string]string{
		"pkg/mixed.py": `
def f(a, b=None):
    """Doc."""
`,
		"pkg/mixed.pyi": `
from typing import overload

@overload
def f(a: int) -> int: ...
@overload
def f(a: str, b: int) -> str: ...
`,
		"pkg/only.pyi": `
from typing import overload

@overload
def g(a: int) -> int: ...
@overload
def g(a: str) -> str: ...

class C:
    @overload
    def m(self, a: int) -> int: ...
    @overload
    def m(self, a: str) -> str: ...
`,
	})
	f := stubFunc(t, mods["pkg.mixed"], "f")
	if len(f.Overloads) != 2 {
		t.Errorf("f has %d overloads, want 2", len(f.Overloads))
	}
	if f.Parameters[0].Type != nil || f.Returns != nil {
		t.Errorf("f takes no single signature from its overloads: %+v, %+v", f.Parameters[0].Type, f.Returns)
	}
	if f.Docstring == nil {
		t.Error("f lost its docstring")
	}
	g := stubFunc(t, mods["pkg.only"], "g")
	if len(mods["pkg.only"].Functions) != 1 || len(g.Overloads) != 1 {
		t.Errorf("g: %d functions and %d overloads, want 1 and 1", len(mods["pkg.only"].Functions), len(g.Overloads))
	}
	if m := mods["pkg.only"].Classes[0].Methods; len(m) != 1 || len(m[0].Overloads) != 1 {
		t.Errorf("C methods = %+v", m)
	}
}

func TestExtractPackage_StubFillsImportedDefinition(t *testing.T) {
	// attrs' shape: the .pyi defines what the .py imports from a private .py.
	mods := stubPackage(t, map[string]string{
		"pkg/__init__.py":  "from ._impl import run\n",
		"pkg/__init__.pyi": "def run(n: int) -> int: ...\n",
		"pkg/_impl.py":     "def run(n):\n    \"\"\"Run it.\"\"\"\n",
	})
	run := stubFunc(t, mods["pkg"], "run")
	if run.Docstring == nil || run.Docstring.Text != "Run it." {
		t.Errorf("run docstring = %+v, want the .py text", run.Docstring)
	}
	if run.Returns == nil || run.Parameters[0].Type == nil {
		t.Errorf("run = %+v, want the stub's types", run)
	}
	if run.Origin != "pkg._impl" {
		t.Errorf("run origin = %q, want pkg._impl", run.Origin)
	}
}

func TestExtractPackage_StubInPrivateCompiledModule(t *testing.T) {
	// pandas' shape: public names come through private modules with only a .pyi,
	// and "try/except" fallbacks do not hide them.
	mods := stubPackage(t, map[string]string{
		"pkg/__init__.py":                "from pkg.core.api import Stamp, NA\n__all__ = ['Stamp', 'NA']\n",
		"pkg/core/__init__.py":           "",
		"pkg/core/api.py":                "from pkg._libs import NA, Stamp\n",
		"pkg/_libs/__init__.py":          "from pkg._libs.tslibs import Stamp\nfrom pkg._libs.missing import NA\n",
		"pkg/_libs/missing.pyi":          "class NAType: ...\nNA: NAType\n",
		"pkg/_libs/tslibs/__init__.py":   "from .stamps import Stamp\n__all__ = ['Stamp']\n",
		"pkg/_libs/tslibs/stamps.pyi":    "class Stamp:\n    def now(self) -> Stamp: ...\n",
		"pkg/_libs/tslibs/unrelated.pyi": "class Other: ...\n",
	})
	published := func(mod, name string) bool {
		m := mods[mod]
		if m == nil {
			return false
		}
		return slices.ContainsFunc(m.Classes, func(c *model.Class) bool { return c.Name == name }) ||
			slices.ContainsFunc(m.Attributes, func(a *model.Attribute) bool { return a.Name == name }) ||
			slices.ContainsFunc(m.Aliases, func(a *model.Alias) bool { return a.Name == name })
	}
	for _, name := range []string{"Stamp", "NA"} {
		if !published("pkg", name) {
			t.Errorf("pkg does not publish %s", name)
		}
	}
	for mod := range mods {
		if strings.Contains(mod, "_libs") {
			t.Errorf("private module %s is published", mod)
		}
	}
}

func TestExtractPackage_StubImportsDoNotReexport(t *testing.T) {
	mods := stubPackage(t, map[string]string{
		"pkg/__init__.pyi": `
from ._types import Hidden
from ._types import Shown as Shown
from ._star import *
from . import sub
`,
		"pkg/_types.pyi": "class Hidden: ...\nclass Shown: ...\n",
		"pkg/_star.pyi":  "class Starred: ...\n",
		"pkg/sub.pyi":    "def go() -> None: ...\n",
	})
	m := mods["pkg"]
	got := map[string]bool{}
	for _, c := range m.Classes {
		got[c.Name] = true
	}
	if !got["Shown"] || !got["Starred"] || got["Hidden"] {
		t.Errorf("classes = %v, want Shown and Starred only", got)
	}
}

func TestExtractPackage_StubAllLists(t *testing.T) {
	// The .py has no readable __all__, so the stub's list adds names without
	// taking any away.
	mods := stubPackage(t, map[string]string{
		"pkg/__init__.py":  "import os\n__all__ = sorted(os.listdir('.'))\nfrom ._a import Listed\n\ndef from_py(): ...\n",
		"pkg/__init__.pyi": "from ._a import Listed\n__all__ = ['Listed', 'from_py']\n",
		"pkg/_a.py":        "class Listed: ...\n",
	})
	m := mods["pkg"]
	stubFunc(t, m, "from_py")
	if !slices.ContainsFunc(m.Classes, func(c *model.Class) bool { return c.Name == "Listed" }) {
		t.Errorf("classes = %+v, want Listed", m.Classes)
	}
}

func TestExtractPackage_StubReplacesPlainAttribute(t *testing.T) {
	mods := stubPackage(t, map[string]string{
		"pkg/mod.py":  "make = build()\nLIMIT = 5\n",
		"pkg/mod.pyi": "def make(n: int) -> int: ...\nLIMIT: int\n",
	})
	m := mods["pkg.mod"]
	stubFunc(t, m, "make")
	if len(m.Attributes) != 1 || m.Attributes[0].Name != "LIMIT" || m.Attributes[0].Value != "5" || m.Attributes[0].Type == nil {
		t.Errorf("attributes = %+v, want typed LIMIT with its value", m.Attributes)
	}
}

func TestExtractPackage_StubDirectory(t *testing.T) {
	mods := stubPackage(t, map[string]string{
		"pkg/__init__.py":        "def f(a): ...\n",
		"pkg/__init__.pyi":       "def f(a: bytes) -> bytes: ...\n",
		"pkg-stubs/__init__.pyi": "def f(a: int) -> int: ...\n",
		"pkg-stubs/extra.pyi":    "def e() -> None: ...\n",
	})
	if f := stubFunc(t, mods["pkg"], "f"); f.Returns == nil || f.Returns.Name != "int" {
		t.Errorf("f returns = %+v, want the stub directory's int", f.Returns)
	}
	stubFunc(t, mods["pkg.extra"], "e")
}

func TestExtractPackage_StubRulesForTestsAndPrivacy(t *testing.T) {
	mods := stubPackage(t, map[string]string{
		"pkg/__init__.pyi":     "def f() -> None: ...\n",
		"pkg/tests/data.pyi":   "def t() -> None: ...\n",
		"pkg/test_thing.pyi":   "def t() -> None: ...\n",
		"pkg/conftest.pyi":     "def t() -> None: ...\n",
		"pkg/_private.pyi":     "def p() -> None: ...\n",
		"pkg/sub/__init__.pyi": "from ._inner import go as go\n",
		"pkg/sub/_inner.pyi":   "def go() -> None: ...\n",
	})
	want := []string{"pkg", "pkg.sub"}
	got := slices.Sorted(maps.Keys(mods))
	if !slices.Equal(got, want) {
		t.Errorf("modules = %v, want %v", got, want)
	}
	// A relative import in a package's __init__.pyi must find the sibling.
	if go_ := stubFunc(t, mods["pkg.sub"], "go"); go_.Origin != "pkg.sub._inner" {
		t.Errorf("go origin = %q, want pkg.sub._inner", go_.Origin)
	}
}

func TestExtractPackage_MergedFilesParseOnceEach(t *testing.T) {
	orig := extractModuleFn
	defer func() { extractModuleFn = orig }()

	for _, size := range []int{0, 8} { // Below and above parallelThreshold.
		files := map[string][]byte{
			"pkg/__init__.py":  []byte("from .a import f\n"),
			"pkg/__init__.pyi": []byte("def f(n: int) -> int: ...\n"),
			"pkg/a.py":         []byte("def f(n): ...\n"),
			"pkg/a.pyi":        []byte("def f(n: int) -> int: ...\n"),
			"pkg/only.pyi":     []byte("def g() -> None: ...\n"),
		}
		for i := range size {
			files[fmt.Sprintf("pkg/m%d.py", i)] = []byte("x = 1\n")
		}
		var mu sync.Mutex
		calls := map[string]int{}
		extractModuleFn = func(name string, src []byte) (*model.Module, []error) {
			mu.Lock()
			calls[string(src)]++
			mu.Unlock()
			return orig(name, src)
		}
		ExtractPackage(context.Background(), "pkg", files, []string{"pkg"})
		var total int
		for _, n := range calls {
			total += n
		}
		if total != len(files) {
			t.Errorf("size %d: %d parses, want one per file (%d)", size, total, len(files))
		}
		if len(calls) == 0 || calls["def g() -> None: ...\n"] != 1 {
			t.Errorf("size %d: stub-only file parsed %d times", size, calls["def g() -> None: ...\n"])
		}
	}
}

func TestExtractPackage_StubPanicKeepsSource(t *testing.T) {
	orig := extractModuleFn
	defer func() { extractModuleFn = orig }()
	extractModuleFn = func(name string, src []byte) (*model.Module, []error) {
		if strings.Contains(string(src), "boom") {
			panic("bad stub")
		}
		return orig(name, src)
	}
	mods := stubPackage(t, map[string]string{
		"pkg/a.py":  "def f(): ...\n",
		"pkg/a.pyi": "boom: int\n",
		"pkg/b.py":  "boom = 1\n",
		"pkg/b.pyi": "def g() -> None: ...\n",
	})
	stubFunc(t, mods["pkg.a"], "f")
	stubFunc(t, mods["pkg.b"], "g")
}

func TestExtractPackage_StubOutputIsStable(t *testing.T) {
	files := map[string][]byte{}
	for i := range 12 {
		files[fmt.Sprintf("pkg/m%d.py", i)] = fmt.Appendf(nil, "from ._p%d import *\ndef a(x): ...\n", i)
		files[fmt.Sprintf("pkg/m%d.pyi", i)] = fmt.Appendf(nil, "def a(x: int) -> int: ...\ndef b(y: str) -> str: ...\nclass C%d: ...\nZ: int\n", i)
		files[fmt.Sprintf("pkg/_p%d.py", i)] = fmt.Appendf(nil, "def b(y):\n    \"\"\"From %d.\"\"\"\n", i)
	}
	var first []byte
	for i := range 20 {
		data, err := json.Marshal(ExtractPackage(context.Background(), "pkg", files, []string{"pkg"}))
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = data
		} else if !bytes.Equal(first, data) {
			t.Fatalf("run %d differs from run 0", i)
		}
	}
}

func TestExtractPackage_StubTypeParamsAreNotAPI(t *testing.T) {
	attrNames := func(m *model.Module) []string {
		var names []string
		for _, a := range m.Attributes {
			names = append(names, a.Name)
		}
		return names
	}
	stub := "from typing import ParamSpec, TypeVar, TypeVarTuple\nT = TypeVar('T')\nP = ParamSpec('P')\nTs = TypeVarTuple('Ts')\nLIMIT: int\ndef f(x: T) -> T: ...\n"
	tests := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"stub only", map[string]string{"pkg/mod.pyi": stub}, []string{"LIMIT"}},
		{"with a .py", map[string]string{"pkg/mod.py": "def f(x): ...\n", "pkg/mod.pyi": stub}, []string{"LIMIT"}},
		{"star import in the .py", map[string]string{"pkg/mod.py": "from ._c import *\n", "pkg/mod.pyi": stub}, []string{"LIMIT"}},
		{"dynamic __all__ in the .py", map[string]string{"pkg/mod.py": "__all__ = f()\n", "pkg/mod.pyi": stub}, []string{"LIMIT"}},
		{"readable __all__ in the .py", map[string]string{"pkg/mod.py": "__all__ = ['T', 'f']\n", "pkg/mod.pyi": stub}, []string{"T"}},
		{"readable __all__ in the stub", map[string]string{"pkg/mod.pyi": stub + "__all__ = ['P', 'f']\n"}, []string{"P"}},
		{"stub __all__ with a dynamic .py list", map[string]string{"pkg/mod.py": "__all__ = f()\n", "pkg/mod.pyi": stub + "__all__ = ['Ts']\n"}, []string{"LIMIT", "Ts"}},
		{"star import of a private stub", map[string]string{"pkg/mod.pyi": "from ._t import *\n", "pkg/_t.pyi": stub}, []string{"LIMIT"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := attrNames(stubPackage(t, tt.files)["pkg.mod"])
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("attributes = %v, want %v", got, tt.want)
			}
		})
	}
}
