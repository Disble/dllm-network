package mcp

import (
	"context"
	"testing"
)

func TestListModelsHandler_ReturnsDistinctModelsFromReader(t *testing.T) {
	reader := &fakeReader{modelsResult: []string{"llama3", "mistral"}}

	_, out, err := handleListModels(reader)(context.Background(), nil, listModelsInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reader.modelsCalls != 1 {
		t.Fatalf("Models calls: got %d, want 1", reader.modelsCalls)
	}
	if len(out.Models) != 2 || out.Models[0] != "llama3" || out.Models[1] != "mistral" {
		t.Errorf("Models: got %v, want [llama3 mistral]", out.Models)
	}
}

func TestListModelsHandler_EmptyStore_ReturnsEmptyNotError(t *testing.T) {
	reader := &fakeReader{modelsResult: []string{}}

	_, out, err := handleListModels(reader)(context.Background(), nil, listModelsInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Models) != 0 {
		t.Errorf("Models: got %d entries, want 0", len(out.Models))
	}
}

func TestListModelsHandler_PropagatesReaderError(t *testing.T) {
	reader := &fakeReader{modelsErr: context.DeadlineExceeded}

	_, _, err := handleListModels(reader)(context.Background(), nil, listModelsInput{})
	if err == nil {
		t.Fatal("expected error to propagate from reader.Models, got nil")
	}
}
