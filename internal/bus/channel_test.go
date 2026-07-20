package bus

import (
	"fmt"
	"testing"
	"time"
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

func TestChannelQueueFailDoesNotHoldLockWhileRetryQueueIsFull(t *testing.T) {
	q := NewChannelQueue(1)
	defer q.Close()

	firstReceipt, err := q.Enqueue(Task{ID: "first", Type: "delegate", MaxAttempts: 2})
	if err != nil {
		t.Fatal(err)
	}
	first, ok, err := q.Dequeue(WorkerRef{ID: "w1"})
	if err != nil || !ok || first.Task.ID != firstReceipt.TaskID {
		t.Fatalf("first dequeue = %#v ok=%v err=%v", first, ok, err)
	}
	if _, err := q.Enqueue(Task{ID: "second", Type: "delegate"}); err != nil {
		t.Fatal(err)
	}

	failDone := make(chan error, 1)
	go func() {
		failDone <- q.Fail(first.Task.ID, TaskFailure{Error: "retry", Retryable: true})
	}()

	dequeued := make(chan RunningTask, 1)
	errs := make(chan error, 1)
	go func() {
		running, ok, err := q.Dequeue(WorkerRef{ID: "w2"})
		if err != nil {
			errs <- err
			return
		}
		if !ok {
			errs <- fmt.Errorf("expected queued task")
			return
		}
		dequeued <- running
	}()

	select {
	case err := <-errs:
		t.Fatal(err)
	case running := <-dequeued:
		if running.Task.ID != "second" {
			t.Fatalf("dequeued task = %q, want second", running.Task.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("dequeue blocked while retry was being scheduled")
	}
	select {
	case err := <-failDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("retry did not finish after a worker dequeued")
	}
}

func TestChannelQueueFailHonorsRetryDelay(t *testing.T) {
	q := NewChannelQueue(1)
	defer q.Close()
	if _, err := q.Enqueue(Task{ID: "retry", Type: "delegate", MaxAttempts: 2}); err != nil {
		t.Fatal(err)
	}
	running, ok, err := q.Dequeue(WorkerRef{ID: "w1"})
	if err != nil || !ok {
		t.Fatalf("dequeue ok=%v err=%v", ok, err)
	}
	if err := q.Fail(running.Task.ID, TaskFailure{Error: "retry", Retryable: true, RetryAfter: 50 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := q.Dequeue(WorkerRef{ID: "w2"}); err != nil || ok {
		t.Fatalf("immediate dequeue ok=%v err=%v", ok, err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		retried, ok, err := q.Dequeue(WorkerRef{ID: "w2"})
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			if retried.Task.ID != "retry" || retried.Task.Attempt != 2 {
				t.Fatalf("retried task=%#v", retried)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("retry was not made available after delay")
}
