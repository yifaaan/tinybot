//go:build desktop

package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	desktopApp := NewDesktopApp("")

	err := wails.Run(&options.App{
		Title:            "tinybot Desktop",
		Width:            1440,
		Height:           960,
		MinWidth:         1180,
		MinHeight:        760,
		BackgroundColour: &options.RGBA{R: 12, G: 18, B: 28, A: 1},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: desktopApp.startup,
		Bind: []any{
			desktopApp,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
