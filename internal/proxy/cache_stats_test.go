package proxy

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/obfuscate"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
)

// mockCacheStatsStore is a mock implementation of CacheStatsStore for testing.
type mockCacheStatsStore struct {
	mu          sync.Mutex
	calls       []map[string]int
	callCount   int32
	shouldError bool
	delay       time.Duration
}

func (m *mockCacheStatsStore) IncrementCacheHitCountBatch(ctx context.Context, deltas map[string]int) error {
	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(m.delay):
		}
	}

	atomic.AddInt32(&m.callCount, 1)

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldError {
		return context.DeadlineExceeded
	}

	// Copy the deltas to avoid mutation issues
	copied := make(map[string]int)
	for k, v := range deltas {
		copied[k] = v
	}
	m.calls = append(m.calls, copied)
	return nil
}

func (m *mockCacheStatsStore) getCalls() []map[string]int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func (m *mockCacheStatsStore) getCallCount() int {
	return int(atomic.LoadInt32(&m.callCount))
}

func TestCacheStatsAggregator_RecordCacheHit(t *testing.T) {
	store := &mockCacheStatsStore{}
	logger := zaptest.NewLogger(t)

	config := CacheStatsAggregatorConfig{
		BufferSize:    10,
		FlushInterval: 100 * time.Millisecond,
		BatchSize:     5,
	}

	agg := NewCacheStatsAggregator(config, store, logger)
	agg.Start()

	// Record some cache hits
	agg.RecordCacheHit("token1")
	agg.RecordCacheHit("token2")
	agg.RecordCacheHit("token1")
	agg.RecordCacheHit("token3")
	agg.RecordCacheHit("token1")

	// Should trigger batch flush (5 events)
	time.Sleep(50 * time.Millisecond)

	// Stop and wait for final flush
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := agg.Stop(ctx); err != nil {
		t.Fatalf("failed to stop aggregator: %v", err)
	}

	calls := store.getCalls()
	if len(calls) == 0 {
		t.Fatal("expected at least one flush call")
	}

	// Verify the accumulated deltas
	totalToken1 := 0
	totalToken2 := 0
	totalToken3 := 0
	for _, call := range calls {
		totalToken1 += call["token1"]
		totalToken2 += call["token2"]
		totalToken3 += call["token3"]
	}

	if totalToken1 != 3 {
		t.Errorf("expected token1 count to be 3, got %d", totalToken1)
	}
	if totalToken2 != 1 {
		t.Errorf("expected token2 count to be 1, got %d", totalToken2)
	}
	if totalToken3 != 1 {
		t.Errorf("expected token3 count to be 1, got %d", totalToken3)
	}
}

func TestCacheStatsAggregator_FlushInterval(t *testing.T) {
	store := &mockCacheStatsStore{}
	logger := zaptest.NewLogger(t)

	config := CacheStatsAggregatorConfig{
		BufferSize:    100,
		FlushInterval: 50 * time.Millisecond,
		BatchSize:     1000, // High batch size to ensure time-based flush
	}

	agg := NewCacheStatsAggregator(config, store, logger)
	agg.Start()

	// Record a cache hit
	agg.RecordCacheHit("token1")

	// Wait for time-based flush
	time.Sleep(100 * time.Millisecond)

	// Verify flush happened
	if store.getCallCount() < 1 {
		t.Error("expected at least one flush after flush interval")
	}

	// Stop
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := agg.Stop(ctx); err != nil {
		t.Fatalf("failed to stop aggregator: %v", err)
	}
}

func TestCacheStatsAggregator_BufferFull_DropsEvents(t *testing.T) {
	store := &mockCacheStatsStore{
		delay: 500 * time.Millisecond, // Slow store to cause buffer backup
	}
	logger := zaptest.NewLogger(t)

	config := CacheStatsAggregatorConfig{
		BufferSize:    5,
		FlushInterval: 10 * time.Millisecond,
		BatchSize:     3,
	}

	agg := NewCacheStatsAggregator(config, store, logger)
	agg.Start()

	// Record more events than buffer can hold
	for i := 0; i < 20; i++ {
		agg.RecordCacheHit("token1")
	}

	// Some events should be dropped (no panic, no blocking)
	// Stop
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = agg.Stop(ctx)

	// We just verify it doesn't block or panic
}

