package dispatcher

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/sofatutor/llm-proxy/internal/eventbus"
	"go.uber.org/zap/zaptest"
)

// TestServiceWithRedisStreams tests the dispatcher with Redis Streams backend
func TestServiceWithRedisStreams(t *testing.T) {
	// Start mini redis
	mr := miniredis.RunT(t)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("redis client close: %v", err)
		}
	})

	// Create Redis Streams event bus
	config := eventbus.DefaultRedisStreamsConfig()
	config.ConsumerName = "test-consumer"
	config.BlockTimeout = 100 * time.Millisecond
	config.BatchSize = 5

	adapter := &eventbus.RedisStreamsClientAdapter{Client: client}
	streamsBus := eventbus.NewRedisStreamsEventBus(adapter, config)

	ctx := context.Background()
	err := streamsBus.EnsureConsumerGroup(ctx)
	if err != nil {
		t.Fatalf("Failed to ensure consumer group: %v", err)
	}

	// Create dispatcher
	plugin := &mockPlugin{}
	logger := zaptest.NewLogger(t)

	cfg := Config{
		Plugin:        plugin,
		BufferSize:    100,
		BatchSize:     5,
		FlushInterval: 200 * time.Millisecond,
		RetryAttempts: 2,
		RetryBackoff:  50 * time.Millisecond,
		PluginName:    "test-plugin",
	}

	service, err := NewServiceWithBus(cfg, logger, streamsBus)
	if err != nil {
		t.Fatalf("NewServiceWithBus failed: %v", err)
	}

	// Publish events to the stream
	for i := 0; i < 10; i++ {
		streamsBus.Publish(ctx, eventbus.Event{
			RequestID: fmt.Sprintf("req-%d", i),
			Method:    "POST",
			Path:      "/test",
			Status:    200,
		})
	}

	// Start the dispatcher
	runCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	go func() {
		_ = service.Run(runCtx, false)
	}()

	// Wait for processing
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

	// Check stream metrics
	stats := service.DetailedStats()
	if stats["events_processed"].(int64) == 0 {
		t.Error("Expected events_processed > 0")
	}
}

// TestServiceRecoveryFromDowntime tests recovery from dispatcher downtime
func TestServiceRecoveryFromDowntime(t *testing.T) {
	// Start mini redis
	mr := miniredis.RunT(t)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("redis client close: %v", err)
		}
	})

	// Create Redis Streams event bus
	config := eventbus.DefaultRedisStreamsConfig()
	config.ConsumerName = "test-consumer-recovery"
	config.BlockTimeout = 100 * time.Millisecond
	config.BatchSize = 5

	adapter := &eventbus.RedisStreamsClientAdapter{Client: client}
	streamsBus := eventbus.NewRedisStreamsEventBus(adapter, config)

	ctx := context.Background()
	err := streamsBus.EnsureConsumerGroup(ctx)
	if err != nil {
		t.Fatalf("Failed to ensure consumer group: %v", err)
	}

	// Publish events BEFORE starting dispatcher (simulating downtime)
	for i := 0; i < 5; i++ {
		streamsBus.Publish(ctx, eventbus.Event{
			RequestID: fmt.Sprintf("req-before-%d", i),
			Method:    "POST",
			Path:      "/test",
			Status:    200,
		})
	}

	// Now create and start dispatcher
	plugin := &mockPlugin{}
	logger := zaptest.NewLogger(t)

	cfg := Config{
		Plugin:        plugin,
		BufferSize:    100,
		BatchSize:     5,
		FlushInterval: 200 * time.Millisecond,
		RetryAttempts: 2,
		RetryBackoff:  50 * time.Millisecond,
		PluginName:    "test-plugin",
	}

	service, err := NewServiceWithBus(cfg, logger, streamsBus)
	if err != nil {
		t.Fatalf("NewServiceWithBus failed: %v", err)
	}

	// Start the dispatcher
	runCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	go func() {
		_ = service.Run(runCtx, false)
	}()

	// Wait for processing
	time.Sleep(1 * time.Second)
	_ = service.Stop()

	// Should have processed the events that were published before it started
	processed, _, _ := service.Stats()
	if processed == 0 {
		t.Error("Expected dispatcher to process events from before it started (recovery)")
	}
}

