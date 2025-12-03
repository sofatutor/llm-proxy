package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// mockRedisStreamsClient implements RedisStreamsClient for testing.
type mockRedisStreamsClient struct {
	mu             sync.Mutex
	stream         []mockStreamMessage
	groups         map[string]*mockConsumerGroup
	pendingMsgs    map[string]map[string]*mockPendingMessage // group -> id -> pending
	nextID         int64
	xAddErr        error
	xReadGroupErr  error
	xAckErr        error
	xGroupErr      error
	xPendingErr    error
	xPendingExtErr error
	xClaimErr      error
	xLenErr        error
	xInfoGroupsErr error
}

type mockStreamMessage struct {
	ID     string
	Values map[string]interface{}
}

type mockConsumerGroup struct {
	name          string
	lastDelivered string
	consumers     map[string]*mockConsumer
}

type mockConsumer struct {
	name string
}

type mockPendingMessage struct {
	id           string
	consumer     string
	deliveryTime time.Time
	deliverCount int64
}

func newMockRedisStreamsClient() *mockRedisStreamsClient {
	return &mockRedisStreamsClient{
		stream:      make([]mockStreamMessage, 0),
		groups:      make(map[string]*mockConsumerGroup),
		pendingMsgs: make(map[string]map[string]*mockPendingMessage),
	}
}

func (m *mockRedisStreamsClient) XAdd(ctx context.Context, args *redis.XAddArgs) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.xAddErr != nil {
		return "", m.xAddErr
	}

	m.nextID++
	id := fmt.Sprintf("0-%d", m.nextID)

	// Convert Values to map[string]interface{}
	var values map[string]interface{}
	switch v := args.Values.(type) {
	case map[string]interface{}:
		values = v
	default:
		values = make(map[string]interface{})
	}

	m.stream = append(m.stream, mockStreamMessage{
		ID:     id,
		Values: values,
	})

	// Apply MaxLen
	if args.MaxLen > 0 && int64(len(m.stream)) > args.MaxLen {
		m.stream = m.stream[len(m.stream)-int(args.MaxLen):]
	}

	return id, nil
}

func (m *mockRedisStreamsClient) XReadGroup(ctx context.Context, args *redis.XReadGroupArgs) ([]redis.XStream, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.xReadGroupErr != nil {
		return nil, m.xReadGroupErr
	}

	group, ok := m.groups[args.Group]
	if !ok {
		return nil, errors.New("NOGROUP No such key or consumer group")
	}

	// Ensure consumer exists
	if group.consumers == nil {
		group.consumers = make(map[string]*mockConsumer)
	}
	if _, ok := group.consumers[args.Consumer]; !ok {
		group.consumers[args.Consumer] = &mockConsumer{name: args.Consumer}
	}

	// Check if reading pending ("0") or new (">")
	startID := args.Streams[1]
	var messages []redis.XMessage

	if startID == "0" {
		// Reading pending messages for this consumer
		pending := m.pendingMsgs[args.Group]
		if pending != nil {
			for _, msg := range m.stream {
				if p, ok := pending[msg.ID]; ok && p.consumer == args.Consumer {
					messages = append(messages, redis.XMessage{
						ID:     msg.ID,
						Values: msg.Values,
					})
					if args.Count > 0 && int64(len(messages)) >= args.Count {
						break
					}
				}
			}
		}
	} else if startID == ">" {
		// Reading new messages
		for _, msg := range m.stream {
			// Skip already delivered
			if msg.ID <= group.lastDelivered {
				continue
			}

			messages = append(messages, redis.XMessage{
				ID:     msg.ID,
				Values: msg.Values,
			})

			// Track as pending
			if m.pendingMsgs[args.Group] == nil {
				m.pendingMsgs[args.Group] = make(map[string]*mockPendingMessage)
			}
			m.pendingMsgs[args.Group][msg.ID] = &mockPendingMessage{
				id:           msg.ID,
				consumer:     args.Consumer,
				deliveryTime: time.Now(),
				deliverCount: 1,
			}

			group.lastDelivered = msg.ID

			if args.Count > 0 && int64(len(messages)) >= args.Count {
				break
			}
		}
	}

	if len(messages) == 0 {
		return nil, redis.Nil
	}

	return []redis.XStream{
		{
			Stream:   args.Streams[0],
			Messages: messages,
		},
	}, nil
}

