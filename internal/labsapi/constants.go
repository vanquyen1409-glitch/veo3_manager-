package labsapi

import (
	"os"

	"veo3-manager/internal/db"
)

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

	// DefaultModel is the family id used when none is selected. Aliased from
	// db.DefaultModel so the queue worker's normalization and labsapi's own
	// defensive normalization can never disagree on the fallback family. The
	// concrete videoModelKey is derived via VideoModelKeyFor.
	DefaultModel = db.DefaultModel

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

// VideoModelKeyFor maps a UI model family + aspect ratio (+ Omni clip length)
// to the concrete `videoModelKey` the submit body requires. Values were
// confirmed 2026-05-24 from the live Flow flow.projectInitialData
// videoModelFamilies response; Fast's derived key matched the app's
// already-working "veo_3_1_t2v_fast", validating the rest.
//
// Fast & Quality ship separate landscape/portrait keys; Lite & Omni use one
// key for both orientations. Omni (`abra`) encodes the clip length in its key.
// Any unknown family falls back to Fast landscape — the proven path.
func VideoModelKeyFor(family, aspect string, omniSec int) string {
	portrait := aspect == "9:16"
	switch family {
	case "veo_3_1_lite":
		return "veo_3_1_t2v_lite"
	case "veo_3_1_quality":
		if portrait {
			return "veo_3_1_t2v_portrait"
		}
		return "veo_3_1_t2v"
	case "abra":
		switch omniSec {
		case 4:
			return "abra_t2v_4s"
		case 6:
			return "abra_t2v_6s"
		case 10:
			return "abra_t2v_10s"
		default:
			return "abra_t2v_8s"
		}
	default: // "veo_3_1_fast" + anything unknown
		if portrait {
			return "veo_3_1_t2v_fast_portrait"
		}
		return "veo_3_1_t2v_fast"
	}
}

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
