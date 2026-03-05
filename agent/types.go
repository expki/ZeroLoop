package agent

// FileEvent represents a file change event for WebSocket broadcasting.
type FileEvent struct {
	ProjectID string `json:"project_id"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Action    string `json:"action"` // "created", "changed", "deleted"
}
