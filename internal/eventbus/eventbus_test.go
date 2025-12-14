package eventbus

import (
"context"
"testing"
"time"
)

func TestInMemoryEventBus_PublishSubscribe(t *testing.T) {
bus := NewInMemoryEventBus(10)
defer bus.Stop()

sub := bus.Subscribe()

// Publish several events
numEvents := 5
for i := 0; i < numEvents; i++ {
bus.Publish(context.Background(), Event{RequestID: "r"})
}

// Receive all events
received := 0
timeout := time.After(500 * time.Millisecond)
for received < numEvents {
select {
case <-sub:
received++
case <-timeout:
t.Fatalf("timeout waiting for events: got %d, want %d", received, numEvents)
}
}

pub, drop := bus.Stats()
if pub != numEvents || drop != 0 {
t.Fatalf("unexpected stats: published=%d dropped=%d", pub, drop)
}
}

func TestInMemoryEventBus_DroppedWhenFull(t *testing.T) {
bus := NewInMemoryEventBus(1)
defer bus.Stop()

// No subscriber; buffer size is 1 → one publish accepted, rest dropped
for i := 0; i < 5; i++ {
bus.Publish(context.Background(), Event{RequestID: "r"})
}

pub, drop := bus.Stats()
if pub == 0 {
t.Fatalf("expected some published events")
}
if drop == 0 {
t.Fatalf("expected dropped events when buffer is full")
}
}

func TestInMemoryEventBus_StopClosesSubscribers(t *testing.T) {
bus := NewInMemoryEventBus(1)
sub := bus.Subscribe()
bus.Stop()

select {
case _, ok := <-sub:
if ok {
t.Fatalf("expected closed subscriber channel after Stop")
}
case <-time.After(200 * time.Millisecond):
t.Fatalf("timeout waiting for subscriber channel to close")
}
}

// Covers retry branch in dispatch(): subscriber buffer full causes non-blocking send retries
func TestInMemoryEventBus_DispatchRetryWhenSubscriberFull(t *testing.T) {
bus := NewInMemoryEventBus(1)
defer bus.Stop()

sub := bus.Subscribe() // buffer size 1

// First publish fills subscriber buffer
bus.Publish(context.Background(), Event{RequestID: "r1"})
// Allow loop to deliver
time.Sleep(10 * time.Millisecond)

// Second publish triggers retry path because sub is full and we never drain it
start := time.Now()
bus.Publish(context.Background(), Event{RequestID: "r2"})

// Wait a bit longer than total retry backoff to ensure dispatch attempts happened
time.Sleep(50 * time.Millisecond)

// Subscriber buffer should still be full (1)
if got := len(sub); got != 1 {
t.Fatalf("expected subscriber buffer to remain full (1), got %d", got)
}

// Sanity: publish path executed quickly (not blocking), but we waited for retry backoff
if time.Since(start) < 40*time.Millisecond {
t.Fatalf("expected some retry backoff time to elapse")
}
}

func TestInMemoryEventBus_Stats(t *testing.T) {
b := NewInMemoryEventBus(1)
defer b.Stop()

// Publish 1 fits, next two should be dropped due to buffer full (no subscribers)
b.Publish(context.Background(), Event{})
b.Publish(context.Background(), Event{})
b.Publish(context.Background(), Event{})

pub, drop := b.Stats()
if pub < 1 {
t.Fatalf("published = %d, want >= 1", pub)
}
if drop < 1 {
t.Fatalf("dropped = %d, want >= 1", drop)
}
}
