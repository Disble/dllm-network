package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"dllm-network/internal/telemetry/inference"
)

// openSeededStore opens a fresh temp-dir Store and saves seed, returning the
// store for read-side tests. Shared across sqlite test files to keep seeding
// boilerplate in one place.
func openSeededStore(t *testing.T, seed []inference.Inference) *Store {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "telemetry.db")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	if len(seed) > 0 {
		if err := st.Save(context.Background(), seed); err != nil {
			t.Fatalf("Save seed: %v", err)
		}
	}
	return st
}

// fixtureAt builds a minimal completed inference fixture at a requested time.
func fixtureAt(id string, at time.Time, model, endpoint string, perSec, latencyMS float64) inference.Inference {
	return inference.Inference{
		ID:         id,
		At:         at,
		Endpoint:   endpoint,
		Method:     "POST",
		Model:      model,
		PromptSize: 10,
		Status:     inference.PhaseCompleted,
		StatusCode: 200,
		Tokens: &inference.TokenStats{
			PromptEvalCount: 5,
			EvalCount:       50,
			EvalDuration:    time.Duration(latencyMS) * time.Millisecond,
			TotalDuration:   time.Duration(latencyMS) * time.Millisecond,
			PerSec:          perSec,
			LatencyMS:       latencyMS,
		},
	}
}

// assertInferenceEqual compares the fields of two inference.Inference values
// that matter for the persistence round-trip contract, reporting every
// mismatch (not just the first) so a failing test pinpoints the broken field
// without requiring a second run.
func assertInferenceEqual(t *testing.T, got, want inference.Inference) {
	t.Helper()

	if got.ID != want.ID {
		t.Errorf("ID: got %q, want %q", got.ID, want.ID)
	}
	if !got.At.Equal(want.At) {
		t.Errorf("At: got %v, want %v", got.At, want.At)
	}
	if got.Endpoint != want.Endpoint {
		t.Errorf("Endpoint: got %q, want %q", got.Endpoint, want.Endpoint)
	}
	if got.Method != want.Method {
		t.Errorf("Method: got %q, want %q", got.Method, want.Method)
	}
	if got.Model != want.Model {
		t.Errorf("Model: got %q, want %q", got.Model, want.Model)
	}
	if got.PromptSize != want.PromptSize {
		t.Errorf("PromptSize: got %d, want %d", got.PromptSize, want.PromptSize)
	}
	if got.Streaming != want.Streaming {
		t.Errorf("Streaming: got %v, want %v", got.Streaming, want.Streaming)
	}
	if got.Status != want.Status {
		t.Errorf("Status: got %v, want %v", got.Status, want.Status)
	}
	if got.StatusCode != want.StatusCode {
		t.Errorf("StatusCode: got %d, want %d", got.StatusCode, want.StatusCode)
	}
	if got.RequestBody != want.RequestBody {
		t.Errorf("RequestBody: got %q, want %q", got.RequestBody, want.RequestBody)
	}
	if got.RequestBodyTruncated != want.RequestBodyTruncated {
		t.Errorf("RequestBodyTruncated: got %v, want %v", got.RequestBodyTruncated, want.RequestBodyTruncated)
	}
	if got.ResponseBody != want.ResponseBody {
		t.Errorf("ResponseBody: got %q, want %q", got.ResponseBody, want.ResponseBody)
	}
	if got.ResponseBodyTruncated != want.ResponseBodyTruncated {
		t.Errorf("ResponseBodyTruncated: got %v, want %v", got.ResponseBodyTruncated, want.ResponseBodyTruncated)
	}

	assertTokensEqual(t, got.Tokens, want.Tokens)
	assertGenerationEqual(t, got.Generation, want.Generation)
	assertHeadersEqual(t, "RequestHeaders", got.RequestHeaders, want.RequestHeaders)
	assertHeadersEqual(t, "ResponseHeaders", got.ResponseHeaders, want.ResponseHeaders)
}

func assertGenerationEqual(t *testing.T, got, want *inference.Generation) {
	t.Helper()

	if (got == nil) != (want == nil) {
		t.Errorf("Generation nilness: got %v, want %v", got == nil, want == nil)
		return
	}
	if got == nil {
		return
	}
	if got.Output != want.Output || got.Reasoning != want.Reasoning || got.FinishReason != want.FinishReason {
		t.Errorf("Generation text: got %+v, want %+v", *got, *want)
	}
	if got.ContextSize != want.ContextSize {
		t.Errorf("Generation.ContextSize: got %d, want %d", got.ContextSize, want.ContextSize)
	}
	if len(got.ContextPreview) != len(want.ContextPreview) {
		t.Errorf("Generation.ContextPreview length: got %d, want %d", len(got.ContextPreview), len(want.ContextPreview))
		return
	}
	for i := range got.ContextPreview {
		if got.ContextPreview[i] != want.ContextPreview[i] {
			t.Errorf("Generation.ContextPreview[%d]: got %d, want %d", i, got.ContextPreview[i], want.ContextPreview[i])
		}
	}
}

func assertTokensEqual(t *testing.T, got, want *inference.TokenStats) {
	t.Helper()

	if (got == nil) != (want == nil) {
		t.Errorf("Tokens nilness: got %v, want %v", got == nil, want == nil)
		return
	}
	if got == nil {
		return
	}
	if *got != *want {
		t.Errorf("Tokens: got %+v, want %+v", *got, *want)
	}
}

func assertHeadersEqual(t *testing.T, field string, got, want []inference.Header) {
	t.Helper()

	if (got == nil) != (want == nil) {
		t.Errorf("%s nilness: got %v, want %v", field, got == nil, want == nil)
		return
	}
	if len(got) != len(want) {
		t.Errorf("%s length: got %d, want %d", field, len(got), len(want))
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("%s[%d]: got %+v, want %+v", field, i, got[i], want[i])
		}
	}
}
