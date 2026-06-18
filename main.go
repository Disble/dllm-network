package main

import (
	"embed"
	"log"

	appcore "ollama-telemetry/internal/app"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := appcore.New()

	err := wails.Run(&options.App{
		Title:             "Ollama Telemetry",
		Width:             1280,
		Height:            800,
		StartHidden:       true,
		HideWindowOnClose: true,
		// Frameless: the custom in-app title bar (TitleBar component) renders the
		// brand + window controls, matching the dashboard reference design.
		Frameless: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 8, G: 9, B: 11, A: 1},
		OnStartup:        app.Startup,
		Bind: []any{
			app,
		},
	})

	if err != nil {
		log.Printf("wails startup error: %v", err)
	}
}
