package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStreamsClient interface for Redis Streams operations.
// This abstraction allows for easy mocking in tests.
type RedisStreamsClient interface {
	// XAdd adds an entry to a stream
	XAdd(ctx context.Context, args *redis.XAddArgs) (string, error)
	// XReadGroup reads entries from a stream using a consumer group
	XReadGroup(ctx context.Context, args *redis.XReadGroupArgs) ([]redis.XStream, error)
	// XAck acknowledges processed messages
	XAck(ctx context.Context, stream, group string, ids ...string) (int64, error)
	// XGroupCreateMkStream creates a consumer group (and the stream if needed)
	XGroupCreateMkStream(ctx context.Context, stream, group, start string) error
	// XPending returns pending entries for a consumer group
	XPending(ctx context.Context, stream, group string) (*redis.XPending, error)
	// XPendingExt returns detailed pending entries
	XPendingExt(ctx context.Context, args *redis.XPendingExtArgs) ([]redis.XPendingExt, error)
	// XClaim claims pending messages for a consumer
	XClaim(ctx context.Context, args *redis.XClaimArgs) ([]redis.XMessage, error)
	// XLen returns the length of a stream
	XLen(ctx context.Context, stream string) (int64, error)
	// XInfoGroups returns consumer group info for a stream
	XInfoGroups(ctx context.Context, stream string) ([]redis.XInfoGroup, error)
}

// RedisStreamsClientAdapter adapts go-redis/v9 Client to the RedisStreamsClient interface.
type RedisStreamsClientAdapter struct {
	Client *redis.Client
}

// XAdd adds an entry to a stream
func (a *RedisStreamsClientAdapter) XAdd(ctx context.Context, args *redis.XAddArgs) (string, error) {
	return a.Client.XAdd(ctx, args).Result()
}

// XReadGroup reads entries from a stream using a consumer group
func (a *RedisStreamsClientAdapter) XReadGroup(ctx context.Context, args *redis.XReadGroupArgs) ([]redis.XStream, error) {
	return a.Client.XReadGroup(ctx, args).Result()
}

// XAck acknowledges processed messages
func (a *RedisStreamsClientAdapter) XAck(ctx context.Context, stream, group string, ids ...string) (int64, error) {
	return a.Client.XAck(ctx, stream, group, ids...).Result()
}

// XGroupCreateMkStream creates a consumer group (and the stream if needed)
func (a *RedisStreamsClientAdapter) XGroupCreateMkStream(ctx context.Context, stream, group, start string) error {
	return a.Client.XGroupCreateMkStream(ctx, stream, group, start).Err()
}

// XPending returns pending entries for a consumer group
func (a *RedisStreamsClientAdapter) XPending(ctx context.Context, stream, group string) (*redis.XPending, error) {
	return a.Client.XPending(ctx, stream, group).Result()
}

// XPendingExt returns detailed pending entries
func (a *RedisStreamsClientAdapter) XPendingExt(ctx context.Context, args *redis.XPendingExtArgs) ([]redis.XPendingExt, error) {
	return a.Client.XPendingExt(ctx, args).Result()
}

// XClaim claims pending messages for a consumer
func (a *RedisStreamsClientAdapter) XClaim(ctx context.Context, args *redis.XClaimArgs) ([]redis.XMessage, error) {
	return a.Client.XClaim(ctx, args).Result()
}

// XLen returns the length of a stream
func (a *RedisStreamsClientAdapter) XLen(ctx context.Context, stream string) (int64, error) {
	return a.Client.XLen(ctx, stream).Result()
}

// XInfoGroups returns consumer group info for a stream
func (a *RedisStreamsClientAdapter) XInfoGroups(ctx context.Context, stream string) ([]redis.XInfoGroup, error) {
	return a.Client.XInfoGroups(ctx, stream).Result()
}

