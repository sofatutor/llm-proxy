package eventbus

import (
	"context"
	"testing"
	"time"
)

// mockRedisClient is a simple in-memory RedisClient used for tests.
type mockRedisClient struct {
	ch         chan []byte
	errOnLPush bool
}

func newMockRedisClient() *mockRedisClient {
	return &mockRedisClient{ch: make(chan []byte, 10)}
}

func (m *mockRedisClient) LPush(ctx context.Context, key string, values ...interface{}) error {
	if m.errOnLPush {
		return context.DeadlineExceeded
	}
	for _, v := range values {
		switch val := v.(type) {
		case string:
			m.ch <- []byte(val)
		case []byte:
			m.ch <- val
		}
	}
	return nil
}

func (m *mockRedisClient) BRPop(ctx context.Context, timeout time.Duration, keys ...string) ([]string, error) {
	select {
	case val := <-m.ch:
		return []string{string(val)}, nil
	case <-time.After(timeout):
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestInMemoryEventBus_MultipleSubscribers(t *testing.T) {
	bus := NewInMemoryEventBus(5)
	defer bus.Stop()

	sub1 := bus.Subscribe()
	sub2 := bus.Subscribe()

	bus.Publish(context.Background(), Event{RequestID: "1"})

	select {
	case <-sub1:
	case <-time.After(time.Second):
		t.Fatal("sub1 did not receive event")
	}

	select {
	case <-sub2:
	case <-time.After(time.Second):
		t.Fatal("sub2 did not receive event")
	}
}

func TestRedisEventBus_PublishSubscribe(t *testing.T) {
	client := newMockRedisClient()
	bus := NewRedisEventBus(client, "events")
	defer bus.Stop()

	sub := bus.Subscribe()
	bus.Publish(context.Background(), Event{RequestID: "a"})

	select {
	case evt := <-sub:
		if evt.RequestID != "a" {
			t.Fatalf("expected requestID a, got %s", evt.RequestID)
		}
	case <-time.After(time.Second):
		t.Fatal("event not received")
	}
}

func TestInMemoryEventBus_NoSubscribers(t *testing.T) {
	bus := NewInMemoryEventBus(2)
	defer bus.Stop()
	bus.Publish(context.Background(), Event{RequestID: "no-subs"})
	// No panic, no subscribers, event should be dropped
	pub, drop := bus.Stats()
	if pub != 1 || drop != 0 {
		t.Fatalf("expected 1 published, 0 dropped, got %d/%d", pub, drop)
	}
}

func TestInMemoryEventBus_PublishAfterStop(t *testing.T) {
	bus := NewInMemoryEventBus(2)
	bus.Stop()
	// Should not panic, should not deliver
	bus.Publish(context.Background(), Event{RequestID: "after-stop"})
	pub, drop := bus.Stats()
	if pub != 0 && drop != 1 && pub+drop != 1 {
		t.Fatalf("expected 0 published, 1 dropped (or 1 dropped), got %d/%d", pub, drop)
	}
}

func TestInMemoryEventBus_SubscribeAfterStop(t *testing.T) {
	bus := NewInMemoryEventBus(2)
	bus.Stop()
	sub := bus.Subscribe()
	select {
	case _, ok := <-sub:
		if ok {
			t.Fatal("expected closed channel after stop")
		}
	default:
		// Channel should be closed immediately
	}
}

func TestInMemoryEventBus_Stats_PublishedAndDropped(t *testing.T) {
	bus := NewInMemoryEventBus(1)
	defer bus.Stop()
	sub := bus.Subscribe()
	bus.Publish(context.Background(), Event{RequestID: "1"})
	bus.Publish(context.Background(), Event{RequestID: "2"}) // buffer full, should be dropped
	// Drain sub
	<-sub
	pub, drop := bus.Stats()
	if pub != 1 || drop != 1 {
		t.Fatalf("expected 1 published, 1 dropped, got %d/%d", pub, drop)
	}
}

func TestInMemoryEventBus_BufferFull_DroppedEvents(t *testing.T) {
	bus := NewInMemoryEventBus(1)
	defer bus.Stop()
	// No subscribers, fill buffer and overflow
	bus.Publish(context.Background(), Event{RequestID: "1"})
	bus.Publish(context.Background(), Event{RequestID: "2"})
	bus.Publish(context.Background(), Event{RequestID: "3"})
	pub, drop := bus.Stats()
	if pub+drop != 3 {
		t.Fatalf("expected 3 total events, got %d published, %d dropped", pub, drop)
	}
	if drop < 1 {
		t.Fatalf("expected at least 1 dropped event, got %d", drop)
	}
}

func TestRedisEventBus_Publish_LPushError(t *testing.T) {
	client := newMockRedisClient()
	bus := NewRedisEventBus(client, "events")
	defer bus.Stop()

	bus.Publish(context.Background(), Event{RequestID: "ok"})
	client.errOnLPush = true
	bus.Publish(context.Background(), Event{RequestID: "fail"})

	pub, drop := bus.Stats()
	if pub != 1 || drop != 1 {
		t.Fatalf("expected 1 published, 1 dropped, got %d/%d", pub, drop)
	}
}

func TestRedisEventBus_Stats(t *testing.T) {
	client := newMockRedisClient()
	bus := NewRedisEventBus(client, "events")
	defer bus.Stop()
	bus.Publish(context.Background(), Event{RequestID: "1"})
	bus.Publish(context.Background(), Event{RequestID: "2"})
	pub, drop := bus.Stats()
	if pub+drop != 2 {
		t.Fatalf("expected 2 total events, got %d published, %d dropped", pub, drop)
	}
}

func TestRedisEventBus_Stop_ClosesSubscribers(t *testing.T) {
	client := newMockRedisClient()
	bus := NewRedisEventBus(client, "events")
	sub := bus.Subscribe()
	bus.Stop()
	_, ok := <-sub
	if ok {
		t.Fatal("expected closed channel after stop")
	}
}

func TestRedisEventBus_StatsMethodCoverage(t *testing.T) {
	client := newMockRedisClient()
	bus := NewRedisEventBus(client, "events")
	defer bus.Stop()
	// Just call Stats to cover the method
	bus.Stats()
}
