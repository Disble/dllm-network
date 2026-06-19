package app

import (
	"testing"
	"time"

	"ollama-telemetry/internal/capture/httpx"
	"ollama-telemetry/internal/telemetry/inference"
)

func TestBuildCancelledInference_InferenceEndpoint(t *testing.T) {
	extractor := inference.NewExtractor()
	reqAt := time.Date(2026, 6, 19, 4, 20, 0, 0, time.UTC)
	lastAt := reqAt.Add(2 * time.Second)

	req := httpx.Message{
		Kind:   httpx.KindRequest,
		Method: "POST",
		Path:   "/v1/chat/completions",
		Body:   []byte(`{"model":"gemma4:12b"}`),
	}

	inf, ok := buildCancelledInference(extractor, req, "inf-7", reqAt, lastAt)
	if !ok {
		t.Fatal("expected a cancelled inference for an inference endpoint")
	}
	if inf.Status != inference.PhaseCancelled {
		t.Fatalf("status = %v, want PhaseCancelled", inf.Status)
	}
	if inf.ID != "inf-7" {
		t.Fatalf("id = %q, want inf-7", inf.ID)
	}
	if !inf.At.Equal(reqAt) {
		t.Fatalf("at = %v, want request time %v", inf.At, reqAt)
	}
	if inf.Model != "gemma4:12b" {
		t.Fatalf("model = %q, want gemma4:12b", inf.Model)
	}
	// The observed span MUST be preserved for the waterfall...
	if inf.Tokens == nil {
		t.Fatal("expected timing tokens for the waterfall span, got nil")
	}
	if inf.Tokens.TotalDuration != 2*time.Second {
		t.Fatalf("TotalDuration = %v, want 2s", inf.Tokens.TotalDuration)
	}
	if inf.Tokens.LatencyMS != 2000 {
		t.Fatalf("LatencyMS = %v, want 2000", inf.Tokens.LatencyMS)
	}
	// ...but token counts/throughput stay honestly zero (never measured).
	if inf.Tokens.EvalCount != 0 || inf.Tokens.PerSec != 0 {
		t.Fatalf("expected zero counts/perSec, got evalCount=%d perSec=%v", inf.Tokens.EvalCount, inf.Tokens.PerSec)
	}
}

func TestBuildCancelledInference_MetadataPollSkipped(t *testing.T) {
	extractor := inference.NewExtractor()
	reqAt := time.Now()

	req := httpx.Message{Kind: httpx.KindRequest, Method: "GET", Path: "/api/tags"}

	if _, ok := buildCancelledInference(extractor, req, "inf-1", reqAt, reqAt.Add(time.Second)); ok {
		t.Fatal("metadata-only polls must not surface as cancelled inferences")
	}
}

func TestCancelledTiming(t *testing.T) {
	t0 := time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC)

	if got := cancelledTiming(t0, t0.Add(1500*time.Millisecond)); got == nil {
		t.Fatal("expected non-nil timing for a positive span")
	} else if got.LatencyMS != 1500 || got.TotalDuration != 1500*time.Millisecond {
		t.Fatalf("got LatencyMS=%v TotalDuration=%v, want 1500ms", got.LatencyMS, got.TotalDuration)
	}

	if got := cancelledTiming(t0, t0); got != nil {
		t.Fatalf("expected nil timing for a zero span, got %+v", got)
	}
	if got := cancelledTiming(t0, t0.Add(-time.Second)); got != nil {
		t.Fatalf("expected nil timing for a negative span, got %+v", got)
	}
}
