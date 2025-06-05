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

// Config holds configuration for the dispatcher service
type Config struct {
	BufferSize       int
	BatchSize        int
	FlushInterval    time.Duration
	RetryAttempts    int
	RetryBackoff     time.Duration
	Plugin           BackendPlugin
	EventTransformer EventTransformer
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
		cfg.EventTransformer = &DefaultEventTransformer{}
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

// Stats returns service statistics
func (s *Service) Stats() (processed, dropped, sent int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.eventsProcessed, s.eventsDropped, s.eventsSent
}
