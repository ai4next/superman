package bus

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type ChannelQueue struct {
	mu      sync.Mutex
	ready   chan string
	tasks   map[string]Task
	results map[string]TaskResult
	events  []Event
	broker  Broker
	closed  bool
}

func NewChannelQueue(maxSize int) *ChannelQueue {
	if maxSize <= 0 {
		maxSize = 100
	}
	return &ChannelQueue{
		ready:   make(chan string, maxSize),
		tasks:   make(map[string]Task),
		results: make(map[string]TaskResult),
	}
}

func (q *ChannelQueue) SetBroker(broker Broker) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.broker = broker
}

func (q *ChannelQueue) Enqueue(task Task) (TaskReceipt, error) {
	now := time.Now().UTC()
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return TaskReceipt{}, fmt.Errorf("task queue is closed")
	}
	if strings.TrimSpace(task.ID) == "" {
		task.ID = uuid.NewString()
	}
	if _, ok := q.tasks[task.ID]; ok {
		q.mu.Unlock()
		return TaskReceipt{}, fmt.Errorf("task %q already exists", task.ID)
	}
	if strings.TrimSpace(task.Queue) == "" {
		task.Queue = defaultQueueName
	}
	if task.MaxAttempts <= 0 {
		task.MaxAttempts = defaultMaxAttempts
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	task.UpdatedAt = now
	task.Status = TaskStatusReady
	select {
	case q.ready <- task.ID:
	default:
		q.mu.Unlock()
		return TaskReceipt{}, fmt.Errorf("task queue is full")
	}
	q.tasks[task.ID] = task
	event := Event{Type: EventTaskQueued, At: now, TaskID: task.ID, Metadata: task.Payload}
	q.appendEventLocked(event)
	broker := q.broker
	q.mu.Unlock()
	publishTaskEvent(broker, event)
	return TaskReceipt{TaskID: task.ID, Status: TaskStatusReady}, nil
}

func (q *ChannelQueue) Dequeue(worker WorkerRef) (RunningTask, bool, error) {
	if strings.TrimSpace(worker.ID) == "" {
		worker.ID = "worker-" + uuid.NewString()
	}
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return RunningTask{}, false, fmt.Errorf("task queue is closed")
	}
	for i := 0; i < len(q.ready); i++ {
		taskID := <-q.ready
		task, ok := q.tasks[taskID]
		if !ok || task.Status != TaskStatusReady {
			continue
		}
		if !matchesWorker(task, worker) {
			q.ready <- taskID
			continue
		}
		now := time.Now().UTC()
		task.Status = TaskStatusRunning
		task.Attempt++
		task.UpdatedAt = now
		q.tasks[task.ID] = task
		running := RunningTask{Task: task, WorkerID: worker.ID}
		event := Event{Type: EventTaskStarted, At: now, TaskID: task.ID, WorkerID: worker.ID, Metadata: task.Payload}
		q.appendEventLocked(event)
		broker := q.broker
		q.mu.Unlock()
		publishTaskEvent(broker, event)
		return running, true, nil
	}
	q.mu.Unlock()
	return RunningTask{}, false, nil
}

func (q *ChannelQueue) Ack(taskID string, result TaskResult) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("task id is required")
	}
	now := time.Now().UTC()
	q.mu.Lock()
	task, ok := q.tasks[taskID]
	if !ok {
		q.mu.Unlock()
		return fmt.Errorf("task %q not found", taskID)
	}
	if result.TaskID == "" {
		result.TaskID = task.ID
	}
	if result.Status == "" {
		result.Status = "succeeded"
	}
	if result.FinishedAt.IsZero() {
		result.FinishedAt = now
	}
	q.results[result.TaskID] = result
	task.UpdatedAt = now
	task.Status = TaskStatus("succeeded")
	q.tasks[task.ID] = task
	event := Event{Type: EventTaskSucceeded, At: now, TaskID: result.TaskID, Metadata: task.Payload}
	q.appendEventLocked(event)
	broker := q.broker
	q.mu.Unlock()
	publishTaskEvent(broker, event)
	return nil
}

func (q *ChannelQueue) Fail(taskID string, failure TaskFailure) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("task id is required")
	}
	now := time.Now().UTC()
	if failure.OccurredAt.IsZero() {
		failure.OccurredAt = now
	}
	q.mu.Lock()
	task, ok := q.tasks[taskID]
	if !ok {
		q.mu.Unlock()
		return fmt.Errorf("task %q not found", taskID)
	}
	task.UpdatedAt = now
	var event Event
	if failure.Retryable && task.Attempt < task.MaxAttempts {
		task.Status = TaskStatusReady
		q.tasks[task.ID] = task
		q.ready <- task.ID
		event = Event{Type: EventTaskRetrying, At: now, TaskID: task.ID, Error: failure.Error, Metadata: task.Payload}
	} else {
		task.Status = TaskStatusDead
		q.tasks[task.ID] = task
		event = Event{Type: EventTaskDead, At: now, TaskID: task.ID, Error: failure.Error, Metadata: task.Payload}
	}
	q.appendEventLocked(event)
	broker := q.broker
	q.mu.Unlock()
	publishTaskEvent(broker, event)
	return nil
}

func (q *ChannelQueue) Task(taskID string) (Task, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	task, ok := q.tasks[taskID]
	return task, ok, nil
}

func (q *ChannelQueue) TaskResult(taskID string) (TaskResult, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	result, ok := q.results[taskID]
	return result, ok, nil
}

func (q *ChannelQueue) Events(filter EventFilter, limit int) ([]Event, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []Event
	for _, event := range q.events {
		if !eventMatchesFilter(event, filter) {
			continue
		}
		out = append(out, event)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (q *ChannelQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	return nil
}

func (q *ChannelQueue) appendEventLocked(event Event) {
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.At.IsZero() {
		event.At = time.Now().UTC()
	}
	q.events = append(q.events, event)
}

func publishTaskEvent(broker Broker, event Event) {
	if broker == nil || event.Type == "" {
		return
	}
	_ = broker.Publish(context.Background(), event)
}
