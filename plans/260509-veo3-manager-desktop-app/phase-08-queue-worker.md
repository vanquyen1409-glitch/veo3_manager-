# Phase 08 — Queue Worker (pause/resume/stop, hot-reload config)

## Context
- Parent: [plan.md](plan.md)
- Depends on: [Phase 02](phase-02-database-models.md) (TaskRepo), [Phase 03](phase-03-browser-service.md), [Phase 05](phase-05-google-labs-api-client.md), [Phase 07](phase-07-video-download.md).

## Overview
- **Date:** 2026-05-09
- **Description:** A single background goroutine that drives one task at a time end-to-end: pull next pending → submit via API → wait for success → resolve + download all videos → save outputs → emit progress events. Supports Pause/Resume/Stop from UI without restart, picks up live config changes between tasks.
- **Priority:** Blocking — central orchestrator
- **Implementation status:** Pending
- **Review status:** Pending

## Key insights
- A single-worker model is plenty for v1: Google rate-limits anyway, and serial processing keeps SQLite happy.
- **Hot-reloadable config:** before each task, the worker reads `Settings` under a mutex; a UI edit while running affects only the *next* task — exactly the requirement.
- **Pause** is checked at "safe points" (between tasks, between videos within a task). Mid-download pausing is intentionally not supported in v1 — keep contract simple.
- **Stop** is a hard-cancel via `context.CancelFunc`; in-flight task is marked `cancelled`.
- All state transitions persist to SQLite *and* emit `runtime.EventsEmit` so multiple frontend pages stay in sync.

## Requirements
1. `QueueWorker` exposes:
   - `Start(ctx context.Context)` — starts the goroutine; idempotent.
   - `Stop()` — cancels current ctx; waits for goroutine to exit.
   - `Pause()` / `Resume()` — toggle internal pause flag.
   - `GetState() WorkerState` — returns `idle | running | paused | stopping`.
2. Worker is owned by `App`; bound methods on App proxy to worker:
   - `StartQueue() error`, `PauseQueue() error`, `ResumeQueue() error`, `StopQueue() error`, `GetQueueState() WorkerState`.
3. Persistent queue resume: on `Startup`, sweep `tasks` with `status='running'` → reset to `pending` (the previous run crashed mid-task).
4. Per-task lifecycle:
   - `pending → running` (DB update + event)
   - submit → store mediaIds + seeds
   - for each mediaId, `Wait` → `Resolve` → `Download` → append to `videoPaths`
   - `running → succeeded` if ≥1 video downloaded; `failed` if all error; partial outputs preserved.
5. Events emitted: `queue:state` (worker state), `queue:task` (per-task progress: started, submitted, polling N/M, downloading N/M, done, failed), `queue:stats` (every task completion).
6. Concurrency: a single mutex guards worker state; the actual work runs without holding the mutex.

## Architecture
```
internal/queue/
├── worker.go       // QueueWorker struct, lifecycle, state machine
├── runner.go       // Per-task pipeline (submit → wait → resolve → download)
├── events.go       // Event payload types + emit helpers
└── state.go        // WorkerState enum, transitions
```

### Worker skeleton
```go
type QueueWorker struct {
    ctx       context.Context        // Wails ctx for events
    runCtx    context.Context        // cancellable per Start
    cancel    context.CancelFunc

    repo      *db.TaskRepo
    settings  *db.SettingsRepo
    api       *labsapi.Client
    download  *download.Downloader
    browser   *browser.Service

    mu        sync.Mutex
    state     WorkerState
    paused    bool
    pauseCh   chan struct{}
}

func (w *QueueWorker) Start(ctx context.Context) error {
    w.mu.Lock()
    if w.state != StateIdle { w.mu.Unlock(); return nil }
    w.runCtx, w.cancel = context.WithCancel(ctx)
    w.state = StateRunning
    w.mu.Unlock()
    w.emitState()
    go w.loop()
    return nil
}

func (w *QueueWorker) loop() {
    defer w.exit()
    for {
        if err := w.runCtx.Err(); err != nil { return }
        w.waitIfPaused()

        task, err := w.repo.NextPending()
        if err != nil { w.errLog(err); time.Sleep(2 * time.Second); continue }
        if task == nil {
            // No work; sleep but stay alive so user can enqueue more.
            select {
            case <-w.runCtx.Done(): return
            case <-time.After(2 * time.Second):
            }
            continue
        }

        cfg := w.snapshotConfig()  // live read
        w.runTask(w.runCtx, task, cfg)
    }
}
```

