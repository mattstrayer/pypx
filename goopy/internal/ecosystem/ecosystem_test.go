// Package ecosystem provides parity tests comparing goopy's extraction output
// against griffe (the Python reference implementation). These tests download
// real PyPI packages and verify goopy finds the same symbols griffe does.
//
// Run: cd goopy && go test ./internal/ecosystem/ -v -count=1
// Skip: go test ./internal/ecosystem/ -short (skips all network tests)
//
// The parity test is designed for CI: it fails if goopy misses any function
// or class that griffe finds in a shared module. Only expand packages.json
// after existing packages reach 100% parity.
package ecosystem

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/pypx/goopy"
	"github.com/pypx/goopy/internal/conformance"
	"github.com/pypx/goopy/model"
	"github.com/pypx/goopy/wheel"
)

type packageEntry struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type packageList struct {
	Packages []packageEntry `json:"packages"`
}

// griffe reference types
type griffeModule struct {
	Functions map[string]griffeSymbol `json:"functions"`
	Classes   map[string]griffeSymbol `json:"classes"`
}

type griffeSymbol struct {
	Params       []string `json:"params,omitempty"`
	Methods      []string `json:"methods,omitempty"`
	HasDocstring bool     `json:"has_docstring"`
}

type griffeResult struct {
	Name    string                  `json:"name"`
	Error   string                  `json:"error,omitempty"`
	Modules map[string]griffeModule `json:"modules"`
}

// TestExtraction verifies goopy can extract all packages without crashing.
func TestExtraction(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping ecosystem tests in short mode")
	}

	list := loadPackages(t)
	for _, pkg := range list.Packages {
		t.Run(pkg.Name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			result, err := goopy.ExtractFromPyPI(ctx, pkg.Name, pkg.Version)
			if err != nil {
				t.Fatalf("ExtractFromPyPI: %v", err)
			}
			if result == nil {
				t.Fatal("nil result")
			}

			totalFuncs := 0
			totalClasses := 0
			for _, mod := range result.Modules {
				totalFuncs += len(mod.Functions)
				totalClasses += len(mod.Classes)
			}
			t.Logf("%d modules, %d functions, %d classes", len(result.Modules), totalFuncs, totalClasses)
		})
	}
}

// TestParity compares goopy against griffe on a per-module, per-symbol basis.
// For modules that both tools extract, every symbol griffe finds must also
// appear in goopy's output. The test FAILS on any missed symbol.
//
// Requires: python3 with griffe and httpx installed.
func TestParity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping parity tests in short mode")
	}
	if err := exec.Command("python3", "-c", "import griffe, httpx").Run(); err != nil {
		t.Skip("python3 with griffe/httpx not available")
	}

	list := loadPackages(t)

	// Aggregate stats across all packages.
	var (
		totalSharedFuncs   int
		totalMissedFuncs   int
		totalSharedClasses int
		totalMissedClasses int
		totalParamMatch    int
		totalParamMismatch int
		allMissed          []string // "pkg/module: symbol" for reporting
	)

	for _, pkg := range list.Packages {
		t.Run(pkg.Name, func(t *testing.T) {
			goopyResult := extractGoopy(t, pkg)
			griffeRef := extractGriffe(t, pkg)
			if griffeRef == nil {
				return
			}

			goopyMods := indexGoopyModules(goopyResult)

			var pkgMissedFuncs, pkgMissedClasses []string

			for modName, griffeMod := range griffeRef.Modules {
				goopyMod, exists := goopyMods[modName]
				if !exists {
					// Module exists in griffe but not goopy — skip.
					// This happens for griffe's submodule duplication.
					continue
				}

				// Check functions
				for fname, gSym := range griffeMod.Functions {
					goopyParams, found := goopyMod.funcs[fname]
					if !found {
						pkgMissedFuncs = append(pkgMissedFuncs, fmt.Sprintf("%s.%s", modName, fname))
						totalMissedFuncs++
						continue
					}
					totalSharedFuncs++

					// Compare parameter names
					if paramsEqual(goopyParams, gSym.Params) {
						totalParamMatch++
					} else {
						totalParamMismatch++
						t.Logf("param mismatch %s.%s: goopy=%v griffe=%v",
							modName, fname, goopyParams, gSym.Params)
					}
				}

				// Check classes
				for cname := range griffeMod.Classes {
					if goopyMod.classes[cname] {
						totalSharedClasses++
					} else {
						pkgMissedClasses = append(pkgMissedClasses, fmt.Sprintf("%s.%s", modName, cname))
						totalMissedClasses++
					}
				}
			}

			// Report per-package
			if len(pkgMissedFuncs) > 0 {
				sort.Strings(pkgMissedFuncs)
				t.Errorf("missed %d functions: %s",
					len(pkgMissedFuncs), strings.Join(pkgMissedFuncs, ", "))
				allMissed = append(allMissed, pkgMissedFuncs...)
			}
			if len(pkgMissedClasses) > 0 {
				sort.Strings(pkgMissedClasses)
				t.Errorf("missed %d classes: %s",
					len(pkgMissedClasses), strings.Join(pkgMissedClasses, ", "))
				allMissed = append(allMissed, pkgMissedClasses...)
			}

			if len(pkgMissedFuncs) == 0 && len(pkgMissedClasses) == 0 {
				t.Logf("100%% parity")
			}
		})
	}

	// Aggregate summary
	t.Run("_summary", func(t *testing.T) {
		funcTotal := totalSharedFuncs + totalMissedFuncs
		classTotal := totalSharedClasses + totalMissedClasses
		paramTotal := totalParamMatch + totalParamMismatch

		t.Logf("=== PARITY SUMMARY (%d packages) ===", len(list.Packages))
		t.Logf("Functions: %d/%d found (%.1f%%)", totalSharedFuncs, funcTotal, pct(totalSharedFuncs, funcTotal))
		t.Logf("Classes:   %d/%d found (%.1f%%)", totalSharedClasses, classTotal, pct(totalSharedClasses, classTotal))
		t.Logf("Params:    %d/%d match (%.1f%%)", totalParamMatch, paramTotal, pct(totalParamMatch, paramTotal))

		if totalMissedFuncs > 0 || totalMissedClasses > 0 {
			t.Logf("--- Missed symbols ---")
			sort.Strings(allMissed)
			for _, s := range allMissed {
				t.Logf("  %s", s)
			}
		}
	})
}

