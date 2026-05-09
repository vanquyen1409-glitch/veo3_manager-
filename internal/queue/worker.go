package queue

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"veo3-manager/internal/db"
	"veo3-manager/internal/download"
	"veo3-manager/internal/labsapi"
)

// APIPort is the subset of *labsapi.Client the worker needs. Defining it as
// a local interface lets tests inject a fake API without touching real
// labs.google. *labsapi.Client satisfies it without any code changes.
type APIPort interface {
	Submit(ctx context.Context, prompt string, cfg labsapi.SubmitConfig) ([]labsapi.MediaRef, error)
	Wait(ctx context.Context, refs []labsapi.MediaRef, opts labsapi.PollOptions) ([]labsapi.FinalMedia, error)
}

// BrowserPort is the subset of *browser.Service the worker needs.
// *browser.Service satisfies it without any code changes.
type BrowserPort interface {
	EnsureProjectView() error
}

// DownloadPort is the subset of *download.Downloader the worker needs.
// *download.Downloader satisfies it without any code changes.
type DownloadPort interface {
	Fetch(ctx context.Context, redirectURL, dstPath string, onProgress download.ProgressFn) error
}

// Deps bundles the services the worker orchestrates.
type Deps struct {
	Tasks    *db.TaskRepo
	Settings *db.SettingsRepo
	Browser  BrowserPort
	API      APIPort
	Download DownloadPort
	Logger   *slog.Logger
}

// Worker is a single-goroutine task processor with pause/resume/stop.
type Worker struct {
	deps Deps

	wailsCtx context.Context

	mu        sync.Mutex
	state     State
	paused    bool
	runCtx    context.Context
	cancel    context.CancelFunc
	resumeCh  chan struct{}
	stoppedCh chan struct{}
}

func New(d Deps) *Worker {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Worker{deps: d, state: StateIdle}
}

// Bind injects the Wails ctx for emitting events.
func (w *Worker) Bind(ctx context.Context) {
	w.mu.Lock()
	w.wailsCtx = ctx
	w.mu.Unlock()
}

// State returns a snapshot of the current state.
func (w *Worker) State() State {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.state
}

// Start launches the loop goroutine if not already running.
func (w *Worker) Start(parent context.Context) error {
	w.mu.Lock()
	if w.state == StateRunning || w.state == StatePaused {
		w.mu.Unlock()
		return nil
	}
	if w.state == StateStopping {
		w.mu.Unlock()
		return errors.New("worker stopping; try again shortly")
	}
	w.runCtx, w.cancel = context.WithCancel(parent)
	w.resumeCh = make(chan struct{}, 1)
	w.stoppedCh = make(chan struct{})
	w.state = StateRunning
	w.paused = false
	w.mu.Unlock()

	w.emitState("")
	go w.loop()
	return nil
}

// Stop cancels the run context. Idempotent.
func (w *Worker) Stop() {
	w.mu.Lock()
	if w.state == StateIdle {
		w.mu.Unlock()
		return
	}
	w.state = StateStopping
	cancel := w.cancel
	stoppedCh := w.stoppedCh
	// Wake up any pause-wait so it can observe the cancel.
	if w.resumeCh != nil {
		select {
		case w.resumeCh <- struct{}{}:
		default:
		}
	}
	w.mu.Unlock()
	w.emitState("")

	if cancel != nil {
		cancel()
	}
	if stoppedCh != nil {
		<-stoppedCh
	}
}

// Pause toggles into paused state at the next safe point.
func (w *Worker) Pause() {
	w.mu.Lock()
	if w.state != StateRunning {
		w.mu.Unlock()
		return
	}
	w.state = StatePaused
	w.paused = true
	w.mu.Unlock()
	w.emitState("")
}

// Resume returns from paused → running.
func (w *Worker) Resume() {
	w.mu.Lock()
	if w.state != StatePaused {
		w.mu.Unlock()
		return
	}
	w.state = StateRunning
	w.paused = false
	ch := w.resumeCh
	w.mu.Unlock()
	if ch != nil {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	w.emitState("")
}
