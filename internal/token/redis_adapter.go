package token

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisGoRateLimitAdapter adapts go-redis/v9 Client to the RedisRateLimitClient interface.
type RedisGoRateLimitAdapter struct {
	Client *redis.Client
}

// NewRedisGoRateLimitAdapter creates a new adapter for rate limiting operations
func NewRedisGoRateLimitAdapter(client *redis.Client) *RedisGoRateLimitAdapter {
	return &RedisGoRateLimitAdapter{Client: client}
}

// Incr atomically increments a key and returns the new value
func (a *RedisGoRateLimitAdapter) Incr(ctx context.Context, key string) (int64, error) {
	return a.Client.Incr(ctx, key).Result()
}

// Get retrieves the value of a key
func (a *RedisGoRateLimitAdapter) Get(ctx context.Context, key string) (string, error) {
	result, err := a.Client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return result, err
}

// Set sets the value of a key
func (a *RedisGoRateLimitAdapter) Set(ctx context.Context, key, value string) error {
	return a.Client.Set(ctx, key, value, 0).Err()
}

// Expire sets a TTL on a key
func (a *RedisGoRateLimitAdapter) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return a.Client.Expire(ctx, key, expiration).Err()
}

// SetNX sets a key only if it doesn't exist (for distributed locking)
func (a *RedisGoRateLimitAdapter) SetNX(ctx context.Context, key string, value string, expiration time.Duration) (bool, error) {
	return a.Client.SetNX(ctx, key, value, expiration).Result()
}

// Del deletes a key
func (a *RedisGoRateLimitAdapter) Del(ctx context.Context, key string) error {
	return a.Client.Del(ctx, key).Err()
}