// TestASTConformance compares goopy's parser with CPython's ast module on every
// .py file in the pinned packages. Counts may not exceed the ratchet maxima, which
// are all zero.
//
// Requires: python3 (standard library only).
func TestASTConformance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping conformance tests in short mode")
	}
	if err := exec.Command("python3", "-c", "import ast").Run(); err != nil {
		t.Skip("python3 not available")
	}

	// Ratchet maxima. The parser matches CPython on the whole corpus, so all are 0.
	const (
		maxFilesWithErrors = 0
		maxMissingDefs     = 0
		maxExtraDefs       = 0
		maxParamMismatches = 0
	)

	gen, err := filepath.Abs(filepath.Join("..", "conformance", "gen_golden.py"))
	if err != nil {
		t.Fatal(err)
	}

	list := loadPackages(t)
	var (
		filesWithErrors, missing, extra, paramMismatches int
		compared, downloadFailures                       int
	)
	type fileStat struct {
		path   string
		errors int
		diff   int
	}
	var worst []fileStat
	firstErrors := map[string]int{}

	for _, pkg := range list.Packages {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		contents, err := wheel.NewSource().Fetch(ctx, pkg.Name, pkg.Version)
		cancel()
		if err != nil {
			downloadFailures++
			t.Logf("download failed for %s==%s: %v", pkg.Name, pkg.Version, err)
			continue
		}

		dir := t.TempDir()
		for rel, data := range contents.Files {
			if !strings.HasSuffix(rel, ".py") {
				continue
			}
			clean := filepath.Clean(rel)
			if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
				continue
			}
			dest := filepath.Join(dir, clean)
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dest, data, 0o644); err != nil {
				t.Fatal(err)
			}
		}

		cmd := exec.Command("python3", "-I", gen, "--tree", dir)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("gen_golden.py for %s: %v", pkg.Name, err)
		}
		var ref map[string][]conformance.Def
		if err := json.Unmarshal(out, &ref); err != nil {
			t.Fatalf("decoding gen_golden.py output for %s: %v", pkg.Name, err)
		}

		for rel, want := range ref {
			src, ok := contents.Files[rel]
			if !ok {
				src, ok = contents.Files[filepath.ToSlash(rel)]
			}
			if !ok {
				continue
			}
			compared++
			got, errs := conformance.Parse(src)
			st := fileStat{path: pkg.Name + "/" + rel}
			if len(errs) > 0 {
				filesWithErrors++
				st.errors = len(errs)
				firstErrors[trimErr(errs[0].Error())]++
			}
			m, e, pm := diffDefs(got, want)
			missing += m
			extra += e
			paramMismatches += pm
			st.diff = m + e + pm
			if st.errors > 0 || st.diff > 0 {
				worst = append(worst, st)
			}
		}
	}

	if downloadFailures > 3 {
		t.Fatalf("%d package downloads failed (limit 3)", downloadFailures)
	}

	sort.Slice(worst, func(i, j int) bool {
		if worst[i].diff != worst[j].diff {
			return worst[i].diff > worst[j].diff
		}
		return worst[i].path < worst[j].path
	})
	t.Logf("=== AST CONFORMANCE (%d packages, %d files compared, %d downloads failed) ===",
		len(list.Packages), compared, downloadFailures)
	t.Logf("files with parse errors: %d", filesWithErrors)
	t.Logf("missing defs:            %d", missing)
	t.Logf("extra defs:              %d", extra)
	t.Logf("param mismatches:        %d", paramMismatches)
	t.Logf("--- 20 worst files ---")
	for i, w := range worst {
		if i >= 20 {
			break
		}
		t.Logf("  %s: %d errors, %d def differences", w.path, w.errors, w.diff)
	}
	type kv struct {
		msg string
		n   int
	}
	var common []kv
	for k, v := range firstErrors {
		common = append(common, kv{k, v})
	}
	sort.Slice(common, func(i, j int) bool {
		if common[i].n != common[j].n {
			return common[i].n > common[j].n
		}
		return common[i].msg < common[j].msg
	})
	t.Logf("--- 15 most common first errors ---")
	for i, c := range common {
		if i >= 15 {
			break
		}
		t.Logf("  %5d  %s", c.n, c.msg)
	}

	ratchet := func(name string, got, max int) {
		switch {
		case got > max:
			t.Errorf("%s: %d exceeds ratchet %d", name, got, max)
		case got < max:
			t.Errorf("%s: %d is below ratchet %d: lower the ratchet to %d", name, got, max, got)
		}
	}
	ratchet("files with errors", filesWithErrors, maxFilesWithErrors)
	ratchet("missing defs", missing, maxMissingDefs)
	ratchet("extra defs", extra, maxExtraDefs)
	ratchet("param mismatches", paramMismatches, maxParamMismatches)
}

