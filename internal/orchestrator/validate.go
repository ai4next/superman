package orchestrator

import (
	"fmt"
	"strings"
)

func ValidatePlan(plan Plan) error {
	if strings.TrimSpace(plan.ID) == "" {
		return fmt.Errorf("plan id is required")
	}
	if strings.TrimSpace(plan.Goal) == "" {
		return fmt.Errorf("plan goal is required")
	}
	if len(plan.Tasks) == 0 {
		return fmt.Errorf("plan must contain at least one task")
	}
	tasks := make(map[string]TaskNode, len(plan.Tasks))
	for _, task := range plan.Tasks {
		if strings.TrimSpace(task.ID) == "" {
			return fmt.Errorf("task id is required")
		}
		if _, exists := tasks[task.ID]; exists {
			return fmt.Errorf("duplicate task %q", task.ID)
		}
		if strings.TrimSpace(task.Expert) == "" {
			return fmt.Errorf("task %q expert is required", task.ID)
		}
		if strings.TrimSpace(task.Input.Prompt) == "" {
			return fmt.Errorf("task %q input prompt is required", task.ID)
		}
		tasks[task.ID] = task
	}
	for _, task := range plan.Tasks {
		for _, dep := range task.DependsOn {
			if _, ok := tasks[dep]; !ok {
				return fmt.Errorf("task %q depends on missing task %q", task.ID, dep)
			}
		}
	}
	for _, join := range plan.Joins {
		if strings.TrimSpace(join.ID) == "" {
			return fmt.Errorf("join id is required")
		}
		for _, dep := range join.DependsOn {
			if _, ok := tasks[dep]; !ok {
				return fmt.Errorf("join %q depends on missing task %q", join.ID, dep)
			}
		}
	}
	return detectCycle(plan.Tasks)
}

func detectCycle(tasks []TaskNode) error {
	graph := make(map[string][]string, len(tasks))
	for _, task := range tasks {
		graph[task.ID] = task.DependsOn
	}
	visiting := make(map[string]bool, len(tasks))
	visited := make(map[string]bool, len(tasks))
	var visit func(string) error
	visit = func(id string) error {
		if visiting[id] {
			return fmt.Errorf("task dependency cycle at %q", id)
		}
		if visited[id] {
			return nil
		}
		visiting[id] = true
		for _, dep := range graph[id] {
			if err := visit(dep); err != nil {
				return err
			}
		}
		visiting[id] = false
		visited[id] = true
		return nil
	}
	for _, task := range tasks {
		if err := visit(task.ID); err != nil {
			return err
		}
	}
	return nil
}
