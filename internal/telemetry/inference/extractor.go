package inference

import (
	"bytes"
	"encoding/json"
	"time"

	"ollama-telemetry/internal/capture/httpx"
)

// inferenceEndpoints is the set of Ollama-native endpoints that produce token
// performance metrics in their done:true terminal NDJSON line.
var inferenceEndpoints = map[string]bool{
	"/api/generate": true,
	"/api/chat":     true,
}

// openaiEndpoints is the set of Ollama's OpenAI-compatible endpoints. Their
// responses are Server-Sent Events (streaming: data: {json} ... data: [DONE])
// or a single chat.completion JSON (non-streaming), and token counts arrive in
// an OpenAI-style `usage` object — NOT the Ollama done:true NDJSON shape.
var openaiEndpoints = map[string]bool{
	"/v1/chat/completions": true,
	"/v1/completions":      true,
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
	requestBody, requestTruncated := TruncateBody(req.Body)
	responseBody, responseTruncated := TruncateBody(resp.Body)
	inf := Inference{
		At:         time.Now(),
		Endpoint:   meta.endpoint,
		Method:     meta.method,
		Model:      meta.model,
		PromptSize: meta.promptSize,
		// DevTools-Network detail fields, populated for every phase. ResponseBody
		// here is the single terminal message; the pipeline overrides it with the
		// full assembled stream for streamed responses.
		StatusCode:            resp.StatusCode,
		RequestBody:           requestBody,
		RequestBodyTruncated:  requestTruncated,
		ResponseBody:          responseBody,
		ResponseBodyTruncated: responseTruncated,
		RequestHeaders:        convertHeaders(req.Headers),
		ResponseHeaders:       convertHeaders(resp.Headers),
	}

	// OpenAI-compatible endpoint: SSE / chat.completion semantics.
	if openaiEndpoints[meta.endpoint] {
		inf.Streaming = true
		if openAIDone(resp.Body) {
			inf.Status = PhaseCompleted
			// Counts come from a `usage` object in THIS body (non-stream) or, for
			// streaming, from the accumulated body the pipeline supplies later.
			inf.Tokens = ExtractOpenAIStats(resp.Body)
		} else {
			inf.Status = PhaseInProgress
		}
		return inf, true
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

// openAIDone reports whether an OpenAI-compatible response body marks the
// exchange complete: the SSE [DONE] sentinel (streaming) or a full
// chat.completion object (non-streaming). Streaming content/usage chunks carry
// object "chat.completion.chunk" and are NOT terminal.
func openAIDone(body []byte) bool {
	trimmed := bytes.TrimSpace(body)
	if bytes.Equal(trimmed, []byte("[DONE]")) {
		return true
	}
	var obj struct {
		Object string `json:"object"`
	}
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return false
	}
	return obj.Object == "chat.completion"
}

// ExtractOpenAIStats scans an OpenAI-compatible response body for the first
// `usage` object and returns the token counts. The body may be a single
// chat.completion JSON (non-streaming) or an assembled SSE blob of
// `data: {json}` lines (streaming). Returns nil — never fabricated — when no
// usage is present (e.g. a stream without stream_options.include_usage).
//
// OpenAI exposes only token COUNTS, not Ollama's nanosecond durations, so
// PerSec/LatencyMS stay zero here; the pipeline derives latency from wall clock.
func ExtractOpenAIStats(body []byte) *TokenStats {
	for _, line := range bytes.Split(body, []byte("\n")) {
		line = bytes.TrimSpace(bytes.TrimPrefix(bytes.TrimSpace(line), []byte("data: ")))
		if len(line) == 0 {
			continue
		}
		var obj struct {
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(line, &obj); err != nil || obj.Usage == nil {
			continue
		}
		return &TokenStats{
			PromptEvalCount: obj.Usage.PromptTokens,
			EvalCount:       obj.Usage.CompletionTokens,
		}
	}
	return nil
}

// MaxBodyBytes is the SAFETY CEILING for how many bytes of a request/response
// body are retained — NOT a data-minimization cap.
//
// Design decision (2026-06-18): raised from 64 KiB to 16 MiB so that real
// Ollama exchanges are captured in full. This tool is a network-clone inspector
// for Ollama: the whole point is to SEE the complete prompt and generated
// output. A 64 KiB cap truncated long chats, generated code, and reasoning
// traces mid-stream, which also broke the detail view's JSON/NDJSON pretty-
// printer (a body cut at an arbitrary byte is no longer valid to parse).
//
// Why a ceiling still exists (do NOT remove it):
//   - Backend memory is already bounded: store.Recent keeps only
//     defaultRecentModelLimit (12) completed inferences, so worst-case retention
//     is ~12 × (request + response) bodies. At 16 MiB that is a comfortable
//     upper bound for a local single-user tool.
//   - The REAL hang risk is the FRONTEND, not Go. The detail inspector renders
//     the body in a <pre> and runs JSON.parse + pretty-print on the webview's
//     main thread (see inference-detail-code-block.tsx). A pathological payload
//     of tens/hundreds of MiB (e.g. a huge embeddings batch) would freeze the
//     render, not the capture pipeline. The ceiling protects that path.
//
// If you ever see "Truncated at capture limit." on a legitimate response, it is
// safe to raise this value — backend memory scales linearly and stays bounded
// by the 12-event retention. Re-evaluate the frontend render cost before going
// far past ~16 MiB.
const MaxBodyBytes = 16 * 1024 * 1024

// TruncateBody returns the body as a string capped at MaxBodyBytes, and whether
// truncation occurred. A nil/empty body returns ("", false).
func TruncateBody(body []byte) (string, bool) {
	if len(body) <= MaxBodyBytes {
		return string(body), false
	}
	return string(body[:MaxBodyBytes]), true
}

// convertHeaders maps wire headers into domain headers, preserving order.
// Returns nil for an empty input so "not captured" stays distinct from "empty".
func convertHeaders(in []httpx.Header) []Header {
	if len(in) == 0 {
		return nil
	}
	out := make([]Header, len(in))
	for i, h := range in {
		out[i] = Header{Name: h.Name, Value: h.Value}
	}
	return out
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
