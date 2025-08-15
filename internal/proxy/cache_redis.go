package proxy

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisCache implements httpCache using Redis.
// It stores cachedResponse as JSON and uses Redis TTL for expiration.
type redisCache struct {
	client *redis.Client
	prefix string
}

func newRedisCache(client *redis.Client, keyPrefix string) *redisCache {
	if keyPrefix == "" {
		keyPrefix = "llmproxy:cache:"
	}
	return &redisCache{client: client, prefix: keyPrefix}
}

type redisCachedResponse struct {
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers"`
	Body       []byte              `json:"body"`
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
		// expiresAt not needed; Redis TTL enforces expiry
		expiresAt: time.Now().Add(time.Second),
	}, true
}

func (r *redisCache) Set(key string, value cachedResponse) {
	ctx := context.Background()
	// Serialize
	ser := redisCachedResponse{StatusCode: value.statusCode, Headers: value.headers, Body: value.body}
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
