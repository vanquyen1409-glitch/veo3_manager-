# Phase 01 — Project Bootstrap & Frameless Shell

## Context
- Parent: [plan.md](plan.md)
- Research: [Wails+Frontend](research/researcher-01-wails-frontend.md)
- Depends on: nothing (entry point)

## Overview
- **Date:** 2026-05-09
- **Description:** Initialize the Wails v2 project with the React+TS+Vite+Tailwind stack, configure a frameless window, wire up directory layout, set up dark-theme Tailwind, install all Go and frontend dependencies, and verify the app launches a blank dark window.
- **Priority:** Blocking — required by all other phases
- **Implementation status:** Pending
- **Review status:** Pending

## Key insights
- Wails generates TS bindings under `frontend/wailsjs/go/...` automatically when `Bind` is set; treat that directory as build output (gitignore).
- Frameless mode + transparent titlebar on Win10 can flicker; start with `frameless: true` only and add `transparentTitleBar` later if visuals demand it.
- WebView2 runtime must be present on Win10; ship installer with `-webview2 embed`.
- `wails dev` watches Go changes via `wails dev -frontenddevserverurl http://localhost:34115` so frontend Vite HMR works while Go rebuilds.

## Requirements
1. New Wails v2 project named `veo3-manager`, react-ts template.
2. Module path `github.com/<owner>/veo3-manager` (ask user before final path; placeholder `veo3-manager` until then).
3. Frontend deps: `react@18`, `react-dom`, `react-router-dom@6`, `tailwindcss@3`, `postcss`, `autoprefixer`, `zustand`, `lucide-react`, `clsx`, `tailwind-merge`.
4. Backend deps: `github.com/wailsapp/wails/v2`, `github.com/go-rod/rod`, `github.com/go-rod/stealth`, `modernc.org/sqlite`, `github.com/google/uuid`.
5. Tailwind `darkMode: 'class'`, `dark` class permanent on `<html>` (single dark theme requested).
6. Frameless window 1280×800, min 1024×640.
7. Working `wails dev` produces a window showing only the Tailwind-styled placeholder ("VEO3 Manager — booting…").

## Architecture
```
veo3-manager/
├── app.go                       // App struct + lifecycle hooks
├── main.go                      // wails.Run, options, Bind list
├── go.mod / go.sum
├── wails.json
├── build/                        // Wails-managed icons, NSIS templates
├── internal/
│   ├── app/                      // App-level wiring (will fill in later phases)
│   ├── config/                   // Settings struct + JSON file loader (later phase)
│   ├── db/                       // Phase 02
│   ├── browser/                  // Phase 03
│   ├── labsapi/                  // Phase 05
│   ├── automation/               // Phase 06
│   ├── download/                 // Phase 07
│   ├── queue/                    // Phase 08
│   └── server/                   // Phase 04 (asset middleware)
└── frontend/
    ├── package.json
    ├── vite.config.ts
    ├── tailwind.config.ts
    ├── postcss.config.js
    ├── tsconfig.json
    ├── index.html
    └── src/
        ├── main.tsx
        ├── App.tsx               // shell + routing (Phase 09)
        ├── pages/                // Phase 09–11
        ├── components/
        │   ├── TitleBar.tsx
        │   └── ui/               // Tailwind primitives
        ├── store/                // Zustand stores (Phase 09+)
        ├── lib/                  // utils, wailsjs re-exports
        └── styles/index.css
```

## Related code files (to create)
- `wails.json`
- `main.go`
- `app.go`
- `frontend/tailwind.config.ts`
- `frontend/postcss.config.js`
- `frontend/src/styles/index.css`
- `frontend/src/main.tsx`
- `frontend/src/App.tsx`
- `frontend/src/components/TitleBar.tsx`

## Implementation steps
1. **Install Wails CLI** locally if missing: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`. Verify with `wails doctor`.
2. **Init project** in repo root: `wails init -n veo3-manager -t react-ts -d .`. Move generated files to repo root (or initialize in subdir then mv).
3. **Edit `wails.json`** → set `"frontend:install": "npm install"`, `"frontend:build": "npm run build"`, `"frontend:dev:watcher": "npm run dev"`, `"frontend:dev:serverUrl": "auto"`.
4. **Edit `main.go`** with `options.App{ Title: "VEO3 Manager", Width: 1280, Height: 800, MinWidth: 1024, MinHeight: 640, Frameless: true, BackgroundColour: &options.RGBA{R:15,G:23,B:42,A:1}, Bind: []interface{}{app}, Windows: &windows.Options{WebviewIsTransparent: false, DisableWindowIcon: false} }`.
5. **`app.go`** — minimal `App` struct with `Startup`, `DOMReady`, `BeforeClose`, `Shutdown` (empty bodies + a `Greet(name string) string` placeholder so TS bindings generate).
6. **Frontend deps** in `frontend/`: `npm i react-router-dom zustand lucide-react clsx tailwind-merge && npm i -D tailwindcss postcss autoprefixer`. Then `npx tailwindcss init -p`.
7. **Tailwind config** — `content: ['./index.html','./src/**/*.{ts,tsx}']`, `darkMode: 'class'`, custom palette (slate-base + accent indigo/emerald/red).
8. **`index.css`** — Tailwind directives + base `body { @apply bg-slate-950 text-slate-100; }` and `html.dark` always set.
9. **`main.tsx`** — set `document.documentElement.classList.add('dark')` before render.
10. **`App.tsx`** — minimal shell rendering `<TitleBar/>` + `<main className="p-6">VEO3 Manager — booting…</main>`.
11. **`TitleBar.tsx`** — use inline style `{ '--wails-draggable': 'drag' }` on root, `'no-drag'` on buttons. Buttons call `WindowMinimise / WindowToggleMaximise / WindowClose` from `../wailsjs/runtime/runtime`.
12. **gitignore** — `build/bin/`, `frontend/dist/`, `frontend/node_modules/`, `frontend/wailsjs/`, `*.exe`, `*.db`, `chromedata/`.
13. Run `wails dev`. Confirm window opens, frameless, dark, custom titlebar buttons work.

## Todo list
- [ ] Install Wails CLI; `wails doctor` clean.
- [ ] Scaffold project (`wails init`).
- [ ] Configure `wails.json`, `main.go`, `app.go`.
- [ ] Install frontend deps, init Tailwind + PostCSS.
- [ ] Add Tailwind config + `index.css` dark base.
- [ ] Implement `TitleBar` with draggable region.
- [ ] Add `.gitignore`.
- [ ] Smoke test `wails dev`.

## Success criteria
- `wails dev` launches frameless window in <5s on Win10/11 with WebView2.
- Custom titlebar drag, minimize, maximize, close all work.
- Tailwind dark utilities apply (e.g. `bg-slate-950` visible).
- Generated `frontend/wailsjs/go/main/App.d.ts` includes `Greet`.
- `wails build` produces a working `.exe`.

## Risk assessment
- **WebView2 missing on Win10 dev box** → run `MicrosoftEdgeWebView2RuntimeInstaller.exe`.
- **Frameless drag region eats clicks** → forgot `no-drag` on inner buttons; verify each control reacts.
- **Tailwind purge removes classes** → ensure `content` glob covers all `.tsx`.

## Security considerations
- Pin dependency versions in `package.json` and `go.mod`; no `^` ranges for Wails/rod/stealth.
- No telemetry or analytics scripts.
- App data dir under `%APPDATA%/veo3-manager` (created in Phase 02); placeholder created here only if needed for Chrome profile later.

## Next steps
Phase 02 — Database Layer & Domain Models.
