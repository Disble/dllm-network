package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Opt-in diagnostic logging for the capture pipeline, for field debugging of
// live traffic. Disabled unless the OLLAMA_CAPTURE_DEBUG environment variable
// is non-empty. When enabled, writes a per-segment trace to
// %TEMP%/ollama-capture-debug.log.
var (
	capDebugOnce sync.Once
	capDebugFile *os.File
	capDebugOn   bool
)

func capLog(format string, args ...any) {
	capDebugOnce.Do(func() {
		if os.Getenv("OLLAMA_CAPTURE_DEBUG") == "" {
			return
		}
		path := filepath.Join(os.TempDir(), "ollama-capture-debug.log")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return
		}
		capDebugFile = f
		capDebugOn = true
	})
	if !capDebugOn {
		return
	}
	fmt.Fprintf(capDebugFile, "%s "+format+"\n", append([]any{time.Now().Format("15:04:05.000")}, args...)...)
}
