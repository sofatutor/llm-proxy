# EventBus Package

This package provides an asynchronous, non-blocking event bus for observability and instrumentation events in the LLM proxy. It supports both in-memory and Redis backends for local and distributed event delivery.

## Purpose & Responsibilities

The `eventbus` package serves as the foundation of the proxy's observability pipeline:

- **Non-blocking Event Publishing**: Events are published asynchronously to avoid blocking request handling
- **Fan-out Broadcasting**: Multiple subscribers can receive all published events
- **Backend Flexibility**: Supports in-memory (single process) and Redis (distributed) backends
- **Graceful Degradation**: Buffer overflow drops events rather than blocking
- **Monotonic Event IDs**: Redis backend assigns sequential `LogID` values for ordering

## Event Schema

```go
type Event struct {
    LogID           int64           // Monotonic event log ID (Redis only)
    RequestID       string          // Unique request identifier
    Method          string          // HTTP method (GET, POST, etc.)
    Path            string          // Request path
    Status          int             // HTTP response status code
    Duration        time.Duration   // Request duration
    ResponseHeaders http.Header     // Response headers
    ResponseBody    []byte          // Response body
    RequestBody     []byte          // Request body
}
```

## EventBus Interface

All event bus implementations satisfy this interface:

```go
type EventBus interface {
    // Publish sends an event to the bus (non-blocking)
    Publish(ctx context.Context, evt Event)
    
    // Subscribe returns a channel receiving all published events
    Subscribe() <-chan Event
    
    // Stop gracefully shuts down the event bus
    Stop()
}
```

## In-Memory Implementation

The `InMemoryEventBus` is suitable for single-process deployments and local development.

### Usage

```go
package main

import (
    "context"
    "time"
    
    "github.com/sofatutor/llm-proxy/internal/eventbus"
)

func main() {
    // Create event bus with buffer size of 1000
    bus := eventbus.NewInMemoryEventBus(1000)
    defer bus.Stop()
    
    // Subscribe to events
    events := bus.Subscribe()
    
    // Start consumer goroutine
    go func() {
        for evt := range events {
            // Process event
            log.Printf("Event: %s %s -> %d", evt.Method, evt.Path, evt.Status)
        }
    }()
    
    // Publish events
    bus.Publish(context.Background(), eventbus.Event{
        RequestID: "req-123",
        Method:    "POST",
        Path:      "/v1/chat/completions",
        Status:    200,
        Duration:  150 * time.Millisecond,
    })
}
```

### Characteristics

| Feature | Behavior |
|---------|----------|
| Delivery | Single process only |
| Buffer | Configurable via `bufferSize` parameter |
| Overflow | Events dropped (non-blocking) |
| Retry | Up to 3 retries with exponential backoff per subscriber |
| Shutdown | Graceful close of all subscriber channels |

### Statistics

```go
bus := eventbus.NewInMemoryEventBus(1000)
// ... publish events ...
published, dropped := bus.Stats()
log.Printf("Published: %d, Dropped: %d", published, dropped)
```

## Redis Implementation

The `RedisEventBus` supports distributed deployments where multiple processes need to share events.

### Usage

```go
package main

import (
    "context"
    "time"
    
    "github.com/redis/go-redis/v9"
    "github.com/sofatutor/llm-proxy/internal/eventbus"
)

func main() {
    // Create Redis client
    client := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })
    
    // Wrap with adapter
    adapter := &eventbus.RedisGoClientAdapter{Client: client}
    
    // Create event bus for publishing
    bus := eventbus.NewRedisEventBusPublisher(adapter, "llm-proxy:events")
    
    // Publish events (non-blocking)
    bus.Publish(context.Background(), eventbus.Event{
        RequestID: "req-456",
        Method:    "POST",
        Path:      "/v1/chat/completions",
        Status:    200,
        Duration:  200 * time.Millisecond,
    })
}
```

