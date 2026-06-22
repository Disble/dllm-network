// Package capture defines the CaptureSource port and its implementations.
// Only windivert_windows.go contains OS-bound syscall code. All other files
// in this package are pure Go and build cross-platform without administrator
// rights or a capture driver installed.
package capture

import (
	"context"
	"time"

	"dllm-network/internal/capture/reassembly"
)

// FourTuple identifies a TCP connection by its endpoint addresses. It is the
// same type as reassembly.FourTuple re-exported here for callers that import
// only the capture package.
type FourTuple = reassembly.FourTuple

// Direction identifies which direction a packet travelled relative to the
// Ollama server (port 11434).
type Direction = reassembly.Direction

const (
	// DirToServer marks a segment heading toward the Ollama server.
	DirToServer = reassembly.DirToServer
	// DirFromServer marks a segment heading from the Ollama server to the client.
	DirFromServer = reassembly.DirFromServer
)

// Segment is a single captured TCP payload, parsed from a raw IP+TCP packet.
// It carries the connection 4-tuple, direction, raw payload bytes, the TCP
// sequence number (for reassembly ordering), and a wall-clock timestamp.
type Segment struct {
	Tuple   FourTuple
	Dir     Direction
	Payload []byte
	SeqNo   uint32
	At      time.Time
}

// SourceStatus describes the current operational state of a CaptureSource.
// It is always safe to read — even when the source is inactive — so callers
// can surface degradation information without risking a nil-dereference or
// panic.
type SourceStatus struct {
	// Active is true when the source is open and delivering segments.
	Active bool
	// Elevated is true when the process has the required OS privileges
	// (administrator on Windows) to load the capture driver.
	Elevated bool
	// Reason is a human-readable explanation when Active is false, e.g.
	// "requires administrator" or "closed".
	Reason string
}

// CaptureSource is the Strategy port for OS-level packet capture.
// Implementations: windivertSource (real, windows+admin), fakeSource
// (test replay of golden byte fixtures), noopSource (rollback / unelevated
// fallback that surfaces degraded status without crashing).
//
// Lifecycle: Open → Recv (loop) → Close.  Close is idempotent.
// Status may be called at any time after construction.
type CaptureSource interface {
	// Open initialises the underlying capture handle.  It must be called
	// once before Recv.  Returning an error does NOT mean the source is
	// broken — callers should inspect Status() to distinguish "open but
	// unelevated (graceful degradation)" from a hard failure.
	Open() error

	// Recv blocks until a segment is available or ctx is done.  It returns
	// the segment and nil on success; ctx.Err() (or a wrapped equivalent)
	// when the context is cancelled; or a non-nil error for driver-level
	// failures.
	Recv(ctx context.Context) (Segment, error)

	// Close releases the underlying capture handle.  Idempotent — safe to
	// call multiple times.
	Close() error

	// Status returns the current operational state of the source.  It is
	// always safe to call, even before Open or after Close.
	Status() SourceStatus
}
