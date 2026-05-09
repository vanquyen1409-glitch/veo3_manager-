package main

import (
	"fmt"

	"veo3-manager/internal/db"
)

// GetSettings returns the full settings bundle (with defaults applied).
func (a *App) GetSettings() (db.SettingsBundle, error) {
	if a.settings == nil {
		return db.SettingsBundle{}, fmt.Errorf("database not initialized")
	}
	return a.settings.GetBundle()
}

// SaveSettings persists the provided bundle.
func (a *App) SaveSettings(b db.SettingsBundle) error {
	if a.settings == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.settings.SetBundle(b)
}
