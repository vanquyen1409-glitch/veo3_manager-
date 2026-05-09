# VEO3 Manager — Windows Desktop App

**Date:** 2026-05-09
**Status:** Planned (awaiting review)
**Stack:** Go 1.22+ / Wails v2 / WebView2 / go-rod + stealth / modernc SQLite • React 18 / Vite / TS / Tailwind dark / Zustand / Lucide

## Goal
Automate batch AI-video creation against `labs.google`. User logs into Google once via embedded Chrome, then enters prompts and presses Start. App submits prompts to the unofficial `aisandbox-pa.googleapis.com/v1` API using a bearer token scraped from `__NEXT_DATA__`, polls until `MEDIA_GENERATION_STATUS_SUCCESSFUL`, opens each redirect URL in a fresh Chrome tab to harvest the signed GCS URL, then downloads MP4s to disk. SQLite persists queue + history. Frameless React UI (Dashboard / Queue / History / Settings) shows live progress.

## Critical constraints (from prompt — treat as ground truth)
- Token = `window.__NEXT_DATA__` field, not API key. Only obtainable in a real signed-in Chrome page.
- Stealth + `--disable-blink-features=AutomationControlled` mandatory or Google blocks.
- Persistent `--user-data-dir` keeps Google login. Reuse running Chrome via `localhost:9222/json/version` before launching new.
- Slate.js prompt editor → must use CDP `Input.insertText` (typing fails).
- Submit button: filter `y > 680` (multiple buttons share label).
- Settings dropdowns: open via `aria-haspopup="menu"` button containing `crop_`. Aspect/count = `role="tab"` `data-state`. Model = `role="menuitem"`.
- Download: cannot fetch with bearer; open redirect URL in new Chrome tab → final GCS URL → plain `http.Get`.
- Success status string = `MEDIA_GENERATION_STATUS_SUCCESSFUL` (not `COMPLETED`).
- Only working model: `veo_3_1_t2v_fast_ultra`. Polling every 10s, 5min timeout. 1–4 outputs per prompt with random seed.
- WebView cannot read filesystem → custom HTTP handler `/localfile/{path}` for video preview.
- Config edits in UI must apply on next task without restarting queue (mutex-guarded read).

## Phases

| # | Phase | Status |
|---|---|---|
| 01 | [Project Bootstrap & Frameless Shell](phase-01-project-bootstrap.md) | Pending |
| 02 | [Database Layer & Domain Models](phase-02-database-models.md) | Pending |
| 03 | [Browser Service (rod + stealth + token)](phase-03-browser-service.md) | Pending |
| 04 | [Local File HTTP Handler](phase-04-localfile-http-handler.md) | Pending |
| 05 | [Google Labs API Client (submit + poll)](phase-05-google-labs-api-client.md) | Pending |
| 06 | [UI Automation Helpers (Slate.js + dropdowns)](phase-06-ui-automation-helpers.md) | Pending |
| 07 | [Video Download Pipeline](phase-07-video-download.md) | Pending |
| 08 | [Queue Worker (pause/resume/stop, hot-reload)](phase-08-queue-worker.md) | Pending |
| 09 | [Frontend Shell + Routing + Toast](phase-09-frontend-shell.md) | Pending |
| 10 | [Frontend Queue Page](phase-10-frontend-queue.md) | Pending |
| 11 | [Frontend Dashboard / History / Settings](phase-11-frontend-dashboard-history-settings.md) | Pending |
| 12 | [Build, Installer, Polish](phase-12-build-installer.md) | Pending |

## Research
- [Wails + Frontend stack](research/researcher-01-wails-frontend.md)
- [go-rod + stealth + SQLite + queue patterns](research/researcher-02-rod-sqlite-queue.md)

## Top risks
1. `__NEXT_DATA__` shape may shift — extraction must be defensive (try multiple paths, fail loudly).
2. Stealth alone may not be enough — ship a "open visible Chrome and let me solve any captcha" fallback flow.
3. GCS signed URL TTL unknown — download immediately upon harvest.
4. Wails frameless on Win10 + transparent titlebar can render artifacts — fallback to non-transparent if needed.
5. `wails dev` debug port (9222 if defaulted) can collide with our Chrome debug port — pin both explicitly.

## Out of scope (v1)
Multi-account, headless mode, image-to-video, scheduling, cloud sync, telemetry.
