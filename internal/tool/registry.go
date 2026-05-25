package tool

import (
	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/expert"
	"github.com/ai4next/superman/internal/permission"
	supermansession "github.com/ai4next/superman/internal/session"
	"google.golang.org/adk/tool"
)

// ExpertManager provides read access to the expert registry.
type ExpertManager interface {
	List() []*expert.Spec
}

// Dependencies holds shared dependencies for all tools.
type Dependencies struct {
	Config      *config.Config
	Permissions interface {
		RequiresConfirmation(permission.Request) bool
	}
	FileTracker interface {
		RecordFileRead(appName, userID, sessionID, path string) error
		RecordFileWrite(appName, userID, sessionID, path string) error
		RecordFileRevision(appName, userID, sessionID, path, action, before, after string, beforeMissing bool) (supermansession.FileRevision, error)
	}
	ExpertManager  ExpertManager `json:"-"`
	DelegateRunner DelegateRunner
	ExpertTools    bool
}

func (d Dependencies) requiresConfirmation(toolName, action string, input any) bool {
	req := permission.Request{
		ToolName: toolName,
		Action:   action,
		Input:    input,
	}
	if d.Permissions != nil {
		return d.Permissions.RequiresConfirmation(req)
	}
	if d.Config == nil {
		return false
	}
	policy := permission.NewPolicy(
		d.Config.Permissions.SkipRequests,
		d.Config.Permissions.AllowedTools,
		d.Config.Permissions.RiskyTools,
	)
	return policy.RequiresConfirmation(req)
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
	if deps.ExpertTools && deps.DelegateRunner != nil {
		tools = append(tools, newDelegateTool(deps))
	}

	return tools
}
