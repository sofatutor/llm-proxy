package dispatcher

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/eventbus"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// test-scoped mock Redis client implementing eventbus.RedisClient for Redis log bus
type mockRedisClientLog struct {
	list  [][]byte
	ttl   time.Duration
	seq   map[string]int64
	store map[string]string
}

func newMockRedisClientLog() *mockRedisClientLog {
	return &mockRedisClientLog{list: make([][]byte, 0), seq: make(map[string]int64), store: make(map[string]string)}
}

func (m *mockRedisClientLog) LPush(_ context.Context, _ string, values ...interface{}) error {
	for _, v := range values {
		var b []byte
		switch val := v.(type) {
		case string:
			b = []byte(val)
		case []byte:
			b = val
		}
		m.list = append([][]byte{b}, m.list...)
	}
	return nil
}

func (m *mockRedisClientLog) LRANGE(_ context.Context, _ string, start, stop int64) ([]string, error) {
	ln := int64(len(m.list))
	if ln == 0 {
		return []string{}, nil
	}
	if start < 0 {
		start = ln + start
	}
	if stop < 0 {
		stop = ln + stop
	}
	if start < 0 {
		start = 0
	}
	if stop >= ln {
		stop = ln - 1
	}
	if start > stop || start >= ln {
		return []string{}, nil
	}
	out := make([]string, 0, stop-start+1)
	for i := start; i <= stop; i++ {
		out = append(out, string(m.list[i]))
	}
	return out, nil
}

func (m *mockRedisClientLog) LLEN(_ context.Context, _ string) (int64, error) {
	return int64(len(m.list)), nil
}
func (m *mockRedisClientLog) EXPIRE(_ context.Context, _ string, expiration time.Duration) error {
	m.ttl = expiration
	return nil
}
func (m *mockRedisClientLog) LTRIM(_ context.Context, _ string, start, stop int64) error {
	ln := int64(len(m.list))
	if start < 0 {
		start = ln + start
	}
	if stop < 0 {
		stop = ln + stop
	}
	if start < 0 {
		start = 0
	}
	if stop >= ln {
		stop = ln - 1
	}
	if start > stop || start >= ln {
		m.list = [][]byte{}
		return nil
	}
	m.list = m.list[start : stop+1]
	return nil
}
func (m *mockRedisClientLog) Incr(_ context.Context, key string) (int64, error) {
	if m.seq == nil {
		m.seq = make(map[string]int64)
	}
	m.seq[key]++
	return m.seq[key], nil
}
func (m *mockRedisClientLog) Get(_ context.Context, key string) (string, error) {
	return m.store[key], nil
}
func (m *mockRedisClientLog) Set(_ context.Context, key, value string) error {
	if m.store == nil {
		m.store = make(map[string]string)
	}
	m.store[key] = value
	return nil
}

// mockPlugin is a mock implementation of BackendPlugin for testing
type mockPlugin struct {
	mu       sync.Mutex
	events   [][]EventPayload
	initCfg  map[string]string
	sendErr  error
	closeErr error
	OnSend   func([]EventPayload) error
}

func (m *mockPlugin) Init(cfg map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.initCfg = cfg
	return nil
}

func (m *mockPlugin) SendEvents(ctx context.Context, events []EventPayload) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sendErr != nil {
		return m.sendErr
	}
	m.events = append(m.events, events)
	if m.OnSend != nil {
		return m.OnSend(events)
	}
	return nil
}

func (m *mockPlugin) Close() error {
	return m.closeErr
}

func (m *mockPlugin) getEvents() [][]EventPayload {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Return a copy to avoid races
	result := make([][]EventPayload, len(m.events))
	copy(result, m.events)
	return result
}

func TestNewService(t *testing.T) {
	plugin := &mockPlugin{}
	logger := zaptest.NewLogger(t)

	// Test with valid config
	cfg := Config{
		Plugin: plugin,
	}

	service, err := NewService(cfg, logger)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	if service == nil {
		t.Fatal("Expected non-nil service")
	}

	// Test defaults are applied
	if service.config.BufferSize != 1000 {
		t.Errorf("Expected BufferSize 1000, got %d", service.config.BufferSize)
	}

	if service.config.BatchSize != 100 {
		t.Errorf("Expected BatchSize 100, got %d", service.config.BatchSize)
	}

	// Test without plugin
	cfg.Plugin = nil
	_, err = NewService(cfg, logger)
	if err == nil {
		t.Fatal("Expected error when plugin is nil")
	}
}

