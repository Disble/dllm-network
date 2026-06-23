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
	assertContextFound(t, got, ok, err)
	assertMetadataMatches(t, got, "inf-ctx", "llama3")
	assertTokensNil(t, got)
	assertRequestHeadersMatch(t, got)
	assertResponseHeadersNil(t, got)
	assertRequestBodyChunkMatches(t, got)
	assertAllSectionsAvailable(t, got)
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
	assertContextFound(t, got, ok, err)
	assertTokensMatch(t, got, 90)
	assertResponseHeadersNil(t, got)
	assertExhaustedResponseBodyChunk(t, got)
	assertResponseHeadersUnavailable(t, got)
	assertResponseBodyAndTokensAvailable(t, got)
}

// assertContextFound verifies the context lookup succeeded and returned ok=true.
func assertContextFound(t *testing.T, got store.GetInferenceContextResult, ok bool, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("GetInferenceContext: %v", err)
	}
	if !ok {
		t.Fatal("expected inference context to be found")
	}
}

// assertMetadataMatches verifies the result metadata id and model match.
func assertMetadataMatches(t *testing.T, got store.GetInferenceContextResult, wantID, wantModel string) {
	t.Helper()
	if got.Metadata == nil || got.Metadata.ID != wantID || got.Metadata.Model != wantModel {
		t.Fatalf("Metadata: got %+v", got.Metadata)
	}
}

// assertTokensNil verifies Tokens is nil when not requested.
func assertTokensNil(t *testing.T, got store.GetInferenceContextResult) {
	t.Helper()
	if got.Tokens != nil {
		t.Fatalf("Tokens: expected nil when not requested, got %+v", got.Tokens)
	}
}

// assertTokensMatch verifies Tokens is populated with the expected per-second rate.
func assertTokensMatch(t *testing.T, got store.GetInferenceContextResult, wantPerSec float64) {
	t.Helper()
	if got.Tokens == nil || got.Tokens.PerSec != wantPerSec {
		t.Fatalf("Tokens: got %+v", got.Tokens)
	}
}

// assertRequestHeadersMatch verifies the result contains exactly the expected
// request header.
func assertRequestHeadersMatch(t *testing.T, got store.GetInferenceContextResult) {
	t.Helper()
	if len(got.RequestHeaders) != 1 || got.RequestHeaders[0].Name != "Content-Type" {
		t.Fatalf("RequestHeaders: got %+v", got.RequestHeaders)
	}
}

// assertResponseHeadersNil verifies ResponseHeaders is nil when not requested or
// unavailable.
func assertResponseHeadersNil(t *testing.T, got store.GetInferenceContextResult) {
	t.Helper()
	if got.ResponseHeaders != nil {
		t.Fatalf("ResponseHeaders: expected nil, got %+v", got.ResponseHeaders)
	}
}

// assertRequestBodyChunkMatches verifies the request body chunk pagination and
// truncation flags.
func assertRequestBodyChunkMatches(t *testing.T, got store.GetInferenceContextResult) {
	t.Helper()
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
}

// assertAllSectionsAvailable verifies every section is reported as available.
func assertAllSectionsAvailable(t *testing.T, got store.GetInferenceContextResult) {
	t.Helper()
	if !got.AvailableSections.Metadata || !got.AvailableSections.Tokens || !got.AvailableSections.RequestHeaders {
		t.Fatalf("AvailableSections: got %+v", got.AvailableSections)
	}
	if !got.AvailableSections.ResponseHeaders || !got.AvailableSections.RequestBody || !got.AvailableSections.ResponseBody {
		t.Fatalf("AvailableSections body/header flags: got %+v", got.AvailableSections)
	}
}

// assertExhaustedResponseBodyChunk verifies an out-of-range body chunk is empty
// and reports no remaining bytes.
func assertExhaustedResponseBodyChunk(t *testing.T, got store.GetInferenceContextResult) {
	t.Helper()
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
}

// assertResponseHeadersUnavailable verifies ResponseHeaders is flagged as unavailable.
func assertResponseHeadersUnavailable(t *testing.T, got store.GetInferenceContextResult) {
	t.Helper()
	if got.AvailableSections.ResponseHeaders {
		t.Fatalf("AvailableSections.ResponseHeaders: got true, want false")
	}
}

// assertResponseBodyAndTokensAvailable verifies ResponseBody and Tokens sections
// are flagged as available.
func assertResponseBodyAndTokensAvailable(t *testing.T, got store.GetInferenceContextResult) {
	t.Helper()
	if !got.AvailableSections.ResponseBody || !got.AvailableSections.Tokens {
		t.Fatalf("AvailableSections: got %+v", got.AvailableSections)
	}
}
