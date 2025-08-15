package dispatcher

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/eventbus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// mockPluginForCoverage implements BackendPlugin for testing error scenarios
type mockPluginForCoverage struct {
	sendEventsFunc func(ctx context.Context, events []EventPayload) error
	sendCallCount  int
}

func (m *mockPluginForCoverage) Init(cfg map[string]string) error {
	return nil
}

func (m *mockPluginForCoverage) SendEvents(ctx context.Context, events []EventPayload) error {
	m.sendCallCount++
	if m.sendEventsFunc != nil {
		return m.sendEventsFunc(ctx, events)
	}
	return nil
}

func (m *mockPluginForCoverage) Close() error {
	return nil
}

// Test sendBatchWithResult error scenarios to improve coverage
func TestService_sendBatchWithResult_ErrorScenarios(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tests := []struct {
		name           string
		retryAttempts  int
		sendEventsFunc func(ctx context.Context, events []EventPayload) error
		expectError    bool
		expectDropped  int64
		expectSent     int64
	}{
		{
			name:          "permanent_backend_error",
			retryAttempts: 2,
			sendEventsFunc: func(ctx context.Context, events []EventPayload) error {
				return &PermanentBackendError{Msg: "permanent error"}
			},
			expectError:   false, // should not return error for permanent errors
			expectDropped: 2,     // events should be dropped
			expectSent:    0,
		},
		{
			name:          "retry_until_success",
			retryAttempts: 3,
			sendEventsFunc: func() func(context.Context, []EventPayload) error {
				callCount := 0
				return func(ctx context.Context, events []EventPayload) error {
					callCount++
					if callCount <= 2 {
						return fmt.Errorf("temporary error %d", callCount)
					}
					return nil // succeed on third attempt
				}
			}(),
			expectError:   false,
			expectDropped: 0,
			expectSent:    2,
		},
		{
			name:          "context_cancelled_during_retry",
			retryAttempts: 3,
			sendEventsFunc: func(ctx context.Context, events []EventPayload) error {
				return fmt.Errorf("temporary error")
			},
			expectError:   true, // context cancellation should return error
			expectDropped: 0,
			expectSent:    0,
		},
		{
			name:          "exhaust_all_retries",
			retryAttempts: 2,
			sendEventsFunc: func(ctx context.Context, events []EventPayload) error {
				return fmt.Errorf("persistent error")
			},
			expectError:   true,
			expectDropped: 2,
			expectSent:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := &mockPluginForCoverage{sendEventsFunc: tt.sendEventsFunc}
			config := Config{
				Plugin:        plugin,
				PluginName:    "test-plugin",
				RetryAttempts: tt.retryAttempts,
				RetryBackoff:  10 * time.Millisecond, // short backoff for tests
			}

			service := &Service{
				config: config,
				logger: logger,
			}

			batch := []EventPayload{
				{Event: "test1", Type: "llm", Timestamp: time.Now()},
				{Event: "test2", Type: "llm", Timestamp: time.Now()},
			}

			ctx := context.Background()
			if tt.name == "context_cancelled_during_retry" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 25*time.Millisecond)
				defer cancel()
			}

			err := service.sendBatchWithResult(ctx, batch)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			processed, dropped, sent := service.Stats()
			assert.Equal(t, tt.expectDropped, dropped, "dropped events count")
			assert.Equal(t, tt.expectSent, sent, "sent events count")
			assert.Equal(t, int64(0), processed) // processEvents tracks this, not sendBatchWithResult
		})
	}
}

// Test service stop during batch sending
func TestService_sendBatchWithResult_StopDuringRetry(t *testing.T) {
	logger := zaptest.NewLogger(t)
	plugin := &mockPluginForCoverage{
		sendEventsFunc: func(ctx context.Context, events []EventPayload) error {
			return fmt.Errorf("temporary error")
		},
	}

	config := Config{
		Plugin:        plugin,
		PluginName:    "test-plugin", 
		RetryAttempts: 3,
		RetryBackoff:  100 * time.Millisecond,
	}

	service := &Service{
		config: config,
		logger: logger,
		stopCh: make(chan struct{}),
	}

	batch := []EventPayload{
		{Event: "test", Type: "llm", Timestamp: time.Now()},
	}

	// Stop the service after a short delay to test stop channel
	go func() {
		time.Sleep(50 * time.Millisecond)
		close(service.stopCh)
	}()

	err := service.sendBatchWithResult(context.Background(), batch)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stopped")

	_, dropped, _ := service.Stats()
	assert.Equal(t, int64(0), dropped) // should not be marked as dropped when stopped
}

// Test NewServiceWithBus error scenarios for coverage
func TestNewServiceWithBus_ErrorScenarios(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("nil_plugin", func(t *testing.T) {
		bus := eventbus.NewInMemoryEventBus(100)
		service, err := NewServiceWithBus(Config{}, logger, bus)
		assert.Error(t, err)
		assert.Nil(t, service)
		assert.Contains(t, err.Error(), "backend plugin is required")
	})

	t.Run("valid_plugin", func(t *testing.T) {
		plugin := &mockPluginForCoverage{}
		bus := eventbus.NewInMemoryEventBus(100)
		service, err := NewServiceWithBus(Config{Plugin: plugin}, logger, bus)
		assert.NoError(t, err)
		assert.NotNil(t, service)
		if service != nil {
			service.Stop()
		}
	})
}

// Test processEvents edge cases for better coverage
func TestService_processEvents_LogBasedBus(t *testing.T) {
	logger := zaptest.NewLogger(t)
	plugin := &mockPluginForCoverage{}
	
	config := Config{
		Plugin:        plugin,
		PluginName:    "test-dispatcher",
		BatchSize:     2,
		FlushInterval: 100 * time.Millisecond,
	}

	// Create a mock Redis client and use it for a log-based event bus
	mockRedis := newMockRedisClientLog()
	bus := eventbus.NewRedisEventBusPublisher(mockRedis, "test-events")

	service, err := NewServiceWithBus(config, logger, bus)
	require.NoError(t, err)
	require.NotNil(t, service)

	// Start processing in background with proper WaitGroup management
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Add to WaitGroup before starting processEvents
	service.wg.Add(1)
	go service.processEvents(ctx)

	// Wait for timeout or completion
	done := make(chan struct{})
	go func() {
		service.wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		// Test completed successfully with timeout
	case <-done:
		// Test completed successfully when processEvents finished
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Test timeout")
	}

	service.Stop()
}

// Test NewService default configuration coverage
func TestNewService_DefaultConfig(t *testing.T) {
	logger := zaptest.NewLogger(t)

	config := Config{
		Plugin: &mockPluginForCoverage{},
	}

	service, err := NewService(config, logger)
	assert.NoError(t, err)
	assert.NotNil(t, service)

	// Check that default values were applied
	assert.Equal(t, 100, service.config.BatchSize)
	assert.Equal(t, 5*time.Second, service.config.FlushInterval)
	
	service.Stop()
}