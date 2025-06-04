// Package dispatcher implements the event dispatcher service that subscribes
// to the event bus and delivers events to various backends (file, Helicone, Lunary, etc.).
package dispatcher

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/sofatutor/llm-proxy/internal/eventbus"
)

// Plugin defines the interface that all dispatcher backends must implement.
type Plugin interface {
	// Init initializes the plugin with the given configuration.
	Init(config map[string]string) error

	// Send delivers an event to the backend.
	Send(ctx context.Context, event eventbus.Event) error

	// Name returns the plugin name for identification.
	Name() string
}

// Dispatcher manages event dispatching to various backends.
type Dispatcher struct {
	eventBus  eventbus.EventBus
	plugin    Plugin
	batchSize int
	workers   int
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// Config holds dispatcher configuration.
type Config struct {
	BatchSize int
	Workers   int
}

// New creates a new dispatcher instance.
func New(eventBus eventbus.EventBus, plugin Plugin, config Config) *Dispatcher {
	if config.BatchSize <= 0 {
		config.BatchSize = 10
	}
	if config.Workers <= 0 {
		config.Workers = 1
	}

	return &Dispatcher{
		eventBus:  eventBus,
		plugin:    plugin,
		batchSize: config.BatchSize,
		workers:   config.Workers,
		stopCh:    make(chan struct{}),
	}
}

// Run starts the dispatcher workers and blocks until stopped.
func (d *Dispatcher) Run(ctx context.Context) error {
	log.Printf("Starting dispatcher with plugin: %s", d.plugin.Name())
	log.Printf("Config - batch size: %d, workers: %d", d.batchSize, d.workers)

	eventCh := d.eventBus.Subscribe()

	// Start workers
	for i := 0; i < d.workers; i++ {
		d.wg.Add(1)
		go d.worker(ctx, eventCh, i)
	}

	// Wait for context cancellation or stop signal
	select {
	case <-ctx.Done():
		log.Println("Context cancelled, shutting down dispatcher")
	case <-d.stopCh:
		log.Println("Stop signal received, shutting down dispatcher")
	}

	// Wait for workers to finish
	d.wg.Wait()
	log.Println("Dispatcher shut down complete")

	return nil
}

// Stop gracefully stops the dispatcher.
func (d *Dispatcher) Stop() {
	close(d.stopCh)
}

// worker processes events from the event channel.
func (d *Dispatcher) worker(ctx context.Context, eventCh <-chan eventbus.Event, workerID int) {
	defer d.wg.Done()

	log.Printf("Worker %d started", workerID)
	defer log.Printf("Worker %d stopped", workerID)

	batch := make([]eventbus.Event, 0, d.batchSize)
	ticker := time.NewTicker(5 * time.Second) // Flush batch every 5 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Process remaining batch before exiting
			if len(batch) > 0 {
				d.processBatch(ctx, batch, workerID)
			}
			return
		case <-d.stopCh:
			// Process remaining batch before exiting
			if len(batch) > 0 {
				d.processBatch(ctx, batch, workerID)
			}
			return
		case event, ok := <-eventCh:
			if !ok {
				// Channel closed, process remaining batch
				if len(batch) > 0 {
					d.processBatch(ctx, batch, workerID)
				}
				return
			}

			batch = append(batch, event)

			// Process batch when it's full
			if len(batch) >= d.batchSize {
				d.processBatch(ctx, batch, workerID)
				batch = batch[:0] // Reset batch
			}
		case <-ticker.C:
			// Periodic flush of incomplete batch
			if len(batch) > 0 {
				d.processBatch(ctx, batch, workerID)
				batch = batch[:0] // Reset batch
			}
		}
	}
}

// processBatch sends a batch of events to the plugin.
func (d *Dispatcher) processBatch(ctx context.Context, batch []eventbus.Event, workerID int) {
	log.Printf("Worker %d processing batch of %d events", workerID, len(batch))

	for _, event := range batch {
		if err := d.sendWithRetry(ctx, event); err != nil {
			log.Printf("Worker %d failed to send event %s: %v", workerID, event.RequestID, err)
		}
	}
}

// sendWithRetry sends an event with basic retry logic.
func (d *Dispatcher) sendWithRetry(ctx context.Context, event eventbus.Event) error {
	maxRetries := 3
	backoff := time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				backoff *= 2 // Exponential backoff
			}
		}

		if err := d.plugin.Send(ctx, event); err == nil {
			return nil // Success
		} else if attempt == maxRetries-1 {
			return err // Final attempt failed
		}
	}

	return nil // Should never reach here
}
