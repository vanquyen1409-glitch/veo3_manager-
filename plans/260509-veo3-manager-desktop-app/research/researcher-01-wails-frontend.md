# Wails v2 Frontend Research: Go + React + TypeScript + Vite + Tailwind

**Date**: 2026-05-09 | **Status**: Current (2025/2026) sources | **Token-efficient**: Terse format

---

## 1. Wails v2 Setup Essentials

**Project Init**: `wails init -n veo3-manager -t react-ts` creates React+TS scaffold with Vite.

**wails.json core**:
```json
{
  "name": "veo3-manager",
  "outputfilename": "veo3-manager",
  "frontend": "frontend",
  "dev": "npm run dev",
  "build": "npm run build",
  "assetserver": {
    "assetsdir": "frontend/dist",
    "middleware": "AssetServerMiddleware"
  },
  "windows": {
    "frameless": true,
    "transparentTitleBar": true,
    "webviewIsTransparent": true
  }
}
```

**app.go lifecycle**:
```go
type App struct{}

func (a *App) Startup(ctx context.Context) {
  // Init on first load
}

func (a *App) DOMReady(ctx context.Context) {
  // Frontend ready, safe to emit events
}

func (a *App) BeforeClose(ctx context.Context) bool {
  // Return false to prevent close
  return true
}

func (a *App) Shutdown(ctx context.Context) {
  // Cleanup goroutines
}
```

**Build modes**: `wails dev` (hot reload, debug port 6060), `wails build` (prod binary). Flag `-webview2 embed` bundles bootstrapper (~150KB).

---

## 2. Go ↔ JS Binding

**Struct binding** in main.go:
```go
app := NewApp()
err := wails.Run(&options.App{
  Bind: []interface{}{app},
  // ...
})
```

Wails auto-generates TypeScript at `wailsjs/go/main`. Methods must be **exported** (capitalized) and return `(result, error)`.

**Error handling**:
```go
func (a *App) GetQueue(ctx context.Context) ([]Job, error) {
  if err != nil {
    return nil, err // Sends to JS as {code, message, extra}
  }
  return jobs, nil
}
```

**Event emission** from goroutine:
```go
func (a *App) StartQueueWorker(ctx context.Context) {
  go func() {
    for job := range a.queue {
      runtime.EventsEmit(ctx, "queueProgress", map[string]interface{}{
        "jobId": job.ID,
        "progress": 45,
        "status": "processing",
      })
    }
  }()
}
```

**React listener**:
```tsx
useEffect(() => {
  const unsubscribe = EventsOn("queueProgress", (data: QueueProgress) => {
    setProgress(data);
  });
  return () => unsubscribe?.();
}, []);
```

**Best practice**: Emit progress events at ~100-500ms intervals (not per-frame). Unbind listeners on component unmount.

---

## 3. Custom HTTP Middleware for Local Files

**Problem**: `<video src="/localfile/C:/path/video.mp4">` fails with file:// URL. **Solution**: Custom AssetServer handler.

**wails.json**:
```json
"assetserver": {
  "assetsdir": "frontend/dist",
  "middleware": "AssetServerMiddleware"
}
```

**main.go**:
```go
type AssetServerMiddleware struct{}

func (m *AssetServerMiddleware) Handle(next http.Handler) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    if strings.HasPrefix(r.URL.Path, "/localfile/") {
      filePath := strings.TrimPrefix(r.URL.Path, "/localfile/")
      http.ServeFile(w, r, filePath)
      return
    }
    next.ServeHTTP(w, r)
  })
}
```

Bind in options:
```go
AssetServer: &assetserver.Options{
  Handler: &AssetServerMiddleware{},
}
```

---

## 4. Frameless Window + Custom Titlebar

**Enable frameless** in wails.json:
```json
"windows": {
  "frameless": true,
  "webviewIsTransparent": true
}
```

**React titlebar component**:
```tsx
export function TitleBar() {
  return (
    <header style={{ "--wails-draggable": "drag" } as CSSProperties}>
      <div className="flex justify-between items-center h-12 px-4 bg-slate-900">
        <span>veo3-manager</span>
        <div style={{ "--wails-draggable": "no-drag" } as CSSProperties} className="flex gap-2">
          <button onClick={() => RuntimeMinimise()}>−</button>
          <button onClick={() => RuntimeToggleMaximise()}>□</button>
          <button onClick={() => RuntimeClose()}>✕</button>
        </div>
      </div>
    </header>
  );
}
```

