// Package benchmark compares goopy extraction against the Python docs-worker
// sidecar for speed, memory, and correctness.
//
// Run: cd goopy && go test ./internal/benchmark/ -v -count=1 -timeout 600s
// Requires: Python sidecar running at localhost:8001
//   Start with: cd docs-worker && uvicorn main:app --port 8001
package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/pypx/goopy"
)

type benchPkg struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// sidecar response types (match docs-worker output)
type sidecarResponse struct {
	Empty   bool          `json:"empty"`
	Reason  string        `json:"reason"`
	Modules []sidecarMod  `json:"modules"`
	Error   string        `json:"error,omitempty"`
}

type sidecarMod struct {
	Name       string       `json:"name"`
	Functions  []sidecarSym `json:"functions"`
	Classes    []sidecarSym `json:"classes"`
	Exceptions []sidecarSym `json:"exceptions"`
}

type sidecarSym struct {
	Name string `json:"name"`
}

type result struct {
	pkg          string
	goopyTime    time.Duration
	sidecarTime  time.Duration
	goopyMem     uint64 // bytes allocated
	goopyFuncs   int
	goopyClasses int
	sidecarFuncs int
	sidecarClasses int
	sidecarExceptions int
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

func TestMigrationBenchmark(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping benchmark in short mode")
	}

	sidecarURL := os.Getenv("SIDECAR_URL")
	if sidecarURL == "" {
		sidecarURL = "http://localhost:8001"
	}

	// Check sidecar availability
	sidecarAvailable := checkSidecar(sidecarURL)
	if !sidecarAvailable {
		t.Logf("WARNING: Sidecar not available at %s — running goopy-only benchmark", sidecarURL)
		t.Logf("Start sidecar with: cd docs-worker && uvicorn main:app --port 8001")
	}

	client := &http.Client{Timeout: 120 * time.Second}
	var results []result

	for _, pkg := range packages {
		t.Run(pkg.Name, func(t *testing.T) {
			r := result{pkg: pkg.Name}

			// --- Goopy ---
			runtime.GC()
			var memBefore runtime.MemStats
			runtime.ReadMemStats(&memBefore)

			goopyStart := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			goopyPkg, err := goopy.ExtractFromPyPI(ctx, pkg.Name, pkg.Version)
			cancel()
			r.goopyTime = time.Since(goopyStart)

			if err != nil {
				t.Fatalf("goopy failed: %v", err)
			}

			var memAfter runtime.MemStats
			runtime.ReadMemStats(&memAfter)
			r.goopyMem = memAfter.TotalAlloc - memBefore.TotalAlloc

			for _, mod := range goopyPkg.Modules {
				r.goopyFuncs += len(mod.Functions)
				r.goopyClasses += len(mod.Classes)
			}

			// --- Sidecar ---
			if sidecarAvailable {
				sidecarStart := time.Now()
				sidecarResp := callSidecar(t, client, sidecarURL, pkg.Name, pkg.Version)
				r.sidecarTime = time.Since(sidecarStart)

				if sidecarResp != nil {
					for _, mod := range sidecarResp.Modules {
						r.sidecarFuncs += len(mod.Functions)
						r.sidecarClasses += len(mod.Classes)
						r.sidecarExceptions += len(mod.Exceptions)
					}
				}
			}

			results = append(results, r)

			// Per-package log
			if sidecarAvailable {
				speedup := float64(r.sidecarTime) / float64(r.goopyTime)
				t.Logf("goopy: %v (%d funcs, %d classes, %.1f MB) | sidecar: %v (%d funcs, %d classes) | %.1fx faster",
					r.goopyTime.Round(time.Millisecond),
					r.goopyFuncs, r.goopyClasses,
					float64(r.goopyMem)/(1024*1024),
					r.sidecarTime.Round(time.Millisecond),
					r.sidecarFuncs, r.sidecarClasses+r.sidecarExceptions,
					speedup)
			} else {
				t.Logf("goopy: %v (%d funcs, %d classes, %.1f MB alloc)",
					r.goopyTime.Round(time.Millisecond),
					r.goopyFuncs, r.goopyClasses,
					float64(r.goopyMem)/(1024*1024))
			}
		})
	}

	// Summary
	t.Run("_summary", func(t *testing.T) {
		var totalGoopyTime, totalSidecarTime time.Duration
		var totalGoopyMem uint64
		var totalGoopyFuncs, totalGoopyClasses int
		var totalSidecarFuncs, totalSidecarClasses int

		for _, r := range results {
			totalGoopyTime += r.goopyTime
			totalSidecarTime += r.sidecarTime
			totalGoopyMem += r.goopyMem
			totalGoopyFuncs += r.goopyFuncs
			totalGoopyClasses += r.goopyClasses
			totalSidecarFuncs += r.sidecarFuncs
			totalSidecarClasses += r.sidecarClasses + r.sidecarExceptions
		}

		t.Logf("")
		t.Logf("=== MIGRATION BENCHMARK RESULTS ===")
		t.Logf("Packages tested: %d", len(results))
		t.Logf("")
		t.Logf("SPEED")
		t.Logf("  goopy total:   %s", totalGoopyTime.Round(time.Millisecond))
		if sidecarAvailable {
			t.Logf("  sidecar total: %s", totalSidecarTime.Round(time.Millisecond))
			speedup := float64(totalSidecarTime) / float64(totalGoopyTime)
			t.Logf("  speedup:       %.1fx faster", speedup)
		}
		t.Logf("")
		t.Logf("MEMORY (goopy)")
		t.Logf("  total alloc:   %.1f MB", float64(totalGoopyMem)/(1024*1024))
		t.Logf("  avg per pkg:   %.1f MB", float64(totalGoopyMem)/(1024*1024)/float64(len(results)))
		t.Logf("")
		t.Logf("CORRECTNESS")
		t.Logf("  goopy:   %d functions, %d classes", totalGoopyFuncs, totalGoopyClasses)
		if sidecarAvailable {
			t.Logf("  sidecar: %d functions, %d classes", totalSidecarFuncs, totalSidecarClasses)
		}
	})
}

func checkSidecar(url string) bool {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url + "/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

func callSidecar(t *testing.T, client *http.Client, url, name, version string) *sidecarResponse {
	t.Helper()
	body := fmt.Sprintf(`{"name":"%s","version":"%s"}`, name, version)
	resp, err := client.Post(url+"/generate", "application/json", strings.NewReader(body))
	if err != nil {
		t.Logf("sidecar request failed for %s: %v", name, err)
		return nil
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Logf("sidecar read failed for %s: %v", name, err)
		return nil
	}

	if resp.StatusCode != 200 {
		t.Logf("sidecar returned %d for %s: %s", resp.StatusCode, name, string(data))
		return nil
	}

	var result sidecarResponse
	if err := json.Unmarshal(data, &result); err != nil {
		t.Logf("sidecar JSON parse failed for %s: %v", name, err)
		return nil
	}

	return &result
}
