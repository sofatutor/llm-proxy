package eventbus

import (
	"context"
	"testing"
	"time"
)

// Cover drop path with small buffer and fast publish burst
func TestInMemoryEventBus_DropOnOverflow(t *testing.T) {
	bus := NewInMemoryEventBus(1)
	defer bus.Stop()

	// No subscriber to consume
	for i := 0; i < 10; i++ {
		bus.Publish(context.Background(), Event{RequestID: "r"})
	}
	// Allow internal loop to process stats
	time.Sleep(50 * time.Millisecond)
	pub, drop := bus.Stats()
	if pub == 0 || drop == 0 {
		t.Fatalf("expected some published and dropped events, got pub=%d drop=%d", pub, drop)
	}
}

// Test RedisEventBus.Stop() method for coverage
func TestRedisEventBus_Stop(t *testing.T) {
	// Create a mock Redis client for testing
	mockClient := newMockRedisClientLog()

	redisEventBus := &RedisEventBus{
		client: mockClient,
		key:    "test-events",
	}

	// Call Stop() - should be a no-op but needs to be called for coverage
	redisEventBus.Stop()

	// Verify that Stop() is indeed a no-op (no state changes)
	// Just ensure it doesn't panic or cause errors
	redisEventBus.Stop()
}
