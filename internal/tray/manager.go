package tray

// Config describes the system tray icon, tooltip, and the callbacks fired when
// the user opens the main window or asks the application to exit.
type Config struct {
	Icon    []byte
	Tooltip string
	OnOpen  func()
	OnExit  func()
}

// TrayManager owns the lifecycle of the background system tray.
type TrayManager interface {
	Start(Config) error
	Stop() error
}

// DefaultTooltip is shown when hovering the tray icon.
const DefaultTooltip = "dllm-network"
