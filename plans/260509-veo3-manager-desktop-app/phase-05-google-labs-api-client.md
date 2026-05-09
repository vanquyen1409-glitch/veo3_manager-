# Phase 05 — Google Labs API Client

## Context
- Parent: [plan.md](plan.md)
- Depends on: [Phase 03](phase-03-browser-service.md) (token), [Phase 02](phase-02-database-models.md) (config).
- Research: API constants come from the user-supplied prompt (treated as ground truth).

## Overview
- **Date:** 2026-05-09
- **Description:** Thin HTTP client for the unofficial `aisandbox-pa.googleapis.com/v1` API. Two operations: submit a generation (returns media IDs) and poll status. Handles auth header, JSON encoding, retry on 401 (refresh token then retry once), success status `MEDIA_GENERATION_STATUS_SUCCESSFUL`.
- **Priority:** Blocking — queue cannot run without it
- **Implementation status:** Pending
- **Review status:** Pending

## Key insights
- Bearer token comes from `BrowserService.Token()`; on 401, call `RefreshToken()` and retry exactly once.
- Each prompt fans out into 1–4 generations with **distinct random seeds**. The API may accept seed per-request or accept count + auto-seed; we write the call so the seed list is provided to keep it deterministic from our side.
- Status polling: 10s interval, 5 min total timeout. Aborts cleanly on `context.Cancel`.
- Error categorization: 4xx (client/data), 5xx (transient → retry once with backoff), network errors (transient), unknown status (treat as failure).
- We only ship the constant model `veo_3_1_t2v_fast_ultra` until other models are confirmed working.

## Requirements
1. `Client` struct holds an `*http.Client` (timeouts: 30s connect, 60s response), token-getter callback, refresh callback, base URL configurable for tests.
2. `Submit(ctx, prompt, cfg) ([]MediaRef, error)` — returns one ref per video to poll.
3. `Poll(ctx, ref MediaRef) (Status, error)` — single-call status check.
4. `Wait(ctx, ref MediaRef) (FinalMedia, error)` — loops `Poll` every 10s until success / failure / timeout / ctx cancel.
5. Returns are typed; never `map[string]any` to UI.
6. All requests include identical headers as a real Chrome session: `Authorization: Bearer …`, `Content-Type: application/json`, `User-Agent` matching the active Chrome page (queried once from `BrowserService`).

## Architecture
```
internal/labsapi/
├── client.go        // Client, NewClient, do() with auth + retry
├── submit.go        // Submit + request payload builder
├── poll.go          // Poll, Wait
├── types.go         // MediaRef, Status, FinalMedia, ApiError
└── constants.go     // BaseURL, endpoints, success status
```

### Endpoints (URL paths placeholders — confirm exact paths during impl)
- `POST /v1/sandbox/generate` — body example:
  ```json
  {
    "prompt": "<text>",
    "model": "veo_3_1_t2v_fast_ultra",
    "aspectRatio": "16:9",
    "outputCount": 4,
    "seeds": [12345, 23456, 34567, 45678]
  }
  ```
  Returns: `{ "mediaIds": ["m_abc","m_def",…], "operationId": "op_x" }`.
- `POST /v1/sandbox/status` — body `{ "mediaId": "m_abc" }`. Returns: `{ "status": "MEDIA_GENERATION_STATUS_SUCCESSFUL", "videoUrl": "https://…", "thumbnailUrl": "…" }` or `{ "status": "MEDIA_GENERATION_STATUS_PENDING" }` etc.
- **Important:** The exact endpoint paths and field names must be reverse-engineered from a real Chrome session DevTools→Network on the first run; the placeholders above match the prompt's description but verify before locking strings.

### Types
```go
type Status string
const (
    StatusPending    Status = "MEDIA_GENERATION_STATUS_PENDING"
    StatusInProgress Status = "MEDIA_GENERATION_STATUS_IN_PROGRESS"
    StatusSuccess    Status = "MEDIA_GENERATION_STATUS_SUCCESSFUL"
    StatusFailed     Status = "MEDIA_GENERATION_STATUS_FAILED"
)

type MediaRef struct {
    MediaID     string `json:"mediaId"`
    Seed        int64  `json:"seed"`
    OperationID string `json:"operationId,omitempty"`
}

type FinalMedia struct {
    MediaRef
    VideoURL    string
    ThumbURL    string
}
```

## Related code files
- `internal/labsapi/client.go` etc.
- `internal/queue/runner.go` (Phase 08) — calls `Submit` then `Wait`.

## Implementation steps
1. **`Client.do(req)`** — set Bearer header (token from callback), set UA, send. On 401: invalidate token, call refresh callback, retry once. On 5xx: one retry with 1s sleep. On 2xx with non-JSON body: error.
2. **`Submit`** — build body with prompt + cfg + N random seeds (`crypto/rand` int63). Decode JSON to a typed struct, then convert to `[]MediaRef` (one per mediaId).
3. **`Poll`** — POST to status endpoint, decode `{status,videoUrl,thumbnailUrl}`. On `Success`, populate `FinalMedia`.
4. **`Wait`** — `time.NewTicker(pollInterval)`; loop reads from ticker + ctx; per tick call `Poll`. Bail on terminal status, ctx done, or `5m` overall.
5. **Error type** — `ApiError{StatusCode int; Code string; Message string; Raw []byte}` with `Error()` method.
6. **Logging** — every request logs method, path, status, duration; bodies redacted. Use `log/slog`.
7. **Tests** — table-driven against an `httptest.Server` that mimics the API. Mock 401 → token refresh → 200 path explicitly.

## Todo list
- [ ] Define types + constants.
- [ ] Implement `do()` with auth, refresh, retry semantics.
- [ ] Implement `Submit` with seed generation.
- [ ] Implement `Poll` and `Wait` (ticker + ctx).
- [ ] Implement typed `ApiError`.
- [ ] Unit tests against `httptest.Server`.
- [ ] Manual run: with valid token, submit a real prompt, watch logs through to success.

## Success criteria
- Real prompt submission against real labs.google returns ≥1 mediaId.
- `Wait` returns `FinalMedia` with non-empty `VideoURL` within 5min.
- 401 path triggers refresh + retry without surfacing to caller.
- Unit tests pass; race detector clean.

## Risk assessment
- **Endpoint or field rename by Google** → centralize all URL/field strings in `constants.go` + `types.go` so a 1-file patch fixes them. On unknown response shape, log full raw body (with token redacted).
- **Rate limiting** → polling at 10s is conservative; if 429 returned, double interval up to 60s.
- **Long-running prompts** > 5 min → expose `pollTimeoutMs` setting so user can extend.

## Security considerations
- Token never logged.
- HTTPS enforced; refuse any non-HTTPS base URL.
- Body length cap (e.g. 10 MiB) on responses to avoid memory abuse.

## Next steps
Phase 06 — UI Automation Helpers (Slate.js prompt + dropdown selection); used as fallback or for in-page settings sync.
