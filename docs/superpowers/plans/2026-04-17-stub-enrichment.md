# Stub Package Type Enrichment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enrich API docs type signatures for popular untyped packages (Django, DRF, requests, etc.) by fetching and merging type stub packages from PyPI into the docs handler pipeline.

**Architecture:** After extracting source docs with goopy, the handler looks up a curated stub package name, downloads the stub wheel directly (collecting `.pyi` files which the normal wheel fetcher skips), normalizes file paths, calls `goopy.ExtractPackage`, and merges stub types into the source result — filling gaps only, never overriding source annotations. All logic is in the handler layer; goopy is unchanged.

**Tech Stack:** Go, `archive/zip`, `net/http`, `github.com/pypx/goopy` (ExtractPackage), `github.com/pypx/api/internal/pypi`

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `api/internal/handler/stubs.go` | Create | Registry, version resolution, wheel download, normalization, index building |
| `api/internal/handler/stubs_test.go` | Create | Unit tests for all stubs.go functions |
| `api/internal/handler/docs.go` | Modify | Update convertFunction/convertClass/convertToDocsResponse signatures; wire stub enrichment into Get() |
| `api/internal/handler/docs_unit_test.go` | Modify | Add merge logic tests |

---

### Task 1: Curated registry + lookup helper

**Files:**
- Create: `api/internal/handler/stubs.go`
- Create: `api/internal/handler/stubs_test.go`

- [ ] **Step 1: Write failing tests**

```go
// api/internal/handler/stubs_test.go
package handler

import "testing"

func TestLookupStubPackage(t *testing.T) {
	tests := []struct {
		name     string
		wantStub string
		wantOK   bool
	}{
		{"django", "django-stubs", true},
		{"Django", "django-stubs", true},
		{"DJANGO", "django-stubs", true},
		{"djangorestframework", "djangorestframework-stubs", true},
		{"requests", "types-requests", true},
		{"boto3", "boto3-stubs", true},
		{"unknownpkg", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		got, ok := lookupStubPackage(tt.name)
		if ok != tt.wantOK {
			t.Errorf("lookupStubPackage(%q) ok = %v, want %v", tt.name, ok, tt.wantOK)
		}
		if got != tt.wantStub {
			t.Errorf("lookupStubPackage(%q) = %q, want %q", tt.name, got, tt.wantStub)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd api && go test ./internal/handler/ -run TestLookupStubPackage -v
```

Expected: FAIL — `lookupStubPackage` undefined

- [ ] **Step 3: Implement the registry and lookup**

```go
// api/internal/handler/stubs.go
package handler

import (
	"context"
	"strings"
)

// stubRegistry maps lowercase PyPI package names to their known type stub packages.
// Only add entries for verified, well-maintained stub packages.
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

// lookupStubPackage returns the stub package name for a source package, if known.
func lookupStubPackage(name string) (string, bool) {
	stub, ok := stubRegistry[strings.ToLower(name)]
	return stub, ok
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd api && go test ./internal/handler/ -run TestLookupStubPackage -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/handler/stubs.go api/internal/handler/stubs_test.go
git commit -m "feat: add curated stub package registry and lookup helper"
```

---

### Task 2: Stub version resolution

**Files:**
- Modify: `api/internal/handler/stubs.go`
- Modify: `api/internal/handler/stubs_test.go`

- [ ] **Step 1: Write failing tests**

Add to `stubs_test.go`:

