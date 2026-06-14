package app

import (
	"context"
	"sync"

	"ollama-telemetry/internal/telemetry"
	"ollama-telemetry/internal/telemetry/orchestrator"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type Window interface {
	Show(context.Context)
	Hide(context.Context)
	Quit(context.Context)
}

type Orchestrator interface {
	Pause(context.Context) error
	Resume(context.Context) error
	Stop(context.Context) error
	State() orchestrator.State
}

type Dependencies struct {
	Window       Window
	Orchestrator Orchestrator
	Config       telemetry.Config
}

type Status struct {
	WindowVisible   bool   `json:"windowVisible"`
	CollectionState string `json:"collectionState"`
}

// App owns the Wails lifecycle hooks needed for the runtime shell slice.
type App struct {
	mu           sync.RWMutex
	ctx          context.Context
	window       Window
	orchestrator Orchestrator
	config       telemetry.Config
	visible      bool
}

// New creates the application binding used by Wails.
func New() *App {
	return NewWithDependencies(Dependencies{})
}

func NewWithDependencies(deps Dependencies) *App {
	config := deps.Config.WithDefaults()
	orchestratorInstance := deps.Orchestrator
	if orchestratorInstance == nil {
		orchestratorInstance = orchestrator.New(config)
	}

	window := deps.Window
	if window == nil {
		window = wailsWindow{}
	}

	return &App{
		window:       window,
		orchestrator: orchestratorInstance,
		config:       config,
	}
}

// Startup captures the Wails context for later tray and runtime work.
func (app *App) Startup(ctx context.Context) {
	app.mu.Lock()
	defer app.mu.Unlock()
	app.ctx = ctx
	app.visible = false
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
	app.mu.RUnlock()

	shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := app.orchestrator.Stop(shutdownCtx); err != nil {
		return err
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
