// Package goopy extracts structured API documentation from Python source code.
package goopy

import (
	"context"
	"strings"

	"github.com/pypx/goopy/extractor"
	"github.com/pypx/goopy/model"
	"github.com/pypx/goopy/parser"
	"github.com/pypx/goopy/wheel"
)

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
func ExtractPackage(name string, files map[string][]byte, topLevelPkgs []string) *model.Package {
	pkg := &model.Package{Name: name}

	for _, pkgName := range topLevelPkgs {
		for path, src := range files {
			if belongsToPackage(path, pkgName) && !isPrivateModule(path) {
				modName := pathToModuleName(path)
				mod, _ := ExtractModule(modName, src) // errors are non-fatal for package extraction
				if hasContent(mod) {
					pkg.Modules = append(pkg.Modules, mod)
				}
			}
		}
	}

	return pkg
}

func belongsToPackage(path, pkgName string) bool {
	return strings.HasPrefix(path, pkgName+"/")
}

func pathToModuleName(path string) string {
	name := strings.TrimSuffix(path, ".py")
	name = strings.TrimSuffix(name, "/__init__")
	return strings.ReplaceAll(name, "/", ".")
}

// ExtractFromPyPI downloads a package wheel from PyPI and extracts documentation.
func ExtractFromPyPI(ctx context.Context, name, version string) (*model.Package, error) {
	src := wheel.NewSource()
	contents, err := src.Fetch(ctx, name, version)
	if err != nil {
		return nil, err
	}
	pkg := ExtractPackage(name, contents.Files, contents.TopLevelPkgs)
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