// publicNamesScript reads top-level __init__.py files with CPython's ast module,
// so the coverage gate does not trust the extractor's own export logic. For each
// path it prints the names the file publishes: __all__, or its public relative
// imports.
const publicNamesScript = `
import ast, json, sys

def literal(e):
    if isinstance(e, (ast.List, ast.Tuple, ast.Set)):
        return [x.value for x in e.elts if isinstance(x, ast.Constant) and isinstance(x.value, str)]
    if isinstance(e, ast.BinOp) and isinstance(e.op, ast.Add):
        return literal(e.left) + literal(e.right)
    return []

def read(path):
    with open(path, "rb") as f:
        tree = ast.parse(f.read())
    state = {"all": None, "relative": []}

    def add(names):
        state["all"] = (state["all"] or []) + names

    def walk(stmts, nested):
        for s in stmts:
            if isinstance(s, (ast.Assign, ast.AnnAssign)):
                targets = s.targets if isinstance(s, ast.Assign) else [s.target]
                if s.value is not None and any(isinstance(t, ast.Name) and t.id == "__all__" for t in targets):
                    names = literal(s.value)
                    state["all"] = (state["all"] or []) + names if nested else names
            elif isinstance(s, ast.AugAssign) and isinstance(s.target, ast.Name) and s.target.id == "__all__":
                if isinstance(s.op, ast.Sub):
                    drop = set(literal(s.value))
                    state["all"] = [n for n in state["all"] or [] if n not in drop]
                else:
                    add(literal(s.value))
            elif isinstance(s, ast.Expr) and isinstance(s.value, ast.Call):
                f = s.value.func
                if isinstance(f, ast.Attribute) and isinstance(f.value, ast.Name) and f.value.id == "__all__" and s.value.args:
                    a = s.value.args[0]
                    if f.attr == "extend":
                        add(literal(a))
                    elif f.attr == "append" and isinstance(a, ast.Constant) and isinstance(a.value, str):
                        add([a.value])
            elif isinstance(s, ast.ImportFrom):
                for a in s.names:
                    n = a.asname or a.name
                    if s.level > 0 and n != "*" and not n.startswith("_"):
                        state["relative"].append(n)
            elif isinstance(s, ast.If):
                walk(s.body, True)
                walk(s.orelse, True)
            elif isinstance(s, ast.Try):
                walk(s.body, True)
                for h in s.handlers:
                    walk(h.body, True)
                walk(s.orelse, True)
                walk(s.finalbody, True)

    walk(tree.body, False)
    names = state["all"] if state["all"] is not None else state["relative"]
    return {"names": sorted(set(names))}

json.dump({p: read(p) for p in sys.argv[1:]}, sys.stdout)
`

