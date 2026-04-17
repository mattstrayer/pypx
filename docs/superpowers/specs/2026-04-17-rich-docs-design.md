# Rich API Docs Design

**Date:** 2026-04-17
**Status:** Approved

## Problem

Django's API docs on pypx show empty or unhelpful signatures and no docstrings. Three compounding causes:

1. **Class methods are dropped** — goopy extracts them; the handler discards them. For Django's ORM-heavy API (`QuerySet`, `Model`, `Form`) this means users see a class name and nothing else.
2. **Docstring types parsed but never sent** — goopy's Sphinx/Google/NumPy parsers correctly extract structured `Params[].Type` and `Returns.Type`, but the handler sends only `Docstring.Text` (raw markup string), discarding all structured data.
3. **Raw docstring text renders as noise** — users see `:param value: The input` instead of prose because the frontend receives the unstripped raw text.

## Solution

Fix the handler-to-frontend pipeline. All the data exists in goopy's model — it just needs to flow through.

## Data Model (`docs.go`)

### New `DocRaise` struct

```go
type DocRaise struct {
    Type        string `json:"type"`
    Description string `json:"description"`
}
```

### Updated `DocSymbol`

```go
type DocSymbol struct {
    Name       string      `json:"name"`
    Kind       string      `json:"kind"`
    Signature  string      `json:"signature"`
    Docstring  string      `json:"docstring"`
    Parameters []DocParam  `json:"parameters"`
    Returns    *DocReturn  `json:"returns,omitempty"`
    Raises     []DocRaise  `json:"raises,omitempty"`
    Methods    []DocSymbol `json:"methods,omitempty"` // populated for kind="class" only
}
```

`Docstring` remains a plain string but is stripped of Sphinx/Google/NumPy markup lines before sending.

## Handler Changes (`api/internal/handler/docs.go`)

### 1. `docstringText` — clean markup before sending

Strip lines starting with `:param`, `:type`, `:returns:`, `:rtype:`, `:raises:` and trim trailing whitespace. Returns clean prose summary only.

### 2. `convertFunction` — backfill types from docstring

- **Parameter types:** if `p.Type == nil` and `p.DocParam != nil && p.DocParam.Type != ""`, use `p.DocParam.Type` as the parameter type.
- **Return type:** if `fn.Returns == nil` and `fn.Docstring != nil && fn.Docstring.Returns != nil`, populate `DocReturn` from `fn.Docstring.Returns.Type` and `fn.Docstring.Returns.Description`.
- **Raises:** populate `Raises []DocRaise` from `fn.Docstring.Raises`.

### 3. `buildFuncSignature` — use docstring return as fallback

After parameter list, if `fn.Returns` annotation is nil, check `fn.Docstring.Returns.Type` and append `-> <type>` if present.

### 4. `convertClass` — include methods

Convert each entry in `cls.Methods` via `convertFunction` and attach as `Methods []DocSymbol` on the class symbol.

### Filter unchanged

`convertToDocsResponse` already filters modules to only those with functions/classes/exceptions. No change needed there.

## Frontend Changes (`web/`)

### `types/api.ts`

Add to `DocSymbol`:
```ts
raises?: DocRaise[]
methods?: DocSymbol[]
```

Add new type:
```ts
interface DocRaise {
  type: string
  description: string
}
```

### `docs.vue` — methods under classes

After a class's signature/docstring/params block, render its methods. Each method uses the same layout as a top-level function (signature, params table, returns, raises). Methods list is **collapsed by default** — toggled open per class — since some Django classes have 30+ methods.

### `docs.vue` — raises section

Below the Returns block, add a Raises section using the same table styling as Parameters. Only rendered when `raises.length > 0`.

### Sidebar unchanged

Classes remain top-level sidebar entries. Methods are not listed in the sidebar (deferred — too noisy for packages with 1,700+ classes).

## Out of Scope

- Surfacing class methods in the sidebar (deferred)
- Typeshed / `.pyi` stub integration
- Rendering docstrings as HTML (prose text is sufficient)
- Changes to goopy itself — all fixes are in the handler layer

## Testing

- `docs_test.go` — add cases for: docstring type backfill on params, docstring return backfill, raises extraction, class methods in response, markup stripping
- Manual verification against Django 6.0.4: spot-check `QuerySet`, `Model`, `Form`, `HttpRequest` for method visibility and type completeness
