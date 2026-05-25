package components

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	supermansession "github.com/ai4next/superman/internal/session"
)

func TestRenderSessionDialog(t *testing.T) {
	view := RenderSessionDialog(SessionDialogData{
		Sessions: []supermansession.Metadata{
			{SessionID: "s1", Title: "First", MessageCount: 2, FileCount: 1},
			{SessionID: "s2", Title: "Second", MessageCount: 3, FileCount: 0},
		},
		Selected: 1,
		Current:  "s1",
	}, 80, 24)
	for _, want := range []string{"Sessions", "s1", "First", "s2", "Second", "Enter switch"} {
		if !strings.Contains(view, want) {
			t.Fatalf("dialog missing %q:\n%s", want, view)
		}
	}
}

func TestRenderCommandDialog(t *testing.T) {
	view := RenderCommandDialog(CommandDialogData{
		Commands: []CommandDialogItem{
			{ID: "new", Title: "New Session", Description: "Start fresh", Key: "ctrl+n"},
			{ID: "files", Title: "Files", Description: "Show files", Key: "/files"},
		},
		Selected: 1,
		Query:    "fi",
	}, 80, 24)
	for _, want := range []string{"Commands", "filter: fi", "New Session", "Files", "Enter run", "/files"} {
		if !strings.Contains(view, want) {
			t.Fatalf("command dialog missing %q:\n%s", want, view)
		}
	}
}

func TestRenderCommandPanel(t *testing.T) {
	view := RenderCommandPanel(CommandDialogData{
		Commands: []CommandDialogItem{
			{ID: "new", Title: "New Session", Description: "Start fresh", Key: "/new"},
			{ID: "files", Title: "Files", Description: "Show files", Key: "/files"},
		},
		Selected: 1,
		Query:    "fi",
	}, 60, 8)
	for _, want := range []string{"Commands / fi", "Files", "/files", "Enter run"} {
		if !strings.Contains(view, want) {
			t.Fatalf("command panel missing %q:\n%s", want, view)
		}
	}
	assertRenderedWidth(t, view, 60)
}

func TestRenderSearchResults(t *testing.T) {
	view := RenderSearchResults(SearchResultsData{
		Results: []supermansession.MessageSearchResult{
			{
				Metadata: supermansession.Metadata{SessionID: "s1", Title: "Cache Work"},
				Message:  supermansession.Message{Role: supermansession.MessageUser},
				Preview:  "Investigate cache invalidation policy",
			},
		},
		Selected: 0,
		Query:    "cache",
	}, 100, 24)
	for _, want := range []string{"Search History", "query: cache", "s1", "Cache Work", "Investigate cache", "Enter switch", "i insert"} {
		if !strings.Contains(view, want) {
			t.Fatalf("search dialog missing %q:\n%s", want, view)
		}
	}
}

func TestRenderDialogsKeepChineseWithinWidth(t *testing.T) {
	width := 64
	sessionView := RenderSessionDialog(SessionDialogData{
		Sessions: []supermansession.Metadata{
			{SessionID: "中文-session", Title: "中文会话标题很长用于测试宽度", MessageCount: 12, FileCount: 3},
		},
		Selected: 0,
		Current:  "中文-session",
	}, width, 20)
	assertRenderedWidth(t, sessionView, width)
	if !strings.Contains(sessionView, "中文") {
		t.Fatalf("session dialog missing Chinese text:\n%s", sessionView)
	}

	commandView := RenderCommandDialog(CommandDialogData{
		Commands: []CommandDialogItem{
			{ID: "search", Title: "搜索历史记录", Description: "查找中文上下文", Key: "/search"},
		},
		Selected: 0,
		Query:    "中文",
	}, width, 20)
	assertRenderedWidth(t, commandView, width)

	searchView := RenderSearchResults(SearchResultsData{
		Results: []supermansession.MessageSearchResult{
			{
				Metadata: supermansession.Metadata{SessionID: "会话一", Title: "中文任务"},
				Message:  supermansession.Message{Role: supermansession.MessageUser},
				Preview:  "这是一段中文搜索结果预览",
			},
		},
		Selected: 0,
		Query:    "中文",
	}, width, 20)
	assertRenderedWidth(t, searchView, width)
}

func assertRenderedWidth(t *testing.T, view string, maxWidth int) {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > maxWidth {
			t.Fatalf("line width = %d > %d:\n%s\nfull view:\n%s", got, maxWidth, line, view)
		}
	}
}
