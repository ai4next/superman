package bus

import (
	"testing"

	"google.golang.org/adk/session"
	"google.golang.org/adk/tool/toolconfirmation"
	"google.golang.org/genai"
)

func TestFromADKEventProjectsTextAndTools(t *testing.T) {
	ev := session.NewEvent("run-1")
	ev.Content = &genai.Content{Parts: []*genai.Part{
		genai.NewPartFromText("hello"),
		genai.NewPartFromFunctionCall("write", map[string]any{"path": "a.txt"}),
		genai.NewPartFromFunctionResponse("write", map[string]any{"status": "success"}),
	}}
	ev.Content.Parts[1].FunctionCall.ID = "tool-1"
	ev.Content.Parts[2].FunctionResponse.ID = "tool-1"

	got := FromADKEvent("session-1", ev)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].Type != EventTextDelta || got[0].Text != "hello" {
		t.Fatalf("text event = %+v", got[0])
	}
	if got[1].Type != EventToolCallStarted || got[1].ToolID != "tool-1" || got[1].ToolName != "write" {
		t.Fatalf("tool start = %+v", got[1])
	}
	if got[2].Type != EventToolCallFinished || got[2].Status != "success" {
		t.Fatalf("tool finish = %+v", got[2])
	}
}

func TestFromADKEventProjectsADKConfirmationCall(t *testing.T) {
	ev := session.NewEvent("run-1")
	ev.Content = genai.NewContentFromFunctionCall(toolconfirmation.FunctionCallName, map[string]any{
		"originalFunctionCall": map[string]any{
			"id":   "write-1",
			"name": "write",
			"args": map[string]any{"path": "a.txt"},
		},
		"toolConfirmation": map[string]any{"hint": "approve write"},
	}, genai.RoleModel)
	ev.Content.Parts[0].FunctionCall.ID = "confirm-1"

	got := FromADKEvent("session-1", ev)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Type != EventPermissionRequested || got[0].ToolID != "confirm-1" || got[0].ToolName != "write" {
		t.Fatalf("confirmation event = %+v", got[0])
	}
	if got[0].Args == "" || got[0].Args == "{}" {
		t.Fatalf("confirmation args should include original call args: %+v", got[0])
	}
}

func TestEvolutionEvents(t *testing.T) {
	started := EvolutionStarted("s1", "superman")
	if started.Type != EventEvolutionStarted || started.SessionID != "s1" || started.Role != "superman" {
		t.Fatalf("started = %+v", started)
	}
	finished := EvolutionFinished("s1", "superman", "/tmp/projection.md")
	if finished.Type != EventEvolutionFinished || finished.Path != "/tmp/projection.md" {
		t.Fatalf("finished = %+v", finished)
	}
	failed := EvolutionFailed("s1", "superman", assertErr("boom"))
	if failed.Type != EventEvolutionFailed || failed.Error != "boom" {
		t.Fatalf("failed = %+v", failed)
	}
}

func TestSessionCompactedEvent(t *testing.T) {
	got := SessionCompacted("s1", 12)
	if got.Type != EventSessionCompacted || got.SessionID != "s1" || got.Count != 12 {
		t.Fatalf("event = %+v", got)
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
