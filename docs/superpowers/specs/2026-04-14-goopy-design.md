# goopy: Pure Go Python Documentation Extractor

**Date:** 2026-04-14
**Status:** Approved

## Motivation

pypx currently uses a Python sidecar (docs-worker) powered by griffe to extract API documentation from PyPI wheels. This works but adds deployment complexity (a separate Python container in Docker Compose) and suffers from cold-start latency (~5-10s on first request, 150s timeout).

goopy replaces this with a pure Go library that parses Python source code and extracts structured documentation. It eliminates the Python runtime dependency, dramatically improves performance, and is designed for eventual open-source release as a standalone Go module.

### Goals

1. **Deployment simplification** — remove the Python container from the stack
2. **Performance** — eliminate cold-start penalty; parsing should be milliseconds, not seconds
3. **Richer data model** — extract class methods, properties, structured docstrings, type trees (not just flat signatures)
4. **Open-source ready** — clean public API, independently importable Go module
5. **Long-term griffe parity** — start with a richer foundation than the current worker, build toward full griffe feature parity over time

### Non-Goals

- Executing Python code (static analysis only)
- Supporting C extension-only packages (no `.py` source to parse)
- Full Python interpreter or type checker
- Replacing griffe in the Python ecosystem (different language, different audience)

## Architecture

### Module Layout

`goopy/` lives at the repo root with its own `go.mod`, making it independently importable (`github.com/pypx/goopy` or similar).

```
goopy/
  go.mod
  goopy.go                   # Public API: Extract(source) -> Package

  token/
    token.go                 # Token types (DEF, CLASS, INDENT, DEDENT, etc.)

  lexer/
    lexer.go                 # Pass 1: source bytes -> token stream

  parser/
    parser.go                # Pass 2: token stream -> AST

  ast/
    ast.go                   # AST node types (Module, FunctionDef, ClassDef, etc.)

  extractor/
    extractor.go             # Walks AST -> documentation model

  docstring/
    docstring.go             # Dispatcher: detect style, delegate to parser
    google.go                # Google-style parser
    numpy.go                 # NumPy-style parser
    sphinx.go                # Sphinx/reST-style parser
    model.go                 # Parsed docstring sections

  model/
    model.go                 # Public output types: Package, Module, Function, Class, etc.
    typeexpr.go              # Structured type expression tree

  wheel/
    wheel.go                 # Download wheel from PyPI, unzip, yield .py files

  ecosystem/
    ecosystem_test.go        # Ecosystem test runner
    packages.json            # Curated package list
```

### Data Flow

```
source bytes -> lexer -> tokens -> parser -> AST -> extractor -> doc model
```

The pipeline is strictly one-directional. Each stage has a single responsibility and a clean interface to the next.

## Parser Strategy

### Two-Pass Architecture

**Pass 1 — Lexer:** Converts source bytes into a flat token stream, handling Python's indentation-sensitive syntax.

