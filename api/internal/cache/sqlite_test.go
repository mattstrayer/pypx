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

func TestCacheBusyTimeout(t *testing.T) {
	c := newTestCache(t)

	var timeout int
	if err := c.db.QueryRow(`PRAGMA busy_timeout`).Scan(&timeout); err != nil {
		t.Fatalf("PRAGMA busy_timeout: %v", err)
	}
	if timeout != 5000 {
		t.Errorf("busy_timeout = %d, want 5000", timeout)
	}
}

func TestPurgeDocsBefore(t *testing.T) {
	c := newTestCache(t)
	keys := []string{
		"docs:httpx:1", "docs-err:httpx:1", "diff:httpx:1:2", "diffjson:httpx:1:2",
		"docs:v1:httpx:1", "docs-err:v1:httpx:1", "diff:v1:httpx:1:2", "diffjson:v1:httpx:1:2",
		"docs:v0:httpx:1", "pkg:httpx", "raw:httpx",
	}
	for _, k := range keys {
		if err := c.Set(k, []byte("x"), 0); err != nil {
			t.Fatal(err)
		}
	}
	n, err := c.PurgeDocsBefore(1)
	if err != nil {
		t.Fatalf("PurgeDocsBefore: %v", err)
	}
	if n != 5 {
		t.Errorf("deleted %d rows, want 5", n)
	}
	gone := map[string]bool{
		"docs:httpx:1": true, "docs-err:httpx:1": true, "diff:httpx:1:2": true,
		"diffjson:httpx:1:2": true, "docs:v0:httpx:1": true,
	}
	for _, k := range keys {
		data, _, err := c.Get(k, 0)
		if err != nil {
			t.Fatal(err)
		}
		if gone[k] && data != nil {
			t.Errorf("%s should be purged", k)
		}
		if !gone[k] && data == nil {
			t.Errorf("%s should be kept", k)
		}
	}
}
