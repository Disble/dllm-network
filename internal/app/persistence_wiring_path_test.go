package app

import (
	"testing"

	"ollama-telemetry/internal/store/sqlite"
)

// TestDefaultDBPath_DelegatesToSharedSqliteResolver locks in that App's
// defaultDBPath and the stdio sidecar (cmd/ollama-telemetry-mcp) resolve the
// exact same file: defaultDBPath must be a thin wrapper over
// sqlite.DefaultPath(), the shared resolver, not a parallel reimplementation
// that could silently drift from it.
func TestDefaultDBPath_DelegatesToSharedSqliteResolver(t *testing.T) {
	want, wantErr := sqlite.DefaultPath()

	got, gotErr := defaultDBPath()

	if (gotErr == nil) != (wantErr == nil) {
		t.Fatalf("defaultDBPath() error = %v, sqlite.DefaultPath() error = %v", gotErr, wantErr)
	}
	if gotErr != nil {
		return
	}
	if got != want {
		t.Errorf("defaultDBPath() = %q, want %q (sqlite.DefaultPath())", got, want)
	}
}