// TestServiceLagMonitoring tests lag monitoring for Redis Streams
func TestServiceLagMonitoring(t *testing.T) {
	// Start mini redis
	mr := miniredis.RunT(t)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("redis client close: %v", err)
		}
	})

	// Create Redis Streams event bus
	config := eventbus.DefaultRedisStreamsConfig()
	config.ConsumerName = "test-consumer-lag"
	config.BlockTimeout = 100 * time.Millisecond
	config.BatchSize = 2

	adapter := &eventbus.RedisStreamsClientAdapter{Client: client}
	streamsBus := eventbus.NewRedisStreamsEventBus(adapter, config)

	ctx := context.Background()
	err := streamsBus.EnsureConsumerGroup(ctx)
	if err != nil {
		t.Fatalf("Failed to ensure consumer group: %v", err)
	}

	// Create a slow plugin to create lag
	var processedCount atomic.Int64
	plugin := &mockPlugin{}
	plugin.OnSend = func(batch []EventPayload) error {
		time.Sleep(100 * time.Millisecond) // Slow processing
		processedCount.Add(int64(len(batch)))
		return nil
	}

	logger := zaptest.NewLogger(t)

	cfg := Config{
		Plugin:        plugin,
		BufferSize:    100,
		BatchSize:     2,
		FlushInterval: 50 * time.Millisecond,
		RetryAttempts: 2,
		RetryBackoff:  50 * time.Millisecond,
		PluginName:    "test-plugin",
	}

	service, err := NewServiceWithBus(cfg, logger, streamsBus)
	if err != nil {
		t.Fatalf("NewServiceWithBus failed: %v", err)
	}

	// Start the dispatcher
	runCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	go func() {
		_ = service.Run(runCtx, false)
	}()

	// Publish events rapidly to create lag
	for i := 0; i < 10; i++ {
		streamsBus.Publish(ctx, eventbus.Event{
			RequestID: fmt.Sprintf("req-%d", i),
			Method:    "POST",
			Path:      "/test",
			Status:    200,
		})
	}

	// Wait a bit for metrics to update
	time.Sleep(500 * time.Millisecond)

	// Wait for processing to complete
	time.Sleep(2 * time.Second)
	_ = service.Stop()

	if processedCount.Load() == 0 {
		t.Error("Expected slow plugin to process at least one event")
	}

	// Eventually all should be processed
	finalProcessed, _, _ := service.Stats()
	if finalProcessed == 0 {
		t.Error("Expected some events to be processed")
	}
}

// TestServiceHealthCheckWithStreams tests health check with Redis Streams
func TestServiceHealthCheckWithStreams(t *testing.T) {
	// Start mini redis
	mr := miniredis.RunT(t)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("redis client close: %v", err)
		}
	})

	// Create Redis Streams event bus
	config := eventbus.DefaultRedisStreamsConfig()
	config.ConsumerName = "test-consumer-health"
	config.BlockTimeout = 100 * time.Millisecond

	adapter := &eventbus.RedisStreamsClientAdapter{Client: client}
	streamsBus := eventbus.NewRedisStreamsEventBus(adapter, config)

	ctx := context.Background()
	err := streamsBus.EnsureConsumerGroup(ctx)
	if err != nil {
		t.Fatalf("Failed to ensure consumer group: %v", err)
	}

	// Create dispatcher
	plugin := &mockPlugin{}
	logger := zaptest.NewLogger(t)

	cfg := Config{
		Plugin:        plugin,
		BufferSize:    100,
		BatchSize:     5,
		FlushInterval: 200 * time.Millisecond,
		PluginName:    "test-plugin",
	}

	service, err := NewServiceWithBus(cfg, logger, streamsBus)
	if err != nil {
		t.Fatalf("NewServiceWithBus failed: %v", err)
	}

	// Check initial health
	health := service.Health(ctx)
	if !health.Healthy {
		t.Error("Expected initial health to be healthy")
	}

	// Start service and process some events
	runCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	go func() {
		_ = service.Run(runCtx, false)
	}()

	// Publish a few events
	for i := 0; i < 5; i++ {
		streamsBus.Publish(ctx, eventbus.Event{
			RequestID: fmt.Sprintf("req-%d", i),
			Method:    "POST",
		})
	}

	time.Sleep(500 * time.Millisecond)

	// Check health again
	health = service.Health(ctx)
	if !health.Healthy {
		t.Errorf("Expected healthy status after processing, got: %v", health)
	}

	_ = service.Stop()
}