- Tracks an indentation stack (starts with `[0]`)
- At each line start, compares leading whitespace to the stack top
- Greater indent: push level, emit `INDENT`
- Lesser indent: pop levels, emit `DEDENT` per popped level (error if indent doesn't match any stack level)
- Handles line continuations: explicit (`\`) and implicit (inside `()`, `[]`, `{}` via bracket depth tracking)
- String literals: single/double/triple-quoted, f-strings (nested `{}` depth tracking), raw strings, byte strings
- Comments discarded
- At EOF, emits `DEDENT` for every remaining level on the stack

Token types (~40-50): keywords (`def`, `class`, `import`, `from`, `as`, `if`, `else`, `async`, `return`, `raise`, `pass`, `None`, `True`, `False`, `type`), operators (`->`, `:`, `=`, `|`, `*`, `**`, `.`, `...`, `/`, `@`, `,`), delimiters (`(`, `)`, `[`, `]`, `{`, `}`), `NAME`, `NUMBER`, `STRING`, `INDENT`, `DEDENT`, `NEWLINE`, `EOF`.

**Pass 2 — Parser:** Hand-written recursive descent, producing an AST.

```
module       -> stmt* EOF
stmt         -> decorated | funcdef | classdef | import_stmt | assign | annassign
              | if_stmt | passthrough
decorated    -> ('@' expr NEWLINE)+ (funcdef | classdef)
funcdef      -> 'async'? 'def' NAME typeargs? '(' args ')' ('->' expr)? ':' block
classdef     -> 'class' NAME typeargs? ('(' arglist ')')? ':' block
block        -> NEWLINE INDENT stmt+ DEDENT
import_stmt  -> 'import' dotted_as_names
              | 'from' dots? dotted_name 'import' import_names
assign       -> expr_list '=' expr
annassign    -> expr ':' expr ('=' expr)?
if_stmt      -> 'if' expr ':' block ('elif' expr ':' block)* ('else' ':' block)?
passthrough  -> (for|while|try|with|match) ... ':' block
expr         -> or_expr ('|' or_expr)*
atom         -> NAME | NUMBER | STRING | 'None' | 'True' | 'False' | '...'
              | '(' expr ')' | '[' expr_list ']'
trailer      -> '.' NAME | '(' arglist ')' | '[' expr ']'
```

**`PassThrough` for non-doc statements:** Statements like `for`, `while`, `try`, `with`, `match` are parsed shallowly. We recurse into their bodies to find nested `def`/`class` definitions but don't model control flow semantics.

**Error recovery:** On unexpected tokens, skip to the next `NEWLINE`/`DEDENT` and continue. Collect errors in a list rather than aborting — one weird construct shouldn't prevent extracting docs from the rest of the file.

## AST Design

Internal representation (not part of public API). Key node types:

### Statements

- `Module` — top-level, contains `Body []Stmt`
- `FunctionDef` — `Name`, `Args *Arguments`, `Body`, `Decorators`, `Returns Expr`, `IsAsync`, `TypeParams`
- `ClassDef` — `Name`, `Bases []Expr`, `Keywords []Keyword`, `Body`, `Decorators`, `TypeParams`
- `Assign` — `Targets []Expr`, `Value Expr`
- `AnnAssign` — `Target Expr`, `Annotation Expr`, `Value Expr`
- `Import` / `ImportFrom` — `Names []ImportAlias`, `Module string`, `Level int`
- `If` — `Test Expr`, `Body []Stmt`, `OrElse []Stmt`
- `ExprStmt` — `Value Expr` (docstrings are expression statements containing string constants)
- `PassThrough` — `Kind string`, `Body []Stmt` (catch-all for statements we shallow-parse)

### Expressions

Subset covering what's needed for annotations, defaults, decorators, and base classes:

- `Name` — `ID string`
- `Attribute` — `Value Expr`, `Attr string` (e.g., `os.path`)
- `Subscript` — `Value Expr`, `Slice Expr` (e.g., `dict[str, int]`)
- `BinOp` — `Left Expr`, `Op string`, `Right Expr` (primarily `|` for union types)
- `Tuple`, `List` — `Elts []Expr`
- `Constant` — `Value string`, `Kind string` (str, int, float, bool, none, ellipsis)
- `Call` — `Func Expr`, `Args []Expr`, `Keywords []Keyword`
- `Starred` — `Value Expr`

### Arguments

```
Arguments {
    PosOnlyArgs  []Arg   // before /
    Args         []Arg   // positional-or-keyword
    VarArg       *Arg    // *args
    KwOnlyArgs   []Arg   // after *
    VarKwArg     *Arg    // **kwargs
    Defaults     []Expr  // defaults for Args (right-aligned)
    KwDefaults   []Expr  // defaults for KwOnlyArgs (1:1, nil = no default)
}
```

### Type Parameters (PEP 695)

```
TypeParam {
    Name        string
    Kind        string   // "typevar", "paramspec", "typevartuple"
    Bound       Expr
    Default     Expr
    Constraints []Expr
}
```

All nodes carry `Pos` (line, col, byte offset) and `End` position info.

## Public Data Model

### Core Types

```go
type Package struct {
    Name    string   `json:"name"`
    Modules []Module `json:"modules"`
}

type Module struct {
    Name        string      `json:"name"`
    Docstring   string      `json:"docstring"`
    Functions   []Function  `json:"functions"`
    Classes     []Class     `json:"classes"`
    Exceptions  []Class     `json:"exceptions"`
    Constants   []Attribute `json:"constants"`
    TypeAliases []TypeAlias `json:"type_aliases"`
    Exports     []string    `json:"exports"`
}

type Function struct {
    Name          string      `json:"name"`
    Signature     string      `json:"signature"`
    Docstring     *Docstring  `json:"docstring"`
    Parameters    []Parameter `json:"parameters"`
    Returns       *TypeRef    `json:"returns"`
    Decorators    []string    `json:"decorators"`
    IsAsync       bool        `json:"is_async"`
    IsProperty    bool        `json:"is_property"`
    IsStatic      bool        `json:"is_static"`
    IsClassMethod bool        `json:"is_classmethod"`
    IsAbstract    bool        `json:"is_abstract"`
}

type Class struct {
    Name        string      `json:"name"`
    Signature   string      `json:"signature"`
    Docstring   *Docstring  `json:"docstring"`
    Parameters  []Parameter `json:"parameters"`
    Bases       []string    `json:"bases"`
    Decorators  []string    `json:"decorators"`
    Methods     []Function  `json:"methods"`
    Properties  []Function  `json:"properties"`
    Attributes  []Attribute `json:"attributes"`
    IsAbstract  bool        `json:"is_abstract"`
    IsDataclass bool        `json:"is_dataclass"`
}

type Parameter struct {
    Name        string    `json:"name"`
    Type        *TypeExpr `json:"type,omitempty"`
    Default     string    `json:"default,omitempty"`
    Description string    `json:"description,omitempty"`
    Kind        ParamKind `json:"kind"`
}

type Attribute struct {
    Name      string    `json:"name"`
    Type      *TypeExpr `json:"type,omitempty"`
    Value     string    `json:"value,omitempty"`
    Docstring string    `json:"docstring,omitempty"`
}

type TypeAlias struct {
    Name  string    `json:"name"`
    Value *TypeExpr `json:"value"`
}

type TypeRef struct {
    Type        *TypeExpr `json:"type"`
    Description string    `json:"description,omitempty"`
}
```

### Structured Type Expressions

Annotations are parsed into a recursive tree, not stored as strings. Every node retains the original text in `Raw`.

```go
type TypeExpr struct {
    Kind     TypeExprKind `json:"kind"`
    Name     string       `json:"name,omitempty"`
    Args     []TypeExpr   `json:"args,omitempty"`
    Elements []TypeExpr   `json:"elements,omitempty"`
    Returns  *TypeExpr    `json:"returns,omitempty"`
    Value    string       `json:"value,omitempty"`
    Raw      string       `json:"raw"`
}

type TypeExprKind string  // name, generic, union, optional, callable, tuple, literal, none, ellipsis, unpack, paramspec
```

### Parsed Docstrings

```go
type Docstring struct {
    Raw         string         `json:"raw"`
    Summary     string         `json:"summary"`
    Description string         `json:"description,omitempty"`
    Params      []DocParam     `json:"params,omitempty"`
    Returns     []DocReturn    `json:"returns,omitempty"`
    Raises      []DocRaises    `json:"raises,omitempty"`
    Examples    []string       `json:"examples,omitempty"`
    Notes       string         `json:"notes,omitempty"`
    Style       DocstringStyle `json:"style"`
}

type DocstringStyle string  // google, numpy, sphinx, plain
```

### Parameter Kinds

```go
type ParamKind string  // positional_only, positional_or_keyword, var_positional, keyword_only, var_keyword
```

## Extractor Semantics

The extractor walks the AST and applies semantic interpretation to produce the public model:

- **Public/private filtering** — skip `_`-prefixed names unless `__all__` is defined (then use that)
- **`__all__` resolution** — parse list/tuple of strings, support `+= [...]` augmentation
- **Class method categorization** — use decorator info to sort into methods, properties, static/class methods
- **`__init__` parameter hoisting** — class parameters come from `__init__` signature
- **Exception detection** — class is an exception if any base name contains "Exception", "Error", "Warning", or "BaseException"
- **Alias resolution** — follow `from .submodule import Foo` to the definition when source is available; top-level re-exports take priority
- **Nested definition discovery** — recurse into `PassThrough` bodies and submodules
- **Annotation -> TypeExpr conversion** — `Name` -> `TypeName`, `Subscript` -> `TypeGeneric`, `BinOp("|")` -> `TypeUnion`, etc.
- **Docstring merging** — merge parsed docstring parameter descriptions into the `Parameter` list by name

## Docstring Parsing

Three parsers behind a common interface, plus auto-detection.

### Style Detection (run in order)

1. Contains `:param `, `:type `, `:returns:`, `:rtype:` -> Sphinx
2. Contains section header followed by dashes on next line -> NumPy
3. Contains `Args:`, `Returns:`, `Raises:`, `Example:` at line start with indent -> Google
4. Fallback -> Plain

### Parsing

Each parser splits into:
- **Summary** — first paragraph
- **Description** — subsequent paragraphs before the first section
- **Structured sections** — Params, Returns, Raises, Examples, Notes

Parsed param descriptions are merged back into the signature's `Parameter` list by the extractor. Docstring types serve as fallback when annotations are missing.

## Wheel Integration

The `wheel` package handles PyPI download + zip extraction in pure Go:

1. `GET pypi.org/pypi/{name}/{version}/json` -> find wheel URL (prefer `none-any`)
2. `HEAD` -> check `Content-Length` against 50MB limit
3. `GET` -> download into memory
4. `archive/zip` -> filter for `.py` files, skip `__pycache__`
5. Read `*.dist-info/top_level.txt` for package names, fall back to directory inference

No third-party dependencies — `archive/zip` and `net/http` handle everything.

## pypx Integration

The `DocsHandler` changes from an HTTP call to the Python sidecar to a direct Go function call:

```go
// Before
resp, err := h.httpClient.Post(h.sidecarURL+"/generate", ...)

// After
result, err := goopy.ExtractFromPyPI(ctx, name, version)
```

Deployment changes:
- Remove `docs-worker/` directory
- Remove docs-worker service from `docker-compose.yml`
- Remove `sidecarURL` config from the API
- Remove the 150-second timeout (Go parsing is milliseconds, not seconds)

Handler response types updated to reflect the richer model. Existing fields remain backward compatible.

## Testing Strategy

### Tier 1: Unit Tests (~hundreds)

- **Lexer** — indentation scenarios, string literals, f-strings, line continuations, bracket depth
- **Parser** — AST output for each grammar production, error recovery
- **Docstring** — all three styles with edge cases (multi-line params, nested indentation, mixed styles)
- **TypeExpr** — annotation parsing from simple (`str`) to complex (`Callable[[int, str], Awaitable[dict[str, list[tuple[int, ...]]]]]`)
- **Extractor** — `__all__`, decorator semantics, `__init__` hoisting, alias resolution, exception detection

### Tier 2: Golden-File Tests (~500+)

- Port griffe's test fixtures (Python source -> expected doc model output)
- Port CPython's `snippets.py` (~219 snippets, parse + structural assertions)
- Curated real-world snippets from popular packages

### Tier 3: Ecosystem Tests (CI, nightly)

```json
{
  "packages": [
    {"name": "requests",  "version": "2.32.3", "expect_functions": 20, "expect_classes": 10},
    {"name": "flask",     "version": "3.1.0",  "expect_functions": 15, "expect_classes": 25},
    {"name": "pydantic",  "version": "2.10.0", "expect_functions": 30, "expect_classes": 80}
  ]
}
```

The ecosystem runner:
1. Downloads each wheel
2. Parses all `.py` files
3. Asserts: no panics, no errors, symbol counts within expected ranges
4. Optionally compares output against a griffe baseline

### Griffe Comparison Mode

Run both goopy and griffe on the same package, diff the outputs. Provides a concrete parity measure and catches regressions.

## Target Package Scope

Initially targeting the top ~5,000 PyPI packages by download count. These cover 95%+ of pypx traffic and tend to be well-structured with standard conventions. Edge cases (dynamic metaprogramming, namespace packages, exotic syntax) handled on a best-effort basis and expanded over time.

## Key Risks

1. **Python grammar complexity** — the indentation-sensitive lexer and full expression grammar for annotations are the hardest parts. Mitigated by the two-pass architecture and extensive test suites.
2. **Edge cases in real packages** — some packages use unconventional patterns. Mitigated by the ecosystem test suite and error recovery (partial results over failures).
3. **Maintenance burden** — new Python syntax additions require parser updates. Mitigated by the grammar being relatively stable (major additions happen every 1-2 years) and the ecosystem test suite catching regressions.
4. **f-string parsing** — f-strings with nested braces and format specs are notoriously tricky. For doc extraction we need to preserve f-strings as their raw source text (e.g., `f"hello {name}"` stored as-is in default values and annotations). We do not need to evaluate or parse the interpolation expressions — just correctly lex the full string including nested braces so the lexer doesn't get confused by `}` inside an f-string.
