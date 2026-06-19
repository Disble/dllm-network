package persistence

import (
	"context"
	"time"

	"ollama-telemetry/internal/telemetry/inference"
)

// defaultBatchSize and defaultFlushInterval implement design D4's batch
// policy: flush at batchSize OR flushInterval, whichever comes first.
const (
	defaultBatchSize     = 64
	defaultFlushInterval = 250 * time.Millisecond
)

// batcher owns the drain loop that reads off the Subscriber's channel and
// writes batched INSERTs via Writer. It runs in its own goroutine, entirely
// separate from the bus-handler goroutine (the capture loop) that feeds the
// channel — this separation is what keeps HandleEvent non-blocking.
//
// It also owns the retention prune trigger (architecture/mcp-serving-
// retention decision): every successful flush is immediately followed by a
// Writer.Prune(ctx, retentionCount, pruneAgeDisabled) call, IN THIS SAME
// drain goroutine — never in the bus-handler goroutine — so pruning can
// never add latency to the synchronous bus.Publish call path.
type batcher struct {
	writer         Writer
	buf            <-chan inference.Inference
	batchSize      int
	flushInterval  time.Duration
	retentionCount int

	stopCh chan struct{}
	doneCh chan struct{}
}

// newBatcher creates a batcher reading from buf. batchSize and
// flushInterval of zero fall back to the design defaults (64 / 250ms).
// retentionCount of zero disables the count cap (Writer.Prune's own
// convention: a zero value for either parameter disables that cap) — tests
// that don't care about retention can pass 0.
func newBatcher(writer Writer, buf <-chan inference.Inference, batchSize int, flushInterval time.Duration, retentionCount int) *batcher {
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}
	if flushInterval <= 0 {
		flushInterval = defaultFlushInterval
	}
	return &batcher{
		writer:         writer,
		buf:            buf,
		batchSize:      batchSize,
		flushInterval:  flushInterval,
		retentionCount: retentionCount,
		stopCh:         make(chan struct{}),
		doneCh:         make(chan struct{}),
	}
}

// run is the long-running drain loop. It accumulates inferences off buf and
// flushes via Writer.Save whenever the accumulated batch reaches batchSize
// OR the flushInterval ticker fires, whichever happens first. On stop it
// drains and flushes any remaining buffered items before returning, so no
// inference enqueued before shutdown is silently lost.
func (b *batcher) run(ctx context.Context) {
	defer close(b.doneCh)

	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()

	pending := make([]inference.Inference, 0, b.batchSize)

	flush := func() {
		if len(pending) == 0 {
			return
		}
		if err := b.writer.Save(ctx, pending); err == nil {
			// Prune runs only after a successful flush, in this drain
			// goroutine — the bus-handler goroutine (HandleEvent) never
			// touches the writer, so pruning can never block
			// bus.Publish's synchronous call path (design D7 constraint).
			_ = b.writer.Prune(ctx, b.retentionCount, pruneAgeDisabled)
		}
		pending = pending[:0]
	}

	for {
		select {
		case inf, ok := <-b.buf:
			if !ok {
				flush()
				return
			}
			pending = append(pending, inf)
			if len(pending) >= b.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-b.stopCh:
			b.drainRemaining(ctx, &pending)
			flush()
			return
		}
	}
}

// drainRemaining performs a final non-blocking drain of any items already
// sitting in the channel buffer at stop time, appending them to pending so
// the subsequent flush persists them too. It never blocks waiting for new
// sends — only items already queued are collected.
func (b *batcher) drainRemaining(_ context.Context, pending *[]inference.Inference) {
	for {
		select {
		case inf, ok := <-b.buf:
			if !ok {
				return
			}
			*pending = append(*pending, inf)
		default:
			return
		}
	}
}

// stop signals the drain loop to perform its final flush and exit, then
// blocks until it has done so.
func (b *batcher) stop() {
	close(b.stopCh)
	<-b.doneCh
}
