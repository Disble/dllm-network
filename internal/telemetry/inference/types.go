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
