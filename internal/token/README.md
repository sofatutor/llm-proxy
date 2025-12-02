# Token Package

## Purpose & Responsibilities

The `token` package provides comprehensive token lifecycle management for the LLM Proxy. It handles:

- Secure token generation using UUIDv7 (time-ordered)
- Token validation with caching for performance
- Token expiration and automatic revocation
- Per-token rate limiting (in-memory and distributed)
- Usage tracking and statistics
- Token revocation (individual and bulk)

## Key Types & Interfaces

| Type | Description |
|------|-------------|
| `Manager` | Unified interface for all token operations |
| `TokenData` | Core token data structure with all properties |
| `TokenValidator` | Interface for token validation |
| `TokenStore` | Interface for token persistence |
| `CachedValidator` | Validator with in-memory caching |
| `Revoker` | Handles token revocation operations |
| `TokenGenerator` | Generates secure tokens with options |

### Core Interfaces

```go
// TokenValidator validates tokens and returns project IDs
type TokenValidator interface {
    ValidateToken(ctx context.Context, token string) (string, error)
    ValidateTokenWithTracking(ctx context.Context, token string) (string, error)
}

// TokenStore persists and retrieves tokens
type TokenStore interface {
    GetTokenByID(ctx context.Context, tokenID string) (TokenData, error)
    IncrementTokenUsage(ctx context.Context, tokenID string) error
    CreateToken(ctx context.Context, token TokenData) error
    UpdateToken(ctx context.Context, token TokenData) error
    ListTokens(ctx context.Context) ([]TokenData, error)
    GetTokensByProjectID(ctx context.Context, projectID string) ([]TokenData, error)
}

// ManagerStore combines all store interfaces
type ManagerStore interface {
    TokenStore
    RevocationStore
    RateLimitStore
}
```

### TokenData Structure

```go
type TokenData struct {
    Token         string     // The token ID (sk-...)
    ProjectID     string     // Associated project ID
    ExpiresAt     *time.Time // Expiration time (nil = no expiration)
    IsActive      bool       // Active status
    DeactivatedAt *time.Time // When deactivated (nil = not deactivated)
    RequestCount  int        // Number of requests made
    MaxRequests   *int       // Maximum requests allowed (nil = unlimited)
    CreatedAt     time.Time  // Creation timestamp
    LastUsedAt    *time.Time // Last usage timestamp
    CacheHitCount int        // Number of cache hits
}
```

## Usage Examples

### Using the Token Manager

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/sofatutor/llm-proxy/internal/database"
    "github.com/sofatutor/llm-proxy/internal/token"
)

func main() {
    // Setup database
    db, _ := database.New(database.DefaultConfig())
    defer db.Close()

    // Create token store from database
    store := database.NewTokenStore(db)

    // Create token manager with caching
    manager, err := token.NewManager(store, true) // true = enable caching
    if err != nil {
        log.Fatal(err)
    }

    // Create a new token
    options := token.TokenOptions{
        Expiration:  24 * time.Hour,
        MaxRequests: intPtr(1000),
    }
    
    tokenData, err := manager.CreateToken(ctx, "project-123", options)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Created token: %s\n", tokenData.Token)

    // Validate token
    projectID, err := manager.ValidateToken(ctx, tokenData.Token)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Token belongs to project: %s\n", projectID)
}

func intPtr(i int) *int { return &i }
```

### Token Generation (UUIDv7)

Tokens use UUIDv7 which includes a timestamp component, making them time-ordered:

```go
// Generate a new token
tokenStr, err := token.GenerateToken()
if err != nil {
    log.Fatal(err)
}
// Result: sk-AYB2gH5x...22chars (base64url encoded UUIDv7)

