package mcp

import (
	"context"

	"dllm-network/internal/store"
	"dllm-network/internal/telemetry/inference"
)

// fakeReader is an in-memory test double for store.InferenceReader. It lets
// handler tests assert input->call->output mapping without a real SQLite
// store. Each method records the arguments it was called with so tests can
// assert the handler translated its typed input into the correct Filter/id.
type fakeReader struct {
	resolveResult store.ResolveInferenceContextResult
	resolveErr    error
	resolveCalls  int

	searchResult    store.SearchInferencesResult
	searchErr       error
	lastSearchQuery store.SearchInferencesQuery
	searchCalls     int

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

	getContextResult store.GetInferenceContextResult
	getContextOK     bool
	getContextErr    error
	lastContextQuery store.GetInferenceContextQuery
	getContextCalls  int
}

func (f *fakeReader) ResolveInferenceContext(_ context.Context) (store.ResolveInferenceContextResult, error) {
	f.resolveCalls++
	return f.resolveResult, f.resolveErr
}

func (f *fakeReader) SearchInferences(_ context.Context, query store.SearchInferencesQuery) (store.SearchInferencesResult, error) {
	f.lastSearchQuery = query
	f.searchCalls++
	return f.searchResult, f.searchErr
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

func (f *fakeReader) GetInferenceContext(_ context.Context, query store.GetInferenceContextQuery) (store.GetInferenceContextResult, bool, error) {
	f.lastContextQuery = query
	if query.Body != nil {
		body := *query.Body
		f.lastContextQuery.Body = &body
	}
	if len(query.Sections) > 0 {
		f.lastContextQuery.Sections = append([]store.InferenceContextSection(nil), query.Sections...)
	}
	f.getContextCalls++
	return f.getContextResult, f.getContextOK, f.getContextErr
}

var _ store.InferenceReader = (*fakeReader)(nil)
