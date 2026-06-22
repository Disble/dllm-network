//go:build windows

package capture

import (
	// embed is required for the go:embed directives below that bundle WinDivert
	// assets into the binary.
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// windivertDLL and windivertSYS hold the WinDivert driver assets embedded at
// compile time. They are written beside the executable on first use so the
// binary is self-contained — the user does not need to install WinDivert
// separately. WinDivert.dll loads WinDivert64.sys from the same directory, so
// both files must be co-located with the binary.
//
// The go:embed paths are relative to the package source file, which lives at
// internal/capture/. The checked-in assets live under internal/capture/windivert/
// so go:embed can reference them directly (go:embed does not allow ".." path
// traversal). Local extraction, POC, and verification files under tools/ are
// developer scratch artifacts and are intentionally ignored.

//go:embed windivert/WinDivert.dll
var windivertDLL []byte

//go:embed windivert/WinDivert64.sys
var windivertSYS []byte

var (
	writeOnce  sync.Once
	writeError error
)

// EnsureWinDivertAssets writes WinDivert.dll and WinDivert64.sys beside the
// running executable if they are not already present. It is called
// automatically by NewWinDivertSource before attempting to load the DLL, so
// the binary is self-contained.
//
// The write is guarded by sync.Once — subsequent calls are no-ops that return
// the result of the first invocation. If the executable directory is not
// writable (e.g. Program Files without elevation) the function returns an
// error; callers should surface this in SourceStatus.Reason rather than
// crashing.
func EnsureWinDivertAssets() error {
	writeOnce.Do(func() {
		writeError = writeAssets()
	})
	return writeError
}

func writeAssets() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("windivert embed: cannot determine executable path: %w", err)
	}
	dir := filepath.Dir(exePath)

	assets := []struct {
		name string
		data []byte
	}{
		{"WinDivert.dll", windivertDLL},
		{"WinDivert64.sys", windivertSYS},
	}

	for _, a := range assets {
		dest := filepath.Join(dir, a.name)
		if _, err := os.Stat(dest); err == nil {
			// File already exists — skip (do not overwrite a newer installed
			// version).
			continue
		}
		if err := os.WriteFile(dest, a.data, 0o644); err != nil {
			return fmt.Errorf("windivert embed: write %s: %w", a.name, err)
		}
	}

	return nil
}
