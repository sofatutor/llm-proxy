package eventbus

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestInMemoryEventBus(t *testing.T) {
	bus := NewInMemoryEventBus(5)

	// Test publishing and subscribing
	event := Event{
		RequestID: "test-123",
		Method:    "POST",
		Path:      "/v1/test",
		Status:    200,
		Duration:  time.Millisecond * 100,
	}

	// Subscribe before publishing
	eventCh := bus.Subscribe()

	// Publish event
	ctx := context.Background()
	bus.Publish(ctx, event)

	// Receive event
	select {
	case received := <-eventCh:
		assert.Equal(t, event.RequestID, received.RequestID)
		assert.Equal(t, event.Method, received.Method)
		assert.Equal(t, event.Path, received.Path)
		assert.Equal(t, event.Status, received.Status)
	case <-time.After(time.Second):
		t.Fatal("Event not received within timeout")
	}
}

func TestInMemoryEventBus_BufferFull(t *testing.T) {
	// Create small buffer
	bus := NewInMemoryEventBus(2)
	eventCh := bus.Subscribe()

	ctx := context.Background()

	// Fill the buffer
	for i := 0; i < 3; i++ {
		event := Event{RequestID: fmt.Sprintf("test-%d", i)}
		bus.Publish(ctx, event)
	}

	// Should receive first 2 events, third should be dropped
	received := 0
	for i := 0; i < 2; i++ {
		select {
		case <-eventCh:
			received++
		case <-time.After(100 * time.Millisecond):
			break
		}
	}

	assert.Equal(t, 2, received)
}

func TestRedisEventBus_Integration(t *testing.T) {
	// Skip if Redis is not available
	redisURL := "redis://localhost:6379/0"
	bus, err := NewRedisEventBus(redisURL, "test-stream", "test-group")
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer bus.Close()

	event := Event{
		RequestID: "redis-test-123",
		Method:    "POST",
		Path:      "/v1/redis-test",
		Status:    200,
		Duration:  time.Millisecond * 150,
	}

	// Subscribe
	eventCh := bus.Subscribe()

	// Publish
	ctx := context.Background()
	bus.Publish(ctx, event)

	// Wait for event
	select {
	case received := <-eventCh:
		assert.Equal(t, event.RequestID, received.RequestID)
		assert.Equal(t, event.Method, received.Method)
		assert.Equal(t, event.Path, received.Path)
		assert.Equal(t, event.Status, received.Status)
	case <-time.After(5 * time.Second):
		t.Fatal("Event not received within timeout")
	}
}
