package eventbus

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"log"

	"github.com/redis/go-redis/v9"
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

// Extend RedisClient interface for LRANGE, LLEN, EXPIRE, LTRIM, Incr, Get, Set
type RedisClient interface {
	LPush(ctx context.Context, key string, values ...interface{}) error
	LRANGE(ctx context.Context, key string, start, stop int64) ([]string, error)
	LLEN(ctx context.Context, key string) (int64, error)
	EXPIRE(ctx context.Context, key string, expiration time.Duration) error
	LTRIM(ctx context.Context, key string, start, stop int64) error
	Incr(ctx context.Context, key string) (int64, error)
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

// RedisGoClientAdapter adapts go-redis/v9 Client to the RedisClient interface.
type RedisGoClientAdapter struct {
	Client *redis.Client
}

// Extend RedisGoClientAdapter to implement new methods
func (a *RedisGoClientAdapter) LRANGE(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return a.Client.LRange(ctx, key, start, stop).Result()
}
func (a *RedisGoClientAdapter) LLEN(ctx context.Context, key string) (int64, error) {
	return a.Client.LLen(ctx, key).Result()
}
func (a *RedisGoClientAdapter) EXPIRE(ctx context.Context, key string, expiration time.Duration) error {
	return a.Client.Expire(ctx, key, expiration).Err()
}
func (a *RedisGoClientAdapter) LTRIM(ctx context.Context, key string, start, stop int64) error {
	return a.Client.LTrim(ctx, key, start, stop).Err()
}
func (a *RedisGoClientAdapter) LPush(ctx context.Context, key string, values ...interface{}) error {
	return a.Client.LPush(ctx, key, values...).Err()
}
func (a *RedisGoClientAdapter) Incr(ctx context.Context, key string) (int64, error) {
	return a.Client.Incr(ctx, key).Result()
}
func (a *RedisGoClientAdapter) Get(ctx context.Context, key string) (string, error) {
	return a.Client.Get(ctx, key).Result()
}
func (a *RedisGoClientAdapter) Set(ctx context.Context, key, value string) error {
	return a.Client.Set(ctx, key, value, 0).Err()
}

// Refactor RedisEventBus: remove BRPOP/loop, add non-destructive read
// Remove NewRedisEventBusSubscriber and loop()
// Add ReadEvents and SetTTL methods

// NewRedisEventBusLog creates a Redis event bus that acts as a persistent log (non-destructive, with TTL and optional max length)
func NewRedisEventBusLog(client RedisClient, key string, ttl time.Duration, maxLen int64) *RedisEventBus {
	return &RedisEventBus{
		client: client,
		key:    key,
		logTTL: ttl,
		maxLen: maxLen,
	}
}

// RedisEventBus is a Redis-backed EventBus implementation. Events are encoded as
// JSON and pushed to a Redis list. This version is a persistent log: events are never removed by consumers.
type RedisEventBus struct {
	client RedisClient
	key    string

	stats  busStats
	logTTL time.Duration // TTL for the Redis list
	maxLen int64         // Max length for the Redis list
}

// NewRedisEventBusPublisher creates a Redis event bus that only publishes events (no background consumer).
func NewRedisEventBusPublisher(client RedisClient, key string) *RedisEventBus {
	return &RedisEventBus{
		client: client,
		key:    key,
	}
}

// Publish pushes the event JSON to the Redis list.
func (b *RedisEventBus) Publish(ctx context.Context, evt Event) {
	// Assign a monotonic LogID
	seq, err := b.client.Incr(ctx, b.key+":seq")
	if err != nil {
		log.Printf("[eventbus] Failed to increment event log sequence: %v", err)
		b.stats.dropped.Add(1)
		return
	}
	evt.LogID = seq
	data, err := json.Marshal(evt)
	if err != nil {
		log.Printf("[eventbus] Failed to marshal event: %v", err)
		return
	}
	if err := b.client.LPush(ctx, b.key, data); err != nil {
		log.Printf("[eventbus] Failed to publish event to Redis key %s: %v", b.key, err)
		b.stats.dropped.Add(1)
		return
	}
	if b.maxLen > 0 {
		_ = b.client.LTRIM(ctx, b.key, 0, b.maxLen-1)
	}
	if b.logTTL > 0 {
		_ = b.client.EXPIRE(ctx, b.key, b.logTTL)
	}
	b.stats.published.Add(1)
}

// ReadEvents returns events in [start, end] (inclusive, like LRANGE)
func (b *RedisEventBus) ReadEvents(ctx context.Context, start, end int64) ([]Event, error) {
	items, err := b.client.LRANGE(ctx, b.key, start, end)
	if err != nil {
		return nil, err
	}
	var events []Event
	for _, item := range items {
		var evt Event
		if err := json.Unmarshal([]byte(item), &evt); err == nil {
			events = append(events, evt)
		}
	}
	return events, nil
}

// EventCount returns the current number of events in the log
func (b *RedisEventBus) EventCount(ctx context.Context) (int64, error) {
	return b.client.LLEN(ctx, b.key)
}

// Stop is a no-op for the log-based RedisEventBus (required to satisfy EventBus interface)
func (b *RedisEventBus) Stop() {}

// Subscribe is not supported for the log-based RedisEventBus. It returns a closed channel.
func (b *RedisEventBus) Subscribe() <-chan Event {
	ch := make(chan Event)
	close(ch)
	return ch
}

// Client returns the underlying RedisClient for this RedisEventBus
func (b *RedisEventBus) Client() RedisClient {
	return b.client
}

// MockRedisClientLog implements RedisClient for log pattern (exported for use in other packages)
type MockRedisClientLog struct {
	list  [][]byte
	ttl   time.Duration
	seq   map[string]int64  // for Incr
	store map[string]string // for Get/Set
}

// NewMockRedisClientLog creates a new mock Redis client for log-based event bus (exported)
func NewMockRedisClientLog() *MockRedisClientLog {
	return &MockRedisClientLog{list: make([][]byte, 0), seq: make(map[string]int64), store: make(map[string]string)}
}

func (m *MockRedisClientLog) LPush(ctx context.Context, key string, values ...interface{}) error {
	for _, v := range values {
		var b []byte
		switch val := v.(type) {
		case string:
			b = []byte(val)
		case []byte:
			b = val
		}
		m.list = append([][]byte{b}, m.list...)
	}
	return nil
}

func (m *MockRedisClientLog) LRANGE(ctx context.Context, key string, start, stop int64) ([]string, error) {
	ln := int64(len(m.list))
	if ln == 0 {
		return []string{}, nil
	}
	// Handle negative indices
	if start < 0 {
		start = ln + start
	}
	if stop < 0 {
		stop = ln + stop
	}
	if start < 0 {
		start = 0
	}
	if stop >= ln {
		stop = ln - 1
	}
	if start > stop || start >= ln {
		return []string{}, nil
	}
	out := make([]string, 0, stop-start+1)
	for i := start; i <= stop; i++ {
		out = append(out, string(m.list[i]))
	}
	return out, nil
}

func (m *MockRedisClientLog) LLEN(ctx context.Context, key string) (int64, error) {
	return int64(len(m.list)), nil
}

func (m *MockRedisClientLog) EXPIRE(ctx context.Context, key string, expiration time.Duration) error {
	m.ttl = expiration
	return nil
}

func (m *MockRedisClientLog) LTRIM(ctx context.Context, key string, start, stop int64) error {
	ln := int64(len(m.list))
	if start < 0 {
		start = ln + start
	}
	if stop < 0 {
		stop = ln + stop
	}
	if start < 0 {
		start = 0
	}
	if stop >= ln {
		stop = ln - 1
	}
	if start > stop || start >= ln {
		m.list = [][]byte{}
		return nil
	}
	m.list = m.list[start : stop+1]
	return nil
}

func (m *MockRedisClientLog) Incr(ctx context.Context, key string) (int64, error) {
	m.seq[key]++
	return m.seq[key], nil
}

func (m *MockRedisClientLog) Get(ctx context.Context, key string) (string, error) {
	return m.store[key], nil
}

func (m *MockRedisClientLog) Set(ctx context.Context, key, value string) error {
	m.store[key] = value
	return nil
}
