package orchestrator

import (
	"github.com/ai4next/superman/internal/bus"
)

type ReconcileResult struct {
	Changed  bool              `json:"changed"`
	Queued   []bus.TaskReceipt `json:"queued,omitempty"`
	Complete bool              `json:"complete"`
	Failed   bool              `json:"failed"`
}

func Reconcile(plan *Plan, queue bus.TaskQueue) (ReconcileResult, error) {
	result := ReconcileResult{}
	if plan == nil {
		return result, nil
	}
	for _, task := range plan.Tasks {
		busTaskID := TaskBusID(plan, task.ID)
		if busTaskID == "" {
			continue
		}
		if _, ok, err := queue.TaskResult(busTaskID); err != nil {
			return result, err
		} else if ok && plan.State(task.ID) != TaskStatusSucceeded {
			plan.SetState(task.ID, TaskStatusSucceeded)
			result.Changed = true
			continue
		}
		if queued, ok, err := queue.Task(busTaskID); err != nil {
			return result, err
		} else if ok {
			next := taskStatusFromBus(queued.Status)
			if next != "" && plan.State(task.ID) != next {
				plan.SetState(task.ID, next)
				result.Changed = true
			}
		}
	}
	if hasDeadTask(plan) {
		if plan.Status != PlanStatusFailed {
			plan.Status = PlanStatusFailed
			result.Changed = true
		}
		result.Failed = true
		return result, nil
	}
	queued, err := Scheduler{Queue: queue}.EnqueueReady(plan)
	if err != nil {
		return result, err
	}
	if len(queued) > 0 {
		result.Queued = queued
		result.Changed = true
	}
	if allTasksSucceeded(plan) {
		if plan.Status != PlanStatusDone {
			plan.Status = PlanStatusDone
			result.Changed = true
		}
		if plan.FinalOutput == "" {
			output, err := Aggregate(*plan, queue)
			if err != nil {
				return result, err
			}
			plan.FinalOutput = output
			result.Changed = true
		}
		result.Complete = true
	} else if plan.Status == "" || plan.Status == PlanStatusPending {
		plan.Status = PlanStatusRunning
		result.Changed = true
	}
	return result, nil
}

func taskStatusFromBus(status bus.TaskStatus) TaskStatus {
	switch status {
	case bus.TaskStatusReady:
		return TaskStatusQueued
	case bus.TaskStatusRunning:
		return TaskStatusRunning
	case bus.TaskStatusDead:
		return TaskStatusDead
	default:
		return ""
	}
}

func allTasksSucceeded(plan *Plan) bool {
	if plan == nil || len(plan.Tasks) == 0 {
		return false
	}
	for _, task := range plan.Tasks {
		if plan.State(task.ID) != TaskStatusSucceeded {
			return false
		}
	}
	return true
}

func hasDeadTask(plan *Plan) bool {
	if plan == nil {
		return false
	}
	for _, task := range plan.Tasks {
		if plan.State(task.ID) == TaskStatusDead {
			return true
		}
	}
	return false
}
