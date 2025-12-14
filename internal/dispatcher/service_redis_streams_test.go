package dispatcher

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sofatutor/llm-proxy/internal/eventbus"
	"go.uber.org/zap/zaptest"
)

type fakeRedisStreamsClient struct {
	pendingCount int64
	streamLen    int64

	xPendingErr error
	xLenErr     error
}

func (f *fakeRedisStreamsClient) XAdd(_ context.Context, _ *redis.XAddArgs) (string, error) {
	return "0-1", nil
}

func (f *fakeRedisStreamsClient) XReadGroup(_ context.Context, _ *redis.XReadGroupArgs) ([]redis.XStream, error) {
	return nil, nil
}

func (f *fakeRedisStreamsClient) XAck(_ context.Context, _, _ string, _ ...string) (int64, error) {
	return 0, nil
}

func (f *fakeRedisStreamsClient) XGroupCreateMkStream(_ context.Context, _, _, _ string) error {
	return nil
}

func (f *fakeRedisStreamsClient) XPending(_ context.Context, _, _ string) (*redis.XPending, error) {
	if f.xPendingErr != nil {
		return nil, f.xPendingErr
	}
	return &redis.XPending{Count: f.pendingCount}, nil
}

func (f *fakeRedisStreamsClient) XPendingExt(_ context.Context, _ *redis.XPendingExtArgs) ([]redis.XPendingExt, error) {
	return nil, nil
}

func (f *fakeRedisStreamsClient) XClaim(_ context.Context, _ *redis.XClaimArgs) ([]redis.XMessage, error) {
	return nil, nil
}

func (f *fakeRedisStreamsClient) XLen(_ context.Context, _ string) (int64, error) {
	if f.xLenErr != nil {
		return 0, f.xLenErr
	}
	return f.streamLen, nil
}

func (f *fakeRedisStreamsClient) XInfoGroups(_ context.Context, _ string) ([]redis.XInfoGroup, error) {
	return nil, nil
}

