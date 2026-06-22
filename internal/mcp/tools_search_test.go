package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"dllm-network/internal/store"
	"dllm-network/internal/telemetry/inference"
)

func TestSearchInferencesHandler_MapsArgsToSearchQueryAndReturnsSummaries(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.June, 19, 8, 0, 0, 0, time.UTC)
	reader := &fakeReader{searchResult: store.SearchInferencesResult{
		Items: []store.InferenceSummary{{
			ID: "inf-2", At: base, Model: "llama3", Endpoint: "/api/generate", Method: "POST",
			Status: "completed", StatusCode: 200, Streaming: true, PromptSize: 42,
		}},
		NextCursor: "opaque-next",
	}}

	_, out, err := handleSearchInferences(reader)(context.Background(), nil, searchInferencesInput{
		Model: "llama3", Endpoint: "/api/generate", Status: "completed",
		Since: base.Add(-time.Hour).Format(time.RFC3339), Until: base.Add(time.Hour).Format(time.RFC3339),
		Limit: 10, Cursor: "opaque-current",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reader.searchCalls != 1 {
		t.Fatalf("SearchInferences calls: got %d, want 1", reader.searchCalls)
	}
	if reader.lastSearchQuery.Model != "llama3" || reader.lastSearchQuery.Endpoint != "/api/generate" {
		t.Fatalf("query mapping mismatch: %+v", reader.lastSearchQuery)
	}
	if reader.lastSearchQuery.Status == nil || *reader.lastSearchQuery.Status != inference.PhaseCompleted {
		t.Fatalf("query status mismatch: %+v", reader.lastSearchQuery.Status)
	}
	if reader.lastSearchQuery.Cursor != "opaque-current" {
		t.Fatalf("Cursor: got %q, want opaque-current", reader.lastSearchQuery.Cursor)
	}
	if len(out.Items) != 1 || out.Items[0].ID != "inf-2" {
		t.Fatalf("Items: got %+v", out.Items)
	}
	if out.NextCursor != "opaque-next" {
		t.Fatalf("NextCursor: got %q, want opaque-next", out.NextCursor)
	}
	raw, err := json.Marshal(out.Items[0])
	if err != nil {
		t.Fatalf("Marshal summary: %v", err)
	}
	if strings.Contains(string(raw), "requestBody") || strings.Contains(string(raw), "responseBody") {
		t.Fatalf("search summary must omit heavy bodies, got %s", string(raw))
	}
}

func TestSearchInferencesHandler_DefaultsLimitAndRejectsOutOfRange(t *testing.T) {
	t.Parallel()

	reader := &fakeReader{}

	_, _, err := handleSearchInferences(reader)(context.Background(), nil, searchInferencesInput{Limit: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reader.lastSearchQuery.Limit != defaultSearchLimit {
		t.Fatalf("default limit: got %d, want %d", reader.lastSearchQuery.Limit, defaultSearchLimit)
	}

	_, _, err = handleSearchInferences(reader)(context.Background(), nil, searchInferencesInput{Limit: maxSearchLimit + 1})
	if err == nil {
		t.Fatal("expected error for limit above maximum, got nil")
	}
}
