package mcp

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ollama-telemetry/internal/store"
)

// TestRunStdio_DelegatesToServerRunWithGivenTransport proves the stdio
// runner is a thin adapter: it calls srv.Run(ctx, transport) with whatever
// mcp.Transport it is given, rather than hardcoding *mcp.StdioTransport
// internally. This is the seam an HTTP transport could reuse later without
// touching tool/resource registration. We substitute the SDK's in-memory
// transport here instead of real stdio so the test runs without spinning
// up OS pipes.
func TestRunStdio_DelegatesToServerRunWithGivenTransport(t *testing.T) {
	reader := &fakeReader{resolveResult: store.ResolveInferenceContextResult{SupportedFilters: []string{"model"}}}
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

	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "resolve_inference_context", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool call reported error: %+v", res.Content)
	}
	if reader.resolveCalls != 1 {
		t.Fatalf("ResolveInferenceContext calls: got %d, want 1", reader.resolveCalls)
	}
}

func TestRunStdio_LegacyToolsAndResourcesAreUnavailable(t *testing.T) {
	reader := &fakeReader{resolveResult: store.ResolveInferenceContextResult{SupportedFilters: []string{"model"}}}
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

	for _, toolName := range []string{"query_inferences", "get_inference", "get_stats", "list_models"} {
		res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: toolName, Arguments: map[string]any{}})
		if err == nil && (res == nil || !res.IsError) {
			t.Fatalf("legacy tool %q unexpectedly succeeded", toolName)
		}
	}

	for _, uri := range []string{"inference://recent", "inference://inf-1"} {
		if _, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri}); err == nil {
			t.Fatalf("legacy resource %q unexpectedly succeeded", uri)
		}
	}
}
