package sqlite

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"ollama-telemetry/internal/telemetry/inference"
)

// seedInferences saves count synthetic inferences spaced one minute apart
// starting at base, returning them in insertion (chronological) order. A
// shared fixture avoids duplicating boilerplate across the prune tests.
func seedInferences(t *testing.T, st *Store, base time.Time, count int) []inference.Inference {
	t.Helper()

	infs := make([]inference.Inference, 0, count)
	for i := 0; i < count; i++ {
		infs = append(infs, inference.Inference{
			ID:       fmt.Sprintf("inf-%03d", i),
			At:       base.Add(time.Duration(i) * time.Minute),
			Endpoint: "/api/generate",
			Method:   "POST",
			Model:    "llama3:8b",
			Status:   inference.PhaseCompleted,
		})
	}

	if err := st.Save(context.Background(), infs); err != nil {
		t.Fatalf("seed Save: %v", err)
	}
	return infs
}

// TestStore_PruneByCount (task 1.7) verifies Prune deletes the oldest rows
// once the total row count exceeds maxCount, leaving exactly maxCount rows —
// the newest ones — queryable.
func TestStore_PruneByCount(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "telemetry.db")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	base := time.Date(2026, time.June, 18, 12, 0, 0, 0, time.UTC)
	infs := seedInferences(t, st, base, 10)

	ctx := context.Background()
	if err := st.Prune(ctx, 4, 0); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	count := countRows(t, st)
	if count != 4 {
		t.Fatalf("expected 4 rows after pruning to maxCount=4, got %d", count)
	}

	// The 4 newest (last seeded) rows must remain; the 6 oldest must be gone.
	for i, inf := range infs {
		_, ok, err := st.Get(ctx, inf.ID)
		if err != nil {
			t.Fatalf("Get %s: %v", inf.ID, err)
		}
		wantPresent := i >= len(infs)-4
		if ok != wantPresent {
			t.Errorf("id %s: present=%v, want %v (index %d of %d)", inf.ID, ok, wantPresent, i, len(infs))
		}
	}
}

// TestStore_PruneByAge (task 1.7) verifies Prune deletes rows whose At
// timestamp is older than maxAge relative to time.Now, regardless of count.
func TestStore_PruneByAge(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "telemetry.db")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	now := time.Now().UTC()
	ctx := context.Background()

	stale := inference.Inference{
		ID: "inf-stale", At: now.Add(-48 * time.Hour),
		Endpoint: "/api/generate", Method: "POST", Model: "llama3:8b",
		Status: inference.PhaseCompleted,
	}
	fresh := inference.Inference{
		ID: "inf-fresh", At: now.Add(-1 * time.Minute),
		Endpoint: "/api/generate", Method: "POST", Model: "llama3:8b",
		Status: inference.PhaseCompleted,
	}
	if err := st.Save(ctx, []inference.Inference{stale, fresh}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := st.Prune(ctx, 0, 24*time.Hour); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	if _, ok, err := st.Get(ctx, stale.ID); err != nil {
		t.Fatalf("Get stale: %v", err)
	} else if ok {
		t.Errorf("expected stale row to be pruned, but it is still present")
	}

	if _, ok, err := st.Get(ctx, fresh.ID); err != nil {
		t.Fatalf("Get fresh: %v", err)
	} else if !ok {
		t.Errorf("expected fresh row to survive pruning, but it is missing")
	}
}

func countRows(t *testing.T, st *Store) int {
	t.Helper()

	var n int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM inferences`).Scan(&n); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return n
}
