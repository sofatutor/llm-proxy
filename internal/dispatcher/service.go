package dispatcher

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/sofatutor/llm-proxy/internal/eventbus"
	"go.uber.org/zap"
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

	// Start event processing goroutine
	s.wg.Add(1)
	// Signal that Add(1) has completed before any potential Stop() Wait
	close(s.startedCh)
	go s.processEvents(ctx)

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

	// Start metrics tracking
	metricsTicker := time.NewTicker(10 * time.Second)
	defer metricsTicker.Stop()
	go s.trackMetrics(ctx, metricsTicker.C)

	sub := s.eventBus.Subscribe()
	batch := make([]EventPayload, 0, s.config.BatchSize)
	ticker := time.NewTicker(s.config.FlushInterval)
	defer ticker.Stop()

	// Detect if Subscribe returns a closed channel (log-based bus)
	closed := false
	select {
	case _, ok := <-sub:
		if !ok {
			closed = true
		}
	default:
	}

	if closed {
		// Log-based consumption: poll for new events using ReadEvents
		// Persist last-seen LogID in Redis for this dispatcher
		client, ok := s.eventBus.(*eventbus.RedisEventBus)
		if !ok {
			s.logger.Error("eventBus is not RedisEventBus; cannot persist offset")
			return
		}
		redisClient := client.Client()
		// Use the dispatcher type (plugin name) for offset key: one dispatcher per type
		serviceType := s.config.PluginName
		if serviceType == "" {
			serviceType = "default"
		}
		dispatcherKey := "llm-proxy-dispatcher:" + serviceType + ":last_id"
		// Read last-seen LogID from Redis
		var lastSeenID int64 = 0
		if val, err := redisClient.Get(ctx, dispatcherKey); err == nil && val != "" {
			if id, err := strconv.ParseInt(val, 10, 64); err == nil {
				lastSeenID = id
			}
		}
		for {
			select {
			case <-ticker.C:
				ctxPoll, cancel := context.WithTimeout(ctx, s.config.FlushInterval)
				defer cancel()
				events, err := s.eventBus.(*eventbus.RedisEventBus).ReadEvents(ctxPoll, 0, -1)
				if err != nil {
					s.logger.Error("Failed to read events from log", zap.Error(err))
					continue
				}
				// Filter events with LogID > lastSeenID
				newEvents := make([]eventbus.Event, 0)
				for _, evt := range events {
					if evt.LogID > lastSeenID {
						newEvents = append(newEvents, evt)
					}
				}
				// Reverse newEvents to process from oldest to newest
				for i, j := 0, len(newEvents)-1; i < j; i, j = i+1, j-1 {
					newEvents[i], newEvents[j] = newEvents[j], newEvents[i]
				}
				if len(newEvents) > 0 {
					if newEvents[0].LogID > lastSeenID+1 {
						s.logger.Warn("Missed events due to TTL or trimming", zap.Int64("last_seen_id", lastSeenID), zap.Int64("first_log_id", newEvents[0].LogID))
					}
					// Prepare batch and track maxLogID in this batch
					maxLogID := lastSeenID
					batch = batch[:0]
					for _, evt := range newEvents {
						payload, err := s.config.EventTransformer.Transform(evt)
						if err != nil {
							s.mu.Lock()
							s.eventsDropped++
							s.mu.Unlock()
							s.logger.Error("Failed to transform event", zap.Error(err))
							continue
						}
						if payload == nil {
							continue
						}
						batch = append(batch, *payload)
						s.mu.Lock()
						s.eventsProcessed++
						s.lastProcessedAt = time.Now()
						s.mu.Unlock()
						if evt.LogID > maxLogID {
							maxLogID = evt.LogID
						}
					}
					if len(batch) > 0 {
						// Only update/persist lastSeenID if sendBatch succeeds
						err := s.sendBatchWithResult(ctx, batch)
						if err == nil {
							lastSeenID = maxLogID
							_ = redisClient.Set(ctx, dispatcherKey, strconv.FormatInt(lastSeenID, 10))
						}
						batch = batch[:0]
					}
				}
			case <-s.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}

	// Channel-based (in-memory) event bus
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
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
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
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
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

// trackMetrics periodically updates metrics like processing rate and lag
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

// HealthStatus represents the health status of the dispatcher
type HealthStatus struct {
	Healthy         bool      `json:"healthy"`
	Status          string    `json:"status"`
	EventsProcessed int64     `json:"events_processed"`
	EventsDropped   int64     `json:"events_dropped"`
	EventsSent      int64     `json:"events_sent"`
	ProcessingRate  float64   `json:"processing_rate"`
	LagCount        int64     `json:"lag_count"`
	StreamLength    int64     `json:"stream_length"`
	LastProcessedAt time.Time `json:"last_processed_at"`
	Message         string    `json:"message,omitempty"`
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

		// Consider unhealthy if lag is very high (>10000 messages)
		if stats.LagCount > 10000 {
			stats.Healthy = false
			stats.Status = "unhealthy"
			stats.Message = fmt.Sprintf("High lag: %d pending messages", stats.LagCount)
			return stats
		}

		// Consider unhealthy if we haven't processed anything in the last 5 minutes and there are pending messages
		if !stats.LastProcessedAt.IsZero() && time.Since(stats.LastProcessedAt) > 5*time.Minute && stats.LagCount > 0 {
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
