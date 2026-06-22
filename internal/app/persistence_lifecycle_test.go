package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"dllm-network/internal/events"
	"dllm-network/internal/telemetry"
	"dllm-network/internal/telemetry/inference"
	"dllm-network/internal/telemetry/orchestrator"
)

// TestAppStartup_StartsPersistenceSubscriber asserts that Startup starts the
// persistence subscriber's drain loop: a completed inference published on
// the shared bus is persisted via the injected Writer without the test
// needing to touch a real database.
func TestAppStartup_StartsPersistenceSubscriber(t *testing.T) {
	t.Parallel()

	writer := &fakePersistenceWriter{}
	app := NewWithDependencies(Dependencies{
		Window:            &fakeWindow{},
		Orchestrator:      &fakeOrchestrator{state: orchestrator.StateRunning},
		Config:            telemetry.Config{ShutdownTimeout: time.Second},
		PersistenceWriter: writer,
	})

	app.Startup(context.Background())
	t.Cleanup(func() { _ = app.Quit() })

	app.bus.Publish(events.Event{
		Topic:   topicInferenceCompleted,
		Payload: inference.Inference{ID: "inf-lifecycle-1"},
	})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && writer.RowCount() == 0 {
		time.Sleep(10 * time.Millisecond)
	}

	if got := writer.RowCount(); got != 1 {
		t.Fatalf("expected startup to start the subscriber drain loop and persist 1 row, got %d", got)
	}
}

// TestAppQuit_StopsAndFlushesPersistenceSubscriber asserts that Quit stops
// the subscriber (final flush) and closes the store, mirroring the existing
// pipeline run/stop lifecycle.
func TestAppQuit_StopsAndFlushesPersistenceSubscriber(t *testing.T) {
	t.Parallel()

	writer := &fakePersistenceWriter{}
	app := NewWithDependencies(Dependencies{
		Window:            &fakeWindow{},
		Orchestrator:      &fakeOrchestrator{state: orchestrator.StateRunning},
		Config:            telemetry.Config{ShutdownTimeout: time.Second},
		PersistenceWriter: writer,
	})

	app.Startup(context.Background())

	// Publish a row that has NOT yet hit the flush thresholds (default
	// batchSize=64 / 250ms) — Quit's final flush must still persist it.
	app.bus.Publish(events.Event{
		Topic:   topicInferenceCompleted,
		Payload: inference.Inference{ID: "inf-lifecycle-quit-1"},
	})

	if err := app.Quit(); err != nil {
		t.Fatalf("quit: %v", err)
	}

	if got := writer.RowCount(); got != 1 {
		t.Fatalf("expected quit's final flush to persist the buffered row, got %d", got)
	}
	if !writer.closed {
		t.Fatal("expected quit to close the persistence writer/store")
	}
}

// TestAppStartup_NoPersistenceWriter_NeverTouchesDisk asserts that an App
// built via NewWithDependencies WITHOUT a PersistenceWriter never opens the
// real production sqlite.Store on disk — Startup must run with persistence
// disabled (app.persistence stays nil) rather than silently falling back to
// a real file under the user's profile. Only New() (the production Wails
// entrypoint, see useProductionStore) wires the real store. This locks in
// the regression fixed during Slice 2 apply: every pre-existing app_test.go
// /capture_pipeline_test.go test calls Startup without injecting a writer
// and must stay disk-free.
func TestAppStartup_NoPersistenceWriter_NeverTouchesDisk(t *testing.T) {
	t.Parallel()

	app := NewWithDependencies(Dependencies{
		Window:       &fakeWindow{},
		Orchestrator: &fakeOrchestrator{state: orchestrator.StateRunning},
		Config:       telemetry.Config{ShutdownTimeout: time.Second},
	})

	app.Startup(context.Background())
	t.Cleanup(func() { _ = app.Quit() })

	app.mu.RLock()
	persistenceLC := app.persistence
	app.mu.RUnlock()

	if persistenceLC != nil {
		t.Fatal("expected NewWithDependencies without PersistenceWriter to leave persistence disabled (nil), not open a real store")
	}
}

// fakePersistenceWriter is a test double satisfying both persistence.Writer
// (Save, Prune) and an optional Close, letting App tests avoid opening a
// real sqlite.Store on disk. Save/Prune run on the subscriber's own drain
// goroutine while tests read RowCount/closed from the test goroutine, so
// access is guarded by mu.
type fakePersistenceWriter struct {
	mu      sync.Mutex
	batches [][]inference.Inference
	closed  bool
}

func (w *fakePersistenceWriter) Save(ctx context.Context, infs []inference.Inference) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.batches = append(w.batches, append([]inference.Inference(nil), infs...))
	return nil
}

// Prune is a no-op here: these App-level lifecycle tests assert
// startup/shutdown wiring, not retention behavior (covered at the
// internal/persistence integration-test level instead).
func (w *fakePersistenceWriter) Prune(ctx context.Context, maxCount int, maxAge time.Duration) error {
	return nil
}

func (w *fakePersistenceWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	return nil
}

func (w *fakePersistenceWriter) RowCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := 0
	for _, b := range w.batches {
		n += len(b)
	}
	return n
}
