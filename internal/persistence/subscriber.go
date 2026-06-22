// Package persistence implements the async write path (design D4/D7) that
// durably persists completed inferences without ever blocking the capture
// loop. A Subscriber wires into events.Bus as a SYNCHRONOUS handler — its
// HandleEvent method therefore does nothing but a single non-blocking
// channel send. All batching and INSERT work happens in a separate goroutine
// (see batch.go) that drains the channel on its own schedule.
package persistence

import (
	"context"
	"sync/atomic"
	"time"

	"dllm-network/internal/events"
	"dllm-network/internal/telemetry/inference"
)

// topicInferenceCompleted is the events.Bus topic the capture pipeline
// publishes to on each completed inference (design D7 write trigger).
const topicInferenceCompleted = "inference.completed"

// defaultBufferCapacity is the channel capacity backing the subscriber
// (design D4): large enough to absorb bursts without dropping under normal
// flush cadence, small enough to bound memory if the sink stalls.
const defaultBufferCapacity = 1024

// defaultRetentionCount is the rolling COUNT cap applied on every batch
// flush (architecture/mcp-serving-retention decision): this app mirrors
// Chrome DevTools' Network tab, where captured data is session-scoped, not
// a long-term archive. Keeping the most-recent N rows bounds storage growth
// while behaving like the reference UX (old entries roll off). No
// age-based retention is used — see pruneAgeDisabled.
const defaultRetentionCount = 5000

// pruneAgeDisabled is the maxAge value passed to Writer.Prune to disable its
// independent age-based cap (internal/store/sqlite/prune.go: "A zero value
// for either parameter disables that cap"). This project deliberately uses
// only the count cap (defaultRetentionCount) per the session-scoped
// retention decision — age-based pruning is out of scope.
const pruneAgeDisabled time.Duration = 0

// Writer is the persistence sink the Subscriber batches writes into. It is
// satisfied by *sqlite.Store; kept as a narrow interface here so this
// package's tests never need a real database.
//
// Prune is included alongside Save (not a separate port) because the
// retention decision is flush-driven: the batcher's drain goroutine calls
// both on every successful flush, and a fake test double must be able to
// observe/stub both without the test reaching for a second interface.
type Writer interface {
	Save(ctx context.Context, infs []inference.Inference) error
	Prune(ctx context.Context, maxCount int, maxAge time.Duration) error
}

// Subscriber consumes completed inferences from events.Bus and persists them
// in batches via Writer. Construct with NewSubscriber, subscribe HandleEvent
// to the bus (via Subscribe), then call Run in its own goroutine and Stop on
// shutdown.
type Subscriber struct {
	writer  Writer
	buf     chan inference.Inference
	dropped atomic.Int64

	batcher *batcher
}

// NewSubscriber creates a Subscriber with the default buffer capacity,
// batch policy (design D4: cap=1024, batchSize=64, flushInterval=250ms),
// and retention cap (defaultRetentionCount, count-only, no age cap).
func NewSubscriber(writer Writer) *Subscriber {
	return newSubscriberWithRetention(writer, defaultBufferCapacity, defaultRetentionCount, defaultBatchSize, defaultFlushInterval)
}

// newSubscriberWithCapacity is the capacity-injectable constructor used by
// tests to exercise backpressure with a small buffer instead of waiting to
// fill 1024 slots. The batcher still uses the design's default batch size,
// flush interval, and retention cap.
func newSubscriberWithCapacity(writer Writer, capacity int) *Subscriber {
	return newSubscriberWithRetention(writer, capacity, defaultRetentionCount, defaultBatchSize, defaultFlushInterval)
}

// newSubscriberWithRetention is the fully-injectable constructor used by
// tests that need to exercise the prune-on-flush wiring deterministically
// with a small retention cap and fast batch/flush thresholds, instead of
// waiting on the production defaults (cap=1024, retention=5000).
func newSubscriberWithRetention(writer Writer, capacity, retentionCount, batchSize int, flushInterval time.Duration) *Subscriber {
	buf := make(chan inference.Inference, capacity)
	return &Subscriber{
		writer:  writer,
		buf:     buf,
		batcher: newBatcher(writer, buf, batchSize, flushInterval, retentionCount),
	}
}

// Subscribe registers HandleEvent on bus for the inference-completed topic
// and returns the unsubscribe func (events.Bus.Subscribe's return value).
func (s *Subscriber) Subscribe(bus *events.Bus) func() {
	return bus.Subscribe(topicInferenceCompleted, s.HandleEvent)
}

// Run starts the batcher's drain loop and blocks until ctx is cancelled or
// Stop is called. Intended to be launched in its own goroutine from the
// App's startup path; mirrors the capture pipeline's run/stop lifecycle.
func (s *Subscriber) Run(ctx context.Context) {
	s.batcher.run(ctx)
}

// Stop signals the drain loop to flush any remaining buffered inferences
// and exit, then blocks until it has done so. Safe to call once after Run
// has been started in another goroutine.
func (s *Subscriber) Stop() {
	s.batcher.stop()
}

// HandleEvent is the events.Bus handler. bus.Publish calls handlers
// SYNCHRONOUSLY and inline with the publisher's own goroutine (the capture
// loop), so this method MUST be cheap and MUST NEVER block: it does a single
// non-blocking channel send and returns. On a full buffer it applies
// drop-oldest backpressure (design D4) — telemetry is loss-tolerant, the
// capture loop is not.
func (s *Subscriber) HandleEvent(event events.Event) {
	inf, ok := event.Payload.(inference.Inference)
	if !ok {
		return
	}
	s.enqueue(inf)
}

// enqueue performs the non-blocking drop-oldest send. It first attempts a
// direct send; if the buffer is full it drains exactly one pending item
// (the oldest) to make room, counts it as dropped, and retries once. Both
// the drain and the retry are non-blocking selects, so enqueue can never
// stall the caller even if a concurrent drain loop is racing to read from
// the same channel.
func (s *Subscriber) enqueue(inf inference.Inference) {
	select {
	case s.buf <- inf:
		return
	default:
	}

	select {
	case <-s.buf:
		s.dropped.Add(1)
	default:
		// Channel drained concurrently between the two selects — nothing to
		// drop, fall through and attempt the send below.
	}

	select {
	case s.buf <- inf:
	default:
		// Buffer refilled concurrently before the retry — drop the new
		// item rather than blocking.
		s.dropped.Add(1)
	}
}

// Dropped returns the cumulative count of inferences dropped due to
// backpressure (buffer full, no room even after one drain attempt).
func (s *Subscriber) Dropped() int64 {
	return s.dropped.Load()
}
