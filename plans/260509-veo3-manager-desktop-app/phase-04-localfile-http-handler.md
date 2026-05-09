# Phase 04 — Local File HTTP Handler

## Context
- Parent: [plan.md](plan.md)
- Depends on: [Phase 01](phase-01-project-bootstrap.md)
- Research: [Wails AssetServer Middleware](research/researcher-01-wails-frontend.md#3-custom-http-middleware-for-local-files)

## Overview
- **Date:** 2026-05-09
- **Description:** Register a Wails `AssetServer` handler that serves any whitelisted local file under URL pattern `/localfile/<encoded-absolute-path>`. Used by the History page and Queue progress carousel to play locally-stored MP4s in `<video>` tags.
- **Priority:** Blocking for Phase 11 (History/preview); non-blocking for queue logic
- **Implementation status:** Pending
- **Review status:** Pending

## Key insights
- WebView cannot load `file://` URLs from a Wails app; we must serve via HTTP.
- AssetServer middleware sees every WebView request; we intercept the prefix and pass others through.
- HTTP `Range` requests are required for `<video>` seeking. `http.ServeFile` already handles this — no need to re-implement.
- Path traversal is the main risk: only serve files inside `outputDir` (or any explicitly registered directory).

## Requirements
1. URL pattern: `/localfile/{base64url-of-abs-path}` (avoid Windows drive-letter slash issues).
2. Handler reads base64url-decoded path, validates it lives under one of the registered roots, then `http.ServeFile`.
3. Roots are registered at startup: `outputDir`, plus any future thumbnail dir.
4. Return `403` for paths outside roots; `404` for missing.
5. Frontend helper: `localFileURL(absPath: string) → /localfile/<base64url>`.

## Architecture
```
internal/server/
├── localfile.go       // Handler + path safety
└── middleware.go      // Wraps next AssetServer handler
```

### Handler skeleton
```go
type LocalFileHandler struct {
    roots []string // absolute, normalized
}

func (h *LocalFileHandler) allowed(p string) bool {
    abs, err := filepath.Abs(filepath.Clean(p))
    if err != nil { return false }
    for _, r := range h.roots {
        if strings.HasPrefix(strings.ToLower(abs), strings.ToLower(r)+string(filepath.Separator)) || strings.EqualFold(abs, r) {
            return true
        }
    }
    return false
}

func (h *LocalFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    enc := strings.TrimPrefix(r.URL.Path, "/localfile/")
    raw, err := base64.RawURLEncoding.DecodeString(enc)
    if err != nil { http.Error(w, "bad path", 400); return }
    p := string(raw)
    if !h.allowed(p) { http.Error(w, "forbidden", 403); return }
    if _, err := os.Stat(p); err != nil { http.Error(w, "not found", 404); return }
    w.Header().Set("Cache-Control", "no-store") // mp4 should not stick in cache
    http.ServeFile(w, r, p)
}
```

### Wails wiring
```go
app := NewApp()
local := &server.LocalFileHandler{ Roots: []string{outputDir} }

middleware := func(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if strings.HasPrefix(r.URL.Path, "/localfile/") {
            local.ServeHTTP(w, r); return
        }
        next.ServeHTTP(w, r)
    })
}

wails.Run(&options.App{
    AssetServer: &assetserver.Options{
        Assets:     embeddedFS,
        Middleware: assetserver.Middleware(middleware),
    },
    // …
})
```

### Frontend helper (TS)
```ts
export function localFileURL(absPath: string) {
  const b64 = btoa(unescape(encodeURIComponent(absPath)))
    .replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
  return `/localfile/${b64}`;
}
```

## Related code files
- `internal/server/localfile.go`
- `internal/server/middleware.go`
- `main.go` — wire middleware into `AssetServer.Options`.
- `frontend/src/lib/url.ts` — `localFileURL`.

## Implementation steps
1. Implement `LocalFileHandler` with `roots` list and `allowed` predicate (case-insensitive on Windows).
2. Implement middleware wrapper as above.
3. Wire middleware into `wails.Run` options.
4. Add `RegisterRoot(path string)` so other modules can extend during startup (e.g. thumbnail dir).
5. Frontend: `localFileURL` util + use it from a tiny `<VideoPlayer src={localFileURL(p)} />` component (one-liner around `<video controls>`).
6. Smoke test: drop an mp4 in `outputDir`, hardcode a `<video src={localFileURL(...)}/>` in `App.tsx`, confirm playback + scrubbing.

## Todo list
- [ ] Implement handler with base64url path decoding.
- [ ] Implement allowlist root validation.
- [ ] Wire AssetServer middleware in `main.go`.
- [ ] Add `localFileURL` TS helper.
- [ ] Smoke test playback + Range scrubbing.

## Success criteria
- `<video>` plays a 100MB+ mp4 with smooth seeking (Range requests visible in DevTools).
- Path outside `outputDir` returns 403.
- Non-existent path returns 404.
- No CORS warnings.

## Risk assessment
- **Path traversal** mitigated via root allowlist + `filepath.Clean`.
- **Concurrency** — `http.ServeFile` is safe; no shared mutable state.
- **Long Windows paths** (>260) — `\\?\` prefix handled by `filepath.Abs` on modern Windows; document the limit if encountered.

## Security considerations
- Roots strictly limited to user-controlled output paths.
- No directory listing; only single-file serving.
- Cache disabled to avoid stale views after re-download.

## Next steps
Phase 05 — Google Labs API Client.
