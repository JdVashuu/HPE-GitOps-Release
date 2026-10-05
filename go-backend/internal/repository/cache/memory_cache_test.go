package cache

import (
	"testing"
	"time"
)

func TestMemoryCache_GetSet(t *testing.T) {
	c := NewMemoryCache()

	c.Set("key1", "val1", 500*time.Millisecond)

	val, found := c.Get("key1")
	if !found || val != "val1" {
		t.Fatalf("expected val1, got %v", val)
	}

	// Wait for expiration
	time.Sleep(550 * time.Millisecond)
	_, foundAfter := c.Get("key1")
	if foundAfter {
		t.Fatalf("expected key1 to expire")
	}
}

func TestMemoryCache_Locking(t *testing.T) {
	c := NewMemoryCache()

	c.RLock()
	c.RUnlock()

	c.Lock()
	c.Unlock()
}
