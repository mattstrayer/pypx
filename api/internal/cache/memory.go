package cache

import (
	"container/list"
	"sync"
	"time"
)

// defaultMaxBytes is the default approximate byte budget for the in-memory
// cache tier, used when no WithMaxBytes option is supplied.
const defaultMaxBytes = 128 << 20 // 128 MiB

// memItem holds a single in-memory cache entry. It is stored as the Value
// of a container/list.Element so recency order can be tracked in O(1).
type memItem struct {
	key       string
	data      []byte
	createdAt time.Time
}

// MemoryCache is an access-ordered (true LRU) in-memory cache with TTL,
// bounded by both entry count and an approximate total-byte budget.
// It wraps a Cache (SQLite) and checks memory first, promoting hits from
// SQLite to memory.
type MemoryCache struct {
	sqlite *Cache

	// mu guards ll/items/curBytes. This is a full Mutex rather than an
	// RWMutex: Get() mutates recency order (MoveToFront) on every hit, so
	// even reads need the exclusive lock. The critical section is a map
	// lookup plus an O(1) list splice — far cheaper than the O(n) full-map
	// scan the previous insertion-order eviction required, so contention is not a
	// concern despite losing the RLock fast path.
	mu sync.Mutex

	ll         *list.List // front = most recently used, back = least recently used
	items      map[string]*list.Element
	maxEntries int
	maxBytes   int64
	curBytes   int64
}

// MemoryOption configures optional MemoryCache parameters.
type MemoryOption func(*MemoryCache)

// WithMaxBytes sets the approximate total-byte budget for the in-memory
// cache tier. Entry size is approximated as len(key)+len(data).
func WithMaxBytes(n int64) MemoryOption {
	return func(mc *MemoryCache) { mc.maxBytes = n }
}

// NewMemoryCache creates a MemoryCache backed by sqliteCache with a capacity
// of maxSize entries and, by default, a defaultMaxBytes byte budget. Use
// WithMaxBytes to override the byte budget.
func NewMemoryCache(sqliteCache *Cache, maxSize int, opts ...MemoryOption) *MemoryCache {
	mc := &MemoryCache{
		sqlite:     sqliteCache,
		ll:         list.New(),
		items:      make(map[string]*list.Element, maxSize),
		maxEntries: maxSize,
		maxBytes:   defaultMaxBytes,
	}
	for _, o := range opts {
		o(mc)
	}
	return mc
}

// entrySize approximates the memory footprint of a cache entry.
func entrySize(key string, data []byte) int64 {
	return int64(len(key) + len(data))
}

// Get checks memory first, then falls through to SQLite.
// If found in SQLite but not memory, promotes to memory.
func (mc *MemoryCache) Get(key string, ttl time.Duration) (data []byte, fresh bool, err error) {
	mc.mu.Lock()
	el, ok := mc.items[key]
	var itemData []byte
	var itemCreatedAt time.Time
	if ok {
		mc.ll.MoveToFront(el)
		item := el.Value.(*memItem)
		// Copy fields while holding the lock: addLocked mutates an
		// existing memItem in place, so reading item.data/createdAt
		// after unlocking would race with a concurrent Set.
		itemData = item.data
		itemCreatedAt = item.createdAt
	}
	mc.mu.Unlock()

	if ok {
		age := time.Since(itemCreatedAt)
		return itemData, age < ttl, nil
	}

	// Fall through to SQLite — reads value and created_at atomically.
	var createdAt time.Time
	data, fresh, createdAt, err = mc.sqlite.getWithTime(key, ttl)
	if err != nil || data == nil {
		return data, fresh, err
	}

	// Promote to memory cache with double-check to avoid overwriting
	// a fresher write that happened between unlock and re-lock.
	// Only promote if we have a valid timestamp — never fabricate freshness.
	if !createdAt.IsZero() {
		mc.mu.Lock()
		if _, exists := mc.items[key]; !exists {
			mc.addLocked(key, data, createdAt)
		}
		mc.mu.Unlock()
	}

	return data, fresh, nil
}

// Set writes to both memory and SQLite.
func (mc *MemoryCache) Set(key string, value []byte, ttl time.Duration) error {
	mc.mu.Lock()
	mc.addLocked(key, value, time.Now())
	mc.mu.Unlock()

	return mc.sqlite.Set(key, value, ttl)
}

// addLocked inserts or updates an entry, marking it most-recently-used, then
// evicts from the back until both the entry-count and byte budgets are
// satisfied. Must be called with mc.mu held.
func (mc *MemoryCache) addLocked(key string, data []byte, createdAt time.Time) {
	size := entrySize(key, data)

	// Oversize entries are not cached in memory; SQLite still holds them.
	if size > mc.maxBytes {
		return
	}

	if el, exists := mc.items[key]; exists {
		item := el.Value.(*memItem)
		mc.curBytes += size - entrySize(item.key, item.data)
		item.data = data
		item.createdAt = createdAt
		mc.ll.MoveToFront(el)
	} else {
		item := &memItem{key: key, data: data, createdAt: createdAt}
		el := mc.ll.PushFront(item)
		mc.items[key] = el
		mc.curBytes += size
	}

	for (mc.ll.Len() > mc.maxEntries || mc.curBytes > mc.maxBytes) && mc.ll.Len() > 0 {
		mc.removeElement(mc.ll.Back())
	}
}

// removeElement removes el from the list and map, adjusting curBytes.
// Must be called with mc.mu held.
func (mc *MemoryCache) removeElement(el *list.Element) {
	item := el.Value.(*memItem)
	delete(mc.items, item.key)
	mc.ll.Remove(el)
	mc.curBytes -= entrySize(item.key, item.data)
}

// Delete removes a key from both the memory layer and the underlying SQLite cache.
func (mc *MemoryCache) Delete(key string) error {
	mc.mu.Lock()
	if el, exists := mc.items[key]; exists {
		mc.removeElement(el)
	}
	mc.mu.Unlock()
	return mc.sqlite.Delete(key)
}

// Close closes the underlying SQLite cache.
func (mc *MemoryCache) Close() error {
	return mc.sqlite.Close()
}