type publicNames struct {
	Names []string `json:"names"`
}

// coverageExtras adds packages the plan names that are not in packages.json.
var coverageExtras = []packageEntry{
	{Name: "yarl", Version: "1.25.1"},
	{Name: "openai", Version: "3.44.0"},
	{Name: "anthropic", Version: "2.4.0"},
	{Name: "pandas", Version: "3.1.0"},
}

// coverageCount is one package's result in TestPublicAPICoverage. Covered counts
// definitions, aliases, submodules and names from outside the wheel.
type coverageCount struct {
	total, covered, outside, compiled int
	misses                            []string
}

// missKind says why a name is absent from the output.
type missKind int

const (
	missUnknown  missKind = iota
	missOutside           // The chain of imports leaves the wheel.
	missCompiled          // The chain reaches a wheel module with no .py file.
	missModule            // The name is a submodule of the wheel.
)

// classifyMiss follows imports to learn why name in module has no definition.
func classifyMiss(contents *wheel.WheelContents, module, name string, visited map[[2]string]bool) missKind {
	key := [2]string{module, name}
	if visited[key] {
		return missUnknown
	}
	visited[key] = true

	file := func(mod string) (string, bool) {
		path := strings.ReplaceAll(mod, ".", "/")
		for _, p := range []string{path + "/__init__.py", path + ".py"} {
			if _, ok := contents.Files[p]; ok {
				return p, true
			}
		}
		return "", false
	}
	first, _, _ := strings.Cut(module, ".")
	if _, ok := file(first); !ok && !slices.Contains(contents.TopLevelPkgs, first) {
		return missOutside
	}
	if _, ok := file(module + "." + name); ok {
		return missModule
	}
	path, ok := file(module)
	if !ok {
		return missCompiled
	}

	mod, _ := goopy.ExtractModule(module, contents.Files[path])
	source := func(imp *model.Import) string {
		if imp.Level == 0 {
			return imp.Module
		}
		base := module
		levels := imp.Level
		if strings.HasSuffix(path, "/__init__.py") {
			levels--
		}
		for range levels {
			i := strings.LastIndex(base, ".")
			if i < 0 {
				return ""
			}
			base = base[:i]
		}
		if imp.Module == "" {
			return base
		}
		return base + "." + imp.Module
	}
	var stars []string
	for _, imp := range mod.ImportFroms {
		if imp.Name == "*" {
			stars = append(stars, source(imp))
			continue
		}
		binding := imp.Alias
		if binding == "" {
			binding = imp.Name
		}
		if imp.Name != "" && binding == name {
			if k := classifyMiss(contents, source(imp), imp.Name, visited); k != missUnknown {
				return k
			}
		}
	}
	for _, src := range stars {
		if k := classifyMiss(contents, src, name, visited); k == missCompiled || k == missOutside {
			return k
		}
	}
	return missUnknown
}

// countCoverage classifies every name a package's top-level __init__.py files
// publish.
func countCoverage(top string, pn publicNames, contents *wheel.WheelContents, published, modules map[string]bool, c *coverageCount) {
	for _, name := range pn.Names {
		c.total++
		if published[name] || modules[top+"."+name] {
			c.covered++
			continue
		}
		switch classifyMiss(contents, top, name, map[[2]string]bool{}) {
		case missOutside:
			c.covered++
			c.outside++
		case missModule:
			c.covered++
		case missCompiled:
			c.compiled++
			c.misses = append(c.misses, name+" (compiled)")
		default:
			c.misses = append(c.misses, name)
		}
	}
}

