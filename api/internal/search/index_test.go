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

// TestNewIndex_RebuildsFTSFromMeta verifies that opening an index with
// existing meta data rebuilds the FTS table correctly, preserving
// download counts and deduplicating any stale FTS rows.
func TestNewIndex_RebuildsFTSFromMeta(t *testing.T) {
	// Use a temp file so we can close and reopen the index.
	tmpFile := t.TempDir() + "/test.db"

	// First: create index, insert data, update downloads.
	idx1, err := NewIndex(tmpFile)
	if err != nil {
		t.Fatalf("NewIndex(1): %v", err)
	}
	if err := idx1.UpsertBatch([]PackageEntry{
		{Name: "flask", Summary: "A web framework", Downloads: 0},
		{Name: "django", Summary: "The web framework", Downloads: 0},
	}); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}
	if err := idx1.UpdateDownloadsBatch([]PackageEntry{
		{Name: "flask", Downloads: 220_000_000},
		{Name: "django", Downloads: 100_000_000},
	}); err != nil {
		t.Fatalf("UpdateDownloadsBatch: %v", err)
	}
	idx1.Close()

	// Second: reopen the index (triggers FTS rebuild from meta).
	idx2, err := NewIndex(tmpFile)
	if err != nil {
		t.Fatalf("NewIndex(2): %v", err)
	}
	defer idx2.Close()

	// Search should still work and downloads should be preserved.
	results, err := idx2.Search("flask", 1)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("no results after FTS rebuild")
	}
	if results[0].Name != "flask" {
		t.Errorf("expected 'flask', got %q", results[0].Name)
	}
	if results[0].Downloads != 220_000_000 {
		t.Errorf("expected downloads=220000000 after rebuild, got %d", results[0].Downloads)
	}

	// No duplicates after rebuild.
	results, _ = idx2.Search("django", 10)
	if len(results) != 1 {
		t.Errorf("expected 1 result for django after rebuild, got %d", len(results))
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

// TestTopByDownloads verifies that the top packages are returned in descending
// download order, filtered to only packages with downloads > 0.
func TestTopByDownloads(t *testing.T) {
	idx := mustNewIndex(t)

	packages := []PackageEntry{
		{Name: "numpy", Summary: "Scientific computing", Downloads: 80_000_000},
		{Name: "requests", Summary: "HTTP for Humans", Downloads: 50_000_000},
		{Name: "obscure-pkg", Summary: "Not in top list", Downloads: 0},
		{Name: "flask", Summary: "A micro web framework", Downloads: 30_000_000},
	}
	if err := idx.UpsertBatch(packages); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}

	results, err := idx.TopByDownloads(3)
	if err != nil {
		t.Fatalf("TopByDownloads: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Name != "numpy" {
		t.Errorf("expected first result 'numpy', got %q", results[0].Name)
	}
	if results[1].Name != "requests" {
		t.Errorf("expected second result 'requests', got %q", results[1].Name)
	}
	if results[2].Name != "flask" {
		t.Errorf("expected third result 'flask', got %q", results[2].Name)
	}
	// obscure-pkg must not appear (downloads = 0)
	for _, r := range results {
		if r.Name == "obscure-pkg" {
			t.Errorf("expected 'obscure-pkg' (downloads=0) to be excluded")
		}
	}
}

// TestTopByDownloads_Empty verifies an empty result set when no packages have
// downloads > 0.
func TestTopByDownloads_Empty(t *testing.T) {
	idx := mustNewIndex(t)

	if err := idx.UpsertBatch([]PackageEntry{
		{Name: "foo", Summary: "A package", Downloads: 0},
	}); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}

	results, err := idx.TopByDownloads(10)
	if err != nil {
		t.Fatalf("TopByDownloads: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// TestIndexBusyTimeout verifies that the busy_timeout PRAGMA is configured to
// 5000ms so that concurrent writers wait instead of immediately returning SQLITE_BUSY.
func TestIndexBusyTimeout(t *testing.T) {
	idx := mustNewIndex(t)

	var timeout int
	if err := idx.db.QueryRow(`PRAGMA busy_timeout`).Scan(&timeout); err != nil {
		t.Fatalf("PRAGMA busy_timeout: %v", err)
	}
	if timeout != 5000 {
		t.Errorf("busy_timeout = %d, want 5000", timeout)
	}
}

// TestTopByDownloads_DefaultLimit verifies that calling TopByDownloads(0)
// uses the default limit of 12 and returns results in correct order.
func TestTopByDownloads_DefaultLimit(t *testing.T) {
	idx := mustNewIndex(t)

	// Insert 15 packages with non-zero downloads.
	packages := make([]PackageEntry, 15)
	for i := range packages {
		packages[i] = PackageEntry{
			Name:      "pkg-" + string(rune('a'+i)),
			Summary:   "A package",
			Downloads: int64(1000 * (15 - i)), // descending: 15000, 14000, ..., 1000
		}
	}
	if err := idx.UpsertBatch(packages); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}

	results, err := idx.TopByDownloads(0)
	if err != nil {
		t.Fatalf("TopByDownloads(0): %v", err)
	}

	if len(results) != 12 {
		t.Errorf("TopByDownloads(0) should default to 12 results, got %d", len(results))
	}

	// Verify first result has the highest downloads.
	if results[0].Downloads != 15000 {
		t.Errorf("expected first result downloads=15000, got %d", results[0].Downloads)
	}

	// Verify ordering is descending by downloads.
	for i := 1; i < len(results); i++ {
		if results[i].Downloads > results[i-1].Downloads {
			t.Errorf("results not in descending order: results[%d]=%d > results[%d]=%d",
				i, results[i].Downloads, i-1, results[i-1].Downloads)
		}
	}
}
