package app

import (
	"context"
	"sync"

	"ollama-telemetry/internal/activity"
	"ollama-telemetry/internal/capture"
	"ollama-telemetry/internal/dashboard"
	"ollama-telemetry/internal/store"
	"ollama-telemetry/internal/telemetry"
	"ollama-telemetry/internal/telemetry/ollama"
	"ollama-telemetry/internal/telemetry/orchestrator"
	"ollama-telemetry/internal/tray"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// newCaptureSource is a package-level func-var seam that mirrors newTicker /
// newWailsEmitter. In production it returns the real WinDivert source; in
// tests it is overridden via Dependencies.CaptureSource injection.
var newCaptureSource = func() capture.CaptureSource {
	return newWinDivertCapture()
}

const (
	defaultOllamaBaseURL    = "http://127.0.0.1:11434"
	defaultRecentModelLimit = 12
	defaultRecentEventLimit = 12
)

type Window interface {
	Show(context.Context)
	Hide(context.Context)
	Quit(context.Context)
}

type Orchestrator interface {
	Start(context.Context) error
	Pause(context.Context) error
	Resume(context.Context) error
	Stop(context.Context) error
	State() orchestrator.State
}

type Dependencies struct {
	Window        Window
	Orchestrator  Orchestrator
	TrayManager   tray.TrayManager
	Config        telemetry.Config
	// CaptureSource is the packet-capture Strategy port. When nil,
	// NewWithDependencies falls back to the newCaptureSource func-var which
	// returns the real WinDivert source (or noop on non-windows). Tests inject
	// a fake source here to exercise the full pipeline without a driver.
	CaptureSource capture.CaptureSource
	// wailsEmitter overrides the default Wails runtime emitter. Used in tests
	// to intercept emitted snapshots without a live Wails runtime.
	wailsEmitter  wailsEmitter
}

type Status struct {
	WindowVisible   bool   `json:"windowVisible"`
	CollectionState string `json:"collectionState"`
}

// App owns the Wails lifecycle hooks needed for the runtime shell slice.
type App struct {
	mu            sync.RWMutex
	ctx           context.Context
	window        Window
	orchestrator  Orchestrator
	trayManager   tray.TrayManager
	config        telemetry.Config
	visible       bool
	captureSource capture.CaptureSource
	pipeline      *inferencePipeline
	emitter       wailsEmitter
}

// New creates the application binding used by Wails.
func New() *App {
	return NewWithDependencies(Dependencies{
		TrayManager: tray.NewSystrayManager(),
	})
}

func NewWithDependencies(deps Dependencies) *App {
	config := deps.Config.WithDefaults()

	// Resolve emitter: use injected override (tests) or production Wails emitter.
	emitter := deps.wailsEmitter
	if emitter.emit == nil {
		emitter = newWailsEmitter()
	}

	// Shared store for snapshot transitions and inference completions.
	// Created once and shared between the orchestrator publisher and the
	// capture pipeline so both write into the same bounded history.
	recentStore := store.NewRecent(defaultRecentModelLimit, defaultRecentEventLimit)

	// The shared assembler prevents the two emitter paths from clobbering each
	// other. Both the orchestrator tick publisher and the capture pipeline write
	// their partial state (Ollama/System vs Inference/Capture) into the assembler,
	// which merges and emits a COMPLETE Snapshot on every write.
	sharedPublisher := dashboard.NewPublisher(nil, recentStore, emitter)
	assembler := newSnapshotAssembler(sharedPublisher)

	orchestratorInstance := deps.Orchestrator
	if orchestratorInstance == nil {
		// Wire the orchestrator publisher directly to the assembler's OllamaSystem
		// path so that every tick emits a COMPLETE merged Snapshot, not just a
		// partial Ollama/System snapshot that would clobber the Inference data.
		runtimePublisher := newRuntimePublisherWithDependencies(
			activity.NewEngine(),
			recentStore,
			ollamaSystemPublisher{assembler},
		)
		poller := ollama.NewPoller(ollama.NewClient(defaultOllamaBaseURL, nil, nil), nil)
		orchestratorInstance = orchestrator.NewWithDependencies(config, orchestrator.Dependencies{
			Poller:    poller,
			Publisher: runtimePublisher,
		})
	}

	window := deps.Window
	if window == nil {
		window = wailsWindow{}
	}

	// Resolve capture source: use injected override or production source.
	src := deps.CaptureSource
	if src == nil {
		src = newCaptureSource()
	}

	// The capture pipeline publishes through the capturePublisher adapter, which
	// routes into the shared assembler and emits a COMPLETE merged Snapshot —
	// never an Inference-only partial that would clobber the Ollama/System data.
	pipeline := newInferencePipeline(src, recentStore, capturePublisher{assembler})

	return &App{
		window:        window,
		orchestrator:  orchestratorInstance,
		trayManager:   deps.TrayManager,
		config:        config,
		captureSource: src,
		pipeline:      pipeline,
		emitter:       emitter,
	}
}

// Startup captures the Wails context and starts the runtime loop.
func (app *App) Startup(ctx context.Context) {
	app.mu.Lock()
	app.ctx = ctx
	app.visible = false
	trayManager := app.trayManager
	pipeline := app.pipeline
	app.mu.Unlock()

	_ = app.orchestrator.Start(ctx)

	// Start the capture pipeline. Graceful degradation: if the source is not
	// Active (unelevated, noop), the goroutine exits immediately without crash.
	if pipeline != nil {
		pipeline.run(ctx)
	}

	if trayManager != nil {
		_ = trayManager.Start(tray.Config{
			Icon:    tray.DefaultIcon,
			Tooltip: tray.DefaultTooltip,
			OnOpen:  func() { _ = app.Show() },
			OnExit:  func() { _ = app.Quit() },
		})
	}
}

func (app *App) Show() error {
	app.mu.Lock()
	defer app.mu.Unlock()

	app.window.Show(app.operationContext())
	app.visible = true
	return nil
}

func (app *App) Hide() error {
	app.mu.Lock()
	defer app.mu.Unlock()

	app.window.Hide(app.operationContext())
	app.visible = false
	return nil
}

func (app *App) Pause() error {
	app.mu.RLock()
	ctx := app.operationContext()
	app.mu.RUnlock()

	return app.orchestrator.Pause(ctx)
}

func (app *App) Resume() error {
	app.mu.RLock()
	ctx := app.operationContext()
	app.mu.RUnlock()

	return app.orchestrator.Resume(ctx)
}

func (app *App) Quit() error {
	app.mu.RLock()
	ctx := app.operationContext()
	timeout := app.config.ShutdownTimeout
	pipeline := app.pipeline
	app.mu.RUnlock()

	shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := app.orchestrator.Stop(shutdownCtx); err != nil {
		return err
	}

	// Stop the capture pipeline after the orchestrator — capture goroutine is
	// cancelled and its context propagated through the shutdownCtx timeout.
	if pipeline != nil {
		pipeline.stop()
	}

	if app.trayManager != nil {
		_ = app.trayManager.Stop()
	}

	app.window.Quit(ctx)

	app.mu.Lock()
	app.visible = false
	app.mu.Unlock()

	return nil
}

func (app *App) Status() Status {
	app.mu.RLock()
	defer app.mu.RUnlock()

	return Status{
		WindowVisible:   app.visible,
		CollectionState: string(app.orchestrator.State()),
	}
}

// Health exposes a placeholder binding for later runtime slices.
func (app *App) Health() string {
	return "runtime-shell-ready"
}

func (app *App) operationContext() context.Context {
	if app.ctx != nil {
		return app.ctx
	}

	return context.Background()
}

type wailsWindow struct{}

func (wailsWindow) Show(ctx context.Context) {
	wruntime.Show(ctx)
}

func (wailsWindow) Hide(ctx context.Context) {
	wruntime.Hide(ctx)
}

func (wailsWindow) Quit(ctx context.Context) {
	wruntime.Quit(ctx)
}

// newRuntimePublisher creates the orchestrator snapshot publisher with a fresh
// recent store and emitter. Used in tests that need a standalone publisher
// without the shared snapshotAssembler.
func newRuntimePublisher(emitter dashboard.Emitter) orchestrator.SnapshotPublisher {
	recent := store.NewRecent(defaultRecentModelLimit, defaultRecentEventLimit)
	pub := newRuntimePublisherWithDependencies(activity.NewEngine(), recent, dashboard.NewPublisher(nil, recent, emitter))
	return pub
}
