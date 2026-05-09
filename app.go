package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"veo3-manager/internal/browser"
	"veo3-manager/internal/db"
	"veo3-manager/internal/download"
	"veo3-manager/internal/labsapi"
	"veo3-manager/internal/queue"
	"veo3-manager/internal/server"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails-bound singleton; later phases hang services off this struct.
type App struct {
	ctx     context.Context
	version string

	appDataDir string
	outputDir  string

	sqlDB      *sql.DB
	tasks      *db.TaskRepo
	settings   *db.SettingsRepo
	browser    *browser.Service
	api        *labsapi.Client
	downloader *download.Downloader
	worker     *queue.Worker
	localFiles *server.LocalFileHandler
}

func NewApp(version string, localFiles *server.LocalFileHandler) *App {
	return &App{version: version, localFiles: localFiles}
}

// Startup is called once after the runtime is up. Initialize services here.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	if err := a.initPaths(); err != nil {
		log.Printf("init paths: %v", err)
		return
	}

	sqlDB, err := db.Open(a.appDataDir)
	if err != nil {
		log.Printf("db open: %v", err)
		return
	}
	a.sqlDB = sqlDB
	a.tasks = db.NewTaskRepo(sqlDB)
	a.settings = db.NewSettingsRepo(sqlDB, db.DefaultDirs{
		UserDataDir: filepath.Join(a.appDataDir, "chromedata"),
		OutputDir:   a.outputDir,
	})

	n, err := a.tasks.ResetStaleRunning()
	if err != nil {
		log.Printf("reset stale running: %v", err)
	} else if n > 0 {
		log.Printf("reset %d stale running tasks -> pending", n)
	}

	a.registerLocalFileRoots()

	a.browser = browser.New(func() browser.Config {
		b, err := a.settings.GetBundle()
		if err != nil {
			return browser.Config{}
		}
		return browser.Config{
			ChromePath:  b.ChromePath,
			UserDataDir: b.UserDataDir,
			DebugPort:   b.ChromeDebugPort,
		}
	})
	a.browser.Bind(ctx, func(c context.Context, ev string, data ...interface{}) {
		wruntime.EventsEmit(c, ev, data...)
	})

	a.api = labsapi.NewClient(a.browser, nil)
	a.downloader = download.New(a.browser)

	a.worker = queue.New(queue.Deps{
		Tasks:    a.tasks,
		Settings: a.settings,
		Browser:  a.browser,
		API:      a.api,
		Download: a.downloader,
	})
	a.worker.Bind(ctx)

	// Background browser warm-up: kick off Ensure() right after Startup so
	// Chrome connects + the labs.google tab loads while the user is still
	// looking at the splash. By the time they click "Submit" on a prompt,
	// the worker can skip the cold-connect path (~2-3 s saved on first task).
	//
	// We run this in a goroutine so a flaky Chrome env doesn't block the
	// UI from coming up. Errors are logged-only - the user can still
	// trigger Ensure() manually from the connect button.
	go func() {
		if _, err := a.browser.Ensure(ctx); err != nil {
			log.Printf("browser warm-up: %v", err)
		}
	}()
}

func (a *App) DOMReady(ctx context.Context) {}

func (a *App) BeforeClose(ctx context.Context) bool { return false }

func (a *App) Shutdown(ctx context.Context) {
	if a.worker != nil {
		a.worker.Stop()
	}
	if a.browser != nil {
		a.browser.Stop()
	}
	if a.sqlDB != nil {
		// Fold the WAL back into the main DB so we don't leave a 100MB+
		// queue.db-wal sidecar. Best-effort; closes regardless.
		db.CheckpointOnClose(a.sqlDB)
		_ = a.sqlDB.Close()
	}
}

// initPaths picks platform-correct dirs for app data + video output:
//
//	Windows: %APPDATA%\veo3-manager       , %USERPROFILE%\Videos\VEO3
//	macOS:   ~/Library/Application Support/veo3-manager , ~/Movies/VEO3
//	Linux:   ~/.config/veo3-manager       , ~/Videos/VEO3
//
// os.UserConfigDir / os.UserHomeDir handle the platform mapping and
// fall back gracefully if env vars are missing.
func (a *App) initPaths() error {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("user config dir: %w", err)
	}
	a.appDataDir = filepath.Join(cfgDir, "veo3-manager")
	if err := os.MkdirAll(a.appDataDir, 0o755); err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		// Fall back to the config dir's parent so we still have a writable
		// location instead of '' which would land in cwd.
		home = filepath.Dir(a.appDataDir)
	}
	// macOS keeps videos under ~/Movies, the rest of the world under ~/Videos.
	videosBase := "Videos"
	if runtime.GOOS == "darwin" {
		videosBase = "Movies"
	}
	a.outputDir = filepath.Join(home, videosBase, "VEO3")
	if err := os.MkdirAll(a.outputDir, 0o755); err != nil {
		return err
	}
	// Writability probe: catch permission/quota issues at startup, not on
	// the first task. Failed probe is logged but non-fatal so the user can
	// still open Settings and pick a different output dir.
	if err := probeWritable(a.outputDir); err != nil {
		log.Printf("output dir not writable (%s): %v - reconfigure in Settings", a.outputDir, err)
	}
	return nil
}

// probeWritable creates and removes a tiny sentinel file. Cheaper than a real
// download attempt and surfaces the same EACCES / read-only errors.
func probeWritable(dir string) error {
	probe := filepath.Join(dir, ".veo3-write-probe")
	f, err := os.Create(probe)
	if err != nil {
		return err
	}
	_ = f.Close()
	return os.Remove(probe)
}

// registerLocalFileRoots adds the default output dir plus the user-configured
// output dirs so the localfile asset server may serve them.
func (a *App) registerLocalFileRoots() {
	if a.localFiles == nil {
		return
	}
	a.localFiles.RegisterRoot(a.outputDir)
	b, err := a.settings.GetBundle()
	if err != nil {
		return
	}
	a.localFiles.RegisterRoot(b.OutputDir)
	if b.DefaultConfig.OutputDir == "" {
		return
	}
	a.localFiles.RegisterRoot(b.DefaultConfig.OutputDir)
}

// Version returns the embedded build version.
func (a *App) Version() string { return a.version }

// AppDataDir returns the directory where DB and Chrome profile live.
func (a *App) AppDataDir() string { return a.appDataDir }
