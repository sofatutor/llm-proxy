package logging

import (
	"context"
	"sync"
	"time"
)

// Sender defines the interface used to send log batches to an external system.
type Sender interface {
	Send(ctx context.Context, batch [][]byte) error
}

// ExternalLogger asynchronously sends logs to an external system.
type ExternalLogger struct {
	enabled       bool
	buffer        chan []byte
	batchSize     int
	flushInterval time.Duration
	sender        Sender
	stop          chan struct{}
	wg            sync.WaitGroup
	// Retry/fallback
	retryInterval   time.Duration
	maxRetries      int
	fallbackToLocal bool
	localFallback   func([][]byte) // Called with batch if fallback is needed
}

// NewExternalLogger creates a new ExternalLogger. If enabled is false, the
// logger is effectively disabled. bufferSize and batchSize must be >0;
// flushInterval controls how often batches are flushed if not full.
// retryInterval and maxRetries control retry policy. fallbackToLocal enables fallback.
func NewExternalLogger(enabled bool, bufferSize, batchSize int, flushInterval, retryInterval time.Duration, maxRetries int, fallbackToLocal bool, sender Sender, localFallback func([][]byte)) *ExternalLogger {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	if batchSize <= 0 {
		batchSize = 10
	}
	if flushInterval <= 0 {
		flushInterval = time.Second
	}
	if retryInterval <= 0 {
		retryInterval = 5 * time.Second
	}
	if maxRetries < 0 {
		maxRetries = 3
	}
	el := &ExternalLogger{
		enabled:         enabled,
		buffer:          make(chan []byte, bufferSize),
		batchSize:       batchSize,
		flushInterval:   flushInterval,
		sender:          sender,
		stop:            make(chan struct{}),
		retryInterval:   retryInterval,
		maxRetries:      maxRetries,
		fallbackToLocal: fallbackToLocal,
		localFallback:   localFallback,
	}
	if enabled && sender != nil {
		el.wg.Add(1)
		go el.run()
	}
	return el
}

// Log queues a log entry for asynchronous delivery. If the buffer is full, the
// entry is dropped.
func (l *ExternalLogger) Log(entry []byte) {
	if !l.enabled {
		return
	}
	select {
	case l.buffer <- entry:
	default:
		// drop when buffer is full
	}
}

// Close stops the worker and flushes any remaining logs.
func (l *ExternalLogger) Close() {
	if !l.enabled {
		return
	}
	close(l.stop)
	l.wg.Wait()
}

func (l *ExternalLogger) run() {
	defer l.wg.Done()

	ticker := time.NewTicker(l.flushInterval)
	defer ticker.Stop()

	batch := make([][]byte, 0, l.batchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		var err error
		for attempt := 0; attempt <= l.maxRetries; attempt++ {
			err = l.sender.Send(context.Background(), batch)
			if err == nil {
				break
			}
			time.Sleep(l.retryInterval)
		}
		if err != nil && l.fallbackToLocal && l.localFallback != nil {
			// Fallback to local logging
			l.localFallback(batch)
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-l.stop:
			flush()
			return
		case entry := <-l.buffer:
			batch = append(batch, entry)
			if len(batch) >= l.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}
