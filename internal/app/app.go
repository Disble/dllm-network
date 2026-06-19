package app

import (
	"context"
	"log"
	"sync"
	"time"

	"ollama-telemetry/internal/activity"
	"ollama-telemetry/internal/capture"
	"ollama-telemetry/internal/dashboard"
	"ollama-telemetry/internal/events"
	"ollama-telemetry/internal/persistence"
	"ollama-telemetry/internal/store"
	"ollama-telemetry/internal/telemetry"
	"ollama-telemetry/internal/telemetry/inference"
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
	// PersistenceWriter is the durable-write sink (design D7) the capture
	// pipeline's completed inferences are batched into. When nil,
	// NewWithDependencies opens the production sqlite.Store at the default
	// config-dir path. Tests inject a fake Writer to exercise the async
	// write path without touching disk.
	PersistenceWriter persistence.Writer
	// CaptureEmitInterval bounds how often the capture pipeline's snapshot is
	// emitted to the frontend (conflated via coalescingProjector). Zero means
	// synchronous pass-through — the default for tests, which assert emissions
	// deterministically. The production entrypoint New() sets
	// defaultCaptureEmitInterval.
	CaptureEmitInterval time.Duration
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
	captureEmit   *coalescingProjector
	emitter       wailsEmitter
	bus           *events.Bus

	// persistenceWriter is the durable-write sink resolved at construction
	// time. Dependencies.PersistenceWriter (tests) is used verbatim — when
	// unset, it stays nil and Startup runs WITHOUT durable persistence
	// rather than silently opening a real file on disk. Only the production
	// entrypoint New() supplies the real sqlite.Store (deferred via
	// newProductionStore so opening the file happens lazily at Startup, not
	// at construction — mirroring the capture source's Open() happening in
	// pipeline.run(), not in newInferencePipeline).
	persistenceWriter  persistence.Writer
	useProductionStore bool
	persistence        *persistenceLifecycle

	// inferenceReader is the read-side handle backing the on-demand
	// InferenceDetail binding. It is the same durable store the persistence
	// writer wraps (the production *sqlite.Store implements both ports), set in
	// Startup once the writer is resolved. Nil when persistence is unavailable,
	// in which case the binding returns the zero value.
	inferenceReader store.InferenceReader
}

// newProductionStore is a package-level func-var seam mirroring
// newCaptureSource: production code points it at the real sqlite.Store
// opened at the default config-dir path. It is only ever invoked from
// Startup (lazily, never from the constructor) and only when New() (not
// NewWithDependencies) constructed the App, so unit tests that build an App
// via NewWithDependencies without a PersistenceWriter never touch disk.
var newProductionStore = openDefaultStore

// New creates the application binding used by Wails. Unlike
// NewWithDependencies, New wires the real production sqlite.Store as the
// persistence writer (opened lazily on Startup).
func New() *App {
	app := NewWithDependencies(Dependencies{
		TrayManager:         tray.NewSystrayManager(),
		CaptureEmitInterval: defaultCaptureEmitInterval,
	})
	app.useProductionStore = true
	return app
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

	// bus carries completed inferences from the capture pipeline to the
	// persistence subscriber (design D7). One bus per App, shared between
	// the pipeline's publish call and the subscriber's HandleEvent.
	bus := events.NewBus()

	// The capture pipeline publishes through the capturePublisher adapter, which
	// routes into the shared assembler and emits a COMPLETE merged Snapshot —
	// never an Inference-only partial that would clobber the Ollama/System data.
	// The capturePublisher is wrapped in a coalescingProjector so the capture
	// hot path is not stalled by per-segment JSON marshal + Wails emit (which
	// would back-pressure the WinDivert queue into dropping packets). Tests pass
	// CaptureEmitInterval=0 for synchronous, deterministic emission.
	captureEmit := newCoalescingProjector(capturePublisher{assembler}, deps.CaptureEmitInterval)
	pipeline := newInferencePipeline(src, recentStore, captureEmit, bus)

	return &App{
		window:            window,
		orchestrator:      orchestratorInstance,
		trayManager:       deps.TrayManager,
		config:            config,
		captureSource:     src,
		pipeline:          pipeline,
		captureEmit:       captureEmit,
		emitter:           emitter,
		bus:               bus,
		persistenceWriter: deps.PersistenceWriter,
	}
}

