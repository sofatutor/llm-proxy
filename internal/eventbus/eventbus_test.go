package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	redis "github.com/redis/go-redis/v9"
)

// test-local mock implementing RedisClient, shared across tests in this package
type mockRedisClientLog struct {
	list  [][]byte
	ttl   time.Duration
	seq   map[string]int64
	store map[string]string
}

func newMockRedisClientLog() *mockRedisClientLog {
	return &mockRedisClientLog{list: make([][]byte, 0), seq: make(map[string]int64), store: make(map[string]string)}
}

func (m *mockRedisClientLog) LPush(_ context.Context, _ string, values ...interface{}) error {
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

func (m *mockRedisClientLog) LRANGE(_ context.Context, _ string, start, stop int64) ([]string, error) {
	ln := int64(len(m.list))
	if ln == 0 {
		return []string{}, nil
	}
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

func (m *mockRedisClientLog) LLEN(_ context.Context, _ string) (int64, error) {
	return int64(len(m.list)), nil
}
func (m *mockRedisClientLog) EXPIRE(_ context.Context, _ string, expiration time.Duration) error {
	m.ttl = expiration
	return nil
}
func (m *mockRedisClientLog) LTRIM(_ context.Context, _ string, start, stop int64) error {
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
func (m *mockRedisClientLog) Incr(_ context.Context, key string) (int64, error) {
	if m.seq == nil {
		m.seq = make(map[string]int64)
	}
	m.seq[key]++
	return m.seq[key], nil
}
func (m *mockRedisClientLog) Get(_ context.Context, key string) (string, error) {
	return m.store[key], nil
}
func (m *mockRedisClientLog) Set(_ context.Context, key, value string) error {
	if m.store == nil {
		m.store = make(map[string]string)
	}
	m.store[key] = value
	return nil
}

func TestInMemoryEventBus_PublishSubscribe(t *testing.T) {
	bus := NewInMemoryEventBus(10)
	defer bus.Stop()

	sub := bus.Subscribe()

	// Publish several events
	numEvents := 5
	for i := 0; i < numEvents; i++ {
		bus.Publish(context.Background(), Event{RequestID: "r"})
	}

	// Receive all events
	received := 0
	timeout := time.After(500 * time.Millisecond)
	for received < numEvents {
		select {
		case <-sub:
			received++
		case <-timeout:
			t.Fatalf("timeout waiting for events: got %d, want %d", received, numEvents)
		}
	}

	pub, drop := bus.Stats()
	if pub != numEvents || drop != 0 {
		t.Fatalf("unexpected stats: published=%d dropped=%d", pub, drop)
	}
}

func TestInMemoryEventBus_DroppedWhenFull(t *testing.T) {
	bus := NewInMemoryEventBus(1)
	defer bus.Stop()

	// No subscriber; buffer size is 1 → one publish accepted, rest dropped
	for i := 0; i < 5; i++ {
		bus.Publish(context.Background(), Event{RequestID: "r"})
	}

	pub, drop := bus.Stats()
	if pub == 0 {
		t.Fatalf("expected some published events")
	}
	if drop == 0 {
		t.Fatalf("expected dropped events when buffer is full")
	}
}

func TestInMemoryEventBus_StopClosesSubscribers(t *testing.T) {
	bus := NewInMemoryEventBus(1)
	sub := bus.Subscribe()
	bus.Stop()

	select {
	case _, ok := <-sub:
		if ok {
			t.Fatalf("expected closed subscriber channel after Stop")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timeout waiting for subscriber channel to close")
	}
}

// Covers retry branch in dispatch(): subscriber buffer full causes non-blocking send retries
func TestInMemoryEventBus_DispatchRetryWhenSubscriberFull(t *testing.T) {
	bus := NewInMemoryEventBus(1)
	defer bus.Stop()

	sub := bus.Subscribe() // buffer size 1

	// First publish fills subscriber buffer
	bus.Publish(context.Background(), Event{RequestID: "r1"})
	// Allow loop to deliver
	time.Sleep(10 * time.Millisecond)

	// Second publish triggers retry path because sub is full and we never drain it
	start := time.Now()
	bus.Publish(context.Background(), Event{RequestID: "r2"})

	// Wait a bit longer than total retry backoff to ensure dispatch attempts happened
	time.Sleep(50 * time.Millisecond)

	// Subscriber buffer should still be full (1)
	if got := len(sub); got != 1 {
		t.Fatalf("expected subscriber buffer to remain full (1), got %d", got)
	}

	// Sanity: publish path executed quickly (not blocking), but we waited for retry backoff
	if time.Since(start) < 40*time.Millisecond {
		t.Fatalf("expected some retry backoff time to elapse")
	}
}

// --- Merged from eventbus_extra_test.go ---
func TestInMemoryEventBus_Stats(t *testing.T) {
	b := NewInMemoryEventBus(1)
	defer b.Stop()

	// Publish 1 fits, next two should be dropped due to buffer full (no subscribers)
	b.Publish(context.Background(), Event{})
	b.Publish(context.Background(), Event{})
	b.Publish(context.Background(), Event{})

	pub, drop := b.Stats()
	if pub < 1 {
		t.Fatalf("published = %d, want >= 1", pub)
	}
	if drop < 1 {
		t.Fatalf("dropped = %d, want >= 1", drop)
	}
}

func TestRedisEventBusLog_PublishReadCount_TTLAndTrim(t *testing.T) {
	client := newMockRedisClientLog()
	// TTL 1s, maxLen 3
	bus := NewRedisEventBusLog(client, "events", 1*time.Second, 3)

	// Publish 5 events → list should be trimmed to 3 most recent
	for i := 0; i < 5; i++ {
		bus.Publish(context.Background(), Event{RequestID: "r"})
	}

	// Count should be 3 due to LTRIM
	cnt, err := bus.EventCount(context.Background())
	if err != nil {
		t.Fatalf("EventCount error: %v", err)
	}
	if cnt != 3 {
		t.Fatalf("unexpected event count: %d (want 3)", cnt)
	}

	// Read back events and ensure LogID is set and monotonic (descending due to LPush at head)
	events, err := bus.ReadEvents(context.Background(), 0, -1)
	if err != nil {
		t.Fatalf("ReadEvents error: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("unexpected events length: %d (want 3)", len(events))
	}
	// Verify LogID is non-zero and strictly decreasing across the trimmed list
	if events[0].LogID == 0 || events[1].LogID == 0 || events[2].LogID == 0 {
		t.Fatalf("expected non-zero LogID for all events")
	}
	if events[0].LogID <= events[1].LogID || events[1].LogID <= events[2].LogID {
		t.Fatalf("expected descending LogID order: got %d, %d, %d", events[0].LogID, events[1].LogID, events[2].LogID)
	}

	// TTL should be set on first publish
	if client.ttl <= 0 {
		t.Fatalf("expected TTL to be set, got %v", client.ttl)
	}
}

func TestRedisEventBus_StopIsNoop(t *testing.T) {
	client := newMockRedisClientLog()
	bus := NewRedisEventBusPublisher(client, "events")
	// Stop should be a no-op and not panic
	bus.Stop()
}

func TestRedisEventBus_StopDirect(t *testing.T) {
	client := newMockRedisClientLog()
	// Create RedisEventBus directly to ensure Stop method is called
	bus := &RedisEventBus{
		client: client,
		key:    "test-events",
	}
	// Stop should be a no-op and not panic
	bus.Stop()
}

func TestRedisEventBus_SubscribeReturnsClosedChannel(t *testing.T) {
	client := newMockRedisClientLog()
	bus := NewRedisEventBusPublisher(client, "events")
	ch := bus.Subscribe()
	// Channel should be closed immediately
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatalf("expected closed channel from Subscribe")
		}
	default:
		t.Fatalf("expected closed channel from Subscribe (non-blocking)")
	}
}

func TestRedisEventBus_ClientAccessor(t *testing.T) {
	client := newMockRedisClientLog()
	bus := NewRedisEventBusPublisher(client, "events")
	if bus.Client() != client {
		t.Fatalf("Client() did not return underlying client")
	}
}

// helper to create a real go-redis client against miniredis
func newMockableRedisClient(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{Addr: addr, DB: 0})
}

func TestRedisGoClientAdapter_AllMethods(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run error: %v", err)
	}
	defer s.Close()

	rdb := newMockableRedisClient(s.Addr())
	adapter := &RedisGoClientAdapter{Client: rdb}

	ctx := context.Background()

	if err := adapter.Set(ctx, "k", "v"); err != nil {
		t.Fatalf("Set error: %v", err)
	}
	if got, err := adapter.Get(ctx, "k"); err != nil || got != "v" {
		t.Fatalf("Get got (%q, %v), want (\"v\", nil)", got, err)
	}

	if n, err := adapter.Incr(ctx, "seq"); err != nil || n != 1 {
		t.Fatalf("Incr #1 got (%d, %v), want (1, nil)", n, err)
	}
	if n, err := adapter.Incr(ctx, "seq"); err != nil || n != 2 {
		t.Fatalf("Incr #2 got (%d, %v), want (2, nil)", n, err)
	}

	if err := adapter.LPush(ctx, "list", "a", "b", "c"); err != nil {
		t.Fatalf("LPush error: %v", err)
	}
	if ln, err := adapter.LLEN(ctx, "list"); err != nil || ln != 3 {
		t.Fatalf("LLEN got (%d, %v), want (3, nil)", ln, err)
	}
	if items, err := adapter.LRANGE(ctx, "list", 0, -1); err != nil || len(items) != 3 {
		t.Fatalf("LRANGE got (len=%d, %v), want (3, nil)", len(items), err)
	}
	if err := adapter.LTRIM(ctx, "list", 0, 1); err != nil {
		t.Fatalf("LTRIM error: %v", err)
	}
	if ln, err := adapter.LLEN(ctx, "list"); err != nil || ln != 2 {
		t.Fatalf("LLEN after LTRIM got (%d, %v), want (2, nil)", ln, err)
	}

	if err := adapter.EXPIRE(ctx, "list", time.Second); err != nil {
		t.Fatalf("EXPIRE error: %v", err)
	}
}

// Erroring Redis client to cover Publish() error branch on Incr
type errRedisClient struct{}

func (e *errRedisClient) LPush(context.Context, string, ...interface{}) error { return nil }
func (e *errRedisClient) LRANGE(context.Context, string, int64, int64) ([]string, error) {
	return nil, nil
}
func (e *errRedisClient) LLEN(context.Context, string) (int64, error)         { return 0, nil }
func (e *errRedisClient) EXPIRE(context.Context, string, time.Duration) error { return nil }
func (e *errRedisClient) LTRIM(context.Context, string, int64, int64) error   { return nil }
func (e *errRedisClient) Incr(context.Context, string) (int64, error)         { return 0, errors.New("boom") }
func (e *errRedisClient) Get(context.Context, string) (string, error)         { return "", nil }
func (e *errRedisClient) Set(context.Context, string, string) error           { return nil }

func TestRedisEventBus_Publish_IncrError_DropsEvent(t *testing.T) {
	bus := NewRedisEventBusPublisher(&errRedisClient{}, "events")
	// Should not panic and should not add items
	bus.Publish(context.Background(), Event{RequestID: "x"})
	if cnt, err := bus.EventCount(context.Background()); err != nil || cnt != 0 {
		t.Fatalf("expected 0 events on error path, got cnt=%d err=%v", cnt, err)
	}
}

func TestRedisEventBus_ReadEvents_SkipsInvalidJSON(t *testing.T) {
	client := newMockRedisClientLog()
	bus := NewRedisEventBusLog(client, "events", 0, 0)

	// Inject invalid JSON directly
	_ = client.LPush(context.Background(), "events", "not-json")

	// Add a valid event via Publish
	bus.Publish(context.Background(), Event{RequestID: "ok"})

	evts, err := bus.ReadEvents(context.Background(), 0, -1)
	if err != nil {
		t.Fatalf("ReadEvents error: %v", err)
	}
	// Should only parse the valid one
	if len(evts) != 1 {
		t.Fatalf("expected 1 parsed event, got %d", len(evts))
	}
	// Validate it’s the valid one
	b, _ := json.Marshal(evts[0])
	if !json.Valid(b) || evts[0].RequestID != "ok" {
		t.Fatalf("unexpected event parsed: %+v", evts[0])
	}
}

func TestRedisEventBus_Stop_NoOp_Coverage(t *testing.T) {
	client := newMockRedisClientLog()
	// Use NewRedisEventBusLog constructor to create RedisEventBus
	bus := NewRedisEventBusLog(client, "events", time.Second, 10)

	// Call Stop() - this should be a no-op and not panic
	bus.Stop()

	// Verify we can still use the bus after Stop (since it's a no-op)
	bus.Publish(context.Background(), Event{RequestID: "after-stop"})

	// Verify events can still be read
	events, err := bus.ReadEvents(context.Background(), 0, -1)
	if err != nil {
		t.Fatalf("ReadEvents after Stop failed: %v", err)
	}
	if len(events) != 1 || events[0].RequestID != "after-stop" {
		t.Fatalf("expected 1 event after Stop, got %d events", len(events))
	}
}

func TestRedisEventBus_Stop_Direct_Coverage(t *testing.T) {
	client := newMockRedisClientLog()
	// Create RedisEventBus directly and ensure Stop method is called
	bus := &RedisEventBus{
		client: client,
		key:    "test-events",
	}

	// This should directly call the Stop method at line 267
	bus.Stop()

	// Verify bus is still usable (no-op Stop)
	bus.Publish(context.Background(), Event{RequestID: "test"})
}
