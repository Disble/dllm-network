package mcp

import (
	"context"

	"ollama-telemetry/internal/store"
	"ollama-telemetry/internal/telemetry/inference"
)

// fakeReader is an in-memory test double for store.InferenceReader. It lets
// handler tests assert input->call->output mapping without a real SQLite
// store. Each method records the arguments it was called with so tests can
// assert the handler translated its typed input into the correct Filter/id.
type fakeReader struct {
	queryResult []inference.Inference
	queryErr    error
	lastFilter  store.Filter
	queryCalls  int

	getResult inference.Inference
	getOK     bool
	getErr    error
	lastGetID string
	getCalls  int

	statsResult store.Stats
	statsErr    error
	statsFilter store.Filter
	statsCalls  int

	modelsResult []string
	modelsErr    error
	modelsCalls  int
}

func (f *fakeReader) Query(_ context.Context, filter store.Filter) ([]inference.Inference, error) {
	f.lastFilter = filter
	f.queryCalls++
	return f.queryResult, f.queryErr
}

func (f *fakeReader) Get(_ context.Context, id string) (inference.Inference, bool, error) {
	f.lastGetID = id
	f.getCalls++
	return f.getResult, f.getOK, f.getErr
}

func (f *fakeReader) Stats(_ context.Context, filter store.Filter) (store.Stats, error) {
	f.statsFilter = filter
	f.statsCalls++
	return f.statsResult, f.statsErr
}

func (f *fakeReader) Models(_ context.Context) ([]string, error) {
	f.modelsCalls++
	return f.modelsResult, f.modelsErr
}

var _ store.InferenceReader = (*fakeReader)(nil)
