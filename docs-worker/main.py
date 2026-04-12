import io
import os
import re
import tempfile
import zipfile
from typing import Any

import griffe
import httpx
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

app = FastAPI()


class GenerateRequest(BaseModel):
    name: str
    version: str


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


def transform_module(module: Any) -> dict:
    functions = []
    classes = []
    exceptions = []

    for name, member in module.members.items():
        if name.startswith("_"):
            continue
        kind = member.kind.value
        sym: dict[str, Any] = {
            "name": member.name,
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


@app.post("/generate")
async def generate(req: GenerateRequest) -> dict:
    # Fetch PyPI metadata to get wheel URL.
    pypi_url = f"https://pypi.org/pypi/{req.name}/{req.version}/json"
    try:
        resp = httpx.get(pypi_url, timeout=15)
        resp.raise_for_status()
        data = resp.json()
    except Exception as e:
        raise HTTPException(status_code=502, detail=f"PyPI fetch failed: {e}")

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
        return {"empty": True, "reason": "no_wheel", "modules": []}

    # Download wheel.
    try:
        wheel_resp = httpx.get(wheel_url, timeout=60, follow_redirects=True)
        wheel_resp.raise_for_status()
        wheel_bytes = wheel_resp.content
    except Exception as e:
        raise HTTPException(status_code=502, detail=f"Wheel download failed: {e}")

    # Open as zip.
    try:
        zf = zipfile.ZipFile(io.BytesIO(wheel_bytes))
    except Exception as e:
        raise HTTPException(status_code=502, detail=f"Wheel open failed: {e}")

    py_files = {
        name: zf.read(name)
        for name in zf.namelist()
        if name.endswith(".py") and "__pycache__" not in name
    }

    if not py_files:
        return {"empty": True, "reason": "no_python_source", "modules": []}

    top_pkgs = get_top_level_packages(zf, req.name)

    modules = []
    with tempfile.TemporaryDirectory() as tmpdir:
        # Write .py files to temp dir.
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
                module = griffe.load(pkg_name, search_paths=[tmpdir])
                transformed = transform_module(module)
                if any(transformed[k] for k in ("functions", "classes", "exceptions")):
                    modules.append(transformed)
            except Exception:
                continue

    if not modules:
        return {"empty": True, "reason": "no_python_source", "modules": []}

    return {"empty": False, "reason": "", "modules": modules}


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
