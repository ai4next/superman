package orchestrator

import (
	"fmt"

	"github.com/ai4next/superman/internal/bus"
)

type Scheduler struct {
	Queue bus.TaskQueue
}

func (s Scheduler) EnqueueReady(plan *Plan) ([]bus.TaskReceipt, error) {
	if plan == nil {
		return nil, fmt.Errorf("plan is nil")
	}
	if s.Queue == nil {
		return nil, fmt.Errorf("task queue is nil")
	}
	if err := ValidatePlan(*plan); err != nil {
		return nil, err
	}
	var receipts []bus.TaskReceipt
	for i := range plan.Tasks {
		node := plan.Tasks[i]
		if plan.State(node.ID) != TaskStatusPending && plan.State(node.ID) != TaskStatusReady {
			continue
		}
		if TaskBusID(plan, node.ID) != "" {
			continue
		}
		if !dependenciesSucceeded(plan, node) {
			plan.SetState(node.ID, TaskStatusPending)
			continue
		}
		receipt, err := s.Queue.Enqueue(bus.Task{
			ID:          plan.ID + ":" + node.ID,
			Type:        "delegate",
			Queue:       "experts",
			MaxAttempts: effectiveMaxAttempts(node.Retry),
			Payload: map[string]string{
				"plan_id":     plan.ID,
				"task_id":     node.ID,
				"expert_name": node.Expert,
				"task":        node.Input.Prompt,
				"title":       node.Title,
				"type":        node.Type,
			},
		})
		if err != nil {
			return receipts, err
		}
		plan.SetState(node.ID, TaskStatusQueued)
		if plan.Tasks[i].Metadata == nil {
			plan.Tasks[i].Metadata = make(map[string]string)
		}
		plan.Tasks[i].Metadata["bus_task_id"] = receipt.TaskID
		receipts = append(receipts, receipt)
	}
	return receipts, nil
}

func dependenciesSucceeded(plan *Plan, node TaskNode) bool {
	for _, dep := range node.DependsOn {
		if plan.State(dep) != TaskStatusSucceeded {
			return false
		}
	}
	return true
}

func effectiveMaxAttempts(policy RetryPolicy) int {
	if policy.MaxAttempts > 0 {
		return policy.MaxAttempts
	}
	return 2
}

func MarkSucceeded(plan *Plan, taskID string) {
	plan.SetState(taskID, TaskStatusSucceeded)
}

func TaskBusID(plan *Plan, taskID string) string {
	if plan == nil {
		return ""
	}
	for _, task := range plan.Tasks {
		if task.ID == taskID && task.Metadata != nil {
			return task.Metadata["bus_task_id"]
		}
	}
	return ""
}
