package dispatcher

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sofatutor/llm-proxy/internal/eventbus"
	"go.uber.org/zap"
)

const (
	// maxBackoffDuration is the maximum backoff duration for retries
	maxBackoffDuration = 30 * time.Second
	// maxHealthyLagCount is the maximum number of pending messages before the dispatcher is considered unhealthy
	maxHealthyLagCount = 10000
	// maxInactivityDuration is the maximum time without processing activity before the dispatcher is considered unhealthy
	maxInactivityDuration = 5 * time.Minute
	// metricsUpdateInterval is the interval at which metrics are updated
	metricsUpdateInterval = 10 * time.Second
)

// Config holds configuration for the dispatcher service
type Config struct {
	BufferSize       int
	BatchSize        int
	FlushInterval    time.Duration
	RetryAttempts    int
	RetryBackoff     time.Duration
	Plugin           BackendPlugin
	EventTransformer EventTransformer
	PluginName       string
	Verbose          bool // If true, include response_headers and extra debug info
}

// Service represents the event dispatcher service
type Service struct {
	config   Config
	eventBus eventbus.EventBus
	logger   *zap.Logger
	stopCh   chan struct{}
	wg       sync.WaitGroup
	stopOnce sync.Once
	// startedCh is closed after the event processing goroutine has been added
	// to the WaitGroup. This avoids a data race between Wait() and Add(1).
	startedCh chan struct{}

	// metrics
	mu              sync.Mutex
	eventsProcessed int64
	eventsDropped   int64
	eventsSent      int64
	lastProcessedAt time.Time
	processingRate  float64 // events per second
	lagCount        int64   // current lag (pending messages)
	streamLength    int64   // total messages in stream
}

// NewService creates a new dispatcher service
func NewService(cfg Config, logger *zap.Logger) (*Service, error) {
	if cfg.Plugin == nil {
		return nil, fmt.Errorf("backend plugin is required")
	}

	if cfg.EventTransformer == nil {
		cfg.EventTransformer = NewDefaultEventTransformer(cfg.Verbose)
	}

	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 1000
	}

	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}

	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}

	if cfg.RetryAttempts <= 0 {
		cfg.RetryAttempts = 3
	}

	if cfg.RetryBackoff <= 0 {
		cfg.RetryBackoff = time.Second
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	// Create event bus for the dispatcher
	bus := eventbus.NewInMemoryEventBus(cfg.BufferSize)

	return &Service{
		config:    cfg,
		eventBus:  bus,
		logger:    logger,
		stopCh:    make(chan struct{}),
		startedCh: make(chan struct{}),
	}, nil
}

// NewServiceWithBus creates a new dispatcher service with a provided event bus.
func NewServiceWithBus(cfg Config, logger *zap.Logger, bus eventbus.EventBus) (*Service, error) {
	if cfg.Plugin == nil {
		return nil, fmt.Errorf("backend plugin is required")
	}

	if cfg.EventTransformer == nil {
		cfg.EventTransformer = NewDefaultEventTransformer(cfg.Verbose)
	}

	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 1000
	}

	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}

	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}

	if cfg.RetryAttempts <= 0 {
		cfg.RetryAttempts = 3
	}

	if cfg.RetryBackoff <= 0 {
		cfg.RetryBackoff = time.Second
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	if bus == nil {
		return nil, fmt.Errorf("event bus must not be nil")
	}

	return &Service{
		config:    cfg,
		eventBus:  bus,
		logger:    logger,
		stopCh:    make(chan struct{}),
		startedCh: make(chan struct{}),
	}, nil
}

// Run starts the dispatcher service and blocks until stopped
func (s *Service) Run(ctx context.Context, detach bool) error {
	s.logger.Info("Starting event dispatcher service")

	if detach {
		return s.runDetached(ctx)
	}

	return s.runForeground(ctx)
}

// runDetached runs the service in background mode
func (s *Service) runDetached(ctx context.Context) error {
	s.logger.Info("Running in detached mode")

	// For detached mode, we still run in foreground but could be enhanced
	// to fork the process or use systemd/supervisor in production
	return s.runForeground(ctx)
}

