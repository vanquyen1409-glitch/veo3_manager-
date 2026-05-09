package browser

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

// OpenSafeLogin launches a vanilla Chrome (NO --remote-debugging-port and NO
// automation flags) pointed at accounts.google.com. The user signs in to
// Gmail in this window, closes Chrome, and the cookies persist in the
// shared user-data-dir. The next normal CDP connection then sees a
// pre-authenticated session and skips Google's "browser may not be secure"
// gate entirely.
//
// We close any existing CDP-driven Chrome we own first to avoid two Chrome
// processes fighting over the same profile dir.
func (s *Service) OpenSafeLogin(ctx context.Context) error {
	cfg := s.cfgFn()
	chromePath, err := detectChrome(cfg.ChromePath)
	if err != nil {
		return err
	}
	userDataDir := cfg.UserDataDir
	if userDataDir == "" {
		return fmt.Errorf("userDataDir not configured")
	}

	// Drop any existing CDP-driven Chrome we own — only one Chrome process
	// may hold the profile lock.
	s.mu.Lock()
	if s.weOwn && s.browser != nil {
		_ = s.browser.Close()
	}
	s.browser = nil
	s.page = nil
	s.weOwn = false
	s.pid = 0
	s.wsURL = ""
	s.token = ""
	s.tokenAt = time.Time{}
	prev := s.safeLoginCmd
	s.safeLoginCmd = nil
	s.mu.Unlock()

	if prev != nil && prev.Process != nil {
		_ = prev.Process.Kill()
	}

	cmd, err := launchSafeLogin(chromePath, userDataDir, "https://accounts.google.com/")
	if err != nil {
		s.transition(StatusError, err.Error())
		return err
	}
	// Capture the long-lived background ctx (set by Bind) under the same lock
	// that writes safeLoginCmd. Reading s.wailsCtx inside the goroutine without
	// the lock is a data race vs Bind. Fall back to context.Background() if
	// Bind has not run yet (e.g. early shutdown scenarios).
	s.mu.Lock()
	s.safeLoginCmd = cmd
	bgCtx := s.wailsCtx
	s.mu.Unlock()
	if bgCtx == nil {
		bgCtx = context.Background()
	}
	s.transition(StatusNeedsLogin, "đăng nhập Gmail trong cửa sổ Chrome an toàn")

	// Watch the process: when user closes the window, auto-reconnect via CDP
	// so the UI flips to "ready" using the cookies just saved.
	go func(c *exec.Cmd, ctx context.Context) {
		_ = c.Wait()
		slog.Info("safe-login Chrome closed, auto-reconnecting via CDP")
		time.Sleep(800 * time.Millisecond)
		s.mu.Lock()
		s.safeLoginCmd = nil
		s.mu.Unlock()
		if _, err := s.Ensure(ctx); err != nil {
			slog.Warn("auto-reconnect after safe-login failed", "err", err)
		}
	}(cmd, bgCtx)

	return nil
}

// OpenLoginFlow ensures Chrome is up, brings the labs.google tab to the
// front and starts a background goroutine that polls for token presence.
// Returns immediately so the UI does not block.
func (s *Service) OpenLoginFlow(ctx context.Context) error {
	if _, err := s.Ensure(ctx); err != nil {
		return err
	}
	s.mu.RLock()
	page := s.page
	s.mu.RUnlock()
	if page == nil {
		return ErrNotConnected
	}
	bring := proto.PageBringToFront{}
	_ = bring.Call(page)
	s.transition(StatusNeedsLogin, "đăng nhập trong cửa sổ Chrome")
	s.startLoginPoll()
	return nil
}

// startLoginPoll runs a background poller that flips status to ready as soon
// as a token can be extracted. Idempotent — second call while one is already
// running is a no-op.
func (s *Service) startLoginPoll() {
	s.mu.Lock()
	if s.polling {
		s.mu.Unlock()
		return
	}
	s.polling = true
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			s.polling = false
			s.mu.Unlock()
		}()

		deadline := time.Now().Add(5 * time.Minute)
		for time.Now().Before(deadline) {
			s.mu.RLock()
			page := s.page
			s.mu.RUnlock()
			if page == nil {
				return
			}
			tok, err := extractToken(page)
			if err == nil && tok != "" {
				s.setToken(tok)
				s.transition(StatusReady, "")
				return
			}
			time.Sleep(3 * time.Second)
		}
	}()
}
