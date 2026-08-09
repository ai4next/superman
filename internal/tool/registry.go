package tool

import (
	"time"

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
	EvolverTools      bool
}

// RegisterAll creates and returns all enabled tools.
func RegisterAll(deps Dependencies) []tool.Tool {
	if deps.Config == nil {
		deps.Config = &config.Config{}
	} else {
		// Tool registration happens for every model request. Apply defaults to a
		// request-local copy so concurrent sessions never mutate shared config.
		cfg := *deps.Config
		deps.Config = &cfg
	}
	applyToolDefaults(deps.Config)
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
	if deps.Config.Memory.Search.Enabled {
		tools = append(tools, newMemorySearchTool(deps))
	}
	if shouldRegisterDelegateTool(deps) {
		tools = append(tools, newDelegateTool(deps))
	}
	if deps.ExpertTools && deps.Orchestrator != nil {
		tools = append(tools, newOrchestrateTool(deps))
	}

	return tools
}

func applyToolDefaults(cfg *config.Config) {
	if cfg.Tools.Exec.Timeout <= 0 {
		cfg.Tools.Exec.Timeout = config.Duration(30 * time.Second)
	}
	if cfg.Tools.Exec.MaxOutputSize <= 0 {
		cfg.Tools.Exec.MaxOutputSize = 1_048_576
	}
	if cfg.Tools.Read.MaxSize <= 0 {
		cfg.Tools.Read.MaxSize = 10_485_760
	}
	if cfg.Tools.Write.MaxSize <= 0 {
		cfg.Tools.Write.MaxSize = 10_485_760
	}
	if cfg.Memory.Search.MaxResults == 0 {
		cfg.Memory.Search.MaxResults = 8
	}
	if cfg.Memory.Search.Enabled && !cfg.Memory.Search.FTSEnabled && !cfg.Memory.Search.ScanEnabled && !cfg.Memory.Search.VectorEnabled {
		cfg.Memory.Search.FTSEnabled = true
		cfg.Memory.Search.ScanEnabled = true
	}
}

func shouldRegisterDelegateTool(deps Dependencies) bool {
	if !deps.ExpertTools || (deps.DelegateRunner == nil && deps.DelegateScheduler == nil) || deps.ExpertManager == nil {
		return false
	}
	return len(deps.ExpertManager.List()) > 0
}
