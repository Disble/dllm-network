// Package inference implements a pure extractor that derives domain-level
// inference metrics from HTTP request/response exchanges captured by
// internal/capture/httpx. It has ZERO dependency on any capture driver, OS
// API, or live source — fully unit-testable with golden fixtures.
package inference

import (
	"encoding/json"
	"go/build"
	"strings"
	"testing"

	"ollama-telemetry/internal/capture/httpx"
)

// ---- helpers ----------------------------------------------------------------

// makeRequest builds a minimal httpx.Message of KindRequest.
func makeRequest(method, path string, body []byte) httpx.Message {
	return httpx.Message{
		Kind:   httpx.KindRequest,
		Method: method,
		Path:   path,
		Body:   body,
	}
}

// makeResponseLine builds an httpx.Message of KindResponse representing a
// single NDJSON line. done should be set for the terminal line.
func makeResponseLine(body []byte, done bool) httpx.Message {
	return httpx.Message{
		Kind: httpx.KindResponse,
		Body: body,
		Done: done,
	}
}

// marshalJSON is a test helper that panics on error (tests only).
func marshalJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// ---- 3.1 RED: POST /api/chat request metadata --------------------------------

// TestExtractor_PostChatRequestMetadata verifies that a well-formed POST
// /api/chat request body is parsed for endpoint, method, model, and size.
func TestExtractor_PostChatRequestMetadata(t *testing.T) {
	t.Parallel()

	body := marshalJSON(map[string]interface{}{
		"model": "llama3",
		"messages": []map[string]string{
			{"role": "user", "content": "Hello, how are you?"},
		},
		"options": map[string]interface{}{
			"temperature": 0.7,
		},
	})

	req := makeRequest("POST", "/api/chat", body)
	// No response Messages yet — simulate exchange with terminal done line later.
	resp := makeResponseLine(marshalJSON(map[string]interface{}{"done": true, "eval_count": 1}), true)

	e := NewExtractor()
	inf, ok := e.FromExchange(req, resp)

	if !ok {
		t.Fatal("FromExchange returned ok=false for a valid inference request")
	}
	if inf.Endpoint != "/api/chat" {
		t.Errorf("endpoint: got %q, want %q", inf.Endpoint, "/api/chat")
	}
	if inf.Method != "POST" {
		t.Errorf("method: got %q, want %q", inf.Method, "POST")
	}
	if inf.Model != "llama3" {
		t.Errorf("model: got %q, want %q", inf.Model, "llama3")
	}
	if inf.PromptSize <= 0 {
		t.Errorf("PromptSize should be non-zero, got %d", inf.PromptSize)
	}
}

// ---- 3.3 RED: tokens/sec and latency from done:true line ---------------------

// TestExtractor_TokensPerSecAndLatencyFromDoneLine verifies derived metrics are
// computed correctly from the terminal NDJSON line. eval_duration is in
// nanoseconds; tokens/sec = eval_count / (eval_duration / 1e9).
func TestExtractor_TokensPerSecAndLatencyFromDoneLine(t *testing.T) {
	t.Parallel()

	// Fixture: 48 tokens in 2.4 seconds → 20.0 tokens/sec.
	// total_duration 2.6 s → LatencyMS = 2600.0 ms.
	doneBody := marshalJSON(map[string]interface{}{
		"done":              true,
		"prompt_eval_count": 12,
		"eval_count":        48,
		"eval_duration":     2400000000, // nanoseconds
		"total_duration":    2600000000, // nanoseconds
		"load_duration":     50000000,   // nanoseconds
	})

	req := makeRequest("POST", "/api/generate", marshalJSON(map[string]interface{}{
		"model":  "llama3",
		"prompt": "Tell me a story",
	}))
	resp := makeResponseLine(doneBody, true)

	e := NewExtractor()
	inf, ok := e.FromExchange(req, resp)

	if !ok {
		t.Fatal("FromExchange returned ok=false")
	}
	if inf.Status != PhaseCompleted {
		t.Errorf("status: got %v, want PhaseCompleted", inf.Status)
	}

	// TokensPerSec: 48 / 2.4 = 20.0 (exact).
	if inf.Tokens == nil {
		t.Fatal("Tokens must be non-nil for a completed inference")
	}
	const wantTPS = 20.0
	if inf.Tokens.PerSec != wantTPS {
		t.Errorf("TokensPerSec: got %v, want %v", inf.Tokens.PerSec, wantTPS)
	}

	// LatencyMS: total_duration 2_600_000_000 ns = 2600 ms.
	const wantLatencyMS = 2600.0
	if inf.Tokens.LatencyMS != wantLatencyMS {
		t.Errorf("LatencyMS: got %v, want %v", inf.Tokens.LatencyMS, wantLatencyMS)
	}

	// Raw counts must be preserved.
	if inf.Tokens.PromptEvalCount != 12 {
		t.Errorf("PromptEvalCount: got %d, want 12", inf.Tokens.PromptEvalCount)
	}
	if inf.Tokens.EvalCount != 48 {
		t.Errorf("EvalCount: got %d, want 48", inf.Tokens.EvalCount)
	}
}

// ---- 3.5 RED: in-progress streaming (no terminal line yet) ------------------

