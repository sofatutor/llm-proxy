package dispatcher

import (
	"context"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/eventbus"
	"github.com/stretchr/testify/mock"
)

// MockPlugin is a mock implementation of the Plugin interface
type MockPlugin struct {
	mock.Mock
}

func (m *MockPlugin) Init(config map[string]string) error {
	args := m.Called(config)
	return args.Error(0)
}

func (m *MockPlugin) Send(ctx context.Context, event eventbus.Event) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockPlugin) Name() string {
	args := m.Called()
	return args.String(0)
}

func TestDispatcher_BasicFlow(t *testing.T) {
	// Create mock plugin
	mockPlugin := new(MockPlugin)
	mockPlugin.On("Name").Return("test-plugin")
	mockPlugin.On("Send", mock.Anything, mock.Anything).Return(nil)

	// Create event bus and dispatcher
	bus := eventbus.NewInMemoryEventBus(10)
	config := Config{
		BatchSize: 2,
		Workers:   1,
	}
	d := New(bus, mockPlugin, config)

	// Create test event
	event := eventbus.Event{
		RequestID: "test-dispatcher-123",
		Method:    "POST",
		Path:      "/v1/test",
		Status:    200,
		Duration:  time.Millisecond * 100,
	}

	// Start dispatcher in goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = d.Run(ctx) // Error is expected when context is cancelled
	}()

	// Give dispatcher time to start
	time.Sleep(100 * time.Millisecond)

	// Publish event
	bus.Publish(ctx, event)

	// Give time for processing
	time.Sleep(200 * time.Millisecond)

	// Stop dispatcher
	cancel()
	time.Sleep(100 * time.Millisecond)

	// Verify plugin was called
	mockPlugin.AssertCalled(t, "Send", mock.Anything, event)
}
