package app

import "context"

// App owns the Wails lifecycle hooks needed for the scaffold slice.
type App struct {
	ctx context.Context
}

// New creates the application binding used by Wails.
func New() *App {
	return &App{}
}

// Startup captures the Wails context for later tray and runtime work.
func (app *App) Startup(ctx context.Context) {
	app.ctx = ctx
}

// Health exposes a placeholder binding for later runtime slices.
func (app *App) Health() string {
	return "scaffold-ready"
}