**CSS**: Non-draggable buttons must have `--wails-draggable: no-drag` to stay clickable.

---

## 5. React 18 + Vite + Tailwind Dark Mode + Zustand

**tailwind.config.ts**:
```ts
export default {
  darkMode: 'class',
  theme: {
    extend: {
      colors: { /* custom */ }
    }
  }
};
```

**Zustand store** (persist skipped if SQLite is source of truth):
```ts
export const useQueueStore = create<QueueState>((set) => ({
  jobs: [],
  addJob: (job) => set(s => ({ jobs: [...s.jobs, job] })),
  updateProgress: (id, progress) => set(s => ({
    jobs: s.jobs.map(j => j.id === id ? {...j, progress} : j)
  }))
}));
```

**Dark toggle**:
```tsx
const toggleDark = () => {
  document.documentElement.classList.toggle('dark');
};
```

**React Router** (if multi-page): Lightweight, use `createBrowserRouter` + route-based pages (Dashboard, Queue, History, Settings). **Or**: Simple state-based page switching if < 5 routes.

**Lucide icons**: `import { AlertCircle, Play } from 'lucide-react'` → `<Play className="w-4 h-4" />`.

---

## 6. Build & Distribution

**Dev workflow**:
```bash
npm run dev  # Runs frontend Vite dev server + Wails backend
```

**Production build**:
```bash
wails build -nsis -webview2 embed
```

Creates NSIS installer + embedded WebView2 bootstrapper in build/windows/veo3-manager-installer.exe.

**WebView2 requirement**: Win10/11 need runtime (Win11 default). `-webview2 download` prompts user to download; `-webview2 embed` adds ~150KB to binary.

**Code signing**: Requires cert for production (not covered here).

**Dev port conflict**: If CDP debugger (port 6060) collides, use `wails dev -webdev http://localhost:3000` to bind frontend dev to specific port.

---

## Pitfalls & Mitigations

| Pitfall | Fix |
|---------|-----|
| WebView2 not installed on Win10 target | Use `-webview2 embed` flag, test on clean Win10 VM |
| `wails dev` hot reload fails | Kill port 3000+6060, restart. Frontend TS types stale? Run `wails dev` again |
| CSP blocks external resources | Add `<meta http-equiv="Content-Security-Policy" ...>` or configure in Go headers |
| Video src doesn't load | Use `/localfile/C:/path` + custom middleware handler (see §3) |
| Frameless buttons not clickable | Forget `--wails-draggable: no-drag` on buttons inside drag region |
| Events never arrive in React | Listener mounted after emit? Use `EventsOn` in `useEffect` with cleanup |

---

## Unresolved Questions

1. **Video streaming performance**: Does custom middleware scale for multiple concurrent video requests? Recommend testing with simulated slow network.
2. **Zustand + SQLite sync**: Should store emit events when DB changes? Or only update on explicit mutations?
3. **TypeScript types for generated wailsjs**: Stale after Go changes? Auto-regen on `wails dev`?

---

## Sources

- [Creating a Project | Wails](https://wails.io/docs/gettingstarted/firstproject/)
- [Templates | Wails](https://wails.io/docs/community/templates/)
- [Events | Wails](https://wails.io/docs/reference/runtime/events/)
- [How does it work? | Wails](https://wails.io/docs/howdoesitwork/)
- [Frameless Applications | Wails](https://wails.io/docs/guides/frameless/)
- [Dynamic Assets | Wails](https://wails.io/docs/guides/dynamic-assets/)
- [NSIS installer | Wails](https://wails.io/docs/guides/windows-installer/)
- [Options | Wails](https://wails.io/docs/reference/options/)
- [Windows | Wails](https://wails.io/docs/guides/windows/)
- [GitHub - lontten/wails-vite-react-ts](https://github.com/lontten/wails-vite-react-ts)
- [assetserver package - pkg.go.dev](https://pkg.go.dev/github.com/wailsapp/wails/v2/pkg/assetserver)
