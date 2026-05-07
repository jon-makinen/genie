package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app, err := NewApp()
	if err != nil {
		log.Fatalf("init app: %v", err)
	}

	err = wails.Run(&options.App{
		Title:             "Genie",
		Width:             560,
		Height:            720,
		MinWidth:          480,
		MinHeight:         560,
		HideWindowOnClose: true,
		StartHidden:       true,
		AssetServer:       &assetserver.Options{Assets: assets},
		BackgroundColour:  &options.RGBA{R: 0, G: 0, B: 0, A: 255},
		OnStartup:         app.startup,
		OnShutdown:        app.shutdown,
		Bind:              []interface{}{app},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			WebviewIsTransparent: true,
			WindowIsTranslucent:  false,
			About: &mac.AboutInfo{
				Title:   "Genie",
				Message: "Local Whisper dictation in your menu bar.",
			},
		},
	})
	if err != nil {
		log.Fatalf("wails run: %v", err)
	}
}