func TestCacheStatsAggregator_BufferFull_DropsEvents_DoesNotLogRawToken(t *testing.T) {
	store := &mockCacheStatsStore{
		delay: 500 * time.Millisecond, // Slow store to cause buffer backup
	}

	core, recorded := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	config := CacheStatsAggregatorConfig{
		BufferSize:    1,
		FlushInterval: time.Hour,
		BatchSize:     1000,
	}

	agg := NewCacheStatsAggregator(config, store, logger)
	secretToken := "sk-THIS_SHOULD_NOT_APPEAR_IN_LOGS"
	agg.RecordCacheHit(secretToken)
	agg.RecordCacheHit(secretToken) // second enqueue should be dropped and logged

	// Start the worker after filling the channel to make the drop deterministic.
	agg.Start()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = agg.Stop(ctx)

	entries := recorded.FilterMessage("cache stats buffer full, dropping event").All()
	if len(entries) == 0 {
		t.Fatalf("expected at least one buffer-full log entry")
	}

	for _, entry := range entries {
		for _, field := range entry.Context {
			if field.Key != "token_id" {
				continue
			}
			if field.String == secretToken {
				t.Fatalf("expected token_id to be obfuscated, got raw token")
			}
			if field.String == "" {
				t.Fatalf("expected token_id field to be set")
			}
			if field.String != obfuscate.ObfuscateTokenGeneric(secretToken) {
				t.Fatalf("expected token_id to be obfuscated; got %q", field.String)
			}
		}
	}
}

func TestCacheStatsAggregator_GracefulShutdown(t *testing.T) {
	store := &mockCacheStatsStore{}
	logger := zaptest.NewLogger(t)

	config := CacheStatsAggregatorConfig{
		BufferSize:    100,
		FlushInterval: time.Hour, // Long interval to ensure shutdown flush
		BatchSize:     1000,
	}

	agg := NewCacheStatsAggregator(config, store, logger)
	agg.Start()

	// Record events
	agg.RecordCacheHit("token1")
	agg.RecordCacheHit("token2")

	// Allow events to be processed
	time.Sleep(10 * time.Millisecond)

	// Stop should flush pending events
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := agg.Stop(ctx); err != nil {
		t.Fatalf("failed to stop aggregator: %v", err)
	}

	calls := store.getCalls()
	if len(calls) == 0 {
		t.Error("expected shutdown to flush pending events")
	}

	// Verify events were flushed
	totalEvents := 0
	for _, call := range calls {
		for _, v := range call {
			totalEvents += v
		}
	}
	if totalEvents != 2 {
		t.Errorf("expected 2 events to be flushed, got %d", totalEvents)
	}
}

func TestCacheStatsAggregator_EmptyTokenID(t *testing.T) {
	store := &mockCacheStatsStore{}
	logger := zaptest.NewLogger(t)

	config := DefaultCacheStatsAggregatorConfig()
	config.FlushInterval = 50 * time.Millisecond

	agg := NewCacheStatsAggregator(config, store, logger)
	agg.Start()

	// Recording empty token ID should be a no-op
	agg.RecordCacheHit("")

	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := agg.Stop(ctx); err != nil {
		t.Fatalf("failed to stop aggregator: %v", err)
	}

	// No events should have been recorded
	calls := store.getCalls()
	for _, call := range calls {
		if len(call) > 0 {
			t.Error("expected no events for empty token ID")
		}
	}
}

func TestCacheStatsAggregator_StoreError(t *testing.T) {
	store := &mockCacheStatsStore{
		shouldError: true,
	}
	logger := zaptest.NewLogger(t)

	config := CacheStatsAggregatorConfig{
		BufferSize:    10,
		FlushInterval: 50 * time.Millisecond,
		BatchSize:     5,
	}

	agg := NewCacheStatsAggregator(config, store, logger)
	agg.Start()

	// Record events
	for i := 0; i < 5; i++ {
		agg.RecordCacheHit("token1")
	}

	// Wait for flush attempt
	time.Sleep(100 * time.Millisecond)

	// Should not panic despite error
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := agg.Stop(ctx); err != nil {
		t.Fatalf("failed to stop aggregator: %v", err)
	}
}

