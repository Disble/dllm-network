package app

import (
	"context"
	"testing"

	"ollama-telemetry/internal/store"
	"ollama-telemetry/internal/telemetry/inference"
)

// fakeInferenceReader is a store.InferenceReader test double; only Get is
// exercised by the InferenceDetail binding.
type fakeInferenceReader struct {
	gotID string
	inf   inference.Inference
	ok    bool
}

func (f *fakeInferenceReader) Get(_ context.Context, id string) (inference.Inference, bool, error) {
	f.gotID = id
	return f.inf, f.ok, nil
}
func (f *fakeInferenceReader) Query(context.Context, store.Filter) ([]inference.Inference, error) {
	return nil, nil
}
func (f *fakeInferenceReader) Stats(context.Context, store.Filter) (store.Stats, error) {
	return store.Stats{}, nil
}
func (f *fakeInferenceReader) Models(context.Context) ([]string, error) { return nil, nil }

func TestInferenceDetail_FetchesFullRecordFromReader(t *testing.T) {
	reader := &fakeInferenceReader{
		inf: inference.Inference{ID: "inf-5", Model: "gemma", ResponseBody: "hello world"},
		ok:  true,
	}
	app := &App{inferenceReader: reader}

	got, err := app.InferenceDetail("inf-5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reader.gotID != "inf-5" {
		t.Fatalf("reader queried %q, want inf-5", reader.gotID)
	}
	if got.ResponseBody != "hello world" || got.ID != "inf-5" {
		t.Fatalf("got %+v, want full record for inf-5", got)
	}
}

func TestInferenceDetail_NilReaderReturnsZero(t *testing.T) {
	app := &App{} // no persistence wired

	got, err := app.InferenceDetail("anything")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "" || got.ResponseBody != "" {
		t.Fatalf("expected zero inference when reader is nil, got %+v", got)
	}
}

func TestInferenceDetail_UnknownIDReturnsZero(t *testing.T) {
	reader := &fakeInferenceReader{ok: false}
	app := &App{inferenceReader: reader}

	got, err := app.InferenceDetail("missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "" {
		t.Fatalf("expected zero inference for unknown id, got %+v", got)
	}
}
