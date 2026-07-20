package runtime

import (
	"strings"

	"github.com/ai4next/superman/internal/bus"
)

// FinalResponseCollector keeps only the last response emitted by the executor.
// Planner and replanner text is internal workflow state, not user-facing output.
type FinalResponseCollector struct {
	author  string
	eventID string
	text    strings.Builder
}

func NewFinalResponseCollector(author string) *FinalResponseCollector {
	return &FinalResponseCollector{author: author}
}

func (c *FinalResponseCollector) Collect(event bus.Event) {
	if c == nil || event.Type != bus.EventTextDelta || event.Author != c.author {
		return
	}
	if event.EventID != "" && event.EventID != c.eventID {
		c.text.Reset()
		c.eventID = event.EventID
	}
	c.text.WriteString(event.Text)
}

func (c *FinalResponseCollector) String() string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.text.String())
}
