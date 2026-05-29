package bus

import (
	"strings"
	"time"
)

const (
	defaultQueueName   = "default"
	defaultMaxAttempts = 1
)

type TaskStatus string

const (
	TaskStatusReady   TaskStatus = "ready"
	TaskStatusRunning TaskStatus = "running"
	TaskStatusDead    TaskStatus = "dead"
)

type Task struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Queue       string            `json:"queue,omitempty"`
	Priority    int               `json:"priority,omitempty"`
	Payload     map[string]string `json:"payload,omitempty"`
	MaxAttempts int               `json:"max_attempts,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Attempt     int               `json:"attempt,omitempty"`
	Status      TaskStatus        `json:"status"`
}

type TaskReceipt struct {
	TaskID string     `json:"task_id"`
	Status TaskStatus `json:"status"`
}

type WorkerRef struct {
	ID     string `json:"id"`
	Queue  string `json:"queue,omitempty"`
	Type   string `json:"type,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Prefix string `json:"prefix,omitempty"`
}

type RunningTask struct {
	Task     Task   `json:"task"`
	WorkerID string `json:"worker_id"`
}

type TaskResult struct {
	TaskID     string    `json:"task_id"`
	Status     string    `json:"status,omitempty"`
	Summary    string    `json:"summary,omitempty"`
	Result     string    `json:"result,omitempty"`
	FinishedAt time.Time `json:"finished_at"`
}

type TaskFailure struct {
	Error      string    `json:"error"`
	Retryable  bool      `json:"retryable"`
	OccurredAt time.Time `json:"occurred_at"`
}

type SweepResult struct {
	Requeued int `json:"requeued"`
	Dead     int `json:"dead"`
}

type TaskQueue interface {
	Enqueue(task Task) (TaskReceipt, error)
	Dequeue(worker WorkerRef) (RunningTask, bool, error)
	Ack(taskID string, result TaskResult) error
	Fail(taskID string, failure TaskFailure) error
	Task(taskID string) (Task, bool, error)
	TaskResult(taskID string) (TaskResult, bool, error)
	Events(filter EventFilter, limit int) ([]Event, error)
	Close() error
}

func matchesWorker(task Task, worker WorkerRef) bool {
	if worker.Queue != "" && task.Queue != worker.Queue {
		return false
	}
	if worker.Type != "" && task.Type != worker.Type {
		return false
	}
	if worker.Prefix != "" && !strings.HasPrefix(task.Type, worker.Prefix) {
		return false
	}
	return true
}
