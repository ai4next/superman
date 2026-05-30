package bus

import "time"

type EventType string

const (
	EventRunStarted          EventType = "run_started"
	EventRunFinished         EventType = "run_finished"
	EventRunFailed           EventType = "run_failed"
	EventRunCanceled         EventType = "run_canceled"
	EventTextDelta           EventType = "text_delta"
	EventToolCallStarted     EventType = "tool_call_started"
	EventToolCallFinished    EventType = "tool_call_finished"
	EventPermissionRequested EventType = "permission_requested"
	EventPermissionGranted   EventType = "permission_granted"
	EventPermissionDenied    EventType = "permission_denied"
	EventEvolutionStarted    EventType = "evolution_started"
	EventEvolutionFinished   EventType = "evolution_finished"
	EventEvolutionFailed     EventType = "evolution_failed"
	EventSessionCompacted    EventType = "session_compacted"

	EventTaskQueued    EventType = "task_queued"
	EventTaskStarted   EventType = "task_started"
	EventTaskSucceeded EventType = "task_succeeded"
	EventTaskFailed    EventType = "task_failed"
	EventTaskRetrying  EventType = "task_retrying"
	EventTaskDead      EventType = "task_dead"
)

type Event struct {
	ID        string    `json:"id,omitempty"`
	Type      EventType `json:"type"`
	At        time.Time `json:"at"`
	SessionID string    `json:"session_id,omitempty"`
	RunID     string    `json:"run_id,omitempty"`
	EventID   string    `json:"event_id,omitempty"`

	Text     string `json:"text,omitempty"`
	Author   string `json:"author,omitempty"`
	ToolID   string `json:"tool_id,omitempty"`
	ToolName string `json:"tool_name,omitempty"`
	Args     string `json:"args,omitempty"`
	Result   string `json:"result,omitempty"`
	Status   string `json:"status,omitempty"`
	Error    string `json:"error,omitempty"`
	Role     string `json:"role,omitempty"`
	Path     string `json:"path,omitempty"`
	Count    int    `json:"count,omitempty"`

	TaskID    string            `json:"task_id,omitempty"`
	WorkerID  string            `json:"worker_id,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	EventJSON string            `json:"-"`
}
