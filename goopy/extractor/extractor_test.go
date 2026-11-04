package extractor

import (
	"slices"
	"testing"

	"github.com/pypx/goopy/model"
	"github.com/pypx/goopy/parser"
)

func extract(src string) *model.Module {
	p := parser.New([]byte(src))
	mod := p.Parse()
	e := New()
	return e.ExtractModule("testmod", mod)
}

func TestExtractSimpleFunction(t *testing.T) {
	m := extract(`
def greet(name: str, times: int = 1) -> str:
    """Say hello.

    Args:
        name: The person's name.
        times: How many times to greet.

    Returns:
        The greeting string.
    """
    return "hello " * times + name
`)

	if len(m.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(m.Functions))
	}
	fn := m.Functions[0]
	if fn.Name != "greet" {
		t.Errorf("expected name 'greet', got %q", fn.Name)
	}
	if fn.Returns == nil || fn.Returns.Kind != model.TypeExprName || fn.Returns.Name != "str" {
		t.Errorf("expected return type str, got %+v", fn.Returns)
	}
	if len(fn.Parameters) != 2 {
		t.Fatalf("expected 2 params, got %d", len(fn.Parameters))
	}
	if fn.Parameters[0].Name != "name" {
		t.Errorf("expected first param 'name', got %q", fn.Parameters[0].Name)
	}
	if fn.Parameters[0].Type == nil || fn.Parameters[0].Type.Name != "str" {
		t.Errorf("expected first param type str, got %+v", fn.Parameters[0].Type)
	}
	if fn.Parameters[1].Default != "1" {
		t.Errorf("expected second param default '1', got %q", fn.Parameters[1].Default)
	}
	if fn.Docstring == nil || fn.Docstring.Text == "" {
		t.Error("expected docstring to be parsed")
	}
	// Check that doc params are attached to parameters.
	if fn.Parameters[0].DocParam == nil {
		t.Error("expected doc param for 'name'")
	} else if fn.Parameters[0].DocParam.Description != "The person's name." {
		t.Errorf("expected doc param description, got %q", fn.Parameters[0].DocParam.Description)
	}
}

func TestExtractClass(t *testing.T) {
	m := extract(`
class Dog:
    """A good dog."""

    breed: str

    def __init__(self, name: str, age: int = 0):
        self.name = name
        self.age = age

    def bark(self) -> str:
        """Make noise."""
        return "woof"
`)

	if len(m.Classes) != 1 {
		t.Fatalf("expected 1 class, got %d", len(m.Classes))
	}
	cls := m.Classes[0]
	if cls.Name != "Dog" {
		t.Errorf("expected 'Dog', got %q", cls.Name)
	}
	if cls.Docstring == nil || cls.Docstring.Text == "" {
		t.Error("expected docstring")
	}

	// Class has no Parameters field - __init__ params are not on the class.
	// Attributes should include class-level and __init__ self.x assignments.
	if len(cls.Attributes) < 2 {
		t.Errorf("expected at least 2 attributes (breed, name, age), got %d", len(cls.Attributes))
	}
}

func TestExtractClassMethods(t *testing.T) {
	m := extract(`
class Foo:
    def __init__(self):
        pass

    def public_method(self):
        pass

    def _private_method(self):
        pass

    def __repr__(self) -> str:
        return "Foo()"
`)

	cls := m.Classes[0]

	// Should include public_method and __repr__, but NOT __init__ or _private_method.
	methodNames := make(map[string]bool)
	for _, method := range cls.Methods {
		methodNames[method.Name] = true
	}
	if !methodNames["public_method"] {
		t.Error("expected public_method in methods")
	}
	if !methodNames["__repr__"] {
		t.Error("expected __repr__ in methods")
	}
	if methodNames["__init__"] {
		t.Error("__init__ should NOT be in methods")
	}
	if methodNames["_private_method"] {
		t.Error("_private_method should NOT be in methods")
	}
}

