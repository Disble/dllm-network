package sqlite

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dllm-network/internal/telemetry/inference"
)

// TestWAL_WriterAndReadOnlyReader_NoBusyErrors (task 1.8, design D3 smoke
// test) opens a writer connection and a separate read-only connection
// against the same temp-dir database file, writes rows from a goroutine
// while polling the row count from the read-only side, and asserts the
// reader observes a monotonically growing count with zero SQLITE_BUSY
// errors. This is the cross-process contract the stdio sidecar depends on:
// it must be able to read while the GUI app holds the writer connection
// open. Skipped under -short since it depends on real wall-clock timing
// across goroutines.
func TestWAL_WriterAndReadOnlyReader_NoBusyErrors(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping WAL concurrency smoke test in -short mode")
	}
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "telemetry.db")

	writer, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open writer: %v", err)
	}
	t.Cleanup(func() { _ = writer.Close() })

	reader, err := OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	t.Cleanup(func() { _ = reader.Close() })

	const totalRows = 50
	ctx := context.Background()
	base := time.Now().UTC()

	writeErrs := make(chan error, 1)
	go func() {
		defer close(writeErrs)
		for i := 0; i < totalRows; i++ {
			inf := inference.Inference{
				ID:       "inf-concurrency-" + time.Duration(i).String(),
				At:       base.Add(time.Duration(i) * time.Millisecond),
				Endpoint: "/api/generate",
				Method:   "POST",
				Model:    "llama3:8b",
				Status:   inference.PhaseCompleted,
			}
			if err := writer.Save(ctx, []inference.Inference{inf}); err != nil {
				writeErrs <- err
				return
			}
			time.Sleep(2 * time.Millisecond)
		}
	}()

	lastCount := -1
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var n int
		row := reader.db.QueryRow(`SELECT COUNT(*) FROM inferences`)
		if err := row.Scan(&n); err != nil {
			if isBusyErr(err) {
				t.Fatalf("reader hit SQLITE_BUSY: %v", err)
			}
			t.Fatalf("reader count query: %v", err)
		}
		if n < lastCount {
			t.Fatalf("row count regressed: had %d, now %d", lastCount, n)
		}
		lastCount = n
		if n >= totalRows {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if err := <-writeErrs; err != nil {
		t.Fatalf("writer goroutine: %v", err)
	}
	if lastCount < totalRows {
		t.Fatalf("expected to observe %d rows from the read-only side, last saw %d", totalRows, lastCount)
	}
}

// isBusyErr matches on the driver's error text rather than a sentinel error
// value: modernc.org/sqlite does not export a stable errors.Is-compatible
// SQLITE_BUSY sentinel, so this string check is the pragmatic boundary —
// scoped to this one concurrency smoke test, not used anywhere in
// production code paths.
func isBusyErr(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "busy")
}
