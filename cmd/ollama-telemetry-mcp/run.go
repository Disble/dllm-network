// Package main implements the stdio MCP sidecar binary
// (cmd/ollama-telemetry-mcp). It is a thin process that opens the SAME
// SQLite database the GUI app writes to (resolved via
// internal/store/sqlite.DefaultPath, design D3/D7) as a READ-ONLY
// connection, builds the transport-decoupled MCP server core
// (internal/mcp.NewServer), and serves it over stdio
// (internal/mcp.Serve) until the client disconnects or the process is
// signaled to stop.
//
// This binary depends on internal/store/sqlite (as a store.InferenceReader
// implementation) and internal/mcp — never on internal/app, and never
// directly on the MCP SDK (github.com/modelcontextprotocol/go-sdk), which
// stays quarantined inside internal/mcp per design D6.
package main

import (
	"context"
	"errors"
	"fmt"

	"ollama-telemetry/internal/store"
)

// closer is satisfied by *sqlite.Store (and any test double) so run can
// release the read-only connection once serve returns.
type closer interface {
	Close() error
}

// runDeps are the injectable seams for run, so the wiring logic itself
// (path resolution -> existence check -> open -> serve -> close) is unit
// testable without touching a real SQLite file or stdio.
type runDeps struct {
	// resolvePath returns the shared DB path (production: sqlite.DefaultPath).
	resolvePath func() (string, error)

	// exists reports whether the DB file at path already exists. The
	// sidecar must never create the database — only the GUI writer does
	// (via sqlite.Open) — so run() checks this BEFORE attempting to open
	// anything read-only.
	exists func(path string) bool

	// openReader opens path as a read-only store.InferenceReader plus a
	// closer to release it (production: sqlite.OpenReadOnly).
	openReader func(path string) (store.InferenceReader, closer, error)

	// serve runs the MCP server backed by reader until the transport
	// session ends or ctx is cancelled (production: build the server via
	// internal/mcp.NewServer then run it via internal/mcp.Serve).
	serve func(ctx context.Context, reader store.InferenceReader) error
}

// errDBNotFound is returned by run when the resolved DB path does not exist
// yet — i.e. the GUI app has never run on this machine. The sidecar is a
// reader; it must fail clearly here rather than silently creating an empty
// database that would mask the real "no data yet" condition.
var errDBNotFound = errors.New("telemetry database not found — start the ollama-telemetry GUI app at least once before running the MCP sidecar")

// run wires path resolution, the missing-DB guard, the read-only store
// open, the MCP serve call, and cleanup. It returns a non-nil error for
// every failure mode the caller (main) should report to stderr with a
// non-zero exit.
func run(ctx context.Context, deps runDeps) error {
	path, err := deps.resolvePath()
	if err != nil {
		return fmt.Errorf("resolve database path: %w", err)
	}

	if !deps.exists(path) {
		return errDBNotFound
	}

	reader, c, err := deps.openReader(path)
	if err != nil {
		return fmt.Errorf("open database read-only at %s: %w", path, err)
	}
	if c != nil {
		defer func() { _ = c.Close() }()
	}

	if err := deps.serve(ctx, reader); err != nil {
		return fmt.Errorf("serve mcp over stdio: %w", err)
	}

	return nil
}
