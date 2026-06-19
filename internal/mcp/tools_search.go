package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ollama-telemetry/internal/store"
)

type searchInferencesInput struct {
	Model    string `json:"model,omitempty" jsonschema:"filter by exact model name"`
	Endpoint string `json:"endpoint,omitempty" jsonschema:"filter by HTTP endpoint path, e.g. /api/generate"`
	Status   string `json:"status,omitempty" jsonschema:"filter by lifecycle status: in_progress, completed, metadata_only, or cancelled"`
	Since    string `json:"since,omitempty" jsonschema:"RFC3339 timestamp; only inferences at or after this time"`
	Until    string `json:"until,omitempty" jsonschema:"RFC3339 timestamp; only inferences before this time"`
	Limit    int    `json:"limit,omitempty" jsonschema:"maximum number of results per page; defaults to 20, max 100"`
	Cursor   string `json:"cursor,omitempty" jsonschema:"opaque cursor returned by a previous search_inferences call"`
}

type searchInferencesOutput struct {
	Items      []store.InferenceSummary `json:"items"`
	NextCursor string                   `json:"nextCursor,omitempty"`
}

func handleSearchInferences(reader store.InferenceReader) mcp.ToolHandlerFor[searchInferencesInput, searchInferencesOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in searchInferencesInput) (*mcp.CallToolResult, searchInferencesOutput, error) {
		query, err := parseSearchFilters(in.Model, in.Endpoint, in.Status, in.Since, in.Until, in.Limit, in.Cursor)
		if err != nil {
			return nil, searchInferencesOutput{}, err
		}

		result, err := reader.SearchInferences(ctx, query)
		if err != nil {
			return nil, searchInferencesOutput{}, err
		}

		return nil, searchInferencesOutput{Items: result.Items, NextCursor: result.NextCursor}, nil
	}
}
