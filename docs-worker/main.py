"""
docs-worker: griffe-based API doc extractor.

Bare ASGI app (no FastAPI/Pydantic) with lazy-loaded heavy deps so the
idle container holds minimal RSS. griffe and httpx are imported on the
first /generate request and stay resident for all subsequent ones.
"""

import io
import json
import os
import re
import tempfile
import zipfile
from typing import Any

# Heavy dependencies loaded lazily on first request.
# At idle the container only pays for stdlib + uvicorn.
_griffe = None
_httpx = None


def _load_deps() -> None:
    global _griffe, _httpx
    if _griffe is None:
        import griffe
        import httpx
        _griffe = griffe
        _httpx = httpx


# ---------------------------------------------------------------------------
# Helpers (receive griffe objects but don't import griffe at module level)
# ---------------------------------------------------------------------------

def normalize_name(name: str) -> str:
    return re.sub(r"[-_.]+", "_", name).lower()


def get_top_level_packages(zf: zipfile.ZipFile, pkg_name: str) -> list[str]:
    """Return importable top-level package names from the wheel."""
    for entry in zf.namelist():
        if entry.endswith(".dist-info/top_level.txt"):
            content = zf.read(entry).decode("utf-8", errors="replace").strip()
            pkgs = [p.strip() for p in content.splitlines() if p.strip()]
            if pkgs:
                return pkgs
    # Infer from wheel structure.
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


def get_signature(member: Any) -> str:
    try:
        params = []
        if hasattr(member, "parameters"):
            for p in member.parameters:
                s = p.name
                if p.annotation is not None:
                    s += f": {p.annotation}"
                if p.default is not None and str(p.default) not in ("", "PosOnlyParam", "KwOnlyParam"):
                    s += f" = {p.default}"
                params.append(s)
        ret = ""
        if hasattr(member, "returns") and member.returns is not None:
            ret = f" -> {member.returns}"
        prefix = "class " if member.kind.value == "class" else "def "
        return f"{prefix}{member.name}({', '.join(params)}){ret}"
    except Exception:
        return member.name


def get_docstring(member: Any) -> str:
    if not member.docstring:
        return ""
    return member.docstring.value or ""


def is_exception_class(member: Any) -> bool:
    if member.kind.value != "class":
        return False
    for base in getattr(member, "bases", []):
        base_str = str(base)
        if any(word in base_str for word in ("Exception", "Error", "Warning", "BaseException")):
            return True
    return False


def transform_parameters(member: Any) -> list[dict]:
    if not hasattr(member, "parameters"):
        return []
    result = []
    for p in member.parameters:
        annotation = None
        if p.annotation is not None:
            try:
                annotation = str(p.annotation)
            except Exception:
                pass
        result.append({"name": p.name, "type": annotation, "description": ""})
    return result


def transform_returns(member: Any) -> dict | None:
    if not hasattr(member, "returns") or member.returns is None:
        return None
    try:
        return {"type": str(member.returns), "description": ""}
    except Exception:
        return None


def _resolve_member(member: Any) -> Any | None:
    """Follow griffe aliases to the real object. Return None if unresolvable."""
    if member.kind.value == "alias":
        try:
            return member.final_target
        except Exception:
            return None
    return member


def _collect_members(module: Any) -> list[tuple[str, Any]]:
    """Walk a module tree and return (name, resolved_member) pairs.

    Top-level re-exports (aliases) are resolved first, then submodules are
    walked for any remaining definitions. Duplicates are skipped by short
    name so each symbol appears only once.
    """
    seen: set[str] = set()
    results: list[tuple[str, Any]] = []

    def _add(name: str, member: Any) -> bool:
        if name in seen:
            return False
        seen.add(name)
        results.append((name, member))
        return True

    # Pass 1: top-level members (resolve aliases = re-exports).
    for name, member in module.members.items():
        if name.startswith("_"):
            continue
        kind = member.kind.value
        if kind == "module":
            continue
        if kind == "alias":
            resolved = _resolve_member(member)
            if resolved is None:
                continue
            member = resolved
            kind = member.kind.value
        if kind in ("function", "class"):
            _add(name, member)

    # Pass 2: walk submodules for definitions not re-exported at top level.
    def _walk_submodules(mod: Any) -> None:
        for name, member in mod.members.items():
            if name.startswith("_"):
                continue
            kind = member.kind.value
            if kind == "module":
                _walk_submodules(member)
                continue
            if kind == "alias":
                continue  # skip aliases in submodules to avoid duplicates
            if kind in ("function", "class"):
                _add(name, member)

    _walk_submodules(module)
    return results


