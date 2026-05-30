package orchestrator

type PlanStatus string

const (
	PlanStatusPending PlanStatus = "pending"
	PlanStatusRunning PlanStatus = "running"
	PlanStatusDone    PlanStatus = "done"
	PlanStatusFailed  PlanStatus = "failed"
)

type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusReady     TaskStatus = "ready"
	TaskStatusQueued    TaskStatus = "queued"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusSucceeded TaskStatus = "succeeded"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusDead      TaskStatus = "dead"
	TaskStatusCanceled  TaskStatus = "canceled"
)

type Plan struct {
	ID          string                `json:"plan_id"`
	Goal        string                `json:"goal"`
	Strategy    string                `json:"strategy,omitempty"`
	Constraints Constraints           `json:"constraints,omitempty"`
	Tasks       []TaskNode            `json:"tasks"`
	Joins       []JoinNode            `json:"joins,omitempty"`
	Status      PlanStatus            `json:"status,omitempty"`
	TaskStates  map[string]TaskStatus `json:"task_states,omitempty"`
	FinalOutput string                `json:"final_output,omitempty"`
}

type Constraints struct {
	MaxParallel    int    `json:"max_parallel,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
	QualityBar     string `json:"quality_bar,omitempty"`
}

type TaskNode struct {
	ID             string            `json:"id"`
	Title          string            `json:"title,omitempty"`
	Expert         string            `json:"expert"`
	Type           string            `json:"type,omitempty"`
	DependsOn      []string          `json:"depends_on,omitempty"`
	Input          TaskInput         `json:"input"`
	ExpectedOutput map[string]any    `json:"expected_output,omitempty"`
	Retry          RetryPolicy       `json:"retry,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

type TaskInput struct {
	Prompt string            `json:"prompt"`
	Params map[string]string `json:"params,omitempty"`
}

type RetryPolicy struct {
	MaxAttempts    int `json:"max_attempts,omitempty"`
	BackoffSeconds int `json:"backoff_seconds,omitempty"`
}

type JoinNode struct {
	ID         string   `json:"id"`
	DependsOn  []string `json:"depends_on"`
	Mode       string   `json:"mode,omitempty"`
	Aggregator string   `json:"aggregator,omitempty"`
}

func (p *Plan) State(taskID string) TaskStatus {
	if p == nil || p.TaskStates == nil {
		return TaskStatusPending
	}
	if state := p.TaskStates[taskID]; state != "" {
		return state
	}
	return TaskStatusPending
}

func (p *Plan) SetState(taskID string, status TaskStatus) {
	if p.TaskStates == nil {
		p.TaskStates = make(map[string]TaskStatus)
	}
	p.TaskStates[taskID] = status
}
