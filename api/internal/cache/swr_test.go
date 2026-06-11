package cache_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pypx/api/internal/cache"
)

// fakeCache is a minimal in-memory Cacher stub for unit testing.
type fakeCache struct {
	mu   sync.Mutex
	data map[string][]byte
	sets atomic.Int32
}

func newFakeCache() *fakeCache {
	return &fakeCache{data: make(map[string][]byte)}
}

func (f *fakeCache) Get(key string, _ time.Duration) ([]byte, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.data[key]
	if !ok {
		return nil, false, nil
	}
	return v, true, nil
}

func (f *fakeCache) Set(key string, value []byte, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sets.Add(1)
	f.data[key] = value
	return nil
}

func (f *fakeCache) Delete(key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.data, key)
	return nil
}

func (f *fakeCache) Close() error { return nil }

// TestRefreshInBackground_DeduplicatesConcurrentCalls fires 50 concurrent
// RefreshInBackground calls for the same key with a gated fetch and asserts
// the fetch ran exactly once after release.
func TestRefreshInBackground_DeduplicatesConcurrentCalls(t *testing.T) {
	const concurrency = 50
	const key = "pkg:testdedup"

	fc := newFakeCache()
	var fetchCount atomic.Int32
	gate := make(chan struct{})

	fetch := func() ([]byte, error) {
		fetchCount.Add(1)
		<-gate // block until released
		return []byte(`{"ok":true}`), nil
	}

	var wg sync.WaitGroup
	wg.Add(concurrency)
	ready := make(chan struct{})

	for range concurrency {
		go func() {
			defer wg.Done()
			<-ready
			cache.RefreshInBackground(fc, key, time.Hour, fetch)
		}()
	}

	close(ready)
	// Give all goroutines time to enter the singleflight group.
	time.Sleep(50 * time.Millisecond)

	// Release the gate so all blocked fetch goroutines can proceed.
	close(gate)

	// Wait until the cache receives the Set (background goroutine finishes).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fc.sets.Load() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	wg.Wait()
	// Allow any trailing goroutines to finish.
	time.Sleep(50 * time.Millisecond)

	if got := fetchCount.Load(); got != 1 {
		t.Errorf("expected fetch to be called exactly once, got %d", got)
	}
	if got := fc.sets.Load(); got != 1 {
		t.Errorf("expected cache.Set to be called exactly once, got %d", got)
	}
}

// TestRefreshInBackground_FailedFetchDoesNotWrite verifies that a failed fetch
// does not call cache.Set.
func TestRefreshInBackground_FailedFetchDoesNotWrite(t *testing.T) {
	const key = "pkg:testfail"

	fc := newFakeCache()
	done := make(chan struct{})

	fetch := func() ([]byte, error) {
		defer close(done)
		return nil, errors.New("upstream unavailable")
	}

	cache.RefreshInBackground(fc, key, time.Hour, fetch)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("fetch was not called within 2s")
	}

	// Small grace period for the goroutine to finish.
	time.Sleep(20 * time.Millisecond)

	if got := fc.sets.Load(); got != 0 {
		t.Errorf("expected cache.Set to not be called on fetch error, got %d calls", got)
	}
}
