package capture

import (
	"context"
	"errors"
	"sync"
)

// ErrFakeExhausted is returned by fakeSource.Recv once all configured
// segments have been replayed. Callers can use errors.Is to detect
// exhaustion vs. context cancellation.
var ErrFakeExhausted = errors.New("fake capture source: all segments replayed")

// fakeSource is a CaptureSource that replays a fixed slice of Segments in
// order. It is used in tests to exercise the downstream pipeline (reassembler,
// HTTP parser, extractor) with real golden-byte traffic without requiring a
// WinDivert driver or administrator privileges.
//
// fakeSource is safe for concurrent use.
type fakeSource struct {
	mu       sync.Mutex
	segments []Segment
	pos      int
	open     bool
}

// NewFakeSource returns a CaptureSource that replays segments in the order
// they appear in the slice. Once all segments have been consumed, Recv returns
// ErrFakeExhausted. Status reports Active=true after Open is called.
//
// This constructor is exported so tests in external (_test) packages can use
// it directly without importing an internal test helper.
func NewFakeSource(segments []Segment) CaptureSource {
	// Defensive copy so callers cannot mutate the backing slice.
	cp := make([]Segment, len(segments))
	for i, s := range segments {
		payload := make([]byte, len(s.Payload))
		copy(payload, s.Payload)
		s.Payload = payload
		cp[i] = s
	}
	return &fakeSource{segments: cp}
}

// Open marks the source as open. Idempotent on repeated calls.
func (f *fakeSource) Open() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.open = true
	return nil
}

// Recv returns the next segment in the replay sequence. If all segments have
// been consumed it returns ErrFakeExhausted. If ctx is already done before
// a segment is available, ctx.Err() is returned. Recv never blocks waiting
// for new segments — the replay is deterministic and finite.
func (f *fakeSource) Recv(ctx context.Context) (Segment, error) {
	// Check context first so a cancelled ctx is never silently ignored.
	select {
	case <-ctx.Done():
		return Segment{}, ctx.Err()
	default:
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.pos >= len(f.segments) {
		return Segment{}, ErrFakeExhausted
	}

	seg := f.segments[f.pos]
	f.pos++
	return seg, nil
}

// Close marks the source as closed. Idempotent.
func (f *fakeSource) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.open = false
	return nil
}

// Status returns Active=true if Open has been called and the source has not
// been closed, Elevated=true always (fake sources run without a driver and
// therefore have no elevation concern).
func (f *fakeSource) Status() SourceStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	return SourceStatus{
		Active:   f.open,
		Elevated: true,
		Reason:   "",
	}
}