func TestServiceProcessEvents(t *testing.T) {
	plugin := &mockPlugin{}
	logger := zaptest.NewLogger(t)

	cfg := Config{
		Plugin:        plugin,
		BufferSize:    10,
		BatchSize:     2,
		FlushInterval: 50 * time.Millisecond,
	}

	service, err := NewService(cfg, logger)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Start processing in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		service.processEvents(ctx)
	}()

	// Give the goroutine time to start
	time.Sleep(10 * time.Millisecond)

	// Send some events
	bus := service.EventBus()
	bus.Publish(context.Background(), eventbus.Event{
		RequestID: "test1",
		Method:    "POST",
		Path:      "/test",
		Status:    200,
	})

	bus.Publish(context.Background(), eventbus.Event{
		RequestID: "test2",
		Method:    "POST",
		Path:      "/test",
		Status:    200,
	})

	// Wait for events to be processed with exponential backoff
	maxWait := 2 * time.Second
	waitTime := 10 * time.Millisecond
	start := time.Now()

	for time.Since(start) < maxWait {
		events := plugin.getEvents()
		if len(events) > 0 {
			break
		}
		time.Sleep(waitTime)
		if waitTime < 100*time.Millisecond {
			waitTime *= 2
		}
	}

	events := plugin.getEvents()

	// Check that events were sent
	if len(events) == 0 {
		t.Fatal("Expected events to be sent to plugin")
	}

	if len(events[0]) != 2 {
		t.Fatalf("Expected batch size 2, got %d", len(events[0]))
	}

	// Stop service
	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestServiceRun(t *testing.T) {
	plugin := &mockPlugin{}
	logger := zaptest.NewLogger(t)

	cfg := Config{
		Plugin:     plugin,
		BufferSize: 10,
		BatchSize:  2,
	}

	service, err := NewService(cfg, logger)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Test foreground run (should return quickly when context is cancelled)
	ctx, cancel := context.WithCancel(context.Background())

	// Start Run in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- service.Run(ctx, false) // foreground mode
	}()

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	cancel()

	// Should return without error
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}

func TestServiceRunDetached(t *testing.T) {
	plugin := &mockPlugin{}
	logger := zaptest.NewLogger(t)

	cfg := Config{
		Plugin:     plugin,
		BufferSize: 10,
		BatchSize:  2,
	}

	service, err := NewService(cfg, logger)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Test detached run (should block until context is cancelled)
	ctx, cancel := context.WithCancel(context.Background())

	// Start Run in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- service.Run(ctx, true) // detached mode
	}()

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Cancel context to stop it
	cancel()

	// Should return without error
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Run in detached mode returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}

func TestServiceStop(t *testing.T) {
	plugin := &mockPlugin{}
	logger := zaptest.NewLogger(t)

	cfg := Config{
		Plugin:     plugin,
		BufferSize: 10,
		BatchSize:  2,
	}

	service, err := NewService(cfg, logger)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Stop should work even if not started
	err = service.Stop()
	if err != nil {
		t.Errorf("Stop returned error: %v", err)
	}

	// Multiple stops should be safe
	err = service.Stop()
	if err != nil {
		t.Errorf("Multiple Stop calls should be safe: %v", err)
	}
	err = service.Stop()
	if err != nil {
		t.Errorf("Multiple Stop calls should be safe: %v", err)
	}
}

func TestServiceStats(t *testing.T) {
	plugin := &mockPlugin{}
	logger := zaptest.NewLogger(t)

	cfg := Config{
		Plugin:     plugin,
		BufferSize: 10,
		BatchSize:  2,
	}

	service, err := NewService(cfg, logger)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Get initial stats
	processed, dropped, sent := service.Stats()
	if processed != 0 {
		t.Errorf("Expected processed 0, got %d", processed)
	}
	if dropped != 0 {
		t.Errorf("Expected dropped 0, got %d", dropped)
	}
	if sent != 0 {
		t.Errorf("Expected sent 0, got %d", sent)
	}
}

