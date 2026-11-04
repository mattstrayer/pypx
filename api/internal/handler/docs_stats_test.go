package handler

import (
	"strings"
	"testing"
	"time"

	"github.com/pypx/goopy/model"
)

func TestExtractionLogLine(t *testing.T) {
	pkg := &model.Package{
		Modules: []*model.Module{{Name: "a"}, {Name: "b"}},
		Stats: model.Stats{
			Files: 5, Modules: 4, FilesWithErrors: 1, Errors: 3, PanickedModules: 0,
			Duration: 42 * time.Millisecond,
		},
	}
	got := extractionLogLine("demo", "1.0", pkg)
	want := "docs: extracted pkg=demo version=1.0 files=5 modules=4 published=2 files_with_errors=1 parse_errors=3 panicked=0 partial=true ms=42"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}

	pkg.Stats.FilesWithErrors, pkg.Stats.Errors = 0, 0
	if got := extractionLogLine("demo", "1.0", pkg); !strings.Contains(got, "partial=false") {
		t.Errorf("clean run should report partial=false: %q", got)
	}
}