// Startup captures the Wails context and starts the runtime loop.
func (app *App) Startup(ctx context.Context) {
	app.mu.Lock()
	app.ctx = ctx
	app.visible = false
	trayManager := app.trayManager
	pipeline := app.pipeline
	captureEmit := app.captureEmit
	bus := app.bus
	writer := app.persistenceWriter
	useProductionStore := app.useProductionStore
	app.mu.Unlock()

	_ = app.orchestrator.Start(ctx)

	// Start the capture snapshot coalescer before the pipeline so the first
	// emitted state is forwarded on cadence. No-op in pass-through mode (tests).
	if captureEmit != nil {
		captureEmit.start(ctx)
	}

	// Start the capture pipeline. Graceful degradation: if the source is not
	// Active (unelevated, noop), the goroutine exits immediately without crash.
	if pipeline != nil {
		pipeline.run(ctx)
	}

	// Lazily open the real sqlite.Store — ONLY for apps constructed via
	// New() (the production Wails entrypoint). Apps built via
	// NewWithDependencies (every test in this package) never touch disk
	// unless the test explicitly injects Dependencies.PersistenceWriter.
	// Storage failures degrade gracefully: the app still runs, just without
	// durable persistence, rather than crashing on startup.
	if writer == nil && useProductionStore {
		store, err := newProductionStore()
		if err != nil {
			log.Printf("persistence: failed to open sqlite store, durable persistence disabled: %v", err)
		} else {
			writer = store
		}
	}

	// Expose a read-side handle for the on-demand inference-detail binding when
	// the resolved writer also supports reads (the production *sqlite.Store
	// does — it implements both ports). The GUI's single WAL connection reads
	// its own writes, so no second connection is needed.
	if reader, ok := writer.(store.InferenceReader); ok {
		app.mu.Lock()
		app.inferenceReader = reader
		app.mu.Unlock()
	}

	// Start the persistence subscriber's drain loop (design D7). Mirrors the
	// capture pipeline's run/stop lifecycle: started here, stopped in Quit.
	var persistenceLC *persistenceLifecycle
	if writer != nil {
		persistenceLC = newPersistenceLifecycle(writer)
		persistenceLC.start(ctx, bus)
	}
	app.mu.Lock()
	app.persistence = persistenceLC
	app.mu.Unlock()

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
	captureEmit := app.captureEmit
	persistenceLC := app.persistence
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

	// Stop the snapshot coalescer after the pipeline (no more Publish calls):
	// its final flush emits the last captured state before shutdown.
	if captureEmit != nil {
		captureEmit.stop()
	}

	// Stop the persistence subscriber after capture: flushes any remaining
	// buffered inferences (including ones from the pipeline.stop() above)
	// and closes the underlying store.
	if persistenceLC != nil {
		persistenceLC.stop()
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

// InferenceDetail returns the full stored inference record (request/response
// bodies and headers included) for id, fetched on demand from the durable
// store. The high-frequency dashboard snapshot ships only metadata for the
// recent list; the detail view calls this when a row is selected — mirroring
// how Chrome DevTools lazily loads a request's body rather than holding every
// body in memory. Returns the zero value (empty id) when persistence is
// unavailable or id is unknown; the frontend then falls back to the live
// snapshot event (which still carries the in-progress body).
func (app *App) InferenceDetail(id string) (inference.Inference, error) {
	app.mu.RLock()
	reader := app.inferenceReader
	ctx := app.operationContext()
	app.mu.RUnlock()

	if reader == nil {
		return inference.Inference{}, nil
	}

	inf, _, err := reader.Get(ctx, id)
	if err != nil {
		return inference.Inference{}, err
	}
	return inf, nil
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
