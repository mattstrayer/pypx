"""Tests for docs-worker helper functions."""

from types import SimpleNamespace
from unittest.mock import MagicMock

import pytest

from main import (
    _collect_members,
    _resolve_member,
    get_signature,
    get_docstring,
    get_top_level_packages,
    is_exception_class,
    normalize_name,
    transform_module,
    transform_parameters,
    transform_returns,
)


# ---------------------------------------------------------------------------
# Factories for fake griffe-like objects
# ---------------------------------------------------------------------------

def _make_kind(value: str):
    return SimpleNamespace(value=value)


def _make_param(name: str, annotation=None, default=None):
    return SimpleNamespace(name=name, annotation=annotation, default=default)


def _make_member(
    name: str,
    kind: str,
    *,
    docstring_value: str | None = None,
    parameters: list | None = None,
    returns=None,
    bases: list | None = None,
    members: dict | None = None,
    path: str | None = None,
):
    m = SimpleNamespace(
        name=name,
        kind=_make_kind(kind),
        docstring=SimpleNamespace(value=docstring_value) if docstring_value else None,
        parameters=parameters or [],
        returns=returns,
        members=members or {},
    )
    if bases is not None:
        m.bases = bases
    if path is not None:
        m.path = path
    return m


def _make_alias(name: str, target=None, *, resolvable: bool = True):
    alias = SimpleNamespace(
        name=name,
        kind=_make_kind("alias"),
    )
    if resolvable and target is not None:
        alias.final_target = target
    elif not resolvable:
        # Simulate unresolvable alias — final_target raises
        def _raise():
            raise Exception("unresolvable")
        alias.final_target = property(lambda self: _raise())
        # Use a mock that raises on attribute access
        alias = MagicMock()
        alias.name = name
        alias.kind = _make_kind("alias")
        alias.final_target = property(lambda self: (_ for _ in ()).throw(Exception("unresolvable")))
        type(alias).final_target = property(lambda self: (_ for _ in ()).throw(Exception("unresolvable")))
    return alias


# ---------------------------------------------------------------------------
# normalize_name
# ---------------------------------------------------------------------------

class TestNormalizeName:
    def test_dashes(self):
        assert normalize_name("my-package") == "my_package"

    def test_dots(self):
        assert normalize_name("my.package") == "my_package"

    def test_mixed(self):
        assert normalize_name("My-Cool.Package") == "my_cool_package"

    def test_already_normalized(self):
        assert normalize_name("requests") == "requests"


# ---------------------------------------------------------------------------
# get_signature
# ---------------------------------------------------------------------------

class TestGetSignature:
    def test_function_no_params(self):
        m = _make_member("foo", "function")
        assert get_signature(m) == "def foo()"

    def test_function_with_params(self):
        m = _make_member("bar", "function", parameters=[
            _make_param("x", annotation="int"),
            _make_param("y", annotation="str", default="'hello'"),
        ])
        assert get_signature(m) == "def bar(x: int, y: str = 'hello')"

    def test_function_with_return(self):
        m = _make_member("baz", "function", returns="bool")
        assert get_signature(m) == "def baz() -> bool"

    def test_class(self):
        m = _make_member("MyClass", "class")
        assert get_signature(m) == "class MyClass()"


# ---------------------------------------------------------------------------
# get_docstring
# ---------------------------------------------------------------------------

class TestGetDocstring:
    def test_with_docstring(self):
        m = _make_member("foo", "function", docstring_value="Does stuff.")
        assert get_docstring(m) == "Does stuff."

    def test_without_docstring(self):
        m = _make_member("foo", "function")
        assert get_docstring(m) == ""


# ---------------------------------------------------------------------------
# is_exception_class
# ---------------------------------------------------------------------------

class TestIsExceptionClass:
    def test_exception_base(self):
        m = _make_member("MyError", "class", bases=["Exception"])
        assert is_exception_class(m) is True

    def test_error_base(self):
        m = _make_member("MyError", "class", bases=["ValueError"])
        assert is_exception_class(m) is True

    def test_warning_base(self):
        m = _make_member("MyWarning", "class", bases=["UserWarning"])
        assert is_exception_class(m) is True

    def test_non_exception_class(self):
        m = _make_member("MyClass", "class", bases=["object"])
        assert is_exception_class(m) is False

    def test_function_not_exception(self):
        m = _make_member("foo", "function")
        assert is_exception_class(m) is False

    def test_no_bases(self):
        m = _make_member("MyClass", "class")
        assert is_exception_class(m) is False


# ---------------------------------------------------------------------------
# transform_parameters / transform_returns
# ---------------------------------------------------------------------------

