# HTTP Response Caching Strategy

This document outlines the HTTP response caching system implemented in the LLM proxy. The caching system provides significant performance improvements through Redis-backed shared caching with HTTP standards compliance.

## Overview

The proxy implements a Redis-backed caching system that improves performance, reduces latency, and decreases load on target APIs. The caching system respects HTTP cache control headers and provides optional in-memory fallback for development environments.

## Goals

1. **Reduce Latency**: Serve cached responses when appropriate to minimize response time
2. **Reduce API Costs**: Minimize redundant API calls to save on usage costs
3. **Improve Reliability**: Provide responses even during brief target API outages
4. **Balance Freshness**: Allow fine-grained control over cache TTL per endpoint and request type

## Cache Architecture

### Redis as Cache Store

Redis will be used as the primary cache store for the following reasons:

1. **Performance**: In-memory operation with sub-millisecond response times
2. **Distributed**: Supports clustered proxy deployments seamlessly
3. **TTL Support**: Built-in time-based expiration
4. **Data Structures**: Rich data structures for flexible caching patterns
5. **Persistence**: Optional persistence for cache warming after restarts

### Cache Keys

Cache keys will be constructed using a deterministic algorithm based on:

1. **Request Path**: The API endpoint being called
2. **Request Method**: GET, POST, etc.
3. **Request Parameters**: Query parameters and/or request body (normalized)
4. **Project ID**: To isolate caches between different projects

Example key format:
```
cache:v1:{project_id}:{endpoint}:{method}:{hash_of_parameters}
```

### Cache Values

Cache entries will store:

1. **Response Body**: The serialized response
2. **Response Headers**: Relevant headers from the original response
3. **Cache Metadata**:
   - Timestamp of original request
   - TTL information
   - Hit count

## Caching Policies

### Cacheable Endpoints

Not all endpoints are suitable for caching. By default:

1. **Cacheable**:
   - `/v1/models` (list models)
   - `/v1/embeddings` (vector embeddings)
   - Other read-only, deterministic endpoints

2. **Not Cacheable by Default**:
   - `/v1/chat/completions` (unless explicitly enabled)
   - Any streaming endpoints
   - Endpoints with side effects

### TTL Configuration

TTL (Time-To-Live) is determined through a precedence hierarchy:

1. **Response Headers** (highest precedence):
   - `s-maxage` directive (shared cache specific)
   - `max-age` directive (general cache TTL)

2. **Default TTL** (fallback):
   - `HTTP_CACHE_DEFAULT_TTL` environment variable
   - Used when upstream permits caching but doesn't specify TTL
   - Default: 300 seconds (5 minutes)

3. **Client-Forced Caching**:
   - When client sends `Cache-Control: public, max-age=N` on request
   - Used for POST requests or when upstream lacks cache directives
   - Enables cache testing and benchmarking scenarios

### Cache Invalidation

Current invalidation mechanisms:

1. **Time-Based Expiration**: Automatic expiration via Redis TTL
2. **Manual Purge**: Planned for future management API
3. **Size Limits**: Objects exceeding `HTTP_CACHE_MAX_OBJECT_BYTES` are not cached
4. **Cache Control Directives**: `no-store` and `private` bypass caching entirely

## Implementation Approach

### Cache Middleware

A caching middleware will be added to the proxy middleware chain with responsibilities for:

1. **Cache Lookup**: Check if a valid cache entry exists
2. **Cache Serving**: Return cached response if available
3. **Cache Storage**: Store new responses in cache
4. **Headers Management**: Handle cache-related headers

```go
func CachingMiddleware(cacheClient *redis.Client, config CacheConfig) Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Skip caching for non-cacheable requests
            if !isCacheable(r, config) {
                next.ServeHTTP(w, r)
                return
            }

            // Generate cache key
            cacheKey := generateCacheKey(r)
            
            // Check cache
            if cacheEntry, found := lookupCache(cacheClient, cacheKey); found {
                // Serve from cache
                serveFromCache(w, cacheEntry)
                recordCacheHit(cacheKey)
                return
            }
            
            // Cache miss - proceed with request
            // Use a response recorder to capture the response
            recorder := newResponseRecorder(w)
            next.ServeHTTP(recorder, r)
            
            // Store response in cache if appropriate
            if shouldCache(recorder, config) {
                storeInCache(cacheClient, cacheKey, recorder, config)
            }
            
            recordCacheMiss(cacheKey)
        })
    }
}
```

