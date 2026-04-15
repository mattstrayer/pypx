"""
griffe_extract.py — Reference extractor for parity testing.

Downloads a wheel from PyPI, extracts with griffe, and outputs a
normalized JSON structure for comparison against goopy's output.

Usage: python3 griffe_extract.py <name> <version>

Output JSON shape:
{
  "name": "package",
  "modules": {
    "module.name": {
      "functions": {"func_name": {"params": ["a", "b"], "has_docstring": true}},
      "classes": {"ClassName": {"methods": ["method1"], "has_docstring": true}}
    }
  }
}
"""

import io
import json
import os
import re
import sys
import tempfile
import zipfile

import griffe
import httpx


def normalize_name(name: str) -> str:
    return re.sub(r"[-_.]+", "_", name).lower()


def get_top_level_packages(zf: zipfile.ZipFile, pkg_name: str) -> list[str]:
    for entry in zf.namelist():
        if entry.endswith(".dist-info/top_level.txt"):
            content = zf.read(entry).decode("utf-8", errors="replace").strip()
            pkgs = [p.strip() for p in content.splitlines() if p.strip()]
            if pkgs:
                return pkgs
    tops: set[str] = set()
    for entry in zf.namelist():
        parts = entry.split("/")
        if (
            len(parts) >= 2
            and not parts[0].endswith(".dist-info")
            and not parts[0].endswith(".data")
            and parts[0]
        ):
            tops.add(parts[0])
    return list(tops) if tops else [normalize_name(pkg_name)]


def extract_member_info(member) -> dict:
    """Extract normalized info from a griffe member."""
    info = {
        "has_docstring": bool(member.docstring and member.docstring.value),
    }
    if hasattr(member, "parameters"):
        info["params"] = [p.name for p in member.parameters]
    if member.kind.value == "class":
        methods = []
        for name, m in member.members.items():
            if name.startswith("_") and name != "__init__":
                continue
            resolved = m
            if m.kind.value == "alias":
                try:
                    resolved = m.final_target
                except Exception:
                    continue
            if resolved.kind.value == "function":
                methods.append(name)
        info["methods"] = sorted(methods)
    return info


def walk_module(module, prefix="") -> dict:
    """Walk a griffe module tree and collect all public functions/classes.

    Only includes directly-defined members (not aliases/re-exports) so the
    comparison with goopy is apples-to-apples. Respects __all__ when present.
    """
    result = {"functions": {}, "classes": {}}

    # Check for __all__ to filter exports
    all_exports = None
    if "__all__" in module.members:
        all_member = module.members["__all__"]
        if hasattr(all_member, 'value') and all_member.value is not None:
            try:
                val_str = str(all_member.value)
                # Parse the __all__ value — it's usually a list or tuple literal
                import ast as pyast
                parsed = pyast.literal_eval(val_str)
                if isinstance(parsed, (list, tuple)):
                    all_exports = set(parsed)
            except Exception:
                pass

    for name, member in module.members.items():
        if name.startswith("_"):
            continue
        # If __all__ exists, only include listed names
        if all_exports is not None and name not in all_exports:
            continue
        kind = member.kind.value
        if kind == "module":
            continue
        # Skip aliases (re-exports) — goopy can't see these since they're
        # defined in a different file. We only compare direct definitions.
        if kind == "alias":
            continue
        # Skip members whose definition is in a different file (imported/re-exported).
        # griffe resolves imports and shows them as direct members, but goopy
        # only sees definitions in the file being parsed.
        try:
            member_file = getattr(member, 'filepath', None)
            module_file = getattr(module, 'filepath', None)
            if member_file and module_file and member_file != module_file:
                continue
        except Exception:
            pass
        if kind == "function":
            result["functions"][name] = extract_member_info(member)
        elif kind == "class":
            result["classes"][name] = extract_member_info(member)

    return result


def extract(name: str, version: str) -> dict:
    # Download wheel
    pypi_url = f"https://pypi.org/pypi/{name}/{version}/json"
    resp = httpx.get(pypi_url, timeout=15)
    resp.raise_for_status()
    data = resp.json()

    wheel_url = None
    for f in data.get("urls", []):
        if f["packagetype"] != "bdist_wheel":
            continue
        if wheel_url is None:
            wheel_url = f["url"]
        if "none-any" in f["filename"]:
            wheel_url = f["url"]
            break

    if not wheel_url:
        return {"name": name, "error": "no_wheel", "modules": {}}

    wheel_resp = httpx.get(wheel_url, timeout=60, follow_redirects=True)
    wheel_resp.raise_for_status()

    zf = zipfile.ZipFile(io.BytesIO(wheel_resp.content))
    py_files = {
        entry: zf.read(entry)
        for entry in zf.namelist()
        if entry.endswith(".py") and "__pycache__" not in entry
    }

    if not py_files:
        return {"name": name, "error": "no_source", "modules": {}}

    top_pkgs = get_top_level_packages(zf, name)

    modules = {}
    with tempfile.TemporaryDirectory() as tmpdir:
        for filename, content in py_files.items():
            filepath = os.path.join(tmpdir, filename)
            os.makedirs(os.path.dirname(filepath), exist_ok=True)
            with open(filepath, "wb") as fh:
                fh.write(content)

        for pkg_name in top_pkgs:
            try:
                module = griffe.load(pkg_name, search_paths=[tmpdir])
            except Exception:
                continue

            # Walk all submodules, skipping private ones
            def walk_all(mod, path=""):
                full_name = f"{path}.{mod.name}" if path else mod.name
                # Skip private modules (match goopy behavior)
                if path and mod.name.startswith("_"):
                    return
                mod_info = walk_module(mod)
                if mod_info["functions"] or mod_info["classes"]:
                    # Deduplicate: only keep first occurrence of each module path
                    if full_name not in modules:
                        modules[full_name] = mod_info

                for child_name, child in mod.members.items():
                    if child.kind.value == "module":
                        walk_all(child, full_name)

            walk_all(module)

    return {"name": name, "modules": modules}


if __name__ == "__main__":
    if len(sys.argv) != 3:
        print(f"Usage: {sys.argv[0]} <name> <version>", file=sys.stderr)
        sys.exit(1)
    result = extract(sys.argv[1], sys.argv[2])
    json.dump(result, sys.stdout, indent=2)
    print()
