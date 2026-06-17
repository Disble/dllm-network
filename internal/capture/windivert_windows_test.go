//go:build windows

package capture_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"ollama-telemetry/internal/capture"
)

// TestWinDivertSource_OpenLifecycle is an INTEGRATION test that exercises the
// real WinDivert adapter against the kernel driver. It is guarded by
// testing.Short() because it requires:
//   - WinDivert.dll and WinDivert64.sys available beside the executable (or
//     written there by the embed-on-first-run logic in 5.9).
//   - Administrator (elevated) privileges on Windows to load the kernel driver.
//
// In normal CI / `go test -short ./internal/capture/...` runs this test is
// skipped. The orchestrator will perform manual elevated verification once
// WU5 is complete.
//
// The test deliberately accepts two valid outcomes:
//  1. Elevated process: Open succeeds, Status is Active+Elevated, a Recv with
//     a short timeout returns without panicking (may time out if no traffic).
//  2. Non-elevated process: Open returns a non-nil error OR Open succeeds but
//     Status reports Active=false, Elevated=false with a human-readable
//     reason — no panic in either case.
func TestWinDivertSource_OpenLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping WinDivert integration test in -short mode (requires admin + driver)")
	}

	src := capture.NewWinDivertSource()

	// Status before Open must be safe to call (no panic).
	preopenStatus := src.Status()
	t.Logf("status before Open: active=%v elevated=%v reason=%q",
		preopenStatus.Active, preopenStatus.Elevated, preopenStatus.Reason)

	openErr := src.Open()

	status := src.Status()
	t.Logf("status after Open (err=%v): active=%v elevated=%v reason=%q",
		openErr, status.Active, status.Elevated, status.Reason)

	if openErr != nil {
		// Open returned an error — acceptable when not elevated. Confirm the
		// status reflects a degraded (non-active) state.
		if status.Active {
			t.Error("Open returned error but Status.Active is true — inconsistent state")
		}
		// Confirm Close is safe to call after a failed Open.
		if err := src.Close(); err != nil {
			t.Errorf("Close after failed Open: unexpected error: %v", err)
		}
		return
	}

	// Open succeeded — the process is elevated and the driver loaded.
	if !status.Elevated {
		t.Error("Open succeeded but Status.Elevated is false — expected elevated=true when driver loaded")
	}

	// Recv with a short timeout; we don't assert a segment is received (there
	// may be no Ollama traffic), but the call must not panic and must respect
	// context cancellation.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	seg, err := src.Recv(ctx)
	if err == nil {
		t.Logf("Recv returned a segment: tuple=%v dir=%v payload=%d bytes seqNo=%d",
			seg.Tuple, seg.Dir, len(seg.Payload), seg.SeqNo)
	} else if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		t.Logf("Recv timed out (no Ollama traffic during test): %v", err)
	} else {
		t.Errorf("Recv returned unexpected error: %v", err)
	}

	// Close must succeed and be idempotent.
	if err := src.Close(); err != nil {
		t.Errorf("Close: unexpected error: %v", err)
	}
	if err := src.Close(); err != nil {
		t.Errorf("Close (second, idempotent): unexpected error: %v", err)
	}
}

// TestWinDivertSource_UnelevatedGracefulDegradation verifies that when the
// process lacks administrator privileges the source reports a clear,
// human-readable reason rather than panicking or returning a raw errno.
//
// This test is also guarded by testing.Short() because it still requires the
// DLL to be loadable (even without elevation, WinDivert.dll must exist).
// Without the DLL present at all, the test is skipped gracefully.
func TestWinDivertSource_UnelevatedGracefulDegradation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping WinDivert integration test in -short mode (requires driver DLL)")
	}

	src := capture.NewWinDivertSource()
	openErr := src.Open()
	status := src.Status()

	t.Logf("open err: %v | active: %v | elevated: %v | reason: %q",
		openErr, status.Active, status.Elevated, status.Reason)

	// Whether elevated or not, the source must have a non-empty reason when
	// not active.
	if !status.Active && status.Reason == "" {
		t.Error("Status.Active=false but Reason is empty — must give callers a human-readable explanation")
	}

	// No matter what, Close must not panic.
	_ = src.Close()
}
