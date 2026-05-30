package bus

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMemoryBrokerFiltersAndPublishes(t *testing.T) {
	broker := NewMemoryBroker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := broker.Subscribe(ctx, EventFilter{Types: []EventType{EventTaskQueued}, TaskID: "t1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := broker.Publish(ctx, Event{Type: EventTaskStarted, TaskID: "t1"}); err != nil {
		t.Fatal(err)
	}
	if err := broker.Publish(ctx, Event{Type: EventTaskQueued, TaskID: "t2"}); err != nil {
		t.Fatal(err)
	}
	if err := broker.Publish(ctx, Event{Type: EventTaskQueued, TaskID: "t1"}); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-events:
		if event.Type != EventTaskQueued || event.TaskID != "t1" {
			t.Fatalf("event = %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestAuditLoggerWritesSubscribedEvents(t *testing.T) {
	broker := NewMemoryBroker()
	path := filepath.Join(t.TempDir(), "events.jsonl")
	logger := NewAuditLogger(path)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := logger.Subscribe(ctx, broker, EventFilter{}); err != nil {
		t.Fatal(err)
	}
	if err := broker.Publish(ctx, Event{Type: EventTaskQueued, TaskID: "t1"}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		data, err := os.ReadFile(path)
		if err == nil && len(data) > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("audit log not written: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestReadAuditLogFiltersAndLimits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	logger := NewAuditLogger(path)
	events := []Event{
		{Type: EventRunStarted, SessionID: "s1", RunID: "r1", At: time.Unix(1, 0)},
		{Type: EventToolCallStarted, SessionID: "s1", RunID: "r1", ToolName: "read", At: time.Unix(2, 0)},
		{Type: EventRunStarted, SessionID: "s2", RunID: "r2", At: time.Unix(3, 0)},
		{Type: EventRunFailed, SessionID: "s1", RunID: "r1", Error: "boom", At: time.Unix(4, 0)},
	}
	for _, event := range events {
		if err := logger.Write(event); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}

	got, err := ReadAuditLog(path, AuditFilter{
		SessionID: "s1",
		Types:     []EventType{EventRunStarted, EventRunFailed},
		Limit:     1,
	})
	if err != nil {
		t.Fatalf("ReadAuditLog: %v", err)
	}
	if len(got) != 1 || got[0].Type != EventRunFailed || got[0].Error != "boom" {
		t.Fatalf("events = %#v", got)
	}
}

func TestDecodeAuditEventsReportsLineNumber(t *testing.T) {
	_, err := DecodeAuditEvents(strings.NewReader("{bad json}\n"), AuditFilter{})
	if err == nil || !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("err = %v, want line number", err)
	}
}

func TestSummarizeAuditEvents(t *testing.T) {
	events := []Event{
		{Type: EventRunStarted, SessionID: "s1", RunID: "r1", At: time.Unix(1, 0)},
		{Type: EventToolCallStarted, SessionID: "s1", RunID: "r1", ToolName: "read", At: time.Unix(2, 0)},
		{Type: EventToolCallFinished, SessionID: "s1", RunID: "r1", ToolName: "read", At: time.Unix(3, 0)},
		{Type: EventRunFailed, SessionID: "s2", RunID: "r2", Error: "boom", At: time.Unix(5, 0)},
		{Type: EventTaskDead, TaskID: "t1", Error: "dead", At: time.Unix(6, 0)},
	}
	got := SummarizeAuditEvents(events)
	if got.Events != 5 || got.ByType[EventRunStarted] != 1 || got.Tools["read"] != 2 || got.Sessions["s1"] != 3 || got.Tasks["t1"] != 1 || got.Errors != 2 {
		t.Fatalf("summary = %#v", got)
	}
	if got.Duration != "5s" {
		t.Fatalf("duration = %q, want 5s", got.Duration)
	}
}