func TestExtractNestedClass(t *testing.T) {
	m := extract(`
class Outer:
    """Outer class."""

    class Inner:
        """Inner class."""

        def m(self) -> int:
            return 1

    class _Hidden:
        pass

    def method(self):
        pass
`)

	if len(m.Classes) != 1 {
		t.Fatalf("expected 1 module-level class, got %d", len(m.Classes))
	}
	outer := m.Classes[0]
	if outer.Name != "Outer" {
		t.Errorf("expected 'Outer', got %q", outer.Name)
	}

	if len(outer.Classes) != 1 {
		t.Fatalf("expected 1 nested class (Inner, _Hidden skipped), got %d", len(outer.Classes))
	}
	inner := outer.Classes[0]
	if inner.Name != "Inner" {
		t.Errorf("expected nested class 'Inner', got %q", inner.Name)
	}
	if inner.Docstring == nil || inner.Docstring.Text == "" {
		t.Error("expected docstring on Inner")
	}
	if len(inner.Methods) != 1 || inner.Methods[0].Name != "m" {
		t.Errorf("expected Inner to have method 'm', got %+v", inner.Methods)
	}

	methodNames := make(map[string]bool)
	for _, method := range outer.Methods {
		methodNames[method.Name] = true
	}
	if !methodNames["method"] {
		t.Error("expected Outer to still have 'method'")
	}
}

func TestExtractDoublyNestedClass(t *testing.T) {
	m := extract(`
class A:
    class B:
        class C:
            pass
`)

	if len(m.Classes) != 1 || m.Classes[0].Name != "A" {
		t.Fatalf("expected module-level class A, got %+v", m.Classes)
	}
	a := m.Classes[0]
	if len(a.Classes) != 1 || a.Classes[0].Name != "B" {
		t.Fatalf("expected A to contain nested class B, got %+v", a.Classes)
	}
	b := a.Classes[0]
	if len(b.Classes) != 1 || b.Classes[0].Name != "C" {
		t.Fatalf("expected B to contain nested class C, got %+v", b.Classes)
	}
}

func TestExtractException(t *testing.T) {
	m := extract(`
class MyError(ValueError):
    """A custom error."""
    pass
`)

	// Exceptions are still Class in our model (no separate type).
	if len(m.Classes) != 1 {
		t.Fatalf("expected 1 class, got %d", len(m.Classes))
	}
	cls := m.Classes[0]
	if cls.Name != "MyError" {
		t.Errorf("expected 'MyError', got %q", cls.Name)
	}
	if len(cls.BaseClasses) != 1 || cls.BaseClasses[0].Name != "ValueError" {
		t.Errorf("expected base class ValueError, got %+v", cls.BaseClasses)
	}
}

func TestSkipPrivate(t *testing.T) {
	m := extract(`
def public_func():
    pass

def _private_func():
    pass

def __dunder_func():
    pass
`)

	names := make(map[string]bool)
	for _, fn := range m.Functions {
		names[fn.Name] = true
	}
	if !names["public_func"] {
		t.Error("expected public_func")
	}
	if names["_private_func"] {
		t.Error("_private_func should be skipped")
	}
	if names["__dunder_func"] {
		t.Error("__dunder_func should be skipped")
	}
}

func TestExtractDecorators(t *testing.T) {
	m := extract(`
@staticmethod
def helper() -> None:
    pass

@some_module.decorator
def fancy():
    pass
`)

	if len(m.Functions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(m.Functions))
	}
	if len(m.Functions[0].Decorators) != 1 || m.Functions[0].Decorators[0] != "staticmethod" {
		t.Errorf("expected decorator 'staticmethod', got %v", m.Functions[0].Decorators)
	}
	if len(m.Functions[1].Decorators) != 1 || m.Functions[1].Decorators[0] != "some_module.decorator" {
		t.Errorf("expected decorator 'some_module.decorator', got %v", m.Functions[1].Decorators)
	}
}

func TestExtractProperty(t *testing.T) {
	m := extract(`
class Circle:
    def __init__(self, radius: float):
        self.radius = radius

    @property
    def area(self) -> float:
        """The area of the circle."""
        return 3.14159 * self.radius ** 2
`)

	cls := m.Classes[0]

	// @property methods should become Attributes with IsProperty=true.
	var propAttr *model.Attribute
	for _, attr := range cls.Attributes {
		if attr.Name == "area" {
			propAttr = attr
			break
		}
	}
	if propAttr == nil {
		t.Fatal("expected 'area' as an attribute")
	}
	if !propAttr.IsProperty {
		t.Error("expected IsProperty=true")
	}
	if propAttr.Type == nil || propAttr.Type.Name != "float" {
		t.Errorf("expected type float, got %+v", propAttr.Type)
	}
	if propAttr.Docstring == nil {
		t.Error("expected docstring on property")
	}

	// Should NOT appear in methods.
	for _, method := range cls.Methods {
		if method.Name == "area" {
			t.Error("area should NOT be in methods")
		}
	}
}

