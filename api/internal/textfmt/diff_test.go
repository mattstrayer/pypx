package textfmt_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pypx/api/internal/changelog"
	"github.com/pypx/api/internal/textfmt"
)

func fixtureDiff() *textfmt.DiffInput {
	return &textfmt.DiffInput{
		Package: "httpx",
		From:    "0.26.0",
		To:      "0.28.1",
		Changelog: []changelog.Entry{
			{Version: "0.28.1", PublishedAt: "2024-12-06", Body: "Fix SSL when verify=False."},
			{Version: "0.28.0", PublishedAt: "2024-11-29", Body: "Drop deprecated APIs."},
			{Version: "0.27.0", PublishedAt: "2024-04-15", Body: "New Auth flow."},
		},
		DepChanges: textfmt.DepDiff{
			Added:   []string{"anyio"},
			Removed: []string{"httpcore-py3"},
			Bumped: []textfmt.DepBump{
				{Name: "httpcore", FromConstraint: "==0.17.*", ToConstraint: "==1.*"},
			},
		},
		APIChanges: textfmt.APIDiff{
			Added: []string{
				"httpx.AsyncClient.aclose",
				"httpx.URL.copy_with",
			},
			Removed: []string{
				"httpx.Client.send_singlerequest",
			},
			Changed: []textfmt.APIChange{
				{
					Path:    "httpx.Client.send",
					FromSig: "def send(request)",
					ToSig:   "def send(request, *, stream=False)",
				},
			},
		},
	}
}

func TestFormatDiff(t *testing.T) {
	got := textfmt.FormatDiff(fixtureDiff())
	goldenPath := filepath.Join("testdata", "diff_sample.golden")
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

func TestFormatDiffPartialUnavailable(t *testing.T) {
	in := &textfmt.DiffInput{
		Package: "httpx",
		From:    "0.26.0",
		To:      "0.28.1",
		ChangelogUnavailable: "no changelog source",
		DepChanges: textfmt.DepDiff{Added: []string{"anyio"}},
		APIChangesUnavailable: "could not extract docs for 0.26.0",
	}
	got := textfmt.FormatDiff(in)
	for _, want := range []string{
		"## changelog\n# unavailable: no changelog source",
		"## dependency changes\n+ added: anyio",
		"## api changes\n# unavailable: could not extract docs for 0.26.0",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestFormatDiffEmptyDepDiff(t *testing.T) {
	in := &textfmt.DiffInput{
		Package: "httpx",
		From:    "0.26.0",
		To:      "0.28.1",
		ChangelogUnavailable: "no changelog source",
		// All three DepDiff slices empty.
		APIChangesUnavailable: "could not extract docs for either version",
	}
	got := textfmt.FormatDiff(in)
	if !strings.Contains(got, "## dependency changes\n# no changes\n") {
		t.Errorf("expected '# no changes' line for empty dep diff, got:\n%s", got)
	}
}

func TestFormatDiffTruncation(t *testing.T) {
	added := make([]string, 200)
	for i := range added {
		added[i] = "pkg.symbol_a"
	}
	in := &textfmt.DiffInput{
		Package: "x",
		From:    "1.0", To: "2.0",
		ChangelogUnavailable: "no",
		DepChangesUnavailable: "no",
		APIChanges: textfmt.APIDiff{
			Added:          added,
			AddedTruncated: 50,
		},
	}
	got := textfmt.FormatDiff(in)
	if !strings.Contains(got, "# truncated: 50 more added") {
		t.Errorf("expected truncation footer, got:\n%s", got)
	}
}
