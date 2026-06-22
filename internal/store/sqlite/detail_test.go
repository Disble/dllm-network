package sqlite

import (
	"context"
	"testing"
	"time"

	"dllm-network/internal/store"
	"dllm-network/internal/telemetry/inference"
)

func TestStore_GetInferenceContext_ReturnsSelectedSectionsAndInitialRequestBodyChunk(t *testing.T) {
	t.Parallel()

	inf := fixtureAt("inf-ctx", time.Date(2026, time.June, 19, 11, 0, 0, 0, time.UTC), "llama3", "/api/generate", 125, 75)
	inf.RequestHeaders = []inference.Header{{Name: "Content-Type", Value: "application/json"}}
	inf.ResponseHeaders = []inference.Header{{Name: "X-Trace", Value: "abc"}}
	inf.RequestBody = "hello world"
	inf.RequestBodyTruncated = true
	inf.ResponseBody = "response body"

	st := openSeededStore(t, []inference.Inference{inf})

	got, ok, err := st.GetInferenceContext(context.Background(), store.GetInferenceContextQuery{
		ID: "inf-ctx",
		Sections: []store.InferenceContextSection{
			store.InferenceContextSectionMetadata,
			store.InferenceContextSectionRequestHeaders,
		},
		Body: &store.InferenceContextBodyRequest{
			Name:   store.InferenceContextBodyRequestBody,
			Offset: 0,
			Limit:  5,
		},
	})
	if err != nil {
		t.Fatalf("GetInferenceContext: %v", err)
	}
	if !ok {
		t.Fatal("expected inference context to be found")
	}
	if got.Metadata == nil || got.Metadata.ID != "inf-ctx" || got.Metadata.Model != "llama3" {
		t.Fatalf("Metadata: got %+v", got.Metadata)
	}
	if got.Tokens != nil {
		t.Fatalf("Tokens: expected nil when not requested, got %+v", got.Tokens)
	}
	if len(got.RequestHeaders) != 1 || got.RequestHeaders[0].Name != "Content-Type" {
		t.Fatalf("RequestHeaders: got %+v", got.RequestHeaders)
	}
	if got.ResponseHeaders != nil {
		t.Fatalf("ResponseHeaders: expected nil when not requested, got %+v", got.ResponseHeaders)
	}
	if got.BodyChunk == nil {
		t.Fatal("BodyChunk: expected non-nil chunk")
	}
	if got.BodyChunk.Name != store.InferenceContextBodyRequestBody {
		t.Fatalf("BodyChunk.Name: got %q", got.BodyChunk.Name)
	}
	if got.BodyChunk.Content != "hello" || got.BodyChunk.NextOffset != 5 || !got.BodyChunk.HasMore {
		t.Fatalf("BodyChunk: got %+v", got.BodyChunk)
	}
	if got.BodyChunk.TotalBytes != len("hello world") || !got.BodyChunk.Truncated {
		t.Fatalf("BodyChunk metadata: got %+v", got.BodyChunk)
	}
	if !got.AvailableSections.Metadata || !got.AvailableSections.Tokens || !got.AvailableSections.RequestHeaders {
		t.Fatalf("AvailableSections: got %+v", got.AvailableSections)
	}
	if !got.AvailableSections.ResponseHeaders || !got.AvailableSections.RequestBody || !got.AvailableSections.ResponseBody {
		t.Fatalf("AvailableSections body/header flags: got %+v", got.AvailableSections)
	}
}

func TestStore_GetInferenceContext_MarksUnavailableSectionAndExhaustedResponseBodyChunk(t *testing.T) {
	t.Parallel()

	inf := fixtureAt("inf-exhausted", time.Date(2026, time.June, 19, 12, 0, 0, 0, time.UTC), "mistral", "/api/chat", 90, 40)
	inf.ResponseHeaders = nil
	inf.ResponseBody = "pong"
	inf.ResponseBodyTruncated = false

	st := openSeededStore(t, []inference.Inference{inf})

	got, ok, err := st.GetInferenceContext(context.Background(), store.GetInferenceContextQuery{
		ID: "inf-exhausted",
		Sections: []store.InferenceContextSection{
			store.InferenceContextSectionTokens,
			store.InferenceContextSectionResponseHeaders,
		},
		Body: &store.InferenceContextBodyRequest{
			Name:   store.InferenceContextBodyResponseBody,
			Offset: 10,
			Limit:  4,
		},
	})
	if err != nil {
		t.Fatalf("GetInferenceContext: %v", err)
	}
	if !ok {
		t.Fatal("expected inference context to be found")
	}
	if got.Tokens == nil || got.Tokens.PerSec != 90 {
		t.Fatalf("Tokens: got %+v", got.Tokens)
	}
	if got.ResponseHeaders != nil {
		t.Fatalf("ResponseHeaders: expected nil for unavailable section, got %+v", got.ResponseHeaders)
	}
	if got.BodyChunk == nil {
		t.Fatal("BodyChunk: expected non-nil chunk")
	}
	if got.BodyChunk.Name != store.InferenceContextBodyResponseBody {
		t.Fatalf("BodyChunk.Name: got %q", got.BodyChunk.Name)
	}
	if got.BodyChunk.Content != "" || got.BodyChunk.Offset != 10 || got.BodyChunk.NextOffset != 10 || got.BodyChunk.HasMore {
		t.Fatalf("BodyChunk exhausted window: got %+v", got.BodyChunk)
	}
	if got.BodyChunk.TotalBytes != len("pong") {
		t.Fatalf("BodyChunk total bytes: got %+v", got.BodyChunk)
	}
	if got.AvailableSections.ResponseHeaders {
		t.Fatalf("AvailableSections.ResponseHeaders: got true, want false")
	}
	if !got.AvailableSections.ResponseBody || !got.AvailableSections.Tokens {
		t.Fatalf("AvailableSections: got %+v", got.AvailableSections)
	}
}
