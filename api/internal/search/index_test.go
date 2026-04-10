package search

import (
	"testing"
)

// sanitizeFTSQueryTests are table-driven unit tests for the sanitizer itself.
var sanitizeFTSQueryTests = []struct {
	name  string
	input string
	want  string
}{
	{"plain", "requests", "requests"},
	{"trims space", "  flask  ", "flask"},
	{"strips quotes", `test "injection`, "test injection"},
	{"strips all quotes", `"quoted"`, "quoted"},
	{"removes OR operator", "test OR *", "test *"},
	{"removes AND operator", "foo AND bar", "foo bar"},
	{"removes NOT operator", "foo NOT bar", "foo bar"},
	{"removes NEAR operator", "foo NEAR bar", "foo bar"},
	{"empty after strip", `""`, ""},
	{"only whitespace", "   ", ""},
}

func mustNewIndex(t *testing.T) *Index {
	t.Helper()
	idx, err := NewIndex(":memory:")
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	t.Cleanup(func() { idx.Close() })
	return idx
}

func mustUpsert(t *testing.T, idx *Index, entry PackageEntry) {
	t.Helper()
	if err := idx.Upsert(entry); err != nil {
		t.Fatalf("Upsert(%q): %v", entry.Name, err)
	}
}

// TestIndexAndSearch verifies that an exact-match package ranks first among
// packages whose names share a common prefix.
func TestIndexAndSearch(t *testing.T) {
	idx := mustNewIndex(t)

	packages := []PackageEntry{
		{Name: "requests", Summary: "HTTP for Humans", Downloads: 50_000_000},
		{Name: "flask", Summary: "A micro web framework", Downloads: 30_000_000},
		{Name: "django", Summary: "The web framework for perfectionists", Downloads: 25_000_000},
		{Name: "request-mock", Summary: "Mock requests library", Downloads: 500_000},
	}
	for _, p := range packages {
		mustUpsert(t, idx, p)
	}

	results, err := idx.Search("requests", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) < 1 {
		t.Fatalf("expected at least 1 result, got 0")
	}
	if results[0].Name != "requests" {
		t.Errorf("expected first result to be %q, got %q", "requests", results[0].Name)
	}
}

