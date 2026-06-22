package main

import (
	"context"
	"fmt"
	"os"

	"dllm-network/internal/mcp"
	"dllm-network/internal/store"
	"dllm-network/internal/store/sqlite"
)

// main wires the production seams and delegates to run. It stays thin —
// all decision logic (path resolution, the missing-DB guard, store-open,
// server construction) lives in run/runDeps so it is unit-testable without
// real stdio or a real database file.
func main() {
	if err := run(context.Background(), productionDeps()); err != nil {
		fmt.Fprintln(os.Stderr, "dllm-network-mcp:", err)
		os.Exit(1)
	}
}

// productionDeps wires run's seams to the real filesystem, the real
// read-only sqlite.Store, and the real internal/mcp stdio server.
func productionDeps() runDeps {
	return runDeps{
		resolvePath: sqlite.DefaultPath,
		exists:      fileExists,
		openReader:  openReadOnlyStore,
		serve:       serveStdio,
	}
}

// fileExists reports whether path names an existing file. It is the
// missing-DB guard: the sidecar must never bring a fresh database into
// existence just because it ran before the GUI app did, and
// sqlite.OpenReadOnly itself cannot be trusted to refuse a missing file
// (the underlying driver can lazily create one depending on DSN form), so
// run() checks existence explicitly before ever calling openReader.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// openReadOnlyStore adapts sqlite.OpenReadOnly to runDeps.openReader's
// signature: the GUI app is the sole writer (design D3/D7); this sidecar
// only ever reads.
func openReadOnlyStore(path string) (store.InferenceReader, closer, error) {
	s, err := sqlite.OpenReadOnly(path)
	if err != nil {
		return nil, nil, err
	}
	return s, s, nil
}

// serveStdio builds the transport-decoupled MCP server core
// (internal/mcp.NewServer) around reader and runs it over the real stdio
// transport (internal/mcp.Serve) until the client disconnects or ctx is
// cancelled. This is the ONLY place in this binary that touches
// internal/mcp's production entrypoints — the MCP SDK itself
// (github.com/modelcontextprotocol/go-sdk) is never imported here or
// anywhere outside internal/mcp.
func serveStdio(ctx context.Context, reader store.InferenceReader) error {
	srv := mcp.NewServer(reader)
	return mcp.Serve(ctx, srv)
}
