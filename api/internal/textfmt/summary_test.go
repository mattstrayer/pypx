package textfmt_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pypx/api/internal/enrichment"
	"github.com/pypx/api/internal/osv"
	"github.com/pypx/api/internal/textfmt"
)

func TestFormatSummary(t *testing.T) {
	pkg := &textfmt.PackageInput{
		Name:           "httpx",
		Version:        "0.27.0",
		Summary:        "The next generation HTTP client.",
		License:        "BSD-3-Clause",
		RequiresPython: ">=3.8",
		InstallSize:    786_432,
		DocURL:         "https://www.python-httpx.org/",
		ProjectURLs: map[string]string{
			"Source": "https://github.com/encode/httpx",
		},
		PythonVersions: enrichment.PythonVersionInfo{MinVersion: "3.8"},
		Dependencies: enrichment.DependencyTree{
			Required: []enrichment.Dependency{
				{Name: "anyio"},
				{Name: "certifi"},
				{Name: "httpcore", Constraint: "==1.*"},
				{Name: "idna"},
				{Name: "sniffio"},
				{Name: "extra-dep"}, // 6th — should be dropped
			},
		},
		ReleaseCadence: enrichment.ReleaseCadence{
			ReleasesLast12Mo:  6,
			LastReleasedAt:    "2024-04-15T10:00:00Z",
		},
	}
	sec := &textfmt.SecurityInput{
		Package:   "httpx",
		CheckedAt: "2026-05-06T12:00:00Z",
		Vulns: []osv.VulnInfo{
			{ID: "GHSA-1", Severity: "HIGH"},
			{ID: "GHSA-2", Severity: "MEDIUM"},
		},
	}
	got := textfmt.FormatSummary(&textfmt.SummaryInput{Package: pkg, Security: sec})

	goldenPath := filepath.Join("testdata", "summary_httpx.golden")
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
	if len(got) > 2048 {
		t.Errorf("summary exceeds 2KB target: %d bytes", len(got))
	}
}

func TestFormatSummaryNoSecurity(t *testing.T) {
	pkg := &textfmt.PackageInput{
		Name:    "safe-pkg",
		Version: "1.0.0",
		Summary: "Safe package.",
	}
	got := textfmt.FormatSummary(&textfmt.SummaryInput{Package: pkg})
	// Just verify it runs and produces non-empty output without security section
	if got == "" {
		t.Fatal("empty output")
	}
	if contains(got, "## security") {
		t.Errorf("expected no security section, got:\n%s", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestMaxSeverityFromCVSS(t *testing.T) {
	cases := []struct {
		name    string
		severity string
		want    string
	}{
		{"plain HIGH", "HIGH", "HIGH"},
		{"plain critical lower", "critical", "CRITICAL"},
		{"cvss with C:H", "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:N/A:N", "HIGH"},
		{"cvss with I:L only", "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:L/A:N", "MEDIUM"},
		{"cvss with all N", "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:N", "LOW"},
		{"unknown", "ZZZ", "UNKNOWN"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pkg := &textfmt.PackageInput{Name: "x", Version: "1"}
			sec := &textfmt.SecurityInput{
				Package:   "x",
				CheckedAt: "2026-05-06T12:00:00Z",
				Vulns:     []osv.VulnInfo{{ID: "x", Severity: tc.severity}},
			}
			got := textfmt.FormatSummary(&textfmt.SummaryInput{Package: pkg, Security: sec})
			if tc.want == "UNKNOWN" {
				if contains(got, "max_severity:") {
					t.Errorf("expected no max_severity line for unknown severity, got:\n%s", got)
				}
			} else {
				expectLine := "max_severity: " + tc.want
				if !contains(got, expectLine) {
					t.Errorf("expected %q in output, got:\n%s", expectLine, got)
				}
			}
		})
	}
}
