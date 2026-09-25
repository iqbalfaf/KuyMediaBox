package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"kuymediabox/internal/config"
	"kuymediabox/internal/pdf"
	"kuymediabox/internal/platform"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	// Window colour before the page paints, matching the saved theme.
	bg := &options.RGBA{R: 14, G: 16, B: 20, A: 255}
	theme := windows.Dark
	if t := app.cfg.Get().Theme; t == config.ThemeLight || (t == config.ThemeSystem && platform.SystemLightTheme()) {
		bg = &options.RGBA{R: 243, G: 244, B: 247, A: 255}
		theme = windows.Light
	}
	err := wails.Run(&options.App{
		Title:            "KuyMediaBox",
		Width:            1280,
		Height:           800,
		MinWidth:         1100,
		MinHeight:        700,
		Frameless:        true,
		BackgroundColour: bg,
		AssetServer: &assetserver.Options{
			Assets: assets,
			// Page and image previews for the PDF tools (/kmb/page, /kmb/img).
			Middleware: pdf.Middleware,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		// File drops are handled by the Wails runtime (OnFileDrop), which also blocks the
		// WebView's own navigation. DisableWebViewDrop must stay off: on WebView2 runtimes that
		// support it, it blocks external drops completely and file drag & drop stops working.
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "b7d3f7a4-6b0e-4d6b-9a55-kuymediabox",
			OnSecondInstanceLaunch: app.onSecondInstance,
		},
		Bind: []interface{}{app},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			Theme:                theme,
			DisableWindowIcon:    false,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
