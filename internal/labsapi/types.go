package labsapi

import "fmt"

// Status wraps the API's MEDIA_GENERATION_STATUS_* string.
type Status string

const (
	StatusPending    Status = StatusPendingStr
	StatusInProgress Status = StatusInProgressStr
	StatusSuccessful Status = StatusSuccessfulStr
	StatusFailed     Status = StatusFailedStr
)

// Terminal returns true if the status is settled.
func (s Status) Terminal() bool {
	return s == StatusSuccessful || s == StatusFailed
}

// MediaRef is a handle returned by Submit; pass back to Poll/Wait.
type MediaRef struct {
	MediaID   string `json:"mediaId"`
	ProjectID string `json:"projectId"`
	Seed      int64  `json:"seed"`
}

// FinalMedia is what Wait returns once a generation is SUCCESSFUL.
// VideoURL is intentionally empty — Labs does not include it in the poll
// response. Caller must use Downloader.Resolve(MediaID) → labs.google
// redirect → flow-content.google signed URL.
type FinalMedia struct {
	MediaRef
	Status Status `json:"status"`
}

// SubmitConfig is the per-batch input the queue worker passes in.
type SubmitConfig struct {
	Model       string  `json:"model"`        // "veo_3_1_t2v_fast"
	AspectRatio string  `json:"aspectRatio"`  // raw UI label "16:9" — converted via AspectRatioFor
	OutputCount int     `json:"outputCount"`  // 1..4
	Seeds       []int64 `json:"seeds,omitempty"`
}

// APIError surfaces non-2xx responses with the body for diagnostics.
type APIError struct {
	StatusCode int
	URL        string
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("labsapi %s: %d %s", e.URL, e.StatusCode, e.Message)
}
