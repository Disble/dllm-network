package mcp

import (
	"context"
	"reflect"
	"testing"
	"time"

	"dllm-network/internal/store"
	"dllm-network/internal/telemetry/inference"
)

func TestGetInferenceContextHandler_MapsSectionsAndBodyRequest(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.June, 19, 13, 0, 0, 0, time.UTC)
	reader := &fakeReader{getContextResult: store.GetInferenceContextResult{
		AvailableSections: store.InferenceContextAvailability{
			Metadata:       true,
			RequestHeaders: true,
			RequestBody:    true,
		},
		Metadata:       &store.InferenceContextMetadata{ID: "inf-1", At: base, Model: "llama3", Endpoint: "/api/generate", Method: "POST", Status: "completed", StatusCode: 200, Streaming: true, PromptSize: 42},
		RequestHeaders: []inference.Header{{Name: "Content-Type", Value: "application/json"}},
		BodyChunk:      &store.InferenceContextBodyChunk{Name: store.InferenceContextBodyRequestBody, Offset: 0, Limit: 5, NextOffset: 5, HasMore: true, TotalBytes: 11, Content: "hello", Truncated: true},
	}, getContextOK: true}

	_, out, err := handleGetInferenceContext(reader)(context.Background(), nil, getInferenceContextInput{
		ID:       "inf-1",
		Sections: []string{"metadata", "request_headers"},
		Body:     &getInferenceContextBodyInput{Name: "request_body", Offset: 0, Limit: 5},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reader.getContextCalls != 1 {
		t.Fatalf("GetInferenceContext calls: got %d, want 1", reader.getContextCalls)
	}
	wantSections := []store.InferenceContextSection{store.InferenceContextSectionMetadata, store.InferenceContextSectionRequestHeaders}
	if !reflect.DeepEqual(reader.lastContextQuery.Sections, wantSections) {
		t.Fatalf("Sections: got %+v, want %+v", reader.lastContextQuery.Sections, wantSections)
	}
	if reader.lastContextQuery.Body == nil || reader.lastContextQuery.Body.Name != store.InferenceContextBodyRequestBody {
		t.Fatalf("Body request: got %+v", reader.lastContextQuery.Body)
	}
	if !out.Found || out.Context.Metadata == nil || out.Context.Metadata.ID != "inf-1" {
		t.Fatalf("Output: got %+v", out)
	}
	if out.Context.BodyChunk == nil || out.Context.BodyChunk.Content != "hello" || !out.Context.BodyChunk.HasMore {
		t.Fatalf("BodyChunk: got %+v", out.Context.BodyChunk)
	}
}

func TestGetInferenceContextHandler_UnknownIDReturnsNotFoundWithoutError(t *testing.T) {
	t.Parallel()

	reader := &fakeReader{getContextOK: false}

	_, out, err := handleGetInferenceContext(reader)(context.Background(), nil, getInferenceContextInput{
		ID:       "missing",
		Sections: []string{"metadata"},
	})
	if err != nil {
		t.Fatalf("expected no error for unknown id, got %v", err)
	}
	if out.Found {
		t.Fatalf("Found: got true, want false")
	}
	if reader.getContextCalls != 1 {
		t.Fatalf("GetInferenceContext calls: got %d, want 1", reader.getContextCalls)
	}
}
