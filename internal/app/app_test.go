package app

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"dllm-network/internal/capture"
	"dllm-network/internal/dashboard"
	"dllm-network/internal/telemetry"
	"dllm-network/internal/telemetry/orchestrator"
	"dllm-network/internal/tray"
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

	if !slices.Equal(controller.calls, []string{"start", "pause", "resume"}) {
		t.Fatalf("expected pause and resume orchestration calls, got %v", controller.calls)
	}
}

func TestAppStartupStartsRuntimeController(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name          string
		controller    *fakeOrchestrator
		expectedCalls []string
	}{
		{
			name:          "startup starts runtime loop with startup context",
			controller:    &fakeOrchestrator{state: orchestrator.StateRunning},
			expectedCalls: []string{"start"},
		},
		{
			name: "startup failure stays hidden and remains deterministic",
			controller: &fakeOrchestrator{
				state:    orchestrator.StateStopped,
				startErr: errors.New("start failed"),
			},
			expectedCalls: []string{"start"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			window := &fakeWindow{}
			app := newTestApp(window, tt.controller, time.Second)

			startupCtx := context.WithValue(context.Background(), testContextKey("startup"), "ready")
			app.Startup(startupCtx)

			if !slices.Equal(tt.controller.calls, tt.expectedCalls) {
				t.Fatalf("expected startup calls %v, got %v", tt.expectedCalls, tt.controller.calls)
			}
			if len(tt.controller.ctxValues) != 1 || tt.controller.ctxValues[0] != "ready" {
				t.Fatalf("expected startup to forward startup context, got %v", tt.controller.ctxValues)
			}
			if got := app.Status(); got.WindowVisible {
				t.Fatalf("expected startup to keep the window hidden, got %+v", got)
			}
			if len(window.calls) != 0 {
				t.Fatalf("expected startup not to touch the window, got %v", window.calls)
			}
		})
	}
}

