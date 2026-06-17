package capture

import (
	"context"
	"sync"
)

// noopSource is a CaptureSource that intentionally does nothing. It is used
// as the rollback / unelevated fallback: the application continues to run in
// poller-only mode while surfacing a degraded SourceStatus to the UI.
//
// noopSource satisfies the CaptureSource interface and is safe for concurrent
// use.
type noopSource struct {
	mu     sync.Mutex
	closed bool
}

// NewNoopSource returns a CaptureSource that never delivers segments and
// always reports an inactive, non-elevated status. Safe for concurrent use.
func NewNoopSource() CaptureSource {
	return &noopSource{}
}

// Open is a no-op; it always succeeds.
func (n *noopSource) Open() error {
	return nil
}

// Recv blocks until ctx is done and then returns ctx.Err(). The noop source
// never delivers segments, so Recv always returns an error — callers must
// treat this as "no data available" rather than a fatal failure.
func (n *noopSource) Recv(ctx context.Context) (Segment, error) {
	<-ctx.Done()
	return Segment{}, ctx.Err()
}

// Close marks the source as closed. Idempotent.
func (n *noopSource) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.closed = true
	return nil
}

// Status always reports an inactive, non-elevated source.
func (n *noopSource) Status() SourceStatus {
	return SourceStatus{
		Active:   false,
		Elevated: false,
		Reason:   "noop source — capture disabled",
	}
}
