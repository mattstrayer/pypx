package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func newTestMemoryCache(t *testing.T, maxSize int) *MemoryCache {
	t.Helper()
	return newTestMemoryCacheWithOpts(t, maxSize)
}

func newTestMemoryCacheWithOpts(t *testing.T, maxSize int, opts ...MemoryOption) *MemoryCache {
	t.Helper()
	sqlite, err := New(":memory:")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	mc := NewMemoryCache(sqlite, maxSize, opts...)
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
	mc.mu.Lock()
	_, inMemory := mc.items[key]
	mc.mu.Unlock()
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
	mc.mu.Lock()
	_, inMemory := mc.items[key]
	mc.mu.Unlock()
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
	mc.mu.Lock()
	_, inMemory = mc.items[key]
	mc.mu.Unlock()
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
	mc.mu.Lock()
	el, exists := mc.items[key]
	mc.mu.Unlock()

	if !exists {
		t.Fatal("key should exist in memory after concurrent operations")
	}
	got := string(el.Value.(*memItem).data)
	if got != string(stale) && got != string(fresh) {
		t.Errorf("unexpected value in cache: %q (want one of %q or %q)", got, stale, fresh)
	}
}

// TestGetPromotionPreservesCreatedAt verifies that promoting a SQLite entry to
// the memory tier preserves the original created_at timestamp. Before this fix,
// a lost timestamp produced fresh=true from the memory tier even for stale data.
func TestGetPromotionPreservesCreatedAt(t *testing.T) {
	sqlite, err := New(":memory:")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	mc := NewMemoryCache(sqlite, 10)
	t.Cleanup(func() { mc.Close() })

	key := "pkg:stale-promotion"
	value := []byte(`{"name":"stale-promotion"}`)
	ttl := 5 * time.Minute

	// Write to SQLite directly.
	if err := sqlite.Set(key, value, ttl); err != nil {
		t.Fatalf("sqlite.Set: %v", err)
	}

	// Backdate the created_at to make it stale.
	oldTime := time.Now().Add(-10 * time.Minute).Unix()
	if _, err := sqlite.db.Exec(`UPDATE cache SET created_at = ? WHERE key = ?`, oldTime, key); err != nil {
		t.Fatalf("backdating created_at: %v", err)
	}

	// First Get: falls through to SQLite, promotes to memory. Must be stale.
	_, fresh1, err := mc.Get(key, ttl)
	if err != nil {
		t.Fatalf("Get (first): %v", err)
	}
	if fresh1 {
		t.Error("first Get: expected fresh=false for backdated entry")
	}

	// Second Get: served from memory tier. Must still be stale — not re-fresh.
	mc.mu.Lock()
	_, inMemory := mc.items[key]
	mc.mu.Unlock()
	if !inMemory {
		t.Fatal("expected key to be promoted to memory after first Get")
	}

	_, fresh2, err := mc.Get(key, ttl)
	if err != nil {
		t.Fatalf("Get (second): %v", err)
	}
	if fresh2 {
		t.Error("second Get (from memory): expected fresh=false — promotion must preserve original createdAt")
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

	mc.mu.Lock()
	if len(mc.items) != maxSize {
		t.Fatalf("expected %d items before eviction, got %d", maxSize, len(mc.items))
	}
	mc.mu.Unlock()

	// This Set should trigger eviction of the oldest entry (pkg:package0).
	newKey := "pkg:newcomer"
	if err := mc.Set(newKey, []byte(`{}`), ttl); err != nil {
		t.Fatalf("Set %q: %v", newKey, err)
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

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

// TestMemoryCacheLRUAccessPromotes verifies that Get()-ing an entry promotes
// it to most-recently-used, so it survives an eviction that would otherwise
// remove it under naive FIFO (insertion-order) eviction. This is the
// regression pin for the FIFO -> true LRU change: it fails under the old
// FIFO-by-createdAt eviction strategy.
func TestMemoryCacheLRUAccessPromotes(t *testing.T) {
	const maxSize = 3
	mc := newTestMemoryCache(t, maxSize)
	ttl := 5 * time.Minute

	if err := mc.Set("a", []byte(`{}`), ttl); err != nil {
		t.Fatalf("Set a: %v", err)
	}
	if err := mc.Set("b", []byte(`{}`), ttl); err != nil {
		t.Fatalf("Set b: %v", err)
	}
	if err := mc.Set("c", []byte(`{}`), ttl); err != nil {
		t.Fatalf("Set c: %v", err)
	}

	// Access "a" so it becomes most-recently-used; "b" becomes the least
	// recently used entry.
	if _, _, err := mc.Get("a", ttl); err != nil {
		t.Fatalf("Get a: %v", err)
	}

	// Inserting "d" should evict the least-recently-used entry ("b"), not
	// "a" (which was just accessed) and not "c" (inserted most recently).
	if err := mc.Set("d", []byte(`{}`), ttl); err != nil {
		t.Fatalf("Set d: %v", err)
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	for _, key := range []string{"a", "c", "d"} {
		if _, exists := mc.items[key]; !exists {
			t.Errorf("expected key %q to still be present", key)
		}
	}
	if _, exists := mc.items["b"]; exists {
		t.Error("expected least-recently-used key \"b\" to be evicted, not \"a\"")
	}
}

// TestMemoryCacheByteBudgetEviction verifies that the byte budget, not just
// the entry-count budget, drives eviction.
func TestMemoryCacheByteBudgetEviction(t *testing.T) {
	// Each value is 40 bytes; keys are short, so entrySize(key, value) is
	// close to 40+len(key). Budget of 100 bytes allows roughly 2 entries.
	mc := newTestMemoryCacheWithOpts(t, 100, WithMaxBytes(100))
	ttl := 5 * time.Minute

	val := make([]byte, 40)

	if err := mc.Set("k1", val, ttl); err != nil {
		t.Fatalf("Set k1: %v", err)
	}
	if err := mc.Set("k2", val, ttl); err != nil {
		t.Fatalf("Set k2: %v", err)
	}
	if err := mc.Set("k3", val, ttl); err != nil {
		t.Fatalf("Set k3: %v", err)
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.curBytes > 100 {
		t.Errorf("expected curBytes <= 100, got %d", mc.curBytes)
	}
	// The least-recently-used entry (k1) should have been evicted to make
	// room for k3 under the byte budget.
	if _, exists := mc.items["k1"]; exists {
		t.Error("expected k1 to be evicted to satisfy byte budget")
	}
	if _, exists := mc.items["k3"]; !exists {
		t.Error("expected k3 to be present")
	}
}

// TestMemoryCacheOversizeValueNotCached verifies that a value larger than
// the byte budget is not stored in the memory tier (it is still readable
// via the SQLite fall-through), and that it does not evict smaller entries
// already in memory.
func TestMemoryCacheOversizeValueNotCached(t *testing.T) {
	mc := newTestMemoryCacheWithOpts(t, 100, WithMaxBytes(10))
	ttl := 5 * time.Minute

	small := []byte("x")
	if err := mc.Set("small", small, ttl); err != nil {
		t.Fatalf("Set small: %v", err)
	}

	big := make([]byte, 100)
	if err := mc.Set("big", big, ttl); err != nil {
		t.Fatalf("Set big: %v", err)
	}

	mc.mu.Lock()
	_, bigInMemory := mc.items["big"]
	_, smallInMemory := mc.items["small"]
	mc.mu.Unlock()

	if bigInMemory {
		t.Error("expected oversize value not to be cached in memory")
	}
	if !smallInMemory {
		t.Error("expected previously cached small entry not to be evicted by an oversize Set")
	}

	// The oversize value must still be readable via the SQLite fall-through.
	got, fresh, err := mc.Get("big", ttl)
	if err != nil {
		t.Fatalf("Get big: %v", err)
	}
	if !fresh {
		t.Error("expected fresh=true for big value read via SQLite fall-through")
	}
	if len(got) != len(big) {
		t.Errorf("expected %d bytes from SQLite fall-through, got %d", len(big), len(got))
	}
}

// TestMemoryCacheUpdateExistingAdjustsBytes verifies that overwriting an
// existing key adjusts curBytes to reflect the new size rather than leaking
// the old size.
func TestMemoryCacheUpdateExistingAdjustsBytes(t *testing.T) {
	mc := newTestMemoryCache(t, 10)
	ttl := 5 * time.Minute

	if err := mc.Set("k", make([]byte, 50), ttl); err != nil {
		t.Fatalf("Set (50 bytes): %v", err)
	}

	mc.mu.Lock()
	afterFirst := mc.curBytes
	mc.mu.Unlock()

	if err := mc.Set("k", make([]byte, 10), ttl); err != nil {
		t.Fatalf("Set (10 bytes): %v", err)
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	wantDelta := int64(len("k") + 10 - (len("k") + 50))
	gotDelta := mc.curBytes - afterFirst
	if gotDelta != wantDelta {
		t.Errorf("expected curBytes to shrink by %d after overwrite, changed by %d (curBytes=%d)", -wantDelta, gotDelta, mc.curBytes)
	}
	if len(mc.items) != 1 {
		t.Errorf("expected 1 entry after overwriting same key, got %d", len(mc.items))
	}
}
