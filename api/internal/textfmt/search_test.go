package textfmt_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pypx/api/internal/search"
	"github.com/pypx/api/internal/textfmt"
)

func TestFormatSearch(t *testing.T) {
	results := []search.PackageEntry{
		{Name: "httpx", Summary: "The next generation HTTP client.", Downloads: 50_000_000},
		{Name: "httpcore", Summary: "A minimal low-level HTTP client.", Downloads: 40_000_000},
		{Name: "httptools", Summary: "A collection of framework independent HTTP protocol utils.", Downloads: 30_000_000},
	}
	got := textfmt.FormatSearch(results)
	goldenPath := filepath.Join("testdata", "search_httpx.golden")
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

func TestFormatSearchEmpty(t *testing.T) {
	got := textfmt.FormatSearch(nil)
	want := "# name\tdownloads\tsummary\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
