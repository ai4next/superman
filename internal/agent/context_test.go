package agent

import (
	"strings"
	"testing"

	supermansession "github.com/ai4next/superman/internal/session"
)

func TestInstructionProviderDoesNotIncludeSessionContext(t *testing.T) {
	provider := instructionProvider(BuildConfig{
		Instruction:     "base instruction",
		ContextMessages: 12,
	})

	got, err := provider(nil, nil)
	if err != nil {
		t.Fatalf("instructionProvider returned error: %v", err)
	}
	if !strings.Contains(got, "base instruction") {
		t.Fatalf("instruction missing base text: %q", got)
	}
	if strings.Contains(got, "Session Context") {
		t.Fatalf("instruction contains session context: %q", got)
	}
}

func TestSessionContextContentsIncludesHistoricalContext(t *testing.T) {
	contents := sessionContextContents(sessionContext{
		Summary:     "earlier decisions",
		MaxMessages: 12,
		Messages: []supermansession.Message{
			{Role: supermansession.MessageUser, Content: "old user request"},
			{Role: supermansession.MessageAssistant, Content: "old assistant answer"},
			{Role: supermansession.MessageUser, Content: "current request"},
		},
	})

	if len(contents) != 1 {
		t.Fatalf("len(contents) = %d, want 1", len(contents))
	}
	got := contents[0].Parts[0].Text
	for _, want := range []string{"Session Context Usage", "Session Summary", "earlier decisions", "old user request", "old assistant answer"} {
		if !strings.Contains(got, want) {
			t.Fatalf("contents missing %q: %q", want, got)
		}
	}
	if strings.Contains(got, "current request") {
		t.Fatalf("contents should not duplicate current request: %q", got)
	}
}

func TestSessionContextContentsSnipsMiddleMessages(t *testing.T) {
	var messages []supermansession.Message
	for i := range 8 {
		messages = append(messages, supermansession.Message{
			Role:    supermansession.MessageUser,
			Content: "message " + string(rune('0'+i)),
		})
	}

	contents := sessionContextContents(sessionContext{Messages: messages, MaxMessages: 5})
	got := contents[0].Parts[0].Text

	for _, want := range []string{"message 0", "message 1", "message 2", "[snipped 3 older session messages]", "message 6"} {
		if !strings.Contains(got, want) {
			t.Fatalf("contents missing %q: %q", want, got)
		}
	}
	if strings.Contains(got, "message 3") || strings.Contains(got, "message 4") || strings.Contains(got, "message 5") {
		t.Fatalf("contents retained snipped middle messages: %q", got)
	}
	if strings.Contains(got, "message 7") {
		t.Fatalf("contents should not duplicate current request: %q", got)
	}
}

func TestSessionContextContentsMicroCompactsOldToolResults(t *testing.T) {
	long := strings.Repeat("x", contextMicroCompactRunes+1)
	window := sessionContext{MaxMessages: 12}
	for i := range 5 {
		window.Messages = append(window.Messages, supermansession.Message{
			Role:     supermansession.MessageTool,
			ToolName: "read",
			Result:   long + string(rune('0'+i)),
		})
	}
	window.Messages = append(window.Messages, supermansession.Message{Role: supermansession.MessageUser, Content: "current"})

	contents := sessionContextContents(window)
	got := contents[0].Parts[0].Text

	if count := strings.Count(got, "Earlier tool result compacted"); count != 2 {
		t.Fatalf("compacted old tool result count = %d, want 2: %q", count, got)
	}
	if !strings.Contains(got, strings.Repeat("x", 20)) {
		t.Fatalf("recent tool results were all compacted: %q", got)
	}
}

func TestCompactSessionContextAppliesToolResultBudget(t *testing.T) {
	large := strings.Repeat("z", contextPersistThresholdRunes+1)
	window := sessionContext{MaxMessages: 12}
	for i := range 7 {
		window.Messages = append(window.Messages, supermansession.Message{
			ID:     string(rune('a' + i)),
			Role:   supermansession.MessageTool,
			Result: large,
		})
	}

	compacted := compactSessionContext(window)

	combined := compacted.Summary
	for _, msg := range compacted.Messages {
		combined += msg.Result
	}
	if !strings.Contains(combined, "<persisted-output>") {
		t.Fatalf("large result was not budget-compacted: %q", combined)
	}
}

func TestCompactSessionContextAutoCompactsOversizedWindow(t *testing.T) {
	window := sessionContext{
		MaxMessages: 12,
		Messages: []supermansession.Message{
			{Role: supermansession.MessageUser, Content: strings.Repeat("a", contextAutoCompactLimitRunes+1)},
			{Role: supermansession.MessageAssistant, Content: "recent answer"},
			{Role: supermansession.MessageUser, Content: "current request"},
		},
	}

	compacted := compactSessionContext(window)

	if !strings.Contains(compacted.Summary, "Auto-compacted session context") {
		t.Fatalf("missing auto compact summary: %q", compacted.Summary)
	}
	if len(compacted.Messages) != 3 {
		t.Fatalf("len(compacted.Messages) = %d, want 3", len(compacted.Messages))
	}
}
