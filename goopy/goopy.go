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
	"time"

	"github.com/pypx/goopy/extractor"
	"github.com/pypx/goopy/model"
	"github.com/pypx/goopy/parser"
	"github.com/pypx/goopy/wheel"
)

// ExtractorVersion identifies the output of this extractor. Bump it in any change
// that alters extraction output, so cached results are regenerated.
const ExtractorVersion = 4

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
// files maps relative paths (e.g., "mypackage/module.py") to source bytes. A .pyi
// file merges with the .py of the same module, or stands in for one.
// Module extraction is parallelized across available CPUs.
func ExtractPackage(ctx context.Context, name string, files map[string][]byte, topLevelPkgs []string) *model.Package {
	return extractPackage(ctx, name, files, topLevelPkgs, func(*model.Module) {})
}

// extractPackage is ExtractPackage with a step that runs on each module once it is
// parsed, before modules can reference each other.
func extractPackage(ctx context.Context, name string, files map[string][]byte, topLevelPkgs []string, afterParse func(*model.Module)) *model.Package {
	start := time.Now()
	items := collectWork(files, topLevelPkgs)

	results := make([]*model.Module, len(items))
	// Each worker writes only its own index, so no lock is needed.
	counts := make([]fileCounts, len(items))
	if len(items) <= parallelThreshold {
		// For small packages, skip goroutine overhead.
		for i, item := range items {
			if ctx.Err() != nil {
				break
			}
			results[i], counts[i] = item.extract()
			items[i].src, items[i].stub = nil, nil // parsing copies what it keeps
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
					results[idx], counts[idx] = items[idx].extract()
					items[idx].src, items[idx].stub = nil, nil // parsing copies what it keeps
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

	r.settleStubDefinitions(parsed)
	r.resolveConstructors(parsed)
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
	pkg.Stats.Modules = len(items)
	for _, c := range counts {
		c.addTo(&pkg.Stats)
	}
	pkg.Stats.Duration = time.Since(start)
	return pkg
}

// work is one module to extract: a .py, a .pyi, or both.
type work struct {
	modName string
	// path is the file's path in the import tree, as a .py file.
	path            string
	src, stub       []byte
	hasSrc, hasStub bool
}

// fileCounts is what one module's extraction saw, for model.Stats.
type fileCounts struct {
	files, filesWithErrors, errors, panicked int
}

// add records one file's outcome.
func (c *fileCounts) add(parseErrs int, panicked bool) {
	c.files++
	c.errors += parseErrs
	if parseErrs > 0 {
		c.filesWithErrors++
	}
	if panicked {
		c.panicked++
	}
}

func (c fileCounts) addTo(s *model.Stats) {
	s.Files += c.files
	s.FilesWithErrors += c.filesWithErrors
	s.Errors += c.errors
	s.PanickedModules += c.panicked
}

// extract parses the module's files, each once, and merges them.
func (w work) extract() (*model.Module, fileCounts) {
	var base, stub *model.Module
	var c fileCounts
	if w.hasSrc {
		var n int
		var panicked bool
		base, n, panicked = extractSafely(w.modName, w.src)
		c.add(n, panicked)
	}
	if w.hasStub {
		var n int
		var panicked bool
		stub, n, panicked = extractSafely(w.modName, w.stub)
		c.add(n, panicked)
	}
	merged, mergePanicked := safely(w.modName, func() *model.Module {
		if stub == nil {
			return base
		}
		extractor.PrepareStub(stub)
		if base == nil {
			return stub
		}
		extractor.MergeStub(base, stub)
		return base
	})
	if mergePanicked {
		c.panicked++
	}
	return merged, c
}

// sourcePath returns the path a file has in the import tree, as a .py file:
// "foo-stubs/bar.pyi" is "foo/bar.py". Every rule for module names, test code and
// private modules reads this form, so it holds for stubs too.
func sourcePath(path string) string {
	if !strings.HasSuffix(path, ".pyi") {
		return path
	}
	path = strings.TrimSuffix(path, "i")
	if first, rest, ok := strings.Cut(path, "/"); ok {
		return strings.TrimSuffix(first, "-stubs") + "/" + rest
	}
	return path
}

// collectWork picks the files to extract, sorted by module name so output is stable
// and a package root ("pkg") precedes its submodules ("pkg.sub"). Private modules
// are collected so re-exports can reach them, but they are never published.
func collectWork(files map[string][]byte, topLevelPkgs []string) []work {
	type candidate struct {
		modName, path, raw string
	}
	var cands []candidate
	testDirs := testFileDirs(files)
	for _, pkgName := range topLevelPkgs {
		dirPrefix := pkgName + "/"
		hasDir := false
		for raw := range files {
			if strings.HasPrefix(raw, dirPrefix) {
				hasDir = true
				break
			}
		}
		for raw := range files {
			path := sourcePath(raw)
			switch {
			case strings.HasPrefix(path, dirPrefix):
				if isTestModule(path, testDirs) {
					continue
				}
			case hasDir || path != pkgName+".py":
				// Only a single-file module distribution (six.py) lives outside
				// its package directory.
				continue
			}
			cands = append(cands, candidate{pathToModuleName(path), path, raw})
		}
	}

	// If "foo.py" and "foo/__init__.py" both exist they share a module name: the
	// package directory's file sorts first, which Python's import system prefers.
	// "foo-stubs/x.pyi" sorts before "foo/x.pyi", and PEP 561 prefers it too.
	isInit := func(path string) bool { return strings.HasSuffix(path, "/__init__.py") }
	slices.SortFunc(cands, func(a, b candidate) int {
		if c := strings.Compare(a.modName, b.modName); c != 0 {
			return c
		}
		if aInit, bInit := isInit(a.path), isInit(b.path); aInit != bInit {
			if aInit {
				return -1
			}
			return 1
		}
		return strings.Compare(a.raw, b.raw)
	})

	byName := map[string]*work{}
	var mods []*work
	for _, c := range cands {
		if c.raw == c.path && byName[c.modName] == nil {
			w := &work{modName: c.modName, path: c.path, src: files[c.raw], hasSrc: true}
			byName[c.modName] = w
			mods = append(mods, w)
		}
	}
	for _, c := range cands {
		if c.raw == c.path {
			continue
		}
		w := byName[c.modName]
		if w == nil {
			w = &work{modName: c.modName, path: c.path}
			byName[c.modName] = w
			mods = append(mods, w)
		}
		// A relative import means something else in a package than in a module,
		// so foo/__init__.pyi cannot describe foo.py.
		if !w.hasStub && isInit(w.path) == isInit(c.path) {
			w.stub, w.hasStub = files[c.raw], true
		}
	}
	slices.SortFunc(mods, func(a, b *work) int { return strings.Compare(a.modName, b.modName) })
	items := make([]work, len(mods))
	for i, w := range mods {
		items[i] = *w
	}
	return items
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
	path   string
	isPkg  bool
	public bool
	// defs holds functions, classes and type aliases. attrs holds attributes
	// whose name has no entry in defs.
	defs  map[string]definition
	attrs map[string]definition
	// stubOnly holds mod.StubOnly: names a lookup finds only if imports do not.
	stubOnly map[string]definition
	// order lists the keys of defs, attrs and stubOnly in a stable order, except
	// a stub's type parameters, which a star import does not bind as API.
	order []string
	// importSources caches importSource for this module's imports.
	importSources map[*model.Import]string
}

func newParsedModule(mod *model.Module, path string) *parsedModule {
	pm := &parsedModule{
		mod:      mod,
		path:     path,
		isPkg:    strings.HasSuffix(path, "/__init__.py"),
		public:   !isPrivateModule(path),
		defs:     map[string]definition{},
		attrs:    map[string]definition{},
		stubOnly: map[string]definition{},
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
				if !a.StubTypeParam {
					pm.order = append(pm.order, a.Name)
				}
			}
		}
	}
	for _, d := range stubDefinitions(mod) {
		isTypeParam := d.attr != nil && d.attr.StubTypeParam
		if !pm.definesDirectly(d.name()) && !isTypeParam {
			pm.order = append(pm.order, d.name())
		}
		pm.stubOnly[d.name()] = d
	}
	return pm
}

