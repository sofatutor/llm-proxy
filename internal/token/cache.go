package token

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"time"
)

// CacheEntry represents a cached token data with expiration
type CacheEntry struct {
	Data       TokenData
	ValidUntil time.Time
}

// cacheEntry is a heap entry for eviction, with index for fast updates
// and tokenID for lookup.
type cacheEntry struct {
	tokenID    string
	validUntil time.Time
	insertedAt int64 // strictly increasing for FIFO eviction
	index      int   // index in the heap
}

type cacheEntryHeap []*cacheEntry

func (h cacheEntryHeap) Len() int           { return len(h) }
func (h cacheEntryHeap) Less(i, j int) bool { return h[i].insertedAt < h[j].insertedAt } // FIFO eviction
func (h cacheEntryHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *cacheEntryHeap) Push(x interface{}) {
	entry := x.(*cacheEntry)
	entry.index = len(*h)
	*h = append(*h, entry)
}

func (h *cacheEntryHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*h = old[0 : n-1]
	return item
}

// CachedValidator wraps a TokenValidator with caching
type CachedValidator struct {
	validator    TokenValidator
	cache        map[string]CacheEntry
	cacheMutex   sync.RWMutex
	cacheTTL     time.Duration
	maxCacheSize int

	// Min-heap for eviction
	heap      cacheEntryHeap
	heapIndex map[string]*cacheEntry // tokenID -> *cacheEntry

	insertCounter int64 // strictly increasing counter for insertedAt

	// For cache stats
	hits       int
	misses     int
	evictions  int
	statsMutex sync.Mutex
}

// CacheOptions defines the options for the token cache
type CacheOptions struct {
	// Time-to-live for cache entries (default: 5 minutes)
	TTL time.Duration

	// Maximum size of the cache (default: 1000)
	MaxSize int

	// Whether to enable automatic cache cleanup (default: true)
	EnableCleanup bool

	// Interval for cache cleanup (default: 1 minute)
	CleanupInterval time.Duration
}

// DefaultCacheOptions returns the default cache options
func DefaultCacheOptions() CacheOptions {
	return CacheOptions{
		TTL:             5 * time.Minute,
		MaxSize:         1000,
		EnableCleanup:   true,
		CleanupInterval: 1 * time.Minute,
	}
}

// NewCachedValidator creates a new validator with caching
func NewCachedValidator(validator TokenValidator, options ...CacheOptions) *CachedValidator {
	opts := DefaultCacheOptions()
	if len(options) > 0 {
		opts = options[0]
	}

	cv := &CachedValidator{
		validator:    validator,
		cache:        make(map[string]CacheEntry),
		cacheTTL:     opts.TTL,
		maxCacheSize: opts.MaxSize,
		heap:         make(cacheEntryHeap, 0, opts.MaxSize),
		heapIndex:    make(map[string]*cacheEntry, opts.MaxSize),
	}

	// Start cache cleanup if enabled
	if opts.EnableCleanup {
		go cv.startCleanup(opts.CleanupInterval)
	}

	return cv
}

// ValidateToken validates a token using the cache when possible
func (cv *CachedValidator) ValidateToken(ctx context.Context, tokenID string) (string, error) {
	// Check cache first
	projectID, found := cv.checkCache(tokenID)
	if found {
		return projectID, nil
	}

	// Cache miss, validate using the underlying validator
	projectID, err := cv.validator.ValidateToken(ctx, tokenID)
	if err != nil {
		return "", err
	}

	// Cache the successful validation
	cv.cacheToken(ctx, tokenID)

	return projectID, nil
}

// ValidateTokenWithTracking validates a token and tracks usage (bypasses cache for tracking)
func (cv *CachedValidator) ValidateTokenWithTracking(ctx context.Context, tokenID string) (string, error) {
	// Always use the underlying validator for tracking requests
	projectID, err := cv.validator.ValidateTokenWithTracking(ctx, tokenID)
	if err != nil {
		return "", err
	}

	// Update the cache if the token is already cached
	cv.invalidateCache(tokenID)

	return projectID, nil
}

// checkCache checks if a token is in the cache and still valid
func (cv *CachedValidator) checkCache(tokenID string) (string, bool) {
	cv.cacheMutex.RLock()
	entry, found := cv.cache[tokenID]
	cv.cacheMutex.RUnlock()

	// Not in cache
	if !found {
		cv.statsMutex.Lock()
		cv.misses++
		cv.statsMutex.Unlock()
		return "", false
	}

	// In cache but expired
	now := time.Now()
	if now.After(entry.ValidUntil) {
		cv.cacheMutex.Lock()
		delete(cv.cache, tokenID)
		cv.cacheMutex.Unlock()

		cv.statsMutex.Lock()
		cv.misses++
		cv.evictions++
		cv.statsMutex.Unlock()

		return "", false
	}

	// In cache and valid
	cv.statsMutex.Lock()
	cv.hits++
	cv.statsMutex.Unlock()

	return entry.Data.ProjectID, true
}

