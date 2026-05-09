# Phase 09 — Frontend Shell + Routing + Toast + Browser Status

## Context
- Parent: [plan.md](plan.md)
- Depends on: [Phase 01](phase-01-project-bootstrap.md), [Phase 03](phase-03-browser-service.md) (for `browserStatus` event), [Phase 08](phase-08-queue-worker.md) (for `queue:state` event).

## Overview
- **Date:** 2026-05-09
- **Description:** App-wide React shell: frameless titlebar, sidebar with 4 nav links and a live browser-status indicator, route outlet for the four pages, toast notifications, and Zustand stores wired to Wails events emitted by Go.
- **Priority:** Blocking for all UI pages
- **Implementation status:** Pending
- **Review status:** Pending

## Key insights
- React Router with a `RootLayout` that owns titlebar + sidebar; `<Outlet/>` swaps page content.
- One global `useAppStore` (Zustand) reflects `browserStatus`, `queueState`, current toast list. Pages subscribe selectively to avoid unnecessary re-renders.
- Wails `EventsOn` returns an unsubscribe function — must be called in `useEffect` cleanup.
- Toast = simple in-memory list with auto-dismiss timers (no library needed).
- Tailwind dark single theme: every component uses `bg-slate-900`, `border-slate-800`, accent `indigo-500` / `emerald-500` / `red-500`.

## Requirements
1. Routes: `/dashboard` (default), `/queue`, `/history`, `/settings`. NotFound → redirect dashboard.
2. Sidebar collapsible to icon-only on narrow widths; live browser status dot (green ready, amber connecting/needs_login, red error/disconnected).
3. Toasts: bottom-right stack of up to 5; types `info | success | warn | error`; default 4s auto-dismiss; user can dismiss manually.
4. Title bar: app icon, title text, draggable region, minimize/maximize/close buttons.
5. Global error boundary catches render errors; shows full-screen error card with reload button.
6. Initial render: call `EnsureBrowser()` and `GetQueueState()` once on mount.

## Architecture
```
frontend/src/
├── App.tsx                       // <BrowserRouter><RootLayout><Outlet/></...
├── components/
│   ├── TitleBar.tsx
│   ├── Sidebar.tsx
│   ├── BrowserStatusDot.tsx
│   ├── Toaster.tsx
│   └── ErrorBoundary.tsx
├── pages/
│   ├── DashboardPage.tsx        // Phase 11
│   ├── QueuePage.tsx            // Phase 10
│   ├── HistoryPage.tsx          // Phase 11
│   └── SettingsPage.tsx         // Phase 11
├── store/
│   ├── appStore.ts              // browser, queue, toast
│   └── eventBridge.ts           // Wails event subscriptions
├── lib/
│   ├── url.ts                   // localFileURL
│   └── format.ts                // ms→hh:mm:ss, byteSize, etc.
└── styles/index.css
```

### appStore (Zustand)
```ts
type AppState = {
  browser: BrowserStatus;          // 'disconnected'|'connecting'|'needs_login'|'ready'|'error'
  queueState: WorkerState;         // 'idle'|'running'|'paused'|'stopping'
  toasts: Toast[];
  setBrowser: (s: BrowserStatus) => void;
  setQueueState: (s: WorkerState) => void;
  pushToast: (t: Omit<Toast,'id'>) => void;
  dismissToast: (id: string) => void;
};

export const useAppStore = create<AppState>((set, get) => ({
  browser: 'disconnected',
  queueState: 'idle',
  toasts: [],
  setBrowser:    (s) => set({ browser: s }),
  setQueueState: (s) => set({ queueState: s }),
  pushToast: (t) => {
    const id = crypto.randomUUID();
    set(s => ({ toasts: [...s.toasts, { ...t, id }] }));
    setTimeout(() => get().dismissToast(id), t.timeoutMs ?? 4000);
  },
  dismissToast: (id) => set(s => ({ toasts: s.toasts.filter(x => x.id !== id) })),
}));
```

### eventBridge
```ts
export function installEventBridge() {
  EventsOn('browserStatus',   (p: any) => useAppStore.getState().setBrowser(p.status));
  EventsOn('queue:state',     (p: any) => useAppStore.getState().setQueueState(p.state));
  EventsOn('queue:task',      (p: any) => useQueueStore.getState().applyTaskEvent(p));
  EventsOn('queue:stats',     (p: any) => useDashboardStore.getState().setStats(p));
}
```

### Sidebar status dot states
| Backend status   | Color   | Tooltip                      |
|------------------|---------|------------------------------|
| disconnected     | red     | "Click Settings → Connect"   |
| connecting       | amber   | "Connecting to Chrome…"      |
| needs_login      | amber   | "Sign in to Google"          |
| ready            | emerald | "Browser ready"              |
| error            | red     | error message                |

## Related code files
- `frontend/src/App.tsx`, `components/*`, `store/*`, `lib/*`.

## Implementation steps
1. **Router** in `App.tsx` with `RootLayout`, four routes + redirect.
2. **`RootLayout`** — flex column: `<TitleBar/>` row, then flex row of `<Sidebar/>` + `<main className="flex-1 overflow-auto"><Outlet/></main>` + `<Toaster/>` overlay.
3. **`TitleBar`** with draggable CSS var, three window-control buttons.
4. **`Sidebar`** — `<NavLink>`s for Dashboard/Queue/History/Settings; bottom-anchored `BrowserStatusDot`.
5. **Stores**: implement `useAppStore`, `useQueueStore` (Phase 10 will extend), `useDashboardStore`.
6. **`eventBridge`** wired from `RootLayout` mount.
7. **Toaster** — simple AnimatePresence-free CSS transitions; absolute `bottom-4 right-4`.
8. **`ErrorBoundary`** — class component wrapping `<RootLayout>`; logs to backend via bound `LogClientError(string)` so we capture in app log file.
9. Call `EnsureBrowser()` once on mount; ignore returned promise (state will arrive via event).

## Todo list
- [ ] Install React Router; configure routes.
- [ ] Build `TitleBar`, `Sidebar`, `BrowserStatusDot`.
- [ ] Implement `useAppStore` and `eventBridge`.
- [ ] Build `Toaster` with auto-dismiss.
- [ ] Add `ErrorBoundary`.
- [ ] Verify navigation, browser dot reacts to event from Go (force-emit a status from a debug button).

## Success criteria
- Sidebar dot turns green within 5s of `EnsureBrowser()` succeeding.
- Toasts queue and auto-dismiss; manual dismiss works.
- All four pages render via routing; URL persists in webview history during session.
- No console errors at idle.

## Risk assessment
- **Event listeners not cleaned up** — `EventsOn` returns unsub; install once at app bootstrap; never dynamic re-subscribe.
- **Tailwind purge over-trims** — verify `safelist` or content globs include component files.
- **Zustand re-render storms** — selectors with shallow equality; never subscribe to whole store.

## Security considerations
- All bound methods only accept primitives or typed structs; UI cannot construct rod calls directly.
- HTML rendering uses React's auto-escape; never `dangerouslySetInnerHTML`.

## Next steps
Phase 10 — Queue page (uses event store + bound queue methods).
