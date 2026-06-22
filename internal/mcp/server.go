// Package mcp builds a transport-decoupled MCP server core exposing the
// captured inference data over the Model Context Protocol. It depends only
// on the store.InferenceReader port (never on a concrete store
// implementation), and quarantines its dependency on
// github.com/modelcontextprotocol/go-sdk entirely within this package — no
// other package in this project imports the SDK.
package mcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"dllm-network/internal/store"
)

// serverName/serverVersion identify this MCP server to connecting clients.
const (
	serverName    = "dllm-network"
	serverVersion = "v1.0.0"
)

// NewServer builds an *mcp.Server with all tools and resources registered
// against reader. The returned server is transport-agnostic: callers decide
// how to run it (see transport.go's RunStdio for the stdio adapter). This
// is the seam that lets an HTTP transport be added later without touching
// any registration logic here.
func NewServer(reader store.InferenceReader) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: serverName, Version: serverVersion}, nil)

	registerTools(srv, reader)

	return srv
}

// registerTools adds the staged Context7-style tools to srv, each bound to
// reader.
func registerTools(srv *mcp.Server, reader store.InferenceReader) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "resolve_inference_context",
		Description: "Return lightweight discovery data for the searchable inference universe, including dimensions, time bounds, counts, and supported filters.",
	}, handleResolveInferenceContext(reader))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "search_inferences",
		Description: "Return paginated lightweight inference summaries ordered newest-first, with a stable opaque cursor for follow-up pages.",
	}, handleSearchInferences(reader))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_inference_context",
		Description: "Return bounded context for one selected inference using explicit sections and body slices on demand.",
	}, handleGetInferenceContext(reader))
}
