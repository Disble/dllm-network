package mcp

import (
	"context"
	"testing"

	"ollama-telemetry/internal/store"
)

func TestGetStatsHandler_MapsModelFilterAndReturnsAggregate(t *testing.T) {
	want := store.Stats{
		Count:        7,
		PerSecP50:    12.5,
		PerSecP95:    20.0,
		LatencyMSP50: 100,
		LatencyMSP95: 250,
		ByModel:      []store.ModelStats{{Model: "llama3", Count: 5}, {Model: "mistral", Count: 2}},
	}
	reader := &fakeReader{statsResult: want}

	_, out, err := handleGetStats(reader)(context.Background(), nil, getStatsInput{Model: "llama3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reader.statsCalls != 1 {
		t.Fatalf("Stats calls: got %d, want 1", reader.statsCalls)
	}
	if reader.statsFilter.Model != "llama3" {
		t.Errorf("filter.Model: got %q, want %q", reader.statsFilter.Model, "llama3")
	}
	if out.Stats.Count != 7 {
		t.Errorf("Count: got %d, want 7", out.Stats.Count)
	}
	if len(out.Stats.ByModel) != 2 {
		t.Errorf("ByModel: got %d entries, want 2", len(out.Stats.ByModel))
	}
}

func TestGetStatsHandler_WithTimeWindow_AppliesSinceUntil(t *testing.T) {
	reader := &fakeReader{statsResult: store.Stats{}}

	since := "2026-01-01T00:00:00Z"
	until := "2026-01-02T00:00:00Z"
	_, _, err := handleGetStats(reader)(context.Background(), nil, getStatsInput{Since: since, Until: until})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reader.statsFilter.Since.IsZero() || reader.statsFilter.Until.IsZero() {
		t.Errorf("expected Since/Until to be parsed and non-zero, got %+v", reader.statsFilter)
	}
}

func TestGetStatsHandler_EmptyDataset_ReturnsZeroCounts(t *testing.T) {
	reader := &fakeReader{statsResult: store.Stats{}}

	_, out, err := handleGetStats(reader)(context.Background(), nil, getStatsInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Stats.Count != 0 {
		t.Errorf("Count: got %d, want 0", out.Stats.Count)
	}
}

func TestGetStatsHandler_PropagatesReaderError(t *testing.T) {
	reader := &fakeReader{statsErr: context.DeadlineExceeded}

	_, _, err := handleGetStats(reader)(context.Background(), nil, getStatsInput{})
	if err == nil {
		t.Fatal("expected error to propagate from reader.Stats, got nil")
	}
}

func TestGetStatsHandler_InvalidSince_ReturnsError(t *testing.T) {
	reader := &fakeReader{}

	_, _, err := handleGetStats(reader)(context.Background(), nil, getStatsInput{Since: "not-a-time"})
	if err == nil {
		t.Fatal("expected error for invalid since timestamp, got nil")
	}
}
