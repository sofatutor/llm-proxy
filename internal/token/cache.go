package token

import (
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

// CachedValidator wraps a TokenValidator with caching
type CachedValidator struct {
	validator    TokenValidator
	cache        map[string]CacheEntry
	cacheMutex   sync.RWMutex
	cacheTTL     time.Duration
	maxCacheSize int

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
	// Type cast to get access to the TokenStore
	standardValidator, ok := cv.validator.(*StandardValidator)
	if !ok {
		// Cannot cache if we don't have access to the store
		return
	}

	// Get token data from the store
	tokenData, err := standardValidator.store.GetTokenByID(ctx, tokenID)
	if err != nil {
		return
	}

	// Don't cache invalid tokens
	if !tokenData.IsValid() {
		return
	}

	cv.cacheMutex.Lock()
	defer cv.cacheMutex.Unlock()

	// Check if we need to evict entries due to size limit
	if cv.maxCacheSize > 0 && len(cv.cache) >= cv.maxCacheSize {
		cv.evictOldest()
	}

	// Add to cache with TTL
	cv.cache[tokenID] = CacheEntry{
		Data:       tokenData,
		ValidUntil: time.Now().Add(cv.cacheTTL),
	}
}

// invalidateCache removes a token from the cache
func (cv *CachedValidator) invalidateCache(tokenID string) {
	cv.cacheMutex.Lock()
	delete(cv.cache, tokenID)
	cv.cacheMutex.Unlock()
}

// evictOldest removes the oldest entries from the cache to make room
func (cv *CachedValidator) evictOldest() {
	// Simple strategy: remove approximately 10% of entries
	toRemove := cv.maxCacheSize / 10
	if toRemove < 1 {
		toRemove = 1
	}

	// Find the oldest entries
	type cacheAge struct {
		key        string
		validUntil time.Time
	}

	oldest := make([]cacheAge, 0, len(cv.cache))
	for k, v := range cv.cache {
		oldest = append(oldest, cacheAge{k, v.ValidUntil})
	}

	// Sort by expiration time (in-place sort would be more efficient)
	// For simplicity, just find the N oldest by iterating
	for i := 0; i < toRemove && i < len(oldest); i++ {
		var oldestKey string
		var oldestTime time.Time

		// Find oldest entry
		first := true
		for k, v := range cv.cache {
			if first || v.ValidUntil.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.ValidUntil
				first = false
			}
		}

		// Remove oldest entry
		if oldestKey != "" {
			delete(cv.cache, oldestKey)
			cv.evictions++
		}
	}
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
			cv.evictions++
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
