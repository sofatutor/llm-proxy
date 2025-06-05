package dispatcher

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/eventbus"
	"go.uber.org/zap/zaptest"
)

// mockPlugin is a mock implementation of BackendPlugin for testing
type mockPlugin struct {
	mu       sync.Mutex
	events   [][]EventPayload
	initCfg  map[string]string
	sendErr  error
	closeErr error
}

func (m *mockPlugin) Init(cfg map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.initCfg = cfg
	return nil
}

func (m *mockPlugin) SendEvents(ctx context.Context, events []EventPayload) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sendErr != nil {
		return m.sendErr
	}
	m.events = append(m.events, events)
	return nil
}

func (m *mockPlugin) Close() error {
	return m.closeErr
}

func (m *mockPlugin) getEvents() [][]EventPayload {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Return a copy to avoid races
	result := make([][]EventPayload, len(m.events))
	copy(result, m.events)
	return result
}

func TestNewService(t *testing.T) {
	plugin := &mockPlugin{}
	logger := zaptest.NewLogger(t)

	// Test with valid config
	cfg := Config{
		Plugin: plugin,
	}

	service, err := NewService(cfg, logger)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	if service == nil {
		t.Fatal("Expected non-nil service")
	}

	// Test defaults are applied
	if service.config.BufferSize != 1000 {
		t.Errorf("Expected BufferSize 1000, got %d", service.config.BufferSize)
	}

	if service.config.BatchSize != 100 {
		t.Errorf("Expected BatchSize 100, got %d", service.config.BatchSize)
	}

	// Test without plugin
	cfg.Plugin = nil
	_, err = NewService(cfg, logger)
	if err == nil {
		t.Fatal("Expected error when plugin is nil")
	}
}

func TestServiceProcessEvents(t *testing.T) {
	plugin := &mockPlugin{}
	logger := zaptest.NewLogger(t)

	cfg := Config{
		Plugin:        plugin,
		BufferSize:    10,
		BatchSize:     2,
		FlushInterval: 50 * time.Millisecond,
	}

	service, err := NewService(cfg, logger)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Start processing in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		service.processEvents(ctx)
	}()

	// Give the goroutine time to start
	time.Sleep(10 * time.Millisecond)

	// Send some events
	bus := service.EventBus()
	bus.Publish(context.Background(), eventbus.Event{
		RequestID: "test1",
		Method:    "POST",
		Path:      "/test",
		Status:    200,
	})

	bus.Publish(context.Background(), eventbus.Event{
		RequestID: "test2",
		Method:    "POST",
		Path:      "/test",
		Status:    200,
	})

	// Wait for events to be processed with exponential backoff
	maxWait := 2 * time.Second
	waitTime := 10 * time.Millisecond
	start := time.Now()

	for time.Since(start) < maxWait {
		events := plugin.getEvents()
		if len(events) > 0 {
			break
		}
		time.Sleep(waitTime)
		if waitTime < 100*time.Millisecond {
			waitTime *= 2
		}
	}

	events := plugin.getEvents()

	// Check that events were sent
	if len(events) == 0 {
		t.Fatal("Expected events to be sent to plugin")
	}

	if len(events[0]) != 2 {
		t.Fatalf("Expected batch size 2, got %d", len(events[0]))
	}

	// Stop service
	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestServiceRun(t *testing.T) {
	plugin := &mockPlugin{}
	logger := zaptest.NewLogger(t)

	cfg := Config{
		Plugin:     plugin,
		BufferSize: 10,
		BatchSize:  2,
	}

	service, err := NewService(cfg, logger)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Test foreground run (should return quickly when context is cancelled)
	ctx, cancel := context.WithCancel(context.Background())

	// Start Run in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- service.Run(ctx, false) // foreground mode
	}()

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	cancel()

	// Should return without error
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}

func TestServiceRunDetached(t *testing.T) {
	plugin := &mockPlugin{}
	logger := zaptest.NewLogger(t)

	cfg := Config{
		Plugin:     plugin,
		BufferSize: 10,
		BatchSize:  2,
	}

	service, err := NewService(cfg, logger)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Test detached run (should block until context is cancelled)
	ctx, cancel := context.WithCancel(context.Background())

	// Start Run in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- service.Run(ctx, true) // detached mode
	}()

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Cancel context to stop it
	cancel()

	// Should return without error
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Run in detached mode returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}

func TestServiceStop(t *testing.T) {
	plugin := &mockPlugin{}
	logger := zaptest.NewLogger(t)

	cfg := Config{
		Plugin:     plugin,
		BufferSize: 10,
		BatchSize:  2,
	}

	service, err := NewService(cfg, logger)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Stop should work even if not started
	err = service.Stop()
	if err != nil {
		t.Errorf("Stop returned error: %v", err)
	}

	// Multiple stops should be safe
	err = service.Stop()
	if err != nil {
		t.Errorf("Multiple Stop calls should be safe: %v", err)
	}
	err = service.Stop()
	if err != nil {
		t.Errorf("Multiple Stop calls should be safe: %v", err)
	}
}

func TestServiceStats(t *testing.T) {
	plugin := &mockPlugin{}
	logger := zaptest.NewLogger(t)

	cfg := Config{
		Plugin:     plugin,
		BufferSize: 10,
		BatchSize:  2,
	}

	service, err := NewService(cfg, logger)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Get initial stats
	processed, dropped, sent := service.Stats()
	if processed != 0 {
		t.Errorf("Expected processed 0, got %d", processed)
	}
	if dropped != 0 {
		t.Errorf("Expected dropped 0, got %d", dropped)
	}
	if sent != 0 {
		t.Errorf("Expected sent 0, got %d", sent)
	}
}

func TestServiceSendBatchErrors(t *testing.T) {
	plugin := &mockPlugin{
		sendErr: fmt.Errorf("mock send error"),
	}
	logger := zaptest.NewLogger(t)

	cfg := Config{
		Plugin:        plugin,
		BufferSize:    10,
		BatchSize:     2,
		RetryAttempts: 2,
		RetryBackoff:  10 * time.Millisecond,
	}

	service, err := NewService(cfg, logger)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Start processing in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		service.processEvents(ctx)
	}()

	// Give the goroutine time to start
	time.Sleep(10 * time.Millisecond)

	// Send events that will fail to send
	bus := service.EventBus()
	bus.Publish(context.Background(), eventbus.Event{
		RequestID: "test1",
		Method:    "POST",
		Path:      "/test",
		Status:    200,
	})

	bus.Publish(context.Background(), eventbus.Event{
		RequestID: "test2",
		Method:    "POST",
		Path:      "/test",
		Status:    200,
	})

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Check stats for errors - sendErr will cause events to be dropped
	_, dropped, _ := service.Stats()
	if dropped == 0 {
		t.Error("Expected dropped > 0 due to send errors")
	}

	// Stop service
	cancel()
	time.Sleep(50 * time.Millisecond)
}