class TestTransformParameters:
    def test_with_params(self):
        m = _make_member("foo", "function", parameters=[
            _make_param("x", annotation="int"),
            _make_param("y"),
        ])
        result = transform_parameters(m)
        assert len(result) == 2
        assert result[0] == {"name": "x", "type": "int", "description": ""}
        assert result[1] == {"name": "y", "type": None, "description": ""}

    def test_no_params_attr(self):
        m = SimpleNamespace(name="foo")
        assert transform_parameters(m) == []


class TestTransformReturns:
    def test_with_return(self):
        m = _make_member("foo", "function", returns="str")
        assert transform_returns(m) == {"type": "str", "description": ""}

    def test_no_return(self):
        m = _make_member("foo", "function")
        assert transform_returns(m) is None


# ---------------------------------------------------------------------------
# _resolve_member
# ---------------------------------------------------------------------------

class TestResolveMember:
    def test_non_alias_passes_through(self):
        m = _make_member("foo", "function")
        assert _resolve_member(m) is m

    def test_alias_resolves_to_target(self):
        target = _make_member("foo", "function")
        alias = _make_alias("foo", target)
        assert _resolve_member(alias) is target

    def test_unresolvable_alias_returns_none(self):
        alias = _make_alias("foo", resolvable=False)
        assert _resolve_member(alias) is None


# ---------------------------------------------------------------------------
# _collect_members
# ---------------------------------------------------------------------------

class TestCollectMembers:
    def test_direct_functions_and_classes(self):
        """Functions and classes defined directly in the module are collected."""
        module = _make_member("mymod", "module", members={
            "foo": _make_member("foo", "function"),
            "Bar": _make_member("Bar", "class"),
        })
        result = _collect_members(module)
        names = [n for n, _ in result]
        assert "foo" in names
        assert "Bar" in names

    def test_private_members_skipped(self):
        """Members starting with _ are excluded."""
        module = _make_member("mymod", "module", members={
            "public": _make_member("public", "function"),
            "_private": _make_member("_private", "function"),
            "__dunder": _make_member("__dunder", "function"),
        })
        result = _collect_members(module)
        names = [n for n, _ in result]
        assert names == ["public"]

    def test_aliases_resolved_at_top_level(self):
        """Re-exported aliases at the top level are resolved to their targets."""
        target = _make_member("get", "function")
        alias = _make_alias("get", target)
        module = _make_member("mymod", "module", members={
            "get": alias,
        })
        result = _collect_members(module)
        assert len(result) == 1
        name, member = result[0]
        assert name == "get"
        assert member is target

    def test_unresolvable_alias_skipped(self):
        """Aliases that can't be resolved are silently skipped."""
        alias = _make_alias("broken", resolvable=False)
        module = _make_member("mymod", "module", members={
            "broken": alias,
        })
        result = _collect_members(module)
        assert len(result) == 0

    def test_submodule_direct_definitions_collected(self):
        """Definitions in submodules are collected."""
        submod = _make_member("sub", "module", members={
            "helper": _make_member("helper", "function"),
        })
        module = _make_member("mymod", "module", members={
            "sub": submod,
        })
        result = _collect_members(module)
        names = [n for n, _ in result]
        assert "helper" in names

    def test_submodule_aliases_skipped(self):
        """Aliases inside submodules are skipped to avoid duplicates."""
        target = _make_member("get", "function")
        alias = _make_alias("get", target)
        submod = _make_member("sub", "module", members={
            "get": alias,
        })
        module = _make_member("mymod", "module", members={
            "sub": submod,
        })
        result = _collect_members(module)
        assert len(result) == 0

    def test_deduplication_top_level_wins(self):
        """If a symbol is re-exported at top level and defined in a submodule,
        the top-level version wins and the submodule copy is skipped."""
        target = _make_member("get", "function", docstring_value="Top-level doc")
        alias = _make_alias("get", target)
        submod_get = _make_member("get", "function", docstring_value="Submodule doc")
        submod = _make_member("api", "module", members={
            "get": submod_get,
        })
        module = _make_member("requests", "module", members={
            "get": alias,
            "api": submod,
        })
        result = _collect_members(module)
        names = [n for n, _ in result]
        assert names.count("get") == 1
        # The resolved alias (top-level) should be the one kept.
        _, member = result[0]
        assert member is target

    def test_submodule_only_definitions_included(self):
        """Symbols defined only in submodules (not re-exported) are included."""
        target = _make_member("get", "function")
        alias = _make_alias("get", target)
        submod = _make_member("api", "module", members={
            "get": _make_member("get", "function"),
            "internal_helper": _make_member("internal_helper", "function"),
        })
        module = _make_member("requests", "module", members={
            "get": alias,
            "api": submod,
        })
        result = _collect_members(module)
        names = [n for n, _ in result]
        assert "get" in names
        assert "internal_helper" in names
        assert len(result) == 2

    def test_nested_submodules(self):
        """Symbols in deeply nested submodules are collected."""
        deep_fn = _make_member("deep_func", "function")
        inner = _make_member("inner", "module", members={
            "deep_func": deep_fn,
        })
        outer = _make_member("outer", "module", members={
            "inner": inner,
        })
        module = _make_member("pkg", "module", members={
            "outer": outer,
        })
        result = _collect_members(module)
        names = [n for n, _ in result]
        assert "deep_func" in names

    def test_non_function_class_kinds_skipped(self):
        """Members that aren't functions or classes are ignored."""
        module = _make_member("mymod", "module", members={
            "CONSTANT": _make_member("CONSTANT", "attribute"),
        })
        result = _collect_members(module)
        assert len(result) == 0


