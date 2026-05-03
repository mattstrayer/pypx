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
- **Pure Go** — no CGo, no Python runtime, no external binaries

## What it does *not* do

- Execute Python code (this is static analysis only)
- Resolve dynamic attribute assignment, runtime metaclasses, or anything that depends on import-time effects
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
def greet(name: str) -> str:
    """Return a friendly greeting.

    Args:
        name: who to greet.
    """
    return f"hello, {name}"
`)
    mod, errs := goopy.ExtractModule("example", src)
    if len(errs) > 0 {
        fmt.Println("parse warnings:", errs)
    }
    for _, fn := range mod.Functions {
        fmt.Printf("%s(%s) -> %s — %s\n",
            fn.Name, fn.Signature, fn.ReturnType, fn.Summary)
    }
}
```

### Extract from a published package on PyPI

```go
ctx := context.Background()
pkg, err := goopy.ExtractFromPyPI(ctx, "requests", "") // empty version → latest
if err != nil {
    log.Fatal(err)
}
fmt.Printf("%s has %d modules\n", pkg.Name, len(pkg.Modules))
```

`ExtractFromPyPI` will pick the most appropriate wheel (preferring `py3-none-any`), download it, unzip into memory, and run the parser+extractor over each `.py` file. For packages without a wheel (sdist-only), this returns an error — supply your own `wheel.Fetcher` via `ExtractFromWheel` if you need different selection logic.

## API surface

| Function | Use when |
|---|---|
| `ExtractModule(name, src) (*Module, []error)` | You already have one Python file in memory |
| `ExtractPackage(ctx, name, files, topLevelPkgs) *Package` | You have many files (e.g. unpacked from a wheel) |
| `ExtractFromPyPI(ctx, name, version) (*Package, error)` | You want the convenience flow — fetch + extract |
| `ExtractFromWheel(ctx, fetcher, name, version) (*Package, error)` | You want to plug in a custom wheel source |

The full data model lives in [`model/`](./model). Highlights: `Module`, `Class`, `Function`, `Parameter`, `Docstring` (with `Summary`, `Description`, `Args`, `Returns`, `Raises`, `Examples`).

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
go test ./...
```

A separate slow ecosystem suite parses real wheels for popular packages (numpy, pandas, requests, …) and verifies extraction parity. It's gated by build tags so the default `go test` stays fast; pypx runs it weekly via the `goopy-ecosystem.yml` GitHub Actions workflow.

## License

[MIT](./LICENSE) — same as the parent `pypx` repository. Use it however you like.
