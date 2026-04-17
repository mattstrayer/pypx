# Stub Package Type Enrichment Design

**Date:** 2026-04-17
**Status:** Approved

## Problem

Popular Python packages like Django, DRF, requests, and boto3 lack inline type annotations in their source. goopy extracts what's there, so pypx users see empty or partial signatures for most methods. Type stub packages (`.pyi` files) exist for all of these packages and are maintained by type experts — they provide exactly the missing type information.

## Solution

Maintain a curated registry mapping package names to their known stub packages. After extracting source docs, fetch and parse the stub package using the same goopy pipeline, then merge stub types into the source result — filling gaps only, never overriding source annotations.

## Architecture

All changes are in the handler layer. goopy is unchanged. The stub-enriched result is serialized and cached under the same `docs:{name}:{version}` key as source-only results.

```
Handler receives docs request
  → extract source via goopy.ExtractFromPyPI
  → look up stub package in curated registry
  → if found: resolve stub version, extract via goopy.ExtractFromPyPI
  → merge: fill type gaps in source symbols from stub
  → serialize + cache + return
```

## Curated Registry (`api/internal/handler/stubs.go`)

A hardcoded `map[string]string` in a dedicated file:

```go
var stubRegistry = map[string]string{
    "django":              "django-stubs",
    "djangorestframework": "djangorestframework-stubs",
    "requests":            "types-requests",
    "boto3":               "boto3-stubs",
    "pandas":              "pandas-stubs",
    "pillow":              "types-Pillow",
    "redis":               "types-redis",
    "pyyaml":              "types-PyYAML",
    "stripe":              "types-stripe",
}
```

`lookupStubPackage(name string) (string, bool)` does a case-insensitive lookup.

**Why hardcoded:** Changes are audited via PRs and versioned with the binary. This is a security-sensitive list — wrong entries display incorrect types. The audit trail is a feature. Migrating to a config file later is trivial.

**Adding entries:** One-line PR. No deploy friction beyond normal.

## Stub Version Resolution

1. Call `pypi.FetchPackage(stubPkgName)` to get available versions.
2. Pick the latest release whose major version matches the source package's major version.
3. Fall back to the latest release if no major-version match exists.
4. If PyPI lookup fails, skip stubs silently (serve source-only).

## Stub Extraction

Call `goopy.ExtractFromPyPI(ctx, stubPkgName, stubVersion)` — identical to source extraction. No new goopy API needed.

Build a flat lookup index from the result:

```go
type stubIndex map[string]map[string]*model.Function // module → symbol → stub function
```

Module path matching: strip the stub package's top-level prefix and match on the remainder (e.g. `django_stubs/db/models` → `django/db/models`).

## Merge Logic

`convertFunction` receives an optional `stubFn *model.Function`. Merge rules:

- **Parameter types**: if `dp.Type == ""` after source annotation + docstring backfill, and stub has a matching parameter (by name) with non-empty `Type.Raw`, use stub type.
- **Return type**: if `sym.Returns == nil` after source + docstring backfill, and stub has a non-nil `Returns`, populate `DocReturn` from stub.
- **Docstrings, raises, descriptions**: never taken from stubs. Types only.
- **Signature**: rebuilt after merge so stub-filled types appear.
- **Unmatched parameters**: ignored in both directions. No phantom params inserted.

**Precedence:** Source annotation > docstring-declared type > stub type.

`convertClass` passes the matched stub class's methods to each `convertFunction` call on class methods.

## Graceful Degradation

Any failure in the stub path (PyPI 404, extraction error, context timeout) is logged and silently skipped. The response is always source-only docs rather than an error. Stubs are best-effort enrichment.

## Caching

No change to cache strategy. The stub-enriched result is stored under the existing `docs:{name}:{version}` key with TTL=0 (indefinite, per-version immutability). Stub resolution happens once; subsequent requests hit cache.

## Testing

- **`handler/stubs_test.go`**: registry lookup (case-insensitive, unknown packages), version matching (major match, fallback to latest, empty version list).
- **`handler/docs_test.go`**: param type filled from stub when source has none; source annotation wins over stub; return type filled from stub; docstring never taken from stub; unmatched parameters ignored.
- **Manual**: spot-check `django` (`QuerySet.filter`, `Model.save`) and `djangorestframework` (`APIView.get`, `Serializer.validate`) for stub-filled types.

## Out of Scope

- Admin UI for the registry
- Automatic stub discovery on PyPI (security risk — no namespace protection)
- Exact version matching (deferred until old-version browsing ships)
- Stub packages for packages not in the curated list
- Frontend changes — stub types flow through existing `DocSymbol.parameters[].type` and `DocSymbol.returns.type` fields
- Changes to goopy