// stubDefinitions lists mod.StubOnly in a stable order.
func stubDefinitions(mod *model.Module) []definition {
	var defs []definition
	for _, f := range mod.StubOnly.Functions {
		defs = append(defs, definition{module: mod.Name, fn: f})
	}
	for _, c := range mod.StubOnly.Classes {
		defs = append(defs, definition{module: mod.Name, cls: c})
	}
	for _, t := range mod.StubOnly.TypeAliases {
		defs = append(defs, definition{module: mod.Name, alias: t})
	}
	for _, a := range mod.StubOnly.Attributes {
		defs = append(defs, definition{module: mod.Name, attr: a})
	}
	return defs
}

// settleStubDefinitions decides each name a .pyi defines and its .py may import.
// If the import finds a definition, the stub's types go onto it, so the docstring
// in the .py stays. If not, as with a compiled module, the stub's definition joins
// the module. All lookups run before any change, so module order has no effect.
func (r *resolver) settleStubDefinitions(parsed []*parsedModule) {
	type fill struct{ target, stub definition }
	var fills []fill
	joins := map[*parsedModule]*model.Definitions{}
	for _, pm := range parsed {
		for _, stub := range stubDefinitions(pm.mod) {
			if d, ok := r.lookup(pm.mod.Name, stub.name(), map[[2]string]bool{}, 0); ok && d.module != pm.mod.Name {
				fills = append(fills, fill{d, stub})
				continue
			}
			j := joins[pm]
			if j == nil {
				j = &model.Definitions{}
				joins[pm] = j
			}
			switch {
			case stub.fn != nil:
				j.Functions = append(j.Functions, stub.fn)
			case stub.cls != nil:
				j.Classes = append(j.Classes, stub.cls)
			case stub.attr != nil:
				j.Attributes = append(j.Attributes, stub.attr)
			default:
				j.TypeAliases = append(j.TypeAliases, stub.alias)
			}
		}
	}
	for _, f := range fills {
		switch {
		case f.target.fn != nil && f.stub.fn != nil:
			extractor.MergeFunction(f.target.fn, f.stub.fn)
		case f.target.cls != nil && f.stub.cls != nil:
			extractor.MergeClass(f.target.cls, f.stub.cls)
		case f.target.attr != nil && f.stub.attr != nil:
			extractor.MergeAttribute(f.target.attr, f.stub.attr)
		}
	}
	for _, pm := range parsed {
		if len(pm.stubOnly) == 0 {
			continue
		}
		pm.mod.StubOnly = model.Definitions{}
		if j := joins[pm]; j != nil {
			extractor.AddStubDefinitions(pm.mod, *j)
		}
		*pm = *newParsedModule(pm.mod, pm.path)
	}
	r.stars = nil
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
//
// The result is kept on pm, one string per import, because a resolver asks for the
// same import many times and a package can hold hundreds of thousands of imports.
func importSource(pm *parsedModule, imp *model.Import) string {
	if src, ok := pm.importSources[imp]; ok {
		return src
	}
	if pm.importSources == nil {
		pm.importSources = make(map[*model.Import]string, len(pm.mod.ImportFroms))
	}
	src := resolveImportSource(pm, imp)
	pm.importSources[imp] = src
	return src
}

func resolveImportSource(pm *parsedModule, imp *model.Import) string {
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
		case imp.Name != "" && !imp.NotExported:
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
	if d, ok := pm.stubOnly[name]; ok {
		return d, true
	}
	d, ok := pm.attrs[name]
	return d, ok
}

// exportList returns the names public module pm publishes. With a complete
// __all__ that is every listed name. Otherwise it is the public names imported
// from private modules of the wheel, the names any module marks as explicit
// re-exports, and any names __all__ lists.
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
		if imp.Name == "" || imp.NotExported {
			continue
		}
		src := importSource(pm, imp)
		target, ok := r.modules[src]
		if !ok || (target.public && !isExplicitReexport(imp)) {
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

// isExplicitReexport reports whether imp has the form "from .x import Y as Y", the
// standard way to mark a name as part of a module's public API (PEP 484).
func isExplicitReexport(imp *model.Import) bool {
	return imp.Name != "*" && imp.Alias == imp.Name
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
		typeParams := map[string]bool{}
		for _, a := range slices.Concat(m.Attributes, m.Hidden.Attributes) {
			if a.StubTypeParam {
				typeParams[a.Name] = true
			}
		}
		keepAttr := func(n string) bool { return seen[n] || (keep(n) && !typeParams[n]) }
		m.Attributes, m.Hidden.Attributes = repartition(m.Attributes, m.Hidden.Attributes, func(a *model.Attribute) string { return a.Name }, keepAttr)
		m.TypeAliases, m.Hidden.TypeAliases = repartition(m.TypeAliases, m.Hidden.TypeAliases, func(t *model.TypeAlias) string { return t.Name }, keep)
	}

	// An attribute that shares its name with a working import is a fallback, as
	// in "try: from ._x import Y / except ImportError: Y = None". The import wins.
	for _, imp := range m.ImportFroms {
		if imp.Name == "" || imp.Name == "*" || imp.NotExported {
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
// alias. A private definition is copied once, into the exporting module with the
// shortest path, then the first in sort order, and its other re-exports become
// aliases of that copy.
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
			// The module with the shortest path holds the copy, so a deprecated
			// shim deep in the tree does not. On a tie it is the module that sorts
			// first, which comes first here. Inside that module, prefer the
			// definition's own name over a rename.
			copyIdx = 0
			depth := func(e export) int { return strings.Count(e.pm.mod.Name, ".") }
			for i, e := range g.exports {
				if depth(e) < depth(g.exports[copyIdx]) {
					copyIdx = i
				}
			}
			for i, e := range g.exports {
				if e.pm == g.exports[copyIdx].pm && e.name == id.name {
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

// extractSafely extracts one module and returns nil if extraction panics. It also
// returns the number of parse errors, which do not change the result, and whether
// extraction panicked.
func extractSafely(modName string, src []byte) (mod *model.Module, parseErrs int, panicked bool) {
	mod, panicked = safely(modName, func() *model.Module {
		m, errs := extractModuleFn(modName, src)
		parseErrs = len(errs)
		return m
	})
	return mod, parseErrs, panicked
}

// safely returns nil if fn panics, so a single bad module cannot stop a worker or
// lose other modules. The second value reports the panic.
func safely(modName string, fn func() *model.Module) (mod *model.Module, panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("goopy: extract panic on %s: %v\n%s", modName, r, debug.Stack())
			mod, panicked = nil, true
		}
	}()
	return fn(), false
}

func pathToModuleName(path string) string {
	name := strings.TrimSuffix(sourcePath(path), ".py")
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
	return extractWheel(ctx, wheel.NewSource(), name, version, newTypeFlattener().module)
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
	// Take the sources out of contents, so reading other fields below does not keep
	// them in memory for the whole parse.
	files := contents.Files
	contents.Files = nil
	pkg := extractPackage(ctx, name, files, contents.TopLevelPkgs, afterParse)
	pkg.Version = version
	if contents.Version != "" {
		pkg.Version = contents.Version
	}
	return pkg, nil
}

// testFileDirs returns the directories that directly contain a test_*.py or
// *_test.py file, or a stub of that name.
func testFileDirs(files map[string][]byte) map[string]bool {
	dirs := map[string]bool{}
	for path := range files {
		path = sourcePath(path)
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
