# Response Caching Strategy

This document outlines the caching strategy for the LLM proxy, which will be implemented in the production phase.

## Overview

The proxy will implement a Redis-backed caching system to improve performance, reduce latency, and decrease load on the target API. This caching system will be configurable per endpoint and will respect cache control headers.

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

TTL settings will be configurable at multiple levels:

1. **Global Default**: Base TTL for all cacheable endpoints
2. **Per-Endpoint**: Specific TTL for each endpoint
3. **Per-Request**: Support for cache control headers to override defaults

### Invalidation Strategies

Several cache invalidation strategies will be implemented:

1. **Time-Based Expiration**: Automatic expiration after TTL
2. **Manual Invalidation**: Admin API to clear cache entries
3. **Conditional Invalidation**: Based on response headers (ETag, Last-Modified)
4. **Targeted Invalidation**: Clear cache for specific projects or endpoints

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

## Development Plan

The caching system will be implemented in multiple phases:

1. **Phase 1: Core Caching**
   - Basic Redis integration
   - Simple key generation
   - Time-based expiration

2. **Phase 2: Advanced Features**
   - Per-endpoint configuration
   - Cache control header support
   - Monitoring and metrics

3. **Phase 3: Optimization**
   - Compression
   - Partial result caching
   - Performance tuning