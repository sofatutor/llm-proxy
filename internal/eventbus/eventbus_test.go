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

func TestRedisEventBusLog_PublishReadCount_TTLAndTrim(t *testing.T) {
    client := NewMockRedisClientLog()
    // TTL 1s, maxLen 3
    bus := NewRedisEventBusLog(client, "events", 1*time.Second, 3)

    // Publish 5 events → list should be trimmed to 3 most recent
    for i := 0; i < 5; i++ {
        bus.Publish(context.Background(), Event{RequestID: "r"})
    }

    // Count should be 3 due to LTRIM
    cnt, err := bus.EventCount(context.Background())
    if err != nil {
        t.Fatalf("EventCount error: %v", err)
    }
    if cnt != 3 {
        t.Fatalf("unexpected event count: %d (want 3)", cnt)
    }

    // Read back events and ensure LogID is set and monotonic (descending due to LPush at head)
    events, err := bus.ReadEvents(context.Background(), 0, -1)
    if err != nil {
        t.Fatalf("ReadEvents error: %v", err)
    }
    if len(events) != 3 {
        t.Fatalf("unexpected events length: %d (want 3)", len(events))
    }
    // Verify LogID is non-zero and strictly decreasing across the trimmed list
    if events[0].LogID == 0 || events[1].LogID == 0 || events[2].LogID == 0 {
        t.Fatalf("expected non-zero LogID for all events")
    }
    if !(events[0].LogID > events[1].LogID && events[1].LogID > events[2].LogID) {
        t.Fatalf("expected descending LogID order: got %d, %d, %d", events[0].LogID, events[1].LogID, events[2].LogID)
    }

    // TTL should be set on first publish
    if client.ttl <= 0 {
        t.Fatalf("expected TTL to be set, got %v", client.ttl)
    }
}