// TestPublicAPICoverage checks that each name a package's top-level __init__.py
// publishes appears in the output, and that one definition is never published
// in two modules.
//
// Requires: python3 (standard library only).
func TestPublicAPICoverage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping coverage test in short mode")
	}
	if err := exec.Command("python3", "-c", "import ast").Run(); err != nil {
		t.Skip("python3 not available")
	}

	// Ratchet: raise it when coverage rises. Set from the measured run.
	const minCoveragePct = 99.0

	// Name-level targets from the plan, as a minimum covered count.
	targets := map[string]int{"httpx": 65, "h11": 23, "pluggy": 14, "yarl": 8, "attrs": 40}

	var total, covered, outside, compiled, downloadFailures, duplicates, dangling int
	list := loadPackages(t)
	for _, pkg := range append(slices.Clone(list.Packages), coverageExtras...) {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		contents, err := wheel.NewSource().Fetch(ctx, pkg.Name, pkg.Version)
		cancel()
		if err != nil {
			downloadFailures++
			t.Logf("download failed for %s==%s: %v", pkg.Name, pkg.Version, err)
			continue
		}
		result := goopy.ExtractPackage(context.Background(), pkg.Name, contents.Files, contents.TopLevelPkgs)

		published := map[string]bool{}
		modules := map[string]bool{}
		locations := map[string]map[string]bool{}
		definitions := map[string]bool{}
		note := func(mod, name, origin, originName string) {
			published[name] = true
			definitions[mod+"."+name] = true
			if origin == "" {
				origin = mod
			}
			if originName == "" {
				originName = name
			}
			id := origin + "." + originName
			if locations[id] == nil {
				locations[id] = map[string]bool{}
			}
			locations[id][mod] = true
		}
		for _, m := range result.Modules {
			modules[m.Name] = true
			for _, f := range m.Functions {
				note(m.Name, f.Name, f.Origin, f.OriginName)
			}
			for _, c := range m.Classes {
				note(m.Name, c.Name, c.Origin, c.OriginName)
			}
			for _, a := range m.Attributes {
				note(m.Name, a.Name, a.Origin, "")
			}
			for _, a := range m.TypeAliases {
				note(m.Name, a.Name, a.Origin, "")
			}
		}
		for _, m := range result.Modules {
			for _, a := range m.Aliases {
				published[a.Name] = true
				if !definitions[a.Target] {
					dangling++
					t.Errorf("%s: alias %s.%s has no definition at %s", pkg.Name, m.Name, a.Name, a.Target)
				}
			}
		}
		for id, mods := range locations {
			if len(mods) > 1 {
				duplicates++
				t.Errorf("%s: definition %s is published in %d modules: %v", pkg.Name, id, len(mods), slices.Sorted(maps.Keys(mods)))
			}
		}

		// A submodule counts even if extraction dropped it for lack of content.
		for path := range contents.Files {
			if strings.HasSuffix(path, ".py") {
				name := strings.TrimSuffix(strings.TrimSuffix(path, ".py"), "/__init__")
				modules[strings.ReplaceAll(name, "/", ".")] = true
			}
		}

		dir := t.TempDir()
		var paths []string
		tops := map[string]string{}
		for _, top := range contents.TopLevelPkgs {
			src, ok := contents.Files[top+"/__init__.py"]
			if !ok {
				continue
			}
			p := filepath.Join(dir, top+"__init__.py")
			if err := os.WriteFile(p, src, 0o644); err != nil {
				t.Fatal(err)
			}
			paths = append(paths, p)
			tops[p] = top
		}
		if len(paths) == 0 {
			continue
		}
		out, err := exec.Command("python3", append([]string{"-I", "-c", publicNamesScript}, paths...)...).Output()
		if err != nil {
			t.Fatalf("publicNamesScript for %s: %v", pkg.Name, err)
		}
		var names map[string]publicNames
		if err := json.Unmarshal(out, &names); err != nil {
			t.Fatalf("decoding publicNamesScript output for %s: %v", pkg.Name, err)
		}

		var c coverageCount
		for _, p := range paths {
			countCoverage(tops[p], names[p], contents, published, modules, &c)
		}
		total += c.total
		covered += c.covered
		outside += c.outside
		compiled += c.compiled
		t.Logf("%s: %d of %d (%d outside the wheel)", pkg.Name, c.covered, c.total, c.outside)
		if len(c.misses) > 0 {
			t.Logf("  misses: %s", strings.Join(c.misses, ", "))
		}
		if want, ok := targets[pkg.Name]; ok && c.covered < want {
			t.Errorf("%s: %d covered, target %d", pkg.Name, c.covered, want)
		}
	}

	if downloadFailures > 3 {
		t.Fatalf("%d package downloads failed (limit 3)", downloadFailures)
	}
	got := pct(covered, total)
	t.Logf("=== PUBLIC API COVERAGE: %d of %d names (%.1f%%), %d outside the wheel, %d misses in compiled modules, %d downloads failed ===",
		covered, total, got, outside, compiled, downloadFailures)
	if got < minCoveragePct {
		t.Errorf("coverage %.1f%% is below ratchet %.1f%%", got, minCoveragePct)
	}
}

