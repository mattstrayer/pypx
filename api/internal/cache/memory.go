package cache

import (
	"sync"
	"time"
)

// memItem holds a single in-memory cache entry.
type memItem struct {
	data      []byte
	createdAt time.Time
}

// MemoryCache is a simple in-memory LRU cache with TTL.
// It wraps a Cache (SQLite) and checks memory first.
type MemoryCache struct {
	sqlite  *Cache
	items   map[string]*memItem
	mu      sync.RWMutex
	maxSize int
}

// NewMemoryCache creates a MemoryCache backed by sqliteCache with a capacity of maxSize entries.
func NewMemoryCache(sqliteCache *Cache, maxSize int) *MemoryCache {
	return &MemoryCache{
		sqlite:  sqliteCache,
		items:   make(map[string]*memItem, maxSize),
		maxSize: maxSize,
	}
}

// Get checks memory first, then falls through to SQLite.
// If found in SQLite but not memory, promotes to memory.
func (mc *MemoryCache) Get(key string, ttl time.Duration) (data []byte, fresh bool, err error) {
	mc.mu.RLock()
	item, ok := mc.items[key]
	mc.mu.RUnlock()

	if ok {
		age := time.Since(item.createdAt)
		return item.data, age < ttl, nil
	}

	// Fall through to SQLite.
	data, fresh, err = mc.sqlite.Get(key, ttl)
	if err != nil || data == nil {
		return data, fresh, err
	}

	// Promote to memory cache.
	mc.mu.Lock()
	mc.items[key] = &memItem{data: data, createdAt: time.Now()}
	mc.mu.Unlock()

	return data, fresh, nil
}

// Set writes to both memory and SQLite.
func (mc *MemoryCache) Set(key string, value []byte, ttl time.Duration) error {
	mc.mu.Lock()
	if len(mc.items) >= mc.maxSize {
		mc.evictOldest()
	}
	mc.items[key] = &memItem{data: value, createdAt: time.Now()}
	mc.mu.Unlock()

	return mc.sqlite.Set(key, value, ttl)
}

// evictOldest removes the entry with the oldest createdAt. Must be called with mc.mu held (write lock).
func (mc *MemoryCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for k, v := range mc.items {
		if oldestKey == "" || v.createdAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.createdAt
		}
	}

	if oldestKey != "" {
		delete(mc.items, oldestKey)
	}
}

// Close closes the underlying SQLite cache.
func (mc *MemoryCache) Close() error {
	return mc.sqlite.Close()
}
