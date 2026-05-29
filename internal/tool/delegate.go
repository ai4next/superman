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
	Mode       string `json:"mode,omitempty" jsonschema:"Execution mode: sync or async. Default is sync."`
	Parallel   bool   `json:"parallel,omitempty" jsonschema:"When true, enqueue the task for parallel/asynchronous execution."`
}

type delegateOutput struct {
	Response string `json:"response,omitempty"`
	TaskID   string `json:"task_id,omitempty"`
	Status   string `json:"status,omitempty"`
	Mode     string `json:"mode,omitempty"`
	Message  string `json:"message,omitempty"`
}

// DelegateRunner can execute a task using an expert's prompt.
type DelegateRunner interface {
	RunDelegate(ctx context.Context, specName string, task string) (string, error)
}

// DelegateScheduler can enqueue a task for asynchronous expert execution.
type DelegateScheduler interface {
	EnqueueDelegate(ctx context.Context, req DelegateTaskRequest) (DelegateTaskReceipt, error)
}

type DelegateTaskRequest struct {
	ExpertName string `json:"expert_name"`
	Task       string `json:"task"`
}

type DelegateTaskReceipt struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

func newDelegateTool(deps Dependencies) tool.Tool {
	desc := "Delegate to an expert. Use mode=sync for an immediate result, or mode=async / parallel=true to enqueue work for parallel execution."
	experts := deps.ExpertManager.List()
	if len(experts) > 0 {
		var lines []string
		for _, e := range experts {
			lines = append(lines, fmt.Sprintf("  - %s", e.Name))
		}
		desc += "\nExperts:\n" + strings.Join(lines, "\n")
	}

	handler := func(tctx tool.Context, input delegateInput) (delegateOutput, error) {
		return runDelegateTool(context.Background(), deps, input)
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "delegate",
		Description: desc,
	}, handler)
	return t
}

func runDelegateTool(ctx context.Context, deps Dependencies, input delegateInput) (delegateOutput, error) {
	mode := delegateMode(input)
	if mode == "async" {
		scheduler := deps.DelegateScheduler
		if scheduler == nil {
			if runnerScheduler, ok := deps.DelegateRunner.(DelegateScheduler); ok {
				scheduler = runnerScheduler
			}
		}
		if scheduler == nil {
			return delegateOutput{}, fmt.Errorf("delegate async mode requested but scheduler is not available")
		}
		receipt, err := scheduler.EnqueueDelegate(ctx, DelegateTaskRequest{
			ExpertName: input.ExpertName,
			Task:       input.Task,
		})
		if err != nil {
			return delegateOutput{}, err
		}
		status := firstNonEmpty(receipt.Status, "queued")
		return delegateOutput{
			TaskID:  receipt.TaskID,
			Status:  status,
			Mode:    mode,
			Message: "delegate task queued for parallel execution",
		}, nil
	}

	runner := deps.DelegateRunner
	if runner == nil {
		return delegateOutput{}, fmt.Errorf("delegate runner not available")
	}
	resp, err := runner.RunDelegate(ctx, input.ExpertName, input.Task)
	if err != nil {
		return delegateOutput{}, err
	}
	return delegateOutput{Response: resp, Status: "succeeded", Mode: mode}, nil
}

func delegateMode(input delegateInput) string {
	mode := strings.ToLower(strings.TrimSpace(input.Mode))
	if input.Parallel {
		return "async"
	}
	switch mode {
	case "", "sync", "synchronous":
		return "sync"
	case "async", "asynchronous", "parallel", "queued", "queue":
		return "async"
	default:
		return mode
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
