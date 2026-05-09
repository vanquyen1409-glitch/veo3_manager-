# Phase 02 — Database Layer & Domain Models

## Context
- Parent: [plan.md](plan.md)
- Depends on: [Phase 01](phase-01-project-bootstrap.md)
- Research: [SQLite section](research/researcher-02-rod-sqlite-queue.md)

## Overview
- **Date:** 2026-05-09
- **Description:** Define the persistent data model (tasks, history items, app settings, generation defaults) and provide a thin repository layer using `modernc.org/sqlite`. Schema migrates idempotently at startup. Wails-bound query methods expose CRUD to the UI.
- **Priority:** Blocking — Queue Worker, History page, Settings persistence all need this
- **Implementation status:** Pending
- **Review status:** Pending

## Key insights
- `modernc.org/sqlite` is pure Go (no CGO) → ships in a single .exe. Driver name is `"sqlite"` not `"sqlite3"`.
- SQLite supports a single writer at a time — keep all writes on the queue worker goroutine OR serialize via `sync.Mutex` around `sql.DB`. Reads can be concurrent.
- Use `PRAGMA journal_mode=WAL` for concurrent read while worker writes.
- Video paths are `[]string`; store as JSON in a TEXT column to avoid join complexity for v1.
- Keep `tasks` (live queue) and `history` (completed) in one `tasks` table with `status` enum; "history" is just a filter — saves dual-writes.

## Requirements
1. DB lives at `%APPDATA%/veo3-manager/queue.db`. Create dir on first run.
2. Idempotent migrations on startup. v1 schema set, prepared for additive future migrations via `schema_version` table.
3. Models: `Task`, `GenerationConfig`, `Settings` (key/value), enums for `TaskStatus`.
4. Repository functions: enqueue, list (with filters/pagination), update status, save outputs, delete, requeue, stats.
5. Settings repo: get/set/all (typed accessors). Defaults baked in for first launch.
6. All exported methods on `App` returning `(value, error)` so they auto-bind for the frontend.

## Architecture
```
internal/db/
├── db.go             // Open(), migrate(), pragma setup
├── schema.go         // CREATE TABLE statements + version map
├── tasks.go          // TaskRepo
├── settings.go       // SettingsRepo
└── models.go         // Task, GenerationConfig, TaskStatus
```

### Schema (v1)
```sql
CREATE TABLE IF NOT EXISTS schema_version (version INTEGER PRIMARY KEY);

CREATE TABLE IF NOT EXISTS tasks (
  id            TEXT PRIMARY KEY,            -- uuid
  prompt        TEXT NOT NULL,
  config_json   TEXT NOT NULL,               -- {model, aspectRatio, outputCount, seedBase}
  status        TEXT NOT NULL,               -- pending|running|succeeded|failed|cancelled
  error         TEXT,
  media_ids     TEXT,                        -- JSON array of Google media ids
  video_paths   TEXT,                        -- JSON array of local file paths
  thumb_paths   TEXT,                        -- JSON array (optional)
  attempts      INTEGER NOT NULL DEFAULT 0,
  created_at    DATETIME NOT NULL,
  started_at    DATETIME,
  finished_at   DATETIME,
  source_task_id TEXT                        -- if requeued
);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at DESC);

CREATE TABLE IF NOT EXISTS settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
```

### TaskStatus enum
`pending | running | succeeded | failed | cancelled`

### GenerationConfig (Go)
```go
type GenerationConfig struct {
    Model        string `json:"model"`        // "veo_3_1_t2v_fast_ultra"
    AspectRatio  string `json:"aspectRatio"`  // "16:9" | "9:16"
    OutputCount  int    `json:"outputCount"`  // 1..4
    SeedBase     int64  `json:"seedBase"`     // 0 → randomize per video
}
```

