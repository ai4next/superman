package expert

import "time"

// Status represents the lifecycle stage of an expert.
type Status string

const (
	StatusDraft    Status = "draft"
	StatusActive   Status = "active"
	StatusMature   Status = "mature"
	StatusArchived Status = "archived"
)

// CallMode represents how an expert was invoked.
type CallMode string

const (
	ModeConsult  CallMode = "consult"
	ModeDelegate CallMode = "delegate"
)

// Spec defines an expert agent's identity, trigger conditions, and capabilities.
type Spec struct {
	Name           string    `yaml:"name" json:"name"`
	Summary        string    `yaml:"summary" json:"summary"`
	Description    string    `yaml:"description" json:"description"`
	TriggerPattern string    `yaml:"trigger_pattern" json:"trigger_pattern"`
	ToolAllowlist  []string  `yaml:"tools" json:"tools"`
	SystemPrompt   string    `yaml:"prompt" json:"prompt"`
	Status         Status    `yaml:"status" json:"status"`
	Frequency      int       `yaml:"frequency" json:"frequency"`   // Total invocation count
	Confidence     float64   `yaml:"confidence" json:"confidence"` // 0.0 (low) to 1.0 (high)
	Version        int       `yaml:"version" json:"version"`
	PreviousID     string    `yaml:"previous_id,omitempty" json:"previous_id,omitempty"`
	CreatedAt      time.Time `yaml:"created_at" json:"created_at"`
	UpdatedAt      time.Time `yaml:"updated_at" json:"updated_at"`
}

// CallRecord is a single invocation log entry for an expert.
type CallRecord struct {
	Timestamp  time.Time `json:"timestamp"`
	TaskDesc   string    `json:"task_desc"`
	Mode       CallMode  `json:"mode"`
	Success    bool      `json:"success"`
	DurationMs int64     `json:"duration_ms"`
}