```go
import (
	"testing"

	"github.com/pypx/api/internal/pypi"
)

func TestResolveStubVersion(t *testing.T) {
	releases := map[string][]pypi.ReleaseFile{
		"5.2.0": {},
		"5.1.3": {},
		"5.0.0": {},
		"4.2.1": {},
		"4.0.0": {},
	}

	tests := []struct {
		sourceMajor string
		want        string
	}{
		{"5", "5.2.0"}, // latest matching major
		{"4", "4.2.1"}, // latest matching major
		{"6", "5.2.0"}, // no match — fall back to overall latest
		{"3", "5.2.0"}, // no match — fall back to overall latest
	}
	for _, tt := range tests {
		got := resolveStubVersion(releases, tt.sourceMajor)
		if got != tt.want {
			t.Errorf("resolveStubVersion(releases, %q) = %q, want %q", tt.sourceMajor, got, tt.want)
		}
	}
}

func TestResolveStubVersion_EmptyReleases(t *testing.T) {
	got := resolveStubVersion(map[string][]pypi.ReleaseFile{}, "5")
	if got != "" {
		t.Errorf("resolveStubVersion(empty, \"5\") = %q, want \"\"", got)
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"5.2.0", "5.1.3", 1},
		{"5.1.3", "5.2.0", -1},
		{"5.2.0", "5.2.0", 0},
		{"10.0.0", "9.9.9", 1},
		{"1.0", "1.0.1", -1},
	}
	for _, tt := range tests {
		got := compareVersions(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd api && go test ./internal/handler/ -run "TestResolveStubVersion|TestCompareVersions" -v
```

Expected: FAIL — `resolveStubVersion` and `compareVersions` undefined

- [ ] **Step 3: Implement version resolution**

Add to `api/internal/handler/stubs.go`:

```go
import (
	"context"
	"fmt"
	"strings"

	"github.com/pypx/api/internal/pypi"
)

// resolveStubVersion picks the best stub version for the given source major version.
// It prefers the latest stub release whose major version matches; falls back to the
// latest overall release if no major-version match exists.
func resolveStubVersion(releases map[string][]pypi.ReleaseFile, sourceMajor string) string {
	var matching, all []string
	for v := range releases {
		all = append(all, v)
		if strings.HasPrefix(v, sourceMajor+".") || v == sourceMajor {
			matching = append(matching, v)
		}
	}
	if len(matching) > 0 {
		return latestVersion(matching)
	}
	return latestVersion(all)
}

func latestVersion(versions []string) string {
	if len(versions) == 0 {
		return ""
	}
	best := versions[0]
	for _, v := range versions[1:] {
		if compareVersions(v, best) > 0 {
			best = v
		}
	}
	return best
}

// compareVersions compares two version strings numerically segment by segment.
// Returns 1 if a > b, -1 if a < b, 0 if equal.
func compareVersions(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	n := len(aParts)
	if len(bParts) > n {
		n = len(bParts)
	}
	for i := 0; i < n; i++ {
		var av, bv int
		if i < len(aParts) {
			fmt.Sscanf(aParts[i], "%d", &av) //nolint:errcheck
		}
		if i < len(bParts) {
			fmt.Sscanf(bParts[i], "%d", &bv) //nolint:errcheck
		}
		if av != bv {
			if av > bv {
				return 1
			}
			return -1
		}
	}
	return 0
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd api && go test ./internal/handler/ -run "TestResolveStubVersion|TestCompareVersions" -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/handler/stubs.go api/internal/handler/stubs_test.go
git commit -m "feat: add stub version resolution with major-version matching"
```

---

### Task 3: Stub wheel download and file normalization

**Files:**
- Modify: `api/internal/handler/stubs.go`
- Modify: `api/internal/handler/stubs_test.go`

Context: The standard goopy wheel fetcher only collects `.py` files and would return nothing for stub packages (which only contain `.pyi` files). We download the wheel zip directly here, extract `.pyi` files, rename them to `.py`, and normalize the top-level directory name (e.g. `django_stubs/` → `django/`) so goopy's package-matching logic works correctly.

- [ ] **Step 1: Write failing tests for normalizeStubFiles**

Add to `stubs_test.go`:

