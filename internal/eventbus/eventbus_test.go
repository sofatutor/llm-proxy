package eventbus

import (
	"context"
	"testing"
	"time"
)

// mockRedisClient is a simple in-memory RedisClient used for tests.
type mockRedisClient struct {
	ch chan []byte
}

func newMockRedisClient() *mockRedisClient {
	return &mockRedisClient{ch: make(chan []byte, 10)}
}

func (m *mockRedisClient) LPush(ctx context.Context, key string, values ...interface{}) error {
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