func TestExtractAllExports(t *testing.T) {
	m := extract(`
__all__ = ["public_one", "PublicClass"]

def public_one():
    pass

def public_two():
    pass

def _private():
    pass

class PublicClass:
    pass

class HiddenClass:
    pass
`)

	// Only public_one and PublicClass should be extracted.
	fnNames := make(map[string]bool)
	for _, fn := range m.Functions {
		fnNames[fn.Name] = true
	}
	clsNames := make(map[string]bool)
	for _, cls := range m.Classes {
		clsNames[cls.Name] = true
	}

	if !fnNames["public_one"] {
		t.Error("expected public_one")
	}
	if fnNames["public_two"] {
		t.Error("public_two should be hidden (not in __all__)")
	}
	if fnNames["_private"] {
		t.Error("_private should be hidden")
	}
	if !clsNames["PublicClass"] {
		t.Error("expected PublicClass")
	}
	if clsNames["HiddenClass"] {
		t.Error("HiddenClass should be hidden")
	}
}

func TestExtractAsyncFunction(t *testing.T) {
	m := extract(`
async def fetch(url: str) -> bytes:
    """Fetch a URL."""
    pass
`)

	if len(m.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(m.Functions))
	}
	fn := m.Functions[0]
	if !fn.IsAsync {
		t.Error("expected IsAsync=true")
	}
	if fn.Name != "fetch" {
		t.Errorf("expected name 'fetch', got %q", fn.Name)
	}
}

func TestExtractDataclassAttributes(t *testing.T) {
	m := extract(`
class Point:
    x: float
    y: float
    label: str = "origin"
`)

	cls := m.Classes[0]
	if len(cls.Attributes) < 3 {
		t.Fatalf("expected at least 3 attributes, got %d", len(cls.Attributes))
	}

	attrMap := make(map[string]*model.Attribute)
	for _, a := range cls.Attributes {
		attrMap[a.Name] = a
	}

	if a, ok := attrMap["x"]; !ok {
		t.Error("expected attribute 'x'")
	} else if a.Type == nil || a.Type.Name != "float" {
		t.Errorf("expected type float for x, got %+v", a.Type)
	}

	if a, ok := attrMap["label"]; !ok {
		t.Error("expected attribute 'label'")
	} else if a.Value != `"origin"` {
		t.Errorf("expected value '\"origin\"' for label, got %q", a.Value)
	}
}

func TestTypeExprSimple(t *testing.T) {
	m := extract(`
def f(x: str) -> int:
    pass
`)
	fn := m.Functions[0]
	if fn.Parameters[0].Type.Kind != model.TypeExprName {
		t.Errorf("expected TypeExprName, got %s", fn.Parameters[0].Type.Kind)
	}
	if fn.Parameters[0].Type.Name != "str" {
		t.Errorf("expected 'str', got %q", fn.Parameters[0].Type.Name)
	}
	if fn.Returns.Kind != model.TypeExprName || fn.Returns.Name != "int" {
		t.Errorf("expected return TypeExprName int, got %+v", fn.Returns)
	}
}

func TestTypeExprGeneric(t *testing.T) {
	m := extract(`
def f(x: list[int]) -> dict[str, int]:
    pass
`)
	fn := m.Functions[0]
	pType := fn.Parameters[0].Type
	if pType.Kind != model.TypeExprGeneric {
		t.Errorf("expected TypeExprGeneric, got %s", pType.Kind)
	}
	if pType.Name != "list" {
		t.Errorf("expected 'list', got %q", pType.Name)
	}
	if len(pType.Args) != 1 || pType.Args[0].Name != "int" {
		t.Errorf("expected args [int], got %+v", pType.Args)
	}

	rType := fn.Returns
	if rType.Kind != model.TypeExprGeneric || rType.Name != "dict" {
		t.Errorf("expected dict generic, got %+v", rType)
	}
	if len(rType.Args) != 2 {
		t.Errorf("expected 2 args for dict, got %d", len(rType.Args))
	}
}

func TestTypeExprUnion(t *testing.T) {
	m := extract(`
def f(x: int | str) -> None:
    pass
`)
	fn := m.Functions[0]
	pType := fn.Parameters[0].Type
	if pType.Kind != model.TypeExprUnion {
		t.Errorf("expected TypeExprUnion, got %s", pType.Kind)
	}
	if len(pType.Elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(pType.Elements))
	}
	if pType.Elements[0].Name != "int" || pType.Elements[1].Name != "str" {
		t.Errorf("expected [int, str], got [%s, %s]", pType.Elements[0].Name, pType.Elements[1].Name)
	}
}

