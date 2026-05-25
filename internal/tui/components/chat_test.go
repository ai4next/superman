package components

import (
	"strings"
	"testing"

	"github.com/ai4next/superman/internal/tui/styles"
)

func TestRenderChatToolStatusUsesIconsOnly(t *testing.T) {
	view := RenderChat([]Message{
		{Role: "tool", Tool: "read", Args: "path=a.go", Status: "done", Duration: "10ms"},
		{Role: "tool", Tool: "write", Args: "path=b.go", Status: "error"},
	}, 80)

	for _, want := range []string{styles.ToolSuccessIcon, styles.ToolErrorIcon, "read", "write", "[10ms]"} {
		if !strings.Contains(view, want) {
			t.Fatalf("tool chat missing %q:\n%s", want, view)
		}
	}
	for _, unwanted := range []string{"done", "[error]"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("tool chat should omit status text %q:\n%s", unwanted, view)
		}
	}
}

func TestToolStatusStylePulsesRunningTool(t *testing.T) {
	off := toolStatusStyle("running", false)
	on := toolStatusStyle("running", true)

	if !off.GetFaint() {
		t.Fatal("pulse off should render running icon faint")
	}
	if !on.GetBold() {
		t.Fatal("pulse on should render running icon bold")
	}
}
