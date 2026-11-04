package goopy

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"runtime"
	"slices"
	"strings"
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

func TestFlattenTypes_KeepsRawAndKindEverywhere(t *testing.T) {
	src := []byte(`
from typing import Callable, Dict, List, overload

T = Dict[str, List[int]]
x: List[int] = []

def f(a: Dict[str, List[int]], *b: Callable[[int], str]) -> List[Dict[str, int]]:
    pass

class Base(Dict[str, int]):
    y: List[str]
    def __init__(self, z: List[int]) -> None:
        pass
    def m(self, q: Dict[str, int]) -> List[str]:
        pass
    class Inner:
        def n(self, r: List[int]) -> None:
            pass

def _private(s: List[int]) -> List[int]:
    pass

@overload
def g(t: List[int]) -> Dict[str, int]: ...
@overload
def g(t: List[str]) -> Dict[str, str]: ...
def g(t):
    pass
`)
	mod, errs := ExtractModule("m", src)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	before, err := json.Marshal(mod)
	if err != nil {
		t.Fatal(err)
	}
	flattenTypes(mod)
	after, err := json.Marshal(mod)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(after), `"args"`) || strings.Contains(string(after), `"elements"`) {
		t.Errorf("flattened module still has nested types: %s", after)
	}

	var want []string
	var walkTop func(v any)
	walkTop = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			// A type expression's outer Raw is what stays. Its children go.
			if raw, ok := x["raw"].(string); ok {
				want = append(want, raw)
				return
			}
			for _, k := range slices.Sorted(maps.Keys(x)) {
				walkTop(x[k])
			}
		case []any:
			for _, e := range x {
				walkTop(e)
			}
		}
	}
	var decoded any
	if err := json.Unmarshal(before, &decoded); err != nil {
		t.Fatal(err)
	}
	walkTop(decoded)

	var decodedAfter any
	if err := json.Unmarshal(after, &decodedAfter); err != nil {
		t.Fatal(err)
	}
	beforeRaws := want
	want = nil
	walkTop(decodedAfter)
	if !slices.Equal(beforeRaws, want) || len(want) < 8 {
		t.Errorf("outer Raw values changed:\nbefore %v\nafter  %v", beforeRaws, want)
	}
}

func TestFlattenTypes_CoversStubOnly(t *testing.T) {
	mod, errs := ExtractModule("m", []byte("def f(a: List[int]) -> Dict[str, int]: ...\n"))
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	f := mod.Functions[0]
	mod.Functions = nil
	mod.StubOnly.Functions = []*model.Function{f}
	flattenTypes(mod)
	if got := f.Parameters[0].Type; got.Raw != "List[int]" || got.Args != nil {
		t.Errorf("parameter type = %+v, want only Kind and Raw", got)
	}
	if got := f.Returns; got.Raw != "Dict[str, int]" || got.Args != nil {
		t.Errorf("return type = %+v, want only Kind and Raw", got)
	}
}
