package browser

import "errors"

var (
	ErrChromeNotFound       = errors.New("chrome binary not found")
	ErrNotConnected         = errors.New("browser not connected")
	ErrNotLoggedIn          = errors.New("not logged in to Google")
	ErrTokenMissing         = errors.New("access token missing from page")
	ErrLabsTabUnavailable   = errors.New("labs.google tab not available")
	ErrAlreadyConnecting    = errors.New("already connecting")
)
