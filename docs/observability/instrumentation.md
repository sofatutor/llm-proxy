---
title: Instrumentation
parent: Observability
nav_order: 1
---

# Instrumentation Middleware: Usage & Extension Guide

## Overview
The async instrumentation middleware provides non-blocking, streaming-capable instrumentation for all API calls handled by the LLM Proxy. It captures request/response metadata and emits events to a pluggable event bus for downstream processing (e.g., file, cloud, analytics).

**Note:** Instrumentation middleware and audit logging serve different purposes:
- **Instrumentation**: Captures API request/response metadata for observability and analytics
- **Audit Logging**: Records security-sensitive operations for compliance and investigations (see [Audit Events](#audit-events))

Both systems operate independently and can be configured separately.

## Event Bus & Dispatcher Architecture
- The async event bus is now always enabled and handles all API instrumentation events.
- The event bus supports multiple subscribers (fan-out), batching, retry logic, and graceful shutdown.
- Both in-memory and Redis backends are available for local and distributed event delivery.
- Persistent event logging is handled by a dispatcher CLI or the `--file-event-log` flag on the server, which writes events to a JSONL file.
- Middleware captures and restores the request body for all events, and the event context is richer for diagnostics and debugging.

### Relationship to Audit Logging

The instrumentation event bus is separate from the audit logging system:

- **Event Bus**: Captures API request/response data for observability (instrumentation middleware)
- **Audit Logger**: Records security events directly to file/database (audit middleware)

Both systems can run simultaneously:
- Instrumentation events flow through the event bus to dispatchers
- Audit events are written directly to audit logs (file and/or database)
- No overlap in captured data - instrumentation focuses on API performance, audit focuses on security events

## Audit Events

The proxy emits audit events for security-sensitive operations:

### Proxy Request Audit Events
- **Project Inactive (403)**: When a request is denied due to inactive project status
  - Action: `proxy_request`, Result: `denied`, Reason: `project_inactive`
  - Includes: project ID, token ID, client IP, user agent, HTTP method, endpoint
- **Service Unavailable (503)**: When project status check fails due to database errors
  - Action: `proxy_request`, Result: `error`, Reason: `service_unavailable`
  - Includes: error details, project ID, request metadata

### Management API Audit Events
- **Project Lifecycle**: Create, update (including `is_active` changes), delete operations
- **Token Management**: Create, update, revoke (single and batch operations)
- All events include actor identification, request IDs, and operation metadata

Audit events are stored in the database and written to audit log files for compliance and security investigations.

For complete system observability, both should be enabled in production environments.

## Persistent Event Logging
- To persist all events to a file, use the `--file-event-log` flag when running the server:

```sh
llm-proxy server --file-event-log ./data/events.jsonl
```

- Alternatively, use the standalone dispatcher CLI to subscribe to the event bus and write events to a file or other backends:

```sh
llm-proxy dispatcher --backend file --file ./data/events.jsonl
```

## Configuration Reference
- `OBSERVABILITY_ENABLED`: Deprecated; the async event bus is always enabled.
- `OBSERVABILITY_BUFFER_SIZE` (int): Buffer size for event bus (default: 1000)
- `OBSERVABILITY_MAX_REQUEST_BODY_BYTES` (int64): Max bytes of request body captured into observability events (default: 65536). Does not affect proxying.
- `OBSERVABILITY_MAX_RESPONSE_BODY_BYTES` (int64): Max bytes of response body captured into observability events (default: 262144). Does not affect proxying.
- `FILE_EVENT_LOG`: Path to persistent event log file (enables file event logging via dispatcher)

## How It Works
- The middleware wraps all proxy requests and responses.
- Captures request ID, method, path, status, duration, headers, and full (streamed) response body.
- Emits an event to the async event bus (in-memory or Redis).
- Event delivery is fully async, non-blocking, batched, and resilient to failures.

## Event Bus Backends
- **Redis Streams** (`redis-streams`): **Recommended for production**. Provides consumer groups, acknowledgment, at-least-once delivery, and crash recovery. See [Redis Streams Backend](#redis-streams-backend-recommended-for-production).
- **In-Memory** (`in-memory`): Fast, simple, for local/dev use. Single process only. No durability or delivery guarantees.
- **Custom**: Implement the `EventBus` interface for other backends (Kafka, HTTP, etc.).

## Event Schema Example
```go
// eventbus.Event
Event {
  RequestID       string
  Method          string
  Path            string
  Status          int
  Duration        time.Duration
  ResponseHeaders http.Header
  ResponseBody    []byte
}
```

## Example: Enabling Persistent Logging in Docker
```sh
docker run -d \
  -e FILE_EVENT_LOG=./data/events.jsonl \
  ...
```

## Extending the Middleware
- **Custom Event Schema**: Extend `eventbus.Event` or create your own struct. Update the middleware to emit your custom event type.
- **New Event Bus Backends**: Implement the `EventBus` interface (see `internal/eventbus/eventbus.go`). Plug in your backend (e.g., Redis, Kafka, HTTP, etc.).
- **New Consumers/Dispatchers**: Write a dispatcher that subscribes to the event bus and delivers events to your backend (file, cloud, analytics, etc.).

## Example: Custom EventBus Backend
```go
type MyEventBus struct { /* ... */ }
func (b *MyEventBus) Publish(ctx context.Context, evt eventbus.Event) { /* ... */ }
func (b *MyEventBus) Subscribe() <-chan eventbus.Event { /* ... */ }
```

## Dispatcher CLI Commands

The LLM Proxy now includes a powerful, pluggable dispatcher system for sending observability events to external services. The dispatcher supports multiple backends and can be run as a separate service.

### Available Backends

- **file**: Write events to JSONL file
- **lunary**: Send events to Lunary.ai platform
- **helicone**: Send events to Helicone platform

### Basic Usage

```bash
# File output
llm-proxy dispatcher --service file --endpoint events.jsonl

# Lunary integration
llm-proxy dispatcher --service lunary --api-key $LUNARY_API_KEY

# Helicone integration  
llm-proxy dispatcher --service helicone --api-key $HELICONE_API_KEY

# Custom endpoint for Lunary
llm-proxy dispatcher --service lunary --api-key $LUNARY_API_KEY --endpoint https://custom.lunary.ai/v1/runs/ingest
```

### Configuration Options

| Flag | Default | Description |
|------|---------|-------------|
| `--service` | `file` | Backend service (file, lunary, helicone) |
| `--endpoint` | service-specific | API endpoint or file path |
| `--api-key` | - | API key for external services |
| `--buffer` | `1000` | Event bus buffer size |
| `--batch-size` | `100` | Batch size for sending events |
| `--detach` | `false` | Run in background (daemon mode) |

### Environment Variables

- `LLM_PROXY_API_KEY`: API key for the selected service
- `LLM_PROXY_ENDPOINT`: Default endpoint URL

### Event Format

The dispatcher transforms internal events into a rich format suitable for external services:

```json
{
  "type": "llm",
  "event": "start",
  "runId": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2023-12-01T10:00:00Z",
  "input": {"model": "gpt-4", "messages": [...]},
  "output": {"choices": [...]},
  "metadata": {
    "method": "POST",
    "path": "/v1/chat/completions", 
    "status": 200,
    "duration_ms": 1234,
    "request_id": "req-123"
  }
}
```

### Advanced Features

- **Automatic Retry**: Exponential backoff for failed requests
- **Batching**: Configurable batch sizes for efficiency
- **Graceful Shutdown**: SIGINT/SIGTERM handling
- **Extensible**: Easy to add new backends

## Helicone Manual Logger Integration

The Helicone dispatcher plugin transforms LLM Proxy events into Helicone's [Manual Logger format](https://docs.helicone.ai/features/advanced-usage/custom-models). This enables detailed cost tracking, analytics, and monitoring of custom model endpoints through Helicone.

### Payload Mapping Details

Our implementation maps LLM Proxy events to the Helicone Manual Logger format as follows:

```json
{
  "providerRequest": {
    "url": "/v1/chat/completions",
    "json": { "model": "gpt-4", "messages": [...] },
    "meta": {
      "Helicone-Provider": "openai",
      "Helicone-User-Id": "user-123",
      "request_id": "req-456",
      "provider": "openai"
    }
  },
  "providerResponse": {
    "status": 200,
    "headers": {},
    "json": { "choices": [...], "usage": {...} },
    "base64": "..." // for non-JSON responses
  },
  "timing": {
    "startTime": { "seconds": 1640995200, "milliseconds": 0 },
    "endTime": { "seconds": 1640995201, "milliseconds": 250 }
  }
}
```

### Key Features

**Provider Detection**: Automatically sets `Helicone-Provider` header to prevent categorization as "CUSTOM" model, enabling proper cost calculation.

**Usage Injection**: Injects computed token usage into response JSON when available:
```json
{
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 25,
    "total_tokens": 35
  }
}
```

**Request ID Propagation**: Preserves `request_id` from middleware context for correlation.

**Non-JSON Response Handling**: For binary or non-JSON responses:
- Sets `providerResponse.json` to empty object with explanatory note
- Includes `base64` field for binary data when available

**Metadata Enrichment**: Forwards relevant metadata fields and user properties to Helicone headers.

### Configuration

```bash
# Basic usage
llm-proxy dispatcher --service helicone --api-key $HELICONE_API_KEY

# Custom endpoint (e.g., for EU region)
llm-proxy dispatcher --service helicone \
  --api-key $HELICONE_API_KEY \
  --endpoint https://eu.api.helicone.ai/custom/v1/log
```

### References

- [Helicone Manual Logger Documentation](https://docs.helicone.ai/features/advanced-usage/custom-models)
- [Implementation](../internal/dispatcher/plugins/helicone.go): `heliconePayloadFromEvent` function
- [Tests](../internal/dispatcher/plugins/helicone_payload_test.go): Payload transformation examples

## HTTP Response Caching Integration

The proxy includes HTTP response caching that integrates with the instrumentation and observability system. Caching behavior affects both response headers and event publishing.

### Cache Response Headers

When caching is enabled (`HTTP_CACHE_ENABLED=true`), the proxy adds observability headers to all responses:

- **`X-PROXY-CACHE`**: Indicates cache result
  - `hit`: Response served from cache
  - `miss`: Response not in cache, fetched from upstream
- **`X-PROXY-CACHE-KEY`**: The cache key used for the request (useful for debugging cache behavior)
- **`Cache-Status`**: Standard HTTP cache status header
  - `hit`: Cache hit, response served from cache
  - `miss`: Cache miss, response fetched from upstream
  - `bypass`: Caching bypassed (e.g., due to `Cache-Control: no-store`)
  - `stored`: Response was stored in cache after fetch
  - `conditional-hit`: Conditional request (e.g., `If-None-Match`) resulted in 304

### Cache Metrics

The proxy keeps lightweight, provider-agnostic counters to assess cache effectiveness:

- `cache_hits_total`: Number of requests served from cache (including conditional hits)
- `cache_misses_total`: Number of requests that missed the cache
- `cache_bypass_total`: Number of requests where caching was bypassed (e.g., `no-store`)
- `cache_store_total`: Number of responses stored in cache after upstream fetch

Notes:
- Counters are in-memory and surfaced via the existing metrics endpoint when enabled.
- No external metrics provider is required; Prometheus export is optional and not a core dependency.

### Event Bus Behavior with Caching

The caching system integrates with the instrumentation middleware to optimize performance:

- **Cache Hits**: Events are **not published** to the event bus for cache hits (including conditional hits). This prevents duplicate instrumentation data and reduces event bus load.
- **Cache Misses and Stores**: Events **are published** normally when responses are fetched from upstream, whether they get cached or not.

This behavior ensures that:
- Each unique API call is instrumented exactly once (when first fetched)
- Cache performance doesn't impact event bus throughput
- Downstream analytics systems receive clean, non-duplicated data

### Example Headers

```http
# Cache hit response
HTTP/1.1 200 OK
X-PROXY-CACHE: hit
X-PROXY-CACHE-KEY: llmproxy:cache:proj123:GET:/v1/models:accept-application/json
Cache-Status: hit
Content-Type: application/json

# Cache miss response
HTTP/1.1 200 OK
X-PROXY-CACHE: miss
X-PROXY-CACHE-KEY: llmproxy:cache:proj123:POST:/v1/chat/completions:accept-application/json:body-hash-abc123
Cache-Status: stored
Content-Type: application/json
```

### Debugging Cache Behavior

Use the benchmark tool with `--debug` flag to inspect cache headers:

```bash
llm-proxy benchmark \
  --base-url "http://localhost:8080" \
  --endpoint "/v1/chat/completions" \
  --token "$PROXY_TOKEN" \
  --requests 10 --concurrency 1 \
  --cache \
  --debug \
  --json '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"test"}]}'
```

This will show sample responses with all headers, making it easy to verify cache behavior.

## Prometheus Metrics Endpoint

The proxy provides an additional Prometheus-compatible metrics endpoint for monitoring and alerting. This endpoint complements the existing JSON metrics endpoint without replacing it.

### Endpoints

- **`/metrics`**: Provider-agnostic JSON metrics (default format)
- **`/metrics/prometheus`**: Prometheus text exposition format

Both endpoints are enabled when `ENABLE_METRICS=true` (default).

Both endpoints are available when `ENABLE_METRICS=true` (default).

### Available Metrics

The Prometheus endpoint exposes the following metrics:

#### Application Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `llm_proxy_uptime_seconds` | gauge | Time since the server started |
| `llm_proxy_requests_total` | counter | Total number of proxy requests |
| `llm_proxy_errors_total` | counter | Total number of proxy errors |
| `llm_proxy_cache_hits_total` | counter | Total number of cache hits |
| `llm_proxy_cache_misses_total` | counter | Total number of cache misses |
| `llm_proxy_cache_bypass_total` | counter | Total number of cache bypasses |
| `llm_proxy_cache_stores_total` | counter | Total number of cache stores |

#### Go Runtime Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `llm_proxy_goroutines` | gauge | Number of goroutines currently running |
| `llm_proxy_memory_heap_alloc_bytes` | gauge | Number of heap bytes allocated and currently in use |
| `llm_proxy_memory_heap_sys_bytes` | gauge | Number of heap bytes obtained from the OS |
| `llm_proxy_memory_heap_idle_bytes` | gauge | Number of heap bytes waiting to be used |
| `llm_proxy_memory_heap_inuse_bytes` | gauge | Number of heap bytes that are in use |
| `llm_proxy_memory_heap_released_bytes` | gauge | Number of heap bytes released to the OS |
| `llm_proxy_memory_total_alloc_bytes` | counter | Total number of bytes allocated (cumulative) |
| `llm_proxy_memory_sys_bytes` | gauge | Total number of bytes obtained from the OS |
| `llm_proxy_memory_mallocs_total` | counter | Total number of malloc operations |
| `llm_proxy_memory_frees_total` | counter | Total number of free operations |
| `llm_proxy_gc_runs_total` | counter | Total number of GC runs |
| `llm_proxy_gc_pause_total_seconds` | counter | Total GC pause time in seconds |
| `llm_proxy_gc_next_bytes` | gauge | Target heap size for next GC cycle |

### Example Output

```
# HELP llm_proxy_uptime_seconds Time since the server started
# TYPE llm_proxy_uptime_seconds gauge
llm_proxy_uptime_seconds 3542.12
# HELP llm_proxy_requests_total Total number of proxy requests
# TYPE llm_proxy_requests_total counter
llm_proxy_requests_total 1523
# HELP llm_proxy_errors_total Total number of proxy errors
# TYPE llm_proxy_errors_total counter
llm_proxy_errors_total 12
# HELP llm_proxy_cache_hits_total Total number of cache hits
# TYPE llm_proxy_cache_hits_total counter
llm_proxy_cache_hits_total 842
# HELP llm_proxy_cache_misses_total Total number of cache misses
# TYPE llm_proxy_cache_misses_total counter
llm_proxy_cache_misses_total 681
# HELP llm_proxy_cache_bypass_total Total number of cache bypasses
# TYPE llm_proxy_cache_bypass_total counter
llm_proxy_cache_bypass_total 0
# HELP llm_proxy_cache_stores_total Total number of cache stores
# TYPE llm_proxy_cache_stores_total counter
llm_proxy_cache_stores_total 681
# HELP llm_proxy_goroutines Number of goroutines currently running
# TYPE llm_proxy_goroutines gauge
llm_proxy_goroutines 12
# HELP llm_proxy_memory_heap_alloc_bytes Number of heap bytes allocated and currently in use
# TYPE llm_proxy_memory_heap_alloc_bytes gauge
llm_proxy_memory_heap_alloc_bytes 2097152
# HELP llm_proxy_memory_total_alloc_bytes Total number of bytes allocated (cumulative)
# TYPE llm_proxy_memory_total_alloc_bytes counter
llm_proxy_memory_total_alloc_bytes 104857600
# HELP llm_proxy_gc_runs_total Total number of GC runs
# TYPE llm_proxy_gc_runs_total counter
llm_proxy_gc_runs_total 42
```

### Prometheus Scrape Configuration

Add the following to your Prometheus configuration:

```yaml
scrape_configs:
  - job_name: 'llm-proxy'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics/prometheus'
    scrape_interval: 15s
```

### Example Queries

```promql
# Request rate (per second)
rate(llm_proxy_requests_total[5m])

# Error rate
rate(llm_proxy_errors_total[5m]) / rate(llm_proxy_requests_total[5m])

# Cache hit ratio
llm_proxy_cache_hits_total / (llm_proxy_cache_hits_total + llm_proxy_cache_misses_total)

# Total uptime in hours
llm_proxy_uptime_seconds / 3600

# Memory usage trend
rate(llm_proxy_memory_total_alloc_bytes[5m])

# Heap allocation
llm_proxy_memory_heap_alloc_bytes

# GC frequency
rate(llm_proxy_gc_runs_total[5m])

# Active goroutines
llm_proxy_goroutines
```

### Testing

```bash
# Check Prometheus metrics
curl http://localhost:8080/metrics/prometheus

# Compare with JSON format
curl http://localhost:8080/metrics | jq .
```

### Grafana Dashboard

A ready-to-import Grafana dashboard is available for visualizing LLM Proxy metrics:

- **Dashboard JSON**: [`deploy/helm/llm-proxy/dashboards/llm-proxy.json`](../../deploy/helm/llm-proxy/dashboards/llm-proxy.json)
- **Documentation**: See the [dashboards README](../../deploy/helm/llm-proxy/dashboards/README.md) for import instructions

The dashboard includes panels for:
- Request rate, error rate, and uptime
- Cache performance (hits, misses, bypass, stores)
- Memory usage and Go runtime metrics
- Garbage collection statistics

Import the dashboard into Grafana and configure it to use your Prometheus datasource.

### Notes

- The Prometheus endpoint is lightweight and has no external dependencies
- Metrics are in-memory and reset on server restart
- Both JSON and Prometheus endpoints can be used simultaneously
- No secrets are exposed in metrics output

## Important: In-Memory vs. Redis Event Bus

- The **in-memory event bus** only works within a single process. If you run the proxy and dispatcher as separate processes or containers, they will not share events.
- For distributed, multi-process, or containerized setups, **Redis is required** as the event bus backend.

### Local Redis Setup for Manual Testing

Add the following to your `docker-compose.yml` to run Redis locally:

```yaml
redis:
  image: redis:7
  container_name: llm-proxy-redis
  ports:
    - "6379:6379"
  restart: unless-stopped
```

Configure both the proxy and dispatcher to use Redis Streams:

```bash
LLM_PROXY_EVENT_BUS=redis-streams llm-proxy server ...
LLM_PROXY_EVENT_BUS=redis-streams llm-proxy dispatcher ...
```

This enables full async event delivery and observability pipeline testing across processes.

## Redis Streams Backend (Recommended for Production)

For production deployments requiring **guaranteed delivery** and **at-least-once semantics**, use the Redis Streams backend. It provides:

- **Consumer Groups**: Multiple dispatcher instances can share the workload
- **Acknowledgment**: Messages are only removed after successful processing
- **Crash Recovery**: Pending messages from crashed consumers are automatically claimed
- **Durable Storage**: Messages persist until acknowledged, surviving restarts

### Enabling Redis Streams

Set the event bus backend to `redis-streams`:

```bash
LLM_PROXY_EVENT_BUS=redis-streams llm-proxy server ...
```

### Configuration Options

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `LLM_PROXY_EVENT_BUS` | Event bus backend | `redis-streams` |
| `REDIS_ADDR` | Redis server address | `localhost:6379` |
| `REDIS_DB` | Redis database number | `0` |
| `REDIS_STREAM_KEY` | Stream key name | `llm-proxy-events` |
| `REDIS_CONSUMER_GROUP` | Consumer group name | `llm-proxy-dispatchers` |
| `REDIS_CONSUMER_NAME` | Consumer name (unique per instance) | Auto-generated |
| `REDIS_STREAM_MAX_LEN` | Max stream length (0 = unlimited) | `10000` |
| `REDIS_STREAM_BLOCK_TIME` | Block timeout for reading | `5s` |
| `REDIS_STREAM_CLAIM_TIME` | Min idle time before claiming pending messages | `30s` |
| `REDIS_STREAM_BATCH_SIZE` | Batch size for reading messages | `100` |

### Example Configuration

```bash
# Full Redis Streams configuration
export LLM_PROXY_EVENT_BUS=redis-streams
export REDIS_ADDR=redis.example.com:6379
export REDIS_DB=0
export REDIS_STREAM_KEY=llm-proxy-events
export REDIS_CONSUMER_GROUP=dispatchers
export REDIS_CONSUMER_NAME=dispatcher-1  # Set unique name per instance
export REDIS_STREAM_MAX_LEN=50000
export REDIS_STREAM_BLOCK_TIME=5s
export REDIS_STREAM_CLAIM_TIME=30s
export REDIS_STREAM_BATCH_SIZE=100

llm-proxy server
```

### How It Works

1. **Publishing**: Events are added to the stream via `XADD` with automatic ID generation
2. **Consumer Groups**: Dispatchers join a consumer group and read via `XREADGROUP`
3. **Acknowledgment**: After successful processing, messages are acknowledged via `XACK`
4. **Recovery**: If a consumer crashes, its pending messages are claimed by other consumers after `REDIS_STREAM_CLAIM_TIME`

### Multiple Dispatcher Instances

Redis Streams supports running multiple dispatcher instances that share the workload:

```bash
# Instance 1
REDIS_CONSUMER_NAME=dispatcher-1 llm-proxy dispatcher --service lunary

# Instance 2 (on another host or container)
REDIS_CONSUMER_NAME=dispatcher-2 llm-proxy dispatcher --service lunary
```

Each message is delivered to exactly one consumer in the group. If a consumer fails, its pending messages are automatically reassigned.

### Multiple Dispatcher Services (Fan-out)

If you want **multiple backends** (e.g. **file** and **helicone**) to each receive **100% of events**, do **not** run them in the same consumer group.

- **Same `REDIS_CONSUMER_GROUP`** across multiple dispatcher services = **load balancing** (each event goes to only one service)
- **Different `REDIS_CONSUMER_GROUP`** per service = **fan-out** (each service reads the full stream independently)

Example:

```bash
# File logger consumes all events
REDIS_CONSUMER_GROUP=llm-proxy-dispatchers-file \
  llm-proxy dispatcher --service file --endpoint events.jsonl

# Helicone logger also consumes all events
REDIS_CONSUMER_GROUP=llm-proxy-dispatchers-helicone \
  llm-proxy dispatcher --service helicone --api-key $HELICONE_API_KEY
```

### Redis Streams vs In-Memory

| Feature | In-Memory | Redis Streams |
|---------|-----------|---------------|
| Delivery guarantee | None (buffer overflow drops events) | At-least-once |
| Processes | Single process only | Distributed across multiple processes/hosts |
| Consumer groups | No | Yes |
| Multiple dispatchers | No | Yes (events distributed via consumer groups) |
| Crash recovery | No | Yes (pending message claiming) |
| Acknowledgment | No | Yes |
| Recommended for | Development, local testing | Production, high reliability |

## Redis Streams Rollout Checklist

Use this checklist when enabling Redis Streams in new environments:

### Prerequisites
- [ ] Redis server accessible from all proxy and dispatcher instances
- [ ] `MANAGEMENT_TOKEN` configured for admin operations

### Configuration
- [ ] Set `LLM_PROXY_EVENT_BUS=redis-streams` on proxy and dispatcher
- [ ] Set `REDIS_ADDR` to your Redis server address
- [ ] Set `REDIS_STREAM_KEY` (default: `llm-proxy-events`)
- [ ] Set `REDIS_CONSUMER_GROUP` (default: `llm-proxy-dispatchers`)
- [ ] Configure `REDIS_STREAM_MAX_LEN` based on expected throughput (default: 10000)

### Verification
- [ ] Verify consumer group exists: `redis-cli XINFO GROUPS llm-proxy-events`
- [ ] Check stream length: `redis-cli XLEN llm-proxy-events`
- [ ] Monitor pending count: `redis-cli XPENDING llm-proxy-events llm-proxy-dispatchers`
- [ ] Verify dispatcher is consuming: check logs for "Using Redis Streams event bus"
- [ ] Confirm events are being acknowledged: pending count should remain stable or decrease

### Monitoring
- [ ] Set up alerts for pending count > 1000 (indicates dispatcher lag)
- [ ] Monitor stream length to ensure it stays below max length
- [ ] Track dispatcher health endpoint for lag warnings
- [ ] Monitor dispatcher logs for claim/recovery messages

### Troubleshooting

**High Pending Count:**
- Increase `REDIS_STREAM_BATCH_SIZE` (default: 100)
- Reduce `REDIS_STREAM_CLAIM_TIME` to claim stuck messages faster (default: 30s)
- Scale horizontally: add more dispatcher instances (they share workload via consumer group)
- Check dispatcher logs for errors or slow backend API calls

**Stream Length Growing:**
- Increase `REDIS_STREAM_MAX_LEN` if losing events due to trimming
- Ensure dispatchers are running and healthy
- Check that dispatchers are acknowledging messages (XACK)

## References
- See `internal/middleware/instrumentation.go` for the middleware implementation.
- See `internal/eventbus/eventbus.go` for the event bus interface and in-memory backend.
- See `internal/dispatcher/` for the pluggable dispatcher architecture.
- See `docs/issues/done/phase-5-generic-async-middleware.md` for the original design issue.
- See `docs/issues/done/phase-5-event-dispatcher-service.md` for the dispatcher design.

---
For questions or advanced integration, open an issue or see the code comments for extension points. 