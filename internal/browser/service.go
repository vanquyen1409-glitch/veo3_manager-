package browser

import (
	"context"
	"errors"
	"os/exec"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Config controls how the service connects to Chrome. All fields come from
// the SettingsBundle in the database.
type Config struct {
	ChromePath  string
	UserDataDir string
	DebugPort   int
}

// EmitFn is the runtime.EventsEmit signature; injected so the service can be
// tested without Wails.
type EmitFn func(ctx context.Context, event string, data ...interface{})

// Service owns the Chrome connection used by the queue worker.
type Service struct {
	mu sync.RWMutex

	wailsCtx context.Context
	emit     EmitFn

	cfgFn func() Config // re-read on every connect to pick up settings changes

	browser     *rod.Browser
	page        *rod.Page // signed-in labs.google tab
	weOwn       bool
	pid         int
	wsURL       string
	connectedAt time.Time

	stealthCancel context.CancelFunc // cancels the TargetCreated listener goroutine

	safeLoginCmd *exec.Cmd // running Safe Login Chrome (no CDP); nil when not active

	token   string
	tokenAt time.Time

	status     Status
	statusMsg  string
	connecting bool
	polling    bool // true while a background login poll is running
}

// New creates a Service. cfgFn supplies fresh settings on every connect.
func New(cfgFn func() Config) *Service {
	return &Service{
		cfgFn:  cfgFn,
		status: StatusDisconnected,
	}
}

// Bind wires the Wails runtime into the service. Call once after Wails is up.
func (s *Service) Bind(ctx context.Context, emit EmitFn) {
	s.mu.Lock()
	s.wailsCtx = ctx
	s.emit = emit
	s.mu.Unlock()
}

// Snapshot returns a copy of the current state for the UI.
func (s *Service) Snapshot() Snapshot {
	// cfgFn hits SettingsRepo (DB-backed). Calling it under RLock would
	// stall every concurrent reader on a slow DB query; resolve it first.
	cfg := s.cfgFn()
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap := Snapshot{
		Status:      s.status,
		Message:     s.statusMsg,
		ChromePath:  cfg.ChromePath,
		UserDataDir: cfg.UserDataDir,
		DebugPort:   cfg.DebugPort,
		WeOwn:       s.weOwn,
		WsURL:       s.wsURL,
		PID:         s.pid,
	}
	if !s.tokenAt.IsZero() {
		snap.TokenAgeMs = time.Since(s.tokenAt).Milliseconds()
	}
	return snap
}

// Refresh forces a token re-extraction (used by API client on 401).
func (s *Service) Refresh(ctx context.Context) error {
	s.mu.RLock()
	page := s.page
	s.mu.RUnlock()
	if page == nil {
		if _, err := s.Ensure(ctx); err != nil {
			return err
		}
		s.mu.RLock()
		page = s.page
		s.mu.RUnlock()
		if page == nil {
			return ErrNotConnected
		}
	}
	t, err := extractToken(page)
	if err != nil {
		return err
	}
	s.setToken(t)
	return nil
}

// Token returns a fresh access token, refreshing if older than ttl or empty.
// Returns ErrNotLoggedIn when the page does not yet contain credentials.
func (s *Service) Token(ctx context.Context) (string, error) {
	s.mu.RLock()
	page := s.page
	tok := s.token
	at := s.tokenAt
	s.mu.RUnlock()

	if page == nil {
		return "", ErrNotConnected
	}
	if tok != "" && time.Since(at) < 25*time.Minute {
		return tok, nil
	}
	t, err := extractToken(page)
	if err != nil {
		return "", err
	}
	s.setToken(t)
	return t, nil
}

// NewTab opens a new page in the same browser (used by the downloader).
func (s *Service) NewTab(ctx context.Context, url string) (*rod.Page, error) {
	s.mu.RLock()
	br := s.browser
	s.mu.RUnlock()
	if br == nil {
		return nil, ErrNotConnected
	}
	page, err := br.Page(proto.TargetCreateTarget{URL: url})
	if err != nil {
		return nil, err
	}
	return page, nil
}

// LabsPage returns the currently signed-in labs.google page, used by automation
// helpers (Phase 06).
func (s *Service) LabsPage() (*rod.Page, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.page == nil {
		return nil, ErrNotConnected
	}
	return s.page, nil
}

// Stop closes Chrome only if we launched it; otherwise it just detaches.
func (s *Service) Stop() {
	s.mu.Lock()
	br := s.browser
	weOwn := s.weOwn
	safe := s.safeLoginCmd
	stealthCancel := s.stealthCancel
	s.browser, s.page = nil, nil
	s.token = ""
	s.tokenAt = time.Time{}
	s.weOwn = false
	s.pid = 0
	s.safeLoginCmd = nil
	s.stealthCancel = nil
	s.mu.Unlock()

	// Cancel the TargetCreated listener BEFORE closing the browser so the
	// EachEvent goroutine exits cleanly instead of being left blocked on a
	// channel that the rod browser-close path is also waiting on.
	if stealthCancel != nil {
		stealthCancel()
	}

	if safe != nil && safe.Process != nil {
		_ = safe.Process.Kill()
	}
	if br == nil {
		return
	}
	if weOwn {
		_ = br.Close()
	}
	s.transition(StatusDisconnected, "")
}

// ResetProfile stops the browser then deletes the user-data-dir. Caller must
// reconnect afterwards. Refuses to run when we don't own the browser to avoid
// nuking another Chrome's profile while it's open.
func (s *Service) ResetProfile() error {
	// Snapshot ownership BEFORE Stop() — Stop unconditionally clears weOwn,
	// so without this read we'd lose the chance to refuse the wipe.
	s.mu.RLock()
	weOwn := s.weOwn
	connected := s.browser != nil
	s.mu.RUnlock()
	if connected && !weOwn {
		return errors.New("attached to external Chrome — close it first to avoid wiping its profile")
	}
	s.Stop()
	cfg := s.cfgFn()
	if cfg.UserDataDir == "" {
		return errors.New("userDataDir not set")
	}
	return removeAll(cfg.UserDataDir)
}

func (s *Service) setToken(t string) {
	s.mu.Lock()
	s.token = t
	s.tokenAt = time.Now()
	s.mu.Unlock()
}

func (s *Service) transition(st Status, msg string) {
	s.mu.Lock()
	s.status = st
	s.statusMsg = msg
	ctx := s.wailsCtx
	emit := s.emit
	s.mu.Unlock()
	if ctx != nil && emit != nil {
		emit(ctx, "browserStatus", s.Snapshot())
		return
	}
	if ctx != nil {
		wruntime.EventsEmit(ctx, "browserStatus", s.Snapshot())
	}
}
