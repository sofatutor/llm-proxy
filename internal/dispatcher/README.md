# Dispatcher Package

This package provides a pluggable event dispatcher service that consumes events from the event bus and forwards them to external observability platforms. It supports batching, retry logic, and multiple backend plugins.

## Purpose & Responsibilities

The `dispatcher` package bridges the event bus with external services:

- **Event Consumption**: Subscribes to the event bus and processes incoming events
- **Event Transformation**: Converts internal events to formats required by external services
- **Batching**: Groups events for efficient network transmission
- **Retry Logic**: Handles transient failures with exponential backoff
- **Plugin Architecture**: Supports multiple backend services via plugins

## Architecture Overview

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│    EventBus     │────▶│   Dispatcher    │────▶│ Backend Plugin  │
│   (Source)      │     │   Service       │     │  (File/Cloud)   │
└─────────────────┘     └─────────────────┘     └─────────────────┘
                              │
                        ┌─────▼─────┐
                        │Transformer│
                        └───────────┘
```

## BackendPlugin Interface

All backend plugins implement this interface:

```go
type BackendPlugin interface {
    // Init initializes the plugin with configuration
    Init(cfg map[string]string) error
    
    // SendEvents sends a batch of events to the backend
    SendEvents(ctx context.Context, events []EventPayload) error
    
    // Close cleans up resources
    Close() error
}
```

## EventPayload Schema

Events are transformed to this extended format for external services:

```go
type EventPayload struct {
    Type         string          `json:"type"`
    Event        string          `json:"event"`
    RunID        string          `json:"runId"`
    ParentRunID  *string         `json:"parentRunId,omitempty"`
    Name         *string         `json:"name,omitempty"`
    Timestamp    time.Time       `json:"timestamp"`
    Input        json.RawMessage `json:"input,omitempty"`
    Output       json.RawMessage `json:"output,omitempty"`
    UserID       *string         `json:"userId,omitempty"`
    TokensUsage  *TokensUsage    `json:"tokensUsage,omitempty"`
    Metadata     map[string]any  `json:"metadata,omitempty"`
    Tags         []string        `json:"tags,omitempty"`
    LogID        int64           `json:"log_id"`
}
```

## Available Backend Plugins

### File Plugin

Writes events to a JSONL (JSON Lines) file for local storage and debugging.

```go
package main

import (
    "context"
    
    "github.com/sofatutor/llm-proxy/internal/dispatcher"
    "github.com/sofatutor/llm-proxy/internal/dispatcher/plugins"
)

