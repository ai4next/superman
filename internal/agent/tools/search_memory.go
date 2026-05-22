package tools

import (
	"context"
	"fmt"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type searchMemoryInput struct {
	Query string `json:"query" jsonschema:"Search query to find relevant past memories"`
}

type searchMemoryOutput struct {
	Found   bool           `json:"found"`
	Results []SearchResult `json:"results,omitempty"`
	Message string         `json:"message,omitempty"`
}

func newSearchMemoryTool(deps Dependencies) tool.Tool {
	handler := func(tctx tool.Context, input searchMemoryInput) (searchMemoryOutput, error) {
		if deps.MemorySearcher == nil {
			return searchMemoryOutput{
				Found:   false,
				Message: "Memory search is not available (no memory service configured)",
			}, nil
		}

		results, err := deps.MemorySearcher.Search(context.Background(), input.Query)
		if err != nil {
			return searchMemoryOutput{
				Found:   false,
				Message: fmt.Sprintf("Search failed: %v", err),
			}, err
		}

		if len(results) == 0 {
			return searchMemoryOutput{
				Found:   false,
				Message: "No matching memories found",
			}, nil
		}

		return searchMemoryOutput{
			Found:   true,
			Results: results,
			Message: fmt.Sprintf("Found %d matching memories", len(results)),
		}, nil
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "search_memory",
		Description: "Search past memories for information relevant to the current task. Use this to recall facts, preferences, and context from previous conversations.",
	}, handler)
	return t
}
