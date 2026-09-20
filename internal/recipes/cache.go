package recipes

import (
	"sync"
	"time"
)

// cacheTTL is an hour because Spoonacular's terms forbid keeping its data
// longer than that on the free plan.
const cacheTTL = time.Hour

// cache is a small in-memory store of lookups. The API is billed by the point
// and the coach asks about the same pantry again and again, so repeats are free.
type cache struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]cacheEntry
}

type cacheEntry struct {
	value   any
	expires time.Time
}

func newCache() *cache {
	return &cache{now: time.Now, entries: make(map[string]cacheEntry)}
}

func (c *cache) get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || !c.now().Before(e.expires) {
		delete(c.entries, key)
		return nil, false
	}
	return e.value, true
}

func (c *cache) put(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cacheEntry{value: value, expires: c.now().Add(cacheTTL)}
}
