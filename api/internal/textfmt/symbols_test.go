package textfmt_test

import (
	"fmt"
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

// duplicatePathDocs has many paths that hold both a method and an attribute, and
// two modules that repeat a path of the same kind.
func duplicatePathDocs() *textfmt.DocsInput {
	cls := textfmt.DocSymbolInput{Name: "C", Kind: "class", Signature: "class C"}
	for i := range 200 {
		name := fmt.Sprintf("m%03d", (i*37)%200)
		cls.Methods = append(cls.Methods, textfmt.DocSymbolInput{Name: name, Kind: "method", Signature: "def " + name + "()"})
		cls.Attributes = append(cls.Attributes, textfmt.DocAttributeInput{Name: name})
	}
	return &textfmt.DocsInput{Modules: []textfmt.DocModuleInput{
		{Name: "p", Classes: []textfmt.DocSymbolInput{cls}, Functions: []textfmt.DocSymbolInput{{Name: "f", Kind: "function", Signature: "def first()"}}},
		{Name: "p", Functions: []textfmt.DocSymbolInput{{Name: "f", Kind: "function", Signature: "def second()"}}},
	}}
}

func TestFormatSymbolsDuplicatePaths(t *testing.T) {
	first := textfmt.FormatSymbols(duplicatePathDocs(), "", "", 1000)
	for range 20 {
		if got := textfmt.FormatSymbols(duplicatePathDocs(), "", "", 1000); got != first {
			t.Fatal("output differs between runs")
		}
	}

	// Equal paths order by kind, and equal kinds keep their source order.
	lines := strings.Split(strings.TrimSpace(first), "\n")[1:]
	for i, line := range lines {
		if i == 0 {
			continue
		}
		a, b := strings.Split(lines[i-1], "\t"), strings.Split(line, "\t")
		if a[0] > b[0] || (a[0] == b[0] && a[1] > b[1]) {
			t.Fatalf("out of order at line %d: %q then %q", i, lines[i-1], line)
		}
	}
	if !strings.Contains(first, "p.f\tfunction\tdef first()\np.f\tfunction\tdef second()\n") {
		t.Errorf("equal path and kind lost source order:\n%s", first)
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
