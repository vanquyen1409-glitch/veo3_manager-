# Phase 03 — Browser Service (rod + stealth + token + reuse)

## Context
- Parent: [plan.md](plan.md)
- Depends on: [Phase 02](phase-02-database-models.md) (settings provide `chromePath`, `userDataDir`, `chromeDebugPort`)
- Research: [rod/stealth section](research/researcher-02-rod-sqlite-queue.md)

## Overview
- **Date:** 2026-05-09
- **Description:** A long-lived `BrowserService` that owns the Chrome connection, ensures persistent profile, applies stealth, opens a labs.google session, extracts the bearer token from `__NEXT_DATA__`, exposes the connection to other modules, and emits live status events to the UI.
- **Priority:** Blocking — every API call + download tab depends on this
- **Implementation status:** Pending
- **Review status:** Pending

## Key insights
- **Reuse before launch.** GET `http://localhost:<port>/json/version`. If alive, parse `webSocketDebuggerUrl`, call `rod.New().ControlURL(ws).MustConnect()`. Only if dead do we spawn Chrome ourselves. This lets users keep one Chrome open across restarts and avoids fighting their own browser.
- **Persistent profile is mandatory** to keep Google login. Pass `--user-data-dir=<settings.userDataDir>` to the launcher; never let rod auto-clean it. Use `launcher.New().UserDataDir(dir)` (modern rod) and avoid `Cleanup()`.
- **Stealth is mandatory** OR the API call returns 401/403 and the token field is missing. Wrap each new page with `stealth.Page(browser)` instead of `browser.MustPage()`.
- **`disable-blink-features=AutomationControlled`** must be explicit — stealth alone does not pass this flag.
- **Token shape is fragile.** `__NEXT_DATA__` is a JSON blob set as `<script id="__NEXT_DATA__">…`. The actual access token field path varies; defensive code probes multiple known paths and logs the keys it found.
- **Login is user-driven.** If we can't extract a token, surface "Log in to Google" status and bring the Chrome window forward; user logs in manually, then the worker retries token extraction every few seconds.

## Requirements
1. `BrowserService` is a singleton owned by `App`, started during `Startup`.
2. Chrome connection lifecycle: `Connect() → reuse | launch → ensureLabsTab → extractToken`. Idempotent; safe to call repeatedly.
3. Status state machine: `disconnected | connecting | needs_login | ready | error`. Emit `browserStatus` Wails event on every transition.
4. Token cache with TTL (default 30 min); refresh on demand or on 401 from API client.
5. Chrome path auto-detect on Win: `%PROGRAMFILES%/Google/Chrome/Application/chrome.exe`, `%PROGRAMFILES(X86)%/...`, `%LOCALAPPDATA%/Google/Chrome/Application/chrome.exe`. Fall back to `settings.chromePath` if non-empty.
6. Allow opening login flow: `OpenLoginFlow()` brings labs.google to front and waits for token presence.
7. Provide a `NewTab(url string)` helper used by Phase 06 (UI automation) and Phase 07 (download).
8. On shutdown, do NOT kill external Chrome we connected to (only kill the one we launched).

## Architecture
```
internal/browser/
├── service.go      // BrowserService struct + Start/Stop/Status
├── launcher.go     // detectChrome, launchOrReuse
├── token.go        // ExtractToken (eval JS, parse __NEXT_DATA__)
├── status.go       // status enum + Wails event emit helpers
└── errors.go       // typed errors (ErrNotLoggedIn, ErrTokenMissing, …)
```

### BrowserService fields
```go
type BrowserService struct {
    ctx       context.Context        // Wails ctx for events
    settings  *db.SettingsRepo
    mu        sync.RWMutex
    browser   *rod.Browser
    weOwn     bool                   // true if we launched chrome
    pid       int
    token     string
    tokenAt   time.Time
    status    Status
}
```

### Token extraction JS
```js
(function () {
  try {
    const el = document.getElementById('__NEXT_DATA__');
    if (!el) return { ok: false, reason: 'no_next_data' };
    const data = JSON.parse(el.textContent);
    const pp   = data?.props?.pageProps || {};
    // Probe candidate fields, in order of likelihood:
    const candidates = [
      pp?.session?.access_token,
      pp?.session?.accessToken,
      pp?.token,
      pp?.user?.accessToken,
      pp?.account?.accessToken,
    ];
    const tok = candidates.find(t => typeof t === 'string' && t.length > 20);
    return tok
      ? { ok: true, token: tok }
      : { ok: false, reason: 'token_field_missing', keys: Object.keys(pp) };
  } catch (e) { return { ok: false, reason: String(e) }; }
})()
```

