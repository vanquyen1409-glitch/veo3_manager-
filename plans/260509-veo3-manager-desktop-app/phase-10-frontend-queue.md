# Phase 10 — Frontend Queue Page

## Context
- Parent: [plan.md](plan.md)
- Depends on: [Phase 02](phase-02-database-models.md), [Phase 08](phase-08-queue-worker.md), [Phase 09](phase-09-frontend-shell.md).

## Overview
- **Date:** 2026-05-09
- **Description:** The work-horse page. Top: per-batch generation settings (model, aspect ratio, output count, output dir). Middle: prompt textarea (one prompt per line) + Enqueue button. Bottom: live task list with per-task progress, queue control bar (Start / Pause / Resume / Stop), aggregate progress.
- **Priority:** Blocking — main user touchpoint
- **Implementation status:** Pending
- **Review status:** Pending

## Key insights
- "Enqueue" only persists tasks; the queue worker pulls from DB whenever it's running. Settings panel writes to `settings` (persistent default) but each enqueue snapshots the values into the task's `config_json`, so live edits during a run only affect *future* enqueues + the worker's *next* task.
- Live task list is event-driven: `queue:task` events update individual task rows. Initial load via `ListTasks({status: ['pending','running']})`.
- Multi-video preview happens on History page; Queue page shows compact thumbnails or counts only to avoid clutter.
- Pause/Resume/Stop buttons reflect `queueState` from app store; only one of Start/Resume is visible at a time.

## Requirements
1. Settings panel (collapsible card):
   - Model dropdown (only `veo_3_1_t2v_fast_ultra` for v1; UI ready for more).
   - Aspect ratio: segmented control `16:9 | 9:16 | 1:1`.
   - Output count: stepper 1–4.
   - Output dir: read-only path + "Browse" button (calls `ChooseDirectory` bound method).
   - "Save as default" button (writes to settings table).
2. Prompt input:
   - `<textarea>` accepting one prompt per non-empty line.
   - Token-aware char counter and per-line count.
   - "Enqueue all" → calls `EnqueuePrompts(prompts, cfg)`.
3. Task list:
   - Two sections: "In Progress" (running) at top, "Pending" below.
   - Per row: prompt preview (1 line + ellipsis), status badge, progress bar (videos done / total), elapsed time, cancel button (only on pending).
   - Live updates via store.
4. Control bar:
   - When `queueState='idle'` and pending exists: show **Start**.
   - When `running`: show **Pause** + **Stop**.
   - When `paused`: show **Resume** + **Stop**.
   - When `stopping`: disable all.
5. Toast on every terminal event (success/fail/cancel).

## Architecture
```
frontend/src/pages/QueuePage.tsx
frontend/src/features/queue/
├── SettingsPanel.tsx
├── PromptInput.tsx
├── TaskRow.tsx
├── TaskList.tsx
├── ControlBar.tsx
└── useQueueStore.ts        // task collection + applyTaskEvent
```

### useQueueStore
```ts
type QState = {
  tasks: Record<string, Task>;        // by id
  loadInitial: () => Promise<void>;
  applyTaskEvent: (e: TaskEvent) => void;
  enqueue: (prompts: string[], cfg: GenerationConfig) => Promise<void>;
};

export const useQueueStore = create<QState>((set, get) => ({
  tasks: {},
  loadInitial: async () => {
    const list = await ListTasks({ status: ['pending', 'running'] });
    set({ tasks: Object.fromEntries(list.map(t => [t.id, t])) });
  },
  applyTaskEvent: (e) => set(s => {
    const cur = s.tasks[e.id] || { id: e.id };
    return { tasks: { ...s.tasks, [e.id]: { ...cur, ...e.patch } } };
  }),
  enqueue: async (prompts, cfg) => {
    const created = await EnqueuePrompts(prompts, cfg);
    set(s => ({
      tasks: {
        ...s.tasks,
        ...Object.fromEntries(created.map(t => [t.id, t])),
      },
    }));
  },
}));
```

### ControlBar logic
```tsx
const qs = useAppStore(s => s.queueState);
const hasPending = useQueueStore(s => Object.values(s.tasks).some(t => t.status === 'pending'));

return (
  <div className="flex gap-2 p-4 border-t border-slate-800">
    {qs === 'idle' && hasPending && <Btn onClick={StartQueue}><Play/> Start</Btn>}
    {qs === 'running' && <><Btn onClick={PauseQueue}><Pause/> Pause</Btn><Btn variant="danger" onClick={StopQueue}><Square/> Stop</Btn></>}
    {qs === 'paused'  && <><Btn onClick={ResumeQueue}><Play/> Resume</Btn><Btn variant="danger" onClick={StopQueue}><Square/> Stop</Btn></>}
    {qs === 'stopping' && <Btn disabled><Loader/> Stopping…</Btn>}
  </div>
);
```

### Per-task progress patch payload (from Phase 08)
```ts
type TaskEvent = {
  id: string;
  patch: Partial<Task> & {
    progress?: { phase: 'submitted'|'polling'|'downloading'|'done'; i?: number; n?: number };
  };
};
```

## Related code files
- `frontend/src/pages/QueuePage.tsx`
- `frontend/src/features/queue/*`
- New bound App method: `ChooseDirectory(currentPath string) (string, error)` opening Wails native dir picker.

## Implementation steps
1. Implement `SettingsPanel` with controlled inputs; persist on "Save as default".
2. Implement `PromptInput`; split lines, trim, drop empties; show counts.
3. Implement `TaskRow` with progress bar (numerator from event), elapsed timer (computed from `started_at`).
4. `TaskList` partitions tasks by status; virtualize only if list exceeds 200 (skip in v1).
5. `ControlBar` per logic above.
6. Wire `useQueueStore.loadInitial` on page mount.
7. Wire `applyTaskEvent` from `eventBridge` (Phase 09 already subscribes; just feed into store).
8. Add toasts on terminal events (`succeeded`, `failed`, `cancelled`).

## Todo list
- [ ] `SettingsPanel` with persisted defaults.
- [ ] `PromptInput` with line counter + Enqueue.
- [ ] `TaskList` + `TaskRow` reactive to events.
- [ ] `ControlBar` state-driven button visibility.
- [ ] Browse-directory dialog via Wails runtime.
- [ ] Toast on terminal events.
- [ ] Manual: 5-prompt batch start → pause mid-3rd → resume → all complete.

## Success criteria
- Submitting 10 prompts results in 10 rows in <1s.
- Live progress updates without flicker (≤ 5 re-renders per task per second).
- Pause halts before next pickup; Stop aborts current within 3s.
- "Save as default" persists between app restarts.
- 0 console errors during a 30-task run.

## Risk assessment
- **Heavy event traffic** — coalesce updates via `requestAnimationFrame`-style throttling if profile shows >120 paints/s.
- **Misleading "elapsed" on paused tasks** — pause clock when `queueState='paused'`.
- **User edits settings mid-run** — visual hint in panel: "Changes apply to new prompts" badge.

## Security considerations
- Sanitize prompt input: enforce max length (e.g. 4000 chars per prompt).
- Limit batch size (e.g. ≤500 prompts) to avoid DB surge; configurable.

## Next steps
Phase 11 — Dashboard / History / Settings pages.
