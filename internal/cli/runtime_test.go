package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	supermanruntime "github.com/ai4next/superman/internal/runtime"
)

func TestWriteRuntimeEventsTextAndJSON(t *testing.T) {
	events := []supermanruntime.Event{
		{Type: supermanruntime.EventRunStarted, SessionID: "1", RunID: "r1", At: time.Unix(1, 0)},
		{Type: supermanruntime.EventToolCallStarted, SessionID: "1", RunID: "r1", ToolName: "read", Args: `{"path":"main.go"}`, At: time.Unix(2, 0)},
	}
	var buf bytes.Buffer
	if err := writeRuntimeEvents(&buf, events, false); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "TIME") || !strings.Contains(out, "tool_call_started") || !strings.Contains(out, "main.go") {
		t.Fatalf("runtime events = %s", out)
	}

	buf.Reset()
	if err := writeRuntimeEvents(&buf, events, true); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, `"type": "run_started"`) || !strings.Contains(out, `"session_id": "1"`) {
		t.Fatalf("runtime events json = %s", out)
	}
}

func TestWriteRuntimeEventsEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := writeRuntimeEvents(&buf, nil, false); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "No runtime events") {
		t.Fatalf("empty events = %s", out)
	}
}

func TestWriteRuntimeSummary(t *testing.T) {
	summary := supermanruntime.SummarizeAuditEvents([]supermanruntime.Event{
		{Type: supermanruntime.EventRunStarted, SessionID: "1", RunID: "r1", At: time.Unix(1, 0)},
		{Type: supermanruntime.EventToolCallStarted, SessionID: "1", RunID: "r1", ToolName: "read", At: time.Unix(2, 0)},
		{Type: supermanruntime.EventRunFailed, SessionID: "1", RunID: "r1", Error: "boom", At: time.Unix(3, 0)},
	})
	var buf bytes.Buffer
	if err := writeRuntimeSummary(&buf, summary, false); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "Events: 3") || !strings.Contains(out, "Errors: 1") || !strings.Contains(out, "read: 1") {
		t.Fatalf("runtime summary = %s", out)
	}

	buf.Reset()
	if err := writeRuntimeSummary(&buf, summary, true); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, `"events": 3`) || !strings.Contains(out, `"errors": 1`) {
		t.Fatalf("runtime summary json = %s", out)
	}
}

func TestParseRuntimeEventTypes(t *testing.T) {
	got, err := parseRuntimeEventTypes("run_started, tool_call_finished")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != supermanruntime.EventRunStarted || got[1] != supermanruntime.EventToolCallFinished {
		t.Fatalf("types = %#v", got)
	}
}
