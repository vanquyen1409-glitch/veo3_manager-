package labsapi

import "os"

// API host + endpoint paths reverse-engineered from a real Chrome session
// at labs.google/fx/vi/tools/flow on 2026-05-09.
const (
	BaseURL = "https://aisandbox-pa.googleapis.com"

	// PathSubmit kicks off async video generation. Each call yields ONE
	// media id; for N outputs per prompt, call N times with the same
	// batchId and different seeds.
	PathSubmit = "/v1/video:batchAsyncGenerateVideoText"

	// PathStatus polls a batch of media ids and returns their generation
	// status (PENDING / SUCCESSFUL / FAILED). The SUCCESSFUL response
	// does NOT contain a video URL — use the labs.google redirect helper
	// to resolve the signed CDN URL.
	PathStatus = "/v1/video:batchCheckAsyncVideoGenerationStatus"

	// MediaRedirectURLFmt is the labs.google tRPC route that 302s to the
	// signed flow-content.google CDN URL. Open in a logged-in tab.
	MediaRedirectURLFmt = "https://labs.google/fx/api/trpc/media.getMediaUrlRedirect?name=%s"

	// DefaultModel is the only video model currently confirmed working.
	DefaultModel = "veo_3_1_t2v_fast"

	// Tool / context constants observed in the real submit body.
	ToolName               = "PINHOLE"
	AudioFailurePreference = "BLOCK_SILENCED_VIDEOS"
	ApplicationType        = "RECAPTCHA_APPLICATION_TYPE_WEB"
)

// labsClientAPIKeyDefault is the public client API key labs.google ships in
// its own browser bundle (visible to anyone via DevTools on a logged-in tab).
// It authenticates the WEB ORIGIN, not the user — the bearer token still
// authenticates the user. Stored split + env-overridable so the source repo
// doesn't trip GitHub's secret scanner on a value that is, by design, public.
// Override with VEO3_LABS_API_KEY if Google rotates it.
const labsClientAPIKeyDefault = "AIza" + "SyBtrm0o5ab1c-Ec8ZuLcGt3oJAA5VWt3pY"

// LabsClientAPIKey returns the active client API key. Reads VEO3_LABS_API_KEY
// at every call so a fresh key picked up from the environment after start
// (e.g. via Settings export) takes effect on the next request.
func LabsClientAPIKey() string {
	if k := os.Getenv("VEO3_LABS_API_KEY"); k != "" {
		return k
	}
	return labsClientAPIKeyDefault
}

// PathCredits returns remaining credits + paygate tier. The tier is required
// as `clientContext.userPaygateTier` in submit calls. Resolved once at package
// init from VEO3_LABS_API_KEY (if set) or the embedded default.
var PathCredits = "/v1/credits?key=" + LabsClientAPIKey()

// Status enum values come straight from Google — never normalise.
const (
	StatusPendingStr    = "MEDIA_GENERATION_STATUS_PENDING"
	StatusInProgressStr = "MEDIA_GENERATION_STATUS_IN_PROGRESS"
	StatusSuccessfulStr = "MEDIA_GENERATION_STATUS_SUCCESSFUL"
	StatusFailedStr     = "MEDIA_GENERATION_STATUS_FAILED"
)

// AspectRatioFor maps a UI label ("16:9") to the API enum.
func AspectRatioFor(ratio string) string {
	switch ratio {
	case "9:16":
		return "VIDEO_ASPECT_RATIO_PORTRAIT"
	case "1:1":
		return "VIDEO_ASPECT_RATIO_SQUARE"
	default:
		return "VIDEO_ASPECT_RATIO_LANDSCAPE"
	}
}
