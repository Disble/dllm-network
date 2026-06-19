// Package inference provides pure, driver-free extraction of Ollama inference
// metrics from HTTP request/response exchanges captured by
// internal/capture/httpx. It defines the domain Inference type and derives
// tokens/sec and latency from the terminal done:true NDJSON line.
//
// The package has ZERO dependency on any capture driver, OS API, or the
// internal/capture/reassembly package — it is fully unit-testable without
// elevation using in-memory or golden-byte fixtures.
package inference

import "time"

// Phase describes the lifecycle state of an observed inference request.
type Phase int

const (
	// PhaseInProgress indicates the request is being streamed and the terminal
	// done:true NDJSON line has NOT yet been seen. No derived metrics are
	// available; callers MUST NOT fabricate or default them.
	PhaseInProgress Phase = iota

	// PhaseCompleted indicates the terminal done:true line was observed and all
	// metrics (tokens/sec, latency, token counts) have been derived.
	PhaseCompleted

	// PhaseMetadataOnly indicates the endpoint is not an Ollama inference
	// endpoint (e.g. /api/tags, /api/version, /api/ps). Request metadata such
	// as endpoint and method are available, but token metrics are structurally
	// unavailable and MUST NOT be reported as zero — callers treat Tokens==nil
	// as "not applicable".
	PhaseMetadataOnly

	// PhaseCancelled indicates the request was observed (in-progress) but its
	// connection went idle past the capture timeout before any completion was
	// seen — it was cancelled, abandoned, or its completion packets were lost.
	// Token counts and tokens/sec are unavailable, but the observed wall-clock
	// span (request → last activity) IS recorded in Tokens (TotalDuration /
	// LatencyMS only) so the waterfall can still place the bar. A stuck request
	// MUST surface as cancelled rather than hang in PhaseInProgress forever.
	PhaseCancelled
)

// TokenStats holds the raw Ollama response performance counters and their
// derived metrics. A nil *TokenStats means the metrics are unavailable — NOT
// that they are zero. Callers MUST check for nil before reading any field.
type TokenStats struct {
	// Raw counters from the terminal done:true NDJSON line.
	PromptEvalCount int `json:"promptEvalCount"` // prompt_eval_count
	EvalCount       int `json:"evalCount"`       // eval_count (generated tokens)

	// Raw durations from the terminal done:true NDJSON line (Ollama uses ns).
	EvalDuration  time.Duration `json:"evalDuration"`  // eval_duration
	TotalDuration time.Duration `json:"totalDuration"` // total_duration
	LoadDuration  time.Duration `json:"loadDuration"`  // load_duration

	// Derived metrics.
	// PerSec = EvalCount / EvalDuration.Seconds(); zero if EvalDuration==0.
	PerSec float64 `json:"perSec"`
	// LatencyMS = TotalDuration in milliseconds.
	LatencyMS float64 `json:"latencyMS"`
}

// Header is one HTTP header field surfaced to the dashboard, in wire order with
// original name casing. Mirrors the frontend HttpHeader contract.
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Generation is the normalized, provider-agnostic view of an LLM response's
// generated content. It is derived at the extractor boundary (the anti-
// corruption layer) so the frontend NEVER parses a wire format: Ollama-native
// (`response` / `message.content`, NDJSON) and OpenAI-compatible (`delta.content`
// SSE chunks or a single chat.completion) both collapse into this one shape —
// exactly as TokenStats already does for metrics.
//
// A nil *Generation means the exchange produced no generation payload (a
// metadata-only poll, or a response with no decodable content). Callers MUST
// check for nil before reading any field. Presentation concerns (pretty-printing
// JSON output, eliding the context preview into a string) stay in the frontend —
// this type carries DATA, not formatting.
type Generation struct {
	// Output is the assembled generated text: Ollama's joined `response` /
	// `message.content` tokens, or OpenAI's joined `delta.content`. "" when the
	// model emitted only reasoning (or nothing).
	Output string `json:"output"`

	// Reasoning is the assembled reasoning / thinking trace when the model and
	// endpoint expose one (Ollama `thinking`, OpenAI `delta.reasoning`). "" when
	// absent — most non-reasoning models never populate it.
	Reasoning string `json:"reasoning"`

	// FinishReason is the normalized stop reason ("stop", "length", "tool_calls",
	// …) from the terminal line / chunk. "" when the stream did not report one.
	FinishReason string `json:"finishReason"`

	// ContextSize is the number of Ollama context token IDs returned by
	// /api/generate (the conversational KV-cache handle). 0 when absent — OpenAI
	// endpoints never expose it.
	ContextSize int `json:"contextSize"`

	// ContextPreview holds the first few context token IDs (bounded) so the UI can
	// show a sample without the wire ever carrying a thousand-int array. nil when
	// no context is present. The frontend formats this into a display string.
	ContextPreview []int `json:"contextPreview"`

	// ToolCalls holds the function/tool calls the model emitted, reassembled from
	// the streamed deltas (OpenAI `delta.tool_calls`) or the single message
	// (`message.tool_calls`). nil when the model produced none. For agent clients
	// like GitHub Copilot this — not Output — is the real generated payload.
	ToolCalls []ToolCall `json:"toolCalls"`
}

