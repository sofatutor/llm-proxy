package proxy

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

type cachedResponse struct {
	statusCode int
	headers    http.Header
	body       []byte
	expiresAt  time.Time
	vary       string // Vary header from upstream response for per-response cache key generation
}

// httpCache is a minimal cache interface used by the proxy cache layer.
// Implementations must be safe for concurrent use.
type httpCache interface {
	Get(key string) (cachedResponse, bool)
	Set(key string, value cachedResponse)
	Purge(key string) bool                        // Remove exact key
	PurgePrefix(prefix string) int                // Remove all keys with prefix, return count
}

type inMemoryCache struct {
	mu    sync.RWMutex
	store map[string]cachedResponse
}

func newInMemoryCache() *inMemoryCache {
	return &inMemoryCache{store: make(map[string]cachedResponse)}
}

func (c *inMemoryCache) Get(key string) (cachedResponse, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.store[key]
	if !ok {
		return cachedResponse{}, false
	}
	if time.Now().After(v.expiresAt) {
		return cachedResponse{}, false
	}
	return v, true
}

func (c *inMemoryCache) Set(key string, value cachedResponse) {
	c.mu.Lock()
	c.store[key] = value
	c.mu.Unlock()
}

func (c *inMemoryCache) Purge(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, exists := c.store[key]
	if exists {
		delete(c.store, key)
	}
	return exists
}

func (c *inMemoryCache) PurgePrefix(prefix string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := 0
	for key := range c.store {
		if strings.HasPrefix(key, prefix) {
			delete(c.store, key)
			count++
		}
	}
	return count
}
