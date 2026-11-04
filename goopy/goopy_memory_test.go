package goopy

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pypx/goopy/model"
	"github.com/pypx/goopy/wheel"
)

// trackedFetcher returns sources whose collection it counts. It keeps no reference
// to them.
type trackedFetcher struct {
	files     int
	collected *atomic.Int32
}

func (f trackedFetcher) Fetch(context.Context, string, string) (*wheel.WheelContents, error) {
	files := make(map[string][]byte, f.files)
	for i := range f.files {
		src := make([]byte, 64<<10)
		copy(src, fmt.Sprintf("def f%d(): pass\n", i))
		collected := f.collected
		runtime.SetFinalizer(&src[0], func(*byte) { collected.Add(1) })
		files[fmt.Sprintf("pkg/m%03d.py", i)] = src
	}
	return &wheel.WheelContents{Files: files, TopLevelPkgs: []string{"pkg"}}, nil
}

// TestExtractFromWheel_ReleasesSourcesAsItParses checks that sources already parsed
// can be collected while the last module is still parsing.
func TestExtractFromWheel_ReleasesSourcesAsItParses(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files int
	}{
		{"serial", parallelThreshold},
		{"parallel", parallelThreshold + 2*runtime.GOMAXPROCS(0) + 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Workers may hold one source each, plus the module that observes.
			want := int32(1)
			if tc.name == "serial" {
				want = int32(tc.files - 1)
			}
			last := fmt.Sprintf("pkg.m%03d", tc.files-1)

			var collected, seen atomic.Int32
			orig := extractModuleFn
			defer func() { extractModuleFn = orig }()
			extractModuleFn = func(name string, src []byte) (*model.Module, []error) {
				if name == last {
					deadline := time.Now().Add(5 * time.Second)
					for collected.Load() < want && time.Now().Before(deadline) {
						runtime.GC()
						time.Sleep(10 * time.Millisecond)
					}
					seen.Store(collected.Load())
				}
				return orig(name, src)
			}

			fetcher := trackedFetcher{files: tc.files, collected: &collected}
			pkg, err := ExtractFromWheel(context.Background(), fetcher, "pkg", "1")
			if err != nil {
				t.Fatal(err)
			}
			if len(pkg.Modules) != tc.files {
				t.Errorf("modules = %d, want %d", len(pkg.Modules), tc.files)
			}
			if got := seen.Load(); got < want {
				t.Errorf("%d sources collected before the last module parsed, want at least %d", got, want)
			}
		})
	}
}
