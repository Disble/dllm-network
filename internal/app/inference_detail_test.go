package app

import (
	"context"
	"encoding/json"
	"testing"

	"dllm-network/internal/store"
	"dllm-network/internal/telemetry/inference"
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
func (f *fakeInferenceReader) ResolveInferenceContext(context.Context) (store.ResolveInferenceContextResult, error) {
	return store.ResolveInferenceContextResult{}, nil
}
func (f *fakeInferenceReader) SearchInferences(context.Context, store.SearchInferencesQuery) (store.SearchInferencesResult, error) {
	return store.SearchInferencesResult{}, nil
}
func (f *fakeInferenceReader) GetInferenceContext(context.Context, store.GetInferenceContextQuery) (store.GetInferenceContextResult, bool, error) {
	return store.GetInferenceContextResult{}, false, nil
}
func (f *fakeInferenceReader) Query(context.Context, store.Filter) ([]inference.Inference, error) {
	return nil, nil
}
func (f *fakeInferenceReader) Stats(context.Context, store.Filter) (store.Stats, error) {
	return store.Stats{}, nil
}
func (f *fakeInferenceReader) Models(context.Context) ([]string, error) { return nil, nil }

func TestInferenceDetail_FetchesFullRecordAsJSON(t *testing.T) {
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

	// The binding returns JSON (not the domain type) so Wails can bind it; it
	// must round-trip back into the domain shape.
	var decoded inference.Inference
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("InferenceDetail did not return valid JSON (%q): %v", got, err)
	}
	if decoded.ID != "inf-5" || decoded.ResponseBody != "hello world" {
		t.Fatalf("decoded = %+v, want full record for inf-5", decoded)
	}
}

func TestInferenceDetail_NilReaderReturnsEmpty(t *testing.T) {
	app := &App{} // no persistence wired

	got, err := app.InferenceDetail("anything")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty string when reader is nil, got %q", got)
	}
}

func TestInferenceDetail_UnknownIDReturnsEmpty(t *testing.T) {
	reader := &fakeInferenceReader{ok: false}
	app := &App{inferenceReader: reader}

	got, err := app.InferenceDetail("missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty string for unknown id, got %q", got)
	}
}
