package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Serve runs srv over the real stdio transport (stdin/stdout). This is the
// production entrypoint Slice 5's sidecar binary calls; it is a one-line
// wrapper over RunStdio so the &mcp.StdioTransport{} construction lives in
// exactly one place. Tests exercise RunStdio directly with an in-memory
// transport instead of calling Serve, since Serve's real stdio pipes are
// not unit-testable without spawning a subprocess.
func Serve(ctx context.Context, srv *mcp.Server) error {
	return RunStdio(ctx, srv, &mcp.StdioTransport{})
}
