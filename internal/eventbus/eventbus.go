package eventbus

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Event represents an observability event emitted by the proxy.
type Event struct {
	LogID           int64 // Monotonic event log ID
	RequestID       string
	Method          string
	Path            string
	Status          int
	Duration        time.Duration
	ResponseHeaders http.Header
	ResponseBody    []byte
	RequestBody     []byte
}

// EventBus is a simple interface for publishing events to subscribers.
type EventBus interface {
	Publish(ctx context.Context, evt Event)
	Subscribe() <-chan Event
	Stop()
}

type busStats struct {
	published atomic.Int64
	dropped   atomic.Int64
}

// InMemoryEventBus is an EventBus implementation backed by a buffered channel and
// fan-out broadcasting to multiple subscribers. Events are dispatched
// asynchronously to avoid blocking the request path.
type InMemoryEventBus struct {
	bufferSize    int
	ch            chan Event
	subsMu        sync.RWMutex
	subscribers   []chan Event
	stopCh        chan struct{}
	wg            sync.WaitGroup
	retryInterval time.Duration
	maxRetries    int

	stats busStats
}

// NewInMemoryEventBus creates a new in-memory event bus with the given buffer size.
func NewInMemoryEventBus(bufferSize int) *InMemoryEventBus {
	b := &InMemoryEventBus{
		bufferSize:    bufferSize,
		ch:            make(chan Event, bufferSize),
		stopCh:        make(chan struct{}),
		retryInterval: 10 * time.Millisecond,
		maxRetries:    3,
	}
	b.wg.Add(1)
	go b.loop()
	return b
}

// Publish sends an event to the bus without blocking if the buffer is full.
func (b *InMemoryEventBus) Publish(ctx context.Context, evt Event) {
	select {
	case b.ch <- evt:
		b.stats.published.Add(1)
	default:
		b.stats.dropped.Add(1)
	}
}

// Subscribe returns a channel that receives events published to the bus.
// Each subscriber receives all events.
func (b *InMemoryEventBus) Subscribe() <-chan Event {
	sub := make(chan Event, b.bufferSize)
	b.subsMu.Lock()
	b.subscribers = append(b.subscribers, sub)
	b.subsMu.Unlock()
	return sub
}

func (b *InMemoryEventBus) loop() {
	defer b.wg.Done()
	for {
		select {
		case evt := <-b.ch:
			b.dispatch(evt)
		case <-b.stopCh:
			b.subsMu.Lock()
			for _, sub := range b.subscribers {
				close(sub)
			}
			b.subscribers = nil
			b.subsMu.Unlock()
			return
		}
	}
}

func (b *InMemoryEventBus) dispatch(evt Event) {
	b.subsMu.RLock()
	subs := append([]chan Event(nil), b.subscribers...)
	b.subsMu.RUnlock()
	for _, sub := range subs {
		sent := false
		for i := 0; i <= b.maxRetries; i++ {
			select {
			case sub <- evt:
				sent = true
			default:
				time.Sleep(b.retryInterval * time.Duration(i+1))
			}
			if sent {
				break
			}
		}
	}
}

// Stop gracefully stops the event bus and closes all subscriber channels.
func (b *InMemoryEventBus) Stop() {
	close(b.stopCh)
	b.wg.Wait()
}

// Stats returns the number of published and dropped events.
func (b *InMemoryEventBus) Stats() (published, dropped int) {
	return int(b.stats.published.Load()), int(b.stats.dropped.Load())
}

// Note: Legacy Redis LIST backend has been removed. Use Redis Streams for production
// deployments requiring durability and guaranteed delivery.
