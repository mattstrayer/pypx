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
