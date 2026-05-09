package queue

import (
	"fmt"
	"time"
)

func (w *Worker) loop() {
	defer func() {
		if r := recover(); r != nil {
			w.deps.Logger.Error("queue worker panic", "panic", r)
			w.emitTask("worker", "failed", map[string]any{"error": fmt.Sprintf("worker panic: %v", r)})
		}
		w.mu.Lock()
		w.state = StateIdle
		stoppedCh := w.stoppedCh
		w.stoppedCh = nil
		w.cancel = nil
		w.runCtx = nil
		w.mu.Unlock()
		w.emitState("")
		if stoppedCh != nil {
			close(stoppedCh)
		}
	}()

	for {
		if err := w.runCtx.Err(); err != nil {
			return
		}
		if w.waitIfPaused() {
			return
		}
		task, err := w.deps.Tasks.NextPending()
		if err != nil {
			w.deps.Logger.Error("next pending", "err", err)
			if w.sleepOrCancel(2 * time.Second) {
				return
			}
			continue
		}
		if task == nil {
			if w.sleepOrCancel(2 * time.Second) {
				return
			}
			continue
		}

		bundle, _ := w.deps.Settings.GetBundle()
		w.runTask(w.runCtx, task, bundle)
		w.emitStats()
	}
}

// waitIfPaused blocks until resumed; returns true if cancelled while paused.
func (w *Worker) waitIfPaused() bool {
	for {
		w.mu.Lock()
		paused := w.paused
		ch := w.resumeCh
		ctx := w.runCtx
		w.mu.Unlock()
		if !paused {
			return false
		}
		select {
		case <-ctx.Done():
			return true
		case <-ch:
			// loop back to recheck paused (Resume cleared the flag)
		}
	}
}

// sleepOrCancel sleeps d or returns true if ctx done.
func (w *Worker) sleepOrCancel(d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-w.runCtx.Done():
		return true
	case <-t.C:
		return false
	}
}