func (m *mockRedisStreamsClient) XAck(ctx context.Context, stream, group string, ids ...string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.xAckErr != nil {
		return 0, m.xAckErr
	}

	var count int64
	pending := m.pendingMsgs[group]
	if pending != nil {
		for _, id := range ids {
			if _, ok := pending[id]; ok {
				delete(pending, id)
				count++
			}
		}
	}
	return count, nil
}

func (m *mockRedisStreamsClient) XGroupCreateMkStream(ctx context.Context, stream, group, start string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.xGroupErr != nil {
		return m.xGroupErr
	}

	if _, exists := m.groups[group]; exists {
		return errors.New("BUSYGROUP Consumer Group name already exists")
	}

	m.groups[group] = &mockConsumerGroup{
		name:          group,
		lastDelivered: start,
		consumers:     make(map[string]*mockConsumer),
	}
	return nil
}

func (m *mockRedisStreamsClient) XPending(ctx context.Context, stream, group string) (*redis.XPending, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.xPendingErr != nil {
		return nil, m.xPendingErr
	}

	pending := m.pendingMsgs[group]
	count := int64(0)
	if pending != nil {
		count = int64(len(pending))
	}

	return &redis.XPending{
		Count: count,
	}, nil
}

func (m *mockRedisStreamsClient) XPendingExt(ctx context.Context, args *redis.XPendingExtArgs) ([]redis.XPendingExt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.xPendingExtErr != nil {
		return nil, m.xPendingExtErr
	}

	var result []redis.XPendingExt
	pending := m.pendingMsgs[args.Group]
	for _, p := range pending {
		result = append(result, redis.XPendingExt{
			ID:         p.id,
			Consumer:   p.consumer,
			Idle:       time.Since(p.deliveryTime),
			RetryCount: p.deliverCount,
		})
		if args.Count > 0 && int64(len(result)) >= args.Count {
			break
		}
	}
	return result, nil
}

func (m *mockRedisStreamsClient) XClaim(ctx context.Context, args *redis.XClaimArgs) ([]redis.XMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.xClaimErr != nil {
		return nil, m.xClaimErr
	}

	var messages []redis.XMessage
	pending := m.pendingMsgs[args.Group]

	for _, id := range args.Messages {
		if p, ok := pending[id]; ok {
			// Check idle time
			if time.Since(p.deliveryTime) < args.MinIdle {
				continue
			}

			// Find message in stream
			for _, msg := range m.stream {
				if msg.ID == id {
					messages = append(messages, redis.XMessage{
						ID:     msg.ID,
						Values: msg.Values,
					})

					// Update pending entry
					p.consumer = args.Consumer
					p.deliveryTime = time.Now()
					p.deliverCount++
					break
				}
			}
		}
	}
	return messages, nil
}

func (m *mockRedisStreamsClient) XLen(ctx context.Context, stream string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.xLenErr != nil {
		return 0, m.xLenErr
	}

	return int64(len(m.stream)), nil
}

func (m *mockRedisStreamsClient) XInfoGroups(ctx context.Context, stream string) ([]redis.XInfoGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.xInfoGroupsErr != nil {
		return nil, m.xInfoGroupsErr
	}

	var groups []redis.XInfoGroup
	for name, g := range m.groups {
		groups = append(groups, redis.XInfoGroup{
			Name:            name,
			Consumers:       int64(len(g.consumers)),
			Pending:         int64(len(m.pendingMsgs[name])),
			LastDeliveredID: g.lastDelivered,
		})
	}
	return groups, nil
}

// --- Tests ---