### `runTask` pipeline
```go
func (w *QueueWorker) runTask(ctx context.Context, t *db.Task, cfg db.GenerationConfig) {
    w.repo.UpdateStatus(t.ID, db.StatusRunning, "")
    w.emitTask(t.ID, "running", nil)

    refs, err := w.api.Submit(ctx, t.Prompt, cfg)
    if err != nil { w.fail(t, err, nil); return }
    w.repo.SaveMediaIDs(t.ID, refs)
    w.emitTask(t.ID, "submitted", map[string]any{"count": len(refs)})

    var paths []string
    for i, ref := range refs {
        w.emitTask(t.ID, "polling", map[string]any{"i": i+1, "n": len(refs)})
        final, err := w.api.Wait(ctx, ref)
        if err != nil { continue } // partial OK
        w.emitTask(t.ID, "downloading", map[string]any{"i": i+1, "n": len(refs)})
        gcsURL, err := w.download.Resolve(ctx, final.VideoURL)
        if err != nil { continue }
        dst := filepath.Join(cfg.OutputDir, download.BuildName(t.ID, ref.Seed))
        if err := w.download.Download(ctx, gcsURL, dst); err != nil { continue }
        paths = append(paths, dst)
        w.repo.AppendOutput(t.ID, dst)
    }

    if len(paths) == 0 { w.fail(t, errors.New("all videos failed"), nil); return }
    w.repo.UpdateStatus(t.ID, db.StatusSucceeded, "")
    w.emitTask(t.ID, "succeeded", map[string]any{"paths": paths})
    w.emitStats()
}
```

## Related code files
- `internal/queue/*`
- `app.go` — bind `StartQueue` etc.; instantiate worker in `Startup`.

## Implementation steps
1. Define `WorkerState` and `Event` types.
2. Implement `Start`, `Stop`, `Pause`, `Resume`, `GetState` with strict state machine.
3. Implement `loop` with idle-sleep when no pending tasks.
4. Implement `runTask` with partial-success rules.
5. Implement on-startup recovery: rewrite stale `running` rows back to `pending`.
6. Wire events; document payload shapes in `events.go`.
7. Bind methods on `App`.
8. Manual test: enqueue 3 prompts, hit Pause after 1 finishes, verify 2nd doesn't start; Resume, verify completion; Stop mid-task, verify task marked `cancelled`.

## Todo list
- [ ] Worker struct + state machine.
- [ ] `loop()` with idle-sleep + ctx cancel.
- [ ] `runTask` pipeline (submit → wait → resolve → download).
- [ ] Recovery on startup.
- [ ] Event emission helpers.
- [ ] Bind on App.
- [ ] Manual end-to-end test of pause/resume/stop.

## Success criteria
- 3 prompts enqueued → 3 tasks complete in order; UI events arrive monotonically.
- `Pause()` halts before next task pickup within 2s.
- `Stop()` aborts in-flight HTTP requests via ctx cancel; task row ends as `cancelled`.
- App restart mid-run → orphan `running` rows reset to `pending` and resume.
- Race detector clean under `go test -race ./internal/queue/...`.

## Risk assessment
- **Goroutine leaks** — worker `loop` only exits when `runCtx.Done`; `Stop` always calls cancel.
- **Stale config** — config snapshot read at task start; do not pass mutable pointer through pipeline.
- **Failure cascade** — one task failing must not crash the loop; wrap `runTask` in `recover()`.
- **Browser dies mid-task** — `runTask` consults `BrowserService.Token()` which auto-refreshes; if Chrome dead, status flips to `disconnected` and worker pauses with a clear UI banner.

## Security considerations
- Worker never logs prompts in plaintext if user opts in to a "redact prompts" setting (future). For v1 prompts are logged at debug level only.

## Next steps
Phase 09 — Frontend Shell + Routing + Toast.
