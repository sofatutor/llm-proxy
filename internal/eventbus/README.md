# EventBus Package

Asynchronous, non-blocking event bus for observability and instrumentation events in the LLM proxy.

## Purpose & Responsibilities

- **Non-blocking Event Publishing**: Events published asynchronously to avoid blocking request handling
- **Fan-out Broadcasting**: Multiple subscribers receive all published events
- **Backend Flexibility**: In-memory (single process) or Redis (distributed) backends
- **Graceful Degradation**: Buffer overflow drops events rather than blocking
- **Monotonic Event IDs**: Redis backend assigns sequential `LogID` values for ordering

## Architecture

```mermaid
flowchart TB
    subgraph Publishers
        P1[Proxy Instance 1]
        P2[Proxy Instance 2]
    end
    
    subgraph EventBus["Event Bus (In-Memory or Redis)"]
        Buffer[Event Buffer]
    end
    
    subgraph Subscribers
        D[Dispatcher Service]
        M[Metrics Collector]
    end
    
    P1 -->|Publish| Buffer
    P2 -->|Publish| Buffer
    Buffer -->|Subscribe| D
    Buffer -->|Subscribe| M
```

## Implementations

### In-Memory EventBus

Best for single-process deployments and local development.

| Feature | Behavior |
|---------|----------|
| Delivery | Single process only |
| Buffer | Configurable size (default: 1000) |
| Overflow | Events dropped (non-blocking) |
| Retry | Up to 3 retries with exponential backoff |
| Shutdown | Graceful close of subscriber channels |

**Key Functions**: `NewInMemoryEventBus(bufferSize)`, `Publish()`, `Subscribe()`, `Stop()`, `Stats()`

### Redis EventBus

Best for distributed deployments where multiple processes share events.

| Feature | Behavior |
|---------|----------|
| Delivery | Distributed across processes |
| Storage | Redis LIST with JSON serialization |
| TTL | Configurable expiration |
| Trimming | Automatic via max length |
| Ordering | Monotonic `LogID` via Redis INCR |

**Key Functions**: `NewRedisEventBusPublisher()`, `NewRedisEventBusLog()`, `ReadEvents()`, `EventCount()`

## Event Schema

The `Event` struct captures HTTP request/response data for observability:

| Field | Type | Description |
|-------|------|-------------|
| `LogID` | `int64` | Monotonic event ID (Redis only) |
| `RequestID` | `string` | Unique request identifier |
| `Method` | `string` | HTTP method |
| `Path` | `string` | Request path |
| `Status` | `int` | Response status code |
| `Duration` | `time.Duration` | Request duration |
| `RequestBody` | `[]byte` | Request body |
| `ResponseBody` | `[]byte` | Response body |
| `ResponseHeaders` | `http.Header` | Response headers |

## Configuration Options

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `OBSERVABILITY_BUFFER_SIZE` | Event buffer size for in-memory bus | `1000` |
| `LLM_PROXY_EVENT_BUS` | Backend type: `memory` or `redis` | `memory` |
| `REDIS_URL` | Redis connection URL (when using Redis) | - |

## Integration Flow

```mermaid
sequenceDiagram
    participant Client
    participant Proxy
    participant Middleware
    participant EventBus
    participant Dispatcher
    participant Backend
    
    Client->>Proxy: HTTP Request
    Proxy->>Middleware: Process Request
    Middleware->>EventBus: Publish(Event)
    EventBus-->>Middleware: (non-blocking)
    Middleware->>Proxy: Response
    Proxy->>Client: HTTP Response
    
    EventBus->>Dispatcher: Event (via Subscribe)
    Dispatcher->>Backend: SendEvents(batch)
```

## Testing Guidance

- **Unit Tests**: Create a mock implementing `EventBus` interface to capture published events
- **Integration Tests**: Use `NewInMemoryEventBus` with small buffer size to test overflow behavior
- **Existing Tests**: See `eventbus_test.go` for comprehensive examples

## Related Documentation

- [Dispatcher Package](../dispatcher/README.md) - Event processing and backend delivery
- [Instrumentation Guide](../../docs/instrumentation.md) - Complete observability documentation

## Files

| File | Description |
|------|-------------|
| `eventbus.go` | Core interfaces, Event struct, and implementations |