// RedisStreamsConfig holds configuration for Redis Streams event bus.
type RedisStreamsConfig struct {
	StreamKey        string        // Redis stream key name
	ConsumerGroup    string        // Consumer group name
	ConsumerName     string        // Unique consumer name within the group
	MaxLen           int64         // Max stream length (0 = unlimited, uses MAXLEN ~ approximation)
	BlockTimeout     time.Duration // Block timeout for XREADGROUP (0 = non-blocking)
	ClaimMinIdleTime time.Duration // Minimum idle time before claiming pending messages
	BatchSize        int64         // Number of messages to read at once
}

// DefaultRedisStreamsConfig returns default configuration.
func DefaultRedisStreamsConfig() RedisStreamsConfig {
	return RedisStreamsConfig{
		StreamKey:        "llm-proxy-events",
		ConsumerGroup:    "llm-proxy-dispatchers",
		ConsumerName:     "dispatcher-1",
		MaxLen:           10000,
		BlockTimeout:     5 * time.Second,
		ClaimMinIdleTime: 30 * time.Second,
		BatchSize:        100,
	}
}

// RedisStreamsEventBus implements EventBus using Redis Streams.
// It provides durable, distributed event delivery with consumer groups,
// acknowledgment, and at-least-once delivery semantics.
type RedisStreamsEventBus struct {
	client       RedisStreamsClient
	config       RedisStreamsConfig
	stats        busStats
	stopCh       chan struct{}
	stopOnce     sync.Once
	wg           sync.WaitGroup
	subscribers  []chan Event
	subsMu       sync.RWMutex
	groupCreated atomic.Bool
}

// NewRedisStreamsEventBus creates a new Redis Streams event bus.
func NewRedisStreamsEventBus(client RedisStreamsClient, config RedisStreamsConfig) *RedisStreamsEventBus {
	return &RedisStreamsEventBus{
		client: client,
		config: config,
		stopCh: make(chan struct{}),
	}
}

// EnsureConsumerGroup creates the consumer group if it doesn't exist.
// This should be called before starting to consume messages.
func (b *RedisStreamsEventBus) EnsureConsumerGroup(ctx context.Context) error {
	if b.groupCreated.Load() {
		return nil
	}

	// Try to create the group; if it already exists, that's fine
	err := b.client.XGroupCreateMkStream(ctx, b.config.StreamKey, b.config.ConsumerGroup, "0")
	if err != nil {
		// Check if error is because group already exists
		if isGroupExistsError(err) {
			b.groupCreated.Store(true)
			return nil
		}
		return fmt.Errorf("failed to create consumer group: %w", err)
	}
	b.groupCreated.Store(true)
	return nil
}

// isGroupExistsError checks if the error indicates the group already exists.
func isGroupExistsError(err error) bool {
	if err == nil {
		return false
	}
	// Redis returns "BUSYGROUP Consumer Group name already exists" error
	return err.Error() == "BUSYGROUP Consumer Group name already exists"
}

// Publish adds an event to the Redis stream using XADD.
func (b *RedisStreamsEventBus) Publish(ctx context.Context, evt Event) {
	data, err := json.Marshal(evt)
	if err != nil {
		log.Printf("[eventbus] Failed to marshal event: %v", err)
		b.stats.dropped.Add(1)
		return
	}

	args := &redis.XAddArgs{
		Stream: b.config.StreamKey,
		Values: map[string]interface{}{
			"data": string(data),
		},
	}

	// Apply MaxLen if configured
	if b.config.MaxLen > 0 {
		args.MaxLen = b.config.MaxLen
		args.Approx = true // Use ~ for better performance
	}

	_, err = b.client.XAdd(ctx, args)
	if err != nil {
		log.Printf("[eventbus] Failed to publish event to stream %s: %v", b.config.StreamKey, err)
		b.stats.dropped.Add(1)
		return
	}
	b.stats.published.Add(1)
}

