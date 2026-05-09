package queue

// State enumerates the worker lifecycle as visible to the UI.
type State string

const (
	StateIdle     State = "idle"
	StateRunning  State = "running"
	StatePaused   State = "paused"
	StateStopping State = "stopping"
)