// Generate with options
gen := token.NewTokenGenerator()
tokenStr, expiresAt, maxReqs, err := gen.GenerateWithOptions(
    24*time.Hour,  // expiration
    intPtr(1000),  // max requests
)
```

### Token Validation

```go
// Basic validation (without tracking)
projectID, err := validator.ValidateToken(ctx, tokenStr)
if err != nil {
    switch {
    case errors.Is(err, token.ErrTokenNotFound):
        // Token doesn't exist
    case errors.Is(err, token.ErrTokenInactive):
        // Token is deactivated
    case errors.Is(err, token.ErrTokenExpired):
        // Token has expired
    case errors.Is(err, token.ErrTokenRateLimit):
        // Token reached max requests
    }
}

// Validation with usage tracking
projectID, err := validator.ValidateTokenWithTracking(ctx, tokenStr)
// Increments request count on success
```

### Validation Caching

The `CachedValidator` wraps a validator with an in-memory LRU cache:

```go
// Create cached validator with default options
baseValidator := token.NewValidator(store)
cachedValidator := token.NewCachedValidator(baseValidator)

// With custom options
options := token.CacheOptions{
    TTL:             5 * time.Minute,  // Cache entry TTL
    MaxSize:         1000,             // Maximum cached entries
    EnableCleanup:   true,             // Automatic cleanup
    CleanupInterval: 1 * time.Minute,  // Cleanup frequency
}
cachedValidator := token.NewCachedValidator(baseValidator, options)

// Get cache stats
info, ok := manager.GetCacheInfo()
if ok {
    fmt.Println(info) // "hits=123, misses=45, evictions=10"
}
```

### Token Revocation

```go
// Revoke single token
err := manager.RevokeToken(ctx, tokenID)

// Delete token completely
err := manager.DeleteToken(ctx, tokenID)

// Revoke all expired tokens
count, err := manager.RevokeExpiredTokens(ctx)
fmt.Printf("Revoked %d expired tokens\n", count)

// Revoke all tokens for a project
count, err := manager.RevokeProjectTokens(ctx, projectID)
fmt.Printf("Revoked %d project tokens\n", count)
```

### Automatic Expiration

```go
import "go.uber.org/zap"

logger, _ := zap.NewProduction()

// Start automatic revocation of expired tokens
autoRevoke := manager.StartAutomaticRevocation(
    5 * time.Minute,  // Check interval
    logger,
)
defer autoRevoke.Stop()
```

## Rate Limiting

### In-Memory Rate Limiting

For single-instance deployments:

```go
limiter := token.NewMemoryRateLimiter(rate, capacity)
allowed := limiter.Allow(tokenID)
```

### Distributed Rate Limiting (Redis)

For multi-instance deployments:

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

### Per-Token Rate Limits

```go
// Update token rate limit
maxReqs := 1000
err := manager.UpdateTokenLimit(ctx, tokenID, &maxReqs)

// Reset usage count
err := manager.ResetTokenUsage(ctx, tokenID)
```

See [Distributed Rate Limiting Documentation](/docs/distributed-rate-limiting.md) for more details.

## Configuration

### Cache Configuration

| Option | Description | Default |
|--------|-------------|---------|
| `TTL` | Time-to-live for cache entries | 5 minutes |
| `MaxSize` | Maximum number of cached entries | 1000 |
| `EnableCleanup` | Enable automatic cache cleanup | true |
| `CleanupInterval` | Interval between cleanup runs | 1 minute |

### Rate Limiter Configuration

| Option | Description | Default |
|--------|-------------|---------|
| `KeyPrefix` | Redis key prefix | `ratelimit:` |
| `DefaultWindowDuration` | Rate limit window | 1 minute |
| `DefaultMaxRequests` | Requests per window | 60 |
| `EnableFallback` | Fall back on Redis errors | true |

## Testing Guidance

### Unit Testing

```go
package token_test

import (
    "context"
    "testing"
    "time"

    "github.com/sofatutor/llm-proxy/internal/token"
)