// Subscribe returns a channel that receives events from the stream.
// This starts a background goroutine that reads from the stream using consumer groups.
func (b *RedisStreamsEventBus) Subscribe() <-chan Event {
	ch := make(chan Event, b.config.BatchSize)
	b.subsMu.Lock()
	b.subscribers = append(b.subscribers, ch)
	b.subsMu.Unlock()

	b.wg.Add(1)
	go b.consumeLoop(ch)

	return ch
}

// consumeLoop reads messages from the stream and dispatches to the subscriber channel.
func (b *RedisStreamsEventBus) consumeLoop(ch chan Event) {
	defer b.wg.Done()
	defer close(ch)

	ctx := context.Background()

	// Ensure consumer group exists
	if err := b.EnsureConsumerGroup(ctx); err != nil {
		log.Printf("[eventbus] Failed to ensure consumer group: %v", err)
		return
	}

	// First, process any pending messages (messages we received but didn't acknowledge)
	b.processPendingMessages(ctx, ch)

	// Then start normal consumption
	for {
		select {
		case <-b.stopCh:
			return
		default:
		}

		// Read new messages
		streams, err := b.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    b.config.ConsumerGroup,
			Consumer: b.config.ConsumerName,
			Streams:  []string{b.config.StreamKey, ">"},
			Count:    b.config.BatchSize,
			Block:    b.config.BlockTimeout,
		})

		if err != nil {
			if err == redis.Nil {
				// No new messages, check for pending messages to claim
				b.claimPendingMessages(ctx, ch)
				continue
			}
			// Check if context was cancelled or we're stopping
			select {
			case <-b.stopCh:
				return
			default:
				log.Printf("[eventbus] Error reading from stream: %v", err)
				time.Sleep(time.Second) // Back off on error
				continue
			}
		}

		// Process received messages
		for _, stream := range streams {
			for _, msg := range stream.Messages {
				evt, err := b.parseMessage(msg)
				if err != nil {
					log.Printf("[eventbus] Failed to parse message %s: %v", msg.ID, err)
					// Acknowledge invalid messages so they don't get stuck
					_, _ = b.client.XAck(ctx, b.config.StreamKey, b.config.ConsumerGroup, msg.ID)
					continue
				}

				// Try to send to subscriber
				select {
				case ch <- evt:
					// Message delivered, acknowledge it
					_, err := b.client.XAck(ctx, b.config.StreamKey, b.config.ConsumerGroup, msg.ID)
					if err != nil {
						log.Printf("[eventbus] Failed to acknowledge message %s: %v", msg.ID, err)
					}
				case <-b.stopCh:
					return
				}
			}
		}
	}
}

// processPendingMessages handles messages that were delivered but not acknowledged.
// This is called on startup to handle messages from a previous crash.
func (b *RedisStreamsEventBus) processPendingMessages(ctx context.Context, ch chan Event) {
	// Read our pending messages
	streams, err := b.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    b.config.ConsumerGroup,
		Consumer: b.config.ConsumerName,
		Streams:  []string{b.config.StreamKey, "0"}, // "0" means pending messages
		Count:    b.config.BatchSize,
	})

	if err != nil && err != redis.Nil {
		log.Printf("[eventbus] Error reading pending messages: %v", err)
		return
	}

	for _, stream := range streams {
		for _, msg := range stream.Messages {
			evt, err := b.parseMessage(msg)
			if err != nil {
				log.Printf("[eventbus] Failed to parse pending message %s: %v", msg.ID, err)
				// Acknowledge invalid messages
				_, _ = b.client.XAck(ctx, b.config.StreamKey, b.config.ConsumerGroup, msg.ID)
				continue
			}

			select {
			case ch <- evt:
				_, _ = b.client.XAck(ctx, b.config.StreamKey, b.config.ConsumerGroup, msg.ID)
			case <-b.stopCh:
				return
			}
		}
	}
}

