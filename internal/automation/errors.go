package automation

import "errors"

var (
	ErrPromptEditorMissing  = errors.New("prompt editor not found")
	ErrCreateButtonNotFound = errors.New("create button not found below threshold")
	ErrSettingsButtonMissing = errors.New("settings (crop) button not found")
	ErrTabNotFound          = errors.New("settings tab not found for label")
	ErrModelMenuItemMissing = errors.New("model menu item not found")
)
