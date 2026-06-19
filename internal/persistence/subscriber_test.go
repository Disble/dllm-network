package persistence

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"ollama-telemetry/internal/events"
	"ollama-telemetry/internal/telemetry/inference"
)

// errFakeSave is returned by fakeWriter.Save when configured with saveErr,
// used by tests asserting Prune is skipped on a failed flush.
var errFakeSave = errors.New("fake save error")

// TestSubscriber_NonBlockingSend_OnFullBuffer asserts that HandleEvent (the
// bus handler) never blocks even when the internal channel is at capacity
// and nothing is draining it. This is the core async-write-path contract
// (design D4 / spec "Channel-full does not stall capture"): bus.Publish is
// SYNCHRONOUS, so the handler itself must be a single non-blocking channel
// send. On a full buffer it drops (drop-oldest) rather than blocking.
func TestSubscriber_NonBlockingSend_OnFullBuffer(t *testing.T) {
	t.Parallel()

	sub := newSubscriberWithCapacity(&fakeWriter{}, 4)
	// Do NOT start the drain loop — simulates a full/slow sink. Every
	// HandleEvent call below must return immediately regardless.

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			sub.HandleEvent(events.Event{
				Topic:   topicInferenceCompleted,
				Payload: inference.Inference{ID: "inf-" + string(rune('a'+i%26))},
			})
		}
	}()

	select {
	case <-done:
		// success: 100 sends against a cap=4 channel with no drain returned
		// without blocking.
	case <-time.After(2 * time.Second):
		t.Fatal("HandleEvent blocked on a full/undrained buffer — capture must never stall")
	}

	if got := sub.Dropped(); got == 0 {
		t.Fatalf("expected dropped count > 0 after overflowing a cap=4 buffer with 100 sends, got %d", got)
	}
}

// fakeWriter is a test double for the sqlite.Store write surface, recording
// every batch passed to Save and every Prune call without touching a real
// database. Save/Prune are called from the batcher's own goroutine while
// tests read Batches/Rows/PruneCalls from the test goroutine, so access is
// guarded by mu.
type fakeWriter struct {
	mu         sync.Mutex
	batches    [][]inference.Inference
	delay      time.Duration
	saveErr    error
	pruneCalls []prunCall
}

// prunCall records one Writer.Prune invocation's arguments.
type prunCall struct {
	maxCount int
	maxAge   time.Duration
}

func (w *fakeWriter) Save(ctx context.Context, infs []inference.Inference) error {
	if w.delay > 0 {
		select {
		case <-time.After(w.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	w.mu.Lock()
	w.batches = append(w.batches, append([]inference.Inference(nil), infs...))
	w.mu.Unlock()
	return w.saveErr
}

// Prune records the call so tests can assert the batcher invokes it after
// every successful flush (never inside the bus-handler goroutine).
func (w *fakeWriter) Prune(ctx context.Context, maxCount int, maxAge time.Duration) error {
	w.mu.Lock()
	w.pruneCalls = append(w.pruneCalls, prunCall{maxCount: maxCount, maxAge: maxAge})
	w.mu.Unlock()
	return nil
}

// PruneCalls returns a snapshot of the recorded Prune calls so far.
func (w *fakeWriter) PruneCalls() []prunCall {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]prunCall(nil), w.pruneCalls...)
}

// Batches returns a snapshot of the batches recorded so far.
func (w *fakeWriter) Batches() [][]inference.Inference {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([][]inference.Inference(nil), w.batches...)
}

// RowCount returns the total number of inferences across all recorded
// batches.
func (w *fakeWriter) RowCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := 0
	for _, b := range w.batches {
		n += len(b)
	}
	return n
}
