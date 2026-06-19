package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ollama-telemetry/internal/store"
	"ollama-telemetry/internal/telemetry/inference"
)

// queryInferencesInput is the typed input schema for the query_inferences
// tool. All fields are optional; the zero value of each disables that
// filter constraint, mirroring store.Filter's own zero-value semantics.
// Status/Since/Until are strings (not store.Filter's native types) because
// MCP tool inputs are JSON — Status is parsed via parsePhase, Since/Until via
// RFC 3339.
type queryInferencesInput struct {
	Model    string `json:"model,omitempty" jsonschema:"filter by exact model name"`
	Endpoint string `json:"endpoint,omitempty" jsonschema:"filter by HTTP endpoint path, e.g. /api/generate"`
	Status   string `json:"status,omitempty" jsonschema:"filter by lifecycle status: in_progress, completed, metadata_only, or cancelled"`
	Since    string `json:"since,omitempty" jsonschema:"RFC3339 timestamp; only inferences at or after this time"`
	Until    string `json:"until,omitempty" jsonschema:"RFC3339 timestamp; only inferences before this time"`
	Limit    int    `json:"limit,omitempty" jsonschema:"maximum number of results; 0 means no cap"`
}

// queryInferencesOutput is the typed output schema for the query_inferences
// tool: the matching inferences, most-recent-first (per store.InferenceReader's
// Query contract).
type queryInferencesOutput struct {
	Inferences []inference.Inference `json:"inferences"`
}

// phaseNames maps the wire-friendly status strings accepted by tool inputs to
// the inference.Phase domain enum.
var phaseNames = map[string]inference.Phase{
	"in_progress":   inference.PhaseInProgress,
	"completed":     inference.PhaseCompleted,
	"metadata_only": inference.PhaseMetadataOnly,
	"cancelled":     inference.PhaseCancelled,
}

// parsePhase resolves a status string to *inference.Phase. An empty string
// means "no filter" (nil); any other unrecognized value is an error so
// callers get a clear failure instead of a silently-ignored typo.
func parsePhase(s string) (*inference.Phase, error) {
	if s == "" {
		return nil, nil
	}
	p, ok := phaseNames[s]
	if !ok {
		return nil, fmt.Errorf("unknown status %q: must be one of in_progress, completed, metadata_only, cancelled", s)
	}
	return &p, nil
}

// parseTimeFilter parses an optional RFC3339 timestamp; an empty string
// yields the zero time.Time, which disables that store.Filter bound.
func parseTimeFilter(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid RFC3339 timestamp %q: %w", s, err)
	}
	return t, nil
}

// handleQueryInferences returns a tool handler bound to reader, translating
// queryInferencesInput into a store.Filter and reader.Query call. It is a
// function over the store.InferenceReader port (not a method on *Server) so
// it is independently unit-testable with a fake reader.
func handleQueryInferences(reader store.InferenceReader) mcp.ToolHandlerFor[queryInferencesInput, queryInferencesOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in queryInferencesInput) (*mcp.CallToolResult, queryInferencesOutput, error) {
		status, err := parsePhase(in.Status)
		if err != nil {
			return nil, queryInferencesOutput{}, err
		}
		since, err := parseTimeFilter(in.Since)
		if err != nil {
			return nil, queryInferencesOutput{}, err
		}
		until, err := parseTimeFilter(in.Until)
		if err != nil {
			return nil, queryInferencesOutput{}, err
		}

		filter := store.Filter{
			Model:    in.Model,
			Endpoint: in.Endpoint,
			Status:   status,
			Since:    since,
			Until:    until,
			Limit:    in.Limit,
		}

		results, err := reader.Query(ctx, filter)
		if err != nil {
			return nil, queryInferencesOutput{}, err
		}

		return nil, queryInferencesOutput{Inferences: results}, nil
	}
}