```go
func TestNormalizeStubFiles(t *testing.T) {
	input := map[string][]byte{
		"django_stubs/__init__.pyi":             []byte("x: int"),
		"django_stubs/db/models/__init__.pyi":   []byte("class QuerySet: ..."),
		"django_stubs/http/__init__.pyi":        []byte("class HttpRequest: ..."),
		"django_stubs-1.0.dist-info/METADATA":   []byte("meta"),
	}
	topLevelPkgs := []string{"django_stubs"}

	files, pkgs := normalizeStubFiles(input, topLevelPkgs)

	wantFiles := map[string]bool{
		"django/__init__.py":           true,
		"django/db/models/__init__.py": true,
		"django/http/__init__.py":      true,
	}
	if len(files) != len(wantFiles) {
		t.Errorf("normalizeStubFiles: got %d files, want %d", len(files), len(wantFiles))
	}
	for path := range wantFiles {
		if _, ok := files[path]; !ok {
			t.Errorf("normalizeStubFiles: missing expected path %q", path)
		}
	}
	if len(pkgs) != 1 || pkgs[0] != "django" {
		t.Errorf("normalizeStubFiles: pkgs = %v, want [\"django\"]", pkgs)
	}
}

func TestNormalizeStubFiles_HyphenSuffix(t *testing.T) {
	input := map[string][]byte{
		"requests_stubs/__init__.pyi": []byte("x: int"),
	}
	_, pkgs := normalizeStubFiles(input, []string{"requests_stubs"})
	if len(pkgs) != 1 || pkgs[0] != "requests" {
		t.Errorf("normalizeStubFiles: pkgs = %v, want [\"requests\"]", pkgs)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd api && go test ./internal/handler/ -run TestNormalizeStubFiles -v
```

Expected: FAIL — `normalizeStubFiles` undefined

- [ ] **Step 3: Implement normalizeStubFiles**

Add to `api/internal/handler/stubs.go` (imports section — add `archive/zip`, `bytes`, `io`, `net/http`):

```go
// normalizeStubFiles renames .pyi files to .py and maps stub top-level package
// directories to their source equivalents (e.g. django_stubs/ → django/).
// Non-.pyi files (dist-info, etc.) are excluded.
func normalizeStubFiles(files map[string][]byte, topLevelPkgs []string) (map[string][]byte, []string) {
	renames := make(map[string]string, len(topLevelPkgs))
	normalized := make([]string, 0, len(topLevelPkgs))
	for _, pkg := range topLevelPkgs {
		src := strings.TrimSuffix(strings.TrimSuffix(pkg, "_stubs"), "-stubs")
		renames[pkg] = src
		normalized = append(normalized, src)
	}

	result := make(map[string][]byte, len(files))
	for path, data := range files {
		if !strings.HasSuffix(path, ".pyi") {
			continue
		}
		// .pyi → .py
		p := strings.TrimSuffix(path, ".pyi") + ".py"
		// stub dir → source dir
		for stubDir, srcDir := range renames {
			if strings.HasPrefix(p, stubDir+"/") {
				p = srcDir + "/" + p[len(stubDir)+1:]
				break
			}
		}
		result[p] = data
	}
	return result, normalized
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd api && go test ./internal/handler/ -run TestNormalizeStubFiles -v
```

Expected: PASS

- [ ] **Step 5: Implement fetchStubPackage**

