# Phase 07 — Video Download Pipeline

## Context
- Parent: [plan.md](plan.md)
- Depends on: [Phase 03](phase-03-browser-service.md) (Chrome with cookies), [Phase 05](phase-05-google-labs-api-client.md) (`videoUrl` from FinalMedia).

## Overview
- **Date:** 2026-05-09
- **Description:** Take a `FinalMedia.VideoURL` (a Google redirect URL that requires the logged-in browser session), open it in a hidden Chrome tab, wait for the redirect chain to settle on a signed Google Cloud Storage URL, then download that GCS URL with a vanilla `http.Get` to local disk.
- **Priority:** Blocking — without download, queue produces nothing.
- **Implementation status:** Pending
- **Review status:** Pending

## Key insights
- The first URL is a redirect that requires Google's cookie set → bearer token alone fails. Only an authenticated browser tab can reach the signed GCS URL.
- The signed GCS URL itself is **public-by-signature** — once we have it we can download via plain HTTP without any cookies.
- Two viable capture mechanisms:
  1. **Read final URL after navigation.** Fast and simple but `MustNavigate` follows redirects automatically, so the page may end on the GCS URL itself or on a viewer page.
  2. **Hijack network response.** Hook `proto.NetworkResponseReceived` and capture the first 302 with a `https://*.googleapis.com/*` or `https://storage.googleapis.com/*` Location.
  - Use #2 by default (more reliable across UI changes); fall back to #1 reading `page.MustInfo().URL`.
- Always close the throwaway tab — long-running app must not leak tabs.
- **GCS signed URLs expire** (TTL unknown, treat as short). Download immediately after harvest; do not store the URL.

## Requirements
1. `Downloader` exposes:
   - `Resolve(ctx, redirectURL string) (gcsURL string, err error)`
   - `Download(ctx, gcsURL, dstPath string) error`
   - `Fetch(ctx, redirectURL, dstPath string) error` (Resolve + Download)
2. `Resolve` opens a tab in the live browser (via `BrowserService.NewTab`), captures redirect, closes tab.
3. `Download` writes atomically: stream into `<dstPath>.part`, rename on success.
4. Filenames: `<task-id>_<seed>_<timestamp>.mp4` under user's `outputDir`.
5. Concurrency: serialize at one download at a time inside the queue worker (avoid hammering GCS); allow up to 4 parallel downloads only when explicitly requested.

## Architecture
```
internal/download/
├── downloader.go    // Downloader struct + Fetch
├── resolve.go       // Tab + network-listener mechanism
├── http.go          // streaming http.Get with progress callback
└── filename.go      // sanitize + path build
```

### Resolve via NetworkResponseReceived
```go
func (d *Downloader) Resolve(ctx context.Context, redirectURL string) (string, error) {
    page, err := d.browser.NewTab(redirectURL)
    if err != nil { return "", err }
    defer page.Close()

    var gcsURL string
    done := make(chan struct{})

    page.EachEvent(func(e *proto.NetworkResponseReceived) bool {
        u := e.Response.URL
        if strings.Contains(u, "storage.googleapis.com/") ||
           strings.Contains(u, "googleusercontent.com/") {
            gcsURL = u
            close(done)
            return true // stop listening
        }
        return false
    })()

    select {
    case <-done:
        return gcsURL, nil
    case <-time.After(30 * time.Second):
        // Fallback: read final navigation URL
        if u := page.MustInfo().URL; strings.Contains(u, "googleapis.com") {
            return u, nil
        }
        return "", ErrGCSURLNotCaptured
    case <-ctx.Done():
        return "", ctx.Err()
    }
}
```

### Download with atomic rename
```go
func (d *Downloader) Download(ctx context.Context, url, dst string) error {
    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    resp, err := d.http.Do(req)
    if err != nil { return err }
    defer resp.Body.Close()
    if resp.StatusCode != 200 { return fmt.Errorf("status %d", resp.StatusCode) }

    tmp := dst + ".part"
    f, err := os.Create(tmp)
    if err != nil { return err }
    defer f.Close()

    if _, err := io.Copy(f, resp.Body); err != nil {
        os.Remove(tmp); return err
    }
    if err := f.Sync(); err != nil { return err }
    if err := f.Close(); err != nil { return err }
    return os.Rename(tmp, dst)
}
```

## Related code files
- `internal/download/*`
- `internal/queue/runner.go` (Phase 08) — orchestrates Resolve + Download per media.

## Implementation steps
1. Implement `Downloader` constructor that takes a `*browser.BrowserService` + `*http.Client`.
2. Implement `Resolve` with `EachEvent(NetworkResponseReceived)` listener; allow injecting URL predicate for testing.
3. Implement `Download` with atomic `.part` → rename, progress callback (bytes downloaded, total).
4. Filename helper: `BuildPath(outputDir, taskID, seed int64, ext string) string`. Sanitize Windows-illegal chars.
5. `Fetch` orchestrates and returns final path.
6. Manual test: trigger one generation end-to-end, confirm file size > 100 KB and plays in Media Player.

## Todo list
- [ ] Implement `Resolve` with response listener.
- [ ] Implement `Download` with atomic write.
- [ ] Filename builder + sanitizer.
- [ ] Wire `Fetch` and unit-test against `httptest` for `Download` part.
- [ ] End-to-end manual: real prompt → file on disk → plays.

## Success criteria
- Capture of signed GCS URL succeeds within 30s for normal connections.
- Download of a 720p ~5MB clip completes in <10s on 50 Mbps.
- Interrupting via ctx cancel deletes `.part` file.
- Atomic rename leaves no half-written `.mp4`.

## Risk assessment
- **GCS URL TTL expiry** — Resolve and Download must run back-to-back. If Download fails with 403/404, retry from Resolve once.
- **Tab leak on panic** — `defer page.Close()` and use `recover` in goroutine.
- **Network listener fires for unrelated requests** — predicate is strict (`googleapis.com/.../o/...` style); log if multiple matches.
- **Large files (>1GB)** — stream, never buffer in memory.

## Security considerations
- Validate `dst` is inside `outputDir` (reuse Phase 04 root logic).
- Don't log full GCS URL (it's effectively a credential).

## Next steps
Phase 08 — Queue Worker (orchestrates 03 + 05 + 07).
