// Package goopy extracts structured API documentation from Python source code.
package goopy

import (
	"context"

	"github.com/pypx/goopy/extractor"
	"github.com/pypx/goopy/model"
	"github.com/pypx/goopy/parser"
	"github.com/pypx/goopy/wheel"
)

// ExtractModule parses Python source code and returns structured documentation.
func ExtractModule(name string, src []byte) *model.Module {
	p := parser.New(src)
	mod := p.Parse()
	e := extractor.New()
	return e.ExtractModule(name, mod)
}

// ExtractPackage parses multiple Python source files and returns a Package.
// files maps relative paths (e.g., "mypackage/module.py") to source bytes.
func ExtractPackage(name string, files map[string][]byte, topLevelPkgs []string) *model.Package {
	pkg := &model.Package{Name: name}

	for _, pkgName := range topLevelPkgs {
		for path, src := range files {
			if belongsToPackage(path, pkgName) {
				modName := pathToModuleName(path)
				mod := ExtractModule(modName, src)
				if hasContent(mod) {
					pkg.Modules = append(pkg.Modules, mod)
				}
			}
		}
	}

	return pkg
}

func belongsToPackage(path, pkgName string) bool {
	return len(path) > len(pkgName) && path[:len(pkgName)] == pkgName &&
		path[len(pkgName)] == '/'
}

func pathToModuleName(path string) string {
	name := path
	if len(name) > 3 && name[len(name)-3:] == ".py" {
		name = name[:len(name)-3]
	}
	if len(name) > 9 && name[len(name)-9:] == "/__init__" {
		name = name[:len(name)-9]
	}
	result := make([]byte, len(name))
	for i := range name {
		if name[i] == '/' {
			result[i] = '.'
		} else {
			result[i] = name[i]
		}
	}
	return string(result)
}

// ExtractFromPyPI downloads a package wheel from PyPI and extracts documentation.
func ExtractFromPyPI(ctx context.Context, name, version string) (*model.Package, error) {
	src := wheel.NewSource()
	contents, err := src.Fetch(ctx, name, version)
	if err != nil {
		return nil, err
	}
	return ExtractPackage(name, contents.Files, contents.TopLevelPkgs), nil
}

func hasContent(mod *model.Module) bool {
	return len(mod.Functions) > 0 || len(mod.Classes) > 0 ||
		len(mod.Attributes) > 0 || len(mod.TypeAliases) > 0
}