func TestTypeExprOptional(t *testing.T) {
	m := extract(`
def f(x: Optional[str]) -> None:
    pass
`)
	fn := m.Functions[0]
	pType := fn.Parameters[0].Type
	if pType.Kind != model.TypeExprOptional {
		t.Errorf("expected TypeExprOptional, got %s", pType.Kind)
	}
	if len(pType.Elements) != 1 || pType.Elements[0].Name != "str" {
		t.Errorf("expected elements [str], got %+v", pType.Elements)
	}
}

func TestTypeExprCallable(t *testing.T) {
	m := extract(`
def f(cb: Callable[[int, str], bool]) -> None:
    pass
`)
	fn := m.Functions[0]
	pType := fn.Parameters[0].Type
	if pType.Kind != model.TypeExprCallable {
		t.Errorf("expected TypeExprCallable, got %s", pType.Kind)
	}
	if len(pType.Args) != 2 {
		t.Fatalf("expected 2 callable args, got %d", len(pType.Args))
	}
	if pType.Args[0].Name != "int" || pType.Args[1].Name != "str" {
		t.Errorf("expected args [int, str], got [%s, %s]", pType.Args[0].Name, pType.Args[1].Name)
	}
	if pType.Returns == nil || pType.Returns.Name != "bool" {
		t.Errorf("expected return bool, got %+v", pType.Returns)
	}
}

func TestTypeExprNone(t *testing.T) {
	m := extract(`
def f() -> None:
    pass
`)
	fn := m.Functions[0]
	if fn.Returns.Kind != model.TypeExprNone {
		t.Errorf("expected TypeExprNone, got %s", fn.Returns.Kind)
	}
}

func TestExtractTypeAlias(t *testing.T) {
	m := extract(`
type Vector = list[float]
`)

	if len(m.TypeAliases) != 1 {
		t.Fatalf("expected 1 type alias, got %d", len(m.TypeAliases))
	}
	ta := m.TypeAliases[0]
	if ta.Name != "Vector" {
		t.Errorf("expected name 'Vector', got %q", ta.Name)
	}
	if ta.Value == nil {
		t.Fatal("expected value")
	}
	if ta.Value.Kind != model.TypeExprGeneric || ta.Value.Name != "list" {
		t.Errorf("expected generic list, got %+v", ta.Value)
	}
}

func TestExtractModuleDocstring(t *testing.T) {
	m := extract(`"""This is the module docstring."""

def f():
    pass
`)

	if m.Docstring == nil {
		t.Fatal("expected module docstring")
	}
	if m.Docstring.Text != "This is the module docstring." {
		t.Errorf("expected 'This is the module docstring.', got %q", m.Docstring.Text)
	}
}

func TestExtractMethodIsMethod(t *testing.T) {
	m := extract(`
class Foo:
    def do_thing(self, x: int) -> str:
        pass
`)

	cls := m.Classes[0]
	if len(cls.Methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(cls.Methods))
	}
	method := cls.Methods[0]
	if !method.IsMethod {
		t.Error("expected IsMethod=true")
	}
	// 'self' should be stripped from parameters.
	if len(method.Parameters) != 1 {
		t.Fatalf("expected 1 param (self stripped), got %d", len(method.Parameters))
	}
	if method.Parameters[0].Name != "x" {
		t.Errorf("expected param 'x', got %q", method.Parameters[0].Name)
	}
}

func TestExtractBaseClasses(t *testing.T) {
	m := extract(`
class Child(Parent, Mixin):
    pass
`)

	cls := m.Classes[0]
	if len(cls.BaseClasses) != 2 {
		t.Fatalf("expected 2 base classes, got %d", len(cls.BaseClasses))
	}
	if cls.BaseClasses[0].Name != "Parent" {
		t.Errorf("expected 'Parent', got %q", cls.BaseClasses[0].Name)
	}
	if cls.BaseClasses[1].Name != "Mixin" {
		t.Errorf("expected 'Mixin', got %q", cls.BaseClasses[1].Name)
	}
}

func TestExtractParameterKinds(t *testing.T) {
	m := extract(`
def f(pos_only, /, normal, *args, kw_only, **kwargs):
    pass
`)

	fn := m.Functions[0]
	if len(fn.Parameters) != 5 {
		t.Fatalf("expected 5 params, got %d", len(fn.Parameters))
	}

	expected := []struct {
		name string
		kind model.ParamKind
	}{
		{"pos_only", model.ParamPositionalOnly},
		{"normal", model.ParamPositionalOrKeyword},
		{"args", model.ParamVarPositional},
		{"kw_only", model.ParamKeywordOnly},
		{"kwargs", model.ParamVarKeyword},
	}

	for i, exp := range expected {
		if fn.Parameters[i].Name != exp.name {
			t.Errorf("param %d: expected name %q, got %q", i, exp.name, fn.Parameters[i].Name)
		}
		if fn.Parameters[i].Kind != exp.kind {
			t.Errorf("param %d (%s): expected kind %s, got %s", i, exp.name, exp.kind, fn.Parameters[i].Kind)
		}
	}
}

