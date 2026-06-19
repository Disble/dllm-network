package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ollama-telemetry/internal/store"
)

// getStatsInput is the typed input schema for the get_stats tool. All
// fields are optional, mirroring queryInferencesInput's filter semantics
// (model scopes by exact name; since/until form an RFC3339 time-window).
type getStatsInput struct {
	Model string `json:"model,omitempty" jsonschema:"limit stats to this exact model name"`
	Since string `json:"since,omitempty" jsonschema:"RFC3339 timestamp; only inferences at or after this time"`
	Until string `json:"until,omitempty" jsonschema:"RFC3339 timestamp; only inferences before this time"`
}

// getStatsOutput is the typed output schema for the get_stats tool.
type getStatsOutput struct {
	Stats store.Stats `json:"stats"`
}

// handleGetStats returns a tool handler bound to reader, translating
// getStatsInput into a store.Filter (only Model/Since/Until are meaningful
// for Stats; Endpoint/Status/Limit are left zero) and delegating to
// reader.Stats.
func handleGetStats(reader store.InferenceReader) mcp.ToolHandlerFor[getStatsInput, getStatsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in getStatsInput) (*mcp.CallToolResult, getStatsOutput, error) {
		since, err := parseTimeFilter(in.Since)
		if err != nil {
			return nil, getStatsOutput{}, err
		}
		until, err := parseTimeFilter(in.Until)
		if err != nil {
			return nil, getStatsOutput{}, err
		}

		filter := store.Filter{Model: in.Model, Since: since, Until: until}

		stats, err := reader.Stats(ctx, filter)
		if err != nil {
			return nil, getStatsOutput{}, err
		}

		return nil, getStatsOutput{Stats: stats}, nil
	}
}
