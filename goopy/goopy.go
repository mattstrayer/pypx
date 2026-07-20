// Package goopy extracts structured API documentation from Python source code.
package goopy

import (
	"context"
	"log"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/pypx/goopy/extractor"
	"github.com/pypx/goopy/model"
	"github.com/pypx/goopy/parser"
	"github.com/pypx/goopy/wheel"
)

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
				if !isPrivateModule(path) {
					items = append(items, work{modName: pathToModuleName(path), src: src})
				}
			case !hasDir && path == pkgName+".py":
				// Single-file module distribution (e.g. six.py).
				items = append(items, work{modName: pathToModuleName(path), src: src})
			}
		}
	}

	// For small packages, skip goroutine overhead.
	if len(items) <= parallelThreshold {
		pkg := &model.Package{Name: name}
		for _, item := range items {
			if ctx.Err() != nil {
				break
			}
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("goopy: serial extract panic on %s: %v\n%s", item.modName, r, debug.Stack())
					}
				}()
				mod, _ := ExtractModule(item.modName, item.src)
				if hasContent(mod) {
					pkg.Modules = append(pkg.Modules, mod)
				}
			}()
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
			defer func() {
				if r := recover(); r != nil {
					log.Printf("goopy: worker panic: %v\n%s", r, debug.Stack())
				}
			}()
			for idx := range ch {
				if ctx.Err() != nil {
					return
				}
				mod, _ := ExtractModule(items[idx].modName, items[idx].src)
				results[idx] = mod
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