func TestServiceSendBatchErrors(t *testing.T) {
	plugin := &mockPlugin{
		sendErr: fmt.Errorf("mock send error"),
	}
	logger := zaptest.NewLogger(t)

	cfg := Config{
		Plugin:        plugin,
		BufferSize:    10,
		BatchSize:     2,
		RetryAttempts: 2,
		RetryBackoff:  10 * time.Millisecond,
	}

	service, err := NewService(cfg, logger)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Start processing in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		service.processEvents(ctx)
	}()

	// Give the goroutine time to start
	time.Sleep(10 * time.Millisecond)

	// Send events that will fail to send
	bus := service.EventBus()
	bus.Publish(context.Background(), eventbus.Event{
		RequestID: "test1",
		Method:    "POST",
		Path:      "/test",
		Status:    200,
	})

	bus.Publish(context.Background(), eventbus.Event{
		RequestID: "test2",
		Method:    "POST",
		Path:      "/test",
		Status:    200,
	})

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Check stats for errors - sendErr will cause events to be dropped
	_, dropped, _ := service.Stats()
	if dropped == 0 {
		t.Error("Expected dropped > 0 due to send errors")
	}

	// Stop service
	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestServiceSendBatchWithResult_PermanentError(t *testing.T) {
	plugin := &mockPlugin{sendErr: &PermanentBackendError{Msg: "perm"}}
	cfg := Config{Plugin: plugin, RetryAttempts: 2, RetryBackoff: time.Millisecond}
	bus := eventbus.NewInMemoryEventBus(10)
	svc, err := NewServiceWithBus(cfg, zaptest.NewLogger(t), bus)
	if err != nil {
		t.Fatalf("NewServiceWithBus failed: %v", err)
	}
	err = svc.sendBatchWithResult(context.Background(), []EventPayload{{RunID: "r"}})
	if err != nil {
		t.Fatalf("expected nil error for permanent backend error, got %v", err)
	}
}

func TestServiceStatsGetter(t *testing.T) {
	cfg := Config{Plugin: &mockPlugin{}}
	bus := eventbus.NewInMemoryEventBus(10)
	svc, err := NewServiceWithBus(cfg, zaptest.NewLogger(t), bus)
	if err != nil {
		t.Fatalf("NewServiceWithBus failed: %v", err)
	}
	// Increment internal counters via sendBatch
	svc.mu.Lock()
	svc.eventsProcessed = 3
	svc.eventsDropped = 1
	svc.eventsSent = 2
	svc.mu.Unlock()
	p, d, s := svc.Stats()
	if p != 3 || d != 1 || s != 2 {
		t.Fatalf("unexpected stats p=%d d=%d s=%d", p, d, s)
	}
}

