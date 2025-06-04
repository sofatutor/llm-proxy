package eventbus

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestNewInMemoryEventBus(t *testing.T) {
	tests := []struct {
		name       string
		bufferSize int
	}{
		{
			name:       "small buffer",
			bufferSize: 1,
		},
		{
			name:       "medium buffer",
			bufferSize: 10,
		},
		{
			name:       "large buffer",
			bufferSize: 1000,
		},
		{
			name:       "zero buffer",
			bufferSize: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bus := NewInMemoryEventBus(tt.bufferSize)
			if bus == nil {
				t.Error("NewInMemoryEventBus() returned nil")
			}
			if bus.ch == nil {
				t.Error("NewInMemoryEventBus() did not initialize channel")
			}
			if cap(bus.ch) != tt.bufferSize {
				t.Errorf("NewInMemoryEventBus() channel capacity = %d, want %d", cap(bus.ch), tt.bufferSize)
			}
		})
	}
}

func TestInMemoryEventBus_Publish(t *testing.T) {
	tests := []struct {
		name        string
		bufferSize  int
		events      []Event
		wantDropped bool
	}{
		{
			name:       "single event within buffer",
			bufferSize: 2,
			events: []Event{
				{RequestID: "req1", Method: "GET", Path: "/test", Status: 200},
			},
			wantDropped: false,
		},
		{
			name:       "multiple events within buffer",
			bufferSize: 3,
			events: []Event{
				{RequestID: "req1", Method: "GET", Path: "/test1", Status: 200},
				{RequestID: "req2", Method: "POST", Path: "/test2", Status: 201},
			},
			wantDropped: false,
		},
		{
			name:       "events exceed buffer - should drop",
			bufferSize: 1,
			events: []Event{
				{RequestID: "req1", Method: "GET", Path: "/test1", Status: 200},
				{RequestID: "req2", Method: "POST", Path: "/test2", Status: 201}, // This should be dropped
			},
			wantDropped: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bus := NewInMemoryEventBus(tt.bufferSize)
			ctx := context.Background()

			// Publish all events
			for _, event := range tt.events {
				bus.Publish(ctx, event)
			}

			// Check how many events we can receive
			receivedCount := 0
			for {
				select {
				case <-bus.Subscribe():
					receivedCount++
				default:
					// No more events available
					goto done
				}
			}
		done:

			expectedReceived := len(tt.events)
			if tt.wantDropped {
				expectedReceived = tt.bufferSize
			}

			if receivedCount != expectedReceived {
				t.Errorf("received %d events, want %d", receivedCount, expectedReceived)
			}
		})
	}
}

func TestInMemoryEventBus_Subscribe(t *testing.T) {
	t.Run("subscribe returns channel", func(t *testing.T) {
		bus := NewInMemoryEventBus(5)
		ch := bus.Subscribe()
		if ch == nil {
			t.Error("Subscribe() returned nil channel")
		}
	})

	t.Run("subscribe initializes channel if nil", func(t *testing.T) {
		bus := &InMemoryEventBus{} // Create with nil channel
		ch := bus.Subscribe()
		if ch == nil {
			t.Error("Subscribe() should initialize channel if nil")
		}
		if cap(ch) != 1 {
			t.Errorf("Subscribe() initialized channel with capacity %d, want 1", cap(ch))
		}
	})

	t.Run("multiple subscribers get same channel", func(t *testing.T) {
		bus := NewInMemoryEventBus(5)
		ch1 := bus.Subscribe()
		ch2 := bus.Subscribe()
		if ch1 != ch2 {
			t.Error("Multiple calls to Subscribe() should return the same channel")
		}
	})
}

