package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"ollama-telemetry/internal/store"
	"ollama-telemetry/internal/telemetry/inference"
)

// TestOpenReadOnly_RejectsWrites locks in the hardening fix for a real gotcha
// discovered during Slice 5 apply: modernc.org/sqlite only honors the
// "mode=ro" URI query parameter on file: URI-form DSNs, NOT on bare paths.
// readOnlyDSNOptions is appended to a bare path (path+readOnlyDSNOptions, not
// "file:"+path+readOnlyDSNOptions), so "mode=ro" alone does NOT make the
// connection read-only at the driver level — it is read-only by convention
// only, and any caller that forgets to treat it as such (or any future code
// path that issues a write) would silently succeed and corrupt the
// single-writer WAL invariant (design D3).
//
// The fix adds "_pragma=query_only(true)", which modernc.org/sqlite DOES
// honor on a bare-path DSN (pragmas are applied via repeated
// "_pragma=name(value)" params regardless of DSN form — see writerDSNOptions
// for the existing precedent). query_only rejects every write at the
// connection level, including via Store.Save, independent of the
// uri-only "mode=ro" parameter.
func TestOpenReadOnly_RejectsWrites(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "telemetry.db")

	// Seed the schema via a writer connection first (OpenReadOnly never
	// migrates, so the file+schema must already exist).
	writer, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open (seed writer): %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close seed writer: %v", err)
	}

	ro, err := OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	t.Cleanup(func() { _ = ro.Close() })

	ctx := context.Background()
	inf := inference.Inference{
		ID:       "inf-should-not-be-written",
		At:       time.Date(2026, time.June, 18, 12, 0, 0, 0, time.UTC),
		Endpoint: "/api/generate",
		Method:   "POST",
		Status:   inference.PhaseCompleted,
	}

	err = ro.Save(ctx, []inference.Inference{inf})
	if err == nil {
		t.Fatal("Save: expected an error on a read-only connection, got nil — " +
			"the connection accepted a write, which violates the single-writer WAL invariant")
	}

	// Double-check at the data level: even if Save somehow returned an error
	// after a partial write, the row must not be visible.
	if _, ok, getErr := ro.Get(ctx, inf.ID); getErr == nil && ok {
		t.Fatal("Get: row was persisted despite Save erroring on a read-only connection")
	}
}

// TestOpenReadOnly_ReadPathsStillWork guards against the hardening fix
// (query_only pragma) accidentally breaking the legitimate read surface the
// stdio sidecar depends on (Query/Get/Stats/Models all go through the same
// underlying *sql.DB).
func TestOpenReadOnly_ReadPathsStillWork(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "telemetry.db")

	writer, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open (seed writer): %v", err)
	}

	seeded := inference.Inference{
		ID:       "inf-readable",
		At:       time.Date(2026, time.June, 18, 12, 30, 0, 0, time.UTC),
		Endpoint: "/api/generate",
		Method:   "POST",
		Model:    "gemma3:12b",
		Status:   inference.PhaseCompleted,
	}
	ctx := context.Background()
	if err := writer.Save(ctx, []inference.Inference{seeded}); err != nil {
		t.Fatalf("Save (seed): %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close seed writer: %v", err)
	}

	ro, err := OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	t.Cleanup(func() { _ = ro.Close() })

	got, ok, err := ro.Get(ctx, seeded.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatalf("Get: expected found=true for id %q", seeded.ID)
	}
	if got.ID != seeded.ID {
		t.Errorf("Get: got ID %q, want %q", got.ID, seeded.ID)
	}

	models, err := ro.Models(ctx)
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if len(models) != 1 || models[0] != "gemma3:12b" {
		t.Errorf("Models: got %v, want [gemma3:12b]", models)
	}

	stats, err := ro.Stats(ctx, store.Filter{})
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.Count != 1 {
		t.Errorf("Stats.Count: got %d, want 1", stats.Count)
	}
}
