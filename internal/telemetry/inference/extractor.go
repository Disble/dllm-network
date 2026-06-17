package inference

import (
	"encoding/json"
	"time"

	"ollama-telemetry/internal/capture/httpx"
)

// inferenceEndpoints is the set of Ollama endpoints that produce token
// performance metrics in their done:true terminal NDJSON line.
var inferenceEndpoints = map[string]bool{
	"/api/generate": true,
	"/api/chat":     true,
}

// Extractor derives Inference domain values from captured HTTP exchanges.
// The zero value is ready to use; call NewExtractor for clarity.
type Extractor struct{}

// NewExtractor returns a ready-to-use Extractor.
func NewExtractor() *Extractor { return &Extractor{} }

// FromExchange derives an Inference value from a single captured HTTP
// request/response pair. The response Message should be the terminal done:true
// NDJSON line for completed inferences, or any intermediate line for in-progress
// streams.
//
// Returns (Inference, true) in all cases where the exchange can be classified —
// including metadata-only endpoints and in-progress streams. Returns
// (Inference{}, false) only when the request is structurally unrecognisable
// (e.g. nil body on an inference endpoint where a model field is required).
func (e *Extractor) FromExchange(req, resp httpx.Message) (Inference, bool) {
	meta := extractRequestMetadata(req)
	inf := Inference{
		At:         time.Now(),
		Endpoint:   meta.endpoint,
		Method:     meta.method,
		Model:      meta.model,
		PromptSize: meta.promptSize,
	}

	// Non-inference endpoints produce metadata-only results.
	if !inferenceEndpoints[meta.endpoint] {
		inf.Status = PhaseMetadataOnly
		// Tokens remains nil — metrics are structurally unavailable.
		return inf, true
	}

	// Inference endpoint: check whether the terminal line has been seen.
	if !resp.Done {
		inf.Status = PhaseInProgress
		inf.Streaming = true
		// Tokens remains nil — no derived values without the done:true line.
		return inf, true
	}

	// Terminal done:true line: derive metrics.
	inf.Status = PhaseCompleted
	inf.Streaming = true
	inf.Tokens = extractTokenStats(resp.Body)
	return inf, true
}

// ---- metadata extraction (request side) ------------------------------------

type requestMeta struct {
	endpoint   string
	method     string
	model      string
	promptSize int
}

// extractRequestMetadata pulls endpoint, method, model, and size from the
// parsed HTTP request Message.
func extractRequestMetadata(req httpx.Message) requestMeta {
	m := requestMeta{
		endpoint:   req.Path,
		method:     req.Method,
		promptSize: len(req.Body),
	}

	// Try to decode model from request body JSON.
	if len(req.Body) > 0 {
		var payload struct {
			Model string `json:"model"`
		}
		if err := json.Unmarshal(req.Body, &payload); err == nil {
			m.model = payload.Model
		}
	}

	return m
}

// ---- metrics derivation (response side) ------------------------------------

// ollamaStats mirrors the Ollama terminal NDJSON payload fields that carry
// performance counters. All duration fields are in nanoseconds.
type ollamaStats struct {
	Done            bool  `json:"done"`
	PromptEvalCount int   `json:"prompt_eval_count"`
	EvalCount       int   `json:"eval_count"`
	EvalDuration    int64 `json:"eval_duration"`  // nanoseconds
	TotalDuration   int64 `json:"total_duration"` // nanoseconds
	LoadDuration    int64 `json:"load_duration"`  // nanoseconds
}

// extractTokenStats parses the terminal NDJSON line body and derives
// tokens/sec and latency. Returns a zero-field *TokenStats (non-nil) even
// when the duration is zero to avoid a nil dereference — callers already gate
// on Status==PhaseCompleted.
func extractTokenStats(body []byte) *TokenStats {
	var s ollamaStats
	if err := json.Unmarshal(body, &s); err != nil {
		// Body could not be decoded; return a non-nil but empty stats to signal
		// "completed but counters unreadable" — callers see PhaseCompleted + Tokens!=nil.
		return &TokenStats{}
	}

	stats := &TokenStats{
		PromptEvalCount: s.PromptEvalCount,
		EvalCount:       s.EvalCount,
		EvalDuration:    time.Duration(s.EvalDuration),
		TotalDuration:   time.Duration(s.TotalDuration),
		LoadDuration:    time.Duration(s.LoadDuration),
	}

	// Derive tokens/sec: avoid divide-by-zero when eval_duration is absent.
	if s.EvalDuration > 0 {
		secs := float64(s.EvalDuration) / 1e9
		stats.PerSec = float64(s.EvalCount) / secs
	}

	// Derive latency in milliseconds from total_duration (nanoseconds).
	stats.LatencyMS = float64(s.TotalDuration) / 1e6

	return stats
}