// TestExtractor_StreamingNoTerminalLineYet verifies that when no done:true line
// has been seen, the extractor reports PhaseInProgress with a nil Tokens field
// (no fabricated values).
func TestExtractor_StreamingNoTerminalLineYet(t *testing.T) {
	t.Parallel()

	// Intermediate line — done=false.
	intermediateBody := marshalJSON(map[string]interface{}{
		"model":    "llama3",
		"response": "Hello",
		"done":     false,
	})

	req := makeRequest("POST", "/api/generate", marshalJSON(map[string]interface{}{
		"model":  "llama3",
		"prompt": "Say hello",
	}))
	resp := makeResponseLine(intermediateBody, false) // Done=false: no terminal

	e := NewExtractor()
	inf, ok := e.FromExchange(req, resp)

	if !ok {
		t.Fatal("FromExchange returned ok=false for an in-progress inference")
	}
	if inf.Status != PhaseInProgress {
		t.Errorf("status: got %v, want PhaseInProgress", inf.Status)
	}
	if inf.Tokens != nil {
		t.Errorf("Tokens must be nil when no terminal line seen, got %+v", inf.Tokens)
	}
}

// ---- 3.7 RED: non-inference endpoint has no token metrics -------------------

// TestExtractor_NonInferenceEndpointHasNoTokenMetrics verifies that /api/tags
// (which never returns eval_count/eval_duration) produces an Inference with
// Tokens==nil and Status==PhaseMetadataOnly, not zero-valued fields.
func TestExtractor_NonInferenceEndpointHasNoTokenMetrics(t *testing.T) {
	t.Parallel()

	// /api/tags response is a JSON object listing local models, not NDJSON.
	tagsBody := marshalJSON(map[string]interface{}{
		"models": []map[string]string{
			{"name": "llama3:latest"},
		},
	})

	req := makeRequest("GET", "/api/tags", nil)
	resp := httpx.Message{
		Kind:       httpx.KindResponse,
		StatusCode: 200,
		Body:       tagsBody,
		Done:       false,
	}

	e := NewExtractor()
	inf, ok := e.FromExchange(req, resp)

	// ok=true but metrics unavailable.
	if !ok {
		t.Fatal("FromExchange returned ok=false — non-inference endpoints should still return metadata")
	}
	if inf.Status != PhaseMetadataOnly {
		t.Errorf("status: got %v, want PhaseMetadataOnly", inf.Status)
	}
	if inf.Tokens != nil {
		t.Errorf("Tokens must be nil for non-inference endpoint, got %+v", inf.Tokens)
	}
	if inf.Endpoint != "/api/tags" {
		t.Errorf("endpoint: got %q, want %q", inf.Endpoint, "/api/tags")
	}
}

// ---- table-driven edge cases ------------------------------------------------

// TestExtractor_TableDriven covers additional metadata and status scenarios.
func TestExtractor_TableDriven(t *testing.T) {
	t.Parallel()

	inferenceEndpoints := []string{"/api/generate", "/api/chat"}

	for _, endpoint := range inferenceEndpoints {
		endpoint := endpoint
		t.Run("inference_endpoint_"+endpoint, func(t *testing.T) {
			t.Parallel()

			body := marshalJSON(map[string]interface{}{
				"model":  "mistral",
				"prompt": "Hi",
			})
			doneBody := marshalJSON(map[string]interface{}{
				"done":           true,
				"eval_count":     10,
				"eval_duration":  1000000000, // 1 second → 10 tok/s
				"total_duration": 1200000000,
			})
			req := makeRequest("POST", endpoint, body)
			resp := makeResponseLine(doneBody, true)

			e := NewExtractor()
			inf, ok := e.FromExchange(req, resp)

			if !ok {
				t.Fatalf("expected ok=true for %s", endpoint)
			}
			if inf.Status != PhaseCompleted {
				t.Errorf("expected PhaseCompleted, got %v", inf.Status)
			}
			if inf.Tokens == nil {
				t.Fatal("Tokens must not be nil for completed inference")
			}
			if inf.Tokens.PerSec != 10.0 {
				t.Errorf("PerSec: got %v, want 10.0", inf.Tokens.PerSec)
			}
			if inf.Model != "mistral" {
				t.Errorf("model: got %q, want %q", inf.Model, "mistral")
			}
		})
	}

	t.Run("non_inference_version_endpoint", func(t *testing.T) {
		t.Parallel()
		req := makeRequest("GET", "/api/version", nil)
		resp := httpx.Message{Kind: httpx.KindResponse, StatusCode: 200,
			Body: marshalJSON(map[string]string{"version": "0.1.0"})}

		e := NewExtractor()
		inf, ok := e.FromExchange(req, resp)

		if !ok {
			t.Fatal("expected ok=true for metadata-only endpoint")
		}
		if inf.Status != PhaseMetadataOnly {
			t.Errorf("expected PhaseMetadataOnly, got %v", inf.Status)
		}
		if inf.Tokens != nil {
			t.Errorf("Tokens must be nil for /api/version, got %+v", inf.Tokens)
		}
	})
}

// ---- purity gate ------------------------------------------------------------

// TestExtractor_RunsWithoutElevation asserts this package has zero dependency
// on WinDivert, syscall, or any capture driver. It inspects non-test imports
// via go/build and fails loudly without needing admin rights.
func TestExtractor_RunsWithoutElevation(t *testing.T) {
	t.Parallel()

	pkg, err := build.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("import package: %v", err)
	}

	forbidden := []string{"syscall", "ollama-telemetry/internal/capture/reassembly"}
	for _, imp := range pkg.Imports {
		for _, bad := range forbidden {
			if imp == bad || strings.HasPrefix(imp, bad+"/") {
				t.Fatalf("inference package must not import %q, got %q", bad, imp)
			}
		}
	}
}
