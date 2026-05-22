package tools

import (
	"context"

	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/expert"
	"google.golang.org/adk/tool"
)

// SearchResult is returned by memory search operations.
type SearchResult struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	Layer   int    `json:"layer"`
	Content string `json:"content"`
}

// MemoryStorer is the interface for storing long-term memories.
type MemoryStorer interface {
	StoreString(ctx context.Context, content, category string) (string, error)
}

// MemorySearcher provides read access to stored memories.
type MemorySearcher interface {
	Search(ctx context.Context, query string) ([]SearchResult, error)
}

// ExpertManager provides read and write access to the expert registry.
type ExpertManager interface {
	Search(query string) []*expert.Spec
	List() []*expert.Spec
	Create(spec expert.Spec) (*expert.Spec, error)
}

// Dependencies holds shared dependencies for all tools.
type Dependencies struct {
	Config         *config.Config
	Workspace      string
	MemoryService  MemoryStorer
	MemorySearcher MemorySearcher
	ExpertManager  ExpertManager `json:"-"`
}

// RegisterAll creates and returns all enabled tools.
func RegisterAll(deps Dependencies) []tool.Tool {
	var tools []tool.Tool

	if deps.Config.Tools.CodeRun.Enabled {
		tools = append(tools, newCodeRunTool(deps))
	}
	if deps.Config.Tools.FileRead.Enabled {
		tools = append(tools, newFileReadTool(deps))
	}
	if deps.Config.Tools.FileWrite.Enabled {
		tools = append(tools, newFileWriteTool(deps))
	}
	if deps.Config.Tools.FilePatch.Enabled {
		tools = append(tools, newFilePatchTool(deps))
	}
	if deps.Config.Tools.WebScan.Enabled {
		tools = append(tools, newWebScanTool(deps))
	}
	if deps.Config.Tools.WebExecute.Enabled {
		tools = append(tools, newWebExecuteTool(deps))
	}
	if deps.Config.Tools.AskUser.Enabled {
		tools = append(tools, newAskUserTool(deps))
	}
	if deps.Config.Tools.Checkpoint.Enabled {
		tools = append(tools, newCheckpointTool(deps))
	}
	if deps.Config.Tools.LongTermMemory.Enabled {
		tools = append(tools, newLongTermMemoryTool(deps))
	}
	if deps.Config.Tools.LongTermMemory.Enabled {
		tools = append(tools, newSearchMemoryTool(deps))
	}
	if deps.ExpertManager != nil && deps.Config.Expert.Enabled {
		tools = append(tools, newQueryExpertsTool(deps.ExpertManager))
		tools = append(tools, newCreateExpertTool(deps.ExpertManager))
	}

	return tools
}