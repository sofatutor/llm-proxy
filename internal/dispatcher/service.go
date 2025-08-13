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

	// metrics
	mu              sync.Mutex
	eventsProcessed int64
	eventsDropped   int64
	eventsSent      int64
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
		config:   cfg,
		eventBus: bus,
		logger:   logger,
		stopCh:   make(chan struct{}),
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
		config:   cfg,
		eventBus: bus,
		logger:   logger,
		stopCh:   make(chan struct{}),
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
		s.wg.Wait()

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

// sendBatch sends a batch of events to the configured backend with retry logic
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

		if attempt < s.config.RetryAttempts {
			backoff := time.Duration(attempt+1) * s.config.RetryBackoff
			s.logger.Warn("Failed to send batch, retrying",
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

// sendBatchWithResult sends a batch of events to the configured backend with retry logic and returns the result
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
			backoff := time.Duration(attempt+1) * s.config.RetryBackoff
			s.logger.Warn("Failed to send batch, retrying",
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

// Stats returns service statistics
func (s *Service) Stats() (processed, dropped, sent int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.eventsProcessed, s.eventsDropped, s.eventsSent
}
