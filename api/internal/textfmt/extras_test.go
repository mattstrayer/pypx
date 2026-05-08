package textfmt_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pypx/api/internal/textfmt"
)

func fixtureExtras() *textfmt.ExtrasInput {
	return &textfmt.ExtrasInput{
		Package:        "httpx",
		TypeStatus:     "typed",
		CondaAvailable: true,
		CondaLatest:    "0.27.0",
		HasRepo:        true,
		RepoStars:      13500,
		RepoForks:      850,
		RepoOpenIssues: 65,
	}
}

func TestFormatExtras(t *testing.T) {
	got := textfmt.FormatExtras(fixtureExtras())
	goldenPath := filepath.Join("testdata", "extras_httpx.golden")

	if *update {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v (run with -update to create)", err)
	}
	if got != string(want) {
		t.Errorf("FormatExtras output mismatch.\n--- got ---\n%s\n--- want ---\n%s", got, string(want))
	}
}

func TestFormatExtrasStubs(t *testing.T) {
	in := &textfmt.ExtrasInput{
		Package:     "requests",
		TypeStatus:  "stubs",
		StubPackage: "types-requests",
	}
	got := textfmt.FormatExtras(in)
	want := "package: requests\ntype_status: stubs\nstub_package: types-requests\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatExtrasNoConda(t *testing.T) {
	in := &textfmt.ExtrasInput{
		Package:        "some-pkg",
		TypeStatus:     "untyped",
		CondaAvailable: false,
	}
	got := textfmt.FormatExtras(in)
	want := "package: some-pkg\ntype_status: untyped\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
