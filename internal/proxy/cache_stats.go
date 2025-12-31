// Package proxy provides the transparent reverse proxy implementation.
package proxy

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// CacheStatsStore defines the interface for persisting cache hit counts.
//
// Consistency invariant: Under normal operation, CacheHitCount ≤ RequestCount should
// always hold for any token. Cache hits are only recorded when responses are served
// from cache, which requires a prior upstream request that incremented RequestCount.
//
// If CacheHitCount > RequestCount is observed, it indicates a system issue that should
// be investigated (e.g., request count increment failed while cache hit was recorded).
// The Admin UI uses safeSub to display max(0, RequestCount-CacheHitCount) to handle
// this edge case gracefully.
type CacheStatsStore interface {
	// IncrementCacheHitCountBatch increments cache_hit_count for multiple tokens.
	// The deltas map has token IDs as keys and increment values as values.
	IncrementCacheHitCountBatch(ctx context.Context, deltas map[string]int) error
}

// CacheStatsAggregatorConfig holds configuration for the cache stats aggregator.
type CacheStatsAggregatorConfig struct {
	BufferSize    int           // Size of the buffered channel (default: 1000)
	FlushInterval time.Duration // How often to flush stats to DB (default: 5s)
	BatchSize     int           // Max events before flush (default: 100)
}

// DefaultCacheStatsAggregatorConfig returns the default configuration.
func DefaultCacheStatsAggregatorConfig() CacheStatsAggregatorConfig {
	return CacheStatsAggregatorConfig{
		BufferSize:    1000,
		FlushInterval: 5 * time.Second,
		BatchSize:     100,
	}
}

// CacheStatsAggregator aggregates cache hit events and periodically flushes them to the database.
// It uses a buffered channel for non-blocking enqueue and drops events when the buffer is full.
type CacheStatsAggregator struct {
	config   CacheStatsAggregatorConfig
	store    CacheStatsStore
	logger   *zap.Logger
	eventsCh chan string // channel of token IDs
	stopCh   chan struct{}
	doneCh   chan struct{}
	mu       sync.RWMutex
	stopped  bool
}

// NewCacheStatsAggregator creates a new aggregator with the given configuration.
func NewCacheStatsAggregator(config CacheStatsAggregatorConfig, store CacheStatsStore, logger *zap.Logger) *CacheStatsAggregator {
	if config.BufferSize <= 0 {
		config.BufferSize = 1000
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = 5 * time.Second
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 100
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &CacheStatsAggregator{
		config:   config,
		store:    store,
		logger:   logger,
		eventsCh: make(chan string, config.BufferSize),
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

// Start begins the background aggregation worker.
func (a *CacheStatsAggregator) Start() {
	go a.run()
}

// Stop gracefully shuts down the aggregator, flushing any pending stats.
func (a *CacheStatsAggregator) Stop(ctx context.Context) error {
	a.mu.Lock()
	if a.stopped {
		a.mu.Unlock()
		return nil
	}
	a.stopped = true
	a.mu.Unlock()

	close(a.stopCh)

	select {
	case <-a.doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// RecordCacheHit enqueues a cache hit event for the given token.
// This is non-blocking; if the buffer is full, the event is dropped.
func (a *CacheStatsAggregator) RecordCacheHit(tokenID string) {
	if tokenID == "" {
		return
	}

	a.mu.RLock()
	stopped := a.stopped
	a.mu.RUnlock()
	if stopped {
		return
	}

	select {
	case a.eventsCh <- tokenID:
		// Event enqueued successfully
	default:
		// Buffer full, drop the event
		a.logger.Debug("cache stats buffer full, dropping event",
			zap.String("token_id", tokenID))
	}
}

// run is the main loop of the aggregator worker.
func (a *CacheStatsAggregator) run() {
	defer close(a.doneCh)

	ticker := time.NewTicker(a.config.FlushInterval)
	defer ticker.Stop()

	deltas := make(map[string]int)
	eventCount := 0

	flush := func() {
		if len(deltas) == 0 {
			return
		}

		// Copy and reset
		toFlush := deltas
		deltas = make(map[string]int)
		flushedCount := eventCount
		eventCount = 0

		// Flush with a short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		if err := a.store.IncrementCacheHitCountBatch(ctx, toFlush); err != nil {
			a.logger.Warn("failed to flush cache hit stats",
				zap.Error(err),
				zap.Int("event_count", flushedCount),
				zap.Int("token_count", len(toFlush)))
			// On error, we drop the stats (lossy-tolerant as per design)
		} else {
			a.logger.Debug("flushed cache hit stats",
				zap.Int("event_count", flushedCount),
				zap.Int("token_count", len(toFlush)))
		}
	}

	for {
		select {
		case <-a.stopCh:
			// Drain remaining events
			for {
				select {
				case tokenID := <-a.eventsCh:
					deltas[tokenID]++
					eventCount++
				default:
					// No more events
					flush()
					return
				}
			}

		case tokenID := <-a.eventsCh:
			deltas[tokenID]++
			eventCount++
			if eventCount >= a.config.BatchSize {
				flush()
			}

		case <-ticker.C:
			flush()
		}
	}
}
