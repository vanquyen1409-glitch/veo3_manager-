# VEO3 Manager

Windows desktop app that automates batch AI video generation against
[Google Labs Flow](https://labs.google/fx/tools/flow). Drop in a list of
prompts, press Start, and the app submits each prompt, polls until done,
and downloads every video to disk.

Built with Go + Wails v2 + WebView2 (backend) and React 18 + TypeScript +
Vite + Tailwind + Zustand (frontend).

## System requirements

- Windows 10 21H2+ or Windows 11
- Microsoft Edge WebView2 runtime (auto-installed by the installer)
- Google Chrome (for the automation profile)
- Logged-in Google account with access to Labs Flow

## First run

1. Launch VEO3 Manager. A frameless dark window opens.
2. The app launches Chrome with a persistent profile under
   `%APPDATA%/veo3-manager/chromedata`.
3. Sign in to Google in that Chrome window. The status dot in the sidebar
   turns green when the access token is captured.
4. Open **Queue**, choose model / aspect / outputs / output folder, paste
   prompts (one per line), press **Enqueue all**, then **Start**.

## Local data

| Path | Contents |
| --- | --- |
| `%APPDATA%/veo3-manager/queue.db` | SQLite (tasks, settings, history) |
| `%APPDATA%/veo3-manager/chromedata/` | Persistent Chrome profile |
| `%APPDATA%/veo3-manager/logs/app.log` | Rotated app log (5 MB x 5) |
| `%USERPROFILE%/Videos/VEO3/` | Default output folder for `.mp4` files |

## Build from source

```powershell
# Prereqs
go install github.com/wailsapp/wails/v2/cmd/wails@latest
cd frontend; npm install; cd ..

# Dev
wails dev

# Production build (NSIS installer + portable exe)
.\build\scripts\build.ps1 -Version 0.1.0
```

## Troubleshooting

- **Chrome not detected.** Set the path manually in Settings -> Chrome.
- **Browser status stuck on `needs_login`.** Click "Test connect" in
  Settings - Chrome should come to the front. Sign in and the token will
  be captured automatically.
- **Debug port in use.** Change `chromeDebugPort` in Settings (default 9222).
- **Videos won't preview.** Check the output folder is registered (it is
  added automatically; `Reset Chrome profile` does not affect it).

## Privacy

The app makes network calls only to:
- `aisandbox-pa.googleapis.com` - generation API (your bearer token)
- `storage.googleapis.com` / `googleusercontent.com` - to download
  generated videos

No analytics. No auto-update. All data stays on your machine.