### Streaming Response Handling

Streaming responses require special consideration:

1. **No Caching by Default**: Streaming responses won't be cached by default
2. **Optional Caching**: Configuration option to cache the complete aggregated response
3. **Partial Caching**: Cache initial parts of responses if appropriate

### Cache Control Headers

The proxy will respect and implement standard cache control headers:

1. **Support for Client Headers**:
   - `Cache-Control: no-cache` - Verify with origin before using cached copy
   - `Cache-Control: no-store` - Skip caching entirely

2. **Adding Response Headers**:
   - `X-Cache: HIT/MISS` - Indicate cache result
   - `Age` - Time since response was generated
   - Standard Cache-Control headers

## Performance Considerations

1. **Cache Size Limits**: Configurable maximum cache size with LRU eviction
2. **Memory Pressure**: Monitoring of Redis memory usage
3. **Cache Warmup**: Option to pre-populate cache for frequently used requests
4. **Compression**: Optional compression for large responses

## Metrics and Monitoring

Comprehensive metrics will be collected:

1. **Hit Rate**: Overall and per-endpoint cache hit percentage
2. **Latency Improvement**: Time saved by serving from cache
3. **Cache Size**: Current cache size and item count
4. **Evictions**: Count of cache entries evicted due to memory pressure
5. **TTL Distribution**: Histogram of remaining TTL for cached entries

## Security Considerations

1. **Isolation**: Strict isolation between projects' cache entries
2. **Sensitive Data**: Option to exclude sensitive data from caching
3. **Redis Authentication**: Required Redis authentication
4. **Transport Security**: Encrypted communication with Redis
5. **Response Validation**: Validation of cached responses before serving

## Implementation Status

The core caching system has been implemented and is available in the proxy:

### ✅ Implemented Features

1. **Redis-backed Caching**
   - Primary Redis backend with in-memory fallback
   - Configurable via `HTTP_CACHE_BACKEND` environment variable
   - Redis connection and key prefix configuration

2. **HTTP Standards Compliance**
   - Respects `Cache-Control` directives (`no-store`, `private`, `public`, `max-age`, `s-maxage`)
   - Honors `Authorization` header behavior for shared cache semantics
   - Supports conditional requests with `ETag` and `Last-Modified` validators
   - TTL derivation with `s-maxage` precedence over `max-age`

3. **Request Method Support**
   - GET/HEAD caching by default when upstream permits
   - Optional POST caching when client opts in via request `Cache-Control`
   - Conservative `Vary` handling with subset of request headers

4. **Streaming Response Support**
   - Captures streaming responses while serving to client
   - Stores complete response after streaming completion
   - Subsequent requests serve from cache immediately

5. **Observability Integration**
   - Response headers: `X-PROXY-CACHE`, `X-PROXY-CACHE-KEY`, `Cache-Status`
   - Event bus bypass for cache hits (performance optimization)
   - Cache misses and stores published to event bus

6. **Configuration and Tooling**
   - Environment variable configuration
   - Benchmark CLI with cache testing flags (`--cache`, `--cache-ttl`, `--method`)
   - Size limits and TTL controls

### 🔄 Future Enhancements

1. **Advanced Cache Control**
   - `stale-while-revalidate` and `stale-if-error` support
   - Full per-response `Vary` header handling
   - Upstream conditional revalidation (If-None-Match/If-Modified-Since)

2. **Management Features**
   - Cache purge endpoints and CLI commands
   - Metrics for hits/misses/bypass/store rates
   - Cache warming strategies

3. **Performance Optimizations**
   - Response compression for large objects
   - Bounded L1 in-memory cache layer
   - Cache key optimization

### Configuration

Enable caching with environment variables:

```bash
HTTP_CACHE_ENABLED=true
HTTP_CACHE_BACKEND=redis
REDIS_CACHE_URL=redis://localhost:6379/0
REDIS_CACHE_KEY_PREFIX=llmproxy:cache:
HTTP_CACHE_MAX_OBJECT_BYTES=1048576
HTTP_CACHE_DEFAULT_TTL=300
```

See [API Configuration Guide](api-configuration.md) for complete configuration details.