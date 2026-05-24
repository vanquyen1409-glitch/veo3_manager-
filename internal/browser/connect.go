package browser

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/stealth"
)

const labsURL = "https://labs.google/fx/vi/tools/flow"

// Ensure connects (reusing a running Chrome if possible), opens labs.google,
// and extracts a token. Idempotent — returns immediately if already ready.
func (s *Service) Ensure(ctx context.Context) (Snapshot, error) {
	s.mu.Lock()
	if s.connecting {
		s.mu.Unlock()
		return s.Snapshot(), ErrAlreadyConnecting
	}
	if s.status == StatusReady && s.browser != nil && s.page != nil {
		s.mu.Unlock()
		return s.Snapshot(), nil
	}
	s.connecting = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.connecting = false
		s.mu.Unlock()
	}()

	s.transition(StatusConnecting, "")

	cfg := s.cfgFn()
	slog.Info("browser.Ensure begin", "chromePath", cfg.ChromePath, "userDataDir", cfg.UserDataDir, "port", cfg.DebugPort)

	chromePath, err := detectChrome(cfg.ChromePath)
	if err != nil {
		slog.Error("detectChrome failed", "err", err)
		s.transition(StatusError, err.Error())
		return s.Snapshot(), err
	}
	slog.Info("chrome detected", "path", chromePath)

	port := cfg.DebugPort
	if port <= 0 {
		port = 9222
	}

	br, weOwn, pid, wsURL, err := s.connectOrLaunch(chromePath, cfg.UserDataDir, port)
	if err != nil {
		slog.Error("connectOrLaunch failed", "err", err)
		s.transition(StatusError, err.Error())
		return s.Snapshot(), err
	}
	slog.Info("chrome connected", "weOwn", weOwn, "pid", pid, "ws", wsURL)

	// Disable rod's default device emulation (LaptopWithMDPIScreen) BEFORE any
	// page is created. Otherwise rod pins every new page to a fixed ~1280px
	// viewport and the Flow UI renders small with black bars on the right.
	br.NoDefaultDevice()

	// Build a fresh cancel ctx for this connect cycle's stealth listener.
	// If a previous listener is still alive (shouldn't happen because Stop
	// cancels, but defensively), cancel it before installing the new one.
	stealthCtx, stealthCancel := context.WithCancel(context.Background())
	s.mu.Lock()
	if s.stealthCancel != nil {
		s.stealthCancel()
	}
	s.browser = br
	s.weOwn = weOwn
	s.pid = pid
	s.wsURL = wsURL
	s.connectedAt = time.Now()
	s.stealthCancel = stealthCancel
	s.mu.Unlock()

	// Apply browser-wide stealth + UA override BEFORE any page navigates.
	// Guards against Google's account login screen rejecting us with
	// "This browser or app may not be secure".
	if err := applyBrowserStealth(stealthCtx, br, realisticUA(chromePath)); err != nil {
		slog.Warn("applyBrowserStealth failed (continuing)", "err", err)
	}

	page, err := s.ensureLabsTab(br)
	if err != nil {
		slog.Error("ensureLabsTab failed", "err", err)
		s.transition(StatusError, err.Error())
		return s.Snapshot(), err
	}
	slog.Info("labs tab ready")

	// Maximize the window so the Flow/Veo UI fills the screen. Covers both the
	// fresh-launch and reuse paths (the launch flag only helps the former).
	if err := maximizeWindow(page); err != nil {
		slog.Warn("maximizeWindow failed (continuing)", "err", err)
	}

	if err := s.ensureProjectView(page); err != nil {
		slog.Warn("ensureProjectView failed (continuing)", "err", err)
	}

	s.mu.Lock()
	s.page = page
	s.mu.Unlock()

	tok, err := extractToken(page)
	if err != nil {
		slog.Warn("token extract", "err", err)
		if errors.Is(err, ErrTokenMissing) || errors.Is(err, ErrLabsTabUnavailable) {
			s.transition(StatusNeedsLogin, "đăng nhập Google trong cửa sổ Chrome")
			s.startLoginPoll()
			return s.Snapshot(), nil
		}
		s.transition(StatusError, err.Error())
		return s.Snapshot(), err
	}

	slog.Info("token captured", "len", len(tok))
	s.setToken(tok)
	s.transition(StatusReady, "")
	return s.Snapshot(), nil
}

func (s *Service) connectOrLaunch(chromePath, userDataDir string, port int) (*rod.Browser, bool, int, string, error) {
	if br, ws, err := tryReuse(port); err == nil {
		return br, false, 0, ws, nil
	}
	br, pid, ws, err := launchFresh(chromePath, userDataDir, port)
	if err != nil {
		return nil, false, 0, "", err
	}
	return br, true, pid, ws, nil
}

// ensureLabsTab finds an open labs.google tab or opens one. The returned page
// is wrapped with go-rod/stealth before navigation.
func (s *Service) ensureLabsTab(b *rod.Browser) (*rod.Page, error) {
	pages, err := b.Pages()
	if err == nil {
		for _, p := range pages {
			info, err := p.Info()
			if err != nil {
				continue
			}
			if strings.Contains(info.URL, "labs.google") {
				return p, nil
			}
		}
	}
	page, err := stealth.Page(b)
	if err != nil {
		return nil, fmt.Errorf("stealth page: %w", err)
	}
	if err := page.Navigate(labsURL); err != nil {
		return nil, fmt.Errorf("navigate labs: %w", err)
	}
	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("wait load: %w", err)
	}
	return page, nil
}
