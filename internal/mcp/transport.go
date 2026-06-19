package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RunStdio runs srv over the SDK's stdio transport (newline-delimited JSON
// on stdin/stdout). It blocks until the client disconnects or ctx is
// cancelled, mirroring mcp.Server.Run's own contract.
//
// RunStdio takes an explicit mcp.Transport parameter (rather than
// constructing *mcp.StdioTransport internally and hiding it) so this
// package's design D6 seam stays real and test-visible: tests substitute
// an in-memory transport here (see transport_test.go) to prove the runner
// is a thin pass-through, and a future HTTP transport can reuse the exact
// same registration (NewServer) by calling srv.Run directly with its own
// mcp.Transport — no change needed to this function or to server.go.
func RunStdio(ctx context.Context, srv *mcp.Server, transport mcp.Transport) error {
	return srv.Run(ctx, transport)
}
