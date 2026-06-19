package mcp

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ollama-telemetry/internal/telemetry/inference"
)

// TestNewServer_RegistersAllToolsAndResources exercises the full
// registration wiring (NewServer) end-to-end via the SDK's in-memory
// transport pair, proving tools/resources registered against a fake
// InferenceReader are reachable through a real (non-stdio) MCP session —
// satisfying the "core testable with a faked transport" spec scenario
// without spinning up stdio.
func TestNewServer_RegistersAllToolsAndResources(t *testing.T) {
	reader := &fakeReader{
		queryResult:  []inference.Inference{{ID: "inf-1", Model: "llama3"}},
		getResult:    inference.Inference{ID: "inf-1", Model: "llama3"},
		getOK:        true,
		modelsResult: []string{"llama3"},
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
		"query_inferences": false,
		"get_inference":    false,
		"get_stats":        false,
		"list_models":      false,
	}
	for _, tool := range tools.Tools {
		if _, ok := wantTools[tool.Name]; ok {
			wantTools[tool.Name] = true
		}
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
	foundRecent := false
	for _, res := range resources.Resources {
		if res.URI == "inference://recent" {
			foundRecent = true
		}
	}
	if !foundRecent {
		t.Error("resource inference://recent was not registered")
	}

	templates, err := session.ListResourceTemplates(ctx, &mcp.ListResourceTemplatesParams{})
	if err != nil {
		t.Fatalf("ListResourceTemplates failed: %v", err)
	}
	foundTemplate := false
	for _, tpl := range templates.ResourceTemplates {
		if tpl.URITemplate == "inference://{id}" {
			foundTemplate = true
		}
	}
	if !foundTemplate {
		t.Error("resource template inference://{id} was not registered")
	}
}