func TestRedisStreamsEventBus_Publish(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()
	bus := NewRedisStreamsEventBus(client, config)

	ctx := context.Background()
	evt := Event{RequestID: "test-123", Method: "POST", Path: "/v1/chat/completions", Status: 200}

	bus.Publish(ctx, evt)

	// Verify message was added
	if len(client.stream) != 1 {
		t.Fatalf("expected 1 message in stream, got %d", len(client.stream))
	}

	// Verify stats
	pub, drop := bus.Stats()
	if pub != 1 {
		t.Errorf("expected 1 published, got %d", pub)
	}
	if drop != 0 {
		t.Errorf("expected 0 dropped, got %d", drop)
	}

	// Verify message content
	data := client.stream[0].Values["data"].(string)
	var parsedEvt Event
	if err := json.Unmarshal([]byte(data), &parsedEvt); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}
	if parsedEvt.RequestID != evt.RequestID {
		t.Errorf("expected RequestID %s, got %s", evt.RequestID, parsedEvt.RequestID)
	}
}

func TestRedisStreamsEventBus_Publish_Error(t *testing.T) {
	client := newMockRedisStreamsClient()
	client.xAddErr = errors.New("connection error")

	config := DefaultRedisStreamsConfig()
	bus := NewRedisStreamsEventBus(client, config)

	ctx := context.Background()
	bus.Publish(ctx, Event{RequestID: "test"})

	pub, drop := bus.Stats()
	if pub != 0 {
		t.Errorf("expected 0 published on error, got %d", pub)
	}
	if drop != 1 {
		t.Errorf("expected 1 dropped on error, got %d", drop)
	}
}

func TestRedisStreamsEventBus_Publish_MaxLen(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()
	config.MaxLen = 3
	bus := NewRedisStreamsEventBus(client, config)

	ctx := context.Background()

	// Publish 5 events
	for i := 0; i < 5; i++ {
		bus.Publish(ctx, Event{RequestID: fmt.Sprintf("r%d", i)})
	}

	// Stream should be trimmed to MaxLen
	if len(client.stream) != 3 {
		t.Errorf("expected stream length 3 after trim, got %d", len(client.stream))
	}
}

func TestRedisStreamsEventBus_EnsureConsumerGroup(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()
	bus := NewRedisStreamsEventBus(client, config)

	ctx := context.Background()

	// First call creates group
	err := bus.EnsureConsumerGroup(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second call should be a no-op (group already created)
	err = bus.EnsureConsumerGroup(ctx)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}

	// Verify group exists
	if _, exists := client.groups[config.ConsumerGroup]; !exists {
		t.Error("consumer group was not created")
	}
}

func TestRedisStreamsEventBus_EnsureConsumerGroup_AlreadyExists(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()

	// Pre-create the group
	client.groups[config.ConsumerGroup] = &mockConsumerGroup{
		name:      config.ConsumerGroup,
		consumers: make(map[string]*mockConsumer),
	}

	bus := NewRedisStreamsEventBus(client, config)
	ctx := context.Background()

	// Should succeed even though group exists
	err := bus.EnsureConsumerGroup(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRedisStreamsEventBus_EnsureConsumerGroup_Error(t *testing.T) {
	client := newMockRedisStreamsClient()
	client.xGroupErr = errors.New("some redis error")

	config := DefaultRedisStreamsConfig()
	bus := NewRedisStreamsEventBus(client, config)

	ctx := context.Background()
	err := bus.EnsureConsumerGroup(ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRedisStreamsEventBus_Subscribe_And_Consume(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()
	config.BlockTimeout = 10 * time.Millisecond // Short timeout for test
	config.BatchSize = 10
	bus := NewRedisStreamsEventBus(client, config)

	ctx := context.Background()

	// Publish events before subscribing
	for i := 0; i < 3; i++ {
		bus.Publish(ctx, Event{RequestID: fmt.Sprintf("evt-%d", i)})
	}

	// Subscribe
	ch := bus.Subscribe()

	// Collect received events
	var received []Event
	timeout := time.After(500 * time.Millisecond)

	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				goto done
			}
			received = append(received, evt)
			if len(received) >= 3 {
				goto done
			}
		case <-timeout:
			goto done
		}
	}

done:
	bus.Stop()

	if len(received) != 3 {
		t.Fatalf("expected 3 events, got %d", len(received))
	}
}

func TestRedisStreamsEventBus_Subscribe_AcknowledgesMessages(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()
	config.BlockTimeout = 10 * time.Millisecond
	bus := NewRedisStreamsEventBus(client, config)

	ctx := context.Background()

	// Publish event
	bus.Publish(ctx, Event{RequestID: "ack-test"})

	// Subscribe and consume
	ch := bus.Subscribe()

	select {
	case <-ch:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}

	// Give time for ack
	time.Sleep(50 * time.Millisecond)

	// Check pending count - should be 0 after ack
	pendingCount, err := bus.PendingCount(ctx)
	if err != nil {
		t.Fatalf("error getting pending count: %v", err)
	}

	bus.Stop()

	if pendingCount != 0 {
		t.Errorf("expected 0 pending after ack, got %d", pendingCount)
	}
}

func TestRedisStreamsEventBus_Stop(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()
	config.BlockTimeout = time.Second
	bus := NewRedisStreamsEventBus(client, config)

	// Subscribe to start the consume loop
	ch := bus.Subscribe()

	// Give time for goroutine to start
	time.Sleep(50 * time.Millisecond)

	// Stop should close channels and wait for goroutines
	done := make(chan struct{})
	go func() {
		bus.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Good
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not complete in time")
	}

	// Channel should be closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to be closed")
		}
	default:
		// Channel might already be drained
	}
}

