package browser

import "errors"

var (
	ErrChromeNotFound     = errors.New("chrome binary not found")
	ErrNotConnected       = errors.New("browser not connected")
	ErrNotLoggedIn        = errors.New("not logged in to Google")
	ErrTokenMissing       = errors.New("access token missing from page")
	ErrLabsTabUnavailable = errors.New("labs.google tab not available")
	ErrAlreadyConnecting  = errors.New("already connecting")
	// ErrUnauthorized is returned by PageFetch when labs.google answers 401.
	// The labsapi client detects it (via errors.Is) and retries once after
	// asking the browser service to re-extract the bearer token.
	ErrUnauthorized = errors.New("labs.google: unauthorized")
)
