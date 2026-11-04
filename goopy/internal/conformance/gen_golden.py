"""Generate CPython ast reference data for goopy parser conformance tests.

Usage:
  gen_golden.py --fixtures DIR   write X.golden.json beside each X.py
  gen_golden.py --tree DIR       print {relative_path: [defs]} JSON to stdout for
                                 .py and .pyi files; unparseable files are listed
                                 on stderr
"""
import argparse
import ast
import json
import os
import sys


def params(a):
    out = [x.arg for x in a.posonlyargs] + [x.arg for x in a.args]
    if a.vararg:
        out.append("*" + a.vararg.arg)
    out += [x.arg for x in a.kwonlyargs]
    if a.kwarg:
        out.append("**" + a.kwarg.arg)
    return out


def has_doc(n):
    b = n.body
    return bool(b) and isinstance(b[0], ast.Expr) and isinstance(b[0].value, ast.Constant) and isinstance(b[0].value.value, str)


def walk(stmts, prefix, out):
    for s in stmts:
        if isinstance(s, (ast.FunctionDef, ast.AsyncFunctionDef)):
            out.append(dict(q=prefix + s.name, params=params(s.args), doc=has_doc(s)))
        elif isinstance(s, ast.ClassDef):
            out.append(dict(q=prefix + s.name, params=[], doc=has_doc(s)))
            walk(s.body, prefix + s.name + ".", out)
        elif isinstance(s, (ast.If, ast.For, ast.AsyncFor, ast.While)):
            walk(s.body, prefix, out)
            walk(s.orelse, prefix, out)
        elif isinstance(s, (ast.With, ast.AsyncWith)):
            walk(s.body, prefix, out)
        elif isinstance(s, (ast.Try, ast.TryStar)):
            walk(s.body, prefix, out)
            for h in s.handlers:
                walk(h.body, prefix, out)
            walk(s.orelse, prefix, out)
            walk(s.finalbody, prefix, out)
        elif isinstance(s, ast.Match):
            for c in s.cases:
                walk(c.body, prefix, out)


def defs_of(path):
    with open(path, "rb") as f:
        tree = ast.parse(f.read(), filename=path)
    out = []
    walk(tree.body, "", out)
    out.sort(key=lambda d: (d["q"], "\0".join(d["params"])))
    return out


def main():
    ap = argparse.ArgumentParser()
    g = ap.add_mutually_exclusive_group(required=True)
    g.add_argument("--fixtures")
    g.add_argument("--tree")
    args = ap.parse_args()

    if args.fixtures:
        for name in sorted(os.listdir(args.fixtures)):
            if not name.endswith(".py"):
                continue
            d = defs_of(os.path.join(args.fixtures, name))
            dest = os.path.join(args.fixtures, name[:-3] + ".golden.json")
            with open(dest, "w") as f:
                json.dump(d, f, sort_keys=True, indent=2)
                f.write("\n")
        return

    result = {}
    for root, _, files in os.walk(args.tree):
        for name in files:
            if not name.endswith((".py", ".pyi")):
                continue
            p = os.path.join(root, name)
            rel = os.path.relpath(p, args.tree)
            try:
                result[rel] = defs_of(p)
            except (SyntaxError, ValueError, RecursionError, MemoryError) as e:
                print("unparseable: %s: %s" % (rel, e), file=sys.stderr)
    json.dump(result, sys.stdout, sort_keys=True, indent=2)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
