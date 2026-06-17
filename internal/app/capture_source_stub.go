//go:build !windows

package app

import "ollama-telemetry/internal/capture"

// newWinDivertCapture returns the noop source on non-Windows platforms.
// Capture requires WinDivert which is Windows-only; on other platforms the
// app runs in poller-only (passive) mode.
func newWinDivertCapture() capture.CaptureSource {
	return capture.NewNoopSource()
}