// runForeground runs the service in foreground mode
func (s *Service) runForeground(ctx context.Context) error {
	// Handle graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// Start background goroutines.
	// Add to the WaitGroup before closing startedCh to avoid racing Wait() with Add().
	s.wg.Add(2)
	close(s.startedCh)
	go s.processEvents(ctx)
	go func() {
		defer s.wg.Done()
		metricsTicker := time.NewTicker(metricsUpdateInterval)
		defer metricsTicker.Stop()
		s.trackMetrics(ctx, metricsTicker.C)
	}()

	// Wait for shutdown signal
	select {
	case <-sigs:
		s.logger.Info("Received shutdown signal")
	case <-ctx.Done():
		s.logger.Info("Context cancelled")
	}

	return s.Stop()
}

// Stop gracefully stops the dispatcher service
func (s *Service) Stop() error {
	s.stopOnce.Do(func() {
		s.logger.Info("Stopping event dispatcher service")

		close(s.stopCh)
		// Avoid racing Wait() with a concurrent Add(1) during startup.
		select {
		case <-s.startedCh:
			s.wg.Wait()
		default:
			// Not started; nothing to wait for
		}

		if s.eventBus != nil {
			s.eventBus.Stop()
		}

		if s.config.Plugin != nil {
			if err := s.config.Plugin.Close(); err != nil {
				s.logger.Error("Error closing plugin", zap.Error(err))
			}
		}

		s.logger.Info("Event dispatcher service stopped")
	})
	return nil
}

// EventBus returns the event bus for this service (for connecting with other components)
func (s *Service) EventBus() eventbus.EventBus {
	return s.eventBus
}

// processEvents handles the main event processing loop
func (s *Service) processEvents(ctx context.Context) {
	defer s.wg.Done()

	sub := s.eventBus.Subscribe()
	batch := make([]EventPayload, 0, s.config.BatchSize)
	ticker := time.NewTicker(s.config.FlushInterval)
	defer ticker.Stop()

	// Channel-based event bus (in-memory or Redis Streams)
	for {
		select {
		case evt, ok := <-sub:
			if !ok {
				// Channel closed, flush remaining batch and exit
				if len(batch) > 0 {
					s.sendBatch(ctx, batch)
				}
				return
			}

			// Transform the event
			payload, err := s.config.EventTransformer.Transform(evt)
			if err != nil {
				s.mu.Lock()
				s.eventsDropped++
				s.mu.Unlock()
				s.logger.Error("Failed to transform event", zap.Error(err))
				continue
			}

			if payload == nil {
				// Event was filtered out (e.g., OPTIONS request)
				continue
			}

			batch = append(batch, *payload)
			s.mu.Lock()
			s.eventsProcessed++
			s.lastProcessedAt = time.Now()
			s.mu.Unlock()

			// Send batch if it's full
			if len(batch) >= s.config.BatchSize {
				s.sendBatch(ctx, batch)
				batch = batch[:0] // Reset slice
			}

		case <-ticker.C:
			// Flush batch on timer
			if len(batch) > 0 {
				s.sendBatch(ctx, batch)
				batch = batch[:0] // Reset slice
			}

		case <-s.stopCh:
			// Flush remaining batch and exit
			if len(batch) > 0 {
				s.sendBatch(ctx, batch)
			}
			return
		}
	}
}

