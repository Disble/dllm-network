package mcp

import (
	"context"
	"testing"
	"time"

	"ollama-telemetry/internal/telemetry/inference"
)

func TestQueryInferencesHandler_MapsArgsToFilterAndReturnsResults(t *testing.T) {
	want := []inference.Inference{
		{ID: "inf-1", Model: "llama3", Endpoint: "/api/generate", Status: inference.PhaseCompleted},
		{ID: "inf-2", Model: "llama3", Endpoint: "/api/generate", Status: inference.PhaseCompleted},
	}
	reader := &fakeReader{queryResult: want}

	since := "2026-01-01T00:00:00Z"
	until := "2026-01-02T00:00:00Z"
	in := queryInferencesInput{
		Model:    "llama3",
		Endpoint: "/api/generate",
		Status:   "completed",
		Since:    since,
		Until:    until,
		Limit:    10,
	}

	_, out, err := handleQueryInferences(reader)(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if reader.queryCalls != 1 {
		t.Fatalf("Query calls: got %d, want 1", reader.queryCalls)
	}
	if reader.lastFilter.Model != "llama3" {
		t.Errorf("filter.Model: got %q, want %q", reader.lastFilter.Model, "llama3")
	}
	if reader.lastFilter.Endpoint != "/api/generate" {
		t.Errorf("filter.Endpoint: got %q, want %q", reader.lastFilter.Endpoint, "/api/generate")
	}
	if reader.lastFilter.Status == nil || *reader.lastFilter.Status != inference.PhaseCompleted {
		t.Errorf("filter.Status: got %v, want PhaseCompleted", reader.lastFilter.Status)
	}
	wantSince, _ := time.Parse(time.RFC3339, since)
	if !reader.lastFilter.Since.Equal(wantSince) {
		t.Errorf("filter.Since: got %v, want %v", reader.lastFilter.Since, wantSince)
	}
	wantUntil, _ := time.Parse(time.RFC3339, until)
	if !reader.lastFilter.Until.Equal(wantUntil) {
		t.Errorf("filter.Until: got %v, want %v", reader.lastFilter.Until, wantUntil)
	}
	if reader.lastFilter.Limit != 10 {
		t.Errorf("filter.Limit: got %d, want 10", reader.lastFilter.Limit)
	}

	if len(out.Inferences) != 2 {
		t.Fatalf("Inferences: got %d, want 2", len(out.Inferences))
	}
	if out.Inferences[0].ID != "inf-1" || out.Inferences[1].ID != "inf-2" {
		t.Errorf("Inferences IDs: got [%q, %q], want [inf-1, inf-2]", out.Inferences[0].ID, out.Inferences[1].ID)
	}
}

func TestQueryInferencesHandler_NoFilters_PassesZeroFilter(t *testing.T) {
	reader := &fakeReader{queryResult: []inference.Inference{}}

	_, out, err := handleQueryInferences(reader)(context.Background(), nil, queryInferencesInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if reader.lastFilter.Model != "" || reader.lastFilter.Endpoint != "" {
		t.Errorf("expected zero-value filter, got %+v", reader.lastFilter)
	}
	if reader.lastFilter.Status != nil {
		t.Errorf("expected nil Status (no filter), got %v", *reader.lastFilter.Status)
	}
	if len(out.Inferences) != 0 {
		t.Errorf("expected empty results, got %d", len(out.Inferences))
	}
}

func TestQueryInferencesHandler_PropagatesReaderError(t *testing.T) {
	reader := &fakeReader{queryErr: context.DeadlineExceeded}

	_, _, err := handleQueryInferences(reader)(context.Background(), nil, queryInferencesInput{})
	if err == nil {
		t.Fatal("expected error to propagate from reader.Query, got nil")
	}
}

func TestQueryInferencesHandler_InvalidStatus_ReturnsError(t *testing.T) {
	reader := &fakeReader{}

	_, _, err := handleQueryInferences(reader)(context.Background(), nil, queryInferencesInput{Status: "bogus"})
	if err == nil {
		t.Fatal("expected error for invalid status string, got nil")
	}
}
