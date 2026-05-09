package browser

// Status describes the high-level state of the BrowserService.
type Status string

const (
	StatusDisconnected Status = "disconnected"
	StatusConnecting   Status = "connecting"
	StatusNeedsLogin   Status = "needs_login"
	StatusReady        Status = "ready"
	StatusError        Status = "error"
)

// Snapshot is the lightweight payload emitted on every status transition.
type Snapshot struct {
	Status      Status `json:"status"`
	Message     string `json:"message,omitempty"`
	ChromePath  string `json:"chromePath,omitempty"`
	UserDataDir string `json:"userDataDir,omitempty"`
	DebugPort   int    `json:"debugPort,omitempty"`
	WeOwn       bool   `json:"weOwn,omitempty"`
	TokenAgeMs  int64  `json:"tokenAgeMs,omitempty"`
	WsURL       string `json:"wsUrl,omitempty"`
	PID         int    `json:"pid,omitempty"`
}

// Detail is the heavier payload returned by Service.Detail() — used by the
// "Chrome chi tiết" card on the Settings page. Computed on demand because
// some fields (profile size, email, UA) require IO/CDP calls.
type Detail struct {
	Snapshot

	// Connection
	JsonVersionURL string `json:"jsonVersionUrl"`
	ConnectedAtMs  int64  `json:"connectedAtMs"`

	// Chrome
	ChromeVersion string `json:"chromeVersion,omitempty"`
	UserAgent     string `json:"userAgent,omitempty"`

	// Profile
	ProfileSizeBytes int64  `json:"profileSizeBytes,omitempty"`
	ProfileTabs      int    `json:"profileTabs,omitempty"`
	LoginEmail       string `json:"loginEmail,omitempty"`

	// Anti-bot
	StealthEnabled        bool     `json:"stealthEnabled"`
	BypassFlags           []string `json:"bypassFlags,omitempty"`
	StealthPatches        []string `json:"stealthPatches,omitempty"`
}
