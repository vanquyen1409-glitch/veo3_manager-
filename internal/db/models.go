package db

import "time"

// TaskStatus enumerates the lifecycle of a generation task.
type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusRunning   TaskStatus = "running"
	StatusSucceeded TaskStatus = "succeeded"
	StatusFailed    TaskStatus = "failed"
	StatusCancelled TaskStatus = "cancelled"
)

// GenerationConfig is a per-task snapshot of the user's batch settings.
// Stored as JSON in tasks.config_json so future setting tweaks don't mutate
// already-enqueued tasks.
type GenerationConfig struct {
	// Model is a video model FAMILY id (not the raw videoModelKey). The
	// concrete key is resolved at submit time from (family, aspectRatio,
	// OmniDurationSec) because Fast & Quality use different keys for
	// landscape vs portrait and Omni Flash encodes clip length in its key.
	// See db.NormalizeModelFamily + labsapi.VideoModelKeyFor.
	Model       string `json:"model"`
	AspectRatio string `json:"aspectRatio"`
	OutputCount int    `json:"outputCount"`
	// OmniDurationSec is the clip length in seconds for the Omni Flash
	// (`abra`) family: 4 | 6 | 8 | 10. Ignored by the other families, which
	// only ship a fixed 8 s clip. Defaults to 8.
	OmniDurationSec int   `json:"omniDurationSec"`
	SeedBase        int64 `json:"seedBase"`
	OutputDir       string `json:"outputDir"`
}

// Video model FAMILY ids stored in GenerationConfig.Model. These are the
// stable family ids from the Flow flow.projectInitialData videoModelFamilies
// response — NOT the per-aspect videoModelKey sent to the API (that's derived
// via labsapi.VideoModelKeyFor).
const (
	ModelFast    = "veo_3_1_fast"
	ModelLite    = "veo_3_1_lite"
	ModelQuality = "veo_3_1_quality"
	ModelOmni    = "abra"
	DefaultModel = ModelFast
)

// NormalizeModelFamily coerces any stored model value to a known family id.
// This also migrates legacy values that used the raw videoModelKey
// ("veo_3_1_t2v_fast", "..._ultra"), empty strings, and the old "<TBD:..."
// placeholders — all fall back to the default Fast family.
func NormalizeModelFamily(m string) string {
	switch m {
	case ModelFast, ModelLite, ModelQuality, ModelOmni:
		return m
	default:
		return DefaultModel
	}
}

// NormalizeOmniDuration clamps the Omni Flash clip length to a supported value
// (4/6/8/10 s), defaulting to 8 for empty/unknown input.
func NormalizeOmniDuration(s int) int {
	switch s {
	case 4, 6, 8, 10:
		return s
	default:
		return 8
	}
}

// Task is a single enqueued generation request and its outputs.
type Task struct {
	ID           string     `json:"id"`
	Prompt       string     `json:"prompt"`
	Config       GenerationConfig `json:"config"`
	Status       TaskStatus `json:"status"`
	Error        string     `json:"error,omitempty"`
	MediaIDs     []string   `json:"mediaIds,omitempty"`
	VideoPaths   []string   `json:"videoPaths,omitempty"`
	ThumbPaths   []string   `json:"thumbPaths,omitempty"`
	Attempts     int        `json:"attempts"`
	CreatedAt    time.Time  `json:"createdAt"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	FinishedAt   *time.Time `json:"finishedAt,omitempty"`
	SourceTaskID string     `json:"sourceTaskId,omitempty"`
}

// ListFilter is used by TaskRepo.List for the History + Queue views.
type ListFilter struct {
	Statuses []TaskStatus `json:"statuses,omitempty"`
	Search   string       `json:"search,omitempty"`
	From     *time.Time   `json:"from,omitempty"`
	To       *time.Time   `json:"to,omitempty"`
	Offset   int          `json:"offset,omitempty"`
	Limit    int          `json:"limit,omitempty"`
	OrderBy  string       `json:"orderBy,omitempty"` // "created_at" (default) | "finished_at"
	OrderDir string       `json:"orderDir,omitempty"` // "asc" | "desc" (default)
}

// Stats is the dashboard summary payload.
type Stats struct {
	Today          StatBucket   `json:"today"`
	Last7Days      StatBucket   `json:"last7Days"`
	AllTime        StatBucket   `json:"allTime"`
	RunningCount   int          `json:"runningCount"`
	PendingCount   int          `json:"pendingCount"`
	DailyCounts    []DailyCount `json:"dailyCounts"`
	AvgDurationMs  int64        `json:"avgDurationMs"`
	TotalVideos    int          `json:"totalVideos"`
}

// StatBucket holds counters for a time-window slice.
type StatBucket struct {
	Total     int `json:"total"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}

// DailyCount is one bar of the dashboard mini-chart.
type DailyCount struct {
	Date      string `json:"date"`
	Succeeded int    `json:"succeeded"`
	Failed    int    `json:"failed"`
}

// SettingsBundle exposes all user-tunable settings to the frontend.
type SettingsBundle struct {
	ChromePath       string           `json:"chromePath"`
	UserDataDir      string           `json:"userDataDir"`
	OutputDir        string           `json:"outputDir"`
	ChromeDebugPort  int              `json:"chromeDebugPort"`
	DefaultConfig    GenerationConfig `json:"defaultConfig"`
	PollIntervalMs   int              `json:"pollIntervalMs"`
	PollTimeoutMs    int              `json:"pollTimeoutMs"`
	MaxParallelDl    int              `json:"maxParallelDl"`
}