func TestRedisStreamsEventBus_Stop_Multiple(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()
	bus := NewRedisStreamsEventBus(client, config)

	// Multiple Stop calls should be safe
	bus.Stop()
	bus.Stop()
	bus.Stop()
}

func TestRedisStreamsEventBus_StreamLength(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()
	bus := NewRedisStreamsEventBus(client, config)

	ctx := context.Background()

	// Initially empty
	length, err := bus.StreamLength(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if length != 0 {
		t.Errorf("expected length 0, got %d", length)
	}

	// Publish some events
	for i := 0; i < 5; i++ {
		bus.Publish(ctx, Event{RequestID: fmt.Sprintf("r%d", i)})
	}

	length, err = bus.StreamLength(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if length != 5 {
		t.Errorf("expected length 5, got %d", length)
	}
}

func TestRedisStreamsEventBus_Client(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()
	bus := NewRedisStreamsEventBus(client, config)

	if bus.Client() != client {
		t.Error("Client() did not return underlying client")
	}
}

func TestRedisStreamsEventBus_Acknowledge(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()
	bus := NewRedisStreamsEventBus(client, config)

	ctx := context.Background()

	// Setup: create group and add pending message
	_ = bus.EnsureConsumerGroup(ctx)
	client.mu.Lock()
	client.pendingMsgs[config.ConsumerGroup] = map[string]*mockPendingMessage{
		"0-1": {id: "0-1", consumer: config.ConsumerName, deliveryTime: time.Now()},
	}
	client.mu.Unlock()

	// Acknowledge
	err := bus.Acknowledge(ctx, "0-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify removed from pending
	pendingCount, _ := bus.PendingCount(ctx)
	if pendingCount != 0 {
		t.Errorf("expected 0 pending after manual ack, got %d", pendingCount)
	}
}

func TestRedisStreamsEventBus_ParseMessage_Errors(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()
	bus := NewRedisStreamsEventBus(client, config)

	// Missing data field
	_, err := bus.parseMessage(redis.XMessage{ID: "0-1", Values: map[string]interface{}{}})
	if err == nil {
		t.Error("expected error for missing data field")
	}

	// Invalid data type
	_, err = bus.parseMessage(redis.XMessage{ID: "0-1", Values: map[string]interface{}{"data": 123}})
	if err == nil {
		t.Error("expected error for invalid data type")
	}

	// Invalid JSON
	_, err = bus.parseMessage(redis.XMessage{ID: "0-1", Values: map[string]interface{}{"data": "not-json"}})
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDefaultRedisStreamsConfig(t *testing.T) {
	config := DefaultRedisStreamsConfig()

	if config.StreamKey == "" {
		t.Error("StreamKey should not be empty")
	}
	if config.ConsumerGroup == "" {
		t.Error("ConsumerGroup should not be empty")
	}
	if config.ConsumerName == "" {
		t.Error("ConsumerName should not be empty")
	}
	if config.MaxLen <= 0 {
		t.Error("MaxLen should be positive")
	}
	if config.BlockTimeout <= 0 {
		t.Error("BlockTimeout should be positive")
	}
	if config.ClaimMinIdleTime <= 0 {
		t.Error("ClaimMinIdleTime should be positive")
	}
	if config.BatchSize <= 0 {
		t.Error("BatchSize should be positive")
	}
}

func TestIsGroupExistsError(t *testing.T) {
	tests := []struct {
		err    error
		expect bool
	}{
		{nil, false},
		{errors.New("some error"), false},
		{errors.New("BUSYGROUP Consumer Group name already exists"), true},
	}

	for _, tc := range tests {
		got := isGroupExistsError(tc.err)
		if got != tc.expect {
			t.Errorf("isGroupExistsError(%v) = %v, want %v", tc.err, got, tc.expect)
		}
	}
}

// Integration test with miniredis
func TestRedisStreamsEventBus_Integration_Miniredis(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run error: %v", err)
	}
	defer s.Close()

	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	adapter := &RedisStreamsClientAdapter{Client: client}

	config := DefaultRedisStreamsConfig()
	config.StreamKey = "test-stream"
	config.ConsumerGroup = "test-group"
	config.ConsumerName = "test-consumer"
	config.BlockTimeout = 100 * time.Millisecond
	config.BatchSize = 10

	bus := NewRedisStreamsEventBus(adapter, config)
	defer bus.Stop()

	ctx := context.Background()

	// Ensure consumer group
	if err := bus.EnsureConsumerGroup(ctx); err != nil {
		t.Fatalf("EnsureConsumerGroup error: %v", err)
	}

	// Publish events
	for i := 0; i < 5; i++ {
		bus.Publish(ctx, Event{
			RequestID: fmt.Sprintf("req-%d", i),
			Method:    "POST",
			Path:      "/v1/chat/completions",
			Status:    200,
		})
	}

	// Verify stream length
	length, err := bus.StreamLength(ctx)
	if err != nil {
		t.Fatalf("StreamLength error: %v", err)
	}
	if length != 5 {
		t.Errorf("expected stream length 5, got %d", length)
	}

	// Subscribe and consume
	ch := bus.Subscribe()
	var received []Event

	timeout := time.After(2 * time.Second)
	for len(received) < 5 {
		select {
		case evt, ok := <-ch:
			if !ok {
				t.Fatal("channel closed unexpectedly")
			}
			received = append(received, evt)
		case <-timeout:
			t.Fatalf("timeout: only received %d events", len(received))
		}
	}

	if len(received) != 5 {
		t.Errorf("expected 5 events, got %d", len(received))
	}

	// Verify stats
	pub, drop := bus.Stats()
	if pub != 5 {
		t.Errorf("expected 5 published, got %d", pub)
	}
	if drop != 0 {
		t.Errorf("expected 0 dropped, got %d", drop)
	}
}

// Test RedisStreamsClientAdapter methods
func TestRedisStreamsClientAdapter_AllMethods(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run error: %v", err)
	}
	defer s.Close()

	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	adapter := &RedisStreamsClientAdapter{Client: client}

	ctx := context.Background()
	stream := "test-stream"
	group := "test-group"
	consumer := "test-consumer"

	// XGroupCreateMkStream
	err = adapter.XGroupCreateMkStream(ctx, stream, group, "0")
	if err != nil {
		t.Fatalf("XGroupCreateMkStream error: %v", err)
	}

	// XAdd
	id, err := adapter.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]interface{}{"data": "test"},
	})
	if err != nil {
		t.Fatalf("XAdd error: %v", err)
	}
	if id == "" {
		t.Error("XAdd returned empty ID")
	}

	// XLen
	length, err := adapter.XLen(ctx, stream)
	if err != nil {
		t.Fatalf("XLen error: %v", err)
	}
	if length != 1 {
		t.Errorf("expected length 1, got %d", length)
	}

	// XReadGroup
	streams, err := adapter.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{stream, ">"},
		Count:    10,
	})
	if err != nil {
		t.Fatalf("XReadGroup error: %v", err)
	}
	if len(streams) != 1 || len(streams[0].Messages) != 1 {
		t.Errorf("expected 1 stream with 1 message, got %d streams", len(streams))
	}

	// XPending
	pending, err := adapter.XPending(ctx, stream, group)
	if err != nil {
		t.Fatalf("XPending error: %v", err)
	}
	if pending.Count != 1 {
		t.Errorf("expected 1 pending, got %d", pending.Count)
	}

	// XPendingExt
	pendingExt, err := adapter.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: stream,
		Group:  group,
		Start:  "-",
		End:    "+",
		Count:  10,
	})
	if err != nil {
		t.Fatalf("XPendingExt error: %v", err)
	}
	if len(pendingExt) != 1 {
		t.Errorf("expected 1 pending entry, got %d", len(pendingExt))
	}

	// XAck
	acked, err := adapter.XAck(ctx, stream, group, id)
	if err != nil {
		t.Fatalf("XAck error: %v", err)
	}
	if acked != 1 {
		t.Errorf("expected 1 acked, got %d", acked)
	}

	// XInfoGroups
	groups, err := adapter.XInfoGroups(ctx, stream)
	if err != nil {
		t.Fatalf("XInfoGroups error: %v", err)
	}
	if len(groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(groups))
	}
}

