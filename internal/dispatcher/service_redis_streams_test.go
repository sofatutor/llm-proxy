package dispatcher

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/eventbus"
	"go.uber.org/zap/zaptest"
)

// TestServiceHealthCheck tests the health check functionality
func TestServiceHealthCheck(t *testing.T) {
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
	defer func() {
		_ = service.Stop()
	}()

	ctx := context.Background()

	// Initial health should be healthy
	health := service.Health(ctx)
	if !health.Healthy {
		t.Errorf("Expected healthy status initially, got: %v", health)
	}
	if health.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got: %s", health.Status)
	}
}

// TestServiceDetailedStats tests detailed statistics
func TestServiceDetailedStats(t *testing.T) {
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
	defer func() {
		_ = service.Stop()
	}()

	// Get initial detailed stats
	stats := service.DetailedStats()
	if stats == nil {
		t.Fatal("DetailedStats returned nil")
	}

	// Check expected fields
	expectedFields := []string{
		"events_processed",
		"events_dropped",
		"events_sent",
		"processing_rate",
		"lag_count",
		"stream_length",
		"last_processed_at",
	}

	for _, field := range expectedFields {
		if _, ok := stats[field]; !ok {
			t.Errorf("DetailedStats missing field: %s", field)
		}
	}

	// Initial values should be zero
	if stats["events_processed"].(int64) != 0 {
		t.Errorf("Expected events_processed 0, got %v", stats["events_processed"])
	}
}

// TestServiceExponentialBackoff tests exponential backoff retry logic
func TestServiceExponentialBackoff(t *testing.T) {
	retryCount := 0
	retryTimes := []time.Time{}

	plugin := &mockPlugin{}
	plugin.OnSend = func(batch []EventPayload) error {
		retryCount++
		retryTimes = append(retryTimes, time.Now())
		if retryCount <= 2 {
			return fmt.Errorf("temporary error")
		}
		return nil // succeed on third attempt
	}

	logger := zaptest.NewLogger(t)

	cfg := Config{
		Plugin:        plugin,
		BufferSize:    10,
		BatchSize:     1,
		RetryAttempts: 3,
		RetryBackoff:  100 * time.Millisecond,
		FlushInterval: 50 * time.Millisecond,
	}

	service, err := NewService(cfg, logger)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Start service in background first
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		_ = service.Run(ctx, false)
	}()

	// Give service time to start
	time.Sleep(100 * time.Millisecond)

	// Publish an event
	service.eventBus.Publish(context.Background(), eventbus.Event{
		RequestID: "test-1",
		Method:    "POST",
		Path:      "/test",
		Status:    200,
	})

	// Wait for retries to complete
	time.Sleep(2 * time.Second)
	_ = service.Stop()

	// Should have tried at least 3 times
	if retryCount < 3 {
		t.Errorf("Expected at least 3 retry attempts, got %d", retryCount)
	}

	// Check that backoff times increase exponentially
	if len(retryTimes) >= 3 {
		const minBackoffMultiplier = 1.5 // Allow some margin for scheduling delays
		firstBackoff := retryTimes[1].Sub(retryTimes[0])
		secondBackoff := retryTimes[2].Sub(retryTimes[1])

		// Second backoff should be roughly 2x the first (exponential)
		// Allow some margin for scheduling delays
		if secondBackoff < time.Duration(float64(firstBackoff)*minBackoffMultiplier) {
			t.Errorf("Expected exponential backoff, got first=%v, second=%v", firstBackoff, secondBackoff)
		}
	}
}

// TestServicePermanentError tests that permanent errors don't retry
func TestServicePermanentError(t *testing.T) {
	retryCount := 0
	plugin := &mockPlugin{}
	plugin.OnSend = func(batch []EventPayload) error {
		retryCount++
		return &PermanentBackendError{Msg: "permanent failure"}
	}

	logger := zaptest.NewLogger(t)

	cfg := Config{
		Plugin:        plugin,
		BufferSize:    10,
		BatchSize:     1,
		RetryAttempts: 3,
		RetryBackoff:  50 * time.Millisecond,
		FlushInterval: 50 * time.Millisecond,
	}

	service, err := NewService(cfg, logger)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Start service in background first
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	go func() {
		_ = service.Run(ctx, false)
	}()

	// Give service time to start
	time.Sleep(100 * time.Millisecond)

	// Publish an event
	service.eventBus.Publish(context.Background(), eventbus.Event{
		RequestID: "test-1",
		Method:    "POST",
		Path:      "/test",
		Status:    200,
	})

	// Wait for processing
	time.Sleep(500 * time.Millisecond)
	_ = service.Stop()

	// Should only try once (no retries for permanent errors)
	if retryCount != 1 {
		t.Errorf("Expected 1 attempt for permanent error, got %d", retryCount)
	}

	// Event should be counted as dropped
	_, dropped, _ := service.Stats()
	if dropped != 1 {
		t.Errorf("Expected 1 dropped event, got %d", dropped)
	}
}

// TestServiceMetricsTracking tests that metrics are tracked correctly
func TestServiceMetricsTracking(t *testing.T) {
	plugin := &mockPlugin{}
	logger := zaptest.NewLogger(t)

	cfg := Config{
		Plugin:        plugin,
		BufferSize:    100,
		BatchSize:     10,
		FlushInterval: 100 * time.Millisecond,
	}

	service, err := NewService(cfg, logger)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Start service first
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		_ = service.Run(ctx, false)
	}()

	// Give service time to start
	time.Sleep(100 * time.Millisecond)

	// Publish multiple events
	for i := 0; i < 50; i++ {
		service.eventBus.Publish(context.Background(), eventbus.Event{
			RequestID: fmt.Sprintf("test-%d", i),
			Method:    "POST",
			Path:      "/test",
			Status:    200,
		})
	}

	// Wait for processing and metrics updates
	time.Sleep(1 * time.Second)
	_ = service.Stop()

	// Check stats
	processed, _, sent := service.Stats()
	if processed == 0 {
		t.Error("Expected some events to be processed")
	}
	if sent == 0 {
		t.Error("Expected some events to be sent")
	}

	// Check detailed stats
	stats := service.DetailedStats()
	if stats["events_processed"].(int64) == 0 {
		t.Error("Expected events_processed > 0 in detailed stats")
	}

	// Processing rate should be calculated
	processingRate := stats["processing_rate"].(float64)
	if processingRate < 0 {
		t.Errorf("Expected non-negative processing rate, got %f", processingRate)
	}
}
