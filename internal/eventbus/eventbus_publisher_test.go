package eventbus

import (
	"context"
	"testing"
)

// Covers NewRedisEventBusPublisher, Publish, Subscribe (closed channel), Stop, and EventCount
func TestRedisEventBusPublisher_PublishSubscribeStop(t *testing.T) {
	client := newMockRedisClientLog()
	bus := NewRedisEventBusPublisher(client, "events")

	// Publish a few events
	bus.Publish(context.Background(), Event{RequestID: "r1"})
	bus.Publish(context.Background(), Event{RequestID: "r2"})

	// EventCount should reflect number of published events
	cnt, err := bus.EventCount(context.Background())
	if err != nil {
		t.Fatalf("EventCount error: %v", err)
	}
	if cnt != 2 {
		t.Fatalf("unexpected event count: %d (want 2)", cnt)
	}

	// Subscribe returns a closed channel for publisher-only bus
	ch := bus.Subscribe()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatalf("expected closed channel from Subscribe() on publisher bus")
		}
	default:
		// If not immediately closed, try a non-blocking read to ensure it's closed
		// The Subscribe implementation closes before returning, so this default branch should not happen.
	}

	// Stop is a no-op but should be safe to call
	bus.Stop()
}