// Test claiming pending messages
func TestRedisStreamsEventBus_ClaimPendingMessages(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()
	config.BlockTimeout = 10 * time.Millisecond
	config.ClaimMinIdleTime = 1 * time.Millisecond // Very short for test

	// Setup: create group with a pending message from another consumer
	client.groups[config.ConsumerGroup] = &mockConsumerGroup{
		name:          config.ConsumerGroup,
		lastDelivered: "0-1",
		consumers:     make(map[string]*mockConsumer),
	}
	client.stream = []mockStreamMessage{
		{ID: "0-1", Values: map[string]interface{}{"data": `{"RequestID":"claimed"}`}},
	}
	client.pendingMsgs[config.ConsumerGroup] = map[string]*mockPendingMessage{
		"0-1": {
			id:           "0-1",
			consumer:     "other-consumer",           // Different consumer
			deliveryTime: time.Now().Add(-time.Hour), // Old enough to claim
			deliverCount: 1,
		},
	}

	bus := NewRedisStreamsEventBus(client, config)

	// Mark group as created
	bus.groupCreated.Store(true)

	ch := make(chan Event, 10)

	// Call claimPendingMessages
	ctx := context.Background()
	bus.claimPendingMessages(ctx, ch)

	// Should have claimed the message
	select {
	case evt := <-ch:
		if evt.RequestID != "claimed" {
			t.Errorf("expected RequestID 'claimed', got '%s'", evt.RequestID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected to receive claimed message")
	}
}

// Test processing pending messages on startup
func TestRedisStreamsEventBus_ProcessPendingMessages(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()
	config.BlockTimeout = 10 * time.Millisecond

	// Setup: create group with pending message for this consumer
	client.groups[config.ConsumerGroup] = &mockConsumerGroup{
		name:          config.ConsumerGroup,
		lastDelivered: "0-1",
		consumers: map[string]*mockConsumer{
			config.ConsumerName: {name: config.ConsumerName},
		},
	}
	client.stream = []mockStreamMessage{
		{ID: "0-1", Values: map[string]interface{}{"data": `{"RequestID":"pending"}`}},
	}
	client.pendingMsgs[config.ConsumerGroup] = map[string]*mockPendingMessage{
		"0-1": {
			id:           "0-1",
			consumer:     config.ConsumerName,
			deliveryTime: time.Now(),
			deliverCount: 1,
		},
	}

	bus := NewRedisStreamsEventBus(client, config)
	bus.groupCreated.Store(true)

	ch := make(chan Event, 10)

	ctx := context.Background()
	bus.processPendingMessages(ctx, ch)

	select {
	case evt := <-ch:
		if evt.RequestID != "pending" {
			t.Errorf("expected RequestID 'pending', got '%s'", evt.RequestID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected to receive pending message")
	}
}

// Test consume loop handles XReadGroup errors
func TestRedisStreamsEventBus_ConsumeLoop_HandlesErrors(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()
	config.BlockTimeout = 10 * time.Millisecond

	// Pre-create group
	client.groups[config.ConsumerGroup] = &mockConsumerGroup{
		name:          config.ConsumerGroup,
		lastDelivered: "0",
		consumers:     make(map[string]*mockConsumer),
	}

	// Set error for XReadGroup
	client.xReadGroupErr = errors.New("connection lost")

	bus := NewRedisStreamsEventBus(client, config)
	ch := bus.Subscribe()

	// Let it run briefly
	time.Sleep(50 * time.Millisecond)

	bus.Stop()

	// Channel should be closed without panic
	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed")
	}
}

// Test Publish with JSON marshal error - use event with ResponseHeaders
func TestRedisStreamsEventBus_Publish_JSONMarshalCoverage(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()
	bus := NewRedisStreamsEventBus(client, config)

	ctx := context.Background()

	// Normal event should work
	evt := Event{
		RequestID: "test-marshal",
		Method:    "POST",
		Path:      "/v1/chat",
		Status:    200,
	}
	bus.Publish(ctx, evt)

	pub, drop := bus.Stats()
	if pub != 1 || drop != 0 {
		t.Errorf("expected 1 published, 0 dropped, got pub=%d, drop=%d", pub, drop)
	}
}

// Test XClaim with no eligible messages
func TestRedisStreamsEventBus_ClaimPendingMessages_NoEligible(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()
	config.ClaimMinIdleTime = time.Hour // Very long, nothing should be claimed

	// Setup: create group with a recent pending message
	client.groups[config.ConsumerGroup] = &mockConsumerGroup{
		name:          config.ConsumerGroup,
		lastDelivered: "0-1",
		consumers:     make(map[string]*mockConsumer),
	}
	client.stream = []mockStreamMessage{
		{ID: "0-1", Values: map[string]interface{}{"data": `{"RequestID":"recent"}`}},
	}
	client.pendingMsgs[config.ConsumerGroup] = map[string]*mockPendingMessage{
		"0-1": {
			id:           "0-1",
			consumer:     "other-consumer",
			deliveryTime: time.Now(), // Recent, not eligible for claim
			deliverCount: 1,
		},
	}

	bus := NewRedisStreamsEventBus(client, config)
	bus.groupCreated.Store(true)

	ch := make(chan Event, 10)

	ctx := context.Background()
	bus.claimPendingMessages(ctx, ch)

	// Should not receive any messages
	select {
	case <-ch:
		t.Error("expected no message to be claimed")
	default:
		// Good, no message claimed
	}
}

// Test processPendingMessages with XReadGroup error
func TestRedisStreamsEventBus_ProcessPendingMessages_Error(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()

	// Pre-create group
	client.groups[config.ConsumerGroup] = &mockConsumerGroup{
		name:          config.ConsumerGroup,
		lastDelivered: "0",
		consumers:     make(map[string]*mockConsumer),
	}

	// Set error for reading
	client.xReadGroupErr = errors.New("connection error")

	bus := NewRedisStreamsEventBus(client, config)
	bus.groupCreated.Store(true)

	ch := make(chan Event, 10)
	ctx := context.Background()

	// Should not panic, just log error
	bus.processPendingMessages(ctx, ch)

	select {
	case <-ch:
		t.Error("expected no message on error")
	default:
		// Good
	}
}

// Test claimPendingMessages with XPendingExt error
func TestRedisStreamsEventBus_ClaimPendingMessages_XPendingExtError(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()

	client.xPendingExtErr = errors.New("pending error")

	bus := NewRedisStreamsEventBus(client, config)

	ch := make(chan Event, 10)
	ctx := context.Background()

	// Should return early without panic
	bus.claimPendingMessages(ctx, ch)
}

// Test claimPendingMessages with XClaim error
func TestRedisStreamsEventBus_ClaimPendingMessages_XClaimError(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()
	config.ClaimMinIdleTime = 1 * time.Millisecond

	// Setup pending message that's old enough
	client.pendingMsgs[config.ConsumerGroup] = map[string]*mockPendingMessage{
		"0-1": {
			id:           "0-1",
			consumer:     "other",
			deliveryTime: time.Now().Add(-time.Hour),
			deliverCount: 1,
		},
	}

	client.xClaimErr = errors.New("claim error")

	bus := NewRedisStreamsEventBus(client, config)

	ch := make(chan Event, 10)
	ctx := context.Background()

	// Should log error but not panic
	bus.claimPendingMessages(ctx, ch)
}

// Test EnsureConsumerGroup returns early when already created
func TestRedisStreamsEventBus_EnsureConsumerGroup_AlreadyFlagged(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()
	bus := NewRedisStreamsEventBus(client, config)

	// Pre-set flag
	bus.groupCreated.Store(true)

	ctx := context.Background()
	err := bus.EnsureConsumerGroup(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not have called XGroupCreateMkStream
	if len(client.groups) != 0 {
		t.Error("should not have created group when flag already set")
	}
}

// Test Acknowledge with error
func TestRedisStreamsEventBus_Acknowledge_Error(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()
	bus := NewRedisStreamsEventBus(client, config)

	client.xAckErr = errors.New("ack error")

	ctx := context.Background()
	err := bus.Acknowledge(ctx, "0-1")
	if err == nil {
		t.Error("expected error from Acknowledge")
	}
}

// Test StreamLength with error
func TestRedisStreamsEventBus_StreamLength_Error(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()
	bus := NewRedisStreamsEventBus(client, config)

	client.xLenErr = errors.New("xlen error")

	ctx := context.Background()
	_, err := bus.StreamLength(ctx)
	if err == nil {
		t.Error("expected error from StreamLength")
	}
}

// Test PendingCount with error
func TestRedisStreamsEventBus_PendingCount_Error(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()
	bus := NewRedisStreamsEventBus(client, config)

	client.xPendingErr = errors.New("pending error")

	ctx := context.Background()
	_, err := bus.PendingCount(ctx)
	if err == nil {
		t.Error("expected error from PendingCount")
	}
}

// Test processPendingMessages with invalid JSON in pending message
func TestRedisStreamsEventBus_ProcessPendingMessages_InvalidJSON(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()
	config.BlockTimeout = 10 * time.Millisecond

	// Setup: group with pending message containing invalid JSON
	client.groups[config.ConsumerGroup] = &mockConsumerGroup{
		name:          config.ConsumerGroup,
		lastDelivered: "0-1",
		consumers: map[string]*mockConsumer{
			config.ConsumerName: {name: config.ConsumerName},
		},
	}
	client.stream = []mockStreamMessage{
		{ID: "0-1", Values: map[string]interface{}{"data": "invalid-json"}},
	}
	client.pendingMsgs[config.ConsumerGroup] = map[string]*mockPendingMessage{
		"0-1": {
			id:           "0-1",
			consumer:     config.ConsumerName,
			deliveryTime: time.Now(),
			deliverCount: 1,
		},
	}

	bus := NewRedisStreamsEventBus(client, config)
	bus.groupCreated.Store(true)

	ch := make(chan Event, 10)
	ctx := context.Background()

	// Should not send invalid message to channel
	bus.processPendingMessages(ctx, ch)

	select {
	case <-ch:
		t.Error("should not receive invalid JSON message")
	default:
		// Good
	}
}

// Test claimPendingMessages with invalid JSON in claimed message
func TestRedisStreamsEventBus_ClaimPendingMessages_InvalidJSON(t *testing.T) {
	client := newMockRedisStreamsClient()
	config := DefaultRedisStreamsConfig()
	config.ClaimMinIdleTime = 1 * time.Millisecond

	// Setup: old pending message with invalid JSON
	client.groups[config.ConsumerGroup] = &mockConsumerGroup{
		name:          config.ConsumerGroup,
		lastDelivered: "0-1",
		consumers:     make(map[string]*mockConsumer),
	}
	client.stream = []mockStreamMessage{
		{ID: "0-1", Values: map[string]interface{}{"data": "not-valid-json"}},
	}
	client.pendingMsgs[config.ConsumerGroup] = map[string]*mockPendingMessage{
		"0-1": {
			id:           "0-1",
			consumer:     "other-consumer",
			deliveryTime: time.Now().Add(-time.Hour),
			deliverCount: 1,
		},
	}

	bus := NewRedisStreamsEventBus(client, config)
	bus.groupCreated.Store(true)

	ch := make(chan Event, 10)
	ctx := context.Background()

	bus.claimPendingMessages(ctx, ch)

	select {
	case <-ch:
		t.Error("should not receive invalid JSON message")
	default:
		// Good, invalid message was acked but not sent
	}
}