func TestServiceSendBatchWithResult_RetryBackoff(t *testing.T) {
	attempts := 0
	plugin := &mockPlugin{OnSend: func(_ []EventPayload) error {
		attempts++
		if attempts < 2 {
			return fmt.Errorf("temp fail")
		}
		return nil
	}}
	cfg := Config{Plugin: plugin, RetryAttempts: 3, RetryBackoff: 5 * time.Millisecond}
	bus := eventbus.NewInMemoryEventBus(10)
	svc, err := NewServiceWithBus(cfg, zaptest.NewLogger(t), bus)
	if err != nil {
		t.Fatalf("NewServiceWithBus failed: %v", err)
	}
	err = svc.sendBatchWithResult(context.Background(), []EventPayload{{RunID: "r"}})
	if err != nil {
		t.Fatalf("expected success after retry, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestService_TimerFlushSendsBatch(t *testing.T) {
	// Plugin that records sends
	type recordPlugin struct {
		mu    sync.Mutex
		sends int
	}
	var rp recordPlugin
	done := make(chan struct{}, 1)
	var once sync.Once
	plugin := &mockPlugin{OnSend: func(_ []EventPayload) error {
		rp.mu.Lock()
		rp.sends++
		rp.mu.Unlock()
		once.Do(func() { done <- struct{}{} })
		return nil
	}}

	cfg := Config{
		Plugin:           plugin,
		EventTransformer: NewDefaultEventTransformer(false),
		BufferSize:       10,
		BatchSize:        10,                    // large so timer triggers flush
		FlushInterval:    10 * time.Millisecond, // short timer
		RetryAttempts:    1,
		RetryBackoff:     time.Millisecond,
	}
	bus := eventbus.NewInMemoryEventBus(10)
	svc, err := NewServiceWithBus(cfg, zaptest.NewLogger(t), bus)
	if err != nil {
		t.Fatalf("NewServiceWithBus failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Wait for service goroutine to complete to avoid race on logger
	serviceComplete := make(chan struct{})
	go func() {
		defer close(serviceComplete)
		_ = svc.Run(ctx, false)
	}()

	// Give Run/processEvents time to subscribe to the bus before publishing
	time.Sleep(20 * time.Millisecond)

	// Publish a single event and rely on timer flush
	bus.Publish(context.Background(), eventbus.Event{RequestID: "flush", Method: "POST"})

	// Wait until a send occurs or timeout
	select {
	case <-done:
		// ok
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for timer flush send")
	}
	cancel()
	_ = svc.Stop()
	<-serviceComplete // Wait for service goroutine to complete

	rp.mu.Lock()
	sends := rp.sends
	rp.mu.Unlock()
	if sends == 0 {
		t.Fatalf("expected at least one send via timer flush, got %d", sends)
	}
}

func TestService_StopIdempotent(t *testing.T) {
	cfg := Config{
		Plugin:           &mockPlugin{},
		EventTransformer: NewDefaultEventTransformer(false),
		BufferSize:       2,
		BatchSize:        1,
		FlushInterval:    time.Millisecond,
	}
	bus := eventbus.NewInMemoryEventBus(2)
	svc, err := NewServiceWithBus(cfg, zap.NewNop(), bus)
	if err != nil {
		t.Fatalf("NewServiceWithBus failed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())

	// Wait for service goroutine to complete to avoid race on logger
	serviceComplete := make(chan struct{})
	go func() {
		defer close(serviceComplete)
		_ = svc.Run(ctx, false)
	}()

	cancel()
	if err := svc.Stop(); err != nil {
		t.Fatalf("first Stop err: %v", err)
	}
	<-serviceComplete // Wait for service goroutine to complete

	if err := svc.Stop(); err != nil {
		t.Fatalf("second Stop err: %v", err)
	}
}

func TestDispatcher_DoesNotDispatchDuplicates(t *testing.T) {
	client := newMockRedisClientLog()
	bus := eventbus.NewRedisEventBusLog(client, "events", 0, 0)
	plugin := &mockPlugin{}
	logger := zaptest.NewLogger(t)

	cfg := Config{
		Plugin:           plugin,
		PluginName:       "file",
		EventTransformer: NewDefaultEventTransformer(false),
		FlushInterval:    10 * time.Millisecond,
		BatchSize:        10,
	}

	service, err := NewServiceWithBus(cfg, logger, bus)
	if err != nil {
		t.Fatalf("NewServiceWithBus failed: %v", err)
	}

	// Publish 100 events
	n := 100
	for i := 0; i < n; i++ {
		bus.Publish(context.Background(), eventbus.Event{RequestID: fmt.Sprintf("req-%d", i), Method: "POST"})
	}

	dispatched := make(map[int64]struct{})
	var dispatchedMu sync.Mutex
	plugin.OnSend = func(batch []EventPayload) error {
		dispatchedMu.Lock()
		defer dispatchedMu.Unlock()
		for _, evt := range batch {
			logID := evt.LogID
			if _, exists := dispatched[logID]; exists {
				t.Fatalf("Duplicate dispatch of LogID %d", logID)
			}
			dispatched[logID] = struct{}{}
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Wait for service goroutine to complete to avoid race on logger
	serviceComplete := make(chan struct{})
	go func() {
		defer close(serviceComplete)
		_ = service.Run(ctx, false)
	}()

	// Wait for all events to be dispatched
	for i := 0; i < 100; i++ {
		time.Sleep(10 * time.Millisecond)
		dispatchedMu.Lock()
		cnt := len(dispatched)
		dispatchedMu.Unlock()
		if cnt == n {
			break
		}
	}
	_ = service.Stop()
	<-serviceComplete // Wait for service goroutine to complete

	dispatchedMu.Lock()
	finalCnt := len(dispatched)
	dispatchedMu.Unlock()
	if finalCnt != n {
		t.Fatalf("Expected %d unique events dispatched, got %d", n, finalCnt)
	}
}
