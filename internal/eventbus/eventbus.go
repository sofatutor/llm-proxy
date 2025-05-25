package eventbus

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// Event represents an observability event emitted by the proxy.
type Event struct {
	RequestID       string
	Method          string
	Path            string
	Status          int
	Duration        time.Duration
	ResponseHeaders http.Header
	ResponseBody    []byte
}

// EventBus is a simple interface for publishing events to subscribers.
type EventBus interface {
	Publish(ctx context.Context, evt Event)
	Subscribe() <-chan Event
}

// InMemoryEventBus is a basic EventBus implementation backed by a buffered channel.
type InMemoryEventBus struct {
	ch   chan Event
	once sync.Once
}

// NewInMemoryEventBus creates a new in-memory event bus with the given buffer size.
func NewInMemoryEventBus(bufferSize int) *InMemoryEventBus {
	return &InMemoryEventBus{ch: make(chan Event, bufferSize)}
}

// Publish sends an event to the bus without blocking if the buffer is full.
func (b *InMemoryEventBus) Publish(ctx context.Context, evt Event) {
	select {
	case b.ch <- evt:
	default:
		// drop event if buffer is full
	}
}

// Subscribe returns a channel that receives events published to the bus.
// Multiple subscribers can read from the same channel.
func (b *InMemoryEventBus) Subscribe() <-chan Event {
	b.once.Do(func() {
		// ensure channel is created
		if b.ch == nil {
			b.ch = make(chan Event, 1)
		}
	})
	return b.ch
}