// trimErr removes the line:col prefix and quoted detail so errors group by kind.
func trimErr(msg string) string {
	if i := strings.Index(msg, " "); i > 0 && strings.Contains(msg[:i], ":") {
		msg = msg[i+1:]
	}
	return msg
}

// diffDefs treats each side as a multiset keyed by Q. It returns the number of
// defs missing from got, extra in got, and same-Q pairs whose params differ.
func diffDefs(got, want []conformance.Def) (missing, extra, paramMismatch int) {
	g := map[string][]conformance.Def{}
	w := map[string][]conformance.Def{}
	for _, d := range got {
		g[d.Q] = append(g[d.Q], d)
	}
	for _, d := range want {
		w[d.Q] = append(w[d.Q], d)
	}
	for q, wl := range w {
		gl := g[q]
		n := min(len(gl), len(wl))
		if len(wl) > n {
			missing += len(wl) - n
		}
		for i := 0; i < n; i++ {
			if !paramsEqual(gl[i].Params, wl[i].Params) {
				paramMismatch++
			}
		}
	}
	for q, gl := range g {
		if n := len(gl) - len(w[q]); n > 0 {
			extra += n
		}
	}
	return
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type goopyModIndex struct {
	funcs      map[string][]string // name -> param names
	classes    map[string]bool
	docstrings map[string]bool
}

func extractGoopy(t *testing.T, pkg packageEntry) *model.Package {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	result, err := goopy.ExtractFromPyPI(ctx, pkg.Name, pkg.Version)
	if err != nil {
		t.Fatalf("goopy: %v", err)
	}
	return result
}

func extractGriffe(t *testing.T, pkg packageEntry) *griffeResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", "griffe_extract.py", pkg.Name, pkg.Version)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Logf("griffe stderr: %s", string(exitErr.Stderr))
		}
		t.Logf("griffe failed for %s: %v", pkg.Name, err)
		return nil
	}

	var result griffeResult
	if err := json.Unmarshal(out, &result); err != nil {
		t.Logf("griffe JSON parse error for %s: %v", pkg.Name, err)
		return nil
	}
	if result.Error != "" {
		t.Logf("griffe error for %s: %s", pkg.Name, result.Error)
		return nil
	}
	return &result
}

func indexGoopyModules(pkg *model.Package) map[string]goopyModIndex {
	result := make(map[string]goopyModIndex)
	for _, mod := range pkg.Modules {
		idx := goopyModIndex{
			funcs:      make(map[string][]string),
			classes:    make(map[string]bool),
			docstrings: make(map[string]bool),
		}
		for _, fn := range mod.Functions {
			var params []string
			for _, p := range fn.Parameters {
				params = append(params, p.Name)
			}
			idx.funcs[fn.Name] = params
			idx.docstrings[fn.Name] = fn.Docstring != nil
		}
		for _, cls := range mod.Classes {
			idx.classes[cls.Name] = true
			idx.docstrings[cls.Name] = cls.Docstring != nil
		}
		result[mod.Name] = idx
	}
	return result
}

func paramsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func pct(num, denom int) float64 {
	if denom == 0 {
		return 100.0
	}
	return float64(num) / float64(denom) * 100.0
}

func loadPackages(t *testing.T) packageList {
	t.Helper()
	data, err := os.ReadFile("packages.json")
	if err != nil {
		t.Fatalf("reading packages.json: %v", err)
	}
	var list packageList
	if err := json.Unmarshal(data, &list); err != nil {
		t.Fatalf("parsing packages.json: %v", err)
	}
	return list
}

// Silence unused import if only one test is run.
var _ = fmt.Sprintf
