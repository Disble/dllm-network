package tray

import "sync"

type menuItem interface {
	Clicked() <-chan struct{}
}

// Package-level seams default to no-ops so the manager is fully testable and
// builds without the real tray dependency. The platform binding overrides them.
var (
	runWithExternalLoop func(onReady, onExit func()) = func(onReady, _ func()) {
		go onReady()
	}
	setIcon func([]byte) = func([]byte) {
		// no-op seam stub; real implementation provided by platform binding.
	}
	setTooltip func(string) = func(string) {
		// no-op seam stub; real implementation provided by platform binding.
	}
	addMenuItem func(string, string) menuItem = func(string, string) menuItem {
		// no-op seam stub; real implementation provided by platform binding.
		return nil
	}
	quit func() = func() {
		// no-op seam stub; real implementation provided by platform binding.
	}
)

// SystrayManager drives a background system tray icon with Open and Exit items.
type SystrayManager struct {
	mu       sync.Mutex
	stopOnce sync.Once
	started  bool
}

// NewSystrayManager creates a SystrayManager backed by the active platform seams.
func NewSystrayManager() *SystrayManager {
	return &SystrayManager{}
}

// Start launches the tray loop, wiring the configured icon, tooltip, and the
// Open/Exit menu callbacks.
func (m *SystrayManager) Start(config Config) error {
	m.mu.Lock()
	m.started = true
	m.stopOnce = sync.Once{}
	m.mu.Unlock()

	runWithExternalLoop(func() {
		setIcon(config.Icon)
		if config.Tooltip != "" {
			setTooltip(config.Tooltip)
		}

		openItem := addMenuItem("Abrir", "Abrir la ventana principal")
		exitItem := addMenuItem("Salir", "Salir de dllm-network")

		go listenMenuItem(openItem, config.OnOpen)
		go listenMenuItem(exitItem, config.OnExit)
	}, func() {})

	return nil
}

// Stop tears the tray down at most once.
func (m *SystrayManager) Stop() error {
	m.mu.Lock()
	started := m.started
	m.mu.Unlock()
	if !started {
		return nil
	}

	m.stopOnce.Do(func() {
		quit()
	})

	return nil
}

func listenMenuItem(item menuItem, callback func()) {
	if item == nil || callback == nil {
		return
	}

	for range item.Clicked() {
		callback()
	}
}
