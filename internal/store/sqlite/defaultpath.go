package sqlite

import (
	"fmt"
	"os"
	"path/filepath"
)

// defaultDBFileName is the SQLite database file name under the app's local
// (non-roaming) data directory (design D3:
// %LOCALAPPDATA%/dllm-network/telemetry.db on Windows, via
// os.UserCacheDir()).
const defaultDBFileName = "telemetry.db"

// defaultAppDirName is the per-app subdirectory created under the resolved
// cache directory.
const defaultAppDirName = "dllm-network"

// DefaultPath resolves the production SQLite database path under the
// user's local (non-roaming) data directory, creating the parent
// "dllm-network" directory if needed.
//
// On Windows os.UserCacheDir() resolves to %LOCALAPPDATA% (design D3); a
// telemetry DB must NOT roam across machines, so os.UserConfigDir()
// (%APPDATA%\Roaming) would be the wrong choice here.
//
// DefaultPath lives in this package (rather than internal/app) so both the
// GUI writer (internal/app, via Open) and the stdio sidecar (cmd/, via
// OpenReadOnly) resolve the exact same file without the sidecar needing to
// import internal/app, which it must not (design D6/D7: the sidecar depends
// only on this package and internal/mcp).
//
// DefaultPath does NOT create or touch the database file itself — only its
// parent directory — so callers can distinguish "DB not opened yet" from
// "DB never existed" by stat'ing the returned path before opening.
func DefaultPath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("sqlite: resolve user cache dir: %w", err)
	}
	appDir := filepath.Join(dir, defaultAppDirName)
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return "", fmt.Errorf("sqlite: create app data dir: %w", err)
	}
	return filepath.Join(appDir, defaultDBFileName), nil
}
