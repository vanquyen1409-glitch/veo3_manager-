# Manual Test — Crash Recovery

`ResetStaleRunning` flips any task stuck in `running` back to `pending` on app
startup. This guards against the case where the app is killed mid-task (kill
-9, BSOD, OOM). Without it, the orphan task would never be retried because
the queue worker only picks up `pending` rows.

This is impossible to reproduce inside a unit test (you can't kill the test
process and replay state cleanly), so we verify it manually after every
behaviour change to the worker loop or DB schema.

## Steps

1. **Boot the app** (`wails dev` or run a dev build).
2. **Submit a long-running task** — any prompt that goes through `submit ->
   poll`. Wait until you see the task transition to `running` in the UI.
3. **Kill the process forcefully** while it's still in `running`:
   - Windows: open Task Manager → find `veo3-manager.exe` → "End task"
     (NOT "End process tree" — we want the parent to die without graceful
     shutdown).
   - Or PowerShell: `Stop-Process -Name veo3-manager -Force`
   - Linux/macOS: `kill -9 $(pgrep veo3-manager)`
4. **Inspect the DB** to confirm the task is stuck on `running`:
   ```bash
   sqlite3 "$APPDATA/veo3-manager/queue.db" \
     "SELECT id, status FROM tasks ORDER BY started_at DESC LIMIT 1"
   ```
   - macOS: `~/Library/Application Support/veo3-manager/queue.db`
   - Linux: `~/.config/veo3-manager/queue.db`
5. **Restart the app** normally.
6. **Verify the task transitioned to `pending`** in the UI within ~1 second
   of startup. Same SQL query as step 4 should now show `pending`.
7. **Verify the worker re-picks it up** when you Resume / Start the queue.

## Expected log line

On startup `app.go` emits:

```
reset N stale running tasks -> pending
```

where N is the number of orphans recovered. If you see this on a clean
startup with N=0, all good — that's the no-op happy path.

## Failure modes to watch

| Symptom | Likely cause |
|---|---|
| Task stays `running` after restart | `ResetStaleRunning` not called, or DB write didn't commit (check sqlite WAL file) |
| Task flips to `pending` but `attempts` keeps growing per restart | Worker picks it up before user can review — by design, but if it's an infinite-failing task this can mask the real error. Mitigation: cap `attempts` in a future migration |
| App refuses to start with "database is locked" | Previous process still has a lock on `queue.db` — wait 5s for SQLite WAL recovery, or delete the WAL/SHM sidecars manually |

## Why no automated test for this

To genuinely test "process died, then restarted", you need:
1. A subprocess that opens the DB, inserts a `running` row, then `os.Exit(2)`
2. A second subprocess that opens the same DB and asserts the row is now `pending`

This is doable but adds a non-trivial harness for a 5-line code path that has not regressed in the last year of similar Wails+SQLite apps. Cost > benefit. Manual checklist wins.

If `ResetStaleRunning` ever changes (e.g. selectively requeues only certain
statuses), upgrade this checklist accordingly.
