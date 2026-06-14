package orchestrator

import (
	"context"
	"testing"
	"time"

	"ollama-telemetry/internal/telemetry"
)

func TestNewUsesDefaultCadencesAndRunningState(t *testing.T) {
	t.Parallel()

	runtime := New(telemetry.Config{})

	if got := runtime.State(); got != StateRunning {
		t.Fatalf("expected new orchestrator to start running, got %q", got)
	}

	config := runtime.Config()
	if config.ShutdownTimeout <= 0 {
		t.Fatalf("expected positive shutdown timeout, got %s", config.ShutdownTimeout)
	}

	if config.Cadence.API <= 0 || config.Cadence.Logs <= 0 || config.Cadence.System <= 0 {
		t.Fatalf("expected positive cadence defaults, got %+v", config.Cadence)
	}
}

func TestOrchestratorPauseResumeStopTransitions(t *testing.T) {
	t.Parallel()

	runtime := New(telemetry.Config{
		ShutdownTimeout: 200 * time.Millisecond,
		Cadence: telemetry.CadenceConfig{
			API:    time.Second,
			Logs:   2 * time.Second,
			System: 3 * time.Second,
		},
	})

	if err := runtime.Pause(context.Background()); err != nil {
		t.Fatalf("pause: %v", err)
	}

	if got := runtime.State(); got != StatePaused {
		t.Fatalf("expected paused state, got %q", got)
	}

	if err := runtime.Resume(context.Background()); err != nil {
		t.Fatalf("resume: %v", err)
	}

	if got := runtime.State(); got != StateRunning {
		t.Fatalf("expected running state after resume, got %q", got)
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if err := runtime.Stop(stopCtx); err != nil {
		t.Fatalf("stop: %v", err)
	}

	if got := runtime.State(); got != StateStopped {
		t.Fatalf("expected stopped state after stop, got %q", got)
	}

	if err := runtime.Resume(context.Background()); err != nil {
		t.Fatalf("resume after stop should be ignored safely, got %v", err)
	}

	if got := runtime.State(); got != StateStopped {
		t.Fatalf("expected stopped state to remain terminal, got %q", got)
	}
	if err := runtime.Pause(context.Background()); err != nil {
		t.Fatalf("pause after stop should be ignored safely, got %v", err)
	}
	if got := runtime.State(); got != StateStopped {
		t.Fatalf("expected stopped state to remain terminal after pause, got %q", got)
	}
}
