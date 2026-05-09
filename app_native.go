package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ChooseDirectory opens the Wails native folder picker; returns "" on cancel.
func (a *App) ChooseDirectory(currentPath string) (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("runtime not ready")
	}
	dir, err := wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:                "Choose folder",
		DefaultDirectory:     currentPath,
		CanCreateDirectories: true,
	})
	if err != nil {
		return "", err
	}
	if dir != "" && a.localFiles != nil {
		a.localFiles.RegisterRoot(dir)
	}
	return dir, nil
}

// OpenInExplorer opens the parent folder of path in Windows Explorer with the
// file pre-selected. Falls back to opening the dir if /select fails.
func (a *App) OpenInExplorer(p string) error {
	if p == "" {
		return fmt.Errorf("empty path")
	}
	cmd := exec.Command("explorer", "/select,", p)
	return cmd.Start()
}

// OpenLogsFolder opens the application data dir (where logs land in Phase 12).
func (a *App) OpenLogsFolder() error {
	target := filepath.Join(a.appDataDir, "logs")
	_ = os.MkdirAll(target, 0o755)
	cmd := exec.Command("explorer", target)
	return cmd.Start()
}

// OpenProfileFolder opens the persistent Chrome profile dir in Explorer.
// Useful when the user wants to inspect cookies / extensions manually.
func (a *App) OpenProfileFolder() error {
	if a.settings == nil {
		return fmt.Errorf("settings not initialized")
	}
	b, err := a.settings.GetBundle()
	if err != nil {
		return err
	}
	dir := b.UserDataDir
	if dir == "" {
		dir = filepath.Join(a.appDataDir, "chromedata")
	}
	_ = os.MkdirAll(dir, 0o755)
	cmd := exec.Command("explorer", dir)
	return cmd.Start()
}

