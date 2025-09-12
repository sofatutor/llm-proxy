package proxy

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisCache implements httpCache using Redis.
// It stores cachedResponse as JSON and uses Redis TTL for expiration.
type redisCache struct {
	client    *redis.Client
	prefix    string
	scanCount int
}

// redisScanCount controls the SCAN batch size used when purging by prefix.
// Larger values reduce round-trips but increase per-iteration workload.
// It can be configured via REDIS_SCAN_COUNT env var; defaults to 2048 when unset/invalid.
var redisScanCount = func() int {
	const defaultScan = 2048
	if v := os.Getenv("REDIS_SCAN_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultScan
}()

func newRedisCache(client *redis.Client, keyPrefix string) *redisCache {
	if keyPrefix == "" {
		keyPrefix = "llmproxy:cache:"
	}
	return &redisCache{client: client, prefix: keyPrefix, scanCount: redisScanCount}
}

type redisCachedResponse struct {
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers"`
	Body       []byte              `json:"body"`
	Vary       string              `json:"vary"` // Vary header for per-response cache key generation
}

func (r *redisCache) Get(key string) (cachedResponse, bool) {
	ctx := context.Background()
	data, err := r.client.Get(ctx, r.prefix+key).Bytes()
	if err != nil {
		return cachedResponse{}, false
	}
	var rc redisCachedResponse
	if err := json.Unmarshal(data, &rc); err != nil {
		return cachedResponse{}, false
	}
	// Convert map to http.Header lazily in caller; keep simple here
	hdr := make(map[string][]string, len(rc.Headers))
	for k, v := range rc.Headers {
		hdr[k] = v
	}
	return cachedResponse{
		statusCode: rc.StatusCode,
		headers:    hdr,
		body:       rc.Body,
		vary:       rc.Vary, // Include vary field
		// expiresAt not needed; Redis TTL enforces expiry
		expiresAt: time.Now().Add(time.Second),
	}, true
}

func (r *redisCache) Set(key string, value cachedResponse) {
	ctx := context.Background()
	// Serialize
	ser := redisCachedResponse{StatusCode: value.statusCode, Headers: value.headers, Body: value.body, Vary: value.vary}
	payload, err := json.Marshal(ser)
	if err != nil {
		return
	}
	ttl := time.Until(value.expiresAt)
	if ttl <= 0 {
		return
	}
	_ = r.client.Set(ctx, r.prefix+key, payload, ttl).Err()
}

// Purge removes a single cache entry by exact key. Returns true if deleted.
func (r *redisCache) Purge(key string) bool {
	ctx := context.Background()
	res := r.client.Del(ctx, r.prefix+key)
	n, _ := res.Result()
	return n > 0
}

// PurgePrefix removes all cache entries whose keys start with the given prefix.
// Returns number of deleted keys. Uses SCAN to avoid blocking Redis.
func (r *redisCache) PurgePrefix(prefix string) int {
	ctx := context.Background()
	fullPrefix := r.prefix + prefix
	var cursor uint64
	total := 0
	for {
		keys, next, err := r.client.Scan(ctx, cursor, fullPrefix+"*", int64(r.scanCount)).Result()
		if err != nil {
			// Abort on scan error to avoid infinite loop; return what we deleted so far
			return total
		}
		cursor = next
		if len(keys) > 0 {
			delCount, _ := r.client.Del(ctx, keys...).Result()
			total += int(delCount)
		}
		if cursor == 0 {
			break
		}
	}
	return total
}