func TestTokenValidation(t *testing.T) {
    // Create mock store
    store := &MockTokenStore{
        tokens: map[string]token.TokenData{
            "sk-validtoken12345678": {
                Token:     "sk-validtoken12345678",
                ProjectID: "project-1",
                IsActive:  true,
            },
        },
    }

    validator := token.NewValidator(store)

    // Test valid token
    projectID, err := validator.ValidateToken(context.Background(), "sk-validtoken12345678")
    if err != nil {
        t.Errorf("Expected no error, got %v", err)
    }
    if projectID != "project-1" {
        t.Errorf("Expected project-1, got %s", projectID)
    }

    // Test invalid token
    _, err = validator.ValidateToken(context.Background(), "sk-invalidtoken12345")
    if err != token.ErrTokenNotFound {
        t.Errorf("Expected ErrTokenNotFound, got %v", err)
    }
}
```

### Testing Token Format

```go
func TestTokenFormat(t *testing.T) {
    tests := []struct {
        token   string
        wantErr bool
    }{
        {"sk-AYB2gH5xQZ1234567890ab", false}, // Valid format
        {"invalid", true},                     // Missing prefix
        {"sk-short", true},                    // Too short
        {"sk-has/special/chars!!!", true},    // Invalid characters
    }

    for _, tt := range tests {
        err := token.ValidateTokenFormat(tt.token)
        if (err != nil) != tt.wantErr {
            t.Errorf("ValidateTokenFormat(%q) error = %v, wantErr %v", tt.token, err, tt.wantErr)
        }
    }
}
```

### Testing with Database

```go
func TestTokenManagerIntegration(t *testing.T) {
    // Create in-memory database
    db, _ := database.New(database.Config{Path: ":memory:"})
    defer db.Close()

    store := database.NewTokenStore(db)
    manager, _ := token.NewManager(store, true)

    ctx := context.Background()

    // Create token
    data, err := manager.CreateToken(ctx, "project-1", token.TokenOptions{
        Expiration: time.Hour,
    })
    if err != nil {
        t.Fatal(err)
    }

    // Validate
    projectID, err := manager.ValidateToken(ctx, data.Token)
    if err != nil {
        t.Fatal(err)
    }
    if projectID != "project-1" {
        t.Errorf("Expected project-1, got %s", projectID)
    }
}
```

## Troubleshooting

### Common Errors

| Error | Cause | Solution |
|-------|-------|----------|
| `ErrTokenNotFound` | Token doesn't exist in store | Verify token was created |
| `ErrTokenInactive` | Token has been deactivated | Check if token was revoked |
| `ErrTokenExpired` | Token past expiration time | Create new token |
| `ErrTokenRateLimit` | MaxRequests reached | Increase limit or reset usage |
| `ErrInvalidTokenFormat` | Token string malformed | Check prefix and length |

### Cache Issues

| Symptom | Cause | Solution |
|---------|-------|----------|
| Stale data after token update | Cache TTL | Reduce TTL or clear cache |
| High memory usage | Large cache | Reduce MaxSize |
| Slow validation | Cache misses | Increase MaxSize or TTL |

### Rate Limiting Issues

| Symptom | Cause | Solution |
|---------|-------|----------|
| Inconsistent limits (multi-instance) | Using in-memory limiter | Switch to Redis limiter |
| Redis errors | Connection issues | Enable fallback mode |
| Rate limit not enforced | MaxRequests is nil | Set MaxRequests on token |

## Related Packages

| Package | Relationship |
|---------|--------------|
| [`database`](../database/README.md) | Token persistence layer |
| [`proxy`](../proxy/README.md) | Uses TokenValidator interface |
| [`server`](../server/README.md) | Token management API |

## Files

| File | Description |
|------|-------------|
| `token.go` | Token generation and format validation |
| `validate.go` | StandardValidator implementation |
| `cache.go` | CachedValidator with LRU cache |
| `manager.go` | Unified Manager interface |
| `ratelimit.go` | In-memory rate limiter |
| `redis_ratelimit.go` | Distributed Redis rate limiter |
| `redis_adapter.go` | Redis client adapter |
| `revoke.go` | Token revocation operations |
| `expiration.go` | Expiration checking and auto-revoke |
| `utils.go` | Helper functions (obfuscation, info) |