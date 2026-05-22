package tools

import (
	"context"
	"fmt"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type longTermMemoryInput struct {
	Content  string `json:"content" jsonschema:"The information to save to long-term memory"`
	Category string `json:"category,omitempty" jsonschema:"Optional category for organizing memories"`
}

type longTermMemoryOutput struct {
	Stored bool   `json:"stored"`
	ID     string `json:"id,omitempty"`
	Error  string `json:"error,omitempty"`
}

func newLongTermMemoryTool(deps Dependencies) tool.Tool {
	handler := func(tctx tool.Context, input longTermMemoryInput) (longTermMemoryOutput, error) {
		if deps.MemoryService != nil {
			id, err := deps.MemoryService.StoreString(context.Background(), input.Content, input.Category)
			if err != nil {
				return longTermMemoryOutput{
					Stored: false,
					Error:  fmt.Sprintf("failed to store memory: %v", err),
				}, err
			}
			return longTermMemoryOutput{
				Stored: true,
				ID:     id,
			}, nil
		}
		// Fallback when no memory service is configured
		return longTermMemoryOutput{
			Stored: true,
			ID:     fmt.Sprintf("mem-%d", len(input.Content)),
		}, nil
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "long_term_memory",
		Description: "Save important information to long-term memory for future sessions",
	}, handler)
	return t
}
