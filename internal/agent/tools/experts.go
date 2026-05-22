package tools

import (
	"context"
	"fmt"

	"github.com/ai4next/superman/internal/expert"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// expertInfo is the public representation of an expert returned to the agent.
type expertInfo struct {
	Name           string   `json:"name"`
	Summary        string   `json:"summary"`
	Description    string   `json:"description"`
	ToolAllowlist  []string `json:"tools"`
	SystemPrompt   string   `json:"system_prompt"`
	Status         string   `json:"status"`
}

type queryExpertsInput struct {
	TaskDescription string `json:"task_description" jsonschema:"Describes the current task to find matching experts"`
}

type queryExpertsOutput struct {
	Found   bool         `json:"found"`
	Experts []expertInfo `json:"experts,omitempty"`
	Message string       `json:"message,omitempty"`
}

func newQueryExpertsTool(em ExpertManager) tool.Tool {
	handler := func(tctx tool.Context, input queryExpertsInput) (queryExpertsOutput, error) {
		results := em.Search(input.TaskDescription)
		if len(results) == 0 {
			return queryExpertsOutput{
				Found:   false,
				Message: "No matching experts found",
			}, nil
		}

		experts := make([]expertInfo, 0, len(results))
		for _, s := range results {
			experts = append(experts, expertInfo{
				Name:          s.Name,
				Summary:       s.Summary,
				Description:   s.Description,
				ToolAllowlist: s.ToolAllowlist,
				SystemPrompt:  s.SystemPrompt,
				Status:        string(s.Status),
			})
		}

		return queryExpertsOutput{
			Found:   true,
			Experts: experts,
			Message: fmt.Sprintf("Found %d matching experts", len(experts)),
		}, nil
	}

	t, _ := functiontool.New(functiontool.Config{
		Name:        "query_experts",
		Description: "Search for expert agents that match the current task. Use this to find specialized experts that can help with specific types of work.",
	}, handler)
	return t
}

type createExpertInput struct {
	Name           string   `json:"name" jsonschema:"Unique name for the expert"`
	Summary        string   `json:"summary" jsonschema:"Short summary of the expert's purpose"`
	Description    string   `json:"description" jsonschema:"Detailed description of what the expert does"`
	TriggerPattern string   `json:"trigger_pattern" jsonschema:"Keyword pattern that triggers this expert"`
	Tools          []string `json:"tools" jsonschema:"List of tool names the expert is allowed to use"`
	SystemPrompt   string   `json:"system_prompt" jsonschema:"System prompt that defines the expert's behavior"`
}

type createExpertOutput struct {
	Created bool   `json:"created"`
	Name    string `json:"name,omitempty"`
	Error   string `json:"error,omitempty"`
}

func newCreateExpertTool(em ExpertManager) tool.Tool {
	handler := func(tctx tool.Context, input createExpertInput) (createExpertOutput, error) {
		if input.Name == "" {
			return createExpertOutput{
				Created: false,
				Error:   "expert name is required",
			}, nil
		}

		spec := expert.Spec{
			Name:           input.Name,
			Summary:        input.Summary,
			Description:    input.Description,
			TriggerPattern: input.TriggerPattern,
			ToolAllowlist:  input.Tools,
			SystemPrompt:   input.SystemPrompt,
		}

		created, err := em.Create(spec)
		if err != nil {
			return createExpertOutput{
				Created: false,
				Name:    input.Name,
				Error:   fmt.Sprintf("Failed to create expert: %v", err),
			}, nil
		}

		return createExpertOutput{
			Created: true,
			Name:    created.Name,
		}, nil
	}

	t, _ := functiontool.New(functiontool.Config{
		Name:        "create_expert",
		Description: "Create a new expert agent with a name, description, trigger pattern, tool allowlist, and system prompt. Use this to define reusable specialized agents for common task types.",
	}, handler)
	return t
}

type delegateInput struct {
	ExpertName string `json:"expert_name" jsonschema:"Name of the expert to delegate to"`
	Task       string `json:"task" jsonschema:"The task description to send to the expert"`
}

type delegateOutput struct {
	Success  bool   `json:"success"`
	Response string `json:"response,omitempty"`
	Error    string `json:"error,omitempty"`
}

// DelegateRunner can execute a task using an expert's prompt.
type DelegateRunner interface {
	RunDelegate(ctx context.Context, specName string, task string) (string, error)
}

func newDelegateTool(runner DelegateRunner) tool.Tool {
	handler := func(tctx tool.Context, input delegateInput) (delegateOutput, error) {
		if runner == nil {
			return delegateOutput{Success: false, Error: "Delegate runner not available"}, nil
		}
		resp, err := runner.RunDelegate(context.Background(), input.ExpertName, input.Task)
		if err != nil {
			return delegateOutput{Success: false, Error: err.Error()}, nil
		}
		return delegateOutput{Success: true, Response: resp}, nil
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "delegate_to_expert",
		Description: "Delegate a task to an expert agent for independent execution. The expert will use its own system prompt and tools. Use this when a task needs deep specialization.",
	}, handler)
	return t
}