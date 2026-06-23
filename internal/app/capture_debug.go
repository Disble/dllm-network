package app

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Opt-in diagnostic logging for the capture pipeline, for field debugging of
// live traffic. Disabled unless the OLLAMA_CAPTURE_DEBUG environment variable
// is non-empty. When enabled, writes a per-segment trace to a temporary file
// with an unpredictable name.
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
		f, err := os.CreateTemp("", "ollama-capture-debug-*.log")
		if err != nil {
			return
		}
		fmt.Fprintf(os.Stderr, "ollama capture debug log: %s\n", f.Name())
		fmt.Fprintf(f, "ollama capture debug log: %s\n", f.Name())
		capDebugFile = f
		capDebugOn = true
	})
	if !capDebugOn {
		return
	}
	fmt.Fprintf(capDebugFile, "%s "+format+"\n", append([]any{time.Now().Format("15:04:05.000")}, args...)...)
}
