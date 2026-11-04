// Package benchmark times goopy extraction on real PyPI packages. For each
// package it reports the fetch phase (PyPI lookup, wheel read, unzip), the
// extract phase (parse and model), source bytes read, and bytes allocated.
//
// Run: cd goopy && go test ./internal/benchmark/ -v -count=1 -timeout 600s
// The test needs network access to PyPI. It skips in short mode.
package benchmark

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/pypx/goopy"
	"github.com/pypx/goopy/wheel"
)

type benchPkg struct {
	Name    string
	Version string
}

type result struct {
	pkg         string
	fetchTime   time.Duration
	extractTime time.Duration
	sourceBytes int
	files       int
	allocBytes  uint64
	funcs       int
	classes     int
}

var packages = []benchPkg{
	{"click", "8.1.8"},
	{"requests", "2.32.3"},
	{"flask", "3.1.0"},
	{"httpx", "0.28.1"},
	{"rich", "13.9.4"},
	{"attrs", "25.1.0"},
	{"marshmallow", "4.3.0"},
	{"starlette", "1.0.0"},
	{"jinja2", "3.1.6"},
	{"werkzeug", "3.1.8"},
}

func TestExtractionBenchmark(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping benchmark in short mode")
	}

	source := wheel.NewSource()
	var results []result

	for _, pkg := range packages {
		t.Run(pkg.Name, func(t *testing.T) {
			r := result{pkg: pkg.Name}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			runtime.GC()
			var before runtime.MemStats
			runtime.ReadMemStats(&before)

			start := time.Now()
			contents, err := source.Fetch(ctx, pkg.Name, pkg.Version)
			r.fetchTime = time.Since(start)
			if err != nil {
				t.Fatalf("fetch failed: %v", err)
			}
			r.files = len(contents.Files)
			for _, src := range contents.Files {
				r.sourceBytes += len(src)
			}

			start = time.Now()
			parsed := goopy.ExtractPackage(ctx, pkg.Name, contents.Files, contents.TopLevelPkgs)
			r.extractTime = time.Since(start)

			var after runtime.MemStats
			runtime.ReadMemStats(&after)
			r.allocBytes = after.TotalAlloc - before.TotalAlloc

			for _, mod := range parsed.Modules {
				r.funcs += len(mod.Functions)
				r.classes += len(mod.Classes)
			}

			results = append(results, r)
			t.Logf("fetch %v | extract %v | %d files, %.2f MB source | %d funcs, %d classes | %.1f MB alloc",
				r.fetchTime.Round(time.Millisecond), r.extractTime.Round(time.Millisecond),
				r.files, mb(uint64(r.sourceBytes)), r.funcs, r.classes, mb(r.allocBytes))
		})
	}

	t.Run("_summary", func(t *testing.T) {
		var fetch, extract time.Duration
		var source, alloc uint64
		var funcs, classes int
		for _, r := range results {
			fetch += r.fetchTime
			extract += r.extractTime
			source += uint64(r.sourceBytes)
			alloc += r.allocBytes
			funcs += r.funcs
			classes += r.classes
		}
		t.Logf("packages: %d", len(results))
		t.Logf("fetch total:   %s", fetch.Round(time.Millisecond))
		t.Logf("extract total: %s", extract.Round(time.Millisecond))
		t.Logf("source read:   %.2f MB", mb(source))
		t.Logf("allocated:     %.1f MB", mb(alloc))
		t.Logf("symbols:       %d functions, %d classes", funcs, classes)
	})
}

func mb(n uint64) float64 {
	return float64(n) / (1024 * 1024)
}
