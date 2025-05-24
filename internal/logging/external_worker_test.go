package logging

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeSender struct {
	mu    sync.Mutex
	calls [][]string
}

func (f *fakeSender) Send(ctx context.Context, batch [][]byte) error {
	strs := make([]string, len(batch))
	for i, b := range batch {
		strs[i] = string(b)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, strs)
	return nil
}

type failSender struct {
	failures int
	calls    [][]string
}

func (f *failSender) Send(ctx context.Context, batch [][]byte) error {
	if f.failures > 0 {
		f.failures--
		return assert.AnError
	}
	strs := make([]string, len(batch))
	for i, b := range batch {
		strs[i] = string(b)
	}
	f.calls = append(f.calls, strs)
	return nil
}

func TestExternalLogger_Batching(t *testing.T) {
	fs := &fakeSender{}
	logger := NewExternalLogger(true, 5, 2, 50*time.Millisecond, 5*time.Millisecond, 3, false, fs, nil)

	logger.Log([]byte("a"))
	logger.Log([]byte("b"))
	logger.Log([]byte("c"))

	time.Sleep(120 * time.Millisecond)
	logger.Close()

	fs.mu.Lock()
	defer fs.mu.Unlock()
	assert.Len(t, fs.calls, 2)
	assert.Equal(t, []string{"a", "b"}, fs.calls[0])
	assert.Equal(t, []string{"c"}, fs.calls[1])
}

func TestExternalLogger_BufferOverflow(t *testing.T) {
	fs := &fakeSender{}
	logger := NewExternalLogger(true, 2, 2, 50*time.Millisecond, 5*time.Millisecond, 3, false, fs, nil)

	logger.Log([]byte("1"))
	logger.Log([]byte("2"))
	logger.Log([]byte("3")) // should be dropped

	time.Sleep(60 * time.Millisecond)
	logger.Close()

	fs.mu.Lock()
	defer fs.mu.Unlock()
	assert.Len(t, fs.calls, 1)
	assert.Equal(t, []string{"1", "2"}, fs.calls[0])
}

func TestExternalLogger_RetryAndFallback(t *testing.T) {
	fs := &failSender{failures: 5} // More failures than max retries to ensure fallback
	var fallbackBatches []string
	fallbackCh := make(chan struct{}, 1)
	logger := NewExternalLogger(true, 5, 2, 10*time.Millisecond, 5*time.Millisecond, 2, true, fs, func(batch [][]byte) {
		for _, b := range batch {
			fallbackBatches = append(fallbackBatches, string(b))
		}
		fallbackCh <- struct{}{}
	})

	logger.Log([]byte("x"))
	logger.Log([]byte("y"))

	time.Sleep(30 * time.Millisecond) // Allow flush interval to trigger
	logger.Close()

	select {
	case <-fallbackCh:
		// fallback called
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for fallback to be called")
	}

	assert.GreaterOrEqual(t, len(fallbackBatches), 2)
	assert.Contains(t, fallbackBatches, "x")
	assert.Contains(t, fallbackBatches, "y")
}
