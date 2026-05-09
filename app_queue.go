package main

import (
	"fmt"

	"veo3-manager/internal/queue"
)

// StartQueue launches the background worker if not already running.
// Connects the browser first so the API has a token.
func (a *App) StartQueue() error {
	if a.worker == nil {
		return fmt.Errorf("worker not initialized")
	}
	if a.browser != nil {
		if _, err := a.browser.Ensure(a.ctx); err != nil {
			return fmt.Errorf("browser: %w", err)
		}
	}
	return a.worker.Start(a.ctx)
}

// PauseQueue halts the worker before its next pickup.
func (a *App) PauseQueue() {
	if a.worker != nil {
		a.worker.Pause()
	}
}

// ResumeQueue continues a paused worker.
func (a *App) ResumeQueue() {
	if a.worker != nil {
		a.worker.Resume()
	}
}

// StopQueue cancels the worker context; in-flight task is marked cancelled.
func (a *App) StopQueue() {
	if a.worker != nil {
		a.worker.Stop()
	}
}

// GetQueueState returns the current worker state as a string
// ("idle"|"running"|"paused"|"stopping"). Returned as string so Wails
// generates a clean TS signature.
func (a *App) GetQueueState() string {
	if a.worker == nil {
		return string(queue.StateIdle)
	}
	return string(a.worker.State())
}
