package queue

import (
	"fmt"
	"time"
)

// Idle-poll backoff: when no work is available, back off exponentially from
// 2s up to 30s. Cuts idle DB load 15× compared to the old fixed 2s poll
// (NextPending hits the indexed status query each time, but Stats() also
// fires on completion — keeping the loop quiet when nothing is pending
// keeps both happy). Resets to idleSleepMin as soon as a task is found.
const (
	idleSleepMin = 2 * time.Second
	idleSleepMax = 30 * time.Second
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

	idleSleep := idleSleepMin
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
			if w.sleepOrCancel(idleSleep) {
				return
			}
			idleSleep = nextIdleSleep(idleSleep)
			continue
		}
		if task == nil {
			if w.sleepOrCancel(idleSleep) {
				return
			}
			idleSleep = nextIdleSleep(idleSleep)
			continue
		}

		// Found work — reset backoff so the next batch is picked up promptly.
		idleSleep = idleSleepMin
		bundle, _ := w.deps.Settings.GetBundle()
		w.runTask(w.runCtx, task, bundle)
		w.emitStats()
	}
}

// nextIdleSleep doubles d capped at idleSleepMax.
func nextIdleSleep(d time.Duration) time.Duration {
	d *= 2
	if d > idleSleepMax {
		return idleSleepMax
	}
	return d
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
