package capture_test

import (
	"context"
	"testing"
	"time"

	"dllm-network/internal/capture"
)

// TestNoopSource_OpenRecvCloseLifecycle asserts that the CaptureSource
// interface contract is satisfied by the no-op source without touching any
// driver, syscall, or OS API. It verifies the full Open→Recv→Close lifecycle
// and confirms Status() reports an inactive, non-elevated source.
func TestNoopSource_OpenRecvCloseLifecycle(t *testing.T) {
	t.Parallel()

	src := capture.NewNoopSource()

	// Open must succeed without error.
	if err := src.Open(); err != nil {
		t.Fatalf("Open: unexpected error: %v", err)
	}

	// Status must report inactive with no reason string — the noop source is
	// intentionally non-functional; it surfaces that state honestly.
	status := src.Status()
	if status.Active {
		t.Errorf("Status.Active: want false, got true")
	}
	if status.Elevated {
		t.Errorf("Status.Elevated: want false, got true")
	}

	// Recv on a noop source must return immediately with an error (or a
	// context-cancelled signal) — it never blocks waiting for packets that
	// will never arrive. A cancelled context is the canonical way to signal
	// the caller.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := src.Recv(ctx)
	if err == nil {
		t.Error("Recv: expected error (noop never delivers segments), got nil")
	}

	// Close must be idempotent — calling it twice must not panic or error.
	if err := src.Close(); err != nil {
		t.Errorf("Close (first): unexpected error: %v", err)
	}
	if err := src.Close(); err != nil {
		t.Errorf("Close (second, idempotent): unexpected error: %v", err)
	}
}
