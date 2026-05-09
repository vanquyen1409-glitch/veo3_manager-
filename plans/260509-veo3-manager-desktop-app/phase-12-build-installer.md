# Phase 12 — Build, Installer, Polish

## Context
- Parent: [plan.md](plan.md)
- Depends on: All previous phases.

## Overview
- **Date:** 2026-05-09
- **Description:** Production build, NSIS installer, app icon, version metadata, log file rotation, single-instance lock, error reporting plumbing, and final polish (DPI awareness, splash, performance pass).
- **Priority:** High — required to ship
- **Implementation status:** Pending
- **Review status:** Pending

## Key insights
- `wails build -nsis -webview2 embed -ldflags "-s -w -X main.version=…"` produces a portable + installer pair with the WebView2 bootstrapper bundled (~150 KB cost).
- Single-instance lock prevents two app instances fighting over SQLite + Chrome debug port. Use a Windows named mutex (`golang.org/x/sys/windows`).
- Log rotation: single file `%APPDATA%/veo3-manager/logs/app.log`, rotate at 5 MB, keep 5 archives. Use `gopkg.in/natefinch/lumberjack.v2`.
- DPI awareness on Windows: set in app manifest (Wails handles this when targeting Win10 SDK).
- Splash screen optional; given Startup is fast (<1s), skip.

## Requirements
1. Reproducible build script.
2. NSIS installer with: app icon, default install path `Program Files\veo3-manager`, Start Menu shortcut, optional desktop shortcut, uninstaller, WebView2 runtime check + install fallback.
3. Version metadata embedded; visible in Settings → Debug.
4. Single-instance lock at startup.
5. App data directory created on first run; documented in Settings.
6. Logs rotated; "Open Logs Folder" works.
7. Crash recovery: any panic in worker is recovered + reported via toast + log; app remains alive.
8. Performance budget at idle: <50 MB RAM after ready, <2% CPU.
9. README with Install / First Run / Troubleshooting sections.

## Architecture
```
build/
├── windows/
│   ├── icon.ico
│   ├── installer/
│   │   ├── project.nsi             // Wails-generated; we customize sections
│   │   └── wails_tools.nsh
│   └── info.json                    // Version / Company / FileDescription
├── scripts/
│   ├── build.ps1                    // wraps wails build
│   └── version.ps1                  // bumps version in info.json + main.go
internal/app/
├── singleinstance.go                // Windows mutex
└── log.go                           // lumberjack init
```

### Single-instance lock (Windows)
```go
//go:build windows
package app

import (
    "golang.org/x/sys/windows"
)

func AcquireSingleInstance(name string) (release func(), err error) {
    n, _ := windows.UTF16PtrFromString(`Global\` + name)
    h, err := windows.CreateMutex(nil, false, n)
    if err != nil { return nil, err }
    if windows.GetLastError() == windows.ERROR_ALREADY_EXISTS {
        windows.CloseHandle(h)
        return nil, ErrAlreadyRunning
    }
    return func() { windows.CloseHandle(h) }, nil
}
```

### Logger init
```go
func InitLogger(dir string) *slog.Logger {
    f := &lumberjack.Logger{
        Filename:   filepath.Join(dir, "app.log"),
        MaxSize:    5,   // MB
        MaxBackups: 5,
        Compress:   true,
    }
    return slog.New(slog.NewTextHandler(io.MultiWriter(os.Stdout, f), nil))
}
```

### `build.ps1`
```powershell
param([string]$Version = "0.1.0")
$env:CGO_ENABLED = "0"
wails build -nsis -webview2 embed `
  -ldflags "-s -w -X main.version=$Version -X main.buildDate=$(Get-Date -Format o)"
```

## Related code files
- `internal/app/singleinstance.go`, `internal/app/log.go`.
- `main.go` — call `AcquireSingleInstance` at top; init logger; pass to services.
- `build/windows/info.json` (Wails-managed).
- `build/scripts/build.ps1`.
- `README.md`.

## Implementation steps
1. **Logger** — wire `slog` + lumberjack; pass logger to all services via constructor.
2. **Single-instance lock** — call at very start of `main`; if `ErrAlreadyRunning`, show MessageBox "VEO3 Manager is already running" and exit.
3. **Panic recovery** — wrap worker goroutine in `recover()`; log + emit toast event; restart worker once after backoff.
4. **Crash dialog** — Go-side; if `Startup` panics, write to log and surface via `windows.MessageBox`.
5. **Version metadata** — set via `-X main.version`; expose in `GetDebugInfo()`.
6. **Icon** — drop `icon.ico` (multi-res 16/32/48/256) into `build/windows/icon.ico`.
7. **Build script** — `build.ps1` builds installer + portable. Output to `build/bin/`.
8. **Test installer**: clean Win10 VM (no WebView2) → install → first launch → log in → run a prompt end-to-end → uninstall → confirm clean removal except `%APPDATA%/veo3-manager` (intentionally kept; uninstaller offers checkbox to delete).
9. **README** sections: System requirements, Install, First Run (login), Configure output dir, Troubleshooting (Chrome not detected, login keeps redirecting, queue stuck), Privacy.
10. **Performance pass**: profile with `wails dev -devtools`; check React DevTools for unnecessary re-renders; close any leaked goroutines; verify idle RAM/CPU.

## Todo list
- [ ] Logger + log rotation.
- [ ] Single-instance lock.
- [ ] Panic recovery in worker + crash dialog.
- [ ] Embed version + build date.
- [ ] Build script (`build.ps1`).
- [ ] App icon.
- [ ] Installer test on clean Win10 VM.
- [ ] README.
- [ ] Performance pass (idle <50 MB RAM, <2% CPU).

## Success criteria
- Single .exe installer <30 MB; installs in <30s; works on Win10 21H2 + Win11.
- WebView2 auto-installs if missing.
- Second launch attempt shows the message box and does not double-run.
- 24h idle test: no memory growth, no zombie Chrome.
- Cold-start to "ready" status < 8s on a logged-in profile.

## Risk assessment
- **NSIS template drift** — if Wails updates change the template, regenerate and re-apply our customizations; keep our edits as a small patch file.
- **Antivirus false positives** on unsigned exe — document workaround; future: code signing.
- **WebView2 install requires admin** — installer must elevate or use per-user install path.

## Security considerations
- Installer signed in future (out of scope for v1; document in README).
- Uninstaller offers to wipe `%APPDATA%/veo3-manager` (containing Chrome profile + DB) but defaults to keep.
- No telemetry, no auto-update, no network calls beyond Google Labs.

## Next steps
Ship → collect user feedback → iterate (multi-account, headless mode, image-to-video).
