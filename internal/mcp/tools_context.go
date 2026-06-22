package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"dllm-network/internal/store"
)

type resolveInferenceContextInput struct{}

type resolveInferenceContextOutput struct {
	Context store.ResolveInferenceContextResult `json:"context"`
}

func handleResolveInferenceContext(reader store.InferenceReader) mcp.ToolHandlerFor[resolveInferenceContextInput, resolveInferenceContextOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ resolveInferenceContextInput) (*mcp.CallToolResult, resolveInferenceContextOutput, error) {
		result, err := reader.ResolveInferenceContext(ctx)
		if err != nil {
			return nil, resolveInferenceContextOutput{}, err
		}
		return nil, resolveInferenceContextOutput{Context: result}, nil
	}
}
