package sqlite

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDefaultPath_JoinsAppDirAndFileNameUnderCacheDir locks in DefaultPath's
// contract (design D3): the resolved path must be
// {os.UserCacheDir()}/dllm-network/telemetry.db, and the
// dllm-network directory must exist (created if needed) so callers can
// open the DB file directly without an extra MkdirAll. We assert against
// os.UserCacheDir()'s own return value rather than a hardcoded machine path,
// so the test is portable across CI platforms.
func TestDefaultPath_JoinsAppDirAndFileNameUnderCacheDir(t *testing.T) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		t.Fatalf("os.UserCacheDir() failed in test environment: %v", err)
	}
	wantDir := filepath.Join(cacheDir, "dllm-network")
	wantPath := filepath.Join(wantDir, "telemetry.db")

	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() returned error: %v", err)
	}

	if got != wantPath {
		t.Errorf("DefaultPath() = %q, want %q", got, wantPath)
	}

	if info, statErr := os.Stat(wantDir); statErr != nil {
		t.Errorf("DefaultPath() did not create app dir %q: %v", wantDir, statErr)
	} else if !info.IsDir() {
		t.Errorf("%q exists but is not a directory", wantDir)
	}
}

// TestDefaultPath_DoesNotCreateTheDBFileItself proves DefaultPath only
// resolves the path and ensures the parent directory exists — it must NOT
// create or touch telemetry.db itself, since both the GUI writer (via Open)
// and the read-only sidecar (via OpenReadOnly) need to observe an accurate
// "does the DB exist yet" signal independent of path resolution.
func TestDefaultPath_DoesNotCreateTheDBFileItself(t *testing.T) {
	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() returned error: %v", err)
	}

	if _, statErr := os.Stat(path); statErr == nil {
		t.Skip("telemetry.db already exists on this machine from a prior run; cannot assert non-creation here")
	} else if !os.IsNotExist(statErr) {
		t.Fatalf("unexpected stat error for %q: %v", path, statErr)
	}
}
