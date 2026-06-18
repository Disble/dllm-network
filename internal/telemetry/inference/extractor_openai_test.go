package inference

import (
	"testing"

	"ollama-telemetry/internal/capture/httpx"
)

// Real captured fixtures from Ollama's OpenAI-compatible endpoint
// (POST /v1/chat/completions on gemma4:12b). Streaming responses are SSE
// (data: {json}\n\n ... data: [DONE]); non-streaming is a single JSON body.
const (
	v1ContentChunk = `{"id":"chatcmpl-852","object":"chat.completion.chunk","created":1781801403,"model":"gemma4:12b","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"},"finish_reason":null}]}`
	v1UsageChunk   = `{"id":"chatcmpl-852","object":"chat.completion.chunk","created":1781801404,"model":"gemma4:12b","choices":[],"usage":{"prompt_tokens":21,"completion_tokens":5,"total_tokens":26}}`
	v1NonStream    = `{"id":"chatcmpl-379","object":"chat.completion","created":1781801472,"model":"gemma4:12b","choices":[{"index":0,"message":{"role":"assistant","content":"Hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":17,"completion_tokens":3,"total_tokens":20}}`
)

// TestExtractor_OpenAIEndpointRecognised verifies /v1/chat/completions is
// classified as an inference endpoint (NOT metadata-only), with the model
// parsed from the OpenAI-style request body.
func TestExtractor_OpenAIEndpointRecognised(t *testing.T) {
	t.Parallel()

	req := makeRequest("POST", "/v1/chat/completions",
		[]byte(`{"model":"gemma4:12b","messages":[{"role":"user","content":"hi"}]}`))
	// An empty response models request-only observation (in-progress).
	inf, ok := NewExtractor().FromExchange(req, httpx.Message{})
	if !ok {
		t.Fatal("expected ok=true for /v1/chat/completions")
	}
	if inf.Status == PhaseMetadataOnly {
		t.Fatal("/v1/chat/completions must NOT be metadata-only")
	}
	if inf.Status != PhaseInProgress {
		t.Errorf("request-only should be in-progress, got status=%d", inf.Status)
	}
	if inf.Model != "gemma4:12b" {
		t.Errorf("model: got %q, want gemma4:12b", inf.Model)
	}
}

// TestExtractor_OpenAIContentChunkInProgress verifies a streamed content delta
// is in-progress (no terminal, no metrics).
func TestExtractor_OpenAIContentChunkInProgress(t *testing.T) {
	t.Parallel()

	req := makeRequest("POST", "/v1/chat/completions", []byte(`{"model":"gemma4:12b"}`))
	inf, _ := NewExtractor().FromExchange(req, httpx.Message{Kind: httpx.KindResponse, Body: []byte(v1ContentChunk)})
	if inf.Status != PhaseInProgress {
		t.Errorf("content chunk should be in-progress, got status=%d", inf.Status)
	}
	if inf.Tokens != nil {
		t.Error("content chunk must not carry token metrics")
	}
}

// TestExtractor_OpenAIDoneCompletes verifies the SSE [DONE] sentinel marks the
// stream complete.
func TestExtractor_OpenAIDoneCompletes(t *testing.T) {
	t.Parallel()

	req := makeRequest("POST", "/v1/chat/completions", []byte(`{"model":"gemma4:12b"}`))
	inf, _ := NewExtractor().FromExchange(req, httpx.Message{Kind: httpx.KindResponse, Body: []byte("[DONE]")})
	if inf.Status != PhaseCompleted {
		t.Errorf("[DONE] should complete the stream, got status=%d", inf.Status)
	}
}

// TestExtractor_OpenAINonStreamCompletesWithStats verifies a non-streaming
// chat.completion body completes and yields token counts from `usage`.
func TestExtractor_OpenAINonStreamCompletesWithStats(t *testing.T) {
	t.Parallel()

	req := makeRequest("POST", "/v1/chat/completions", []byte(`{"model":"gemma4:12b"}`))
	inf, _ := NewExtractor().FromExchange(req, httpx.Message{Kind: httpx.KindResponse, Body: []byte(v1NonStream)})
	if inf.Status != PhaseCompleted {
		t.Fatalf("non-stream completion should complete, got status=%d", inf.Status)
	}
	if inf.Tokens == nil {
		t.Fatal("non-stream completion must carry token counts from usage")
	}
	if inf.Tokens.PromptEvalCount != 17 || inf.Tokens.EvalCount != 3 {
		t.Errorf("counts: got prompt=%d eval=%d, want 17/3", inf.Tokens.PromptEvalCount, inf.Tokens.EvalCount)
	}
}

// TestExtractOpenAIStats covers usage extraction from an assembled SSE blob and
// the honest nil when no usage is present (stream without include_usage).
func TestExtractOpenAIStats(t *testing.T) {
	t.Parallel()

	// Assembled SSE blob: content chunk + usage chunk + [DONE], each as a data: line.
	blob := []byte("data: " + v1ContentChunk + "\ndata: " + v1UsageChunk + "\ndata: [DONE]\n")
	stats := ExtractOpenAIStats(blob)
	if stats == nil {
		t.Fatal("expected stats from the usage chunk")
	}
	if stats.PromptEvalCount != 21 || stats.EvalCount != 5 {
		t.Errorf("counts: got prompt=%d eval=%d, want 21/5", stats.PromptEvalCount, stats.EvalCount)
	}

	// Stream WITHOUT a usage chunk (include_usage not set) → honest nil.
	noUsage := []byte("data: " + v1ContentChunk + "\ndata: [DONE]\n")
	if ExtractOpenAIStats(noUsage) != nil {
		t.Error("no usage chunk → stats must be nil (not fabricated)")
	}
}
