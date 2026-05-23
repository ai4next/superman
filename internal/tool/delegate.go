package tool

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type delegateInput struct {
	ExpertName string `json:"expert_name" jsonschema:"Expert name"`
	Task       string `json:"task" jsonschema:"Task for the expert"`
}

type delegateOutput struct {
	Response string `json:"response,omitempty"`
}

// DelegateRunner can execute a task using an expert's prompt.
type DelegateRunner interface {
	RunDelegate(ctx context.Context, specName string, task string) (string, error)
}

func newDelegateTool(deps Dependencies) tool.Tool {
	desc := "Delegate to an expert."
	if deps.ExpertManager != nil {
		experts := deps.ExpertManager.List()
		if len(experts) > 0 {
			var lines []string
			for _, e := range experts {
				lines = append(lines, fmt.Sprintf("  - %s", e.Name))
			}
			desc += "\nExperts:\n" + strings.Join(lines, "\n")
		} else {
			desc += "\nNo experts available."
		}
	}

	handler := func(tctx tool.Context, input delegateInput) (delegateOutput, error) {
		runner := deps.DelegateRunner
		if runner == nil {
			return delegateOutput{}, fmt.Errorf("Delegate runner not available")
		}
		resp, err := runner.RunDelegate(context.Background(), input.ExpertName, input.Task)
		if err != nil {
			return delegateOutput{}, err
		}
		return delegateOutput{Response: resp}, nil
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "delegate_to_expert",
		Description: desc,
	}, handler)
	return t
}
