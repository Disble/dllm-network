package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"dllm-network/internal/telemetry/inference"
)

// TestStore_SaveAndGet_RoundTrip is the first vertical tracer bullet for the
// SQLite persistence store (slice 1 / PR1, task 1.1). It exercises the public
// Open -> Save -> Get contract end to end against a real temp-dir database,
// including the edge cases the domain explicitly guards: nil Tokens (status
// not applicable, never coerced to zero), truncated bodies, and nil header
// slices (not captured, distinct from empty).
func TestStore_SaveAndGet_RoundTrip(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "telemetry.db")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	want := inference.Inference{
		ID:                    "inf-1",
		At:                    time.Date(2026, time.June, 18, 9, 30, 0, 0, time.UTC),
		Endpoint:              "/api/generate",
		Method:                "POST",
		Model:                 "gemma3:12b",
		PromptSize:            64,
		Streaming:             true,
		Status:                inference.PhaseCompleted,
		Tokens:                nil, // status not applicable: must round-trip as nil, not zero-value.
		StatusCode:            200,
		RequestBody:           "truncated-request",
		RequestBodyTruncated:  true,
		ResponseBody:          "truncated-response",
		ResponseBodyTruncated: true,
		RequestHeaders:        nil, // not captured, distinct from empty slice.
		ResponseHeaders:       nil,
	}

	ctx := context.Background()
	if err := st.Save(ctx, []inference.Inference{want}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, ok, err := st.Get(ctx, want.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatalf("Get: expected found=true for id %q", want.ID)
	}

	if got.Tokens != nil {
		t.Errorf("expected nil Tokens to round-trip as nil, got %+v", got.Tokens)
	}
	if got.RequestHeaders != nil {
		t.Errorf("expected nil RequestHeaders to round-trip as nil, got %+v", got.RequestHeaders)
	}
	if got.ResponseHeaders != nil {
		t.Errorf("expected nil ResponseHeaders to round-trip as nil, got %+v", got.ResponseHeaders)
	}

	want.Tokens = nil // already asserted above; zero it for the bulk comparison below.
	assertInferenceEqual(t, got, want)
}

// TestStore_RoundTrip_FullMetricsAndHeaders (task 1.5) covers the opposite
// edge from the nil-Tokens case above: a fully completed inference with
// non-nil TokenStats and non-empty request/response headers must preserve
// every metric field and header ordering exactly.
func TestStore_RoundTrip_FullMetricsAndHeaders(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "telemetry.db")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	want := inference.Inference{
		ID:         "inf-full",
		At:         time.Date(2026, time.June, 18, 10, 0, 0, 0, time.UTC),
		Endpoint:   "/api/generate",
		Method:     "POST",
		Model:      "llama3:8b",
		PromptSize: 128,
		Streaming:  true,
		Status:     inference.PhaseCompleted,
		Tokens: &inference.TokenStats{
			PromptEvalCount: 12,
			EvalCount:       340,
			EvalDuration:    900 * time.Millisecond,
			TotalDuration:   1200 * time.Millisecond,
			LoadDuration:    50 * time.Millisecond,
			PerSec:          377.7,
			LatencyMS:       1200,
		},
		StatusCode:   200,
		RequestBody:  `{"model":"llama3:8b"}`,
		ResponseBody: `{"done":true}`,
		Generation: &inference.Generation{
			Output:         "Hola mundo",
			Reasoning:      "thinking...",
			FinishReason:   "stop",
			ContextSize:    8,
			ContextPreview: []int{1, 2, 3, 4, 5, 6},
		},
		RequestHeaders: []inference.Header{
			{Name: "Content-Type", Value: "application/json"},
			{Name: "X-Request-Id", Value: "abc-123"},
		},
		ResponseHeaders: []inference.Header{
			{Name: "Content-Type", Value: "application/x-ndjson"},
		},
	}

	ctx := context.Background()
	if err := st.Save(ctx, []inference.Inference{want}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, ok, err := st.Get(ctx, want.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatalf("Get: expected found=true for id %q", want.ID)
	}

	assertInferenceEqual(t, got, want)
}

// TestStore_NullTokenColumns_WhenTokensNil (task 1.6) verifies the flattened
// scalar token-stat columns are written as SQL NULL — not 0 — when
// inference.Inference.Tokens is nil. This guards the storage layer's half of
// the domain's nil-vs-zero "not applicable" contract directly against the
// database, independent of the detail-blob round-trip already covered by
// TestStore_SaveAndGet_RoundTrip.
func TestStore_NullTokenColumns_WhenTokensNil(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "telemetry.db")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	ctx := context.Background()
	inf := inference.Inference{
		ID:       "inf-metadata-only",
		At:       time.Date(2026, time.June, 18, 11, 0, 0, 0, time.UTC),
		Endpoint: "/api/tags",
		Method:   "GET",
		Status:   inference.PhaseMetadataOnly,
		Tokens:   nil,
	}
	if err := st.Save(ctx, []inference.Inference{inf}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var (
		promptEvalCount, evalCount, evalDurationNS sql.NullInt64
		totalDurationNS, loadDurationNS            sql.NullInt64
		perSec, latencyMS                          sql.NullFloat64
	)
	row := st.db.QueryRowContext(ctx, `SELECT prompt_eval_count, eval_count,
		eval_duration_ns, total_duration_ns, load_duration_ns, per_sec, latency_ms
		FROM inferences WHERE id = ?`, inf.ID)
	if err := row.Scan(&promptEvalCount, &evalCount, &evalDurationNS,
		&totalDurationNS, &loadDurationNS, &perSec, &latencyMS); err != nil {
		t.Fatalf("scan token columns: %v", err)
	}

	checkNull := func(name string, valid bool) {
		if valid {
			t.Errorf("%s: expected NULL (Tokens==nil), got a non-NULL value", name)
		}
	}
	checkNull("prompt_eval_count", promptEvalCount.Valid)
	checkNull("eval_count", evalCount.Valid)
	checkNull("eval_duration_ns", evalDurationNS.Valid)
	checkNull("total_duration_ns", totalDurationNS.Valid)
	checkNull("load_duration_ns", loadDurationNS.Valid)
	checkNull("per_sec", perSec.Valid)
	checkNull("latency_ms", latencyMS.Valid)
}
