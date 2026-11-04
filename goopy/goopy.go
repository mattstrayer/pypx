// Package goopy extracts structured API documentation from Python source code.
package goopy

import (
	"context"
	"log"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"sync"

	"github.com/pypx/goopy/extractor"
	"github.com/pypx/goopy/model"
	"github.com/pypx/goopy/parser"
	"github.com/pypx/goopy/wheel"
)

// ExtractorVersion identifies the output of this extractor. Bump it in any change
// that alters extraction output, so cached results are regenerated.
const ExtractorVersion = 1

// parallelThreshold is the minimum number of files needed to use goroutine
// parallelism. Below this threshold, serial extraction avoids goroutine overhead.
const parallelThreshold = 4

// ExtractModule parses Python source code and returns structured documentation.
// The returned Module may be partial if the source contains syntax errors.
// Parse errors are returned as the second value but do not prevent extraction.
func ExtractModule(name string, src []byte) (*model.Module, []error) {
	p := parser.New(src)
	mod := p.Parse()
	e := extractor.New()
	result := e.ExtractModule(name, mod)

	var errs []error
	for _, pe := range p.Errors() {
		errs = append(errs, pe)
	}
	return result, errs
}

// ExtractPackage parses multiple Python source files and returns a Package.
// files maps relative paths (e.g., "mypackage/module.py") to source bytes.
// Module extraction is parallelized across available CPUs.
func ExtractPackage(ctx context.Context, name string, files map[string][]byte, topLevelPkgs []string) *model.Package {
	// Collect eligible files.
	type work struct {
		modName string
		path    string
		src     []byte
	}
	var items []work
	for _, pkgName := range topLevelPkgs {
		dirPrefix := pkgName + "/"
		hasDir := false
		for path := range files {
			if strings.HasPrefix(path, dirPrefix) {
				hasDir = true
				break
			}
		}
		for path, src := range files {
			switch {
			case strings.HasPrefix(path, dirPrefix):
				if !isPrivateModule(path) && !isTestModule(path) {
					items = append(items, work{modName: pathToModuleName(path), path: path, src: src})
				}
			case !hasDir && path == pkgName+".py":
				// Single-file module distribution (e.g. six.py).
				items = append(items, work{modName: pathToModuleName(path), path: path, src: src})
			}
		}
	}

	// Map iteration order is random. Sort by module name so output is stable
	// and a package root ("pkg") precedes its submodules ("pkg.sub").
	// If "foo.py" and "foo/__init__.py" both exist they share a module name:
	// keep the package directory's file, which Python's import system prefers.
	slices.SortFunc(items, func(a, b work) int {
		if c := strings.Compare(a.modName, b.modName); c != 0 {
			return c
		}
		aInit, bInit := strings.HasSuffix(a.path, "/__init__.py"), strings.HasSuffix(b.path, "/__init__.py")
		if aInit != bInit {
			if aInit {
				return -1
			}
			return 1
		}
		return strings.Compare(a.path, b.path)
	})
	items = slices.CompactFunc(items, func(a, b work) bool { return a.modName == b.modName })

	// For small packages, skip goroutine overhead.
	if len(items) <= parallelThreshold {
		pkg := &model.Package{Name: name}
		for _, item := range items {
			if ctx.Err() != nil {
				break
			}
			if mod := extractSafely(item.modName, item.src); mod != nil && hasContent(mod) {
				pkg.Modules = append(pkg.Modules, mod)
			}
		}
		return pkg
	}

	// Parallel extraction with bounded workers.
	workers := runtime.GOMAXPROCS(0)
	if workers > len(items) {
		workers = len(items)
	}

	results := make([]*model.Module, len(items))
	var wg sync.WaitGroup
	ch := make(chan int, len(items))

	for i := range items {
		ch <- i
	}
	close(ch)

	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for idx := range ch {
				if ctx.Err() != nil {
					return
				}
				results[idx] = extractSafely(items[idx].modName, items[idx].src)
			}
		}()
	}
	wg.Wait()

	pkg := &model.Package{Name: name}
	for _, mod := range results {
		if mod != nil && hasContent(mod) {
			pkg.Modules = append(pkg.Modules, mod)
		}
	}
	return pkg
}

// extractModuleFn is the extraction call used by extractSafely. Tests replace it.
var extractModuleFn = ExtractModule

// extractSafely extracts one module and returns nil if extraction panics, so a
// single bad module cannot stop a worker or lose other modules.
func extractSafely(modName string, src []byte) (mod *model.Module) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("goopy: extract panic on %s: %v\n%s", modName, r, debug.Stack())
			mod = nil
		}
	}()
	mod, _ = extractModuleFn(modName, src)
	return mod
}

func pathToModuleName(path string) string {
	name := strings.TrimSuffix(path, ".py")
	name = strings.TrimSuffix(name, "/__init__")
	return strings.ReplaceAll(name, "/", ".")
}

// ExtractFromPyPI downloads a package wheel from PyPI and extracts documentation.
func ExtractFromPyPI(ctx context.Context, name, version string) (*model.Package, error) {
	return ExtractFromWheel(ctx, wheel.NewSource(), name, version)
}

// ExtractFromWheel downloads a package wheel using the given fetcher and extracts documentation.
// Use this with a custom wheel.Fetcher for testing or non-PyPI sources.
func ExtractFromWheel(ctx context.Context, fetcher wheel.Fetcher, name, version string) (*model.Package, error) {
	contents, err := fetcher.Fetch(ctx, name, version)
	if err != nil {
		return nil, err
	}
	pkg := ExtractPackage(ctx, name, contents.Files, contents.TopLevelPkgs)
	pkg.Version = version
	return pkg, nil
}

// isTestModule returns true for test code: any path segment after the first
// named tests or test, or a file named test_*.py, *_test.py, or conftest.py.
// The first segment (the top-level package) is exempt. A "testing" package
// stays public because it is often documented API (e.g. numpy.testing).
func isTestModule(path string) bool {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if i == 0 {
			continue
		}
		if i < len(parts)-1 {
			if part == "tests" || part == "test" {
				return true
			}
			continue
		}
		switch {
		case part == "tests.py", part == "test.py", part == "conftest.py",
			strings.HasPrefix(part, "test_") && strings.HasSuffix(part, ".py"),
			strings.HasSuffix(part, "_test.py"):
			return true
		}
	}
	return false
}

// isPrivateModule returns true if any path component (other than __init__.py)
// starts with underscore. Private modules are excluded from extraction.
func isPrivateModule(path string) bool {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if i == 0 {
			continue // skip the top-level package dir
		}
		// __init__.py is not private
		if part == "__init__.py" {
			continue
		}
		if len(part) > 0 && part[0] == '_' {
			return true
		}
	}
	return false
}

func hasContent(mod *model.Module) bool {
	return len(mod.Functions) > 0 || len(mod.Classes) > 0 ||
		len(mod.Attributes) > 0 || len(mod.TypeAliases) > 0 ||
		mod.Docstring != nil || len(mod.Imports) > 0
}
