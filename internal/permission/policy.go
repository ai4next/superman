package permission

import "slices"

const (
	ToolCodeRun = "code_run"
	ToolWrite   = "write"
	ToolPatch   = "patch"
)

// Policy controls which tool calls need human confirmation before execution.
type Policy struct {
	SkipRequests bool
	AllowedTools []string
	RiskyTools   []string
}

type Request struct {
	ToolName string
	Action   string
	Input    any
}

func DefaultRiskyTools() []string {
	return []string{
		ToolCodeRun,
		ToolWrite,
		ToolPatch,
	}
}

func NewPolicy(skip bool, allowedTools []string, riskyTools []string) Policy {
	if riskyTools == nil {
		riskyTools = DefaultRiskyTools()
	}
	return Policy{
		SkipRequests: skip,
		AllowedTools: slices.Clone(allowedTools),
		RiskyTools:   slices.Clone(riskyTools),
	}
}

// RequiresConfirmation returns true when a tool call should be paused for HITL approval.
func (p Policy) RequiresConfirmation(req Request) bool {
	if p.SkipRequests {
		return false
	}
	if req.ToolName == "" {
		return false
	}
	if p.allowed(req.ToolName, req.Action) {
		return false
	}
	return slices.Contains(p.RiskyTools, req.ToolName)
}

func (p Policy) allowed(toolName, action string) bool {
	if slices.Contains(p.AllowedTools, toolName) {
		return true
	}
	if action != "" && slices.Contains(p.AllowedTools, toolName+":"+action) {
		return true
	}
	return false
}
