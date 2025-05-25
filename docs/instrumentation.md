# Instrumentation Middleware: Usage & Extension Guide

## Overview
The async instrumentation middleware provides non-blocking, streaming-capable instrumentation for all API calls handled by the LLM Proxy. It captures request/response metadata and emits events to a pluggable event bus for downstream processing (e.g., file, cloud, analytics).

## Enabling Instrumentation
To enable the middleware, set the following environment variable:

```sh
OBSERVABILITY_ENABLED=true
```

You can also control the event buffer size:

```sh
OBSERVABILITY_BUFFER_SIZE=200  # Default: 100
```

## Configuration Reference
- `OBSERVABILITY_ENABLED` (bool): Enable/disable the instrumentation middleware (default: false)
- `OBSERVABILITY_BUFFER_SIZE` (int): Buffer size for event bus (default: 100)

## How It Works
- When enabled, the middleware wraps all proxy requests and responses.
- Captures request ID, method, path, status, duration, headers, and full (streamed) response body.
- Emits an event to the configured event bus (in-memory by default).
- Event delivery is fully async and non-blocking.

## Event Bus Backends
- **In-Memory** (default): Fast, simple, for local/dev use.
- **Custom/Redis**: Implement the `EventBus` interface and inject via config for distributed or persistent event delivery.

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

## Example: Enabling in Docker
```sh
docker run -d \
  -e OBSERVABILITY_ENABLED=true \
  -e OBSERVABILITY_BUFFER_SIZE=200 \
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