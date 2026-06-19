package persistence

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"ollama-telemetry/internal/events"
	"ollama-telemetry/internal/store"
	"ollama-telemetry/internal/store/sqlite"
	"ollama-telemetry/internal/telemetry/inference"
)

// TestSubscriber_PruneOnFlush_LiveWiring is the integration proof (verify
// report CRITICAL-1) that bounded retention actually fires through the LIVE
// production wiring — not just as an isolated, never-called sqlite.Store
// method. It pushes more than the retention cap's worth of completed
// inferences through a real events.Bus into a real Subscriber backed by a
// real sqlite.Store (t.TempDir), lets the drain loop flush, and asserts the
// persisted row count is bounded at the cap with the most-recent rows kept
// (oldest pruned) — exactly the decision recorded in
// architecture/mcp-serving-retention: a session-scoped rolling COUNT cap, no
// age-based retention, pruned on every batch flush.
func TestSubscriber_PruneOnFlush_LiveWiring(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "telemetry.db")
	st, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	// A small cap and small batch/flush thresholds keep this test fast and
	// deterministic while still exercising >1 flush cycle (multiple Prune
	// calls), proving prune-on-every-flush rather than prune-once.
	const cap = 5
	sub := newSubscriberWithRetention(st, 64, cap, defaultBatchSize, 20*time.Millisecond)

	bus := events.NewBus()
	unsubscribe := sub.Subscribe(bus)
	t.Cleanup(unsubscribe)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sub.Run(ctx)
	t.Cleanup(sub.Stop)

	const total = 12 // > cap, spread across multiple flush intervals
	base := time.Now().UTC()
	for i := 0; i < total; i++ {
		bus.Publish(events.Event{
			Topic: topicInferenceCompleted,
			Payload: inference.Inference{
				ID:       fmt.Sprintf("inf-live-%03d", i),
				At:       base.Add(time.Duration(i) * time.Second),
				Endpoint: "/api/generate",
				Method:   "POST",
				Model:    "llama3:8b",
				Status:   inference.PhaseCompleted,
			},
		})
		// Small stagger so successive publishes land in separate flush
		// intervals, forcing multiple Save+Prune cycles rather than one
		// single batch — proves prune runs on EVERY flush, not just once.
		time.Sleep(8 * time.Millisecond)
	}

	// Wait for the newest row to be visible (proves every publish made it
	// through Save) AND for the row count to settle at the cap (proves the
	// LAST flush's Prune call has run) — checking only "count <= cap" would
	// false-positive on an early intermediate flush cycle, before all 12
	// inferences have even been ingested.
	newestID := fmt.Sprintf("inf-live-%03d", total-1)
	deadline := time.Now().Add(3 * time.Second)
	var rowCount int
	for time.Now().Before(deadline) {
		_, newestPresent, err := st.Get(ctx, newestID)
		if err != nil {
			t.Fatalf("Get newest: %v", err)
		}
		rowCount = countLiveRows(t, st)
		if newestPresent && rowCount <= cap {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if rowCount != cap {
		t.Fatalf("expected persisted row count bounded at retention cap=%d after live flush-driven pruning, got %d", cap, rowCount)
	}

	// Most-recent-kept: the newest (total-cap .. total-1) IDs must survive;
	// the oldest (0 .. total-cap-1) must be gone.
	for i := 0; i < total; i++ {
		id := fmt.Sprintf("inf-live-%03d", i)
		_, ok, err := st.Get(ctx, id)
		if err != nil {
			t.Fatalf("Get %s: %v", id, err)
		}
		wantPresent := i >= total-cap
		if ok != wantPresent {
			t.Errorf("id %s: present=%v, want %v (index %d of %d, cap=%d)", id, ok, wantPresent, i, total, cap)
		}
	}
}

func countLiveRows(t *testing.T, st *sqlite.Store) int {
	t.Helper()

	results, err := st.Query(context.Background(), store.Filter{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	return len(results)
}