func TestCacheStatsAggregator_DoubleStop(t *testing.T) {
	store := &mockCacheStatsStore{}
	logger := zaptest.NewLogger(t)

	config := DefaultCacheStatsAggregatorConfig()
	agg := NewCacheStatsAggregator(config, store, logger)
	agg.Start()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// First stop
	if err := agg.Stop(ctx); err != nil {
		t.Fatalf("first stop failed: %v", err)
	}

	// Second stop should be a no-op
	if err := agg.Stop(ctx); err != nil {
		t.Fatalf("second stop failed: %v", err)
	}
}

func TestCacheStatsAggregator_RecordAfterStop(t *testing.T) {
	store := &mockCacheStatsStore{}
	logger := zaptest.NewLogger(t)

	config := DefaultCacheStatsAggregatorConfig()
	agg := NewCacheStatsAggregator(config, store, logger)
	agg.Start()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := agg.Stop(ctx); err != nil {
		t.Fatalf("failed to stop aggregator: %v", err)
	}

	// Recording after stop should be a no-op (no panic)
	agg.RecordCacheHit("token1")
}

func TestCacheStatsAggregator_DefaultConfig(t *testing.T) {
	config := DefaultCacheStatsAggregatorConfig()

	if config.BufferSize != 1000 {
		t.Errorf("expected BufferSize 1000, got %d", config.BufferSize)
	}
	if config.FlushInterval != 5*time.Second {
		t.Errorf("expected FlushInterval 5s, got %v", config.FlushInterval)
	}
	if config.BatchSize != 100 {
		t.Errorf("expected BatchSize 100, got %d", config.BatchSize)
	}
}

func TestCacheStatsAggregator_NilLogger(t *testing.T) {
	store := &mockCacheStatsStore{}

	config := DefaultCacheStatsAggregatorConfig()
	config.FlushInterval = 50 * time.Millisecond

	// Should not panic with nil logger
	agg := NewCacheStatsAggregator(config, store, nil)
	agg.Start()

	agg.RecordCacheHit("token1")

	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := agg.Stop(ctx); err != nil {
		t.Fatalf("failed to stop aggregator: %v", err)
	}
}

func TestCacheStatsAggregator_ZeroConfig(t *testing.T) {
	store := &mockCacheStatsStore{}
	logger := zap.NewNop()

	// Zero config should use defaults
	config := CacheStatsAggregatorConfig{}
	agg := NewCacheStatsAggregator(config, store, logger)

	// Verify defaults were applied
	if agg.config.BufferSize != 1000 {
		t.Errorf("expected default BufferSize 1000, got %d", agg.config.BufferSize)
	}
	if agg.config.FlushInterval != 5*time.Second {
		t.Errorf("expected default FlushInterval 5s, got %v", agg.config.FlushInterval)
	}
	if agg.config.BatchSize != 100 {
		t.Errorf("expected default BatchSize 100, got %d", agg.config.BatchSize)
	}
}

func TestCacheStatsAggregator_ConcurrentRecords(t *testing.T) {
	store := &mockCacheStatsStore{}
	logger := zaptest.NewLogger(t)

	config := CacheStatsAggregatorConfig{
		BufferSize:    1000,
		FlushInterval: 100 * time.Millisecond,
		BatchSize:     50,
	}

	agg := NewCacheStatsAggregator(config, store, logger)
	agg.Start()

	// Concurrent writes
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				agg.RecordCacheHit("token1")
			}
		}(i)
	}
	wg.Wait()

	// Stop and verify
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := agg.Stop(ctx); err != nil {
		t.Fatalf("failed to stop aggregator: %v", err)
	}

	// Should have recorded 200 events total for token1
	calls := store.getCalls()
	totalToken1 := 0
	for _, call := range calls {
		totalToken1 += call["token1"]
	}

	if totalToken1 != 200 {
		t.Errorf("expected token1 count to be 200, got %d", totalToken1)
	}
}
