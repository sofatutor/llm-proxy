package proxy

import (
	"net/http"
	"sync"
	"time"
)

type cachedResponse struct {
	statusCode int
	headers    http.Header
	body       []byte
	expiresAt  time.Time
}

// httpCache is a minimal cache interface used by the proxy cache layer.
// Implementations must be safe for concurrent use.
type httpCache interface {
	Get(key string) (cachedResponse, bool)
	Set(key string, value cachedResponse)
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