// ToolCall is one normalized function/tool invocation requested by the model.
// Arguments is the raw JSON arguments string, reassembled across streamed chunks
// for OpenAI (where it arrives in fragments) — the frontend pretty-prints it.
type ToolCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Inference is the domain type produced by the extractor for a single captured
// HTTP exchange. It crosses the anti-corruption boundary from the capture
// pipeline into the dashboard projection layer.
type Inference struct {
	// ID is a stable identity for one logical exchange, assigned by the capture
	// pipeline. It stays constant across the in-progress -> completed updates of
	// the same request so the frontend upserts (not duplicates) on selection.
	ID string `json:"id"`

	// At is the wall-clock time the exchange was processed.
	At time.Time `json:"at"`

	// Request-side metadata.
	Endpoint   string `json:"endpoint"`   // HTTP path (e.g. "/api/generate")
	Method     string `json:"method"`     // HTTP method (e.g. "POST")
	Model      string `json:"model"`      // model field from request JSON body
	PromptSize int    `json:"promptSize"` // byte length of request body (prompt + options)
	Streaming  bool   `json:"streaming"`  // true when response was an NDJSON stream

	// Status describes the lifecycle phase of this inference.
	Status Phase `json:"status"`

	// Tokens holds derived and raw performance metrics. It is nil when
	// metrics are unavailable (Status==PhaseInProgress or PhaseMetadataOnly).
	Tokens *TokenStats `json:"tokens"`

	// Generation holds the normalized generated content (output, reasoning,
	// finish reason, context summary), derived from the assembled response body
	// by ExtractGeneration. nil when the exchange carries no decodable
	// generation payload. The capture pipeline populates it on completion from
	// the full assembled stream; the per-line extractor leaves it nil.
	Generation *Generation `json:"generation"`

	// ---- DevTools-Network detail fields (Slice A) --------------------------
	// StatusCode is the HTTP response status code (0 when not observed).
	StatusCode int `json:"statusCode"`

	// RequestBody is the captured request body (prompt + options), truncated at
	// MaxBodyBytes. RequestBodyTruncated reports whether truncation occurred.
	RequestBody          string `json:"requestBody"`
	RequestBodyTruncated bool   `json:"requestBodyTruncated"`

	// ResponseBody is the assembled response body (the joined NDJSON stream or a
	// single Content-Length payload), truncated at MaxBodyBytes. The capture
	// pipeline assembles this across streamed lines; the extractor seeds it from
	// the single terminal message. ResponseBodyTruncated reports truncation.
	ResponseBody          string `json:"responseBody"`
	ResponseBodyTruncated bool   `json:"responseBodyTruncated"`

	// RequestHeaders / ResponseHeaders are the captured headers in wire order.
	// Nil means not captured (passive mode), distinct from an empty exchange.
	RequestHeaders  []Header `json:"requestHeaders"`
	ResponseHeaders []Header `json:"responseHeaders"`
}
