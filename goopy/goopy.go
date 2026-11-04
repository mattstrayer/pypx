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
	return extractPackage(ctx, name, files, topLevelPkgs, func(*model.Module) {})
}

// extractPackage is ExtractPackage with a step that runs on each module once it is
// parsed, before modules can reference each other.
func extractPackage(ctx context.Context, name string, files map[string][]byte, topLevelPkgs []string, afterParse func(*model.Module)) *model.Package {
	// Collect eligible files. Private modules are parsed so re-exports can reach
	// them, but they are never published.
	type work struct {
		modName string
		path    string
		src     []byte
	}
	var items []work
	testDirs := testFileDirs(files)
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
				if !isTestModule(path, testDirs) {
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

	results := make([]*model.Module, len(items))
	if len(items) <= parallelThreshold {
		// For small packages, skip goroutine overhead.
		for i, item := range items {
			if ctx.Err() != nil {
				break
			}
			results[i] = extractSafely(item.modName, item.src)
			items[i].src = nil // parsing copies what it keeps
			if results[i] != nil {
				afterParse(results[i])
			}
		}
	} else {
		// Parallel extraction with bounded workers.
		workers := runtime.GOMAXPROCS(0)
		if workers > len(items) {
			workers = len(items)
		}

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
					items[idx].src = nil // parsing copies what it keeps
					if results[idx] != nil {
						afterParse(results[idx])
					}
				}
			}()
		}
		wg.Wait()
	}

	r := &resolver{modules: make(map[string]*parsedModule, len(items))}
	var parsed []*parsedModule
	for i, mod := range results {
		if mod == nil {
			continue
		}
		pm := newParsedModule(mod, items[i].path)
		r.modules[mod.Name] = pm
		parsed = append(parsed, pm)
	}

	r.publishExports(parsed)

	pkg := &model.Package{Name: name}
	for _, pm := range parsed {
		if pm.public && hasContent(pm.mod) {
			pm.mod.ImportFroms = nil
			pm.mod.Exports = nil
			pm.mod.Hidden = model.Definitions{}
			pkg.Modules = append(pkg.Modules, pm.mod)
		}
	}
	return pkg
}

// maxImportDepth caps how many import hops one re-export may follow.
const maxImportDepth = 16

// definition is a function, class, attribute or type alias in a module.
// Exactly one of the pointers is set.
type definition struct {
	module string
	fn     *model.Function
	cls    *model.Class
	attr   *model.Attribute
	alias  *model.TypeAlias
}

// name returns the definition's own name.
func (d definition) name() string {
	switch {
	case d.fn != nil:
		return d.fn.Name
	case d.cls != nil:
		return d.cls.Name
	case d.attr != nil:
		return d.attr.Name
	}
	return d.alias.Name
}

// parsedModule is a module plus an index of what it defines. The index never
// includes copies, so one module's copies cannot change another's lookups.
type parsedModule struct {
	mod    *model.Module
	isPkg  bool
	public bool
	// defs holds functions, classes and type aliases. attrs holds attributes
	// whose name has no entry in defs.
	defs  map[string]definition
	attrs map[string]definition
	// order lists the keys of defs and attrs in a stable order.
	order []string
}

func newParsedModule(mod *model.Module, path string) *parsedModule {
	pm := &parsedModule{
		mod:    mod,
		isPkg:  strings.HasSuffix(path, "/__init__.py"),
		public: !isPrivateModule(path),
		defs:   map[string]definition{},
		attrs:  map[string]definition{},
	}
	addDef := func(name string, d definition) {
		if _, ok := pm.defs[name]; !ok {
			pm.defs[name] = d
			pm.order = append(pm.order, name)
		}
	}
	for _, group := range []*model.Definitions{
		{Functions: mod.Functions, Classes: mod.Classes, Attributes: mod.Attributes, TypeAliases: mod.TypeAliases},
		&mod.Hidden,
	} {
		for _, f := range group.Functions {
			addDef(f.Name, definition{module: mod.Name, fn: f})
		}
		for _, c := range group.Classes {
			addDef(c.Name, definition{module: mod.Name, cls: c})
		}
		for _, t := range group.TypeAliases {
			addDef(t.Name, definition{module: mod.Name, alias: t})
		}
	}
	for _, group := range [][]*model.Attribute{mod.Attributes, mod.Hidden.Attributes} {
		for _, a := range group {
			_, inDefs := pm.defs[a.Name]
			_, inAttrs := pm.attrs[a.Name]
			if !inDefs && !inAttrs {
				pm.attrs[a.Name] = definition{module: mod.Name, attr: a}
				pm.order = append(pm.order, a.Name)
			}
		}
	}
	return pm
}