// cacheToken retrieves and caches a token
func (cv *CachedValidator) cacheToken(ctx context.Context, tokenID string) {
	standardValidator, ok := cv.validator.(*StandardValidator)
	if !ok {
		return
	}

	// TokenValidator receives the token *string* (sk-...) in ValidateToken/ValidateTokenWithTracking.
	// Populate cache using token-string lookup.
	tokenData, err := standardValidator.store.GetTokenByToken(ctx, tokenID)
	if err != nil {
		return
	}
	if !tokenData.IsValid() {
		return
	}

	validUntil := time.Now().Add(cv.cacheTTL)

	cv.cacheMutex.Lock()
	defer cv.cacheMutex.Unlock()

	insertedAt := cv.insertCounter
	cv.insertCounter++

	cv.cache[tokenID] = CacheEntry{
		Data:       tokenData,
		ValidUntil: validUntil,
	}
	// Remove old heap entry if present
	if oldEntry, ok := cv.heapIndex[tokenID]; ok {
		idx := oldEntry.index
		heap.Remove(&cv.heap, idx)
		delete(cv.heapIndex, tokenID)
	}
	entry := &cacheEntry{
		tokenID:    tokenID,
		validUntil: validUntil,
		insertedAt: insertedAt,
	}
	heap.Push(&cv.heap, entry)
	cv.heapIndex[tokenID] = entry

	// Evict if over capacity
	if cv.maxCacheSize > 0 && len(cv.cache) > cv.maxCacheSize {
		cv.evictOldest()
	}
}

// invalidateCache removes a token from the cache
func (cv *CachedValidator) invalidateCache(tokenID string) {
	cv.cacheMutex.Lock()
	delete(cv.cache, tokenID)
	// Remove from heap if present
	if entry, ok := cv.heapIndex[tokenID]; ok {
		idx := entry.index
		heap.Remove(&cv.heap, idx)
		delete(cv.heapIndex, tokenID)
	}
	cv.cacheMutex.Unlock()
	// Note: In production, cache and heap sizes should always be consistent
}

// evictOldest removes the single oldest entry from the cache
func (cv *CachedValidator) evictOldest() {
	if cv.heap.Len() == 0 {
		return
	}
	entry := heap.Pop(&cv.heap).(*cacheEntry)
	// Remove from heapIndex
	delete(cv.heapIndex, entry.tokenID)
	// Remove from cache
	delete(cv.cache, entry.tokenID)
	cv.statsMutex.Lock()
	cv.evictions++
	cv.statsMutex.Unlock()
}

// startCleanup periodically cleans up expired entries from the cache
func (cv *CachedValidator) startCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		cv.cleanup()
	}
}

// cleanup removes expired entries from the cache
func (cv *CachedValidator) cleanup() {
	now := time.Now()

	cv.cacheMutex.Lock()
	defer cv.cacheMutex.Unlock()

	for k, v := range cv.cache {
		if now.After(v.ValidUntil) {
			delete(cv.cache, k)
			cv.statsMutex.Lock()
			cv.evictions++
			cv.statsMutex.Unlock()
		}
	}
}

// ClearCache removes all entries from the cache
func (cv *CachedValidator) ClearCache() {
	cv.cacheMutex.Lock()
	cv.cache = make(map[string]CacheEntry)
	cv.cacheMutex.Unlock()
}

// GetCacheStats returns statistics about the cache
func (cv *CachedValidator) GetCacheStats() (hits, misses, evictions, size int) {
	cv.statsMutex.Lock()
	hits = cv.hits
	misses = cv.misses
	evictions = cv.evictions
	cv.statsMutex.Unlock()

	cv.cacheMutex.RLock()
	size = len(cv.cache)
	cv.cacheMutex.RUnlock()

	return
}

// GetCacheInfo returns a formatted string with cache statistics
func (cv *CachedValidator) GetCacheInfo() string {
	hits, misses, evictions, size := cv.GetCacheStats()
	total := hits + misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	return fmt.Sprintf(
		"Cache Stats:\n"+
			"  Size: %d (max: %d)\n"+
			"  Hits: %d (%.1f%%)\n"+
			"  Misses: %d\n"+
			"  Evictions: %d\n"+
			"  TTL: %s",
		size, cv.maxCacheSize, hits, hitRate, misses, evictions, cv.cacheTTL,
	)
}
