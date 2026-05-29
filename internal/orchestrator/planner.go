package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"strings"

	adkmodel "google.golang.org/adk/model"
	"google.golang.org/genai"
)

type PlannerOptions struct {
	Goal    string
	Experts []string
}

func GeneratePlan(ctx context.Context, llm adkmodel.LLM, opts PlannerOptions) (Plan, error) {
	if llm == nil {
		return Plan{}, fmt.Errorf("llm is nil")
	}
	prompt := plannerPrompt(opts)
	req := &adkmodel.LLMRequest{
		Contents: []*genai.Content{genai.NewContentFromText(prompt, genai.RoleUser)},
	}
	text, err := collectLLMText(llm.GenerateContent(ctx, req, false))
	if err != nil {
		return Plan{}, err
	}
	plan, err := ParsePlanJSON(text)
	if err != nil {
		return Plan{}, err
	}
	if plan.Goal == "" {
		plan.Goal = opts.Goal
	}
	if err := ValidatePlan(plan); err != nil {
		return Plan{}, err
	}
	return plan, nil
}

func plannerPrompt(opts PlannerOptions) string {
	experts := strings.Join(opts.Experts, ", ")
	if experts == "" {
		experts = "generalist"
	}
	return fmt.Sprintf(`Create a DAG plan JSON for the user goal.

Return only JSON. No markdown.

Schema:
{
  "plan_id": "short-stable-id",
  "goal": "original goal",
  "strategy": "brief strategy",
  "tasks": [
    {
      "id": "t1",
      "title": "short title",
      "expert": "expert name from available experts",
      "type": "analysis|execution|review",
      "depends_on": [],
      "input": {"prompt": "specific delegated task"}
    }
  ]
}

Available experts: %s
Goal: %s`, experts, opts.Goal)
}

func collectLLMText(seq iter.Seq2[*adkmodel.LLMResponse, error]) (string, error) {
	var builder strings.Builder
	for resp, err := range seq {
		if err != nil {
			return "", err
		}
		if resp == nil || resp.Content == nil {
			continue
		}
		for _, part := range resp.Content.Parts {
			if part != nil {
				builder.WriteString(part.Text)
			}
		}
	}
	text := strings.TrimSpace(builder.String())
	if text == "" {
		return "", fmt.Errorf("planner returned empty response")
	}
	return text, nil
}

func ParsePlanJSON(text string) (Plan, error) {
	text = strings.TrimSpace(text)
	text = stripJSONFence(text)
	start := strings.IndexByte(text, '{')
	end := strings.LastIndexByte(text, '}')
	if start >= 0 && end >= start {
		text = text[start : end+1]
	}
	var plan Plan
	if err := json.Unmarshal([]byte(text), &plan); err != nil {
		return Plan{}, fmt.Errorf("decode planner JSON: %w", err)
	}
	return plan, nil
}

func stripJSONFence(text string) string {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "```") {
		return text
	}
	lines := strings.Split(text, "\n")
	if len(lines) >= 3 {
		lines = lines[1 : len(lines)-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
