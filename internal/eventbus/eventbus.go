package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Event represents an observability event emitted by the proxy.
type Event struct {
	RequestID       string
	Method          string
	Path            string
	Status          int
	Duration        time.Duration
	ResponseHeaders http.Header
	ResponseBody    []byte
}

// EventBus is a simple interface for publishing events to subscribers.
type EventBus interface {
	Publish(ctx context.Context, evt Event)
	Subscribe() <-chan Event
}

// InMemoryEventBus is a basic EventBus implementation backed by a buffered channel.
type InMemoryEventBus struct {
	ch   chan Event
	once sync.Once
}

// NewInMemoryEventBus creates a new in-memory event bus with the given buffer size.
func NewInMemoryEventBus(bufferSize int) *InMemoryEventBus {
	return &InMemoryEventBus{ch: make(chan Event, bufferSize)}
}

// Publish sends an event to the bus without blocking if the buffer is full.
func (b *InMemoryEventBus) Publish(ctx context.Context, evt Event) {
	select {
	case b.ch <- evt:
	default:
		// drop event if buffer is full
	}
}

// Subscribe returns a channel that receives events published to the bus.
// Multiple subscribers can read from the same channel.
func (b *InMemoryEventBus) Subscribe() <-chan Event {
	b.once.Do(func() {
		// ensure channel is created
		if b.ch == nil {
			b.ch = make(chan Event, 1)
		}
	})
	return b.ch
}

// RedisEventBus is an EventBus implementation backed by Redis streams.
type RedisEventBus struct {
	client    *redis.Client
	streamKey string
	groupName string
}

// NewRedisEventBus creates a new Redis-backed event bus.
func NewRedisEventBus(redisURL, streamKey, groupName string) (*RedisEventBus, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	bus := &RedisEventBus{
		client:    client,
		streamKey: streamKey,
		groupName: groupName,
	}

	// Create consumer group if it doesn't exist
	if err := bus.ensureConsumerGroup(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure consumer group: %w", err)
	}

	return bus, nil
}

// ensureConsumerGroup creates the consumer group if it doesn't exist.
func (b *RedisEventBus) ensureConsumerGroup(ctx context.Context) error {
	// Try to create the consumer group, ignore error if it already exists
	err := b.client.XGroupCreate(ctx, b.streamKey, b.groupName, "$").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}
	return nil
}

// Publish sends an event to the Redis stream.
func (b *RedisEventBus) Publish(ctx context.Context, evt Event) {
	data, err := json.Marshal(evt)
	if err != nil {
		// Drop event if marshaling fails
		return
	}

	args := &redis.XAddArgs{
		Stream: b.streamKey,
		Values: map[string]interface{}{
			"event": string(data),
		},
	}

	// Non-blocking publish - if it fails, drop the event
	b.client.XAdd(ctx, args)
}

// Subscribe returns a channel that receives events from the Redis stream.
func (b *RedisEventBus) Subscribe() <-chan Event {
	ch := make(chan Event, 10)

	go func() {
		defer close(ch)

		consumerName := fmt.Sprintf("consumer-%d", time.Now().UnixNano())

		for {
			ctx := context.Background()

			// Read from the consumer group
			streams, err := b.client.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    b.groupName,
				Consumer: consumerName,
				Streams:  []string{b.streamKey, ">"},
				Count:    10,
				Block:    time.Second,
			}).Result()

			if err != nil {
				if err == redis.Nil {
					continue // No new messages
				}
				// Log error and continue
				continue
			}

			for _, stream := range streams {
				for _, message := range stream.Messages {
					if eventData, ok := message.Values["event"].(string); ok {
						var evt Event
						if err := json.Unmarshal([]byte(eventData), &evt); err == nil {
							select {
							case ch <- evt:
								// Acknowledge the message
								b.client.XAck(ctx, b.streamKey, b.groupName, message.ID)
							default:
								// Channel is full, acknowledge and drop
								b.client.XAck(ctx, b.streamKey, b.groupName, message.ID)
							}
						} else {
							// Acknowledge malformed message
							b.client.XAck(ctx, b.streamKey, b.groupName, message.ID)
						}
					} else {
						// Acknowledge message with unexpected format
						b.client.XAck(ctx, b.streamKey, b.groupName, message.ID)
					}
				}
			}
		}
	}()

	return ch
}

// Close cleans up the Redis connection.
func (b *RedisEventBus) Close() error {
	return b.client.Close()
}
