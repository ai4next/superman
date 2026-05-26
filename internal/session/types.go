package session

import (
	"time"

	adksession "google.golang.org/adk/session"
)

type EventType string

const (
	CreatedEvent EventType = "created"
	UpdatedEvent EventType = "updated"
	DeletedEvent EventType = "deleted"
)

type Event[T any] struct {
	Type    EventType `json:"type"`
	Payload T         `json:"payload"`
}

type MessageRole string

const (
	MessageUser      MessageRole = "user"
	MessageAssistant MessageRole = "assistant"
	MessageTool      MessageRole = "tool"
	MessageError     MessageRole = "error"
)

type Message struct {
	ID           string      `json:"id"`
	SessionID    string      `json:"session_id"`
	EventID      string      `json:"event_id"`
	InvocationID string      `json:"invocation_id"`
	Role         MessageRole `json:"role"`
	Content      string      `json:"content,omitempty"`
	ToolName     string      `json:"tool_name,omitempty"`
	ToolID       string      `json:"tool_id,omitempty"`
	Args         string      `json:"args,omitempty"`
	Result       string      `json:"result,omitempty"`
	Status       string      `json:"status,omitempty"`
	Summary      bool        `json:"summary,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

type FileAccess string

const (
	FileRead    FileAccess = "read"
	FileWritten FileAccess = "written"
)

type SessionFile struct {
	Path       string     `json:"path"`
	ReadAt     time.Time  `json:"read_at,omitempty"`
	WrittenAt  time.Time  `json:"written_at,omitempty"`
	ReadCount  int        `json:"read_count,omitempty"`
	WriteCount int        `json:"write_count,omitempty"`
	LastAccess FileAccess `json:"last_access,omitempty"`
}

type FileSnapshot struct {
	Hash      string `json:"hash,omitempty"`
	Size      int    `json:"size"`
	Preview   string `json:"preview,omitempty"`
	Missing   bool   `json:"missing,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

type FileRevision struct {
	ID        string       `json:"id"`
	Path      string       `json:"path"`
	Action    string       `json:"action"`
	Before    FileSnapshot `json:"before"`
	After     FileSnapshot `json:"after"`
	CreatedAt time.Time    `json:"created_at"`
}

type FileChangeSummary struct {
	File           SessionFile  `json:"file"`
	FirstRevision  FileRevision `json:"first_revision"`
	LatestRevision FileRevision `json:"latest_revision"`
	Additions      int          `json:"additions"`
	Deletions      int          `json:"deletions"`
}

type QueuedPrompt struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type SessionReference struct {
	SessionID string      `json:"session_id"`
	Role      MessageRole `json:"role,omitempty"`
	Preview   string      `json:"preview,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
}

type Metadata struct {
	AppName          string    `json:"app_name"`
	UserID           string    `json:"user_id"`
	SessionID        string    `json:"session_id"`
	Title            string    `json:"title"`
	MessageCount     int       `json:"message_count"`
	PromptTokens     int64     `json:"prompt_tokens"`
	CompletionTokens int64     `json:"completion_tokens"`
	SummaryMessageID string    `json:"summary_message_id,omitempty"`
	FileCount        int       `json:"file_count"`
	QueuedPrompts    int       `json:"queued_prompts"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type MessageSearchOptions struct {
	Query     string
	SessionID string
	Roles     []MessageRole
	Limit     int
}

type MessageSearchResult struct {
	Metadata Metadata `json:"metadata"`
	Message  Message  `json:"message"`
	Preview  string   `json:"preview"`
}

type ImportData struct {
	Metadata      Metadata
	Messages      []Message
	Files         []SessionFile
	FileRevisions []FileRevision
	PromptQueue   []QueuedPrompt
	References    []SessionReference
	Overwrite     bool
}

type StorageStats struct {
	RootDir                 string `json:"root_dir,omitempty"`
	Sessions                int    `json:"sessions"`
	Messages                int    `json:"messages"`
	Files                   int    `json:"files"`
	FileRevisions           int    `json:"file_revisions"`
	PromptQueue             int    `json:"prompt_queue"`
	References              int    `json:"references"`
	SessionBytes            int64  `json:"session_bytes"`
	SnapshotCount           int    `json:"snapshot_count"`
	SnapshotBytes           int64  `json:"snapshot_bytes"`
	ReferencedSnapshotCount int    `json:"referenced_snapshot_count"`
	ReferencedSnapshotBytes int64  `json:"referenced_snapshot_bytes"`
	OrphanSnapshotCount     int    `json:"orphan_snapshot_count"`
	OrphanSnapshotBytes     int64  `json:"orphan_snapshot_bytes"`
}

type SnapshotCleanupResult struct {
	DryRun       bool           `json:"dry_run"`
	Removed      int            `json:"removed"`
	RemovedBytes int64          `json:"removed_bytes"`
	Kept         int            `json:"kept"`
	KeptBytes    int64          `json:"kept_bytes"`
	Orphans      []SnapshotInfo `json:"orphans,omitempty"`
}

type SnapshotInfo struct {
	Hash string `json:"hash"`
	Path string `json:"path,omitempty"`
	Size int64  `json:"size"`
}

type storedSession struct {
	ID               int64                  `json:"id,omitempty"`
	AppName          string                 `json:"app_name"`
	UserID           string                 `json:"user_id"`
	SessionID        string                 `json:"session_id"`
	Title            string                 `json:"title"`
	State            map[string]any         `json:"state,omitempty"`
	Events           []*adksession.Event    `json:"events,omitempty"`
	Messages         []Message              `json:"messages,omitempty"`
	Files            map[string]SessionFile `json:"files,omitempty"`
	FileRevisions    []FileRevision         `json:"file_revisions,omitempty"`
	PromptQueue      []QueuedPrompt         `json:"prompt_queue,omitempty"`
	References       []SessionReference     `json:"references,omitempty"`
	PromptTokens     int64                  `json:"prompt_tokens"`
	CompletionTokens int64                  `json:"completion_tokens"`
	SummaryMessageID string                 `json:"summary_message_id,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

func (s *storedSession) setSessionState(key string, value any) {
	if s.State == nil {
		s.State = make(map[string]any)
	}
	s.State[key] = value
}

func (s *storedSession) snapshot(state map[string]any) *sessionView {
	return &sessionView{
		appName:   s.AppName,
		userID:    s.UserID,
		sessionID: s.SessionID,
		state:     state,
		events:    cloneEvents(s.Events),
		updatedAt: s.UpdatedAt,
	}
}
