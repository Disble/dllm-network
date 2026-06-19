package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ollama-telemetry/internal/store"
)

type getInferenceContextInput struct {
	ID string `json:"id" jsonschema:"the inference id to inspect"`
}

type getInferenceContextOutput struct {
	Found   bool                            `json:"found"`
	Context store.GetInferenceContextResult `json:"context"`
}

func handleGetInferenceContext(reader store.InferenceReader) mcp.ToolHandlerFor[getInferenceContextInput, getInferenceContextOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in getInferenceContextInput) (*mcp.CallToolResult, getInferenceContextOutput, error) {
		result, ok, err := reader.GetInferenceContext(ctx, store.GetInferenceContextQuery{ID: in.ID})
		if err != nil {
			return nil, getInferenceContextOutput{}, err
		}
		if !ok {
			return nil, getInferenceContextOutput{Found: false}, nil
		}
		return nil, getInferenceContextOutput{Found: true, Context: result}, nil
	}
}