func TestAppQuitStopsCollectorsBeforeClosingWindow(t *testing.T) {
	t.Parallel()

	t.Run("clean quit drains orchestrator before quitting window", func(t *testing.T) {
		recorder := &callRecorder{}
		window := &fakeWindow{recorder: recorder}
		controller := &fakeOrchestrator{state: orchestrator.StateRunning, recorder: recorder}
		shutdownTimeout := 150 * time.Millisecond
		app := newTestApp(window, controller, shutdownTimeout)
		app.Startup(context.Background())

		before := time.Now()
		if err := app.Quit(); err != nil {
			t.Fatalf("quit: %v", err)
		}

		if !slices.Equal(recorder.calls, []string{"start", "stop", "quit"}) {
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
		app := newTestApp(window, controller, time.Second)
		app.Startup(context.Background())

		err := app.Quit()
		if err == nil || err.Error() != "drain failed" {
			t.Fatalf("expected stop failure to surface, got %v", err)
		}

		if !slices.Equal(recorder.calls, []string{"start", "stop"}) {
			t.Fatalf("expected quit to stop after orchestrator failure, got %v", recorder.calls)
		}
	})
}

func TestWailsEmitterEmitsDashboardSnapshotEvent(t *testing.T) {
	t.Parallel()

	called := 0
	var gotCtxValue any
	var gotEvent string
	var gotPayload []any
	emitter := wailsEmitter{
		emit: func(ctx context.Context, event string, payload ...any) {
			called++
			gotCtxValue = ctx.Value(testContextKey("startup"))
			gotEvent = event
			gotPayload = append([]any(nil), payload...)
		},
	}
	snapshot := dashboard.Snapshot{PublishedAt: time.Date(2026, time.June, 15, 1, 0, 0, 0, time.UTC)}
	ctx := context.WithValue(context.Background(), testContextKey("startup"), "ready")

	if err := emitter.Emit(ctx, dashboard.TopicDashboardSnapshot, snapshot); err != nil {
		t.Fatalf("emit: %v", err)
	}

	if called != 1 {
		t.Fatalf("expected one runtime event emission, got %d", called)
	}
	if gotCtxValue != "ready" {
		t.Fatalf("expected emitter to forward startup context, got %v", gotCtxValue)
	}
	if gotEvent != dashboard.TopicDashboardSnapshot {
		t.Fatalf("expected event %q, got %q", dashboard.TopicDashboardSnapshot, gotEvent)
	}
	if len(gotPayload) != 1 {
		t.Fatalf("expected one payload item, got %d", len(gotPayload))
	}
	payload, ok := gotPayload[0].(dashboard.Snapshot)
	if !ok {
		t.Fatalf("expected dashboard snapshot payload, got %T", gotPayload[0])
	}
	if !payload.PublishedAt.Equal(snapshot.PublishedAt) {
		t.Fatalf("expected publishedAt %s, got %s", snapshot.PublishedAt, payload.PublishedAt)
	}
}

func TestAppStartupStartsSystemTray(t *testing.T) {
	t.Parallel()

	window := &fakeWindow{}
	controller := &fakeOrchestrator{state: orchestrator.StateRunning}
	trayManager := &tray.MockTrayManager{}
	app := NewWithDependencies(Dependencies{
		Window:       window,
		Orchestrator: controller,
		TrayManager:  trayManager,
		Config:       telemetry.Config{ShutdownTimeout: time.Second},
	})

	app.Startup(context.Background())

	if !trayManager.Started {
		t.Fatal("expected startup to start the system tray")
	}
	if trayManager.StartConfig.Tooltip != tray.DefaultTooltip {
		t.Fatalf("expected tray tooltip %q, got %q", tray.DefaultTooltip, trayManager.StartConfig.Tooltip)
	}
	if len(trayManager.StartConfig.Icon) == 0 {
		t.Fatal("expected tray icon bytes to be configured")
	}
	if trayManager.StartConfig.OnOpen == nil || trayManager.StartConfig.OnExit == nil {
		t.Fatal("expected tray open and exit callbacks to be wired")
	}

	trayManager.StartConfig.OnOpen()

	if got := app.Status(); !got.WindowVisible {
		t.Fatalf("expected tray open callback to show the window, got %+v", got)
	}
	if !slices.Equal(window.calls, []string{"show"}) {
		t.Fatalf("expected tray open to call show, got %v", window.calls)
	}
}

func TestAppQuitStopsSystemTray(t *testing.T) {
	t.Parallel()

	trayManager := &tray.MockTrayManager{}
	controller := &fakeOrchestrator{state: orchestrator.StateRunning}
	app := NewWithDependencies(Dependencies{
		Window:       &fakeWindow{},
		Orchestrator: controller,
		TrayManager:  trayManager,
		Config:       telemetry.Config{ShutdownTimeout: time.Second},
	})
	app.Startup(context.Background())

	if err := app.Quit(); err != nil {
		t.Fatalf("quit: %v", err)
	}

	if !trayManager.Stopped {
		t.Fatal("expected quit to stop the system tray")
	}
}

func newTestApp(window *fakeWindow, controller *fakeOrchestrator, timeout time.Duration) *App {
	return NewWithDependencies(Dependencies{
		Window:       window,
		Orchestrator: controller,
		Config:       telemetry.Config{ShutdownTimeout: timeout},
		// Inject inert capture seams so these lifecycle tests never touch the
		// real WinDivert driver or the real Wails runtime: an exhausted fake
		// source emits nothing, and a noop emitter avoids EventsEmit panicking
		// without a live Wails context if any stray segment did arrive.
		CaptureSource: capture.NewFakeSource(nil),
		wailsEmitter:  wailsEmitter{emit: func(context.Context, string, ...any) {}},
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
	ctxValues    []any
	recorder     *callRecorder
	startErr     error
	stopErr      error
	stopDeadline time.Time
}

func (controller *fakeOrchestrator) Start(ctx context.Context) error {
	controller.calls = append(controller.calls, "start")
	controller.ctxValues = append(controller.ctxValues, ctx.Value(testContextKey("startup")))
	controller.record("start")
	if controller.startErr != nil {
		return controller.startErr
	}
	controller.state = orchestrator.StateRunning
	return nil
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
	controller.record("stop")
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

func (controller *fakeOrchestrator) record(call string) {
	if controller.recorder != nil {
		controller.recorder.calls = append(controller.recorder.calls, call)
	}
}

type callRecorder struct {
	calls []string
}
