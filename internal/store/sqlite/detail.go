package sqlite

import (
	"context"
	"encoding/json"
	"fmt"

	"ollama-telemetry/internal/store"
	"ollama-telemetry/internal/telemetry/inference"
)

// detail is the JSON-encoded payload stored in the inferences.detail column.
// It carries everything that is heavy or never filtered/sorted on: request
// and response bodies (with their truncation flags) and the captured
// headers. The full TokenStats is intentionally duplicated here (alongside
// the flattened scalar columns in schema.go) so unmarshalDetail can rebuild
// *inference.TokenStats with a single nil check, without re-deriving it from
// individually-NULLable scalar columns.
type detail struct {
	Tokens                *inference.TokenStats `json:"tokens"`
	RequestBody           string                `json:"requestBody"`
	RequestBodyTruncated  bool                  `json:"requestBodyTruncated"`
	ResponseBody          string                `json:"responseBody"`
	ResponseBodyTruncated bool                  `json:"responseBodyTruncated"`
	RequestHeaders        []inference.Header    `json:"requestHeaders"`
	ResponseHeaders       []inference.Header    `json:"responseHeaders"`
}

// marshalDetail isolates the inference.Inference <-> JSON mapping in one
// place so the domain type (internal/telemetry/inference) never needs to
// import database/sql or know about this storage layout.
func marshalDetail(inf inference.Inference) (string, error) {
	d := detail{
		Tokens:                inf.Tokens,
		RequestBody:           inf.RequestBody,
		RequestBodyTruncated:  inf.RequestBodyTruncated,
		ResponseBody:          inf.ResponseBody,
		ResponseBodyTruncated: inf.ResponseBodyTruncated,
		RequestHeaders:        inf.RequestHeaders,
		ResponseHeaders:       inf.ResponseHeaders,
	}

	raw, err := json.Marshal(d)
	if err != nil {
		return "", fmt.Errorf("sqlite: marshal detail: %w", err)
	}
	return string(raw), nil
}

// unmarshalDetail decodes the detail JSON blob and applies it onto inf,
// preserving the nil-vs-zero contracts (Tokens, RequestHeaders,
// ResponseHeaders) that the domain type requires.
func unmarshalDetail(raw string, inf *inference.Inference) error {
	var d detail
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return fmt.Errorf("sqlite: unmarshal detail: %w", err)
	}

	inf.Tokens = d.Tokens
	inf.RequestBody = d.RequestBody
	inf.RequestBodyTruncated = d.RequestBodyTruncated
	inf.ResponseBody = d.ResponseBody
	inf.ResponseBodyTruncated = d.ResponseBodyTruncated
	inf.RequestHeaders = d.RequestHeaders
	inf.ResponseHeaders = d.ResponseHeaders

	return nil
}

func (s *Store) GetInferenceContext(ctx context.Context, query store.GetInferenceContextQuery) (store.GetInferenceContextResult, bool, error) {
	inf, ok, err := s.Get(ctx, query.ID)
	if err != nil || !ok {
		return store.GetInferenceContextResult{}, ok, err
	}

	availability := store.InferenceContextAvailability{
		Metadata:        true,
		Tokens:          inf.Tokens != nil,
		RequestHeaders:  inf.RequestHeaders != nil,
		ResponseHeaders: inf.ResponseHeaders != nil,
		RequestBody:     inf.RequestBody != "" || inf.RequestBodyTruncated,
		ResponseBody:    inf.ResponseBody != "" || inf.ResponseBodyTruncated,
	}

	result := store.GetInferenceContextResult{AvailableSections: availability}
	sections := normalizeRequestedSections(query.Sections)
	for _, section := range sections {
		switch section {
		case store.InferenceContextSectionMetadata:
			result.Metadata = &store.InferenceContextMetadata{
				ID:         inf.ID,
				At:         inf.At,
				Model:      inf.Model,
				Endpoint:   inf.Endpoint,
				Method:     inf.Method,
				Status:     store.InferenceStatusLabel(inf.Status),
				StatusCode: inf.StatusCode,
				Streaming:  inf.Streaming,
				PromptSize: inf.PromptSize,
			}
		case store.InferenceContextSectionTokens:
			if inf.Tokens != nil {
				tokens := *inf.Tokens
				result.Tokens = &tokens
			}
		case store.InferenceContextSectionRequestHeaders:
			if inf.RequestHeaders != nil {
				result.RequestHeaders = append([]inference.Header(nil), inf.RequestHeaders...)
			}
		case store.InferenceContextSectionResponseHeaders:
			if inf.ResponseHeaders != nil {
				result.ResponseHeaders = append([]inference.Header(nil), inf.ResponseHeaders...)
			}
		}
	}

	if query.Body != nil {
		body := query.Body.Normalized()
		result.BodyChunk = buildBodyChunk(inf, body)
	}

	return result, true, nil
}

func normalizeRequestedSections(sections []store.InferenceContextSection) []store.InferenceContextSection {
	if len(sections) == 0 {
		return []store.InferenceContextSection{store.InferenceContextSectionMetadata}
	}

	result := make([]store.InferenceContextSection, 0, len(sections))
	seen := make(map[store.InferenceContextSection]struct{}, len(sections))
	for _, section := range sections {
		if _, ok := seen[section]; ok {
			continue
		}
		seen[section] = struct{}{}
		result = append(result, section)
	}
	return result
}

func buildBodyChunk(inf inference.Inference, req store.InferenceContextBodyRequest) *store.InferenceContextBodyChunk {
	body, truncated := resolveBodySource(inf, req.Name)
	total := len(body)
	if req.Offset > total {
		return &store.InferenceContextBodyChunk{
			Name:       req.Name,
			Offset:     req.Offset,
			Limit:      req.Limit,
			NextOffset: req.Offset,
			HasMore:    false,
			TotalBytes: total,
			Content:    "",
			Truncated:  truncated,
		}
	}

	end := req.Offset + req.Limit
	if end > total {
		end = total
	}
	return &store.InferenceContextBodyChunk{
		Name:       req.Name,
		Offset:     req.Offset,
		Limit:      req.Limit,
		NextOffset: end,
		HasMore:    end < total,
		TotalBytes: total,
		Content:    body[req.Offset:end],
		Truncated:  truncated,
	}
}

func resolveBodySource(inf inference.Inference, name store.InferenceContextBodyName) (string, bool) {
	if name == store.InferenceContextBodyResponseBody {
		return inf.ResponseBody, inf.ResponseBodyTruncated
	}
	return inf.RequestBody, inf.RequestBodyTruncated
}
