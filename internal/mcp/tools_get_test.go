package mcp

import (
	"context"
	"testing"

	"ollama-telemetry/internal/telemetry/inference"
)

func TestGetInferenceHandler_FoundID_ReturnsFullRecord(t *testing.T) {
	want := inference.Inference{
		ID:              "inf-1",
		Model:           "llama3",
		RequestBody:     "prompt body",
		ResponseBody:    "response body",
		RequestHeaders:  []inference.Header{{Name: "X-Test", Value: "1"}},
		ResponseHeaders: []inference.Header{{Name: "X-Test", Value: "2"}},
	}
	reader := &fakeReader{getResult: want, getOK: true}

	_, out, err := handleGetInference(reader)(context.Background(), nil, getInferenceInput{ID: "inf-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reader.getCalls != 1 {
		t.Fatalf("Get calls: got %d, want 1", reader.getCalls)
	}
	if reader.lastGetID != "inf-1" {
		t.Errorf("Get id: got %q, want %q", reader.lastGetID, "inf-1")
	}
	if !out.Found {
		t.Fatal("Found: got false, want true")
	}
	if out.Inference.RequestBody != "prompt body" || out.Inference.ResponseBody != "response body" {
		t.Errorf("bodies not preserved: got %+v", out.Inference)
	}
	if len(out.Inference.RequestHeaders) != 1 || len(out.Inference.ResponseHeaders) != 1 {
		t.Errorf("headers not preserved: got %+v", out.Inference)
	}
}

func TestGetInferenceHandler_UnknownID_ReturnsNotFoundWithoutError(t *testing.T) {
	reader := &fakeReader{getOK: false}

	_, out, err := handleGetInference(reader)(context.Background(), nil, getInferenceInput{ID: "missing"})
	if err != nil {
		t.Fatalf("expected no error for unknown id, got: %v", err)
	}
	if out.Found {
		t.Error("Found: got true, want false for unknown id")
	}
}

func TestGetInferenceHandler_PropagatesReaderError(t *testing.T) {
	reader := &fakeReader{getErr: context.DeadlineExceeded}

	_, _, err := handleGetInference(reader)(context.Background(), nil, getInferenceInput{ID: "inf-1"})
	if err == nil {
		t.Fatal("expected error to propagate from reader.Get, got nil")
	}
}
