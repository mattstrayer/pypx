package cache

import (
	"testing"
	"time"
)

func newTestCache(t *testing.T) *Cache {
	t.Helper()
	c, err := New(":memory:")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestCacheSetAndGet(t *testing.T) {
	c := newTestCache(t)

	key := "pkg:requests"
	want := []byte(`{"name":"requests"}`)
	ttl := 5 * time.Minute

	if err := c.Set(key, want, ttl); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, fresh, err := c.Get(key, ttl)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("data mismatch: got %q, want %q", got, want)
	}
	if !fresh {
		t.Error("expected fresh=true for newly-set entry")
	}
}

func TestCacheStale(t *testing.T) {
	c := newTestCache(t)

	key := "pkg:flask"
	value := []byte(`{"name":"flask"}`)
	ttl := 50 * time.Millisecond

	if err := c.Set(key, value, ttl); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Wait long enough for the entry to become stale.
	time.Sleep(100 * time.Millisecond)

	got, fresh, err := c.Get(key, ttl)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("expected stale data to still be returned, got nil")
	}
	if string(got) != string(value) {
		t.Errorf("data mismatch: got %q, want %q", got, value)
	}
	if fresh {
		t.Error("expected fresh=false for expired entry")
	}
}

func TestCacheMiss(t *testing.T) {
	c := newTestCache(t)

	data, fresh, err := c.Get("nonexistent-key", time.Minute)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if data != nil {
		t.Errorf("expected nil data for cache miss, got %q", data)
	}
	if fresh {
		t.Error("expected fresh=false for cache miss")
	}
}
