package persistence

import (
	"context"
	"testing"
	"time"

	"dllm-network/internal/events"
	"dllm-network/internal/telemetry/inference"
)

// TestSubscriber_SubscribeToBus_PersistsPublishedInference asserts the
// full wiring: Subscriber.Subscribe registers HandleEvent on the bus for
// the "inference.completed" topic, and a real bus.Publish call reaches the
// subscriber, gets batched, and flushed to the Writer.
func TestSubscriber_SubscribeToBus_PersistsPublishedInference(t *testing.T) {
	t.Parallel()

	bus := events.NewBus()
	writer := &fakeWriter{}
	sub := NewSubscriber(writer)
	unsubscribe := sub.Subscribe(bus)
	t.Cleanup(unsubscribe)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sub.Run(ctx)
	t.Cleanup(sub.Stop)

	bus.Publish(events.Event{
		Topic:   topicInferenceCompleted,
		Payload: inference.Inference{ID: "inf-bus-1", Model: "llama3"},
	})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && writer.RowCount() == 0 {
		time.Sleep(10 * time.Millisecond)
	}

	batches := writer.Batches()
	if writer.RowCount() != 1 {
		t.Fatalf("expected 1 row persisted via bus publish, got %d (batches=%v)", writer.RowCount(), batches)
	}
	if batches[0][0].ID != "inf-bus-1" {
		t.Fatalf("expected persisted inference id inf-bus-1, got %q", batches[0][0].ID)
	}
}

// TestSubscriber_IgnoresOtherTopics asserts the subscriber only reacts to
// the inference-completed topic, never to unrelated bus traffic.
func TestSubscriber_IgnoresOtherTopics(t *testing.T) {
	t.Parallel()

	bus := events.NewBus()
	writer := &fakeWriter{}
	sub := NewSubscriber(writer)
	unsubscribe := sub.Subscribe(bus)
	t.Cleanup(unsubscribe)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sub.Run(ctx)
	t.Cleanup(sub.Stop)

	bus.Publish(events.Event{Topic: "some.other.topic", Payload: "irrelevant"})

	time.Sleep(50 * time.Millisecond)
	if got := writer.RowCount(); got != 0 {
		t.Fatalf("expected subscriber to ignore unrelated topics, got %d rows persisted", got)
	}
}
