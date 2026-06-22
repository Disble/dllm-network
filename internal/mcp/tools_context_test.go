package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"dllm-network/internal/store"
)

func TestResolveInferenceContextHandler_ReturnsReaderUniverse(t *testing.T) {
	t.Parallel()

	oldest := time.Date(2026, time.June, 1, 10, 0, 0, 0, time.UTC)
	latest := oldest.Add(2 * time.Hour)
	reader := &fakeReader{resolveResult: store.ResolveInferenceContextResult{
		Models:           []store.FacetCount{{Value: "llama3", Count: 2}},
		Endpoints:        []store.FacetCount{{Value: "/api/generate", Count: 2}},
		Statuses:         []store.FacetCount{{Value: "completed", Count: 2}},
		TimeRange:        store.InferenceTimeRange{Oldest: &oldest, Latest: &latest},
		Counts:           store.InferenceCounts{Total: 2},
		SupportedFilters: []string{"model", "endpoint", "status", "since", "until"},
	}}

	_, out, err := handleResolveInferenceContext(reader)(context.Background(), nil, resolveInferenceContextInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reader.resolveCalls != 1 {
		t.Fatalf("ResolveInferenceContext calls: got %d, want 1", reader.resolveCalls)
	}
	if out.Context.Counts.Total != 2 {
		t.Fatalf("Counts.Total: got %d, want 2", out.Context.Counts.Total)
	}
	if len(out.Context.Models) != 1 || out.Context.Models[0].Value != "llama3" {
		t.Fatalf("Models: got %+v", out.Context.Models)
	}
	if out.Context.TimeRange.Oldest == nil || !out.Context.TimeRange.Oldest.Equal(oldest) {
		t.Fatalf("TimeRange.Oldest: got %v, want %v", out.Context.TimeRange.Oldest, oldest)
	}
	raw, err := json.Marshal(out.Context)
	if err != nil {
		t.Fatalf("Marshal context: %v", err)
	}
	if strings.Contains(string(raw), "requestBody") || strings.Contains(string(raw), "responseBody") {
		t.Fatalf("resolve output must stay lightweight, got %s", string(raw))
	}
}

func TestResolveInferenceContextHandler_EmptyDatasetStillDeclaresFilters(t *testing.T) {
	t.Parallel()

	reader := &fakeReader{resolveResult: store.ResolveInferenceContextResult{
		SupportedFilters: []string{"model", "endpoint", "status", "since", "until"},
	}}

	_, out, err := handleResolveInferenceContext(reader)(context.Background(), nil, resolveInferenceContextInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Context.SupportedFilters) != 5 {
		t.Fatalf("SupportedFilters: got %d entries, want 5", len(out.Context.SupportedFilters))
	}
	if out.Context.Counts.Total != 0 {
		t.Fatalf("Counts.Total: got %d, want 0", out.Context.Counts.Total)
	}
}
