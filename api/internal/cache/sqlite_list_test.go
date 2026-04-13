package cache_test

import (
	"testing"

	"github.com/pypx/api/internal/cache"
)

func TestListPackageNames(t *testing.T) {
	c, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("new cache: %v", err)
	}
	defer c.Close()

	// Seed: two package keys and one non-package key.
	if err := c.Set("pkg:requests", []byte(`{}`), 0); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := c.Set("pkg:flask", []byte(`{}`), 0); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := c.Set("stats:requests:4w", []byte(`{}`), 0); err != nil {
		t.Fatalf("set: %v", err)
	}

	names, err := c.ListPackageNames()
	if err != nil {
		t.Fatalf("ListPackageNames: %v", err)
	}

	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d: %v", len(names), names)
	}

	got := make(map[string]bool)
	for _, n := range names {
		got[n] = true
	}
	for _, want := range []string{"requests", "flask"} {
		if !got[want] {
			t.Errorf("expected %q in names, got %v", want, names)
		}
	}
}
