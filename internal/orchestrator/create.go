package orchestrator

import (
	"strings"

	"github.com/google/uuid"
)

type CreateOptions struct {
	ID     string
	Goal   string
	Expert string
}

func NewPlan(opts CreateOptions) Plan {
	id := strings.TrimSpace(opts.ID)
	if id == "" {
		id = "plan-" + uuid.NewString()
	}
	goal := strings.TrimSpace(opts.Goal)
	expert := strings.TrimSpace(opts.Expert)
	if expert == "" {
		expert = "generalist"
	}
	return Plan{
		ID:     id,
		Goal:   goal,
		Status: PlanStatusPending,
		Tasks: []TaskNode{{
			ID:     "t1",
			Title:  "Execute goal",
			Expert: expert,
			Type:   "execution",
			Input: TaskInput{
				Prompt: goal,
			},
		}},
	}
}
