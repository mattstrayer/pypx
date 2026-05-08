package textfmt_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/pypx/api/internal/enrichment"
	"github.com/pypx/api/internal/textfmt"
)

var update = flag.Bool("update", false, "update golden files")

func fixturePackage() *textfmt.PackageInput {
	return &textfmt.PackageInput{
		Name:           "httpx",
		Version:        "0.27.0",
		Summary:        "The next generation HTTP client.",
		License:        "BSD-3-Clause",
		Author:         "Tom Christie",
		HomePage:       "https://www.python-httpx.org/",
		RequiresPython: ">=3.8",
		ProjectURLs: map[string]string{
			"Source":        "https://github.com/encode/httpx",
			"Documentation": "https://www.python-httpx.org/",
		},
		InstallSize:  786_432,
		ModuleFormat: "wheel",
		PythonVersions: enrichment.PythonVersionInfo{
			Constraint: ">=3.8",
			MinVersion: "3.8",
		},
		Dependencies: enrichment.DependencyTree{
			Required: []enrichment.Dependency{
				{Name: "anyio", Constraint: ""},
				{Name: "certifi", Constraint: ""},
				{Name: "httpcore", Constraint: "==1.*"},
				{Name: "idna", Constraint: ""},
				{Name: "sniffio", Constraint: ""},
			},
			Extras: map[string][]enrichment.Dependency{
				"http2": {{Name: "h2", Constraint: ">=3,<5"}},
				"socks": {{Name: "socksio", Constraint: "==1.*"}},
			},
		},
		PlatformCoverage: enrichment.PlatformCoverage{PurePython: true},
		ReleaseCadence: enrichment.ReleaseCadence{
			ReleasesLast12Mo:       6,
			AvgDaysBetweenReleases: 60.5,
			LastReleasedAt:         "2024-04-15T10:00:00Z",
		},
		Maintainers: []enrichment.Maintainer{{Name: "Tom Christie", Email: "tom@example.com"}},
		DocURL:      "https://www.python-httpx.org/",
	}
}

func TestFormatPackage(t *testing.T) {
	got := textfmt.FormatPackage(fixturePackage())
	goldenPath := filepath.Join("testdata", "package_httpx.golden")

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
		t.Errorf("FormatPackage output mismatch.\n--- got ---\n%s\n--- want ---\n%s", got, string(want))
	}
}
