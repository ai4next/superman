package bus

import (
	"testing"
)

func TestChannelQueueDequeueAck(t *testing.T) {
	q := NewChannelQueue(10)
	defer q.Close()

	receipt, err := q.Enqueue(Task{Type: "delegate", Queue: "experts", Payload: map[string]string{"expert": "reviewer"}})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.TaskID == "" || receipt.Status != TaskStatusReady {
		t.Fatalf("receipt = %#v", receipt)
	}

	running, ok, err := q.Dequeue(WorkerRef{ID: "w1", Queue: "experts", Type: "delegate"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected task")
	}
	if running.Task.ID != receipt.TaskID || running.Task.Attempt != 1 {
		t.Fatalf("running = %#v", running)
	}

	if err := q.Ack(running.Task.ID, TaskResult{Summary: "done"}); err != nil {
		t.Fatal(err)
	}
	result, ok, err := q.TaskResult(receipt.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || result.TaskID != receipt.TaskID || result.Status != "succeeded" || result.Summary != "done" {
		t.Fatalf("result = %#v ok=%v", result, ok)
	}
}

func TestChannelQueueFailRetriesThenDead(t *testing.T) {
	q := NewChannelQueue(10)
	defer q.Close()

	receipt, err := q.Enqueue(Task{Type: "delegate", Queue: "experts", MaxAttempts: 2})
	if err != nil {
		t.Fatal(err)
	}
	first, ok, err := q.Dequeue(WorkerRef{ID: "w1", Queue: "experts"})
	if err != nil || !ok {
		t.Fatalf("first dequeue ok=%v err=%v", ok, err)
	}
	if err := q.Fail(first.Task.ID, TaskFailure{Error: "temporary", Retryable: true}); err != nil {
		t.Fatal(err)
	}
	second, ok, err := q.Dequeue(WorkerRef{ID: "w2", Queue: "experts"})
	if err != nil || !ok {
		t.Fatalf("second dequeue ok=%v err=%v", ok, err)
	}
	if second.Task.ID != receipt.TaskID || second.Task.Attempt != 2 {
		t.Fatalf("second = %#v", second)
	}
	if err := q.Fail(second.Task.ID, TaskFailure{Error: "final", Retryable: true}); err != nil {
		t.Fatal(err)
	}
	_, ok, err = q.Dequeue(WorkerRef{ID: "w3", Queue: "experts"})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected no ready task after dead")
	}
	events, err := q.Events(EventFilter{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) < 5 || events[len(events)-1].Type != EventTaskDead {
		t.Fatalf("events = %#v", events)
	}
}
