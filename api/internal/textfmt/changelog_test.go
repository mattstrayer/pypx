package textfmt_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pypx/api/internal/changelog"
	"github.com/pypx/api/internal/textfmt"
)

func TestFormatChangelog(t *testing.T) {
	in := &textfmt.ChangelogInput{
		Package: "httpx",
		Source:  "github_releases",
		RepoURL: "https://github.com/encode/httpx",
		Entries: []changelog.Entry{
			{
				Version:     "0.27.0",
				PublishedAt: "2024-04-15",
				Body:        "## Added\n- New `Auth` flow.\n## Fixed\n- Cookie persistence.",
			},
			{
				Version:     "0.26.0",
				PublishedAt: "2024-02-20",
				Body:        "First release of the year.",
			},
		},
	}
	got := textfmt.FormatChangelog(in)
	goldenPath := filepath.Join("testdata", "changelog_httpx.golden")
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
