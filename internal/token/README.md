# Token Management Package

This package handles all token-related operations:

- UUID generation and validation
- Token expiration and rate limiting
- Token validation and authorization
- Token usage tracking and statistics

## Rate Limiting

### In-Memory Rate Limiting

The `MemoryRateLimiter` implements a token bucket algorithm for per-instance rate limiting:

```go
limiter := token.NewMemoryRateLimiter(rate, capacity)
allowed := limiter.Allow(tokenID)
```

### Distributed Rate Limiting (Redis)

The `RedisRateLimiter` implements distributed rate limiting using Redis for global enforcement across multiple instances:

```go
config := token.RedisRateLimiterConfig{
    KeyPrefix:             "ratelimit:",
    DefaultWindowDuration: time.Minute,
    DefaultMaxRequests:    60,
    EnableFallback:        true,
}
limiter := token.NewRedisRateLimiter(redisClient, config)
allowed, err := limiter.Allow(ctx, tokenID)
```

See [Distributed Rate Limiting Documentation](/docs/distributed-rate-limiting.md) for more details.

## Files

| File | Description |
|------|-------------|
| `token.go` | Token generation and validation |
| `validate.go` | Token format validation |
| `ratelimit.go` | Standard and in-memory rate limiters |
| `redis_ratelimit.go` | Distributed Redis-backed rate limiter |
| `redis_adapter.go` | Redis client adapter for go-redis |
| `revoke.go` | Token revocation |
| `expiration.go` | Token expiration handling |
| `cache.go` | Token validation caching |
| `manager.go` | Unified token management interface |