## Related code files (to create)
- `internal/db/db.go`
- `internal/db/schema.go`
- `internal/db/models.go`
- `internal/db/tasks.go`
- `internal/db/settings.go`
- `app.go` — wire repos, expose: `EnqueuePrompts(prompts []string, cfg GenerationConfig) ([]Task, error)`, `ListTasks(filter ListFilter) ([]Task, error)`, `GetTask(id string) (Task, error)`, `RequeueTask(id string) (Task, error)`, `DeleteTask(id string) error`, `GetStats() (Stats, error)`, `GetSettings() (SettingsBundle, error)`, `SaveSettings(SettingsBundle) error`.

## Implementation steps
1. **`db.go`** — `Open(appDataDir string) (*sql.DB, error)`. Apply pragmas: `journal_mode=WAL`, `synchronous=NORMAL`, `foreign_keys=ON`, `busy_timeout=5000`. Then call `migrate(db)`.
2. **`schema.go`** — slice of `migration{ version int; up string }`; `migrate` reads current version, applies missing in tx.
3. **`models.go`** — define structs with JSON tags matching frontend expectations (camelCase).
4. **`tasks.go`** — `Enqueue(prompt string, cfg GenerationConfig) (Task, error)`, `BatchEnqueue([]string, GenerationConfig) ([]Task, error)`, `NextPending() (*Task, error)` (oldest pending), `UpdateStatus(id, status, errMsg)`, `SaveOutputs(id, mediaIDs []string, videoPaths []string)`, `List(filter)`, `Get(id)`, `Delete(id)`, `Requeue(id)`, `Stats()` (counts grouped by status, totals, today's success).
5. **`settings.go`** — `Get(key) (string, error)`, `GetTyped[T any](key, default) (T, error)` via JSON unmarshal, `Set(key, value)`, `GetBundle() (SettingsBundle)` returning all known keys with defaults applied.
6. **`SettingsBundle`** keys: `chromePath`, `userDataDir`, `outputDir`, `chromeDebugPort`, `defaultConfig` (GenerationConfig JSON), `pollIntervalMs`, `pollTimeoutMs`. Defaults: `chromePath=""` (auto-detect), `userDataDir=%APPDATA%/veo3-manager/chromedata`, `outputDir=%USERPROFILE%/Videos/VEO3`, `chromeDebugPort=9222`, `pollIntervalMs=10000`, `pollTimeoutMs=300000`.
7. **`app.go`** — instantiate `*sql.DB`, `TaskRepo`, `SettingsRepo` in `Startup`. Expose bound methods. Use a single `sync.Mutex` for write serialization in v1 (revisit if hot path).
8. **TS types** — after `wails dev` runs once, verify `frontend/wailsjs/go/main/App.d.ts` has the new methods.

## Todo list
- [ ] Implement `db.Open` + pragmas + migration loop.
- [ ] Encode v1 schema in `schema.go`.
- [ ] Implement `TaskRepo` (enqueue, list w/ filter, update, requeue, delete, stats).
- [ ] Implement `SettingsRepo` with typed bundle.
- [ ] Wire repos into `App` and expose bound methods.
- [ ] Manual smoke test: enqueue 2 tasks, restart app, list shows them.

## Success criteria
- Cold start creates `%APPDATA%/veo3-manager/queue.db` with WAL files.
- `EnqueuePrompts` from frontend returns persisted Task rows; data survives restart.
- `GetSettings` returns defaults on first run.
- No CGO build flags needed; `wails build` produces single .exe.

## Risk assessment
- **Locked DB on rapid writes** → `busy_timeout` + WAL. If still issues, gate writes behind worker mutex.
- **Schema drift** → version table + sequential up migrations.
- **JSON columns drift** → wrap marshal/unmarshal in repo, never let raw `[]byte` leak to UI.

## Security considerations
- DB has no secrets in v1. Bearer token is volatile (in-memory only).
- File paths normalized via `filepath.Clean` before storing.

## Next steps
Phase 03 — Browser Service.
