package labsapi

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

	// PathCredits returns remaining credits + paygate tier. The tier is
	// required as `clientContext.userPaygateTier` in submit calls.
	PathCredits = "/v1/credits?key=AIzaSyBtrm0o5ab1c-Ec8ZuLcGt3oJAA5VWt3pY"

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