Add to `api/internal/handler/stubs.go`. This downloads the wheel zip directly (without goopy's wheel.Source, which filters out .pyi files), normalizes the contents, and calls goopy.ExtractPackage.

Update imports in `stubs.go` to include: `"archive/zip"`, `"bytes"`, `"io"`, `"net/http"`, `"github.com/pypx/goopy"`.

```go
// fetchStubPackage downloads a stub wheel, extracts and normalizes .pyi files,
// and returns a parsed model.Package. Returns nil, nil if no wheel URL is found.
func fetchStubPackage(ctx context.Context, stubPkgName string, releases map[string][]pypi.ReleaseFile, version string) (*model.Package, error) {
	files := releases[version]
	var wheelURL string
	for _, f := range files {
		if strings.HasSuffix(f.Filename, ".whl") {
			wheelURL = f.URL
			break
		}
	}
	if wheelURL == "" {
		return nil, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wheelURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
	if err != nil {
		return nil, err
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	rawFiles := make(map[string][]byte)
	topLevelSet := make(map[string]bool)
	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, ".pyi") {
			continue
		}
		parts := strings.SplitN(f.Name, "/", 2)
		if len(parts) < 2 {
			continue
		}
		topLevelSet[parts[0]] = true
		rc, err := f.Open()
		if err != nil {
			continue
		}
		src, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}
		rawFiles[f.Name] = src
	}

	topLevelPkgs := make([]string, 0, len(topLevelSet))
	for pkg := range topLevelSet {
		topLevelPkgs = append(topLevelPkgs, pkg)
	}

	normalizedFiles, normalizedPkgs := normalizeStubFiles(rawFiles, topLevelPkgs)
	pkg := goopy.ExtractPackage(ctx, stubPkgName, normalizedFiles, normalizedPkgs)
	return pkg, nil
}
```

Also add `"github.com/pypx/goopy/model"` to the imports (needed for return type). Full imports for `stubs.go`:

```go
import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/pypx/api/internal/pypi"
	"github.com/pypx/goopy"
	"github.com/pypx/goopy/model"
)
```

- [ ] **Step 6: Verify it compiles**

```bash
cd api && go build ./internal/handler/
```

Expected: no errors

- [ ] **Step 7: Commit**

```bash
git add api/internal/handler/stubs.go api/internal/handler/stubs_test.go
git commit -m "feat: add stub wheel download, normalization, and extraction"
```

---

### Task 4: Stub index

**Files:**
- Modify: `api/internal/handler/stubs.go`
- Modify: `api/internal/handler/stubs_test.go`

The stub index is a flat map from qualified symbol name to `*model.Function`. Keys follow the pattern `"module.FunctionName"` for module-level functions and `"module.ClassName.methodName"` for class methods. This allows O(1) lookup when merging.

- [ ] **Step 1: Write failing tests**

Add to `stubs_test.go`:

```go
func TestBuildStubIndex(t *testing.T) {
	pkg := &model.Package{
		Modules: []*model.Module{
			{
				Name: "django.db.models",
				Functions: []*model.Function{
					{Name: "get_object_or_404"},
				},
				Classes: []*model.Class{
					{
						Name: "QuerySet",
						Methods: []*model.Function{
							{Name: "filter"},
							{Name: "exclude"},
						},
					},
				},
			},
		},
	}

	idx := buildStubIndex(pkg)

	if _, ok := idx["django.db.models.get_object_or_404"]; !ok {
		t.Error("buildStubIndex: missing module-level function key")
	}
	if _, ok := idx["django.db.models.QuerySet.filter"]; !ok {
		t.Error("buildStubIndex: missing class method key")
	}
	if _, ok := idx["django.db.models.QuerySet.exclude"]; !ok {
		t.Error("buildStubIndex: missing class method key")
	}
	if _, ok := idx["django.db.models.QuerySet"]; ok {
		t.Error("buildStubIndex: should not index class itself, only methods")
	}
}

func TestBuildStubIndex_NilPackage(t *testing.T) {
	idx := buildStubIndex(nil)
	if idx == nil {
		t.Error("buildStubIndex(nil) returned nil, want empty map")
	}
	if len(idx) != 0 {
		t.Errorf("buildStubIndex(nil) len = %d, want 0", len(idx))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd api && go test ./internal/handler/ -run TestBuildStubIndex -v
```

Expected: FAIL — `buildStubIndex` undefined

- [ ] **Step 3: Implement stubIndex and buildStubIndex**

Add to `api/internal/handler/stubs.go`:

```go
// stubIndex maps qualified symbol names to stub functions.
// Keys: "module.FuncName" for functions, "module.ClassName.methodName" for methods.
type stubIndex map[string]*model.Function

// buildStubIndex builds a flat lookup map from a parsed stub package.
func buildStubIndex(pkg *model.Package) stubIndex {
	idx := make(stubIndex)
	if pkg == nil {
		return idx
	}
	for _, mod := range pkg.Modules {
		for _, fn := range mod.Functions {
			idx[mod.Name+"."+fn.Name] = fn
		}
		for _, cls := range mod.Classes {
			for _, m := range cls.Methods {
				idx[mod.Name+"."+cls.Name+"."+m.Name] = m
			}
		}
	}
	return idx
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd api && go test ./internal/handler/ -run TestBuildStubIndex -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/handler/stubs.go api/internal/handler/stubs_test.go
git commit -m "feat: add stub index type and builder"
```

---

### Task 5: Merge logic in convertFunction / convertClass / convertToDocsResponse

**Files:**
- Modify: `api/internal/handler/docs.go`
- Modify: `api/internal/handler/docs_unit_test.go`

Context: `convertFunction` gains an optional `stubFn *model.Function` second parameter. When `stubFn` is non-nil, it fills type gaps: if a parameter has no type (after source annotation + docstring fallback), use the stub parameter's type; if the function has no return type, use the stub's return type. Source always wins — stubs only fill nil/empty slots. `convertClass` and `convertToDocsResponse` are updated to thread the stub index through.

- [ ] **Step 1: Write failing tests for merge behavior**

Add to `api/internal/handler/docs_unit_test.go`:

```go
func TestConvertFunction_StubTypeBackfill(t *testing.T) {
	// Source function has no type annotation; stub has type info.
	src := &model.Function{
		Name: "filter",
		Parameters: []*model.Parameter{
			{Name: "self", Kind: model.ParamPositionalOrKeyword},
			{Name: "kwargs", Kind: model.ParamVarKeyword},
		},
	}
	stub := &model.Function{
		Name: "filter",
		Parameters: []*model.Parameter{
			{Name: "self", Kind: model.ParamPositionalOrKeyword},
			{Name: "kwargs", Kind: model.ParamVarKeyword, Type: &model.TypeExpr{Raw: "Any"}},
		},
		Returns: &model.TypeExpr{Raw: "QuerySet[_QS]"},
	}

	sym := convertFunction(src, stub)

	if sym.Returns == nil {
		t.Fatal("convertFunction: Returns should be backfilled from stub")
	}
	if sym.Returns.Type != "QuerySet[_QS]" {
		t.Errorf("convertFunction: Returns.Type = %q, want %q", sym.Returns.Type, "QuerySet[_QS]")
	}
	// self has no type — stub has none either
	if sym.Parameters[0].Type != "" {
		t.Errorf("convertFunction: self.Type = %q, want \"\"", sym.Parameters[0].Type)
	}
	// kwargs filled from stub
	if sym.Parameters[1].Type != "Any" {
		t.Errorf("convertFunction: kwargs.Type = %q, want \"Any\"", sym.Parameters[1].Type)
	}
}

func TestConvertFunction_SourceAnnotationWinsOverStub(t *testing.T) {
	src := &model.Function{
		Name: "save",
		Parameters: []*model.Parameter{
			{Name: "self", Kind: model.ParamPositionalOrKeyword},
			{Name: "value", Kind: model.ParamPositionalOrKeyword, Type: &model.TypeExpr{Raw: "str"}},
		},
		Returns: &model.TypeExpr{Raw: "None"},
	}
	stub := &model.Function{
		Name: "save",
		Parameters: []*model.Parameter{
			{Name: "self", Kind: model.ParamPositionalOrKeyword},
			{Name: "value", Kind: model.ParamPositionalOrKeyword, Type: &model.TypeExpr{Raw: "Any"}},
		},
		Returns: &model.TypeExpr{Raw: "int"},
	}

	sym := convertFunction(src, stub)

	// Source annotation wins
	if sym.Parameters[1].Type != "str" {
		t.Errorf("convertFunction: value.Type = %q, want \"str\" (source wins)", sym.Parameters[1].Type)
	}
	if sym.Returns == nil || sym.Returns.Type != "None" {
		t.Errorf("convertFunction: Returns.Type = %q, want \"None\" (source wins)", sym.Returns.Type)
	}
}

func TestConvertFunction_NilStub(t *testing.T) {
	src := &model.Function{
		Name:       "simple",
		Parameters: []*model.Parameter{{Name: "x", Kind: model.ParamPositionalOrKeyword}},
	}
	// Should not panic with nil stub
	sym := convertFunction(src, nil)
	if sym.Name != "simple" {
		t.Errorf("convertFunction with nil stub: Name = %q, want \"simple\"", sym.Name)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd api && go test ./internal/handler/ -run "TestConvertFunction_Stub|TestConvertFunction_SourceAnnotation|TestConvertFunction_NilStub" -v
```

Expected: FAIL — `convertFunction` takes 1 argument, tests pass 2

- [ ] **Step 3: Update convertFunction signature and merge logic**

In `api/internal/handler/docs.go`, replace the `convertFunction` function (lines 190–241) with:

```go
func convertFunction(fn *model.Function, stubFn *model.Function) DocSymbol {
	sym := DocSymbol{
		Name:       fn.Name,
		Kind:       "function",
		Signature:  buildFuncSignature(fn),
		Docstring:  cleanDocstringText(fn.Docstring),
		Parameters: make([]DocParam, 0, len(fn.Parameters)),
	}

	// Build stub parameter lookup by name for O(1) access.
	var stubParams map[string]*model.Parameter
	if stubFn != nil && len(stubFn.Parameters) > 0 {
		stubParams = make(map[string]*model.Parameter, len(stubFn.Parameters))
		for _, sp := range stubFn.Parameters {
			stubParams[sp.Name] = sp
		}
	}

	for _, p := range fn.Parameters {
		dp := DocParam{Name: p.Name}
		if p.Type != nil {
			dp.Type = p.Type.Raw
		} else if p.DocParam != nil && p.DocParam.Type != "" {
			dp.Type = p.DocParam.Type
		} else if sp, ok := stubParams[p.Name]; ok && sp.Type != nil && sp.Type.Raw != "" {
			// No source annotation or docstring type — fall back to stub.
			dp.Type = sp.Type.Raw
		}
		if p.DocParam != nil {
			dp.Description = p.DocParam.Description
		}
		dp.Kind = string(p.Kind)
		dp.Default = p.Default
		sym.Parameters = append(sym.Parameters, dp)
	}

	// Returns: source annotation > docstring > stub.
	if fn.Returns != nil {
		r := &DocReturn{Type: fn.Returns.Raw}
		if fn.Docstring != nil && fn.Docstring.Returns != nil {
			r.Description = fn.Docstring.Returns.Description
		}
		sym.Returns = r
	} else if fn.Docstring != nil && fn.Docstring.Returns != nil && fn.Docstring.Returns.Type != "" {
		sym.Returns = &DocReturn{
			Type:        fn.Docstring.Returns.Type,
			Description: fn.Docstring.Returns.Description,
		}
	} else if stubFn != nil && stubFn.Returns != nil && stubFn.Returns.Raw != "" {
		sym.Returns = &DocReturn{Type: stubFn.Returns.Raw}
	}

	// Raises: populate from docstring (stubs don't carry raise docs).
	if fn.Docstring != nil && len(fn.Docstring.Raises) > 0 {
		sym.Raises = make([]DocRaise, 0, len(fn.Docstring.Raises))
		for _, r := range fn.Docstring.Raises {
			sym.Raises = append(sym.Raises, DocRaise{
				Type:        r.Type,
				Description: r.Description,
			})
		}
	}

	return sym
}
```

- [ ] **Step 4: Update convertClass to accept stubs and module name**

Replace `convertClass` (lines 243–259) with:

```go
func convertClass(cls *model.Class, stubs stubIndex, modName string) DocSymbol {
	sym := DocSymbol{
		Name:      cls.Name,
		Kind:      "class",
		Signature: buildClassSignature(cls),
		Docstring: cleanDocstringText(cls.Docstring),
	}

	if len(cls.Methods) > 0 {
		sym.Methods = make([]DocSymbol, 0, len(cls.Methods))
		for _, m := range cls.Methods {
			stubFn := stubs[modName+"."+cls.Name+"."+m.Name]
			sym.Methods = append(sym.Methods, convertFunction(m, stubFn))
		}
	}

	return sym
}
```

- [ ] **Step 5: Update convertToDocsResponse to accept and thread stubs**

Replace `convertToDocsResponse` (lines 148–188) with:

```go
func convertToDocsResponse(name, version string, pkg *model.Package, stubs stubIndex) DocsResponse {
	var modules []DocModule

	for _, mod := range pkg.Modules {
		dm := DocModule{
			Name:       mod.Name,
			Functions:  make([]DocSymbol, 0, len(mod.Functions)),
			Classes:    make([]DocSymbol, 0),
			Exceptions: make([]DocSymbol, 0),
		}

		for _, fn := range mod.Functions {
			stubFn := stubs[mod.Name+"."+fn.Name]
			dm.Functions = append(dm.Functions, convertFunction(fn, stubFn))
		}

		for _, cls := range mod.Classes {
			sym := convertClass(cls, stubs, mod.Name)
			if isException(cls) {
				sym.Kind = "exception"
				dm.Exceptions = append(dm.Exceptions, sym)
			} else {
				dm.Classes = append(dm.Classes, sym)
			}
		}

		if len(dm.Functions) > 0 || len(dm.Classes) > 0 || len(dm.Exceptions) > 0 {
			modules = append(modules, dm)
		}
	}

	if modules == nil {
		modules = []DocModule{}
	}

	return DocsResponse{
		Package:   name,
		Version:   version,
		Available: len(modules) > 0,
		Modules:   modules,
	}
}
```

- [ ] **Step 6: Fix the call site in Get() — pass empty stubs for now**

In `api/internal/handler/docs.go`, update the `Get` method call on line 126:

```go
docsResp := convertToDocsResponse(name, version, result, make(stubIndex))
```

This is a temporary placeholder so the code compiles; Task 6 will replace it with real stubs.

- [ ] **Step 7: Verify all tests pass**

```bash
cd api && go test ./internal/handler/ -v
```

Expected: all tests PASS (including the new stub merge tests and all existing tests)

- [ ] **Step 8: Commit**

```bash
git add api/internal/handler/docs.go api/internal/handler/docs_unit_test.go
git commit -m "feat: add stub merge logic to convertFunction, convertClass, convertToDocsResponse"
```

---

### Task 6: Wire stub enrichment into the handler

**Files:**
- Modify: `api/internal/handler/docs.go`

Context: Replace the `make(stubIndex)` placeholder from Task 5 with real stub fetching. After source extraction succeeds, look up the stub package, call `h.pypi.FetchPackage` for the stub, resolve the version, call `fetchStubPackage`, and build the stub index. Any failure is logged and silently skipped (serve source-only). The cache key is unchanged — the enriched result is stored under `docs:{name}:{version}`.

- [ ] **Step 1: Add log import if not present**

In `api/internal/handler/docs.go`, verify `"log"` is in the imports. If not, add it.

- [ ] **Step 2: Replace the placeholder call in Get()**

In `api/internal/handler/docs.go`, replace:

```go
docsResp := convertToDocsResponse(name, version, result, make(stubIndex))
```

with:

```go
stubs := resolveStubs(r.Context(), h.pypi, strings.ToLower(name), version)
docsResp := convertToDocsResponse(name, version, result, stubs)
```

- [ ] **Step 3: Implement resolveStubs**

Add this function to `api/internal/handler/docs.go` (after the `Get` method, before `convertToDocsResponse`):

```go
// resolveStubs fetches and parses a stub package for the given source package,
// returning a populated stubIndex. Returns an empty index if no stubs are
// available or if any step fails — stubs are best-effort enrichment only.
func resolveStubs(ctx context.Context, pypiClient *pypi.Client, pkgName, sourceVersion string) stubIndex {
	stubPkgName, ok := lookupStubPackage(pkgName)
	if !ok {
		return make(stubIndex)
	}

	stubPyPI, err := pypiClient.FetchPackage(stubPkgName)
	if err != nil {
		log.Printf("stubs: failed to fetch %s metadata: %v", stubPkgName, err)
		return make(stubIndex)
	}

	sourceMajor := strings.SplitN(sourceVersion, ".", 2)[0]
	stubVersion := resolveStubVersion(stubPyPI.Releases, sourceMajor)
	if stubVersion == "" {
		log.Printf("stubs: no releases found for %s", stubPkgName)
		return make(stubIndex)
	}

	stubPkg, err := fetchStubPackage(ctx, stubPkgName, stubPyPI.Releases, stubVersion)
	if err != nil {
		log.Printf("stubs: failed to extract %s@%s: %v", stubPkgName, stubVersion, err)
		return make(stubIndex)
	}

	return buildStubIndex(stubPkg)
}
```

- [ ] **Step 4: Add "context" and "log" to docs.go imports if missing**

The imports block in `docs.go` should include:

```go
import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/pypi"
	"github.com/pypx/goopy"
	"github.com/pypx/goopy/model"
)
```

- [ ] **Step 5: Build and verify**

```bash
cd api && go build ./...
```

Expected: no errors

- [ ] **Step 6: Run all handler tests**

```bash
cd api && go test ./internal/handler/ -v
```

Expected: all tests PASS

- [ ] **Step 7: Commit**

```bash
git add api/internal/handler/docs.go
git commit -m "feat: wire stub package enrichment into docs handler"
```

---

### Task 7: Manual verification

**No code changes — verification only.**

- [ ] **Step 1: Start the API server**

```bash
cd api && go run ./cmd/server
```

- [ ] **Step 2: Verify Django stubs enrich the response**

The Django docs cache must be warm for the specific version. Clear it first if needed:

```bash
# Check what version of Django is cached
curl -s http://localhost:8080/api/packages/django/docs | jq '.version'

# The stub lookup uses the cached version. If docs are cached from a prior run,
# the stubs won't have been applied. Flush by restarting with a fresh SQLite cache,
# or use a package whose cache entry is absent (e.g., a new version).
```

Test with a package that isn't cached:

```bash
curl -s http://localhost:8080/api/packages/django/docs | jq '
  .modules[] |
  select(.name | startswith("django.db.models")) |
  .classes[] |
  select(.name == "QuerySet") |
  .methods[:3]
'
```

Expected: methods show `type` fields populated from stubs (e.g. `filter` has `returns.type` of `QuerySet[_QS]` or similar).

- [ ] **Step 3: Verify DRF stubs**

```bash
curl -s http://localhost:8080/api/packages/djangorestframework/docs | jq '
  .modules[] |
  select(.name | startswith("rest_framework.serializers")) |
  .classes[] |
  select(.name == "Serializer") |
  .methods[:3]
'
```

Expected: methods show type info from `djangorestframework-stubs`.

- [ ] **Step 4: Verify source annotation still wins over stubs**

Find a method that has both a source annotation and a stub type — confirm the source type appears in the response.

- [ ] **Step 5: Run full test suite**

```bash
cd api && go test ./...
```

Expected: all tests PASS

- [ ] **Step 6: Final commit (if any fixes needed)**

```bash
git add -p
git commit -m "fix: <description of any manual-verification fixes>"
```
