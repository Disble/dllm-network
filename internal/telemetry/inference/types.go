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
)

// TokenStats holds the raw Ollama response performance counters and their
// derived metrics. A nil *TokenStats means the metrics are unavailable — NOT
// that they are zero. Callers MUST check for nil before reading any field.
type TokenStats struct {
	// Raw counters from the terminal done:true NDJSON line.
	PromptEvalCount int // prompt_eval_count
	EvalCount       int // eval_count (generated tokens)

	// Raw durations from the terminal done:true NDJSON line (Ollama uses ns).
	EvalDuration  time.Duration // eval_duration
	TotalDuration time.Duration // total_duration
	LoadDuration  time.Duration // load_duration

	// Derived metrics.
	// PerSec = EvalCount / EvalDuration.Seconds(); zero if EvalDuration==0.
	PerSec float64
	// LatencyMS = TotalDuration in milliseconds.
	LatencyMS float64
}

// Inference is the domain type produced by the extractor for a single captured
// HTTP exchange. It crosses the anti-corruption boundary from the capture
// pipeline into the dashboard projection layer.
type Inference struct {
	// At is the wall-clock time the exchange was processed.
	At time.Time

	// Request-side metadata.
	Endpoint   string // HTTP path (e.g. "/api/generate")
	Method     string // HTTP method (e.g. "POST")
	Model      string // model field from request JSON body
	PromptSize int    // byte length of request body (prompt + options)
	Streaming  bool   // true when response was an NDJSON stream

	// Status describes the lifecycle phase of this inference.
	Status Phase

	// Tokens holds derived and raw performance metrics. It is nil when
	// metrics are unavailable (Status==PhaseInProgress or PhaseMetadataOnly).
	Tokens *TokenStats
}
