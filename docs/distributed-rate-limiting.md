# Distributed Rate Limiting

This document describes the distributed rate limiting feature, which enables rate limits to be enforced globally across all proxy instances using Redis.

## Overview

In multi-instance deployments, each proxy instance would normally maintain its own rate limit counters. This means that total requests can exceed the intended limit by N times (where N is the number of instances). Distributed rate limiting solves this by using Redis as a shared counter store.

## Features

- **Global Rate Limits**: Rate limits are enforced across all proxy instances
- **Sliding Window Algorithm**: Accurate rate limiting using time-based windows
- **Atomic Operations**: Uses Redis INCR for thread-safe counter updates
- **Graceful Fallback**: Falls back to in-memory rate limiting when Redis is unavailable
- **Per-Token Configuration**: Supports custom rate limits for specific tokens
- **Configurable**: All settings can be customized via environment variables

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Proxy #1      │     │   Proxy #2      │     │   Proxy #3      │
│ ┌─────────────┐ │     │ ┌─────────────┐ │     │ ┌─────────────┐ │
│ │ Rate Limiter│ │     │ │ Rate Limiter│ │     │ │ Rate Limiter│ │
│ └──────┬──────┘ │     │ └──────┬──────┘ │     │ └──────┬──────┘ │
└────────┼────────┘     └────────┼────────┘     └────────┼────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                          ┌──────▼──────┐
                          │    Redis    │
                          │  (Counters) │
                          └─────────────┘
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DISTRIBUTED_RATE_LIMIT_ENABLED` | `false` | Enable Redis-backed distributed rate limiting |
| `DISTRIBUTED_RATE_LIMIT_PREFIX` | `ratelimit:` | Redis key prefix for rate limit counters |
| `DISTRIBUTED_RATE_LIMIT_WINDOW` | `1m` | Sliding window duration (e.g., `30s`, `1m`, `5m`) |
| `DISTRIBUTED_RATE_LIMIT_MAX` | `60` | Maximum requests per window |
| `DISTRIBUTED_RATE_LIMIT_FALLBACK` | `true` | Enable fallback to in-memory when Redis unavailable |
| `REDIS_ADDR` | `localhost:6379` | Redis server address |
| `REDIS_DB` | `0` | Redis database number |

### Example Configuration

```bash
# Enable distributed rate limiting
export DISTRIBUTED_RATE_LIMIT_ENABLED=true
export REDIS_ADDR=redis:6379

# Custom rate limit: 100 requests per 30 seconds
export DISTRIBUTED_RATE_LIMIT_WINDOW=30s
export DISTRIBUTED_RATE_LIMIT_MAX=100

# Enable fallback for high availability
export DISTRIBUTED_RATE_LIMIT_FALLBACK=true
```

## How It Works

### Sliding Window Algorithm

The rate limiter uses a sliding window counter algorithm:

1. When a request arrives, calculate the current window start time
2. Build a Redis key: `{prefix}{tokenID}:{windowStart}`
3. Atomically increment the counter using `INCR`
4. Set TTL on the key (on first increment)
5. Check if count exceeds the limit

### Key Format

Redis keys follow this format:
```
ratelimit:{tokenID}:{windowStartUnix}
```

Example:
```
ratelimit:sk-abc123:1701388800
```

### TTL Management

Keys automatically expire after the window duration plus one second to handle edge cases at window boundaries.

## Fallback Behavior

When Redis is unavailable and fallback is enabled:

1. Rate limiter detects Redis failure
2. Switches to in-memory token bucket algorithm
3. Uses configured fallback rate and capacity
4. Continues to check Redis availability periodically

When fallback is disabled:
- Returns `ErrRedisUnavailable` error
- Requests may be blocked or allowed based on application handling

## API Reference

### RedisRateLimiter

```go
// Create a new distributed rate limiter
config := token.RedisRateLimiterConfig{
    KeyPrefix:             "ratelimit:",
    DefaultWindowDuration: time.Minute,
    DefaultMaxRequests:    60,
    EnableFallback:        true,
    FallbackRate:          1.0,
    FallbackCapacity:      10,
}
limiter := token.NewRedisRateLimiter(redisClient, config)

// Check if request is allowed
allowed, err := limiter.Allow(ctx, tokenID)

// Get remaining requests
remaining, err := limiter.GetRemainingRequests(ctx, tokenID)

// Set custom limit for a token
limiter.SetTokenLimit(tokenID, 100, time.Minute)

// Reset token usage
err := limiter.ResetTokenUsage(ctx, tokenID)

// Check Redis health
err := limiter.CheckRedisHealth(ctx)
```

## Monitoring

### Key Metrics

Monitor these Redis keys to understand rate limiting behavior:

- Count of rate limit keys: `SCAN 0 MATCH ratelimit:* COUNT 100`  
  _(Use `SCAN` for production monitoring; `KEYS` is blocking and only safe for development/debugging.)_
- Current count for a token: `GET ratelimit:{tokenID}:{windowStart}`

### Health Checks

Use `CheckRedisHealth()` to verify Redis connectivity:

```go
if err := limiter.CheckRedisHealth(ctx); err != nil {
    log.Warn("Redis unavailable for rate limiting", zap.Error(err))
}
```

## Best Practices

1. **Enable Fallback**: Always enable fallback in production for high availability
2. **Monitor Redis**: Set up alerts for Redis availability and latency
3. **Tune Window Size**: Smaller windows provide more accurate limiting but increase Redis operations
4. **Key Prefix**: Use unique prefixes if sharing Redis with other applications
5. **TTL Buffer**: The implementation adds 1 second to TTL to handle edge cases

## Troubleshooting

### Common Issues

**Rate limits not enforced globally**
- Verify `DISTRIBUTED_RATE_LIMIT_ENABLED=true`
- Check Redis connectivity from all instances
- Ensure all instances use the same Redis server

**Fallback always active**
- Check Redis connection string
- Verify Redis is running and accessible
- Check network connectivity

**Keys not expiring**
- This is handled automatically; check Redis EXPIRE configuration
- Verify TTL is being set (check `CheckRedisHealth()`)

## Migration Guide

### From Per-Instance Rate Limiting

1. Set up Redis (if not already available)
2. Configure environment variables
3. Enable distributed rate limiting: `DISTRIBUTED_RATE_LIMIT_ENABLED=true`
4. Monitor for any issues
5. Adjust window and max settings as needed

### Rollback

To disable distributed rate limiting:
```bash
export DISTRIBUTED_RATE_LIMIT_ENABLED=false
```

The system will automatically use per-instance rate limiting.
