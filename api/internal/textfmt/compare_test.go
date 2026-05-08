package textfmt_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pypx/api/internal/textfmt"
)

func fixtureCompareThree() *textfmt.CompareInput {
	return &textfmt.CompareInput{
		Packages: []textfmt.ComparePackageInput{
			{
				Name:             "httpx",
				Version:          "0.28.1",
				Summary:          "The next generation HTTP client.",
				License:          "BSD-3-Clause",
				PythonMin:        "3.8",
				InstallSize:      786_432,
				ModuleFormat:     "wheel",
				LastReleasedDate: "2025-09-15",
				ReleasesLast12Mo: 3,
				DepCount:         4,
				Downloads30d:     653_000_000,
				VulnCount:        2,
				Typed:            "yes",
				RepoURL:          "https://github.com/encode/httpx",
				DocURL:           "https://www.python-httpx.org/",
			},
			{
				Name:             "requests",
				Version:          "2.32.3",
				Summary:          "Python HTTP for Humans.",
				License:          "Apache-2.0",
				PythonMin:        "3.8",
				InstallSize:      188_416,
				ModuleFormat:     "wheel",
				LastReleasedDate: "2024-05-29",
				ReleasesLast12Mo: 2,
				DepCount:         4,
				Downloads30d:     712_000_000,
				VulnCount:        0,
				Typed:            "stubs",
				RepoURL:          "https://github.com/psf/requests",
				DocURL:           "https://requests.readthedocs.io/",
			},
			{
				Name:             "aiohttp",
				Version:          "3.10.5",
				Summary:          "Async HTTP client/server framework.",
				License:          "Apache-2.0",
				PythonMin:        "3.8",
				InstallSize:      1_258_291,
				ModuleFormat:     "wheel",
				LastReleasedDate: "2024-08-19",
				ReleasesLast12Mo: 5,
				DepCount:         5,
				Downloads30d:     142_000_000,
				VulnCount:        0,
				Typed:            "yes",
				RepoURL:          "https://github.com/aio-libs/aiohttp",
				DocURL:           "https://docs.aiohttp.org/",
			},
		},
	}
}

func TestFormatCompare(t *testing.T) {
	got := textfmt.FormatCompare(fixtureCompareThree())
	goldenPath := filepath.Join("testdata", "compare_three.golden")
	if *update {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v (run with -update)", err)
	}
	if got != string(want) {
		t.Errorf("mismatch.\n--- got ---\n%s\n--- want ---\n%s", got, string(want))
	}
}

func TestFormatCompareSkippedOnly(t *testing.T) {
	in := &textfmt.CompareInput{
		Skipped: []textfmt.SkippedPackage{
			{Name: "fakepkg1", Reason: "not found"},
			{Name: "fakepkg2", Reason: "fetch error"},
		},
	}
	got := textfmt.FormatCompare(in)
	want := "# skipped: fakepkg1 (not found)\n# skipped: fakepkg2 (fetch error)\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatCompareWithSkippedAndTable(t *testing.T) {
	in := &textfmt.CompareInput{
		Skipped: []textfmt.SkippedPackage{{Name: "fakepkg", Reason: "not found"}},
		Packages: []textfmt.ComparePackageInput{
			{Name: "httpx", Version: "0.28.1"},
		},
	}
	got := textfmt.FormatCompare(in)
	if !strings.HasPrefix(got, "# skipped: fakepkg (not found)\n") {
		t.Errorf("expected leading skipped comment, got:\n%s", got)
	}
	if !strings.Contains(got, "| metric") {
		t.Errorf("expected table header, got:\n%s", got)
	}
	if !strings.Contains(got, "0.28.1") {
		t.Errorf("expected version row to contain 0.28.1, got:\n%s", got)
	}
}

func TestFormatCompareTruncation(t *testing.T) {
	in := &textfmt.CompareInput{
		Packages: []textfmt.ComparePackageInput{
			{Name: "longpkg", Version: "1.0.0", Summary: "This is a very long summary that should be truncated at the limit."},
		},
	}
	got := textfmt.FormatCompare(in)
	// Cells truncated to 28 visible chars total (27 + ellipsis).
	if strings.Contains(got, "should be truncated at the limit") {
		t.Errorf("expected truncation, got full summary:\n%s", got)
	}
	if !strings.Contains(got, "…") {
		t.Errorf("expected ellipsis in truncated output, got:\n%s", got)
	}
}
