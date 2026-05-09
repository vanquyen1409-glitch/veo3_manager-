package main

import (
	"embed"
	"errors"
	"log/slog"
	"os"
	"path/filepath"

	"veo3-manager/internal/app"
	"veo3-manager/internal/server"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

// version is overridden at build time via -ldflags "-X main.version=…".
var version = "dev"

func main() {
	// 1. Single-instance lock — refuse to run twice (avoids fighting over
	// SQLite + Chrome debug port).
	release, err := app.AcquireSingleInstance("VEO3-Manager")
	if err != nil {
		if errors.Is(err, app.ErrAlreadyRunning) {
			os.Exit(0)
		}
		slog.Error("single-instance lock failed", "err", err)
	} else {
		defer release()
	}

	// 2. Logger to stdout + rotating file in the OS-native config dir:
	//    Windows %APPDATA%\veo3-manager, macOS ~/Library/Application Support/veo3-manager,
	//    Linux ~/.config/veo3-manager. UserConfigDir handles all three.
	cfgDir, cfgErr := os.UserConfigDir()
	if cfgErr != nil {
		// Last-resort fallback: cwd. App startup will retry path init with the
		// same logic and surface a clearer error to the UI if this also fails.
		cfgDir = "."
	}
	app.InitLogger(filepath.Join(cfgDir, "veo3-manager"))

	localFiles := server.NewLocalFileHandler()
	bound := NewApp(version, localFiles)

	if err := wails.Run(&options.App{
		Title:            "VEO3 Manager",
		Width:            1280,
		Height:           800,
		MinWidth:         1024,
		MinHeight:        640,
		Frameless:        true,
		WindowStartState: options.Maximised,
		BackgroundColour: &options.RGBA{
			R: 2, G: 6, B: 23, A: 1, // slate-950
		},
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: assetserver.Middleware(server.Middleware(localFiles)),
		},
		OnStartup:     bound.Startup,
		OnDomReady:    bound.DOMReady,
		OnBeforeClose: bound.BeforeClose,
		OnShutdown:    bound.Shutdown,
		Bind: []interface{}{
			bound,
		},
		Windows: &windows.Options{
			WebviewIsTransparent:              false,
			WindowIsTranslucent:               false,
			DisableFramelessWindowDecorations: false,
			DisableWindowIcon:                 false,
		},
	}); err != nil {
		slog.Error("wails run", "err", err)
		os.Exit(1)
	}
}
