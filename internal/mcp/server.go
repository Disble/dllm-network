// Package mcp builds a transport-decoupled MCP server core exposing the
// captured inference data over the Model Context Protocol. It depends only
// on the store.InferenceReader port (never on a concrete store
// implementation), and quarantines its dependency on
// github.com/modelcontextprotocol/go-sdk entirely within this package — no
// other package in this project imports the SDK.
package mcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ollama-telemetry/internal/store"
)

// serverName/serverVersion identify this MCP server to connecting clients.
const (
	serverName    = "ollama-telemetry"
	serverVersion = "v1.0.0"
)

// inferenceRecentURI is the fixed (non-templated) resource URI for the
// bounded recent-inferences list.
const inferenceRecentURI = "inference://recent"

// inferenceByIDURITemplate is the RFC 6570 URI template for fetching one
// inference by id.
const inferenceByIDURITemplate = "inference://{id}"

// NewServer builds an *mcp.Server with all tools and resources registered
// against reader. The returned server is transport-agnostic: callers decide
// how to run it (see transport.go's RunStdio for the stdio adapter). This
// is the seam that lets an HTTP transport be added later without touching
// any registration logic here.
func NewServer(reader store.InferenceReader) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: serverName, Version: serverVersion}, nil)

	registerTools(srv, reader)
	registerResources(srv, reader)

	return srv
}

// registerTools adds the four query/read tools (query_inferences,
// get_inference, get_stats, list_models) to srv, each bound to reader.
func registerTools(srv *mcp.Server, reader store.InferenceReader) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "query_inferences",
		Description: "List captured inferences filtered by model, endpoint, status, and/or a time window, most-recent-first.",
	}, handleQueryInferences(reader))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_inference",
		Description: "Fetch one captured inference by id, including request/response bodies and headers.",
	}, handleGetInference(reader))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_stats",
		Description: "Compute aggregate tokens/sec and latency percentiles plus per-model counts, optionally scoped by model and/or a time window.",
	}, handleGetStats(reader))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_models",
		Description: "List the distinct model names observed in captured inferences.",
	}, handleListModels(reader))
}

// registerResources adds the inference://{id} template and the fixed
// inference://recent resource to srv, each bound to reader.
func registerResources(srv *mcp.Server, reader store.InferenceReader) {
	srv.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "inference-by-id",
		Description: "Fetch one captured inference by id, including request/response bodies and headers.",
		MIMEType:    "application/json",
		URITemplate: inferenceByIDURITemplate,
	}, handleInferenceByID(reader))

	srv.AddResource(&mcp.Resource{
		Name:        "inference-recent",
		Description: "The most recent captured inferences, bounded to a fixed limit.",
		MIMEType:    "application/json",
		URI:         inferenceRecentURI,
	}, handleInferenceRecent(reader))
}