func TestInMemoryEventBus_PublishAndSubscribe(t *testing.T) {
	t.Run("published events can be received", func(t *testing.T) {
		bus := NewInMemoryEventBus(10)
		ctx := context.Background()

		// Create test event
		testEvent := Event{
			RequestID: "test-123",
			Method:    "POST",
			Path:      "/api/test",
			Status:    201,
			Duration:  100 * time.Millisecond,
			ResponseHeaders: http.Header{
				"Content-Type": []string{"application/json"},
			},
			ResponseBody: []byte(`{"success": true}`),
		}

		// Subscribe before publishing
		ch := bus.Subscribe()

		// Publish event
		bus.Publish(ctx, testEvent)

		// Receive event
		select {
		case received := <-ch:
			if received.RequestID != testEvent.RequestID {
				t.Errorf("received RequestID %s, want %s", received.RequestID, testEvent.RequestID)
			}
			if received.Method != testEvent.Method {
				t.Errorf("received Method %s, want %s", received.Method, testEvent.Method)
			}
			if received.Path != testEvent.Path {
				t.Errorf("received Path %s, want %s", received.Path, testEvent.Path)
			}
			if received.Status != testEvent.Status {
				t.Errorf("received Status %d, want %d", received.Status, testEvent.Status)
			}
			if received.Duration != testEvent.Duration {
				t.Errorf("received Duration %v, want %v", received.Duration, testEvent.Duration)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout waiting for event")
		}
	})

	t.Run("context cancellation doesn't affect publish", func(t *testing.T) {
		bus := NewInMemoryEventBus(5)
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		testEvent := Event{RequestID: "test-cancelled", Method: "GET", Path: "/test"}

		// This should not panic or block even with cancelled context
		bus.Publish(ctx, testEvent)

		// Event should still be published
		ch := bus.Subscribe()
		select {
		case received := <-ch:
			if received.RequestID != testEvent.RequestID {
				t.Errorf("received RequestID %s, want %s", received.RequestID, testEvent.RequestID)
			}
		case <-time.After(50 * time.Millisecond):
			t.Error("event was not published despite cancelled context")
		}
	})
}

func TestEvent(t *testing.T) {
	t.Run("event struct fields", func(t *testing.T) {
		headers := http.Header{
			"Content-Type":   []string{"application/json"},
			"Content-Length": []string{"123"},
		}
		body := []byte(`{"test": "data"}`)
		duration := 250 * time.Millisecond

		event := Event{
			RequestID:       "req-456",
			Method:          "PUT",
			Path:            "/api/update",
			Status:          200,
			Duration:        duration,
			ResponseHeaders: headers,
			ResponseBody:    body,
		}

		if event.RequestID != "req-456" {
			t.Errorf("RequestID = %s, want %s", event.RequestID, "req-456")
		}
		if event.Method != "PUT" {
			t.Errorf("Method = %s, want %s", event.Method, "PUT")
		}
		if event.Path != "/api/update" {
			t.Errorf("Path = %s, want %s", event.Path, "/api/update")
		}
		if event.Status != 200 {
			t.Errorf("Status = %d, want %d", event.Status, 200)
		}
		if event.Duration != duration {
			t.Errorf("Duration = %v, want %v", event.Duration, duration)
		}
		if event.ResponseHeaders.Get("Content-Type") != "application/json" {
			t.Error("ResponseHeaders not set correctly")
		}
		if string(event.ResponseBody) != `{"test": "data"}` {
			t.Error("ResponseBody not set correctly")
		}
	})
}

func TestInMemoryEventBus_ConcurrentAccess(t *testing.T) {
	t.Run("concurrent publish and subscribe", func(t *testing.T) {
		bus := NewInMemoryEventBus(100)
		ctx := context.Background()

		// Start publishing events in a goroutine
		go func() {
			for i := 0; i < 50; i++ {
				event := Event{
					RequestID: "concurrent-test",
					Method:    "GET",
					Path:      "/test",
					Status:    200,
				}
				bus.Publish(ctx, event)
			}
		}()

		// Subscribe and count events
		ch := bus.Subscribe()
		receivedCount := 0
		timeout := time.After(1 * time.Second)

		for receivedCount < 50 {
			select {
			case <-ch:
				receivedCount++
			case <-timeout:
				t.Errorf("timeout: only received %d events out of 50", receivedCount)
				return
			}
		}

		if receivedCount != 50 {
			t.Errorf("received %d events, want 50", receivedCount)
		}
	})
}
