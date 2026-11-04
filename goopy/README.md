# goopy

**A pure-Go Python API documentation extractor.**

`goopy` parses Python source code and extracts structured documentation — modules, classes, functions, parameters, type annotations, and docstrings (Google, NumPy, and reStructuredText styles). It can also download a package's wheel directly from PyPI and extract docs from the included `.py` files in-process, with no Python interpreter required.

It was built to power the **Docs** tab on [pypx.app](https://pypx.app), but it's a standalone Go module and works fine as a library.

```
import "github.com/pypx/goopy"
```

## What it does

- **Recursive-descent parser** for Python 3 syntax (functions, classes, decorators, type hints, async, walrus, match statements, etc.)
- **Docstring parsing** — Google, NumPy, and Sphinx/reST styles, normalized into one structured form
- **Wheel fetching** — downloads the best-matching wheel from PyPI's JSON API and extracts the Python source files from the zip
- **Parallel extraction** — bounded worker pool sized to `GOMAXPROCS` for multi-module packages
- **Re-export resolution** — follows imports through the wheel, so an API defined in `_private.py` and published by `__init__.py` is documented. Each definition is published once: a private definition is copied into the export location with the shortest module path, then the first in sort order, with `Origin` set, and every other re-export becomes a `Module.Aliases` entry that points at it
- **Pure Go** — no CGo, no Python runtime, no external binaries

## What it does *not* do

- Execute Python code (this is static analysis only)
- Resolve dynamic attribute assignment, runtime metaclasses, or anything that depends on import-time effects
- Follow runtime re-exports such as a module `__getattr__` or `lazy_import`. `attrs` publishes `__version__` this way, so names like that stay undocumented
- Resolve names imported from another distribution. `from httpx import Timeout` in `openai` has no entry
- Read compiled modules. A name that only exists in a `.so` file, with no `.pyi` to describe it, has no entry
- Read an `__all__` that is built at runtime. Names it lists as literals still count, and the public-name rule covers the rest. A name bound only by `globals().update(...)` has no entry, even if another `__all__` lists it: `scipy.special.multigammaln` reaches its package this way. `scipy.special.sinc` has no entry for a different reason, since `_basic.py` imports it from `numpy`
- Type-check the code — annotations are captured as strings
- Replace tools like Sphinx for full documentation site generation

## Quick start

### Extract from a single source file

```go
package main

import (
    "fmt"

    "github.com/pypx/goopy"
)

func main() {
    src := []byte(`
def greet(name: str, excited: bool = False) -> str:
    """Return a friendly greeting.

    Args:
        name: who to greet.
        excited: end the greeting with an exclamation mark.
    """
    return f"hello, {name}"
`)
    mod, errs := goopy.ExtractModule("example", src)
    if len(errs) > 0 {
        fmt.Println("parse warnings:", errs)
    }
    for _, fn := range mod.Functions {
        fmt.Printf("%s (%s docstring)\n", fn.Name, fn.Docstring.Style)
        for _, p := range fn.Parameters {
            typ := ""
            if p.Type != nil {
                typ = p.Type.Raw
            }
            fmt.Printf("  param %s (%s) default=%q\n", p.Name, typ, p.Default)
        }
        for _, p := range fn.Docstring.Params {
            fmt.Printf("  doc   %s: %s\n", p.Name, p.Description)
        }
        if fn.Returns != nil {
            fmt.Printf("  returns %s\n", fn.Returns.Raw)
        }
    }
}
```

This prints:

```
greet (google docstring)
  param name (str) default=""
  param excited (bool) default="False"
  doc   name: who to greet.
  doc   excited: end the greeting with an exclamation mark.
  returns str
```

### Extract from a published package on PyPI

```go
ctx := context.Background()
pkg, err := goopy.ExtractFromPyPI(ctx, "requests", "") // empty version: latest release
if err != nil {
    log.Fatal(err)
}
fmt.Printf("%s %s has %d modules\n", pkg.Name, pkg.Version, len(pkg.Modules))
```

With an empty version, `pkg.Version` holds the release PyPI served.

`ExtractFromPyPI` picks one wheel (preferring `py3-none-any`, then the smallest), reads the `.py` files into memory, and runs the parser and extractor over each one. Wheels above a size threshold are read with HTTP range requests, so only the zip directory and the `.py` entries cross the network. For packages without a wheel (sdist-only), it returns `wheel.ErrNoArtifact`. Supply your own `wheel.Fetcher` through `ExtractFromWheel` if you need different selection logic.

`ExtractFromPyPI` builds a new `wheel.Source` on each call. To reuse one HTTP client, build a `wheel.Source` once and pass it to `ExtractFromWheel`.

## API surface

| Function | Use when |
|---|---|
| `ExtractModule(name, src) (*Module, []error)` | You already have one Python file in memory |
| `ExtractPackage(ctx, name, files, topLevelPkgs) *Package` | You have many files (e.g. unpacked from a wheel) |
| `ExtractFromPyPI(ctx, name, version) (*Package, error)` | You want the convenience flow — fetch + extract |
| `ExtractFromWheel(ctx, fetcher, name, version) (*Package, error)` | You want to plug in a custom wheel source |

The full data model lives in [`model/`](./model). Highlights: `Module`, `Class`, `Function`, `Parameter`, and `Docstring`. A `Docstring` has `Text` (the docstring body), `Style` (`google`, `numpy`, `sphinx`, or `plain`), and the parsed sections `Params`, `Returns`, `Yields`, and `Raises`.

### Re-exports and aliases

`ExtractPackage` resolves re-exports. A definition that lives in a private module and appears in a public module (for example through `__init__.py`) is published once, in the first export location by sort order. That copy carries `Origin`, the dotted name of the defining module. Every other public module that re-exports the same definition lists it in `Module.Aliases` as a `{Name, Target}` pair.

### Extractor version

`goopy.ExtractorVersion` is a constant that names the current extraction output. Callers that cache results should include it in the cache key. Any change that alters extraction output must raise it, so cached results regenerate.

## Layout

```
goopy/
  goopy.go          Public entry points
  lexer/            Tokenizer
  parser/           Recursive-descent parser → ast/
  ast/              Python AST node types
  extractor/        Walks AST → produces model/ structs
  model/            Public data model
  docstring/        Google / NumPy / Sphinx parsers
  token/            Token kinds
  wheel/            PyPI JSON client + zip extraction
  internal/         Ecosystem parity tests, fixtures
```

## Tests

```bash
go test -short ./...
```

A separate slow ecosystem suite in `internal/ecosystem` downloads real wheels for popular packages (numpy, pandas, requests, and others) and checks extraction against griffe and the CPython AST. These tests call `testing.Short()`, so `go test -short ./...` skips them. Run them without `-short` and with a long `-timeout`, for example `go test ./internal/ecosystem/ -run TestASTConformance -count=1 -timeout 1200s`. The `goopy-ecosystem.yml` GitHub Actions workflow runs the suite on a schedule.

`internal/benchmark` times goopy on a fixed list of packages. Run it with `go test ./internal/benchmark/ -v -count=1 -timeout 600s`.

## License

[MIT](./LICENSE) — same as the parent `pypx` repository. Use it however you like.
