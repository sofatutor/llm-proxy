package token

import (
	"context"
	"sync"
	"time"

	"github.com/sofatutor/llm-proxy/internal/obfuscate"
	"go.uber.org/zap"
)

// UsageStatsStore defines the interface for persisting per-token usage stats.
//
// Implementations are expected to treat tokenID as the token string (sk-...).
type UsageStatsStore interface {
	// IncrementTokenUsageBatch increments request_count for multiple tokens and updates last_used_at.
	// The deltas map has token strings as keys and increment values as values.
	IncrementTokenUsageBatch(ctx context.Context, deltas map[string]int, lastUsedAt time.Time) error
}

// UsageStatsAggregatorConfig holds configuration for the usage stats aggregator.
type UsageStatsAggregatorConfig struct {
	BufferSize    int           // Size of buffered channel (default: 1000)
	FlushInterval time.Duration // How often to flush stats (default: 5s)
	BatchSize     int           // Max events before flush (default: 100)
}

// DefaultUsageStatsAggregatorConfig returns default config.
func DefaultUsageStatsAggregatorConfig() UsageStatsAggregatorConfig {
	return UsageStatsAggregatorConfig{
		BufferSize:    1000,
		FlushInterval: 5 * time.Second,
		BatchSize:     100,
	}
}

// UsageStatsAggregator aggregates token usage events and periodically flushes them.
// It uses a buffered channel for non-blocking enqueue and drops events when the buffer is full.
type UsageStatsAggregator struct {
	config   UsageStatsAggregatorConfig
	store    UsageStatsStore
	logger   *zap.Logger
	eventsCh chan string // token strings
	stopCh   chan struct{}
	doneCh   chan struct{}
	mu       sync.RWMutex
	stopped  bool
}

// NewUsageStatsAggregator creates a new usage stats aggregator.
func NewUsageStatsAggregator(config UsageStatsAggregatorConfig, store UsageStatsStore, logger *zap.Logger) *UsageStatsAggregator {
	cfg := config
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 1000
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &UsageStatsAggregator{
		config:   cfg,
		store:    store,
		logger:   logger,
		eventsCh: make(chan string, cfg.BufferSize),
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

// Start begins the background aggregation worker.
func (a *UsageStatsAggregator) Start() {
	go a.run()
}

// Stop gracefully shuts down the aggregator, flushing any pending stats.
func (a *UsageStatsAggregator) Stop(ctx context.Context) error {
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

// RecordTokenUsage enqueues a token usage event for the given token string.
// This is non-blocking; if the buffer is full, the event is dropped.
func (a *UsageStatsAggregator) RecordTokenUsage(tokenString string) {
	if tokenString == "" {
		return
	}
	a.mu.RLock()
	stopped := a.stopped
	a.mu.RUnlock()
	if stopped {
		return
	}

	select {
	case a.eventsCh <- tokenString:
		// enqueued
	default:
		a.logger.Debug("usage stats buffer full, dropping event",
			zap.String("token", obfuscate.ObfuscateTokenGeneric(tokenString)))
	}
}

func (a *UsageStatsAggregator) run() {
	defer close(a.doneCh)

	ticker := time.NewTicker(a.config.FlushInterval)
	defer ticker.Stop()

	deltas := make(map[string]int)
	eventCount := 0

	flush := func() {
		if eventCount == 0 {
			return
		}

		snapshot := make(map[string]int, len(deltas))
		for tokenID, delta := range deltas {
			snapshot[tokenID] = delta
		}
		deltas = make(map[string]int)
		eventCount = 0

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		now := time.Now().UTC()
		if err := a.store.IncrementTokenUsageBatch(ctx, snapshot, now); err != nil {
			a.logger.Error("failed to flush usage stats batch", zap.Error(err))
		}
	}

	for {
		select {
		case <-a.stopCh:
			// Drain any queued events before the final flush so we don't lose
			// events that were successfully enqueued but not yet processed.
			for {
				select {
				case tokenID := <-a.eventsCh:
					deltas[tokenID]++
					eventCount++
				default:
					flush()
					return
				}
			}
		case <-ticker.C:
			flush()
		case tokenID := <-a.eventsCh:
			deltas[tokenID]++
			eventCount++
			if eventCount >= a.config.BatchSize {
				flush()
			}
		}
	}
}
