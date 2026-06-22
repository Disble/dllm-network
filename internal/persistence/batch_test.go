package persistence

import (
	"context"
	"testing"
	"time"

	"dllm-network/internal/telemetry/inference"
)

// TestSubscriber_BatchFlush_OnSizeOrInterval asserts the batcher flushes
// when the accumulated batch reaches batchSize, without waiting for the
// flush-interval ticker — proving the "whichever first" policy's size arm.
func TestSubscriber_BatchFlush_OnSizeOrInterval(t *testing.T) {
	t.Parallel()

	writer := &fakeWriter{}
	const capacity = 256
	buf := make(chan inference.Inference, capacity)
	// Long interval so only the size threshold can trigger a flush within
	// the test's timeout — isolates the "size" arm of the policy.
	b := newBatcher(writer, buf, 8, time.Hour, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.run(ctx)
	t.Cleanup(b.stop)

	for i := 0; i < 8; i++ {
		buf <- inference.Inference{ID: "inf-size-" + itoaTest(i)}
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && writer.RowCount() < 8 {
		time.Sleep(10 * time.Millisecond)
	}

	if got := writer.RowCount(); got != 8 {
		t.Fatalf("expected 8 rows flushed once batchSize reached, got %d", got)
	}
}

// TestSubscriber_BatchFlush_OnInterval asserts the batcher flushes a
// partial (sub-batchSize) accumulation once the flush interval ticker
// fires — proving the "whichever first" policy's interval arm.
func TestSubscriber_BatchFlush_OnInterval(t *testing.T) {
	t.Parallel()

	writer := &fakeWriter{}
	buf := make(chan inference.Inference, 256)
	// Large batchSize so only the interval ticker can trigger the flush.
	b := newBatcher(writer, buf, 1000, 50*time.Millisecond, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.run(ctx)
	t.Cleanup(b.stop)

	buf <- inference.Inference{ID: "inf-interval-1"}
	buf <- inference.Inference{ID: "inf-interval-2"}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && writer.RowCount() < 2 {
		time.Sleep(10 * time.Millisecond)
	}

	if got := writer.RowCount(); got != 2 {
		t.Fatalf("expected 2 rows flushed once flush interval elapsed, got %d", got)
	}
}

// TestSubscriber_FinalFlushOnStop asserts that Stop drains and flushes any
// remaining buffered inferences that never reached batchSize or an
// interval tick — no enqueued item should be silently lost on shutdown.
func TestSubscriber_FinalFlushOnStop(t *testing.T) {
	t.Parallel()

	writer := &fakeWriter{}
	buf := make(chan inference.Inference, 256)
	// Both thresholds far in the future — only stop's final flush can
	// persist these rows within the test.
	b := newBatcher(writer, buf, 1000, time.Hour, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.run(ctx)

	buf <- inference.Inference{ID: "inf-stop-1"}
	buf <- inference.Inference{ID: "inf-stop-2"}
	buf <- inference.Inference{ID: "inf-stop-3"}

	// Give the drain loop a moment to have these sitting in the channel
	// (not yet flushed) before we stop it.
	time.Sleep(20 * time.Millisecond)
	if got := writer.RowCount(); got != 0 {
		t.Fatalf("expected no flush before stop (size/interval thresholds not met), got %d rows", got)
	}

	b.stop()

	if got := writer.RowCount(); got != 3 {
		t.Fatalf("expected stop to flush the 3 remaining buffered rows, got %d", got)
	}
}

// TestBatcher_PruneCalledAfterEachSuccessfulFlush asserts the seam directly
// (unit-level, fakeWriter, no real DB): every successful Save is followed
// by exactly one Prune call carrying the batcher's configured retentionCount
// and pruneAgeDisabled (age cap off) — never the other way around, and never
// a Prune-without-a-preceding-Save.
func TestBatcher_PruneCalledAfterEachSuccessfulFlush(t *testing.T) {
	t.Parallel()

	writer := &fakeWriter{}
	buf := make(chan inference.Inference, 256)
	const retentionCount = 3
	b := newBatcher(writer, buf, 2, time.Hour, retentionCount)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.run(ctx)
	t.Cleanup(b.stop)

	buf <- inference.Inference{ID: "inf-prune-1"}
	buf <- inference.Inference{ID: "inf-prune-2"} // reaches batchSize=2, triggers a flush

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(writer.PruneCalls()) == 0 {
		time.Sleep(10 * time.Millisecond)
	}

	calls := writer.PruneCalls()
	if len(calls) != 1 {
		t.Fatalf("expected exactly 1 Prune call after 1 flush, got %d", len(calls))
	}
	if calls[0].maxCount != retentionCount {
		t.Errorf("expected Prune maxCount=%d, got %d", retentionCount, calls[0].maxCount)
	}
	if calls[0].maxAge != pruneAgeDisabled {
		t.Errorf("expected Prune maxAge=%v (disabled), got %v", pruneAgeDisabled, calls[0].maxAge)
	}
}

// TestBatcher_PruneSkippedOnSaveError asserts pruning is skipped when Save
// fails — pruning the table based on a flush that didn't actually persist
// would risk deleting rows that were never durably written.
func TestBatcher_PruneSkippedOnSaveError(t *testing.T) {
	t.Parallel()

	writer := &fakeWriter{saveErr: errFakeSave}
	buf := make(chan inference.Inference, 256)
	b := newBatcher(writer, buf, 1, time.Hour, 5)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.run(ctx)
	t.Cleanup(b.stop)

	buf <- inference.Inference{ID: "inf-prune-err-1"}

	time.Sleep(100 * time.Millisecond)

	if got := len(writer.PruneCalls()); got != 0 {
		t.Fatalf("expected 0 Prune calls when Save fails, got %d", got)
	}
}

func itoaTest(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
