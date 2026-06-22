//go:build windows

package app

import "dllm-network/internal/capture"

// newWinDivertCapture returns the real WinDivert capture source on Windows.
// The source is opened lazily (in inferencePipeline.run) so the constructor
// itself never requires elevation.
func newWinDivertCapture() capture.CaptureSource {
	return capture.NewWinDivertSource()
}
