package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"dllm-network/internal/dashboard"
	"dllm-network/internal/telemetry/inference"
)

// fakeProjector records every ProjectionInput forwarded to it, distinguished
// by the current inference id, so tests can assert conflation behavior.
type fakeProjector struct {
	mu    sync.Mutex
	calls []string
}

func (f *fakeProjector) Publish(_ context.Context, input dashboard.ProjectionInput) (dashboard.Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, input.Inference.Current.ID)
	return dashboard.Snapshot{}, nil
}

func (f *fakeProjector) ids() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func inputWithID(id string) dashboard.ProjectionInput {
	return dashboard.ProjectionInput{
		Inference: dashboard.InferenceState{Current: inference.Inference{ID: id}},
	}
}

func TestCoalescingProjector_PassThroughWhenIntervalZero(t *testing.T) {
	inner := &fakeProjector{}
	c := newCoalescingProjector(inner, 0)

	_, _ = c.Publish(context.Background(), inputWithID("a"))
	_, _ = c.Publish(context.Background(), inputWithID("b"))

	// interval<=0 is a synchronous pass-through: every call forwards immediately.
	if got := inner.ids(); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("expected synchronous [a b], got %v", got)
	}
}

func TestCoalescingProjector_ConflatesUntilFlush(t *testing.T) {
	inner := &fakeProjector{}
	c := newCoalescingProjector(inner, time.Hour) // long interval: only manual flush fires

	_, _ = c.Publish(context.Background(), inputWithID("a"))
	_, _ = c.Publish(context.Background(), inputWithID("b"))

	// Conflated: nothing forwarded yet.
	if got := inner.ids(); len(got) != 0 {
		t.Fatalf("expected no forwards before flush, got %v", got)
	}

	// flush forwards only the LATEST pending input.
	if !c.flush(context.Background()) {
		t.Fatal("expected flush to report pending work")
	}
	if got := inner.ids(); len(got) != 1 || got[0] != "b" {
		t.Fatalf("expected conflated [b], got %v", got)
	}

	// Second flush with nothing new pending is a no-op.
	if c.flush(context.Background()) {
		t.Fatal("expected flush to be a no-op when nothing is pending")
	}
	if got := inner.ids(); len(got) != 1 {
		t.Fatalf("expected still [b], got %v", got)
	}
}

func TestCoalescingProjector_StopFinalFlush(t *testing.T) {
	inner := &fakeProjector{}
	c := newCoalescingProjector(inner, time.Hour) // long interval: ticker never fires in-test
	c.start(context.Background())

	_, _ = c.Publish(context.Background(), inputWithID("final"))
	c.stop() // must flush the last pending state before returning

	if got := inner.ids(); len(got) != 1 || got[0] != "final" {
		t.Fatalf("expected final flush [final], got %v", got)
	}
}
