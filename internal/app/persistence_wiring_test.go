package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"dllm-network/internal/events"
	"dllm-network/internal/store/sqlite"
	"dllm-network/internal/telemetry"
	"dllm-network/internal/telemetry/inference"
	"dllm-network/internal/telemetry/orchestrator"
)

// TestNew_WiresProductionStore_OnlyForRealEntrypoint asserts that New() (the
// production Wails entrypoint) marks the App to lazily open a real
// sqlite.Store on Startup, while NewWithDependencies (every other caller,
// i.e. every test) does not. The package-level newProductionStore seam is
// overridden here to a t.TempDir() path so this test never touches the
// real user profile.
func TestNew_WiresProductionStore_OnlyForRealEntrypoint(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "telemetry.db")

	original := newProductionStore
	newProductionStore = func() (*sqlite.Store, error) { return sqlite.Open(dbPath) }
	t.Cleanup(func() { newProductionStore = original })

	app := New()
	// New() wires real Window/Orchestrator/TrayManager defaults that would
	// touch the OS window/tray; override just enough to make Startup safe
	// to call in a unit test, keeping useProductionStore intact.
	app.window = &fakeWindow{}
	app.orchestrator = &fakeOrchestrator{state: orchestrator.StateRunning}
	app.trayManager = nil
	app.config = telemetry.Config{ShutdownTimeout: time.Second}

	if !app.useProductionStore {
		t.Fatal("expected New() to set useProductionStore=true")
	}

	app.Startup(context.Background())
	t.Cleanup(func() { _ = app.Quit() })

	app.bus.Publish(events.Event{
		Topic:   topicInferenceCompleted,
		Payload: inference.Inference{ID: "inf-production-seam-1"},
	})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		app.mu.RLock()
		ready := app.persistence != nil
		app.mu.RUnlock()
		if ready {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	app.mu.RLock()
	persistenceLC := app.persistence
	app.mu.RUnlock()
	if persistenceLC == nil {
		t.Fatal("expected Startup to open the (test-temp-dir) production store and start persistence")
	}
}

// TestNewWithDependencies_DoesNotSetUseProductionStore asserts the inverse:
// the test-facing constructor never opts into the production store seam.
func TestNewWithDependencies_DoesNotSetUseProductionStore(t *testing.T) {
	t.Parallel()

	app := NewWithDependencies(Dependencies{
		Window:       &fakeWindow{},
		Orchestrator: &fakeOrchestrator{state: orchestrator.StateRunning},
		Config:       telemetry.Config{ShutdownTimeout: time.Second},
	})

	if app.useProductionStore {
		t.Fatal("expected NewWithDependencies to leave useProductionStore=false")
	}
}
