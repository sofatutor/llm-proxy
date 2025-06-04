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

## References
- See `internal/middleware/instrumentation.go` for the middleware implementation.
- See `internal/eventbus/eventbus.go` for the event bus interface and in-memory backend.
- See `docs/issues/phase-5-generic-async-middleware.md` for the original design issue.

---
For questions or advanced integration, open an issue or see the code comments for extension points. 