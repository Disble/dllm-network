package mcp

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ollama-telemetry/internal/store"
)

// TestNewServer_RegistersExactThreeToolsAndNoResources exercises the full
// registration wiring (NewServer) end-to-end via the SDK's in-memory
// transport pair, proving the revised Context7-style contract advertises
// exactly three tools and no MCP resources.
func TestNewServer_RegistersExactThreeToolsAndNoResources(t *testing.T) {
	reader := &fakeReader{
		resolveResult: store.ResolveInferenceContextResult{
			SupportedFilters: []string{"model", "endpoint", "status", "since", "until"},
		},
	}

	srv := NewServer(reader)
	if srv == nil {
		t.Fatal("NewServer returned nil")
	}

	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverErrCh := make(chan error, 1)
	go func() { serverErrCh <- srv.Run(ctx, serverTransport) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect failed: %v", err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	wantTools := map[string]bool{
		"resolve_inference_context": false,
		"search_inferences":         false,
		"get_inference_context":     false,
	}
	if len(tools.Tools) != len(wantTools) {
		t.Fatalf("expected exactly %d tools, got %d", len(wantTools), len(tools.Tools))
	}
	for _, tool := range tools.Tools {
		if _, ok := wantTools[tool.Name]; ok {
			wantTools[tool.Name] = true
			continue
		}
		t.Errorf("unexpected tool registered: %q", tool.Name)
	}
	for name, found := range wantTools {
		if !found {
			t.Errorf("tool %q was not registered", name)
		}
	}

	resources, err := session.ListResources(ctx, &mcp.ListResourcesParams{})
	if err != nil {
		t.Fatalf("ListResources failed: %v", err)
	}
	if len(resources.Resources) != 0 {
		t.Fatalf("expected zero resources, got %d", len(resources.Resources))
	}

	templates, err := session.ListResourceTemplates(ctx, &mcp.ListResourceTemplatesParams{})
	if err != nil {
		t.Fatalf("ListResourceTemplates failed: %v", err)
	}
	if len(templates.ResourceTemplates) != 0 {
		t.Fatalf("expected zero resource templates, got %d", len(templates.ResourceTemplates))
	}
}
