package textfmt_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pypx/api/internal/textfmt"
)

func TestFormatSymbolsAll(t *testing.T) {
	got := textfmt.FormatSymbols(fixtureDocs(), "", "", 100)
	goldenPath := filepath.Join("testdata", "symbols_sample.golden")
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

func TestFormatSymbolsQuery(t *testing.T) {
	got := textfmt.FormatSymbols(fixtureDocs(), "client", "", 100)
	if !strings.Contains(got, "httpx.Client\tclass") {
		t.Errorf("expected httpx.Client class line, got:\n%s", got)
	}
	if !strings.Contains(got, "httpx.Client.get\tmethod") {
		t.Errorf("expected httpx.Client.get method line, got:\n%s", got)
	}
	if strings.Contains(got, "httpx.get\t") {
		t.Errorf("expected httpx.get NOT in output, got:\n%s", got)
	}
}

func TestFormatSymbolsKindFilter(t *testing.T) {
	got := textfmt.FormatSymbols(fixtureDocs(), "", "function", 100)
	if !strings.Contains(got, "httpx.get\tfunction") {
		t.Errorf("expected function line, got:\n%s", got)
	}
	if strings.Contains(got, "\tclass\t") || strings.Contains(got, "\tmethod\t") {
		t.Errorf("expected only functions, got:\n%s", got)
	}
}

func TestFormatSymbolsLimit(t *testing.T) {
	got := textfmt.FormatSymbols(fixtureDocs(), "", "", 1)
	// 1 header line + 1 data line = 2 \n total
	lines := strings.Count(got, "\n")
	if lines != 2 {
		t.Errorf("expected 2 lines (header + 1 result), got %d:\n%s", lines, got)
	}
}