// claimPendingMessages claims messages from other consumers that have been idle too long.
// This implements the "at-least-once" delivery guarantee for crashed consumers.
func (b *RedisStreamsEventBus) claimPendingMessages(ctx context.Context, ch chan Event) {
	// Get pending entries that have exceeded the idle time
	pending, err := b.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: b.config.StreamKey,
		Group:  b.config.ConsumerGroup,
		Start:  "-",
		End:    "+",
		Count:  b.config.BatchSize,
	})

	if err != nil || len(pending) == 0 {
		return
	}

	// Find messages that are idle long enough to claim
	var toClaim []string
	for _, p := range pending {
		if p.Idle >= b.config.ClaimMinIdleTime {
			toClaim = append(toClaim, p.ID)
		}
	}

	if len(toClaim) == 0 {
		return
	}

	// Claim the messages
	messages, err := b.client.XClaim(ctx, &redis.XClaimArgs{
		Stream:   b.config.StreamKey,
		Group:    b.config.ConsumerGroup,
		Consumer: b.config.ConsumerName,
		MinIdle:  b.config.ClaimMinIdleTime,
		Messages: toClaim,
	})

	if err != nil {
		log.Printf("[eventbus] Error claiming messages: %v", err)
		return
	}

	// Process claimed messages
	for _, msg := range messages {
		evt, err := b.parseMessage(msg)
		if err != nil {
			log.Printf("[eventbus] Failed to parse claimed message %s: %v", msg.ID, err)
			_, _ = b.client.XAck(ctx, b.config.StreamKey, b.config.ConsumerGroup, msg.ID)
			continue
		}

		select {
		case ch <- evt:
			_, _ = b.client.XAck(ctx, b.config.StreamKey, b.config.ConsumerGroup, msg.ID)
		case <-b.stopCh:
			return
		}
	}
}

// parseMessage extracts an Event from a Redis stream message.
func (b *RedisStreamsEventBus) parseMessage(msg redis.XMessage) (Event, error) {
	var evt Event
	data, ok := msg.Values["data"]
	if !ok {
		return evt, fmt.Errorf("message missing 'data' field")
	}

	dataStr, ok := data.(string)
	if !ok {
		return evt, fmt.Errorf("'data' field is not a string")
	}

	if err := json.Unmarshal([]byte(dataStr), &evt); err != nil {
		return evt, fmt.Errorf("failed to unmarshal event: %w", err)
	}

	return evt, nil
}

// Stop gracefully stops the event bus and closes all subscriber channels.
func (b *RedisStreamsEventBus) Stop() {
	b.stopOnce.Do(func() {
		close(b.stopCh)
		b.wg.Wait()
	})
}

// Stats returns the number of published and dropped events.
func (b *RedisStreamsEventBus) Stats() (published, dropped int) {
	return int(b.stats.published.Load()), int(b.stats.dropped.Load())
}

// StreamLength returns the current length of the stream.
func (b *RedisStreamsEventBus) StreamLength(ctx context.Context) (int64, error) {
	return b.client.XLen(ctx, b.config.StreamKey)
}

// PendingCount returns the number of pending messages in the consumer group.
func (b *RedisStreamsEventBus) PendingCount(ctx context.Context) (int64, error) {
	pending, err := b.client.XPending(ctx, b.config.StreamKey, b.config.ConsumerGroup)
	if err != nil {
		return 0, err
	}
	return pending.Count, nil
}

// Client returns the underlying RedisStreamsClient.
func (b *RedisStreamsEventBus) Client() RedisStreamsClient {
	return b.client
}

// Acknowledge manually acknowledges a message by ID.
// This is useful when external code handles message processing and acknowledgment.
func (b *RedisStreamsEventBus) Acknowledge(ctx context.Context, messageID string) error {
	_, err := b.client.XAck(ctx, b.config.StreamKey, b.config.ConsumerGroup, messageID)
	return err
}
