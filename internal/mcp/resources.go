package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ollama-telemetry/internal/store"
)

// inferenceURIPrefix is the scheme+authority shared by both inference
// resources: inference://{id} and inference://recent.
const inferenceURIPrefix = "inference://"

// recentResourceLimit bounds the inference://recent resource's result size,
// per the spec's "bounded recent list" requirement.
const recentResourceLimit = 20

// handleInferenceByID returns a ResourceHandler for the inference://{id}
// resource template, bound to reader. It extracts the id from the request
// URI itself (the SDK does not pre-parse URI templates into params for
// ResourceHandler) and delegates to reader.Get, mirroring the get_inference
// tool's not-found contract but surfaced as ResourceNotFoundError per
// ResourceHandler's documented contract.
func handleInferenceByID(reader store.InferenceReader) mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		id := strings.TrimPrefix(req.Params.URI, inferenceURIPrefix)

		inf, ok, err := reader.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, mcp.ResourceNotFoundError(req.Params.URI)
		}

		body, err := json.Marshal(inf)
		if err != nil {
			return nil, fmt.Errorf("marshal inference %q: %w", id, err)
		}

		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{URI: req.Params.URI, MIMEType: "application/json", Text: string(body)},
			},
		}, nil
	}
}

// handleInferenceRecent returns a ResourceHandler for the fixed
// inference://recent resource, bound to reader. It queries the most recent
// inferences (no model/endpoint/status/time filter) capped at
// recentResourceLimit.
func handleInferenceRecent(reader store.InferenceReader) mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		results, err := reader.Query(ctx, store.Filter{Limit: recentResourceLimit})
		if err != nil {
			return nil, err
		}

		body, err := json.Marshal(results)
		if err != nil {
			return nil, fmt.Errorf("marshal recent inferences: %w", err)
		}

		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{URI: req.Params.URI, MIMEType: "application/json", Text: string(body)},
			},
		}, nil
	}
}
