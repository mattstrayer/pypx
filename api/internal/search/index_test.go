package search

import (
	"testing"
)

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
