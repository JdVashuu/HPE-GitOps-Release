package cache

import (
	"sync"
	"time"
)

type cacheItem struct {
	value     interface{}
	expiresAt time.Time
}

// MemoryCache provides thread-safe in-memory caching with TTL-based expiration and read/write workspace locking.
type MemoryCache struct {
	mu    sync.RWMutex
	items map[string]cacheItem

	// lockManager handles repository-wide synchronization
	wsMu sync.RWMutex
}

// NewMemoryCache initializes an empty MemoryCache.
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		items: make(map[string]cacheItem),
	}
}

// Get retrieves an item if present and not expired.
func (c *MemoryCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}
	if time.Now().After(item.expiresAt) {
		return nil, false
	}
	return item.value, true
}

// Set stores an item with a given TTL.
func (c *MemoryCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = cacheItem{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
}

// Invalidate removes a specific key from cache.
func (c *MemoryCache) Invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
}

// Clear flushes all entries from the cache.
func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]cacheItem)
}

// WorkspaceLockManager implementation for workspace synchronization:

func (c *MemoryCache) RLock() {
	c.wsMu.RLock()
}

func (c *MemoryCache) RUnlock() {
	c.wsMu.RUnlock()
}

func (c *MemoryCache) Lock() {
	c.wsMu.Lock()
}

func (c *MemoryCache) Unlock() {
	c.wsMu.Unlock()
}