### Persistent Log Mode

For durable event storage with TTL and size limits:

```go
// Create log-based event bus
bus := eventbus.NewRedisEventBusLog(
    adapter,
    "llm-proxy:events",
    24 * time.Hour,   // TTL
    10000,            // Max list length
)

// Read events (non-destructive)
events, err := bus.ReadEvents(ctx, 0, -1)  // All events

// Get event count
count, err := bus.EventCount(ctx)
```

### Characteristics

| Feature | Behavior |
|---------|----------|
| Delivery | Distributed across processes |
| Storage | Redis LIST with JSON serialization |
| TTL | Configurable expiration via `NewRedisEventBusLog` |
| Trimming | Automatic via `maxLen` parameter |
| Ordering | Monotonic `LogID` via Redis INCR |

## Configuration Options

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `OBSERVABILITY_BUFFER_SIZE` | Event buffer size for in-memory bus | `1000` |
| `LLM_PROXY_EVENT_BUS` | Backend type: `memory` or `redis` | `memory` |
| `REDIS_URL` | Redis connection URL (when using Redis backend) | - |

## Integration Patterns

### With Instrumentation Middleware

The event bus is typically connected to the instrumentation middleware:

```go
// Setup in server initialization
bus := eventbus.NewInMemoryEventBus(bufferSize)
instrumentationHandler := middleware.NewInstrumentation(bus, logger)

// Events are published automatically for each proxied request
```

### With Dispatcher Service

The dispatcher subscribes to the event bus and forwards events to external services:

```go
// Subscribe to event bus
events := bus.Subscribe()

// Process events
for evt := range events {
    payload := transformer.Transform(evt)
    plugin.SendEvents(ctx, []EventPayload{payload})
}
```

See the [dispatcher package](../dispatcher/README.md) for detailed dispatcher documentation.

### Production Architecture

For distributed deployments:

```
┌─────────────┐     ┌─────────────┐
│   Proxy 1   │     │   Proxy 2   │
│  (Publisher)│     │  (Publisher)│
└──────┬──────┘     └──────┬──────┘
       │                   │
       └───────┬───────────┘
               │
        ┌──────▼──────┐
        │    Redis    │
        │   EventBus  │
        └──────┬──────┘
               │
        ┌──────▼──────┐
        │ Dispatcher  │
        │  (Consumer) │
        └─────────────┘
```

## Testing Guidance

### Unit Testing with Mock Event Bus

```go
type MockEventBus struct {
    events []eventbus.Event
    mu     sync.Mutex
}

func (m *MockEventBus) Publish(ctx context.Context, evt eventbus.Event) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.events = append(m.events, evt)
}

func (m *MockEventBus) Subscribe() <-chan eventbus.Event {
    ch := make(chan eventbus.Event, 100)
    // Return buffered channel for testing
    return ch
}

func (m *MockEventBus) Stop() {}
```

### Integration Testing

```go
func TestEventBusIntegration(t *testing.T) {
    bus := eventbus.NewInMemoryEventBus(100)
    defer bus.Stop()
    
    events := bus.Subscribe()
    
    // Publish test event
    bus.Publish(context.Background(), eventbus.Event{
        RequestID: "test-123",
        Method:    "GET",
        Path:      "/health",
        Status:    200,
    })
    
    // Verify event received
    select {
    case evt := <-events:
        assert.Equal(t, "test-123", evt.RequestID)
    case <-time.After(time.Second):
        t.Fatal("timeout waiting for event")
    }
}
```

## Related Documentation

- [Dispatcher Package](../dispatcher/README.md) - Event processing and backend delivery
- [Middleware Package](../middleware/README.md) - HTTP middleware including instrumentation
- [Instrumentation Guide](../../docs/instrumentation.md) - Complete observability documentation

## Files

| File | Description |
|------|-------------|
| `eventbus.go` | Core interfaces, Event struct, and implementations |
