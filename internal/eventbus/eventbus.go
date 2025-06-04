package eventbus

import (
	"context"
	"encoding/json"
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
	RequestBody     []byte
}

// EventBus is a simple interface for publishing events to subscribers.
type EventBus interface {
	Publish(ctx context.Context, evt Event)
	Subscribe() <-chan Event
	Stop()
}

type busStats struct {
	published int
	dropped   int
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

	statsMu sync.Mutex
	stats   busStats
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
		b.statsMu.Lock()
		b.stats.published++
		b.statsMu.Unlock()
	default:
		b.statsMu.Lock()
		b.stats.dropped++
		b.statsMu.Unlock()
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
	b.statsMu.Lock()
	defer b.statsMu.Unlock()
	return b.stats.published, b.stats.dropped
}

// RedisClient defines the minimal operations required by the RedisEventBus.
type RedisClient interface {
	LPush(ctx context.Context, key string, values ...interface{}) error
	BRPop(ctx context.Context, timeout time.Duration, keys ...string) ([]string, error)
}

// RedisEventBus is a Redis-backed EventBus implementation. Events are encoded as
// JSON and pushed to a Redis list. A background goroutine pops events and
// broadcasts them to subscribers.
type RedisEventBus struct {
	client        RedisClient
	key           string
	subsMu        sync.RWMutex
	subscribers   []chan Event
	stopCh        chan struct{}
	wg            sync.WaitGroup
	retryInterval time.Duration
	maxRetries    int

	statsMu sync.Mutex
	stats   busStats
}

// NewRedisEventBus creates a new Redis-backed event bus.
func NewRedisEventBus(client RedisClient, key string) *RedisEventBus {
	b := &RedisEventBus{
		client:        client,
		key:           key,
		stopCh:        make(chan struct{}),
		retryInterval: 10 * time.Millisecond,
		maxRetries:    3,
	}
	b.wg.Add(1)
	go b.loop()
	return b
}

// Publish pushes the event JSON to the Redis list.
func (b *RedisEventBus) Publish(ctx context.Context, evt Event) {
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}
	if err := b.client.LPush(ctx, b.key, data); err != nil {
		b.statsMu.Lock()
		b.stats.dropped++
		b.statsMu.Unlock()
		return
	}
	b.statsMu.Lock()
	b.stats.published++
	b.statsMu.Unlock()
}

// Subscribe returns a channel that receives events popped from Redis.
func (b *RedisEventBus) Subscribe() <-chan Event {
	sub := make(chan Event, 1)
	b.subsMu.Lock()
	b.subscribers = append(b.subscribers, sub)
	b.subsMu.Unlock()
	return sub
}

func (b *RedisEventBus) loop() {
	defer b.wg.Done()
	for {
		select {
		case <-b.stopCh:
			b.subsMu.Lock()
			for _, sub := range b.subscribers {
				close(sub)
			}
			b.subscribers = nil
			b.subsMu.Unlock()
			return
		default:
			res, err := b.client.BRPop(context.Background(), time.Second, b.key)
			if err != nil || len(res) == 0 {
				continue
			}
			for _, item := range res {
				var evt Event
				if err := json.Unmarshal([]byte(item), &evt); err == nil {
					b.dispatch(evt)
				}
			}
		}
	}
}

func (b *RedisEventBus) dispatch(evt Event) {
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
func (b *RedisEventBus) Stop() {
	close(b.stopCh)
	b.wg.Wait()
}

// Stats returns the number of published and dropped events.
func (b *RedisEventBus) Stats() (published, dropped int) {
	b.statsMu.Lock()
	defer b.statsMu.Unlock()
	return b.stats.published, b.stats.dropped
}
