package tool

import (
	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/expert"
	"google.golang.org/adk/tool"
)

// ExpertManager provides read access to the expert registry.
type ExpertManager interface {
	List() []*expert.Spec
}

// Dependencies holds shared dependencies for all tools.
type Dependencies struct {
	Config            *config.Config
	ExpertManager     ExpertManager `json:"-"`
	DelegateRunner    DelegateRunner
	DelegateScheduler DelegateScheduler
	Orchestrator      Orchestrator
	ExpertTools       bool
}

// RegisterAll creates and returns all enabled tools.
func RegisterAll(deps Dependencies) []tool.Tool {
	if deps.Config == nil {
		deps.Config = &config.Config{}
	}
	var tools []tool.Tool

	if deps.Config.Tools.Exec.Enabled {
		tools = append(tools, newExecTool(deps))
	}
	if deps.Config.Tools.Read.Enabled {
		tools = append(tools, newReadTool(deps))
	}
	if deps.Config.Tools.Write.Enabled {
		tools = append(tools, newWriteTool(deps))
	}
	if deps.Config.Tools.Patch.Enabled {
		tools = append(tools, newPatchTool(deps))
	}
	if deps.Config.Tools.Ask.Enabled {
		tools = append(tools, newAskTool(deps))
	}
	if shouldRegisterDelegateTool(deps) {
		tools = append(tools, newDelegateTool(deps))
	}
	if deps.ExpertTools && deps.Orchestrator != nil {
		tools = append(tools, newOrchestrateTool(deps))
	}

	return tools
}

func shouldRegisterDelegateTool(deps Dependencies) bool {
	if !deps.ExpertTools || (deps.DelegateRunner == nil && deps.DelegateScheduler == nil) || deps.ExpertManager == nil {
		return false
	}
	return len(deps.ExpertManager.List()) > 0
}
