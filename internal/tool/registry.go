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
	Config         *config.Config
	ExpertManager  ExpertManager `json:"-"`
	DelegateRunner DelegateRunner
	ExpertTools    bool
}

// RegisterAll creates and returns all enabled tools.
func RegisterAll(deps Dependencies) []tool.Tool {
	if deps.Config == nil {
		deps.Config = &config.Config{}
	}
	var tools []tool.Tool

	if deps.Config.Tools.CodeRun.Enabled {
		tools = append(tools, newCodeRunTool(deps))
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
	if deps.Config.Tools.AskUser.Enabled {
		tools = append(tools, newAskUserTool(deps))
	}
	if shouldRegisterDelegateTool(deps) {
		tools = append(tools, newDelegateTool(deps))
	}

	return tools
}

func shouldRegisterDelegateTool(deps Dependencies) bool {
	if !deps.ExpertTools || deps.DelegateRunner == nil || deps.ExpertManager == nil {
		return false
	}
	return len(deps.ExpertManager.List()) > 0
}