func main() {
    plugin := plugins.NewFilePlugin()
    
    err := plugin.Init(map[string]string{
        "endpoint": "/var/log/llm-proxy/events.jsonl",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer plugin.Close()
    
    // Events will be written as JSON lines
    err = plugin.SendEvents(ctx, events)
}
```

**Configuration**:
| Key | Description | Required |
|-----|-------------|----------|
| `endpoint` | File path for JSONL output | Yes |

### Lunary Plugin

Sends events to [Lunary.ai](https://lunary.ai) for LLM observability.

```go
plugin := plugins.NewLunaryPlugin()

err := plugin.Init(map[string]string{
    "api-key":  os.Getenv("LUNARY_API_KEY"),
    "endpoint": "https://api.lunary.ai/v1/runs/ingest",  // Optional
})
```

**Configuration**:
| Key | Description | Required | Default |
|-----|-------------|----------|---------|
| `api-key` | Lunary API key | Yes | - |
| `endpoint` | API endpoint URL | No | `https://api.lunary.ai/v1/runs/ingest` |

### Helicone Plugin

Sends events to [Helicone](https://helicone.ai) using their Manual Logger format.

```go
plugin := plugins.NewHeliconePlugin()

err := plugin.Init(map[string]string{
    "api-key":  os.Getenv("HELICONE_API_KEY"),
    "endpoint": "https://api.worker.helicone.ai/custom/v1/log",  // Optional
})
```

**Configuration**:
| Key | Description | Required | Default |
|-----|-------------|----------|---------|
| `api-key` | Helicone API key | Yes | - |
| `endpoint` | API endpoint URL | No | `https://api.worker.helicone.ai/custom/v1/log` |

**Helicone-Specific Features**:
- Automatic provider detection (sets `Helicone-Provider` header)
- Token usage injection into response JSON
- Request ID propagation for correlation
- Non-JSON response handling (base64 encoding)

## Dispatcher Service

### Service Configuration

```go
type Config struct {
    BufferSize       int           // Event bus buffer size (default: 1000)
    BatchSize        int           // Events per batch (default: 100)
    FlushInterval    time.Duration // Max time between flushes (default: 5s)
    RetryAttempts    int           // Retry count on failure (default: 3)
    RetryBackoff     time.Duration // Initial backoff duration (default: 1s)
    Plugin           BackendPlugin // Required: backend plugin
    EventTransformer EventTransformer // Optional: custom transformer
    PluginName       string        // Plugin name for offset tracking
    Verbose          bool          // Include debug info in payloads
}
```

### Basic Usage

```go
package main

import (
    "context"
    
    "github.com/sofatutor/llm-proxy/internal/dispatcher"
    "github.com/sofatutor/llm-proxy/internal/dispatcher/plugins"
    "go.uber.org/zap"
)

func main() {
    logger, _ := zap.NewProduction()
    
    // Create and initialize plugin
    plugin := plugins.NewFilePlugin()
    plugin.Init(map[string]string{"endpoint": "events.jsonl"})
    
    // Create dispatcher service
    svc, err := dispatcher.NewService(dispatcher.Config{
        BufferSize:    1000,
        BatchSize:     100,
        FlushInterval: 5 * time.Second,
        Plugin:        plugin,
    }, logger)
    if err != nil {
        log.Fatal(err)
    }
    
    // Run dispatcher (blocks until stopped)
    ctx := context.Background()
    if err := svc.Run(ctx, false); err != nil {
        log.Fatal(err)
    }
}
```

### With External Event Bus

For distributed setups using Redis:

```go
// Create Redis event bus
redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
adapter := &eventbus.RedisGoClientAdapter{Client: redisClient}
bus := eventbus.NewRedisEventBusLog(adapter, "llm-proxy:events", 24*time.Hour, 10000)

// Create dispatcher with external bus
svc, err := dispatcher.NewServiceWithBus(dispatcher.Config{
    Plugin:     plugin,
    PluginName: "file",  // Used for offset tracking
}, logger, bus)
```

### Service Statistics

```go
processed, dropped, sent := svc.Stats()
log.Printf("Processed: %d, Dropped: %d, Sent: %d", processed, dropped, sent)
```

## CLI Usage

The dispatcher can be run as a standalone service:

```bash
# File output
llm-proxy dispatcher --service file --endpoint events.jsonl

# Lunary integration
llm-proxy dispatcher --service lunary --api-key $LUNARY_API_KEY

# Helicone integration
llm-proxy dispatcher --service helicone --api-key $HELICONE_API_KEY

# With custom buffer and batch settings
llm-proxy dispatcher --service lunary \
    --api-key $LUNARY_API_KEY \
    --buffer 2000 \
    --batch-size 50
```

### CLI Options

| Flag | Description | Default |
|------|-------------|---------|
| `--service` | Backend service (file, lunary, helicone) | `file` |
| `--endpoint` | API endpoint or file path | Service-specific |
| `--api-key` | API key for external services | - |
| `--buffer` | Event bus buffer size | `1000` |
| `--batch-size` | Batch size for sending events | `100` |
| `--detach` | Run in background (daemon mode) | `false` |

## Configuration Options

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `LLM_PROXY_API_KEY` | API key for selected service | - |
| `LLM_PROXY_ENDPOINT` | Default endpoint URL | - |
| `LLM_PROXY_EVENT_BUS` | Event bus backend (`memory` or `redis`) | `memory` |

## Batching and Retry Logic

### Batching

Events are batched to improve throughput:

1. Events accumulate until `BatchSize` is reached
2. Or `FlushInterval` elapses since last flush
3. Or service shutdown is initiated

### Retry Behavior

On backend failure:

1. Exponential backoff: `attempt * RetryBackoff`
2. Up to `RetryAttempts` retries per batch
3. Permanent errors (HTTP 4xx) are not retried
4. After all retries exhausted, batch is dropped and logged

```go
// Permanent error (not retried)
type PermanentBackendError struct {
    Message string
}
```

## Event Transformation

### Default Transformer

The default transformer converts `eventbus.Event` to `EventPayload`:

```go
transformer := dispatcher.NewDefaultEventTransformer(verbose)
payload, err := transformer.Transform(evt)
```

### Custom Transformer

Implement the `EventTransformer` interface for custom logic:

```go
type EventTransformer interface {
    Transform(evt eventbus.Event) (*EventPayload, error)
}

type MyTransformer struct{}

func (t *MyTransformer) Transform(evt eventbus.Event) (*EventPayload, error) {
    // Custom transformation logic
    return &EventPayload{
        RunID:     evt.RequestID,
        Timestamp: time.Now(),
        // ...
    }, nil
}
```

## Plugin Registry

The plugin registry provides factory functions for creating plugins:

```go
// Get available plugins
names := plugins.ListPlugins()  // ["file", "lunary", "helicone"]

// Create plugin by name
plugin, err := plugins.NewPlugin("lunary")
```

### Adding Custom Plugins

```go
// Register a custom plugin factory
plugins.Registry["custom"] = func() dispatcher.BackendPlugin {
    return NewCustomPlugin()
}
```

## Testing Guidance

### Unit Testing with Mock Plugin

```go
type MockPlugin struct {
    events []dispatcher.EventPayload
    mu     sync.Mutex
}

func (m *MockPlugin) Init(cfg map[string]string) error { return nil }

func (m *MockPlugin) SendEvents(ctx context.Context, events []dispatcher.EventPayload) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.events = append(m.events, events...)
    return nil
}

func (m *MockPlugin) Close() error { return nil }
```

### Testing Transformer

```go
func TestTransformer(t *testing.T) {
    transformer := dispatcher.NewDefaultEventTransformer(false)
    
    evt := eventbus.Event{
        RequestID: "test-123",
        Method:    "POST",
        Path:      "/v1/chat/completions",
        Status:    200,
    }
    
    payload, err := transformer.Transform(evt)
    assert.NoError(t, err)
    assert.Equal(t, "test-123", payload.RunID)
}
```

## Related Documentation

- [EventBus Package](../eventbus/README.md) - Event publishing and subscription
- [EventTransformer Package](../eventtransformer/README.md) - Event transformation utilities
- [Instrumentation Guide](../../docs/instrumentation.md) - Complete observability documentation

## Files

| File | Description |
|------|-------------|
| `service.go` | Main dispatcher service implementation |
| `plugin.go` | BackendPlugin interface and EventPayload struct |
| `transformer.go` | Event transformation logic |
| `errors.go` | Error types including PermanentBackendError |
| `plugins/registry.go` | Plugin factory registry |
| `plugins/file.go` | File backend plugin |
| `plugins/lunary.go` | Lunary.ai backend plugin |
| `plugins/helicone.go` | Helicone backend plugin |