func TestExtractImports(t *testing.T) {
	m := extract(`
import os
from typing import Optional, List
`)

	if len(m.Imports) != 3 {
		t.Fatalf("expected 3 imports, got %d", len(m.Imports))
	}
	names := make(map[string]bool)
	for _, imp := range m.Imports {
		names[imp.Name] = true
	}
	if !names["os"] {
		t.Error("expected import 'os'")
	}
	if !names["Optional"] {
		t.Error("expected import 'Optional'")
	}
	if !names["List"] {
		t.Error("expected import 'List'")
	}
}

func TestExtractNestedInTry(t *testing.T) {
	src := "try:\n    def fallback(): pass\nexcept:\n    pass\n"
	mod := extract(src)
	if len(mod.Functions) != 1 || mod.Functions[0].Name != "fallback" {
		t.Errorf("Functions = %v, expected fallback", mod.Functions)
	}
}

func TestExtractNestedInIf(t *testing.T) {
	src := "if True:\n    class Inner:\n        pass\n"
	mod := extract(src)
	if len(mod.Classes) != 1 || mod.Classes[0].Name != "Inner" {
		t.Errorf("Classes = %v, expected Inner", mod.Classes)
	}
}

func TestExtractAllTuple(t *testing.T) {
	src := "__all__ = (\"public_func\",)\n\ndef public_func(): pass\ndef other_func(): pass\n"
	mod := extract(src)
	if len(mod.Functions) != 1 || mod.Functions[0].Name != "public_func" {
		t.Errorf("Functions = %v, expected only public_func", mod.Functions)
	}
}

func TestTypingModulePrefix(t *testing.T) {
	m := extract(`
def f(x: typing.Optional[str]) -> typing.List[int]:
    pass
`)

	fn := m.Functions[0]
	pType := fn.Parameters[0].Type
	if pType.Kind != model.TypeExprOptional {
		t.Errorf("expected TypeExprOptional for typing.Optional, got %s", pType.Kind)
	}
	rType := fn.Returns
	if rType.Kind != model.TypeExprGeneric || rType.Name != "List" {
		t.Errorf("expected generic List for typing.List, got %+v", rType)
	}
}

func TestConstructorPlain(t *testing.T) {
	m := extract(`
class Client:
    def __init__(self, url: str, timeout: float = 5.0):
        self.url = url
`)
	cls := m.Classes[0]
	if cls.Constructor == nil {
		t.Fatal("expected constructor")
	}
	ps := cls.Constructor.Parameters
	if len(ps) != 2 || ps[0].Name != "url" || ps[1].Default != "5.0" {
		t.Errorf("unexpected params: %+v", ps)
	}
	if len(cls.Methods) != 0 {
		t.Errorf("__init__ must not appear in methods, got %d", len(cls.Methods))
	}
	if len(cls.Attributes) == 0 {
		t.Error("expected init attributes to be kept")
	}
}

func TestConstructorClassDocstringParams(t *testing.T) {
	m := extract(`
class Client:
    """A client.

    Args:
        url: The base URL.
        timeout: Seconds to wait.
    """
    def __init__(self, url, timeout=5):
        pass
`)
	ps := m.Classes[0].Constructor.Parameters
	if ps[0].DocParam == nil || ps[0].DocParam.Description != "The base URL." {
		t.Errorf("expected class docstring merge for url, got %+v", ps[0].DocParam)
	}
	if ps[1].DocParam == nil || ps[1].DocParam.Description != "Seconds to wait." {
		t.Errorf("expected class docstring merge for timeout, got %+v", ps[1].DocParam)
	}
}

func TestConstructorInitDocstringWins(t *testing.T) {
	m := extract(`
class Client:
    """A client.

    Args:
        url: From class.
        timeout: From class.
    """
    def __init__(self, url, timeout=5):
        """Init.

        Args:
            url: From init.
        """
`)
	ps := m.Classes[0].Constructor.Parameters
	if ps[0].DocParam == nil || ps[0].DocParam.Description != "From init." {
		t.Errorf("expected __init__ docstring to win, got %+v", ps[0].DocParam)
	}
	if ps[1].DocParam == nil || ps[1].DocParam.Description != "From class." {
		t.Errorf("expected class docstring fallback, got %+v", ps[1].DocParam)
	}
}

