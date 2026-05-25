package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAuditLoggerWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime", "events.jsonl")
	logger := NewAuditLogger(path)
	want := Event{Type: EventToolCallStarted, SessionID: "s1", ToolName: "write"}
	if err := logger.Write(want); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got := readAuditEvents(t, path)
	if len(got) != 1 {
		t.Fatalf("events len = %d, want 1", len(got))
	}
	if got[0].Type != want.Type || got[0].ToolName != want.ToolName {
		t.Fatalf("event = %+v, want %+v", got[0], want)
	}
}

func TestAuditLoggerSubscribe(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime", "events.jsonl")
	logger := NewAuditLogger(path)
	broker := NewBroker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger.Subscribe(ctx, broker)

	broker.Publish(Event{Type: EventRunStarted, SessionID: "s1"})
	deadline := time.After(time.Second)
	for {
		if events := readAuditEventsIfExists(t, path); len(events) == 1 {
			return
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for audit event")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestReadAuditLogFiltersAndLimits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime", "events.jsonl")
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
	}
	got := SummarizeAuditEvents(events)
	if got.Events != 4 || got.ByType[EventRunStarted] != 1 || got.Tools["read"] != 2 || got.Sessions["s1"] != 3 || got.Errors != 1 {
		t.Fatalf("summary = %#v", got)
	}
	if got.Duration != "4s" {
		t.Fatalf("duration = %q, want 4s", got.Duration)
	}
}

func readAuditEvents(t *testing.T, path string) []Event {
	t.Helper()
	events := readAuditEventsIfExists(t, path)
	if len(events) == 0 {
		t.Fatalf("no audit events in %s", path)
	}
	return events
}

func readAuditEventsIfExists(t *testing.T, path string) []Event {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("open audit log: %v", err)
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatalf("decode event: %v", err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan audit log: %v", err)
	}
	return events
}