// sendBatch sends a batch of events to the configured backend with exponential backoff retry logic
func (s *Service) sendBatch(ctx context.Context, batch []EventPayload) {
	for attempt := 0; attempt <= s.config.RetryAttempts; attempt++ {
		err := s.config.Plugin.SendEvents(ctx, batch)
		if err == nil {
			s.mu.Lock()
			s.eventsSent += int64(len(batch))
			s.mu.Unlock()
			s.logger.Debug("Successfully sent batch",
				zap.Int("batch_size", len(batch)),
				zap.Int("attempt", attempt+1))
			return
		}

		// Check for permanent errors that should not be retried
		if _, ok := err.(*PermanentBackendError); ok {
			s.logger.Warn("Permanent backend error, skipping batch", zap.Error(err), zap.Int("batch_size", len(batch)))
			s.mu.Lock()
			s.eventsDropped += int64(len(batch))
			s.mu.Unlock()
			return
		}

		if attempt < s.config.RetryAttempts {
			// Exponential backoff: 2^attempt * base backoff
			backoff := time.Duration(1<<uint(attempt)) * s.config.RetryBackoff
			// Cap at 30 seconds
			if backoff > maxBackoffDuration {
				backoff = maxBackoffDuration
			}
			s.logger.Warn("Failed to send batch, retrying with exponential backoff",
				zap.Error(err),
				zap.Int("attempt", attempt+1),
				zap.Duration("backoff", backoff))

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			case <-s.stopCh:
				return
			}
		} else {
			s.logger.Error("Failed to send batch after all retries",
				zap.Error(err),
				zap.Int("batch_size", len(batch)))
			s.mu.Lock()
			s.eventsDropped += int64(len(batch))
			s.mu.Unlock()
		}
	}
}

// sendBatchWithResult sends a batch of events to the configured backend with exponential backoff retry logic and returns the result
func (s *Service) sendBatchWithResult(ctx context.Context, batch []EventPayload) error {
	for attempt := 0; attempt <= s.config.RetryAttempts; attempt++ {
		err := s.config.Plugin.SendEvents(ctx, batch)
		if err == nil {
			s.mu.Lock()
			s.eventsSent += int64(len(batch))
			s.mu.Unlock()
			s.logger.Debug("Successfully sent batch",
				zap.Int("batch_size", len(batch)),
				zap.Int("attempt", attempt+1))
			return nil
		}
		// If PermanentBackendError, treat as delivered and do not retry
		if _, ok := err.(*PermanentBackendError); ok {
			s.logger.Warn("Permanent backend error, skipping batch", zap.Error(err), zap.Int("batch_size", len(batch)))
			s.mu.Lock()
			s.eventsDropped += int64(len(batch))
			s.mu.Unlock()
			return nil // treat as delivered
		}
		if attempt < s.config.RetryAttempts {
			// Exponential backoff: 2^attempt * base backoff
			backoff := time.Duration(1<<uint(attempt)) * s.config.RetryBackoff
			// Cap at 30 seconds
			if backoff > maxBackoffDuration {
				backoff = maxBackoffDuration
			}
			s.logger.Warn("Failed to send batch, retrying with exponential backoff",
				zap.Error(err),
				zap.Int("attempt", attempt+1),
				zap.Duration("backoff", backoff))
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			case <-s.stopCh:
				return fmt.Errorf("stopped")
			}
		} else {
			s.logger.Error("Failed to send batch after all retries",
				zap.Error(err),
				zap.Int("batch_size", len(batch)))
			s.mu.Lock()
			s.eventsDropped += int64(len(batch))
			s.mu.Unlock()
			return err
		}
	}
	return fmt.Errorf("unreachable")
}

