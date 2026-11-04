package goopy

import (
	"context"
	"strings"
	"testing"

	"github.com/pypx/goopy/model"
)

// ctorOf returns "a, *b" for the constructor of mod.name, or "<none>".
func ctorOf(t *testing.T, pkg *model.Package, mod, name string) string {
	t.Helper()
	for _, m := range pkg.Modules {
		if m.Name != mod {
			continue
		}
		for _, c := range m.Classes {
			if c.Name != name {
				continue
			}
			if c.Constructor == nil {
				return "<none>"
			}
			var parts []string
			for _, p := range c.Constructor.Parameters {
				s := p.Name
				if p.Kind == model.ParamKeywordOnly {
					s = "kw:" + s
				}
				parts = append(parts, s)
			}
			return strings.Join(parts, ", ")
		}
	}
	t.Fatalf("class %s.%s not found", mod, name)
	return ""
}

func TestInheritedConstructors(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  map[string]string // "module:Class" -> parameters
	}{
		{
			name: "same module, through a class with none",
			files: map[string]string{"pkg/__init__.py": `
class A:
    def __init__(self, x, y=1): pass
class B(A): pass
class C(B): pass
`},
			want: map[string]string{"pkg:B": "x, y", "pkg:C": "x, y"},
		},
		{
			name: "nearest constructor wins",
			files: map[string]string{"pkg/__init__.py": `
class A:
    def __init__(self, x): pass
class B(A):
    def __init__(self, y): pass
class C(B): pass
`},
			want: map[string]string{"pkg:C": "y"},
		},
		{
			name: "across private modules",
			files: map[string]string{
				"pkg/__init__.py": "from ._b import B\nfrom ._c import C\n__all__ = ['B', 'C']\n",
				"pkg/_a.py":       "class A:\n    def __init__(self, x): pass\n",
				"pkg/_b.py":       "from ._a import A\nclass B(A): pass\n",
				"pkg/_c.py":       "from . import _a\nclass C(_a.A): pass\n",
			},
			want: map[string]string{"pkg:B": "x", "pkg:C": "x"},
		},
		{
			name: "generic base and inert bases",
			files: map[string]string{"pkg/__init__.py": `
from abc import ABC
from typing import Generic, TypeVar
T = TypeVar("T")
class A(Generic[T]):
    def __init__(self, x): pass
class B(A[int], ABC): pass
`},
			want: map[string]string{"pkg:B": "x"},
		},
		{
			name: "cycle in bases",
			files: map[string]string{
				"pkg/__init__.py": "from .a import A\nclass B(A): pass\n",
				"pkg/a.py":        "from . import B\nclass A(B): pass\n",
			},
			want: map[string]string{"pkg:B": "<none>", "pkg.a:A": "<none>"},
		},
		{
			name:  "class that is its own base",
			files: map[string]string{"pkg/__init__.py": "class A(A): pass\n"},
			want:  map[string]string{"pkg:A": "<none>"},
		},
		{
			name: "base outside the wheel",
			files: map[string]string{"pkg/__init__.py": `
from other import Base
class A(Base): pass
class B(ValueError): pass
class C(A): pass
`},
			want: map[string]string{"pkg:A": "<none>", "pkg:B": "<none>", "pkg:C": "<none>"},
		},
		{
			name: "multiple inheritance is not guessed",
			files: map[string]string{"pkg/__init__.py": `
class A:
    def __init__(self, x): pass
class B:
    def __init__(self, y): pass
class C(A, B): pass
class D(C): pass
`},
			want: map[string]string{"pkg:C": "<none>", "pkg:D": "<none>"},
		},
		{
			name: "dataclass subclass puts base fields first",
			files: map[string]string{"pkg/__init__.py": `
from dataclasses import dataclass
@dataclass
class A:
    x: int
    y: int = 1
@dataclass
class B(A):
    z: int = 2
class C(B): pass
@dataclass
class D(C):
    w: int = 3
`},
			want: map[string]string{
				"pkg:A": "x, y", "pkg:B": "x, y, z", "pkg:C": "x, y, z", "pkg:D": "x, y, z, w",
			},
		},
		{
			name: "dataclass override keeps the position of the base field",
			files: map[string]string{"pkg/__init__.py": `
from dataclasses import dataclass, field
@dataclass
class A:
    x: int
    y: int = 1
    hidden: int = field(default=0, init=False)
@dataclass
class B(A):
    z: int = 0
    x: str = ""
    hidden: int = 0
`},
			want: map[string]string{"pkg:B": "x, y, hidden, z"},
		},
		{
			name: "keyword-only fields come last",
			files: map[string]string{"pkg/__init__.py": `
from dataclasses import dataclass, KW_ONLY
@dataclass(kw_only=True)
class A:
    x: int = 0
@dataclass
class B(A):
    y: int = 0
    _: KW_ONLY
    z: int = 0
`},
			want: map[string]string{"pkg:B": "y, kw:x, kw:z"},
		},
		{
			name: "dataclass with an unknown base has no constructor",
			files: map[string]string{"pkg/__init__.py": `
from dataclasses import dataclass
from other import Base
@dataclass
class A(Base):
    x: int
`},
			want: map[string]string{"pkg:A": "<none>"},
		},
		{
			name: "plain base adds no dataclass fields",
			files: map[string]string{"pkg/__init__.py": `
from dataclasses import dataclass
class A:
    def __init__(self, ignored): pass
@dataclass
class B(A):
    x: int
`},
			want: map[string]string{"pkg:B": "x"},
		},
		{
			name: "init=False generates nothing, but its fields still count",
			files: map[string]string{"pkg/__init__.py": `
from dataclasses import dataclass
class A:
    def __init__(self, a): pass
@dataclass(init=False)
class B(A):
    x: int = 0
@dataclass
class C(B):
    y: int = 0
`},
			want: map[string]string{"pkg:B": "a", "pkg:C": "x, y"},
		},
		{
			name: "explicit __init__ on a dataclass does not hide its fields",
			files: map[string]string{"pkg/__init__.py": `
from dataclasses import dataclass
@dataclass
class A:
    x: int
    def __init__(self, other): pass
class B(A): pass
@dataclass
class C(A):
    y: int = 0
`},
			want: map[string]string{"pkg:A": "other", "pkg:B": "other", "pkg:C": "x, y"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := map[string][]byte{}
			for path, src := range tt.files {
				files[path] = []byte(src)
			}
			pkg := ExtractPackage(context.Background(), "pkg", files, []string{"pkg"})
			for key, want := range tt.want {
				mod, name, _ := strings.Cut(key, ":")
				if got := ctorOf(t, pkg, mod, name); got != want {
					t.Errorf("%s: got %q, want %q", key, got, want)
				}
			}
		})
	}
}