## Related code files
- `internal/browser/service.go`
- `internal/browser/launcher.go`
- `internal/browser/token.go`
- `internal/browser/status.go`
- `internal/browser/errors.go`
- `app.go` — bind: `EnsureBrowser() (BrowserStatus, error)`, `OpenLoginFlow() error`, `RefreshToken() error`, `GetBrowserStatus() BrowserStatus`.

## Implementation steps
1. **`detectChrome()`** in `launcher.go` — return first existing path from candidate list; honor `settings.chromePath` override.
2. **`tryReuse(port int)`** — HTTP GET with 500ms timeout. On 200 parse `webSocketDebuggerUrl`; `rod.New().ControlURL(ws).Connect()`; ping with `browser.MustVersion()`.
3. **`launchFresh(chromePath, userDataDir, port)`** — `launcher.New().Bin(chromePath).Set("user-data-dir", userDataDir).Set("disable-blink-features", "AutomationControlled").Set("remote-debugging-port", port).Headless(false).Devtools(false).Leakless(true).XVFB(false).MustLaunch()` → `rod.New().ControlURL(wsURL).MustConnect()`. Track `pid` so shutdown can choose to kill it.
4. **`ensureLabsTab(b *rod.Browser)`** — find a page whose URL contains `labs.google/fx/tools/flow`; if none, open new with `stealth.Page(b)`, navigate, wait `LoadState`. Always wrap pages used downstream via `stealth.Page` (or `stealth.MustPage` on first nav).
5. **`extractToken(p *rod.Page)`** — evaluate the JS above via `p.Eval(...)`, type-assert `Value` to `proto.RuntimeRemoteObject` or use `gson` to walk the result. On `ok:false` return typed error; on success cache token + `tokenAt`.
6. **`Start(ctx)`** — set status `connecting` → run `tryReuse` → fallback `launchFresh` → `ensureLabsTab` → `extractToken`. Each transition emits `runtime.EventsEmit(ctx, "browserStatus", BrowserStatus{...})`.
7. **`OpenLoginFlow()`** — focus Chrome window via CDP `Browser.bringToFront`; status `needs_login`. Background goroutine polls `extractToken` every 3s for up to 5min.
8. **`Token()`** — RLock; if older than TTL or empty, call `extractToken`. Returns string token.
9. **`NewTab(url)`** — `b.MustPage(url)` wrapped via `stealth.Page` if it's a non-Google asset; for GCS download a vanilla page is fine since stealth only matters for detection.
10. **Shutdown** — if `weOwn`, `browser.MustClose()` then kill `pid` if still alive; else just `browser.MustIncognito` style detach.

## Todo list
- [ ] Detect Chrome path on Windows.
- [ ] Implement reuse-or-launch logic with port pinning.
- [ ] Wire `go-rod/stealth` to all newly created pages.
- [ ] Implement token JS evaluator with multi-path probe.
- [ ] Status state machine + Wails event emission.
- [ ] `OpenLoginFlow` polling.
- [ ] Bind methods to `App` for the frontend.
- [ ] Manual test: kill existing Chrome → app launches one; restart app → reuses; sign in → token captured.

## Success criteria
- Cold start: app launches Chrome with persistent profile, opens labs.google, surfaces `ready` once token captured.
- Killing Chrome externally flips status to `disconnected`; app reconnects on next API call.
- Token extraction returns ≥20-char string in normal conditions.
- No "Chrome is being controlled by automated test software" banner (stealth working).

## Risk assessment
- **__NEXT_DATA__ shape changes** → defensive probe + verbose logging of `Object.keys(pageProps)` so we can patch quickly.
- **Stealth not enough** → keep an "open Chrome visibly so user can solve any captcha" fallback already in `OpenLoginFlow`.
- **Multiple Chrome instances on user's machine** → we always pin `--remote-debugging-port`; users that already use that port get a clear error in Settings page.
- **Goroutine leaks** → all pollers tied to `ctx`, cancelled in `Shutdown`.

## Security considerations
- Token never written to disk or logs (mask all but last 6 chars).
- `userDataDir` contains user's Google session; document this in Settings; provide "Reset Profile" button (Phase 11) that deletes the dir.
- Never expose token to frontend bindings.

## Next steps
Phase 04 — Local File HTTP Handler (small + independent; can be done in parallel with Phase 05).
