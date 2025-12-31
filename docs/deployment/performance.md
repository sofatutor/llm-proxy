---
title: Performance Tuning
parent: Deployment
nav_order: 2
---

# Performance Tuning Guide

This guide covers configuration options and best practices for optimizing LLM Proxy performance.

## Overview

LLM Proxy performance depends on several factors:
- **Caching**: Reduces upstream API calls and latency
- **Connection pools**: Manages database and upstream connections efficiently
- **Event bus**: Handles async processing without blocking requests
- **Rate limiting**: Prevents abuse while maintaining throughput

## Caching Configuration

HTTP response caching is the most impactful performance optimization, especially for repeated requests.

### Enable Caching

```bash
# Enable caching (enabled by default)
HTTP_CACHE_ENABLED=true

# Choose backend
HTTP_CACHE_BACKEND=redis  # Production
# or
HTTP_CACHE_BACKEND=in-memory  # Development
```

### Backend Selection

| Backend | Use Case | Pros | Cons |
|---------|----------|------|------|
| **in-memory** | Development, single instance | Fast, no dependencies | Not shared, lost on restart |
| **Redis** | Production, multiple instances | Shared, persistent | Requires Redis |

### TTL Tuning

Configure Time-To-Live based on your data freshness requirements:

```bash
# Default TTL when upstream doesn't specify (seconds)
HTTP_CACHE_DEFAULT_TTL=300  # 5 minutes default

# For mostly static content (e.g., model lists)
HTTP_CACHE_DEFAULT_TTL=3600  # 1 hour

# For dynamic content requiring freshness
HTTP_CACHE_DEFAULT_TTL=60  # 1 minute
```

### Cache Size Limits

```bash
# Maximum size for individual cached objects
HTTP_CACHE_MAX_OBJECT_BYTES=1048576  # 1MB default

# Increase for large responses
HTTP_CACHE_MAX_OBJECT_BYTES=5242880  # 5MB
```

### Redis Cache Configuration

```bash
# Redis connection (shared with event bus)
REDIS_ADDR=localhost:6379
REDIS_DB=0

# Key prefix (useful for multi-tenant Redis)
REDIS_CACHE_KEY_PREFIX=llmproxy:cache:
```

### Cache Effectiveness Monitoring

Check cache hit rates via response headers:
- `X-PROXY-CACHE: hit` - Served from cache
- `X-PROXY-CACHE: miss` - Fetched from upstream
- `Cache-Status: stored` - Response was cached

```bash
# Test cache behavior
curl -v -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/v1/models 2>&1 | grep -i cache

# Expected first request: X-PROXY-CACHE: miss
# Expected second request: X-PROXY-CACHE: hit
```

### Cache Stats Aggregation

The proxy tracks per-token cache hits:

```bash
# Buffer size for cache hit events
CACHE_STATS_BUFFER_SIZE=1000  # Default

# Increase for high-throughput environments
CACHE_STATS_BUFFER_SIZE=5000
```

Stats are flushed to the database every 5 seconds or when 100 events accumulate.

### Usage Stats Aggregation

For unlimited tokens, the proxy can batch `request_count`/`last_used_at` updates asynchronously.

```bash
# Buffer size for unlimited-token usage tracking events
USAGE_STATS_BUFFER_SIZE=1000  # Default

# Backwards-compatible: if not set, falls back to CACHE_STATS_BUFFER_SIZE
```

## Database Connection Pool

Properly sized connection pools prevent bottlenecks and connection exhaustion.

### Pool Configuration

```bash
# Maximum open connections
DATABASE_POOL_SIZE=10  # Default

# Maximum idle connections
DATABASE_MAX_IDLE_CONNS=5  # Default

# Connection lifetime
DATABASE_CONN_MAX_LIFETIME=1h  # Default
```

### Sizing Guidelines

| Deployment Size | Pool Size | Idle Conns | Lifetime |
|----------------|-----------|------------|----------|
| Development | 5 | 2 | 1h |
| Small (< 100 RPS) | 10 | 5 | 1h |
| Medium (100-500 RPS) | 25 | 10 | 30m |
| Large (> 500 RPS) | 50 | 20 | 15m |

