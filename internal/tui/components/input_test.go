package components

import (
	"strings"
	"testing"
)

func TestRenderTextareaComposerUsesTextareaView(t *testing.T) {
	view := RenderTextareaComposer("  > hello\n::: world", 6, 1, 2, 40, false)
	if !strings.Contains(view.Content, "hello") || !strings.Contains(view.Content, "world") {
		t.Fatalf("content = %q", view.Content)
	}
	if !strings.Contains(view.Content, "Up/Down history") {
		t.Fatalf("missing history hint: %q", view.Content)
	}
	if view.Height != 4 {
		t.Fatalf("height = %d, want 4", view.Height)
	}
	if view.CursorX != 7 || view.CursorY != 2 {
		t.Fatalf("cursor = (%d,%d), want (7,2)", view.CursorX, view.CursorY)
	}
}

func TestRenderTextareaComposerShowsCancelHintWhenRunning(t *testing.T) {
	view := RenderTextareaComposer("  > running", 4, 0, 1, 50, true)
	if !strings.Contains(view.Content, "/cancel") {
		t.Fatalf("missing cancel hint: %q", view.Content)
	}
}

func TestRenderComposerUsesVerticalCursor(t *testing.T) {
	view := RenderComposer("你好", 1, 40, false)
	if !strings.Contains(view.Content, "|") {
		t.Fatalf("missing vertical cursor: %q", view.Content)
	}
	if strings.Contains(view.Content, "█") {
		t.Fatalf("block cursor should not be rendered: %q", view.Content)
	}
	if view.CursorX != 5 {
		t.Fatalf("cursorX = %d, want display width after one Chinese rune", view.CursorX)
	}
}
