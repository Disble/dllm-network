//go:build !windows

package capture_test

import (
	"context"
	"testing"
	"time"

	"dllm-network/internal/capture"
)

// TestStubSource_ReturnsInactiveStatusWithNoPanic asserts that on non-Windows
// platforms the WinDivert stub source satisfies the CaptureSource contract,
// never panics, and honestly reports an inactive, non-elevated status.
//
// This test only compiles and runs on non-Windows platforms (build tag
// "!windows"). It exists to ensure the stub maintains correct contract
// semantics as a drop-in replacement for the real WinDivert adapter wherever
// the driver is unavailable.
func TestStubSource_ReturnsInactiveStatusWithNoPanic(t *testing.T) {
	t.Parallel()

	src := capture.NewWinDivertSource()

	// Open must not panic and must return at most a non-fatal error.
	if err := src.Open(); err != nil {
		// On non-windows, Open is expected to succeed (noop), or return a
		// clear descriptive error — but must NOT panic.
		t.Logf("Open returned (non-fatal on non-windows): %v", err)
	}

	// Status must indicate the source is not active and not elevated —
	// there is no WinDivert driver on non-Windows.
	status := src.Status()
	if status.Active {
		t.Errorf("Status.Active: want false on non-windows stub, got true")
	}
	if status.Elevated {
		t.Errorf("Status.Elevated: want false on non-windows stub, got true")
	}

	// Recv must not block indefinitely; a short timeout must cause it to
	// return an error via ctx cancellation, not hang.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := src.Recv(ctx)
	if err == nil {
		t.Error("Recv: expected error from non-windows stub, got nil")
	}

	// Close must be idempotent and not panic.
	if err := src.Close(); err != nil {
		t.Errorf("Close (first): unexpected error: %v", err)
	}
	if err := src.Close(); err != nil {
		t.Errorf("Close (second, idempotent): unexpected error: %v", err)
	}
}