func TestConstructorDataclass(t *testing.T) {
	m := extract(`
from dataclasses import dataclass, field
from typing import ClassVar

@dataclass
class Point:
    x: float
    y: float = 0.0
    tags: list = field(default_factory=list)
    count: ClassVar[int] = 0
`)
	c := m.Classes[0].Constructor
	if c == nil {
		t.Fatal("expected synthesized constructor")
	}
	if len(c.Parameters) != 3 {
		t.Fatalf("expected 3 params (ClassVar excluded), got %d: %+v", len(c.Parameters), c.Parameters)
	}
	if c.Parameters[0].Name != "x" || c.Parameters[0].Default != "" {
		t.Errorf("bad x: %+v", c.Parameters[0])
	}
	if c.Parameters[1].Default != "0.0" {
		t.Errorf("bad y default: %q", c.Parameters[1].Default)
	}
	if c.Parameters[2].Default != "field(default_factory=list)" {
		t.Errorf("bad tags default: %q", c.Parameters[2].Default)
	}
	for _, p := range c.Parameters {
		if p.Kind != model.ParamPositionalOrKeyword {
			t.Errorf("param %s kind %s", p.Name, p.Kind)
		}
	}
}

func TestConstructorDataclassKwOnly(t *testing.T) {
	m := extract(`
import dataclasses

@dataclasses.dataclass(kw_only=True)
class Opts:
    a: int
`)
	c := m.Classes[0].Constructor
	if c == nil || len(c.Parameters) != 1 || c.Parameters[0].Kind != model.ParamKeywordOnly {
		t.Fatalf("expected one keyword-only param, got %+v", c)
	}
}

func TestConstructorAbsent(t *testing.T) {
	m := extract(`
class Plain:
    x: int = 1
`)
	if m.Classes[0].Constructor != nil {
		t.Errorf("expected nil constructor, got %+v", m.Classes[0].Constructor)
	}
}

func dcParams(t *testing.T, src string) []*model.Parameter {
	t.Helper()
	m := extract(src)
	c := m.Classes[0].Constructor
	if c == nil {
		t.Fatal("expected constructor")
	}
	return c.Parameters
}

func paramNames(ps []*model.Parameter) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Name)
	}
	return out
}

func TestDataclassInitFalseDecorator(t *testing.T) {
	m := extract(`
from dataclasses import dataclass

@dataclass(init=False)
class A:
    x: int
`)
	if m.Classes[0].Constructor != nil {
		t.Errorf("expected nil constructor, got %+v", m.Classes[0].Constructor)
	}
}

func TestDataclassFieldInitFalse(t *testing.T) {
	ps := dcParams(t, `
import dataclasses
from dataclasses import dataclass, field

@dataclass
class A:
    x: int
    y: int = field(init=False)
    z: int = dataclasses.field(init=False, default=0)
    w: int = 1
`)
	got := paramNames(ps)
	if len(got) != 2 || got[0] != "x" || got[1] != "w" {
		t.Errorf("got %v, want [x w]", got)
	}
}

func TestDataclassKWOnlySentinel(t *testing.T) {
	ps := dcParams(t, `
from dataclasses import dataclass, KW_ONLY

@dataclass
class A:
    a: int
    _: KW_ONLY
    b: int
    c: int = 2
`)
	got := paramNames(ps)
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("got %v, want [a b c]", got)
	}
	if ps[0].Kind != model.ParamPositionalOrKeyword || ps[1].Kind != model.ParamKeywordOnly || ps[2].Kind != model.ParamKeywordOnly {
		t.Errorf("bad kinds: %s %s %s", ps[0].Kind, ps[1].Kind, ps[2].Kind)
	}

	ps = dcParams(t, `
import dataclasses

@dataclasses.dataclass
class B:
    a: int
    _: dataclasses.KW_ONLY
    b: int
`)
	if len(ps) != 2 || ps[1].Kind != model.ParamKeywordOnly {
		t.Errorf("dataclasses.KW_ONLY not handled: %+v", ps)
	}
}

