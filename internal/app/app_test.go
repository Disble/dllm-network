package app

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"ollama-telemetry/internal/telemetry"
	"ollama-telemetry/internal/telemetry/orchestrator"
)

type testContextKey string

func TestAppLifecycleWindowVisibility(t *testing.T) {
	t.Parallel()

	window := &fakeWindow{}
	controller := &fakeOrchestrator{state: orchestrator.StateRunning}
	app := NewWithDependencies(Dependencies{
		Window:       window,
		Orchestrator: controller,
		Config: telemetry.Config{
			ShutdownTimeout: time.Second,
		},
	})
	startupCtx := context.WithValue(context.Background(), testContextKey("startup"), "ready")
	app.Startup(startupCtx)

	if got := app.Status(); got.WindowVisible {
		t.Fatalf("expected startup to remain hidden, got visible status: %+v", got)
	}

	if err := app.Show(); err != nil {
		t.Fatalf("show: %v", err)
	}

	if got := app.Status(); !got.WindowVisible {
		t.Fatalf("expected status to report visible window after show, got %+v", got)
	}

	if err := app.Hide(); err != nil {
		t.Fatalf("hide: %v", err)
	}

	if got := app.Status(); got.WindowVisible {
		t.Fatalf("expected status to report hidden window after hide, got %+v", got)
	}

	if !slices.Equal(window.calls, []string{"show", "hide"}) {
		t.Fatalf("expected window calls [show hide], got %v", window.calls)
	}

	if len(window.ctxValues) != 2 || window.ctxValues[0] != "ready" || window.ctxValues[1] != "ready" {
		t.Fatalf("expected show and hide to reuse startup context, got %v", window.ctxValues)
	}
}

func TestAppLifecyclePauseResume(t *testing.T) {
	t.Parallel()

	controller := &fakeOrchestrator{state: orchestrator.StateRunning}
	app := NewWithDependencies(Dependencies{
		Window:       &fakeWindow{},
		Orchestrator: controller,
		Config:       telemetry.Config{ShutdownTimeout: time.Second},
	})
	app.Startup(context.Background())

	if err := app.Pause(); err != nil {
		t.Fatalf("pause: %v", err)
	}

	if got := app.Status(); got.CollectionState != string(orchestrator.StatePaused) {
		t.Fatalf("expected paused status, got %+v", got)
	}

	if err := app.Resume(); err != nil {
		t.Fatalf("resume: %v", err)
	}

	if got := app.Status(); got.CollectionState != string(orchestrator.StateRunning) {
		t.Fatalf("expected running status after resume, got %+v", got)
	}

	if !slices.Equal(controller.calls, []string{"pause", "resume"}) {
		t.Fatalf("expected pause and resume orchestration calls, got %v", controller.calls)
	}
}

func TestAppQuitStopsCollectorsBeforeClosingWindow(t *testing.T) {
	t.Parallel()

	t.Run("clean quit drains orchestrator before quitting window", func(t *testing.T) {
		recorder := &callRecorder{}
		window := &fakeWindow{recorder: recorder}
		controller := &fakeOrchestrator{state: orchestrator.StateRunning, recorder: recorder}
		shutdownTimeout := 150 * time.Millisecond
		app := NewWithDependencies(Dependencies{
			Window:       window,
			Orchestrator: controller,
			Config:       telemetry.Config{ShutdownTimeout: shutdownTimeout},
		})
		app.Startup(context.Background())

		before := time.Now()
		if err := app.Quit(); err != nil {
			t.Fatalf("quit: %v", err)
		}

		if !slices.Equal(recorder.calls, []string{"stop", "quit"}) {
			t.Fatalf("expected stop before quit, got %v", recorder.calls)
		}

		if controller.stopDeadline.IsZero() {
			t.Fatal("expected stop context to carry a deadline")
		}

		remaining := controller.stopDeadline.Sub(before)
		if remaining <= 0 || remaining > shutdownTimeout {
			t.Fatalf("expected quit timeout within 0..%s, got %s", shutdownTimeout, remaining)
		}
	})

	t.Run("stop failure blocks window quit", func(t *testing.T) {
		recorder := &callRecorder{}
		window := &fakeWindow{recorder: recorder}
		controller := &fakeOrchestrator{
			state:    orchestrator.StateRunning,
			recorder: recorder,
			stopErr:  errors.New("drain failed"),
		}
		app := NewWithDependencies(Dependencies{
			Window:       window,
			Orchestrator: controller,
			Config:       telemetry.Config{ShutdownTimeout: time.Second},
		})
		app.Startup(context.Background())

		err := app.Quit()
		if err == nil || err.Error() != "drain failed" {
			t.Fatalf("expected stop failure to surface, got %v", err)
		}

		if !slices.Equal(recorder.calls, []string{"stop"}) {
			t.Fatalf("expected quit to stop after orchestrator failure, got %v", recorder.calls)
		}
	})
}

type fakeWindow struct {
	calls     []string
	ctxValues []any
	recorder  *callRecorder
}

func (window *fakeWindow) Show(ctx context.Context) {
	window.calls = append(window.calls, "show")
	window.ctxValues = append(window.ctxValues, ctx.Value(testContextKey("startup")))
	window.record("show")
}

func (window *fakeWindow) Hide(ctx context.Context) {
	window.calls = append(window.calls, "hide")
	window.ctxValues = append(window.ctxValues, ctx.Value(testContextKey("startup")))
	window.record("hide")
}

func (window *fakeWindow) Quit(ctx context.Context) {
	window.calls = append(window.calls, "quit")
	window.ctxValues = append(window.ctxValues, ctx.Value(testContextKey("startup")))
	window.record("quit")
}

func (window *fakeWindow) record(call string) {
	if window.recorder != nil {
		window.recorder.calls = append(window.recorder.calls, call)
	}
}

type fakeOrchestrator struct {
	state        orchestrator.State
	calls        []string
	recorder     *callRecorder
	stopErr      error
	stopDeadline time.Time
}

func (controller *fakeOrchestrator) Pause(context.Context) error {
	controller.calls = append(controller.calls, "pause")
	controller.state = orchestrator.StatePaused
	return nil
}

func (controller *fakeOrchestrator) Resume(context.Context) error {
	controller.calls = append(controller.calls, "resume")
	controller.state = orchestrator.StateRunning
	return nil
}

func (controller *fakeOrchestrator) Stop(ctx context.Context) error {
	controller.calls = append(controller.calls, "stop")
	if controller.recorder != nil {
		controller.recorder.calls = append(controller.recorder.calls, "stop")
	}
	controller.stopDeadline, _ = ctx.Deadline()
	if controller.stopErr != nil {
		return controller.stopErr
	}
	controller.state = orchestrator.StateStopped
	return nil
}

func (controller *fakeOrchestrator) State() orchestrator.State {
	return controller.state
}

type callRecorder struct {
	calls []string
}
