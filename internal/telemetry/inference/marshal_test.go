package inference

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestInference_JSONIsCamelCase guards the wire contract with the frontend.
// The TS InferenceEvent/TokenStats expect camelCase keys; without json tags Go
// marshals PascalCase, which leaves event.tokens undefined in the frontend and
// crashes the dashboard render. This test fails loudly if the tags regress.
func TestInference_JSONIsCamelCase(t *testing.T) {
	inf := Inference{
		At:         time.Date(2026, time.June, 17, 3, 0, 0, 0, time.UTC),
		Endpoint:   "/api/generate",
		Method:     "POST",
		Model:      "gemma4:12b",
		PromptSize: 64,
		Streaming:  true,
		Status:     PhaseCompleted,
		Tokens: &TokenStats{
			PromptEvalCount: 5,
			EvalCount:       8,
			EvalDuration:    173 * time.Millisecond,
			TotalDuration:   926 * time.Millisecond,
			LoadDuration:    10 * time.Millisecond,
			PerSec:          46.3,
			LatencyMS:       926,
		},
	}

	raw, err := json.Marshal(inf)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(raw)

	wantKeys := []string{
		`"at"`, `"endpoint"`, `"method"`, `"model"`, `"promptSize"`,
		`"streaming"`, `"status"`, `"tokens"`,
		`"promptEvalCount"`, `"evalCount"`, `"evalDuration"`,
		`"totalDuration"`, `"loadDuration"`, `"perSec"`, `"latencyMS"`,
	}
	for _, k := range wantKeys {
		if !strings.Contains(got, k) {
			t.Errorf("expected camelCase key %s in JSON, got: %s", k, got)
		}
	}

	for _, bad := range []string{`"Endpoint"`, `"Tokens"`, `"PerSec"`, `"PromptSize"`, `"Status"`} {
		if strings.Contains(got, bad) {
			t.Errorf("found PascalCase key %s (breaks frontend contract): %s", bad, got)
		}
	}
}

// TestInference_NilTokensMarshalsNull confirms unavailable metrics serialize as
// JSON null (not an empty object), matching the TS `tokens: TokenStats | null`.
func TestInference_NilTokensMarshalsNull(t *testing.T) {
	inf := Inference{Endpoint: "/api/tags", Status: PhaseMetadataOnly}
	raw, err := json.Marshal(inf)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"tokens":null`) {
		t.Errorf("expected nil tokens to marshal as null, got: %s", raw)
	}
}
