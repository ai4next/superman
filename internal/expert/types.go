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
	Name           string        `yaml:"name" json:"name"`
	Summary        string        `yaml:"summary" json:"summary"`
	Description    string        `yaml:"description" json:"description"`
	Domain         string        `yaml:"domain,omitempty" json:"domain,omitempty"`
	Capabilities   []string      `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
	TriggerPattern string        `yaml:"trigger_pattern" json:"trigger_pattern"`
	ToolAllowlist  []string      `yaml:"tools" json:"tools"`
	SystemPrompt   string        `yaml:"prompt" json:"prompt"`
	InputContract  []string      `yaml:"input_contract,omitempty" json:"input_contract,omitempty"`
	OutputContract []string      `yaml:"output_contract,omitempty" json:"output_contract,omitempty"`
	RoutingPolicy  RoutingPolicy `yaml:"routing_policy,omitempty" json:"routing_policy,omitempty"`
	Status         Status        `yaml:"status" json:"status"`
	Frequency      int           `yaml:"frequency" json:"frequency"`   // Total invocation count
	Confidence     float64       `yaml:"confidence" json:"confidence"` // 0.0 (low) to 1.0 (high)
	Version        int           `yaml:"version" json:"version"`
	PreviousID     string        `yaml:"previous_id,omitempty" json:"previous_id,omitempty"`
	CreatedBy      string        `yaml:"created_by,omitempty" json:"created_by,omitempty"`
	Evidence       []string      `yaml:"evidence,omitempty" json:"evidence,omitempty"`
	Metrics        Metrics       `yaml:"metrics,omitempty" json:"metrics,omitempty"`
	CreatedAt      time.Time     `yaml:"created_at" json:"created_at"`
	UpdatedAt      time.Time     `yaml:"updated_at" json:"updated_at"`
}

// RoutingPolicy controls lightweight online expert routing.
type RoutingPolicy struct {
	MinConfidence float64    `yaml:"min_confidence,omitempty" json:"min_confidence,omitempty"`
	CooldownSec   int        `yaml:"cooldown_sec,omitempty" json:"cooldown_sec,omitempty"`
	Modes         []CallMode `yaml:"modes,omitempty" json:"modes,omitempty"`
}

// Metrics stores the latest persisted expert quality counters.
type Metrics struct {
	TotalCalls    int       `yaml:"total_calls,omitempty" json:"total_calls,omitempty"`
	SuccessCalls  int       `yaml:"success_calls,omitempty" json:"success_calls,omitempty"`
	SuccessRate   float64   `yaml:"success_rate,omitempty" json:"success_rate,omitempty"`
	AvgDurationMs float64   `yaml:"avg_duration_ms,omitempty" json:"avg_duration_ms,omitempty"`
	LastUsed      time.Time `yaml:"last_used,omitempty" json:"last_used,omitempty"`
}

// CallRecord is a single invocation log entry for an expert.
type CallRecord struct {
	Timestamp  time.Time `json:"timestamp"`
	TaskDesc   string    `json:"task_desc"`
	Mode       CallMode  `json:"mode"`
	Success    bool      `json:"success"`
	DurationMs int64     `json:"duration_ms"`
}

// ExpertTask is the structured task package passed to an isolated expert.
type ExpertTask struct {
	Task           string   `json:"task"`
	Goal           string   `json:"goal,omitempty"`
	Constraints    []string `json:"constraints,omitempty"`
	AvailableTools []string `json:"available_tools,omitempty"`
	RelevantFiles  []string `json:"relevant_files,omitempty"`
	MemoryRefs     []string `json:"memory_refs,omitempty"`
	ExpectedOutput string   `json:"expected_output,omitempty"`
}

// ExpertResult is the structured result returned from a delegated expert call.
type ExpertResult struct {
	Success      bool     `json:"success"`
	Summary      string   `json:"summary,omitempty"`
	Findings     []string `json:"findings,omitempty"`
	ActionsTaken []string `json:"actions_taken,omitempty"`
	FilesTouched []string `json:"files_touched,omitempty"`
	ToolCalls    int      `json:"tool_calls,omitempty"`
	Confidence   float64  `json:"confidence,omitempty"`
	Risks        []string `json:"risks,omitempty"`
	NextSteps    []string `json:"next_steps,omitempty"`
	RawResponse  string   `json:"raw_response,omitempty"`
}

// EvolutionAction describes an automatic expert lifecycle change.
type EvolutionAction string

const (
	EvolutionCreate   EvolutionAction = "create"
	EvolutionActivate EvolutionAction = "activate"
	EvolutionOptimize EvolutionAction = "optimize"
	EvolutionReplace  EvolutionAction = "replace"
	EvolutionArchive  EvolutionAction = "archive"
)

// EvolutionRecord is an audit log entry for automatic expert evolution.
type EvolutionRecord struct {
	Timestamp time.Time       `json:"timestamp"`
	Action    EvolutionAction `json:"action"`
	Expert    string          `json:"expert"`
	Version   int             `json:"version,omitempty"`
	Reason    string          `json:"reason"`
	Evidence  []string        `json:"evidence,omitempty"`
}
