# Phase 11 — Frontend Dashboard / History / Settings

## Context
- Parent: [plan.md](plan.md)
- Depends on: [Phase 02](phase-02-database-models.md), [Phase 04](phase-04-localfile-http-handler.md), [Phase 08](phase-08-queue-worker.md), [Phase 09](phase-09-frontend-shell.md).

## Overview
- **Date:** 2026-05-09
- **Description:** Three remaining UI pages.
  - **Dashboard:** at-a-glance stats (today, last 7 days, success rate, current queue).
  - **History:** paginated/filterable table of completed/failed tasks with multi-video carousel preview and requeue.
  - **Settings:** Chrome path / user-data-dir / output dir / debug port / poll timings, plus a debug info panel and "Reset Profile" button.
- **Priority:** High — required for product completeness
- **Implementation status:** Pending
- **Review status:** Pending

## Key insights
- All three pages are read-mostly; perf goal is "snappy at 1k history rows".
- Use server-side pagination (`ListTasks({offset,limit,filter})`) — never load all rows into memory.
- Multi-video carousel is just a horizontal scroll-snap container with `<video>` elements pointed at `localFileURL(path)`.
- Debug panel surfaces values that future bug reports need: Wails version, Go version, Chrome path, profile dir, current token age, recent errors.
- Settings changes must also push the new value to backend so live components re-read (e.g. queue worker uses fresh `outputDir` on the next task).

## Requirements

### Dashboard
1. Top row of stat cards: **Today (success/total)**, **Last 7 days**, **All-time success rate**, **Currently running**.
2. Mini chart: 14-day daily generations bar chart (skip lib for v1 — render with CSS divs).
3. "Recent activity" list (last 10 succeeded/failed tasks) with mini thumbnail.
4. Backed by `GetStats()` call + `queue:stats` event for live refresh.

### History
1. Filter bar: status pill (`all|succeeded|failed|cancelled`), date-range picker (text inputs OK in v1), search box (LIKE on prompt), sort dropdown.
2. Table columns: Created, Prompt (truncated), Status, Outputs (count), Duration, Actions.
3. Click row → side panel opens with full prompt, config used, error (if any), and a multi-video carousel.
4. Carousel: horizontal flex container, `scroll-snap-type: x mandatory`; each `<video controls>` has `localFileURL(path)`.
5. Row actions: **Requeue** (creates a new pending task with same prompt + cfg → toast confirm), **Open file location** (calls `OpenInExplorer(path)` bound method), **Delete** (with confirm dialog).
6. Pagination: 50 rows/page; show total count.

### Settings
1. **Chrome** group: chrome path, user-data-dir, debug port; **Test Connect** button (calls `EnsureBrowser`, surfaces ready/error).
2. **Output** group: output dir (Browse), max parallel downloads (1 v1; reserved for future).
3. **Generation defaults** group: model, aspect, output count, seed mode (random|fixed). Same controls as Queue page; "Save defaults" persists.
4. **Polling** group: poll interval (ms), poll timeout (ms).
5. **Debug Info** card (read-only): app version, Wails version, Go version, OS build, Chrome path resolved, profile dir, token age (mm:ss), last 5 backend log lines.
6. Buttons: **Reset Profile** (deletes user-data-dir after confirm; will require re-login), **Open Logs Folder**, **Copy Diagnostics** (copies all debug info to clipboard).

## Architecture
```
frontend/src/pages/
├── DashboardPage.tsx
├── HistoryPage.tsx
└── SettingsPage.tsx
frontend/src/features/
├── dashboard/StatCard.tsx, RecentActivity.tsx, BarMini.tsx
├── history/HistoryTable.tsx, HistoryFilters.tsx, HistoryDetailDrawer.tsx, VideoCarousel.tsx
└── settings/SettingsForm.tsx, DebugCard.tsx
```

### VideoCarousel (snippet)
```tsx
export function VideoCarousel({ paths }: { paths: string[] }) {
  return (
    <div className="flex gap-3 overflow-x-auto snap-x snap-mandatory pb-2">
      {paths.map(p => (
        <video
          key={p}
          src={localFileURL(p)}
          controls
          preload="metadata"
          className="snap-start rounded-lg w-72 h-40 object-cover bg-black flex-none"
        />
      ))}
    </div>
  );
}
```

### Bound App methods needed for this phase
- `GetStats(range string) Stats` — `range` ∈ `today|7d|all`.
- `ListTasks(filter ListFilter) []Task` (already in Phase 02).
- `RequeueTask(id string) Task` (Phase 02).
- `DeleteTask(id string) error`.
- `OpenInExplorer(path string) error` — uses `runtime.BrowserOpenURL` or `os/exec` with `explorer /select`.
- `OpenLogsFolder() error`.
- `CopyDiagnostics() string` — returns text to copy in JS.
- `ResetBrowserProfile() error` — stops worker, deletes `userDataDir`.
- `ChooseDirectory(current string) (string, error)`.
- `GetDebugInfo() DebugInfo`.

## Related code files
- `frontend/src/pages/{Dashboard,History,Settings}Page.tsx`
- `frontend/src/features/dashboard/*`
- `frontend/src/features/history/*`
- `frontend/src/features/settings/*`
- `app.go` — implement bound methods listed above.

## Implementation steps
1. **Dashboard** — implement `StatCard`, `BarMini` (CSS bars), `RecentActivity`. Fetch stats on mount; subscribe to `queue:stats`.
2. **History list** — filter UI + table + pagination. Debounce search input 300ms.
3. **History detail drawer** — slide-in from right (Tailwind transition); render `VideoCarousel` if `videoPaths.length > 1`, single `<video>` if exactly 1.
4. **Requeue / delete** — confirm dialogs; toasts on result.
5. **OpenInExplorer** — Go side: `exec.Command("explorer", "/select,", path).Start()`.
6. **Settings form** — react-hook-form-lite (skip lib; `useState` is enough for ~10 fields). Save button writes via `SaveSettings`.
7. **Debug card** — auto-refresh every 5s via `setInterval`; "Copy Diagnostics" uses `navigator.clipboard.writeText`.
8. **Reset Profile** — calls `ResetBrowserProfile`; on success, status flips to `disconnected`; user clicks "Test Connect" to re-launch.
9. Manual end-to-end on each page.

## Todo list
- [ ] Dashboard stats + chart + recent activity.
- [ ] History filters, table, pagination, detail drawer, carousel.
- [ ] Settings form with persistence.
- [ ] Debug info card + Copy Diagnostics.
- [ ] Implement bound methods on App.
- [ ] Manual review of all three pages.

## Success criteria
- Dashboard renders stats <300ms after page mount with 1k tasks in DB.
- History pagination is smooth on 5k rows; search results return <200ms.
- Carousel plays multiple videos; seeking works.
- Settings save persists across app restart.
- Reset Profile correctly deletes dir and resets browser status.

## Risk assessment
- **Long Windows path in Open Explorer** — handle UNC/long path; fallback to opening parent dir.
- **Debug card leaks token** — show only token *age*, never the value.
- **Reset Profile while worker runs** — pre-check `queueState`; refuse with clear message.

## Security considerations
- All filesystem mutations confirm via dialog.
- Diagnostics copy explicitly excludes token, full prompts (truncate to 80 chars), and absolute paths beyond `outputDir`.

## Next steps
Phase 12 — Build, installer, polish.
