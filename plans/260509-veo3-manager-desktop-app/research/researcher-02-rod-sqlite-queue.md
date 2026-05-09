# Research: go-rod, Stealth, CDP Input.insertText, SQLite Queue
**Date:** 2026-05-09 | **Max 150 lines, terse format**

## 1. go-rod Launcher with Persistent User Profile

**Persistent profile setup:**
```go
import "github.com/go-rod/rod/lib/launcher"

l := launcher.New().
    Set("user-data-dir", "C:\\AppData\\VEO3Manager").
    Set("disable-blink-features", "AutomationControlled").
    Headless(false).
    Devtools(false).
    MustLaunch()
```

**Bin() vs Set():** `launcher.Bin(chromePath)` specifies custom Chrome binary; `Set(flag, val)` adds launch flags.

**Connect to existing Chrome via /json/version:**
```go
// Before launch, poll localhost
import "net/http"
resp, _ := http.Get("http://localhost:9222/json/version")
// If alive, parse JSON: {"webSocketDebuggerUrl": "ws://..."}
// Then: rod.New().ControlURL(wsUrl).Connect()

// If dead, use MustLaunch() above
```

**Gotcha:** Launcher auto-deletes tempDir for persistent profiles—disable via `launcher.New().KeepUserDataDir()` (RemoteMode only) or manually set custom dir.

---

## 2. go-rod/stealth Integration

**Import & usage:**
```go
import "github.com/go-rod/stealth"

browser := rod.New().MustConnect()
page := stealth.Page(browser) // wraps page with injected evasion scripts
```

**What it patches:** Injects JS via `EvalOnNewDocument` to:
- Remove `webdriver` property
- Hide automation markers in `navigator`
- Fake `chrome` object presence
- Override `navigator.plugins` detection

**Source:** [github.com/go-rod/stealth](https://github.com/go-rod/stealth) uses Puppeteer's stealth-evasions, transpiled to Go.

---

## 3. CDP Input.insertText with go-rod

**Bypass Slate.js keyboard handlers:**
```go
import "github.com/go-rod/rod/lib/proto"

elem, _ := page.Element("textarea[role='textbox']")
elem.MustFocus()

// Use CDP directly for React synthetic event bypass
_ = proto.InputInsertText{
    Text: "Your prompt text here",
}.Call(page)
```

**Why this works:** `Input.insertText` fires browser's native `input` event + `beforeinput`, bypassing Slate's `onKeyDown` listeners. React synthesizes events from native DOM events, so CDP insertion triggers proper React state updates.

---

## 4. Token Extraction via page.Eval()

**Access __NEXT_DATA__ JSON:**
```go
tokenJS := `
  (window.__NEXT_DATA__.props?.pageProps?.session?.access_token ||
   window.__NEXT_DATA__.props?.pageProps?.token)
`
token, _ := page.Eval(tokenJS)
accessToken := token.Value.(string) // cast result
```

**Alternative:** If session in different path, inspect Network tab in DevTools to find actual GraphQL/API auth header bearer token, then extract from storage/cookies.

---

## 5. Video Download Session Reuse

**Capture redirect via new tab + navigation wait:**
```go
// In response listener, capture 302 Location
redirectURL := "" // extracted from proto.NetworkResponseReceived

newTab := browser.MustCreateWindow()
_ = newTab.MustNavigate(redirectURL) // auto-follows redirects, arrives at signed GCS URL
finalURL := newTab.MustInfo().URL

// Download via standard http.Get
resp, _ := http.Get(finalURL)
// No auth needed; GCS URL is pre-signed
```

---

## 6. Background Queue Worker Pattern

**Single-threaded task processor with pause/resume:**
```go
type QueueWorker struct {
    mu    sync.Mutex
    state string // "running", "paused", "stopped"
    ctx   context.Context
    cfg   *Config // hot-reloadable
}

func (w *QueueWorker) Process() {
    for task := range taskCh {
        w.mu.Lock()
        if w.state == "stopped" { w.mu.Unlock(); break }
        cfg := w.cfg // read latest config
        w.mu.Unlock()
        
        // Execute task with cfg
        w.emitProgress(task.ID, "running")
    }
}

// Pause/resume/stop via state mutation + signal
func (w *QueueWorker) Pause() {
    w.mu.Lock()
    w.state = "paused"
    w.mu.Unlock()
}
```

---

## 7. modernc.org/sqlite Setup

**Pure Go driver, no CGO:**
```go
import (
    "database/sql"
    _ "modernc.org/sqlite"
)

db, _ := sql.Open("sqlite", "C:\\AppData\\VEO3Manager\\queue.db")

// Schema migration
db.Exec(`
    CREATE TABLE IF NOT EXISTS tasks (
        id TEXT PRIMARY KEY,
        status TEXT,
        video_paths JSON,
        created_at DATETIME
    );
    CREATE INDEX idx_status ON tasks(status);
`)

// Store JSON array
db.Exec(`INSERT INTO tasks (..., video_paths) VALUES (..., ?)`,
    `["path/1.mp4", "path/2.mp4"]`)
```

**Concurrency:** Single writer only. For queue, use 1 DB connection per worker goroutine or connection pool with low open count.

---

## Unresolved Questions
- Exact schema for labs.google __NEXT_DATA__ (may vary by region/session state)
- GCS signed URL TTL—if >5min tasks fail, may need re-fetch
- Chrome reuse detection—how to reliably test if existing process is dead

---

## Citations
- [launcher pkg.go.dev](https://pkg.go.dev/github.com/go-rod/rod/lib/launcher)
- [github.com/go-rod/rod](https://github.com/go-rod/rod)
- [github.com/go-rod/stealth](https://github.com/go-rod/stealth)
- [LambdaTest: UserDataDir](https://www.lambdatest.com/automation-testing-advisor/golang/methods/rod_go.launcher.UserDataDir)
- [modernc.org/sqlite pkg.go.dev](https://pkg.go.dev/modernc.org/sqlite)
- [River queue docs](https://riverqueue.com/docs/sqlite)
- [NextAuth.js securing pages](https://next-auth.js.org/tutorials/securing-pages-and-api-routes)
- [React SyntheticEvent](https://legacy.reactjs.org/docs/events.html)
- [Slate.js event handlers](https://docs.slatejs.org/walkthroughs/02-adding-event-handlers)
