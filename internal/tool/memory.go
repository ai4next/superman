package tool

import (
	"context"
	"strings"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/ai4next/superman/internal/memory"
)

type memorySearchInput struct {
	Query  string   `json:"query" jsonschema:"Search query"`
	Owners []string `json:"owners,omitempty" jsonschema:"Optional owners to search: superman or expert names"`
	Layers []string `json:"layers,omitempty" jsonschema:"Optional layers to search: l1 or l2"`
	Limit  int      `json:"limit,omitempty" jsonschema:"Maximum results"`
}

type memorySearchOutput struct {
	Results []memory.SearchResult `json:"results"`
}

func newMemorySearchTool(deps Dependencies) tool.Tool {
	handler := func(tctx tool.Context, input memorySearchInput) (memorySearchOutput, error) {
		return runMemorySearch(context.Background(), deps, input)
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "memory_search",
		Description: "Search All L1/L2 memories. Returns owner, layer, path, match type, score, and snippet.",
	}, handler)
	return t
}

func runMemorySearch(ctx context.Context, deps Dependencies, input memorySearchInput) (memorySearchOutput, error) {
	_ = ctx
	svc := memory.NewSearchService(deps.Config)
	results, err := svc.Search(memory.SearchOptions{
		Query:  input.Query,
		Owners: normalizeList(input.Owners),
		Layers: normalizeList(input.Layers),
		Limit:  input.Limit,
	})
	if err != nil {
		return memorySearchOutput{}, err
	}
	return memorySearchOutput{Results: results}, nil
}

func normalizeList(values []string) []string {
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