# ---------------------------------------------------------------------------
# transform_module (integration of collect + transform)
# ---------------------------------------------------------------------------

class TestTransformModule:
    def test_separates_functions_classes_exceptions(self):
        module = _make_member("mymod", "module", members={
            "do_stuff": _make_member("do_stuff", "function"),
            "Widget": _make_member("Widget", "class", bases=["object"]),
            "BadError": _make_member("BadError", "class", bases=["Exception"]),
        })
        result = transform_module(module)
        assert result["name"] == "mymod"
        fn_names = [f["name"] for f in result["functions"]]
        cls_names = [c["name"] for c in result["classes"]]
        exc_names = [e["name"] for e in result["exceptions"]]
        assert "do_stuff" in fn_names
        assert "Widget" in cls_names
        assert "BadError" in exc_names

    def test_empty_module(self):
        module = _make_member("empty", "module", members={})
        result = transform_module(module)
        assert result["functions"] == []
        assert result["classes"] == []
        assert result["exceptions"] == []

    def test_re_exported_aliases_appear(self):
        """The original bug: re-exported aliases were silently dropped."""
        target_fn = _make_member("get", "function", docstring_value="Send a GET request.")
        alias = _make_alias("get", target_fn)
        module = _make_member("requests", "module", members={
            "get": alias,
        })
        result = transform_module(module)
        fn_names = [f["name"] for f in result["functions"]]
        assert "get" in fn_names

    def test_submodule_symbols_appear(self):
        """The original bug: symbols only in submodules were invisible."""
        submod = _make_member("sessions", "module", members={
            "Session": _make_member("Session", "class", bases=["object"]),
        })
        module = _make_member("requests", "module", members={
            "sessions": submod,
        })
        result = transform_module(module)
        cls_names = [c["name"] for c in result["classes"]]
        assert "Session" in cls_names

    def test_no_duplicates_across_reexport_and_submodule(self):
        """A symbol re-exported at top and defined in a submodule appears once."""
        session_cls = _make_member("Session", "class", bases=["object"])
        alias = _make_alias("Session", session_cls)
        submod = _make_member("sessions", "module", members={
            "Session": _make_member("Session", "class", bases=["object"]),
        })
        module = _make_member("requests", "module", members={
            "Session": alias,
            "sessions": submod,
        })
        result = transform_module(module)
        cls_names = [c["name"] for c in result["classes"]]
        assert cls_names.count("Session") == 1


# ---------------------------------------------------------------------------
# get_top_level_packages (uses real zipfile)
# ---------------------------------------------------------------------------

import io
import zipfile

class TestGetTopLevelPackages:
    def _make_zip(self, files: dict[str, str]) -> "zipfile.ZipFile":
        buf = io.BytesIO()
        with zipfile.ZipFile(buf, "w") as zf:
            for name, content in files.items():
                zf.writestr(name, content)
        buf.seek(0)
        return zipfile.ZipFile(buf, "r")

    def test_reads_top_level_txt(self):
        zf = self._make_zip({
            "requests-2.32.3.dist-info/top_level.txt": "requests\n",
            "requests/__init__.py": "",
        })
        assert get_top_level_packages(zf, "requests") == ["requests"]

    def test_infers_from_structure(self):
        zf = self._make_zip({
            "mypackage/__init__.py": "",
            "mypackage/core.py": "",
        })
        assert get_top_level_packages(zf, "mypackage") == ["mypackage"]

    def test_falls_back_to_normalized_name(self):
        zf = self._make_zip({
            "standalone.py": "",  # no directory structure
        })
        result = get_top_level_packages(zf, "My-Package")
        assert result == ["my_package"]