func TestDataclassFieldKWOnlyReordered(t *testing.T) {
	ps := dcParams(t, `
from dataclasses import dataclass, field

@dataclass
class A:
    a: int
    b: int = field(kw_only=True, default=0)
    c: int = 3
`)
	got := paramNames(ps)
	if len(got) != 3 || got[0] != "a" || got[1] != "c" || got[2] != "b" {
		t.Fatalf("got %v, want [a c b]", got)
	}
	if ps[2].Kind != model.ParamKeywordOnly || ps[1].Kind != model.ParamPositionalOrKeyword {
		t.Errorf("bad kinds: %s %s", ps[1].Kind, ps[2].Kind)
	}
}

func TestDataclassInitVar(t *testing.T) {
	ps := dcParams(t, `
import dataclasses
from dataclasses import dataclass, InitVar

@dataclass
class A:
    x: InitVar[int]
    y: dataclasses.InitVar[str] = "a"
`)
	if len(ps) != 2 {
		t.Fatalf("got %v", paramNames(ps))
	}
	if ps[0].Type == nil || ps[0].Type.Raw != "int" {
		t.Errorf("x type = %+v, want int", ps[0].Type)
	}
	if ps[1].Type == nil || ps[1].Type.Raw != "str" {
		t.Errorf("y type = %+v, want str", ps[1].Type)
	}
}

func TestDataclassInheritedBasesReturnNil(t *testing.T) {
	m := extract(`
from dataclasses import dataclass

@dataclass
class Child(Base):
    x: int
`)
	if m.Classes[0].Constructor != nil {
		t.Errorf("expected nil for subclass of unknown base, got %+v", m.Classes[0].Constructor)
	}
	for _, base := range []string{"object", "ABC", "abc.ABC", "Protocol", "typing.Protocol", "Generic[T]", "typing.Generic[T]"} {
		m := extract("from dataclasses import dataclass\n\n@dataclass\nclass C(" + base + "):\n    x: int\n")
		if m.Classes[0].Constructor == nil {
			t.Errorf("base %q should keep the synthesized constructor", base)
		}
	}
}

func TestExtractAllExports_Forms(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		wantNames []string
		wantRefs  []string
		dynamic   bool
	}{
		{name: "list", src: `__all__ = ["a", "b"]`, wantNames: []string{"a", "b"}},
		{name: "tuple", src: `__all__ = ("a", "b")`, wantNames: []string{"a", "b"}},
		{name: "plus", src: `__all__ = ["a"] + ["b"] + ("c",)`, wantNames: []string{"a", "b", "c"}},
		{name: "plus equals", src: "__all__ = [\"a\"]\n__all__ += [\"b\"]", wantNames: []string{"a", "b"}},
		{name: "extend", src: "__all__ = [\"a\"]\n__all__.extend([\"b\", \"c\"])", wantNames: []string{"a", "b", "c"}},
		{name: "append", src: "__all__ = [\"a\"]\n__all__.append(\"b\")", wantNames: []string{"a", "b"}},
		{name: "ref", src: `__all__ = sub.__all__`, wantRefs: []string{"sub"}},
		{name: "refs and names", src: `__all__ = ["a"] + x.__all__ + y.z.__all__`, wantNames: []string{"a"}, wantRefs: []string{"x", "y.z"}},
		{name: "ref plus equals", src: "__all__ = []\n__all__ += sub.__all__", wantRefs: []string{"sub"}},
		{name: "ref extend", src: "__all__ = []\n__all__.extend(sub.__all__)", wantRefs: []string{"sub"}},
		{name: "starred ref", src: `__all__ = ["a", *sub.__all__]`, wantNames: []string{"a"}, wantRefs: []string{"sub"}},
		{name: "duplicates", src: "__all__ = [\"a\", \"a\"]\n__all__ += [\"a\"]", wantNames: []string{"a"}},
		{name: "annotated", src: `__all__: list[str] = ["a"]`, wantNames: []string{"a"}},
		{name: "in if", src: "__all__ = [\"a\"]\nif X:\n    __all__ += [\"b\"]\nelse:\n    __all__ += [\"c\"]", wantNames: []string{"a", "b", "c"}},
		{name: "in try", src: "__all__ = [\"a\"]\ntry:\n    __all__.append(\"b\")\nexcept ImportError:\n    pass", wantNames: []string{"a", "b"}},
		{name: "comprehension", src: `__all__ = [n for n in dir()]`, dynamic: true},
		{name: "call", src: `__all__ = make()`, dynamic: true},
		{name: "partial", src: `__all__ = ["a"] + make()`, wantNames: []string{"a"}, dynamic: true},
		{name: "non string element", src: `__all__ = ["a", foo.__name__]`, wantNames: []string{"a"}, dynamic: true},
		{name: "other attribute", src: `__all__ = sub.names`, dynamic: true},
		{name: "append variable", src: "__all__ = [\"a\"]\n__all__.append(name)", wantNames: []string{"a"}, dynamic: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := extract(tt.src + "\n")
			if m.Exports == nil {
				t.Fatal("Exports is nil")
			}
			if !slices.Equal(m.Exports.Names, tt.wantNames) {
				t.Errorf("Names = %q, want %q", m.Exports.Names, tt.wantNames)
			}
			if !slices.Equal(m.Exports.Refs, tt.wantRefs) {
				t.Errorf("Refs = %q, want %q", m.Exports.Refs, tt.wantRefs)
			}
			if m.Exports.Dynamic != tt.dynamic {
				t.Errorf("Dynamic = %v, want %v", m.Exports.Dynamic, tt.dynamic)
			}
		})
	}
}

