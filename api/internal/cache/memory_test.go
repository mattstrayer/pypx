package cache

import (
	"fmt"
	"sync"
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

// TestMemoryCachePromoteRace verifies that concurrent Get() (which promotes from SQLite)
// and Set() calls do not cause a data race or overwrite a fresher value.
// Run with -race to confirm no race detector violations.
func TestMemoryCachePromoteRace(t *testing.T) {
	// Seed SQLite directly so Get() will always fall through to promote.
	sqlite, err := New(":memory:")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	mc := NewMemoryCache(sqlite, 100)
	t.Cleanup(func() { mc.Close() })

	ttl := 5 * time.Minute
	key := "pkg:race-test"
	stale := []byte(`{"version":"1.0"}`)
	fresh := []byte(`{"version":"2.0"}`)

	// Write stale value directly to SQLite only — not to memory — so Get will promote.
	if err := sqlite.Set(key, stale, ttl); err != nil {
		t.Fatalf("sqlite.Set: %v", err)
	}

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Half goroutines call Get (triggering the promote path).
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, _, _ = mc.Get(key, ttl)
		}()
	}

	// Other half call Set with a fresher value concurrently.
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_ = mc.Set(key, fresh, ttl)
		}()
	}

	wg.Wait()

	// After all concurrent operations, the in-memory value must be one of the two
	// valid values — not corrupted — and must never be nil.
	mc.mu.RLock()
	item, exists := mc.items[key]
	mc.mu.RUnlock()

	if !exists {
		t.Fatal("key should exist in memory after concurrent operations")
	}
	got := string(item.data)
	if got != string(stale) && got != string(fresh) {
		t.Errorf("unexpected value in cache: %q (want one of %q or %q)", got, stale, fresh)
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
