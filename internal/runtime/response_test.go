package runtime

import (
	"testing"

	"github.com/ai4next/superman/internal/bus"
)

func TestFinalResponseCollectorKeepsLastExecutorResponse(t *testing.T) {
	collector := NewFinalResponseCollector("superman_executor")
	for _, event := range []bus.Event{
		{Type: bus.EventTextDelta, Author: "superman_planner", EventID: "plan", Text: "internal plan"},
		{Type: bus.EventTextDelta, Author: "superman_executor", EventID: "execute-1", Text: "partial "},
		{Type: bus.EventTextDelta, Author: "superman_executor", EventID: "execute-1", Text: "answer"},
		{Type: bus.EventTextDelta, Author: "superman_replanner", EventID: "replan", Text: "internal replan"},
		{Type: bus.EventTextDelta, Author: "superman_executor", EventID: "execute-2", Text: "final answer"},
	} {
		collector.Collect(event)
	}
	if got := collector.String(); got != "final answer" {
		t.Fatalf("response = %q", got)
	}
}
