# Google Labs Flow API — reverse-engineered

Captured 2026-05-09 from a real Chrome session at
`https://labs.google/fx/vi/tools/flow/project/<uuid>` via CDP network trace.

## Auth

Bearer token from `__NEXT_DATA__.props.pageProps.session.access_token`.
Format: `ya29.a0AQv...` (OAuth2 access token, ~417 chars).
User email at `__NEXT_DATA__.props.pageProps.session.user.email`.

## Required context

| Field | Source | Example |
|---|---|---|
| `projectId` | URL path `…/project/<uuid>` of any open Flow tab | `2c642102-6882-4db7-99f0-0d50e98d3805` |
| `batchId` | App-generated UUID, shared across all requests in one batch | `28c53393-049d-42a1-b5e6-a55ff8674336` |
| `sessionId` | App-generated, prefixed with `;`, then unix-ms | `;1778320882374` |
| `userPaygateTier` | From `GET /v1/credits` response | `PAYGATE_TIER_ONE` |
| `tool` | Constant | `"PINHOLE"` |

## 1. Submit (`POST` per prompt — 1 prompt = 1 request, but you can fan out multiple seeds with same batchId for N outputs)

```
POST https://aisandbox-pa.googleapis.com/v1/video:batchAsyncGenerateVideoText
Authorization: Bearer <token>
Content-Type: text/plain;charset=UTF-8
Accept: application/json
Origin: https://labs.google
Referer: https://labs.google/
```

Body:
```json
{
  "mediaGenerationContext": {
    "batchId": "<uuid>",
    "audioFailurePreference": "BLOCK_SILENCED_VIDEOS"
  },
  "clientContext": {
    "projectId": "<project-uuid>",
    "tool": "PINHOLE",
    "userPaygateTier": "PAYGATE_TIER_ONE",
    "sessionId": ";1778320882374"
  },
  "requests": [{
    "aspectRatio": "VIDEO_ASPECT_RATIO_LANDSCAPE",
    "textInput": { "structuredPrompt": { "parts": [{ "text": "<prompt>" }] } },
    "videoModelKey": "veo_3_1_t2v_fast",
    "metadata": {},
    "seed": 14441
  }],
  "useV2ModelConfig": true
}
```

Note observed in trace: web client also includes `clientContext.recaptchaContext.{token,applicationType}`. Per user's notes recaptcha is **optional** and the API responds 200 without it — to be confirmed by Go test.

Response:
```json
{
  "operations": [{
    "operation": {"name": "<media-id>"},
    "sceneId": "",
    "status": "MEDIA_GENERATION_STATUS_PENDING"
  }],
  "remainingCredits": 350,
  "workflows": [{
    "name": "<workflow-uuid>",
    "metadata": { "displayName": "...", "primaryMediaId": "<media-id>", "batchId": "...", ... },
    "projectId": "<project-uuid>"
  }],
  "media": [{
    "name": "<media-id>",
    "projectId": "<project-uuid>",
    "workflowId": "<workflow-uuid>",
    "video": { "generatedVideo": { "seed": 14441, "model": "veo_3_1_t2v_fast", ... } }
  }]
}
```

**Key fields to keep:** `media[i].name` (used as media id for poll) and `media[i].projectId`.

## 2. Poll (`POST` — batched check N media at once)

```
POST https://aisandbox-pa.googleapis.com/v1/video:batchCheckAsyncVideoGenerationStatus
Authorization: Bearer <token>
Content-Type: text/plain;charset=UTF-8
```

Body:
```json
{
  "media": [
    {"name": "<media-id-1>", "projectId": "<project-uuid>"},
    {"name": "<media-id-2>", "projectId": "<project-uuid>"}
  ]
}
```

Response: same shape as submit's `media[]`, with one of these statuses in `mediaMetadata.mediaStatus.mediaGenerationStatus`:
- `MEDIA_GENERATION_STATUS_PENDING`
- `MEDIA_GENERATION_STATUS_SUCCESSFUL`
- `MEDIA_GENERATION_STATUS_FAILED`

**Important:** the SUCCESSFUL response does **not** contain a video URL. Download is a separate step.

## 3. Download

The signed GCS URL is **not in any JSON response**. Pattern:

```
https://labs.google/fx/api/trpc/media.getMediaUrlRedirect?name=<media-id>
```

This is a tRPC route on labs.google itself. GET it from a logged-in Chrome session (cookies required) and it 302-redirects to a `https://*.googleusercontent.com/...` (or `storage.googleapis.com`) signed URL. From there a plain `http.Get` (no auth) downloads the mp4.

**This matches the existing Go `download.Resolve()` design exactly** — open the redirect URL in a browser tab, capture the final GCS URL via `Network.responseReceived`.

## 4. Credits (optional, useful)

```
GET https://aisandbox-pa.googleapis.com/v1/credits?key=AIzaSyBtrm0o5ab1c-Ec8ZuLcGt3oJAA5VWt3pY
Authorization: Bearer <token>
```

Response:
```json
{
  "credits": 370,
  "userPaygateTier": "PAYGATE_TIER_ONE",
  "sku": "G1_TIER1",
  "serviceTier": "SERVICE_TIER_INTERMEDIATE",
  "subscriptionCredits": 370
}
```

The `key` query param is a public client API key; the Bearer is also required.
Use the `userPaygateTier` from this response to populate `clientContext.userPaygateTier` for `submit`.

Each `veo_3_1_t2v_fast` video costs ~10 credits (370 → 350 after 2 outputs in trace).

## 5. Enums observed

- `videoModelKey`: `veo_3_1_t2v_fast`  ⚠️ NOT `veo_3_1_t2v_fast_ultra` (the original spec was wrong).
- `aspectRatio`: `VIDEO_ASPECT_RATIO_LANDSCAPE` (16:9). Need to test `VIDEO_ASPECT_RATIO_PORTRAIT` for 9:16.
- `tool`: `PINHOLE`.
- `clientPlatform` (in response only): `CLIENT_PLATFORM_WEB`.
- `videoGenerationMode` (in response only): `VIDEO_GENERATION_MODE_TEXT_TO_VIDEO`.

## 6. Outputs-per-prompt fanout

UI offers `1, x2, x3, x4`. Each output = ONE separate POST to `batchAsyncGenerateVideoText`,
each with **same** `batchId` and **different** `seed`. The trace shows 2 sequential
submits 2ms apart with same `batchId`.

Server returns one `media[i].name` per submit; collect them all then poll all in one
`batchCheckAsync...` call.