// These two shapes come from scipy.special. Python binds the names when it runs
// the modules, and a static reader has no definition to publish. Listing a name
// in __all__ must not invent one, and must not lose the names around it.
func TestRuntimeBoundNamesHaveNoEntry(t *testing.T) {
	files := map[string][]byte{
		"pkg/__init__.py": []byte(`
from . import _basic
from ._basic import *
from ._backends import *
__all__ = _basic.__all__
__all__ += ["multigammaln", "kept"]
from ._kept import kept
`),
		"pkg/_basic.py": []byte(`
from numpy import sinc
__all__ = ["sinc", "own"]
def own(): pass
`),
		"pkg/_stats.py": []byte(`
__all__ = ["multigammaln"]
def multigammaln(a, d): pass
`),
		"pkg/_backends.py": []byte(`
from . import _stats
_funcs = (_stats.multigammaln,)
globals().update({f.__name__: f for f in _funcs})
__all__ = [f.__name__ for f in _funcs]
`),
		"pkg/_kept.py": []byte("def kept(): pass\n"),
	}
	pkg := ExtractPackage(context.Background(), "pkg", files, []string{"pkg"})
	got := map[string]bool{}
	for _, f := range pkg.Modules[0].Functions {
		got[f.Name] = true
	}
	for _, name := range []string{"own", "kept"} {
		if !got[name] {
			t.Errorf("%s is missing: %v", name, got)
		}
	}
	for _, name := range []string{"multigammaln", "sinc"} {
		if got[name] {
			t.Errorf("%s has an entry, but no definition is reachable", name)
		}
	}
}

// A subclass shares no parameters with its base, so its class docstring cannot
// change the base's documentation.
func TestInheritedConstructorKeepsBaseDocs(t *testing.T) {
	files := map[string][]byte{"pkg/__init__.py": []byte(`
class A:
    def __init__(self, x, y):
        """Build.

        Args:
            x: base x.
        """
class B(A):
    """Sub.

    Args:
        x: sub x.
        y: sub y.
    """
`)}
	pkg := ExtractPackage(context.Background(), "pkg", files, []string{"pkg"})
	var a, b *model.Class
	for _, c := range pkg.Modules[0].Classes {
		switch c.Name {
		case "A":
			a = c
		case "B":
			b = c
		}
	}
	if a.Constructor.Parameters[1].DocParam != nil {
		t.Errorf("base parameter y got documentation from the subclass: %+v", a.Constructor.Parameters[1].DocParam)
	}
	if got := b.Constructor.Parameters[0].DocParam.Description; got != "base x." {
		t.Errorf("x = %q, want the __init__ docstring to win", got)
	}
	if got := b.Constructor.Parameters[1].DocParam.Description; got != "sub y." {
		t.Errorf("y = %q, want the class docstring to fill the gap", got)
	}
}

// A dataclass field keeps the description from the class that declares it.
func TestInheritedDataclassFieldDocs(t *testing.T) {
	files := map[string][]byte{"pkg/__init__.py": []byte(`
from dataclasses import dataclass
@dataclass
class A:
    """Base.

    Args:
        x: base x.
    """
    x: int
@dataclass
class B(A):
    y: int = 0
`)}
	pkg := ExtractPackage(context.Background(), "pkg", files, []string{"pkg"})
	for _, c := range pkg.Modules[0].Classes {
		if c.Name == "B" {
			if p := c.Constructor.Parameters[0]; p.DocParam == nil || p.DocParam.Description != "base x." {
				t.Errorf("x = %+v, want the base's description", p.DocParam)
			}
		}
	}
}
