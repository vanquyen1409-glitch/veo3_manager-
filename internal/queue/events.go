package queue

import (
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Event names are the channel keys used by runtime.EventsEmit. Keep in sync
// with eventBridge.ts on the frontend.
const (
	EventQueueState = "queue:state"
	EventQueueTask  = "queue:task"
	EventQueueStats = "queue:stats"
)

// StatePayload is the payload of EventQueueState.
type StatePayload struct {
	State   State  `json:"state"`
	Message string `json:"message,omitempty"`
}

// TaskEvent is the payload of EventQueueTask. The frontend reducer uses
// patch-style updates so we don't have to send the whole row each tick.
type TaskEvent struct {
	ID    string         `json:"id"`
	Phase string         `json:"phase"` // "running"|"submitted"|"polling"|"downloading"|"succeeded"|"failed"|"cancelled"
	Patch map[string]any `json:"patch,omitempty"`
}

func (w *Worker) emitState(msg string) {
	w.mu.Lock()
	ctx := w.wailsCtx
	st := w.state
	w.mu.Unlock()
	if ctx == nil {
		return
	}
	wruntime.EventsEmit(ctx, EventQueueState, StatePayload{State: st, Message: msg})
}

func (w *Worker) emitTask(id, phase string, patch map[string]any) {
	w.mu.Lock()
	ctx := w.wailsCtx
	w.mu.Unlock()
	if ctx == nil {
		return
	}
	wruntime.EventsEmit(ctx, EventQueueTask, TaskEvent{ID: id, Phase: phase, Patch: patch})
}

func (w *Worker) emitStats() {
	w.mu.Lock()
	ctx := w.wailsCtx
	w.mu.Unlock()
	if ctx == nil {
		return
	}
	stats, err := w.deps.Tasks.Stats()
	if err != nil {
		return
	}
	wruntime.EventsEmit(ctx, EventQueueStats, stats)
}
