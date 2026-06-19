package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ollama-telemetry/internal/telemetry/inference"
)

func TestInferenceByIDResource_FoundID_ReturnsRecordAsJSON(t *testing.T) {
	want := inference.Inference{ID: "inf-1", Model: "llama3"}
	reader := &fakeReader{getResult: want, getOK: true}

	req := &mcp.ReadResourceRequest{Params: &mcp.ReadResourceParams{URI: "inference://inf-1"}}
	result, err := handleInferenceByID(reader)(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reader.lastGetID != "inf-1" {
		t.Errorf("Get id: got %q, want %q", reader.lastGetID, "inf-1")
	}
	if len(result.Contents) != 1 {
		t.Fatalf("Contents: got %d, want 1", len(result.Contents))
	}

	var got inference.Inference
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &got); err != nil {
		t.Fatalf("failed to unmarshal resource content: %v", err)
	}
	if got.ID != "inf-1" || got.Model != "llama3" {
		t.Errorf("decoded content: got %+v, want ID=inf-1 Model=llama3", got)
	}
	if result.Contents[0].MIMEType != "application/json" {
		t.Errorf("MIMEType: got %q, want application/json", result.Contents[0].MIMEType)
	}
}

func TestInferenceByIDResource_UnknownID_ReturnsNotFoundError(t *testing.T) {
	reader := &fakeReader{getOK: false}

	req := &mcp.ReadResourceRequest{Params: &mcp.ReadResourceParams{URI: "inference://missing"}}
	_, err := handleInferenceByID(reader)(context.Background(), req)
	if err == nil {
		t.Fatal("expected not-found error for unknown id, got nil")
	}
}

func TestInferenceByIDResource_PropagatesReaderError(t *testing.T) {
	reader := &fakeReader{getErr: context.DeadlineExceeded}

	req := &mcp.ReadResourceRequest{Params: &mcp.ReadResourceParams{URI: "inference://inf-1"}}
	_, err := handleInferenceByID(reader)(context.Background(), req)
	if err == nil {
		t.Fatal("expected error to propagate from reader.Get, got nil")
	}
}

func TestInferenceRecentResource_ReturnsBoundedRecentList(t *testing.T) {
	want := []inference.Inference{
		{ID: "inf-1"},
		{ID: "inf-2"},
	}
	reader := &fakeReader{queryResult: want}

	req := &mcp.ReadResourceRequest{Params: &mcp.ReadResourceParams{URI: "inference://recent"}}
	result, err := handleInferenceRecent(reader)(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reader.lastFilter.Limit != recentResourceLimit {
		t.Errorf("filter.Limit: got %d, want %d", reader.lastFilter.Limit, recentResourceLimit)
	}

	var got []inference.Inference
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &got); err != nil {
		t.Fatalf("failed to unmarshal resource content: %v", err)
	}
	if len(got) != 2 || got[0].ID != "inf-1" || got[1].ID != "inf-2" {
		t.Errorf("decoded content: got %+v", got)
	}
}

func TestInferenceRecentResource_PropagatesReaderError(t *testing.T) {
	reader := &fakeReader{queryErr: context.DeadlineExceeded}

	req := &mcp.ReadResourceRequest{Params: &mcp.ReadResourceParams{URI: "inference://recent"}}
	_, err := handleInferenceRecent(reader)(context.Background(), req)
	if err == nil {
		t.Fatal("expected error to propagate from reader.Query, got nil")
	}
}
