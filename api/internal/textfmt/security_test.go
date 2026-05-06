package textfmt_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pypx/api/internal/osv"
	"github.com/pypx/api/internal/textfmt"
)

func TestFormatSecurity(t *testing.T) {
	in := &textfmt.SecurityInput{
		Package:   "httpx",
		CheckedAt: "2026-05-06T12:00:00Z",
		Vulns: []osv.VulnInfo{
			{ID: "GHSA-aaaa-bbbb-cccc", Severity: "HIGH", AffectedRange: "<0.27.0", FixedIn: "0.27.0", Summary: "Cookie leak via Location header", URL: "https://github.com/advisories/GHSA-aaaa-bbbb-cccc"},
			{ID: "PYSEC-2023-1234", Severity: "MEDIUM", AffectedRange: "<0.26.0", Summary: "ReDoS in URL parsing", URL: "https://osv.dev/PYSEC-2023-1234"},
		},
	}
	got := textfmt.FormatSecurity(in)
	goldenPath := filepath.Join("testdata", "security_httpx.golden")
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

func TestFormatSecurityNoVulns(t *testing.T) {
	in := &textfmt.SecurityInput{Package: "safe-pkg", CheckedAt: "2026-05-06T12:00:00Z"}
	got := textfmt.FormatSecurity(in)
	want := "package: safe-pkg\nchecked_at: 2026-05-06T12:00:00Z\nvuln_count: 0\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
