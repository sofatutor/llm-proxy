# Instrumentation Middleware: Usage & Extension Guide

## Overview
The async instrumentation middleware provides non-blocking, streaming-capable instrumentation for all API calls handled by the LLM Proxy. It captures request/response metadata and emits events to a pluggable event bus for downstream processing (e.g., file, cloud, analytics).

## Event Bus & Dispatcher Architecture
- The async event bus is now always enabled and handles all API instrumentation events.
- The event bus supports multiple subscribers (fan-out), batching, retry logic, and graceful shutdown.
- Both in-memory and Redis backends are available for local and distributed event delivery.
- Persistent event logging is handled by a dispatcher CLI or the `--file-event-log` flag on the server, which writes events to a JSONL file.
- Middleware captures and restores the request body for all events, and the event context is richer for diagnostics and debugging.

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
- `FILE_EVENT_LOG`: Path to persistent event log file (enables file event logging via dispatcher)

## How It Works
- The middleware wraps all proxy requests and responses.
- Captures request ID, method, path, status, duration, headers, and full (streamed) response body.
- Emits an event to the async event bus (in-memory or Redis).
- Event delivery is fully async, non-blocking, batched, and resilient to failures.

## Event Bus Backends
- **In-Memory** (default): Fast, simple, for local/dev use.
- **Redis**: For distributed event delivery and multi-process fan-out.
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

Configure both the proxy and dispatcher to use Redis:

```bash
LLM_PROXY_EVENT_BUS=redis llm-proxy ...
LLM_PROXY_EVENT_BUS=redis llm-proxy dispatcher ...
```

This enables full async event delivery and observability pipeline testing across processes.

## Production Reliability Warning: Event Retention & Loss

> Important: If the Redis-backed event log expires or is trimmed before the dispatcher reads events, those events are irretrievably lost. The dispatcher will log warnings like "Missed events due to TTL or trimming" when it detects gaps.

Recommendations for production:

- Size retention to your worst-case lag: increase the Redis list TTL and max length so they exceed the maximum expected dispatcher downtime/lag and event volume burst.
- Keep the dispatcher continuously running and sized appropriately: raise batch size and processing concurrency to keep up with peak throughput.
- Monitor gaps: alert on warnings about missed events and on dispatcher lag.
- If you require strict, zero-loss semantics, consider a durable queue with consumer offsets (e.g., Redis Streams with consumer groups, Kafka) instead of a simple Redis list with TTL/trim.

## References
- See `internal/middleware/instrumentation.go` for the middleware implementation.
- See `internal/eventbus/eventbus.go` for the event bus interface and in-memory backend.
- See `internal/dispatcher/` for the pluggable dispatcher architecture.
- See `docs/issues/phase-5-generic-async-middleware.md` for the original design issue.
- See `docs/issues/phase-5-event-dispatcher-service.md` for the dispatcher design.

---
For questions or advanced integration, open an issue or see the code comments for extension points. 