func (pm *parsedModule) definesDirectly(name string) bool {
	_, inDefs := pm.defs[name]
	_, inAttrs := pm.attrs[name]
	return inDefs || inAttrs
}

// resolver follows imports between the modules of one wheel. Only modules in
// the wheel are in the table, so an import from another distribution finds
// nothing.
type resolver struct {
	modules map[string]*parsedModule
	// stars caches the names "from mod import *" binds, per module.
	stars map[string]map[string]bool
}

// importBinding returns the local name an import binds.
func importBinding(imp *model.Import) string {
	switch {
	case imp.Alias != "":
		return imp.Alias
	case imp.Name != "":
		return imp.Name
	}
	first, _, _ := strings.Cut(imp.Module, ".")
	return first
}

// importSource returns the dotted module an import reads from. For a relative
// import that climbs above the top of the wheel it returns "".
func importSource(pm *parsedModule, imp *model.Import) string {
	if imp.Level == 0 {
		return imp.Module
	}
	base := pm.mod.Name
	levels := imp.Level
	if pm.isPkg {
		levels-- // A package's own name is its level 1 anchor.
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

// refTarget returns the module that an "other.__all__" reference names, or ""
// if that module is not in the wheel.
func (r *resolver) refTarget(pm *parsedModule, ref string) string {
	first, rest, hasRest := strings.Cut(ref, ".")
	withRest := func(mod string) string {
		if hasRest {
			return mod + "." + rest
		}
		return mod
	}
	for _, imp := range pm.mod.ImportFroms {
		if imp.Name == "*" || importBinding(imp) != first {
			continue
		}
		var target string
		switch {
		case imp.Name != "":
			src := importSource(pm, imp)
			if src == "" {
				continue
			}
			target = src + "." + imp.Name
		case imp.Alias != "":
			target = imp.Module
		default:
			target = first
		}
		if _, ok := r.modules[withRest(target)]; ok {
			return withRest(target)
		}
	}
	// "from .a import *" also binds the submodule a in a package's namespace.
	if pm.isPkg {
		if _, ok := r.modules[pm.mod.Name+"."+ref]; ok {
			return pm.mod.Name + "." + ref
		}
	}
	if _, ok := r.modules[ref]; ok {
		return ref
	}
	return ""
}

// isPartialAll reports whether pm's __all__ has parts that cannot be read, so it
// cannot serve as the complete list of public names.
func (r *resolver) isPartialAll(pm *parsedModule) bool {
	ex := pm.mod.Exports
	if ex == nil {
		return false
	}
	if ex.Dynamic {
		return true
	}
	for _, ref := range ex.Refs {
		if r.refTarget(pm, ref) == "" {
			return true
		}
	}
	return false
}

// appendUnique appends the names in add that seen does not hold yet.
func appendUnique(names []string, seen map[string]bool, add ...string) []string {
	for _, n := range add {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	return names
}

// starNames returns the names "from mod import *" binds: __all__, or every
// public name in the module. visited guards against import cycles.
func (r *resolver) starNames(mod string, visited map[string]bool) []string {
	pm, ok := r.modules[mod]
	if !ok || visited[mod] {
		return nil
	}
	visited[mod] = true

	seen := map[string]bool{}
	var names []string
	if ex := pm.mod.Exports; ex != nil {
		names = appendUnique(names, seen, ex.Names...)
		for _, ref := range ex.Refs {
			if target := r.refTarget(pm, ref); target != "" {
				names = appendUnique(names, seen, r.starNames(target, visited)...)
			}
		}
		if !r.isPartialAll(pm) {
			return names
		}
	}
	for _, n := range pm.order {
		if !strings.HasPrefix(n, "_") {
			names = appendUnique(names, seen, n)
		}
	}
	for _, imp := range pm.mod.ImportFroms {
		switch {
		case imp.Name == "*":
			for _, n := range r.starNames(importSource(pm, imp), visited) {
				if !strings.HasPrefix(n, "_") {
					names = appendUnique(names, seen, n)
				}
			}
		case imp.Name != "":
			if b := importBinding(imp); !strings.HasPrefix(b, "_") {
				names = appendUnique(names, seen, b)
			}
		}
	}
	return names
}

// starSet is starNames as a set, computed once per module.
func (r *resolver) starSet(mod string) map[string]bool {
	if set, ok := r.stars[mod]; ok {
		return set
	}
	set := map[string]bool{}
	for _, n := range r.starNames(mod, map[string]bool{}) {
		set[n] = true
	}
	if r.stars == nil {
		r.stars = map[string]map[string]bool{}
	}
	r.stars[mod] = set
	return set
}

// lookup finds the definition behind name as seen from mod. A function, class or
// type alias in mod comes first. Next come imports, in source order. An attribute
// in mod comes last, because "try: from ._x import Y / except: Y = None" should
// find the class. The second return value is false if nothing is found.
func (r *resolver) lookup(mod, name string, visited map[[2]string]bool, depth int) (definition, bool) {
	pm, ok := r.modules[mod]
	key := [2]string{mod, name}
	if !ok || depth > maxImportDepth || visited[key] {
		return definition{}, false
	}
	visited[key] = true

	if d, ok := pm.defs[name]; ok {
		return d, true
	}
	for _, imp := range pm.mod.ImportFroms {
		if imp.Name == "" || imp.Name == "*" || importBinding(imp) != name {
			continue
		}
		if d, ok := r.lookup(importSource(pm, imp), imp.Name, visited, depth+1); ok {
			return d, true
		}
	}
	for _, imp := range pm.mod.ImportFroms {
		if imp.Name != "*" {
			continue
		}
		src := importSource(pm, imp)
		if r.starSet(src)[name] {
			if d, ok := r.lookup(src, name, visited, depth+1); ok {
				return d, true
			}
		}
	}
	d, ok := pm.attrs[name]
	return d, ok
}

// exportList returns the names public module pm publishes. With a complete
// __all__ that is every listed name. Otherwise it is the public names imported
// from private modules of the wheel, plus any names __all__ lists.
func (r *resolver) exportList(pm *parsedModule) []string {
	if ex := pm.mod.Exports; ex != nil && !r.isPartialAll(pm) {
		return r.starNames(pm.mod.Name, map[string]bool{})
	}
	seen := map[string]bool{}
	var names []string
	if ex := pm.mod.Exports; ex != nil {
		names = appendUnique(names, seen, ex.Names...)
		for _, ref := range ex.Refs {
			if target := r.refTarget(pm, ref); target != "" {
				names = appendUnique(names, seen, r.starNames(target, map[string]bool{})...)
			}
		}
	}
	for _, imp := range pm.mod.ImportFroms {
		if imp.Name == "" {
			continue
		}
		src := importSource(pm, imp)
		target, ok := r.modules[src]
		if !ok || target.public {
			continue
		}
		if imp.Name == "*" {
			for _, n := range r.starNames(src, map[string]bool{}) {
				if !strings.HasPrefix(n, "_") {
					names = appendUnique(names, seen, n)
				}
			}
		} else if b := importBinding(imp); !strings.HasPrefix(b, "_") {
			names = appendUnique(names, seen, b)
		}
	}
	return names
}

// settlePublic decides which of pm's own definitions are public. It returns the
// names pm exports.
func (r *resolver) settlePublic(pm *parsedModule) []string {
	names := r.exportList(pm)
	seen := make(map[string]bool, len(names))
	for _, n := range names {
		seen[n] = true
	}

	// The extractor could not judge names against a __all__ with references or
	// unreadable parts. Now the names are known.
	partial := r.isPartialAll(pm)
	m := pm.mod
	if ex := m.Exports; ex != nil && (ex.Dynamic || len(ex.Refs) > 0) {
		keep := func(n string) bool {
			return seen[n] || (partial && !strings.HasPrefix(n, "_"))
		}
		m.Functions, m.Hidden.Functions = repartition(m.Functions, m.Hidden.Functions, func(f *model.Function) string { return f.Name }, keep)
		m.Classes, m.Hidden.Classes = repartition(m.Classes, m.Hidden.Classes, func(c *model.Class) string { return c.Name }, keep)
		m.Attributes, m.Hidden.Attributes = repartition(m.Attributes, m.Hidden.Attributes, func(a *model.Attribute) string { return a.Name }, keep)
		m.TypeAliases, m.Hidden.TypeAliases = repartition(m.TypeAliases, m.Hidden.TypeAliases, func(t *model.TypeAlias) string { return t.Name }, keep)
	}

	// An attribute that shares its name with a working import is a fallback, as
	// in "try: from ._x import Y / except ImportError: Y = None". The import wins.
	for _, imp := range m.ImportFroms {
		if imp.Name == "" || imp.Name == "*" {
			continue
		}
		b := importBinding(imp)
		if _, isAttr := pm.attrs[b]; !isAttr {
			continue
		}
		d, ok := r.lookup(m.Name, b, map[[2]string]bool{}, 0)
		if !ok || d.module == m.Name {
			continue
		}
		delete(pm.attrs, b)
		isNamedAttr := func(a *model.Attribute) bool { return a.Name == b }
		m.Attributes = slices.DeleteFunc(m.Attributes, isNamedAttr)
		m.Hidden.Attributes = slices.DeleteFunc(m.Hidden.Attributes, isNamedAttr)
		if (m.Exports == nil || partial) && !strings.HasPrefix(b, "_") {
			names = appendUnique(names, seen, b)
		}
	}
	return names
}

// publishExports gives each definition one published location. A definition
// already public in its own module stays there, and every re-export becomes an
// alias. A private definition is copied once, into the exporting module that
// sorts first, and its other re-exports become aliases of that copy.
//
// parsed must be sorted by module name.
func (r *resolver) publishExports(parsed []*parsedModule) {
	type export struct {
		pm   *parsedModule
		name string
	}
	type identity struct{ module, name string }
	type group struct {
		d       definition
		exports []export
	}

	var names [][]string
	var public []*parsedModule
	for _, pm := range parsed {
		if pm.public {
			public = append(public, pm)
			names = append(names, r.settlePublic(pm))
		}
	}

	groups := map[identity]*group{}
	var order []identity
	for i, pm := range public {
		for _, name := range names[i] {
			if pm.definesDirectly(name) {
				continue
			}
			d, ok := r.lookup(pm.mod.Name, name, map[[2]string]bool{}, 0)
			if !ok {
				continue
			}
			id := identity{d.module, d.name()}
			g, ok := groups[id]
			if !ok {
				g = &group{d: d}
				groups[id] = g
				order = append(order, id)
			}
			g.exports = append(g.exports, export{pm, name})
		}
	}

	for _, id := range order {
		g := groups[id]
		target := id.module + "." + id.name
		copyIdx := -1
		if src := r.modules[id.module]; !src.public || !src.isPublished(id.name) {
			// The first export is in the module that sorts first. Inside that
			// module, prefer the definition's own name over a rename.
			copyIdx = 0
			for i, e := range g.exports {
				if e.pm != g.exports[0].pm {
					break
				}
				if e.name == id.name {
					copyIdx = i
					break
				}
			}
			e := g.exports[copyIdx]
			addCopy(e.pm.mod, e.name, g.d)
			target = e.pm.mod.Name + "." + e.name
		}
		for i, e := range g.exports {
			if i != copyIdx {
				e.pm.mod.Aliases = append(e.pm.mod.Aliases, &model.Alias{Name: e.name, Target: target})
			}
		}
	}
	for _, pm := range public {
		slices.SortFunc(pm.mod.Aliases, func(a, b *model.Alias) int { return strings.Compare(a.Name, b.Name) })
	}
}

// isPublished reports whether name is in the module's public lists.
func (pm *parsedModule) isPublished(name string) bool {
	m := pm.mod
	return slices.ContainsFunc(m.Functions, func(f *model.Function) bool { return f.Name == name }) ||
		slices.ContainsFunc(m.Classes, func(c *model.Class) bool { return c.Name == name }) ||
		slices.ContainsFunc(m.Attributes, func(a *model.Attribute) bool { return a.Name == name }) ||
		slices.ContainsFunc(m.TypeAliases, func(t *model.TypeAlias) bool { return t.Name == name })
}

// repartition splits public and hidden items again under a new keep rule.
func repartition[T any](public, hidden []T, name func(T) string, keep func(string) bool) (newPublic, newHidden []T) {
	for _, group := range [][]T{public, hidden} {
		for _, item := range group {
			if keep(name(item)) {
				newPublic = append(newPublic, item)
			} else {
				newHidden = append(newHidden, item)
			}
		}
	}
	return newPublic, newHidden
}

// addCopy appends a shallow copy of d to m under the exported name, so the source
// definition keeps its own name and an empty Origin.
func addCopy(m *model.Module, name string, d definition) {
	switch {
	case d.fn != nil:
		c := *d.fn
		c.Name, c.Origin = name, d.module
		if name != d.fn.Name {
			c.OriginName = d.fn.Name
		}
		m.Functions = append(m.Functions, &c)
	case d.cls != nil:
		c := *d.cls
		c.Name, c.Origin = name, d.module
		if name != d.cls.Name {
			c.OriginName = d.cls.Name
		}
		m.Classes = append(m.Classes, &c)
	case d.attr != nil:
		c := *d.attr
		c.Name, c.Origin = name, d.module
		m.Attributes = append(m.Attributes, &c)
	case d.alias != nil:
		c := *d.alias
		c.Name, c.Origin = name, d.module
		m.TypeAliases = append(m.TypeAliases, &c)
	}
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

// ExtractDocsFromPyPI is ExtractFromPyPI for a caller that reads a type expression
// only through its Raw text. Every other field of a TypeExpr except Kind is empty,
// which lets a package with many annotations fit in much less memory.
func ExtractDocsFromPyPI(ctx context.Context, name, version string) (*model.Package, error) {
	return extractWheel(ctx, wheel.NewSource(), name, version, flattenTypes)
}

// WeighFromPyPI estimates the bytes that ExtractFromPyPI holds in memory for this
// package, without downloading its sources. See wheel.Source.Weigh.
func WeighFromPyPI(ctx context.Context, name, version string) (int64, error) {
	return wheel.NewSource().Weigh(ctx, name, version)
}

// ExtractFromWheel downloads a package wheel using the given fetcher and extracts documentation.
// Use this with a custom wheel.Fetcher for testing or non-PyPI sources.
//
// Nothing here uses contents after ExtractPackage starts, so each source can be
// collected as soon as its module is parsed. The source of a large package would
// otherwise stay in memory next to everything parsed from it.
func ExtractFromWheel(ctx context.Context, fetcher wheel.Fetcher, name, version string) (*model.Package, error) {
	return extractWheel(ctx, fetcher, name, version, func(*model.Module) {})
}

func extractWheel(ctx context.Context, fetcher wheel.Fetcher, name, version string, afterParse func(*model.Module)) (*model.Package, error) {
	contents, err := fetcher.Fetch(ctx, name, version)
	if err != nil {
		return nil, err
	}
	pkg := extractPackage(ctx, name, contents.Files, contents.TopLevelPkgs, afterParse)
	pkg.Version = version
	return pkg, nil
}

// testFileDirs returns the directories that directly contain a test_*.py or
// *_test.py file.
func testFileDirs(files map[string][]byte) map[string]bool {
	dirs := map[string]bool{}
	for path := range files {
		i := strings.LastIndex(path, "/")
		if i < 0 {
			continue
		}
		base := path[i+1:]
		if strings.HasSuffix(base, ".py") && (strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py")) {
			dirs[path[:i]] = true
		}
	}
	return dirs
}

// isTestModule returns true for test code: a "tests" directory, a "test"
// directory that holds test files (testDirs), or a file named test_*.py,
// *_test.py, conftest.py, or tests.py. The first segment (the top-level
// package) is exempt. "test.py" and "testing" stay public because they are
// documented API (werkzeug.test, numpy.testing).
func isTestModule(path string, testDirs map[string]bool) bool {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if i == 0 {
			continue
		}
		if i < len(parts)-1 {
			if part == "tests" {
				return true
			}
			// "test" is public API in some packages (django.test), so it
			// counts as test code only when it directly holds test files.
			if part == "test" && testDirs[strings.Join(parts[:i+1], "/")] {
				return true
			}
			continue
		}
		switch {
		case part == "tests.py", part == "conftest.py",
			strings.HasPrefix(part, "test_") && strings.HasSuffix(part, ".py"),
			strings.HasSuffix(part, "_test.py"):
			return true
		}
	}
	return false
}

// isPrivateModule returns true if any path component (other than __init__.py)
// starts with underscore. Private modules are parsed but never published.
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
		mod.Docstring != nil || len(mod.Imports) > 0 || len(mod.Aliases) > 0
}
