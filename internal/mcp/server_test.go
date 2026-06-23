package mcp

import (
	"context"
	"slices"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"dllm-network/internal/store"
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

	result, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	assertExpectedTools(t, result.Tools)

	assertNoResourcesAdvertised(t, ctx, session)
}

// assertExpectedTools verifies the server advertises exactly the three
// Context7-style tools and none of the legacy tool names.
func assertExpectedTools(t *testing.T, tools []*mcp.Tool) {
	t.Helper()
	wantTools := map[string]bool{
		"resolve_inference_context": false,
		"search_inferences":         false,
		"get_inference_context":     false,
	}
	if len(tools) != len(wantTools) {
		t.Fatalf("expected exactly %d tools, got %d", len(wantTools), len(tools))
	}
	for _, tool := range tools {
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

	gotToolNames := make([]string, 0, len(tools))
	for _, tool := range tools {
		gotToolNames = append(gotToolNames, tool.Name)
	}
	for _, removedName := range []string{"query_inferences", "get_inference", "get_stats", "list_models"} {
		if slices.Contains(gotToolNames, removedName) {
			t.Fatalf("legacy tool %q must not be advertised", removedName)
		}
	}
}

// assertNoResourcesAdvertised verifies the server advertises zero MCP resources
// and zero resource templates.
func assertNoResourcesAdvertised(t *testing.T, ctx context.Context, session *mcp.ClientSession) {
	t.Helper()

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
