package main

import (
	"fmt"

	"veo3-manager/internal/browser"
)

// EnsureBrowser connects (reusing or launching Chrome) and returns the
// resulting status snapshot. Safe to call repeatedly.
func (a *App) EnsureBrowser() (browser.Snapshot, error) {
	if a.browser == nil {
		return browser.Snapshot{Status: browser.StatusError, Message: "browser service not initialized"}, fmt.Errorf("browser not initialized")
	}
	return a.browser.Ensure(a.ctx)
}

// GetBrowserStatus returns the current snapshot without performing a connect.
func (a *App) GetBrowserStatus() browser.Snapshot {
	if a.browser == nil {
		return browser.Snapshot{Status: browser.StatusDisconnected}
	}
	return a.browser.Snapshot()
}

// GetBrowserDetail returns the heavy info payload (profile size, UA, Chrome
// version, email, WS URL, anti-bot flags). Used by the "Chrome chi tiết"
// card on the Settings page. Safe to call when not connected.
func (a *App) GetBrowserDetail() browser.Detail {
	if a.browser == nil {
		return browser.Detail{
			Snapshot:       browser.Snapshot{Status: browser.StatusDisconnected},
			StealthEnabled: true,
			BypassFlags:    browser.BypassFlags,
			StealthPatches: browser.StealthPatches,
		}
	}
	return a.browser.Detail()
}

// OpenLoginFlow brings Chrome to the front and polls for token presence.
// Blocks up to 5 minutes.
func (a *App) OpenLoginFlow() error {
	if a.browser == nil {
		return fmt.Errorf("browser not initialized")
	}
	return a.browser.OpenLoginFlow(a.ctx)
}

// OpenSafeLogin spawns a vanilla Chrome (no CDP) so Google's account login
// page does not detect automation. Use this when normal login fails with
// "This browser or app may not be secure". The user signs in, closes Chrome,
// and the app reconnects automatically using the saved cookies.
func (a *App) OpenSafeLogin() error {
	if a.browser == nil {
		return fmt.Errorf("browser not initialized")
	}
	return a.browser.OpenSafeLogin(a.ctx)
}

// RefreshToken forces a token re-extraction from the active labs.google page.
func (a *App) RefreshToken() error {
	if a.browser == nil {
		return fmt.Errorf("browser not initialized")
	}
	_, err := a.browser.Token(a.ctx)
	return err
}

// ResetBrowserProfile stops Chrome and deletes the persistent profile dir.
// Caller must reconnect to log in again.
func (a *App) ResetBrowserProfile() error {
	if a.browser == nil {
		return fmt.Errorf("browser not initialized")
	}
	return a.browser.ResetProfile()
}
