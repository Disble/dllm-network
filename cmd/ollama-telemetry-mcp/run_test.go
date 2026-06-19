package main

import (
	"context"
	"errors"
	"testing"

	"ollama-telemetry/internal/store"
	"ollama-telemetry/internal/telemetry/inference"
)

// fakeReader is a minimal in-memory store.InferenceReader test double for
// the wiring tests below — they only need to prove serve() was invoked with
// a server built from whatever reader deps.openReader returned, not to
// exercise real MCP tool behavior (that is covered by internal/mcp's own
// test suite).
type fakeReader struct{}

func (fakeReader) ResolveInferenceContext(context.Context) (store.ResolveInferenceContextResult, error) {
	return store.ResolveInferenceContextResult{}, nil
}

func (fakeReader) SearchInferences(context.Context, store.SearchInferencesQuery) (store.SearchInferencesResult, error) {
	return store.SearchInferencesResult{}, nil
}

func (fakeReader) GetInferenceContext(context.Context, store.GetInferenceContextQuery) (store.GetInferenceContextResult, bool, error) {
	return store.GetInferenceContextResult{}, false, nil
}

func (fakeReader) Query(context.Context, store.Filter) ([]inference.Inference, error) {
	return nil, nil
}
func (fakeReader) Get(context.Context, string) (inference.Inference, bool, error) {
	return inference.Inference{}, false, nil
}
func (fakeReader) Stats(context.Context, store.Filter) (store.Stats, error) {
	return store.Stats{}, nil
}
func (fakeReader) Models(context.Context) ([]string, error) { return nil, nil }

var _ store.InferenceReader = fakeReader{}

// TestRun_DBMissing_FailsClearlyWithoutOpening proves run() refuses to open
// (and never creates) the database when existsFunc reports the DB file is
// absent — the sidecar is a reader; it must never bring a fresh, empty
// database into existence just because it ran before the GUI did.
func TestRun_DBMissing_FailsClearlyWithoutOpening(t *testing.T) {
	openCalls := 0
	deps := runDeps{
		resolvePath: func() (string, error) { return "C:/fake/telemetry.db", nil },
		exists:      func(string) bool { return false },
		openReader: func(string) (store.InferenceReader, closer, error) {
			openCalls++
			return fakeReader{}, nil, nil
		},
		serve: func(context.Context, store.InferenceReader) error {
			t.Fatal("serve must not be called when the DB is missing")
			return nil
		},
	}

	err := run(context.Background(), deps)

	if err == nil {
		t.Fatal("run() returned nil error, want a clear failure when the DB file does not exist")
	}
	if openCalls != 0 {
		t.Errorf("openReader was called %d times, want 0 — must not open/create when DB is missing", openCalls)
	}
}

// TestRun_PathResolutionFails_PropagatesError proves a path-resolution
// failure (e.g. os.UserCacheDir() erroring) is surfaced, not swallowed.
func TestRun_PathResolutionFails_PropagatesError(t *testing.T) {
	wantErr := errors.New("boom: cannot resolve cache dir")
	deps := runDeps{
		resolvePath: func() (string, error) { return "", wantErr },
		exists:      func(string) bool { t.Fatal("exists must not be called when path resolution fails"); return false },
		openReader: func(string) (store.InferenceReader, closer, error) {
			t.Fatal("openReader must not be called when path resolution fails")
			return nil, nil, nil
		},
		serve: func(context.Context, store.InferenceReader) error {
			t.Fatal("serve must not be called when path resolution fails")
			return nil
		},
	}

	err := run(context.Background(), deps)

	if !errors.Is(err, wantErr) {
		t.Errorf("run() error = %v, want it to wrap %v", err, wantErr)
	}
}

// TestRun_DBPresent_OpensReadOnlyAndServes proves the happy path: when the
// DB file exists, run() opens it via openReader and hands the resulting
// reader to serve.
func TestRun_DBPresent_OpensReadOnlyAndServes(t *testing.T) {
	wantPath := "C:/real/telemetry.db"
	var openedPath string
	var servedReader store.InferenceReader
	closeCalls := 0

	deps := runDeps{
		resolvePath: func() (string, error) { return wantPath, nil },
		exists:      func(p string) bool { return p == wantPath },
		openReader: func(p string) (store.InferenceReader, closer, error) {
			openedPath = p
			return fakeReader{}, closerFunc(func() error { closeCalls++; return nil }), nil
		},
		serve: func(_ context.Context, r store.InferenceReader) error {
			servedReader = r
			return nil
		},
	}

	if err := run(context.Background(), deps); err != nil {
		t.Fatalf("run() returned unexpected error: %v", err)
	}

	if openedPath != wantPath {
		t.Errorf("openReader called with path %q, want %q", openedPath, wantPath)
	}
	if servedReader == nil {
		t.Error("serve was not called with a reader")
	}
	if closeCalls != 1 {
		t.Errorf("closer.Close() called %d times, want exactly 1", closeCalls)
	}
}

// TestRun_OpenReaderFails_PropagatesError proves an open failure (e.g. a
// corrupt or schema-too-new DB) is surfaced rather than silently ignored.
func TestRun_OpenReaderFails_PropagatesError(t *testing.T) {
	wantErr := errors.New("schema too new")
	deps := runDeps{
		resolvePath: func() (string, error) { return "C:/real/telemetry.db", nil },
		exists:      func(string) bool { return true },
		openReader: func(string) (store.InferenceReader, closer, error) {
			return nil, nil, wantErr
		},
		serve: func(context.Context, store.InferenceReader) error {
			t.Fatal("serve must not be called when openReader fails")
			return nil
		},
	}

	err := run(context.Background(), deps)

	if !errors.Is(err, wantErr) {
		t.Errorf("run() error = %v, want it to wrap %v", err, wantErr)
	}
}

// closerFunc adapts a func() error to the closer interface for tests.
type closerFunc func() error

func (f closerFunc) Close() error { return f() }
