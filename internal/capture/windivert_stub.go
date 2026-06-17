//go:build !windows

package capture

// NewWinDivertSource returns a CaptureSource backed by the WinDivert driver.
// On non-Windows platforms this function returns a no-op source that reports
// an inactive, non-elevated status — WinDivert is a Windows kernel driver
// and cannot run on other operating systems. The noop semantics ensure the
// application compiles and starts cleanly on all platforms, degrading to
// poller-only mode where the live capture source is unavailable.
func NewWinDivertSource() CaptureSource {
	return &noopSource{}
}