// trackMetrics periodically updates metrics like processing rate and Redis Streams lag.
//
// It exits when the service is stopped (s.stopCh) or ctx is canceled.
func (s *Service) trackMetrics(ctx context.Context, ticker <-chan time.Time) {
	lastProcessed := int64(0)
	lastTime := time.Now()

	for {
		select {
		case <-ticker:
			now := time.Now()
			s.mu.Lock()
			currentProcessed := s.eventsProcessed
			s.mu.Unlock()

			// Calculate processing rate
			elapsed := now.Sub(lastTime).Seconds()
			if elapsed > 0 {
				rate := float64(currentProcessed-lastProcessed) / elapsed
				s.mu.Lock()
				s.processingRate = rate
				s.mu.Unlock()
			}

			lastProcessed = currentProcessed
			lastTime = now

			// Update lag metrics for Redis Streams
			if streamsBus, ok := s.eventBus.(*eventbus.RedisStreamsEventBus); ok {
				if pending, err := streamsBus.PendingCount(ctx); err == nil {
					s.mu.Lock()
					s.lagCount = pending
					s.mu.Unlock()
				}
				if length, err := streamsBus.StreamLength(ctx); err == nil {
					s.mu.Lock()
					s.streamLength = length
					s.mu.Unlock()
				}
			}

		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// Stats returns service statistics
func (s *Service) Stats() (processed, dropped, sent int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.eventsProcessed, s.eventsDropped, s.eventsSent
}

// DetailedStats returns detailed service statistics including lag and rate
func (s *Service) DetailedStats() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return map[string]interface{}{
		"events_processed":  s.eventsProcessed,
		"events_dropped":    s.eventsDropped,
		"events_sent":       s.eventsSent,
		"processing_rate":   s.processingRate,
		"lag_count":         s.lagCount,
		"stream_length":     s.streamLength,
		"last_processed_at": s.lastProcessedAt,
	}
}

// HealthStatus represents the health status of the dispatcher.
type HealthStatus struct {
	// Healthy indicates whether the dispatcher is currently healthy.
	Healthy bool `json:"healthy"`
	// Status is a human-readable string describing the current health status.
	Status string `json:"status"`
	// EventsProcessed is the total number of events processed by the dispatcher.
	EventsProcessed int64 `json:"events_processed"`
	// EventsDropped is the total number of events that were dropped and not processed.
	EventsDropped int64 `json:"events_dropped"`
	// EventsSent is the total number of events successfully sent to the backend.
	EventsSent int64 `json:"events_sent"`
	// ProcessingRate is the average number of events processed per second.
	ProcessingRate float64 `json:"processing_rate"`
	// LagCount is the number of pending messages in the event bus that have not yet been processed.
	// For Redis Streams, this is the pending entries count (XPENDING).
	LagCount int64 `json:"lag_count"`
	// StreamLength is the total number of messages currently in the Redis Stream.
	// For Redis Streams, this is the stream length (XLEN).
	StreamLength int64 `json:"stream_length"`
	// LastProcessedAt is the timestamp of the last successfully processed event.
	LastProcessedAt time.Time `json:"last_processed_at"`
	// Message provides additional information about the health status, if any.
	Message string `json:"message,omitempty"`
}

// Health returns the health status of the dispatcher
func (s *Service) Health(ctx context.Context) HealthStatus {
	s.mu.Lock()
	stats := HealthStatus{
		EventsProcessed: s.eventsProcessed,
		EventsDropped:   s.eventsDropped,
		EventsSent:      s.eventsSent,
		ProcessingRate:  s.processingRate,
		LagCount:        s.lagCount,
		StreamLength:    s.streamLength,
		LastProcessedAt: s.lastProcessedAt,
	}
	s.mu.Unlock()

	// Check if we're using Redis Streams
	if streamsBus, ok := s.eventBus.(*eventbus.RedisStreamsEventBus); ok {
		// Update lag and stream length from Redis
		if pending, err := streamsBus.PendingCount(ctx); err == nil {
			stats.LagCount = pending
		}
		if length, err := streamsBus.StreamLength(ctx); err == nil {
			stats.StreamLength = length
		}

		// Consider unhealthy if lag is very high
		if stats.LagCount > maxHealthyLagCount {
			stats.Healthy = false
			stats.Status = "unhealthy"
			stats.Message = fmt.Sprintf("High lag: %d pending messages", stats.LagCount)
			return stats
		}

		// Consider unhealthy if we haven't processed anything recently and there are pending messages
		if !stats.LastProcessedAt.IsZero() && time.Since(stats.LastProcessedAt) > maxInactivityDuration && stats.LagCount > 0 {
			stats.Healthy = false
			stats.Status = "unhealthy"
			stats.Message = fmt.Sprintf("No processing activity for %v with %d pending messages", time.Since(stats.LastProcessedAt), stats.LagCount)
			return stats
		}
	}

	stats.Healthy = true
	stats.Status = "healthy"
	return stats
}
