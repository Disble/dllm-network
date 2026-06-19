package app

import (
	"context"
	"sync"
	"time"

	"ollama-telemetry/internal/dashboard"
)

// defaultCaptureEmitInterval bounds how often the capture pipeline's snapshot
// reaches the frontend. The capture recv-loop can produce a segment per TCP
// packet (hundreds per streamed response); emitting a full snapshot per segment
// made the recv-loop spend its time on JSON marshal + the Wails bridge instead
// of draining the WinDivert segment channel, which back-pressured the kernel
// queue into dropping packets (lost completions → requests stuck "in progress").
// ~16 Hz is smooth for the UI and keeps the hot path free.
const defaultCaptureEmitInterval = 60 * time.Millisecond

// coalescingProjector decorates a snapshotProjector so high-frequency Publish
// calls from the capture hot path are CONFLATED: each call only records the
// latest ProjectionInput (cheap, non-blocking) and a separate cadence goroutine
// forwards the most-recent input to the wrapped projector at most once per
// interval. This moves the expensive Project + JSON marshal + Wails emit OFF
// the capture goroutine so it never stalls draining the WinDivert segment
// channel.
//
// It is the Decorator pattern over snapshotProjector with latest-wins
// (conflation) semantics. With interval <= 0 it degrades to a synchronous
// pass-through — used by tests that assert emissions deterministically.
type coalescingProjector struct {
	inner    snapshotProjector
	interval time.Duration

	mu      sync.Mutex
	latest  dashboard.ProjectionInput
	pending bool

	cancel context.CancelFunc
	done   chan struct{}
}

// newCoalescingProjector wraps inner. interval <= 0 means synchronous
// pass-through (no goroutine, no conflation).
func newCoalescingProjector(inner snapshotProjector, interval time.Duration) *coalescingProjector {
	return &coalescingProjector{inner: inner, interval: interval}
}

// Publish records input as the latest pending state and returns immediately.
// When interval <= 0 it forwards synchronously to inner instead.
func (c *coalescingProjector) Publish(ctx context.Context, input dashboard.ProjectionInput) (dashboard.Snapshot, error) {
	if c.interval <= 0 {
		return c.inner.Publish(ctx, input)
	}

	c.mu.Lock()
	c.latest = input
	c.pending = true
	c.mu.Unlock()

	return dashboard.Snapshot{}, nil
}

// flush forwards the latest pending input to inner. It returns false when
// nothing was pending (so the cadence goroutine can stay idle between bursts).
func (c *coalescingProjector) flush(ctx context.Context) bool {
	c.mu.Lock()
	if !c.pending {
		c.mu.Unlock()
		return false
	}
	input := c.latest
	c.pending = false
	c.mu.Unlock()

	_, _ = c.inner.Publish(ctx, input)
	return true
}

// start launches the cadence goroutine. It is a no-op in pass-through mode.
func (c *coalescingProjector) start(ctx context.Context) {
	if c.interval <= 0 {
		return
	}

	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.done = make(chan struct{})

	go func() {
		defer close(c.done)
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()

		for {
			select {
			case <-runCtx.Done():
				// Final flush: emit the last observed state even though the run
				// context is cancelled, using a fresh context so the emit is not
				// itself cancelled.
				c.flush(context.Background())
				return
			case <-ticker.C:
				c.flush(runCtx)
			}
		}
	}()
}

// stop cancels the cadence goroutine and waits for its final flush. No-op in
// pass-through mode or when start was never called.
func (c *coalescingProjector) stop() {
	if c.cancel == nil {
		return
	}
	c.cancel()
	<-c.done
	c.cancel = nil
}
