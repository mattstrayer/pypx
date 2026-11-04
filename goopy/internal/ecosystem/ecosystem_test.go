// Package ecosystem provides parity tests comparing goopy's extraction output
// against griffe (the Python reference implementation). These tests download
// real PyPI packages and verify goopy finds the same symbols griffe does.
//
// Run: cd goopy && go test ./ecosystem/ -v -count=1
// Skip: go test ./ecosystem/ -short (skips all network tests)
//
// The parity test is designed for CI: it fails if goopy misses any function
// or class that griffe finds in a shared module. Only expand packages.json
// after existing packages reach 100% parity.
package ecosystem

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

			// Filter out known gaps: symbols in private modules where cascading
			// parse errors from complex body expressions prevent extraction.
			// These are tracked here so adding new packages only passes when
			// they reach true 100% parity.
			// Known gaps: symbols in modules where cascading parse errors from
			// complex body expressions prevent full extraction. Each gap is
			// tracked here — when the parser improves, remove entries and the
			// test will enforce the fix.
			knownGaps := map[string]bool{
				// _pytest.reports: tuple unpacking in method bodies cascades
				"_pytest.reports.pytest_report_from_serializable": true,
				"_pytest.reports.pytest_report_to_serializable":   true,
				"_pytest.reports.CollectErrorRepr":                true,
				"_pytest.reports.CollectReport":                   true,
				// pygments.cmdline: `not in` inside generator expr in call args
				"pygments.cmdline.main":          true,
				"pygments.cmdline.HelpFormatter": true,
			}
			pkgMissedFuncs = filterKnown(pkgMissedFuncs, knownGaps)
			pkgMissedClasses = filterKnown(pkgMissedClasses, knownGaps)

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

func filterKnown(items []string, known map[string]bool) []string {
	var result []string
	for _, item := range items {
		if !known[item] {
			result = append(result, item)
		}
	}
	return result
}

// Silence unused import if only one test is run.
var _ = fmt.Sprintf