// TestSearchPrefix verifies that all prefix-matching packages are returned and
// the one with the highest download count (exact name match) ranks first.
func TestSearchPrefix(t *testing.T) {
	idx := mustNewIndex(t)

	packages := []PackageEntry{
		{Name: "flask", Summary: "A micro web framework", Downloads: 30_000_000},
		{Name: "flask-cors", Summary: "Cross-Origin Resource Sharing for Flask", Downloads: 5_000_000},
		{Name: "flask-login", Summary: "User session management for Flask", Downloads: 3_000_000},
	}
	for _, p := range packages {
		mustUpsert(t, idx, p)
	}

	results, err := idx.Search("flask", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Name != "flask" {
		t.Errorf("expected first result to be %q (highest downloads + exact match), got %q", "flask", results[0].Name)
	}
}

// TestSanitizeFTSQuery verifies the sanitizer in isolation.
func TestSanitizeFTSQuery(t *testing.T) {
	for _, tc := range sanitizeFTSQueryTests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeFTSQuery(tc.input)
			if got != tc.want {
				t.Errorf("sanitizeFTSQuery(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestSearchInjection verifies that queries containing FTS5 special syntax
// do not cause errors and return sensible (possibly empty) results.
func TestSearchInjection(t *testing.T) {
	idx := mustNewIndex(t)

	packages := []PackageEntry{
		{Name: "requests", Summary: "HTTP for Humans", Downloads: 50_000_000},
		{Name: "flask", Summary: "A micro web framework", Downloads: 30_000_000},
	}
	for _, p := range packages {
		mustUpsert(t, idx, p)
	}

	cases := []struct {
		name  string
		query string
	}{
		{"double quote injection", `test "injection`},
		{"FTS5 OR operator", `test OR *`},
		{"FTS5 AND operator", `requests AND flask`},
		{"unbalanced quotes", `"`},
		{"only operators", `OR AND NOT`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			results, err := idx.Search(tc.query, 10)
			if err != nil {
				t.Errorf("Search(%q) returned error: %v", tc.query, err)
			}
			// Results may be empty or non-empty — both are valid.
			// The critical invariant is no error (no crash / parse failure).
			_ = results
		})
	}
}

// TestUpdateDownloadsBatch verifies that downloads can be updated without
// overwriting name/summary, and that search results re-rank accordingly.
func TestUpdateDownloadsBatch(t *testing.T) {
	idx := mustNewIndex(t)

	// Insert with zero downloads.
	packages := []PackageEntry{
		{Name: "flask", Summary: "A micro web framework", Downloads: 0},
		{Name: "flask-cors", Summary: "CORS for Flask", Downloads: 0},
		{Name: "flask-login", Summary: "User sessions for Flask", Downloads: 0},
	}
	if err := idx.UpsertBatch(packages); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}

	// Update downloads: flask-cors gets the most.
	updates := []PackageEntry{
		{Name: "flask", Downloads: 1_000_000},
		{Name: "flask-cors", Downloads: 5_000_000},
	}
	if err := idx.UpdateDownloadsBatch(updates); err != nil {
		t.Fatalf("UpdateDownloadsBatch: %v", err)
	}

	results, err := idx.Search("flask", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	// "flask" should still be first (exact match), then flask-cors (5M), then flask-login (0).
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Name != "flask" {
		t.Errorf("expected first result 'flask' (exact match), got %q", results[0].Name)
	}
	if results[1].Name != "flask-cors" {
		t.Errorf("expected second result 'flask-cors' (5M downloads), got %q", results[1].Name)
	}
	if results[1].Summary != "CORS for Flask" {
		t.Errorf("expected summary preserved after download update, got %q", results[1].Summary)
	}
}

// TestUpdateDownloadsBatch_CaseInsensitive verifies that download updates
// match packages regardless of casing (e.g. PyPI has "Flask" but the top
// packages dataset has "flask").
func TestUpdateDownloadsBatch_CaseInsensitive(t *testing.T) {
	idx := mustNewIndex(t)

	// Insert with mixed case (as PyPI Simple API returns).
	packages := []PackageEntry{
		{Name: "Flask", Summary: "A micro web framework", Downloads: 0},
		{Name: "Jinja2", Summary: "A template engine", Downloads: 0},
		{Name: "requests", Summary: "HTTP for Humans", Downloads: 0},
	}
	if err := idx.UpsertBatch(packages); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}

	// Update with lowercase names (as top packages dataset returns).
	updates := []PackageEntry{
		{Name: "flask", Downloads: 220_000_000},
		{Name: "jinja2", Downloads: 300_000_000},
		{Name: "requests", Downloads: 500_000_000}, // exact match
	}
	if err := idx.UpdateDownloadsBatch(updates); err != nil {
		t.Fatalf("UpdateDownloadsBatch: %v", err)
	}

	// All three should have non-zero downloads.
	for _, name := range []string{"flask", "jinja2", "requests"} {
		results, err := idx.Search(name, 1)
		if err != nil {
			t.Fatalf("Search(%q): %v", name, err)
		}
		if len(results) == 0 {
			t.Fatalf("Search(%q): no results", name)
		}
		if results[0].Downloads == 0 {
			t.Errorf("Search(%q): expected non-zero downloads after case-insensitive update, got 0 (name in index: %q)", name, results[0].Name)
		}
	}
}

// TestUpsertBatch_NoDuplicates verifies that calling UpsertBatch multiple
// times with the same packages does not create duplicate FTS rows.
func TestUpsertBatch_NoDuplicates(t *testing.T) {
	idx := mustNewIndex(t)

	packages := []PackageEntry{
		{Name: "flask", Summary: "A web framework", Downloads: 0},
		{Name: "django", Summary: "The web framework", Downloads: 0},
	}

	// Insert twice.
	if err := idx.UpsertBatch(packages); err != nil {
		t.Fatalf("first UpsertBatch: %v", err)
	}
	if err := idx.UpsertBatch(packages); err != nil {
		t.Fatalf("second UpsertBatch: %v", err)
	}

	// Search should return exactly 1 result for "django", not 2.
	results, err := idx.Search("django", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'django', got %d (dupes in FTS)", len(results))
	}
}

// TestSearchEmpty verifies that searching on an empty index returns zero
// results without an error.
func TestSearchEmpty(t *testing.T) {
	idx := mustNewIndex(t)

	results, err := idx.Search("nonexistent", 10)
	if err != nil {
		t.Fatalf("Search on empty index: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}
