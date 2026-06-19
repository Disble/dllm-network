package app

import (
	"os"
	"testing"

	"ollama-telemetry/internal/capture"
)

// TestMain installs an inert capture source as the package default for the
// whole test run so unit tests never open the real WinDivert driver. Without
// this, on an elevated Windows host with live Ollama traffic a Startup-driven
// test could capture a real packet and emit it through the real Wails runtime —
// which panics with no live Wails context and crashes the test binary
// (environment-dependent flake). Tests that need capture behavior inject their
// own CaptureSource via Dependencies, which takes precedence over this default.
func TestMain(m *testing.M) {
	original := newCaptureSource
	newCaptureSource = func() capture.CaptureSource { return capture.NewFakeSource(nil) }
	code := m.Run()
	newCaptureSource = original
	os.Exit(code)
}
