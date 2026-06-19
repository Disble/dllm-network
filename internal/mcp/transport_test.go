package mcp

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestRunStdio_DelegatesToServerRunWithGivenTransport proves the stdio
// runner is a thin adapter: it calls srv.Run(ctx, transport) with whatever
// mcp.Transport it is given, rather than hardcoding *mcp.StdioTransport
// internally. This is the seam an HTTP transport could reuse later without
// touching tool/resource registration. We substitute the SDK's in-memory
// transport here instead of real stdio so the test runs without spinning
// up OS pipes.
func TestRunStdio_DelegatesToServerRunWithGivenTransport(t *testing.T) {
	reader := &fakeReader{modelsResult: []string{"llama3"}}
	srv := NewServer(reader)

	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runErrCh := make(chan error, 1)
	go func() { runErrCh <- RunStdio(ctx, srv, serverTransport) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect failed: %v", err)
	}
	defer session.Close()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_models", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool call reported error: %+v", res.Content)
	}
}
