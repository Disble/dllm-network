package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ollama-telemetry/internal/store"
	"ollama-telemetry/internal/telemetry/inference"
)

// getInferenceInput is the typed input schema for the get_inference tool.
type getInferenceInput struct {
	ID string `json:"id" jsonschema:"the inference id to fetch"`
}

// getInferenceOutput is the typed output schema for the get_inference tool.
// Found distinguishes "no such id" (Found=false, zero Inference, no error)
// from a real fetch failure (handled as a tool error instead), per the
// spec's "Unknown ID" scenario.
type getInferenceOutput struct {
	Found     bool                `json:"found"`
	Inference inference.Inference `json:"inference"`
}

// handleGetInference returns a tool handler bound to reader. Unknown IDs are
// reported via Found=false rather than an error, matching
// store.InferenceReader.Get's not-found contract (ok=false, err=nil).
func handleGetInference(reader store.InferenceReader) mcp.ToolHandlerFor[getInferenceInput, getInferenceOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in getInferenceInput) (*mcp.CallToolResult, getInferenceOutput, error) {
		inf, ok, err := reader.Get(ctx, in.ID)
		if err != nil {
			return nil, getInferenceOutput{}, err
		}
		if !ok {
			return nil, getInferenceOutput{Found: false}, nil
		}
		return nil, getInferenceOutput{Found: true, Inference: inf}, nil
	}
}
