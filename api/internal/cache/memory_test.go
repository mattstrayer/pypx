package cache

import (
	"fmt"
	"testing"
	"time"
)

func newTestMemoryCache(t *testing.T, maxSize int) *MemoryCache {
	t.Helper()
	sqlite, err := New(":memory:")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	mc := NewMemoryCache(sqlite, maxSize)
	t.Cleanup(func() { mc.Close() })
	return mc
}

func TestMemoryCacheHit(t *testing.T) {
	mc := newTestMemoryCache(t, 10)
	ttl := 5 * time.Minute

	key := "pkg:requests"
	want := []byte(`{"name":"requests"}`)

	if err := mc.Set(key, want, ttl); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, fresh, err := mc.Get(key, ttl)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("data mismatch: got %q, want %q", got, want)
	}
	if !fresh {
		t.Error("expected fresh=true for newly-set entry")
	}

	// Verify item is in the in-memory map (not just SQLite).
	mc.mu.RLock()
	_, inMemory := mc.items[key]
	mc.mu.RUnlock()
	if !inMemory {
		t.Error("expected key to be present in in-memory map after Set")
	}
}

func TestMemoryCacheFallthrough(t *testing.T) {
	sqlite, err := New(":memory:")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ttl := 5 * time.Minute
	key := "pkg:flask"
	want := []byte(`{"name":"flask"}`)

	// Write directly to SQLite, bypassing MemoryCache.
	if err := sqlite.Set(key, want, ttl); err != nil {
		t.Fatalf("sqlite.Set: %v", err)
	}

	mc := NewMemoryCache(sqlite, 10)
	t.Cleanup(func() { mc.Close() })

	// Key is not in memory yet.
	mc.mu.RLock()
	_, inMemory := mc.items[key]
	mc.mu.RUnlock()
	if inMemory {
		t.Fatal("key should not be in memory before first Get")
	}

	got, fresh, err := mc.Get(key, ttl)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("data mismatch: got %q, want %q", got, want)
	}
	if !fresh {
		t.Error("expected fresh=true")
	}

	// Now the key should have been promoted to memory.
	mc.mu.RLock()
	_, inMemory = mc.items[key]
	mc.mu.RUnlock()
	if !inMemory {
		t.Error("expected key to be promoted to in-memory map after fallthrough Get")
	}
}

func TestMemoryCacheEviction(t *testing.T) {
	const maxSize = 5
	mc := newTestMemoryCache(t, maxSize)
	ttl := 5 * time.Minute

	// Fill to capacity, inserting with small sleeps so createdAt differs.
	for i := 0; i < maxSize; i++ {
		key := fmt.Sprintf("pkg:package%d", i)
		if err := mc.Set(key, []byte(`{}`), ttl); err != nil {
			t.Fatalf("Set %q: %v", key, err)
		}
		time.Sleep(2 * time.Millisecond)
	}

	mc.mu.RLock()
	if len(mc.items) != maxSize {
		t.Fatalf("expected %d items before eviction, got %d", maxSize, len(mc.items))
	}
	mc.mu.RUnlock()

	// This Set should trigger eviction of the oldest entry (pkg:package0).
	newKey := "pkg:newcomer"
	if err := mc.Set(newKey, []byte(`{}`), ttl); err != nil {
		t.Fatalf("Set %q: %v", newKey, err)
	}

	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if len(mc.items) != maxSize {
		t.Errorf("expected %d items after eviction, got %d", maxSize, len(mc.items))
	}
	if _, exists := mc.items["pkg:package0"]; exists {
		t.Error("expected oldest entry pkg:package0 to be evicted")
	}
	if _, exists := mc.items[newKey]; !exists {
		t.Errorf("expected new key %q to be present after eviction", newKey)
	}
}
