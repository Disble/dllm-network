package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"dllm-network/internal/store"
)

type getInferenceContextInput struct {
	ID       string                        `json:"id" jsonschema:"the inference id to inspect"`
	Sections []string                      `json:"sections,omitempty" jsonschema:"optional sections: metadata, tokens, request_headers, response_headers"`
	Body     *getInferenceContextBodyInput `json:"body,omitempty" jsonschema:"optional body slice request for request_body or response_body"`
}

type getInferenceContextBodyInput struct {
	Name   string `json:"name" jsonschema:"body source: request_body or response_body"`
	Offset int    `json:"offset,omitempty" jsonschema:"byte offset into the selected body"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum bytes to return from the selected body"`
}

type getInferenceContextOutput struct {
	Found   bool                            `json:"found"`
	Context store.GetInferenceContextResult `json:"context"`
}

func handleGetInferenceContext(reader store.InferenceReader) mcp.ToolHandlerFor[getInferenceContextInput, getInferenceContextOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in getInferenceContextInput) (*mcp.CallToolResult, getInferenceContextOutput, error) {
		query, err := parseGetInferenceContextQuery(in)
		if err != nil {
			return nil, getInferenceContextOutput{}, err
		}

		result, ok, err := reader.GetInferenceContext(ctx, query)
		if err != nil {
			return nil, getInferenceContextOutput{}, err
		}
		if !ok {
			return nil, getInferenceContextOutput{Found: false}, nil
		}
		return nil, getInferenceContextOutput{Found: true, Context: result}, nil
	}
}

func parseGetInferenceContextQuery(in getInferenceContextInput) (store.GetInferenceContextQuery, error) {
	sections, err := parseInferenceContextSections(in.Sections)
	if err != nil {
		return store.GetInferenceContextQuery{}, err
	}
	body, err := parseInferenceContextBody(in.Body)
	if err != nil {
		return store.GetInferenceContextQuery{}, err
	}
	return store.GetInferenceContextQuery{ID: in.ID, Sections: sections, Body: body}, nil
}

func parseInferenceContextSections(raw []string) ([]store.InferenceContextSection, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	allowed := make(map[string]store.InferenceContextSection, len(store.SupportedInferenceContextSections()))
	for _, section := range store.SupportedInferenceContextSections() {
		allowed[string(section)] = section
	}

	sections := make([]store.InferenceContextSection, 0, len(raw))
	seen := make(map[store.InferenceContextSection]struct{}, len(raw))
	for _, item := range raw {
		section, ok := allowed[item]
		if !ok {
			return nil, fmt.Errorf("unsupported section %q", item)
		}
		if _, ok := seen[section]; ok {
			continue
		}
		seen[section] = struct{}{}
		sections = append(sections, section)
	}
	return sections, nil
}

func parseInferenceContextBody(raw *getInferenceContextBodyInput) (*store.InferenceContextBodyRequest, error) {
	if raw == nil {
		return nil, nil
	}
	allowed := make(map[string]store.InferenceContextBodyName, len(store.SupportedInferenceContextBodies()))
	for _, body := range store.SupportedInferenceContextBodies() {
		allowed[string(body)] = body
	}
	name, ok := allowed[raw.Name]
	if !ok {
		return nil, fmt.Errorf("unsupported body %q", raw.Name)
	}
	request := store.InferenceContextBodyRequest{Name: name, Offset: raw.Offset, Limit: raw.Limit}.Normalized()
	return &request, nil
}