func TestExtractAllExports_Absent(t *testing.T) {
	m := extract("def f(): pass\n__all__other = [\"x\"]\n")
	if m.Exports != nil {
		t.Errorf("Exports = %+v, want nil", m.Exports)
	}
}

func TestExtractAllExports_InFunctionIgnored(t *testing.T) {
	m := extract("def f():\n    __all__ = [\"x\"]\n")
	if m.Exports != nil {
		t.Errorf("Exports = %+v, want nil", m.Exports)
	}
}

func TestExtractAllExports_FiltersOnlyWhenComplete(t *testing.T) {
	// A complete __all__ hides unlisted names.
	m := extract("__all__ = [\"a\"] + [\"b\"]\ndef a(): pass\ndef c(): pass\n")
	if len(m.Functions) != 1 || m.Functions[0].Name != "a" {
		t.Errorf("complete __all__: Functions = %v", m.Functions)
	}
	if len(m.Hidden.Functions) != 1 || m.Hidden.Functions[0].Name != "c" {
		t.Errorf("complete __all__: Hidden.Functions = %v", m.Hidden.Functions)
	}

	// A partial __all__ cannot judge names yet, so the underscore rule applies.
	m = extract("__all__ = [\"a\"] + sub.__all__\ndef c(): pass\ndef _d(): pass\n")
	if len(m.Functions) != 1 || m.Functions[0].Name != "c" {
		t.Errorf("partial __all__: Functions = %v", m.Functions)
	}
	if len(m.Hidden.Functions) != 1 || m.Hidden.Functions[0].Name != "_d" {
		t.Errorf("partial __all__: Hidden.Functions = %v", m.Hidden.Functions)
	}
}

func TestExtractModule_HiddenDefinitions(t *testing.T) {
	m := extract(`
def _f(): pass
class _C: pass
_a = 1
_b: int = 2
type _T = int
`)
	if len(m.Functions)+len(m.Classes)+len(m.Attributes)+len(m.TypeAliases) != 0 {
		t.Errorf("private names leaked into public lists: %+v", m)
	}
	if len(m.Hidden.Functions) != 1 || len(m.Hidden.Classes) != 1 ||
		len(m.Hidden.Attributes) != 2 || len(m.Hidden.TypeAliases) != 1 {
		t.Errorf("Hidden = %+v", m.Hidden)
	}
}

func TestExtractModule_ImportFroms(t *testing.T) {
	m := extract(`
import os
import a.b as c
from .x import y
from ..p.q import r as s
from . import t
from .u import *
from abs.mod import v
if TYPE_CHECKING:
    from ._w import W
`)
	want := []model.Import{
		{Module: "os"},
		{Module: "a.b", Alias: "c"},
		{Module: "x", Level: 1, Name: "y"},
		{Module: "p.q", Level: 2, Name: "r", Alias: "s"},
		{Level: 1, Name: "t"},
		{Module: "u", Level: 1, Name: "*"},
		{Module: "abs.mod", Name: "v"},
		{Module: "_w", Level: 1, Name: "W"},
	}
	if len(m.ImportFroms) != len(want) {
		t.Fatalf("ImportFroms len = %d, want %d", len(m.ImportFroms), len(want))
	}
	for i, w := range want {
		if *m.ImportFroms[i] != w {
			t.Errorf("ImportFroms[%d] = %+v, want %+v", i, *m.ImportFroms[i], w)
		}
	}
	// The existing field keeps its shape.
	if len(m.Imports) != len(want) || m.Imports[1].Name != "c" {
		t.Errorf("Imports = %+v", m.Imports)
	}
}