### PostgreSQL-Specific Tuning

For PostgreSQL deployments:

```bash
# Larger pool for PostgreSQL (better concurrency than SQLite)
DATABASE_POOL_SIZE=25
DATABASE_MAX_IDLE_CONNS=10
DATABASE_CONN_MAX_LIFETIME=30m
```

Also tune PostgreSQL server settings:
```sql
-- Increase max connections
ALTER SYSTEM SET max_connections = '200';

-- Tune shared buffers (25% of RAM)
ALTER SYSTEM SET shared_buffers = '1GB';

-- Tune work memory
ALTER SYSTEM SET work_mem = '16MB';
```

See [PostgreSQL Troubleshooting](postgresql-troubleshooting.md#performance-issues) for more.

## Event Bus Configuration

The event bus handles async instrumentation without blocking requests.

### Buffer Sizing

```bash
# Event buffer size
OBSERVABILITY_BUFFER_SIZE=1000  # Default

# For high-throughput (> 1000 RPS)
OBSERVABILITY_BUFFER_SIZE=5000

# For very high throughput
OBSERVABILITY_BUFFER_SIZE=10000
```

### Redis Streams Event Bus (Recommended for Production)

```bash
# Use Redis Streams for distributed event handling with guaranteed delivery
LLM_PROXY_EVENT_BUS=redis-streams
REDIS_ADDR=redis:6379
REDIS_STREAM_KEY=llm-proxy-events
REDIS_CONSUMER_GROUP=llm-proxy-dispatchers
```

Benefits:
- At-least-once delivery guarantees
- Events survive proxy restarts
- Multiple dispatcher instances share workload via consumer groups
- Automatic crash recovery and pending message claiming
- Cross-process event sharing

### Dispatcher Tuning

```bash
# Batch size for sending events
llm-proxy dispatcher --batch-size 100  # Default

# Increase for high throughput
llm-proxy dispatcher --batch-size 500

# Buffer size for dispatcher
llm-proxy dispatcher --buffer 2000
```

## Rate Limiting Configuration

Balance protection against abuse with legitimate traffic needs.

### Global Rate Limits

```bash
# Requests per minute globally
GLOBAL_RATE_LIMIT=100  # Default

# Per-IP requests per minute
IP_RATE_LIMIT=30  # Default
```

### Distributed Rate Limiting (Multi-Instance)

For horizontal scaling:

```bash
DISTRIBUTED_RATE_LIMIT_ENABLED=true
DISTRIBUTED_RATE_LIMIT_PREFIX=ratelimit:
DISTRIBUTED_RATE_LIMIT_WINDOW=1m
DISTRIBUTED_RATE_LIMIT_MAX=60

# Fallback when Redis unavailable
DISTRIBUTED_RATE_LIMIT_FALLBACK=true

# HMAC secret for token ID hashing
DISTRIBUTED_RATE_LIMIT_KEY_SECRET=your-secret-key
```

### Per-Token Rate Limits

Set limits when generating tokens:

```bash
llm-proxy manage token generate \
  --project-id <id> \
  --max-requests 1000 \
  --duration 24
```

## Horizontal Scaling

For high availability and throughput, run multiple proxy instances.

### Prerequisites

1. **PostgreSQL** - Required for shared database state
2. **Redis** - Required for:
   - Distributed caching
   - Distributed rate limiting
   - Shared event bus

### Architecture

```
                    ┌─────────────────┐
                    │  Load Balancer  │
                    └────────┬────────┘
                             │
           ┌─────────────────┼─────────────────┐
           │                 │                 │
    ┌──────┴──────┐   ┌──────┴──────┐   ┌──────┴──────┐
    │  Proxy #1   │   │  Proxy #2   │   │  Proxy #3   │
    └──────┬──────┘   └──────┬──────┘   └──────┬──────┘
           │                 │                 │
           └────────────┬────┴─────────────────┘
                        │
         ┌──────────────┼──────────────┐
         │              │              │
    ┌────┴────┐   ┌─────┴─────┐   ┌────┴────┐
    │ Postgres│   │   Redis   │   │ OpenAI  │
    └─────────┘   └───────────┘   └─────────┘
```

### Configuration for Scaling

```bash
# Database - all instances share
DB_DRIVER=postgres
DATABASE_URL=postgres://user:pass@postgres:5432/llmproxy?sslmode=require
DATABASE_POOL_SIZE=20  # Per instance

# Cache and Event bus - all instances share same Redis
HTTP_CACHE_ENABLED=true
HTTP_CACHE_BACKEND=redis
LLM_PROXY_EVENT_BUS=redis-streams
REDIS_ADDR=redis:6379
REDIS_DB=0
REDIS_STREAM_KEY=llm-proxy-events
REDIS_CONSUMER_GROUP=llm-proxy-dispatchers

# Rate limiting - distributed
DISTRIBUTED_RATE_LIMIT_ENABLED=true
```

### Load Balancer Configuration

Use sticky sessions for WebSocket/streaming support:

**nginx example**:
```nginx
upstream llm-proxy {
    ip_hash;  # Sticky sessions
    server proxy1:8080;
    server proxy2:8080;
    server proxy3:8080;
}

server {
    listen 443 ssl;
    
    location / {
        proxy_pass http://llm-proxy;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_read_timeout 300s;
    }
}
```

## Monitoring and Metrics

### Enable Metrics

```bash
ENABLE_METRICS=true
METRICS_PATH=/metrics
```

### Key Metrics to Monitor

| Metric | Description | Alert Threshold |
|--------|-------------|-----------------|
| `cache_hits_total` | Cache hit count | N/A (higher is better) |
| `cache_misses_total` | Cache miss count | N/A |
| `request_duration_seconds` | Request latency | P95 > 5s |
| `active_connections` | Open DB connections | > 80% of pool |
| `error_rate` | 4xx/5xx responses | > 1% |

### Cache Hit Ratio

Calculate cache effectiveness:
```
hit_ratio = cache_hits_total / (cache_hits_total + cache_misses_total)
```

Target: > 30% for typical workloads

### Debug Performance Issues

```bash
# Enable debug logging
LOG_LEVEL=debug

# Check response times
time curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/v1/models

# Check cache headers
curl -v -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/v1/models 2>&1 | grep -E "(X-PROXY-CACHE|Cache-Status)"
```

## Resource Requirements

### Minimum Requirements

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| CPU | 1 core | 2+ cores |
| Memory | 256MB | 512MB+ |
| Disk | 100MB | 1GB+ (for logs/cache) |

### Sizing Guidelines

| Traffic Level | CPU | Memory | Instances |
|--------------|-----|--------|-----------|
| Low (< 10 RPS) | 1 core | 256MB | 1 |
| Medium (10-100 RPS) | 2 cores | 512MB | 1-2 |
| High (100-500 RPS) | 4 cores | 1GB | 2-4 |
| Very High (> 500 RPS) | 4+ cores | 2GB+ | 4+ |

### Container Resources

Docker resource limits:
```yaml
services:
  llm-proxy:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 1G
        reservations:
          cpus: '0.5'
          memory: 256M
```

## Optimization Checklist

### Quick Wins

- [ ] Enable caching with appropriate TTL
- [ ] Use Redis for production caching
- [ ] Size connection pool appropriately
- [ ] Set request limits on tokens

### Production Optimization

- [ ] Deploy multiple instances behind load balancer
- [ ] Use PostgreSQL for shared state
- [ ] Enable distributed rate limiting
- [ ] Configure Redis event bus
- [ ] Monitor cache hit ratio
- [ ] Set up alerting on key metrics

### Advanced Optimization

- [ ] Tune PostgreSQL server settings
- [ ] Configure Redis persistence appropriately
- [ ] Implement request batching in clients
- [ ] Use model-specific token limits
- [ ] Implement token rotation for high-volume clients

## Related Documentation

- [Configuration Reference](configuration.md)
- [Caching Strategy](caching-strategy.md)
- [Instrumentation Guide](instrumentation.md)
- [PostgreSQL Troubleshooting](postgresql-troubleshooting.md)
- [Security Best Practices](security.md)