func newServiceWithFakeStreamsBus(t *testing.T, client *fakeRedisStreamsClient) *Service {
	t.Helper()
	logger := zaptest.NewLogger(t)
	plugin := &mockPlugin{}

	config := eventbus.DefaultRedisStreamsConfig()
	streamsBus := eventbus.NewRedisStreamsEventBus(client, config)

	cfg := Config{
		Plugin:        plugin,
		BufferSize:    10,
		BatchSize:     1,
		FlushInterval: 50 * time.Millisecond,
		RetryAttempts: 1,
		RetryBackoff:  10 * time.Millisecond,
		PluginName:    "test-plugin",
	}

	service, err := NewServiceWithBus(cfg, logger, streamsBus)
	if err != nil {
		t.Fatalf("NewServiceWithBus failed: %v", err)
	}
	return service
}

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
	defer func() {
		_ = service.Stop()
	}()

	// Start service in background first
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runDone := make(chan error, 1)

	go func() {
		runDone <- service.Run(ctx, false)
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

	cancel()
	select {
	case <-runDone:
	case <-time.After(2 * time.Second):
		t.Fatal("service.Run did not exit after cancel")
	}

	// Should have tried exactly 3 times (fail twice, then succeed).
	if retryCount != 3 {
		t.Errorf("Expected 3 retry attempts, got %d", retryCount)
	}
	if len(retryTimes) != 3 {
		t.Fatalf("Expected 3 retry timestamps, got %d", len(retryTimes))
	}

	// Check that backoff times increase exponentially.
	const minBackoffMultiplier = 1.5 // Allow some margin for scheduling delays
	firstBackoff := retryTimes[1].Sub(retryTimes[0])
	secondBackoff := retryTimes[2].Sub(retryTimes[1])

	// Second backoff should be roughly 2x the first (exponential), allowing margin.
	if secondBackoff < time.Duration(float64(firstBackoff)*minBackoffMultiplier) {
		t.Errorf("Expected exponential backoff, got first=%v, second=%v", firstBackoff, secondBackoff)
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
	defer func() {
		_ = service.Stop()
	}()

	// Start service in background first
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runDone := make(chan error, 1)

	go func() {
		runDone <- service.Run(ctx, false)
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

	cancel()
	select {
	case <-runDone:
	case <-time.After(2 * time.Second):
		t.Fatal("service.Run did not exit after cancel")
	}

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
	defer func() {
		_ = service.Stop()
	}()

	// Start service first
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runDone := make(chan error, 1)

	go func() {
		runDone <- service.Run(ctx, false)
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

	cancel()
	select {
	case <-runDone:
	case <-time.After(2 * time.Second):
		t.Fatal("service.Run did not exit after cancel")
	}

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

func TestServiceTrackMetrics_UpdatesRateAndStreamMetrics(t *testing.T) {
	client := &fakeRedisStreamsClient{pendingCount: 7, streamLen: 123}
	service := newServiceWithFakeStreamsBus(t, client)

	ticker := make(chan time.Time, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		service.trackMetrics(ctx, ticker)
		close(done)
	}()

	service.mu.Lock()
	service.eventsProcessed = 10
	service.mu.Unlock()

	ticker <- time.Now()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		service.mu.Lock()
		lag := service.lagCount
		length := service.streamLength
		rate := service.processingRate
		service.mu.Unlock()

		if lag == 7 && length == 123 && rate > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	service.mu.Lock()
	if service.lagCount != 7 {
		service.mu.Unlock()
		t.Fatalf("expected lagCount=7, got %d", service.lagCount)
	}
	if service.streamLength != 123 {
		service.mu.Unlock()
		t.Fatalf("expected streamLength=123, got %d", service.streamLength)
	}
	if service.processingRate <= 0 {
		service.mu.Unlock()
		t.Fatalf("expected processingRate > 0, got %f", service.processingRate)
	}
	service.mu.Unlock()

	cancel()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("trackMetrics did not exit after context cancel")
	}
}

func TestServiceHealth_StreamLagAndInactivityChecks(t *testing.T) {
	tests := []struct {
		name            string
		pending         int64
		streamLen       int64
		lastProcessedAt time.Time
		wantHealthy     bool
		wantMsgContains string
	}{
		{
			name:            "healthy with no lag",
			pending:         0,
			streamLen:       10,
			lastProcessedAt: time.Now(),
			wantHealthy:     true,
		},
		{
			name:            "unhealthy on high lag",
			pending:         maxHealthyLagCount + 1,
			streamLen:       100,
			lastProcessedAt: time.Now(),
			wantHealthy:     false,
			wantMsgContains: "High lag",
		},
		{
			name:            "unhealthy on inactivity with pending",
			pending:         1,
			streamLen:       100,
			lastProcessedAt: time.Now().Add(-maxInactivityDuration - 10*time.Second),
			wantHealthy:     false,
			wantMsgContains: "No processing activity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeRedisStreamsClient{pendingCount: tt.pending, streamLen: tt.streamLen}
			service := newServiceWithFakeStreamsBus(t, client)
			service.mu.Lock()
			service.lastProcessedAt = tt.lastProcessedAt
			service.mu.Unlock()

			health := service.Health(context.Background())
			if health.Healthy != tt.wantHealthy {
				t.Fatalf("expected healthy=%v, got %v (status=%s msg=%q)", tt.wantHealthy, health.Healthy, health.Status, health.Message)
			}
			if tt.wantMsgContains != "" && !strings.Contains(health.Message, tt.wantMsgContains) {
				t.Fatalf("expected message to contain %q, got %q", tt.wantMsgContains, health.Message)
			}
		})
	}
}

func TestServiceHealth_StreamsMetricsErrors_DoNotFailHealth(t *testing.T) {
	client := &fakeRedisStreamsClient{xPendingErr: fmt.Errorf("boom"), xLenErr: fmt.Errorf("boom")}
	service := newServiceWithFakeStreamsBus(t, client)

	service.mu.Lock()
	service.lastProcessedAt = time.Now().Add(-maxInactivityDuration - 10*time.Second)
	service.mu.Unlock()

	health := service.Health(context.Background())
	if !health.Healthy {
		t.Fatalf("expected health to remain healthy when streams metrics fail, got unhealthy (msg=%q)", health.Message)
	}
}