def transform_module(module: Any) -> dict:
    functions = []
    classes = []
    exceptions = []

    for name, member in _collect_members(module):
        kind = member.kind.value
        sym: dict[str, Any] = {
            "name": name,
            "kind": kind,
            "signature": get_signature(member),
            "docstring": get_docstring(member),
            "parameters": transform_parameters(member),
            "returns": transform_returns(member),
        }
        if kind == "function":
            functions.append(sym)
        elif kind == "class":
            if is_exception_class(member):
                exceptions.append(sym)
            else:
                classes.append(sym)

    return {
        "name": module.name,
        "functions": functions,
        "classes": classes,
        "exceptions": exceptions,
    }


# ---------------------------------------------------------------------------
# Core generation logic
# ---------------------------------------------------------------------------

def _generate(name: str, version: str) -> tuple[int, dict]:
    """Return (http_status, result_dict). 200 on success, 502 on upstream error."""
    _load_deps()

    # Fetch PyPI metadata to get wheel URL.
    pypi_url = f"https://pypi.org/pypi/{name}/{version}/json"
    try:
        resp = _httpx.get(pypi_url, timeout=15)
        resp.raise_for_status()
        data = resp.json()
    except Exception as e:
        return 502, {"error": f"PyPI fetch failed: {e}"}

    # Find best wheel URL: prefer pure-python, fall back to first wheel.
    wheel_url: str | None = None
    for f in data.get("urls", []):
        if f["packagetype"] != "bdist_wheel":
            continue
        if wheel_url is None:
            wheel_url = f["url"]
        if "none-any" in f["filename"]:
            wheel_url = f["url"]
            break

    if not wheel_url:
        return 200, {"empty": True, "reason": "no_wheel", "modules": []}

    # Guard against oversized wheels (e.g. torch ~800MB) — check size before downloading.
    MAX_WHEEL_BYTES = 50 * 1024 * 1024  # 50 MB
    try:
        head_resp = _httpx.head(wheel_url, timeout=10, follow_redirects=True)
        cl = head_resp.headers.get("content-length")
        if cl and int(cl) > MAX_WHEEL_BYTES:
            return 200, {"empty": True, "reason": "wheel_too_large", "modules": []}
    except Exception:
        pass  # Can't get size; proceed and hope for the best

    # Download wheel.
    try:
        wheel_resp = _httpx.get(wheel_url, timeout=60, follow_redirects=True)
        wheel_resp.raise_for_status()
        wheel_bytes = wheel_resp.content
    except Exception as e:
        return 502, {"error": f"Wheel download failed: {e}"}

    # Open as zip.
    try:
        zf = zipfile.ZipFile(io.BytesIO(wheel_bytes))
    except Exception as e:
        return 502, {"error": f"Wheel open failed: {e}"}

    py_files = {
        entry: zf.read(entry)
        for entry in zf.namelist()
        if entry.endswith(".py") and "__pycache__" not in entry
    }

    if not py_files:
        return 200, {"empty": True, "reason": "no_python_source", "modules": []}

    top_pkgs = get_top_level_packages(zf, name)

    modules = []
    with tempfile.TemporaryDirectory() as tmpdir:
        for filename, content in py_files.items():
            filepath = os.path.join(tmpdir, filename)
            os.makedirs(os.path.dirname(filepath), exist_ok=True)
            try:
                with open(filepath, "wb") as fh:
                    fh.write(content)
            except Exception:
                continue

        for pkg_name in top_pkgs:
            try:
                module = _griffe.load(pkg_name, search_paths=[tmpdir])
                transformed = transform_module(module)
                if any(transformed[k] for k in ("functions", "classes", "exceptions")):
                    modules.append(transformed)
            except Exception:
                continue

    if not modules:
        return 200, {"empty": True, "reason": "no_python_source", "modules": []}

    return 200, {"empty": False, "reason": "", "modules": modules}


# ---------------------------------------------------------------------------
# Bare ASGI app
# ---------------------------------------------------------------------------

async def _send_response(send, status: int, body: bytes, content_type: bytes = b"application/json") -> None:
    await send({
        "type": "http.response.start",
        "status": status,
        "headers": [
            (b"content-type", content_type),
            (b"content-length", str(len(body)).encode()),
        ],
    })
    await send({"type": "http.response.body", "body": body})


async def app(scope, receive, send) -> None:
    if scope["type"] != "http":
        return

    method: str = scope["method"]
    path: str = scope["path"]

    if path == "/health" and method == "GET":
        await _send_response(send, 200, b"ok", b"text/plain")
        return

    if path == "/generate" and method == "POST":
        body = b""
        while True:
            event = await receive()
            body += event.get("body", b"")
            if not event.get("more_body", False):
                break

        try:
            data = json.loads(body)
            name = str(data["name"])
            version = str(data["version"])
        except Exception:
            await _send_response(send, 400, json.dumps({"error": "invalid request body"}).encode())
            return

        status, result = _generate(name, version)
        await _send_response(send, status, json.dumps(result).encode())
        return

    await _send_response(send, 404, b"not found", b"text/plain")


if __name__ == "__main__":
    import uvicorn
    uvicorn.run("main:app", host="0.0.0.0", port=8000)
