package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ollama-telemetry/internal/store"
)

// listModelsInput is the typed input schema for the list_models tool. It
// takes no parameters; the type exists so AddTool can infer an (empty)
// input schema consistently with the other tools.
type listModelsInput struct{}

// listModelsOutput is the typed output schema for the list_models tool.
type listModelsOutput struct {
	Models []string `json:"models"`
}

// handleListModels returns a tool handler bound to reader, delegating
// directly to reader.Models.
func handleListModels(reader store.InferenceReader) mcp.ToolHandlerFor[listModelsInput, listModelsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ listModelsInput) (*mcp.CallToolResult, listModelsOutput, error) {
		models, err := reader.Models(ctx)
		if err != nil {
			return nil, listModelsOutput{}, err
		}
		return nil, listModelsOutput{Models: models}, nil
	}
}
