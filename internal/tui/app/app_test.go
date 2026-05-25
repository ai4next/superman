package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	supermanagent "github.com/ai4next/superman/internal/agent"
	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/global"
	supermanruntime "github.com/ai4next/superman/internal/runtime"
	supermansession "github.com/ai4next/superman/internal/session"
	"github.com/ai4next/superman/internal/tui/components"
	"google.golang.org/adk/runner"
	adksession "google.golang.org/adk/session"
	"google.golang.org/genai"
)

func TestApplyRuntimeEventCompletesRun(t *testing.T) {
	m := &Model{
		running:    true,
		toolStarts: make(map[string]time.Time),
	}

	m.applyRuntimeEvent(supermanruntime.Event{Type: supermanruntime.EventTextDelta, Text: "hello"})
	m.applyRuntimeEvent(supermanruntime.Event{
		Type:     supermanruntime.EventToolCallStarted,
		ToolID:   "tool-1",
		ToolName: "write",
		Args:     `{"path":"a.txt"}`,
	})
	m.applyRuntimeEvent(supermanruntime.Event{
		Type:     supermanruntime.EventToolCallFinished,
		ToolID:   "tool-1",
		ToolName: "write",
		Status:   "done",
		Result:   `{"ok":true}`,
	})
	m.applyRuntimeEvent(supermanruntime.Event{Type: supermanruntime.EventRunFinished})

	if m.running {
		t.Fatal("run should be finished")
	}
	if len(m.messages) != 2 {
		t.Fatalf("messages len = %d, want 2", len(m.messages))
	}
	if m.messages[0].Role != "agent" || m.messages[0].Content != "hello" {
		t.Fatalf("agent message = %+v", m.messages[0])
	}
	if m.messages[1].Role != "tool" || m.messages[1].Status != "done" {
		t.Fatalf("tool message = %+v", m.messages[1])
	}
}

func TestApplyRuntimeEventKeepsInterleavedOutputOrder(t *testing.T) {
	m := &Model{
		running:    true,
		toolStarts: make(map[string]time.Time),
	}

	m.applyRuntimeEvent(supermanruntime.Event{Type: supermanruntime.EventTextDelta, Text: "before "})
	m.applyRuntimeEvent(supermanruntime.Event{Type: supermanruntime.EventTextDelta, Text: "tool"})
	m.applyRuntimeEvent(supermanruntime.Event{
		Type:     supermanruntime.EventToolCallStarted,
		ToolID:   "tool-1",
		ToolName: "read",
		Args:     `{"path":"a.txt"}`,
	})
	m.applyRuntimeEvent(supermanruntime.Event{
		Type:     supermanruntime.EventToolCallFinished,
		ToolID:   "tool-1",
		ToolName: "read",
		Status:   "done",
		Result:   `{"ok":true}`,
	})
	m.applyRuntimeEvent(supermanruntime.Event{Type: supermanruntime.EventTextDelta, Text: " after"})
	m.applyRuntimeEvent(supermanruntime.Event{Type: supermanruntime.EventRunFinished})

	if len(m.messages) != 3 {
		t.Fatalf("messages len = %d, want 3: %+v", len(m.messages), m.messages)
	}
	if m.messages[0].Role != "agent" || m.messages[0].Content != "before tool" {
		t.Fatalf("first message = %+v", m.messages[0])
	}
	if m.messages[1].Role != "tool" || m.messages[1].Tool != "read" || m.messages[1].Status != "done" {
		t.Fatalf("second message = %+v", m.messages[1])
	}
	if m.messages[2].Role != "agent" || m.messages[2].Content != " after" {
		t.Fatalf("third message = %+v", m.messages[2])
	}
}

func TestApplyRuntimeEventCancelsRun(t *testing.T) {
	m := &Model{
		running:        true,
		toolStarts:     make(map[string]time.Time),
		chatCacheDirty: false,
	}
	m.applyRuntimeEvent(supermanruntime.Event{Type: supermanruntime.EventRunCanceled})
	if m.running {
		t.Fatal("running should be false after cancel")
	}
	if len(m.messages) == 0 || !strings.Contains(m.messages[len(m.messages)-1].Content, "canceled") {
		t.Fatalf("messages = %+v", m.messages)
	}
}

func TestPermissionRequestSurvivesRunFinished(t *testing.T) {
	m := &Model{
		running:    true,
		toolStarts: make(map[string]time.Time),
	}

	m.applyRuntimeEvent(supermanruntime.Event{
		Type:     supermanruntime.EventPermissionRequested,
		ToolID:   "confirm-1",
		ToolName: "write",
		Args:     `{"path":"a.txt"}`,
	})
	m.applyRuntimeEvent(supermanruntime.Event{Type: supermanruntime.EventRunFinished})

	if m.pendingConfirm == nil || m.pendingConfirm.ID != "confirm-1" {
		t.Fatalf("pending confirmation = %+v", m.pendingConfirm)
	}
	if m.running {
		t.Fatal("run should pause after confirmation request")
	}
}

func TestNewInitializesRuntimeAuditLogger(t *testing.T) {
	cfg := &config.Config{
		Workspace: t.TempDir(),
		Model:     config.ModelConfig{Provider: "test", Name: "model"},
	}
	global.SetConfig(cfg)
	t.Cleanup(func() { global.SetConfig(nil) })

	m := New(nil, cfg, runner.PluginConfig{}, adksession.InMemoryService())
	if m.runtimeBroker == nil {
		t.Fatal("runtime broker should be initialized")
	}
	if m.auditLogger == nil {
		t.Fatal("audit logger should be initialized")
	}
}

func TestNewCreatesInitialPersistentSession(t *testing.T) {
	cfg := &config.Config{
		Workspace: t.TempDir(),
		Model:     config.ModelConfig{Provider: "test", Name: "model"},
		Session:   config.SessionConfig{AppName: "app"},
	}
	global.SetConfig(cfg)
	t.Cleanup(func() { global.SetConfig(nil) })
	svc, err := supermansession.NewService()
	if err != nil {
		t.Fatal(err)
	}

	m := New(nil, cfg, runner.PluginConfig{}, svc)
	if m.sessionID == "" {
		t.Fatal("sessionID should be initialized")
	}
	if _, err := svc.Metadata("app", "tui-user", m.sessionID); err != nil {
		t.Fatalf("initial session should exist: %v", err)
	}
}

func TestCompactCommand(t *testing.T) {
	cfg := &config.Config{
		Workspace: t.TempDir(),
		Model:     config.ModelConfig{Provider: "test", Name: "model"},
		Session:   config.SessionConfig{AppName: "app", MaxTurns: 2},
	}
	global.SetConfig(cfg)
	t.Cleanup(func() { global.SetConfig(nil) })
	svc, err := supermansession.NewService()
	if err != nil {
		t.Fatal(err)
	}
	m := New(nil, cfg, runner.PluginConfig{}, svc)

	m.input = "/compact"
	m.cursorPos = len(m.input)
	updated, _ := m.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	model := updated.(*Model)

	if model.input != "" {
		t.Fatalf("input = %q, want empty", model.input)
	}
	if len(model.messages) != 1 || model.messages[0].Role != "system" {
		t.Fatalf("messages = %+v", model.messages)
	}
}

func TestFilesCommand(t *testing.T) {
	cfg := &config.Config{
		Workspace: t.TempDir(),
		Model:     config.ModelConfig{Provider: "test", Name: "model"},
		Session:   config.SessionConfig{AppName: "app"},
	}
	global.SetConfig(cfg)
	t.Cleanup(func() { global.SetConfig(nil) })
	svc, err := supermansession.NewService()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(t.Context(), &adksession.CreateRequest{AppName: "app", UserID: "tui-user", SessionID: "1"}); err != nil {
		t.Fatal(err)
	}
	path := cfg.Workspace + "/main.go"
	if err := svc.RecordFileRead("app", "tui-user", "1", path); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecordFileRevision("app", "tui-user", "1", path, "patch", "old", "new", false); err != nil {
		t.Fatal(err)
	}

	m := New(nil, cfg, runner.PluginConfig{}, svc)
	m.input = "/files"
	m.cursorPos = len(m.input)
	updated, _ := m.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	model := updated.(*Model)

	if model.fileCount != 1 {
		t.Fatalf("fileCount = %d, want 1", model.fileCount)
	}
	if len(model.messages) == 0 || !strings.Contains(model.messages[len(model.messages)-1].Content, "main.go") || !strings.Contains(model.messages[len(model.messages)-1].Content, "+1 -1") {
		t.Fatalf("messages = %+v", model.messages)
	}
}

func TestProcessInputClearsTextareaValue(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	m := New(nil, cfg, runner.PluginConfig{}, svc)
	m.setInput("hello")

	updated, _ := m.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	model := updated.(*Model)

	if model.input != "" || model.inputValue() != "" || model.textarea.Value() != "" {
		t.Fatalf("input not cleared: input=%q inputValue=%q textarea=%q", model.input, model.inputValue(), model.textarea.Value())
	}
}

func TestPromptHistoryNavigation(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	created, err := svc.Get(t.Context(), &adksession.GetRequest{AppName: "app", UserID: "tui-user", SessionID: "1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"first prompt", "second prompt"} {
		event := adksession.NewEvent("inv")
		event.Author = "user"
		event.Content = genai.NewContentFromText(text, genai.RoleUser)
		if err := svc.AppendEvent(t.Context(), created.Session, event); err != nil {
			t.Fatal(err)
		}
	}
	m := New(nil, cfg, runner.PluginConfig{}, svc)
	m.setInput("draft")

	updated, _ := m.handleKey(tea.KeyPressMsg{Code: tea.KeyUp})
	model := updated.(*Model)
	if model.input != "second prompt" {
		t.Fatalf("input after first up = %q", model.input)
	}
	updated, _ = model.handleKey(tea.KeyPressMsg{Code: tea.KeyUp})
	model = updated.(*Model)
	if model.input != "first prompt" {
		t.Fatalf("input after second up = %q", model.input)
	}
	updated, _ = model.handleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	model = updated.(*Model)
	if model.input != "second prompt" {
		t.Fatalf("input after first down = %q", model.input)
	}
	updated, _ = model.handleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	model = updated.(*Model)
	if model.input != "draft" {
		t.Fatalf("input after second down = %q", model.input)
	}
}

func TestChineseTextInput(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	m := New(nil, cfg, runner.PluginConfig{}, svc)

	updated, _ := m.handleKey(tea.KeyPressMsg{Text: "你", Code: '你'})
	model := updated.(*Model)
	updated, _ = model.handleKey(tea.KeyPressMsg{Text: "好", Code: '好'})
	model = updated.(*Model)

	if model.inputValue() != "你好" {
		t.Fatalf("input = %q, want 你好", model.inputValue())
	}
	if model.cursorPos != 2 {
		t.Fatalf("cursorPos = %d, want 2 runes", model.cursorPos)
	}
}

func TestChineseTextInputFallsBackToPrintableCode(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	m := New(nil, cfg, runner.PluginConfig{}, svc)

	updated, _ := m.handleKey(tea.KeyPressMsg{Code: '中'})
	model := updated.(*Model)

	if model.inputValue() != "中" {
		t.Fatalf("input = %q, want 中", model.inputValue())
	}
}

func TestViewUsesTextareaBarCursorForChineseInput(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	m := New(nil, cfg, runner.PluginConfig{}, svc)
	m.width = 80
	m.height = 24
	m.setInput("你好")

	view := m.View()
	if view.Cursor == nil {
		t.Fatal("missing cursor")
	}
	if view.Cursor.Shape != tea.CursorBar {
		t.Fatalf("cursor shape = %v, want bar", view.Cursor.Shape)
	}
	if !strings.Contains(view.Content, "你好") {
		t.Fatalf("view missing chinese input:\n%s", view.Content)
	}
	if m.cursorCol <= 0 {
		t.Fatalf("cursor col = %d, want composer position after input prompt", m.cursorCol)
	}
	if view.Cursor.X != m.cursorCol {
		t.Fatalf("cursor x = %d, want composer cursor col %d", view.Cursor.X, m.cursorCol)
	}
}

func TestViewCursorStaysAfterWideChineseText(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	m := New(nil, cfg, runner.PluginConfig{}, svc)
	m.width = 80
	m.height = 24
	m.setInput("你好啊")

	view := m.View()
	if !strings.Contains(view.Content, "你好啊") {
		t.Fatalf("view should keep all wide chinese text visible at cursor:\n%s", view.Content)
	}
	if view.Cursor.X < 11 {
		t.Fatalf("cursor x = %d, want after prompt and wide chinese text", view.Cursor.X)
	}
}

func TestViewEnablesMouseWheelEvents(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	m := New(nil, cfg, runner.PluginConfig{}, svc)
	m.width = 80
	m.height = 24

	view := m.View()
	if view.MouseMode != tea.MouseModeCellMotion {
		t.Fatalf("mouse mode = %v, want cell motion", view.MouseMode)
	}
}

func TestMouseWheelScrollsChatArea(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	m := New(nil, cfg, runner.PluginConfig{}, svc)
	m.width = 80
	m.height = 24

	updated, _ := m.handleMouseWheel(tea.MouseWheelMsg{X: 10, Y: 2, Button: tea.MouseWheelUp})
	model := updated.(*Model)
	if model.scrollOffset != model.scrollWheelStep() {
		t.Fatalf("scrollOffset = %d, want %d", model.scrollOffset, model.scrollWheelStep())
	}

	updated, _ = model.handleMouseWheel(tea.MouseWheelMsg{X: 10, Y: 2, Button: tea.MouseWheelDown})
	model = updated.(*Model)
	if model.scrollOffset != 0 {
		t.Fatalf("scrollOffset after wheel down = %d, want 0", model.scrollOffset)
	}
}

func TestMouseWheelIgnoresComposerArea(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	m := New(nil, cfg, runner.PluginConfig{}, svc)
	m.width = 80
	m.height = 24
	composerY := m.chatViewportHeight() + 1

	updated, _ := m.handleMouseWheel(tea.MouseWheelMsg{X: 10, Y: composerY, Button: tea.MouseWheelUp})
	model := updated.(*Model)
	if model.scrollOffset != 0 {
		t.Fatalf("scrollOffset = %d, want 0", model.scrollOffset)
	}
}

func TestMouseDragHighlightsAndCopiesChatText(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	m := New(nil, cfg, runner.PluginConfig{}, svc)
	m.width = 80
	m.height = 24
	m.showWelcome = false
	m.messages = []components.Message{{Role: "system", Content: "hello selectable text"}}
	m.chatCacheDirty = true

	updated, _ := m.handleMouseClick(tea.MouseClickMsg{X: 0, Y: 1, Button: tea.MouseLeft})
	model := updated.(*Model)
	updated, _ = model.handleMouseMotion(tea.MouseMotionMsg{X: 8, Y: 1, Button: tea.MouseLeft})
	model = updated.(*Model)
	if !model.selection.Active {
		t.Fatal("selection should be active after drag")
	}
	if selected := model.selectedChatText(); selected == "" {
		t.Fatal("selected text should not be empty")
	}
	view := model.View()
	if !strings.Contains(view.Content, "\x1b[7m") {
		t.Fatalf("view missing reverse highlight:\n%s", view.Content)
	}

	updated, cmd := model.handleMouseRelease(tea.MouseReleaseMsg{X: 8, Y: 1, Button: tea.MouseLeft})
	model = updated.(*Model)
	if !model.selection.Active {
		t.Fatal("selection highlight should remain after release")
	}
	if cmd == nil {
		t.Fatal("release should return clipboard command")
	}
}

func TestMouseClickWithoutDragClearsSelection(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	m := New(nil, cfg, runner.PluginConfig{}, svc)
	m.width = 80
	m.height = 24

	updated, _ := m.handleMouseClick(tea.MouseClickMsg{X: 3, Y: 2, Button: tea.MouseLeft})
	model := updated.(*Model)
	updated, _ = model.handleMouseRelease(tea.MouseReleaseMsg{X: 3, Y: 2, Button: tea.MouseLeft})
	model = updated.(*Model)
	if model.selection.Active {
		t.Fatal("single click without drag should clear selection")
	}
}

func TestCutPlainLineHandlesChineseWidth(t *testing.T) {
	if got := cutPlainLine("你好abc", 0, 4); got != "你好" {
		t.Fatalf("cut = %q, want 你好", got)
	}
	if got := cutPlainLine("你好abc", 4, 6); got != "ab" {
		t.Fatalf("cut = %q, want ab", got)
	}
}

func TestSessionDialogKeyboardWorkflow(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	if _, err := svc.Create(t.Context(), &adksession.CreateRequest{AppName: "app", UserID: "tui-user", SessionID: "2"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Rename("app", "tui-user", "2", "Work"); err != nil {
		t.Fatal(err)
	}
	m := New(nil, cfg, runner.PluginConfig{}, svc)

	updated, _ := m.handleKey(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	model := updated.(*Model)
	if model.sessionDialog == nil || len(model.sessionDialog.Sessions) != 2 {
		t.Fatalf("sessionDialog = %+v", model.sessionDialog)
	}
	target := 0
	for i, meta := range model.sessionDialog.Sessions {
		if meta.SessionID == "2" {
			target = i
			break
		}
	}
	for model.sessionDialog.Selected < target {
		updated, _ = model.handleKey(tea.KeyPressMsg{Code: tea.KeyDown})
		model = updated.(*Model)
	}
	for model.sessionDialog.Selected > target {
		updated, _ = model.handleKey(tea.KeyPressMsg{Code: tea.KeyUp})
		model = updated.(*Model)
	}
	updated, _ = model.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(*Model)
	if model.sessionDialog != nil {
		t.Fatal("session dialog should close after enter")
	}
	if model.sessionID != "2" {
		t.Fatalf("sessionID = %q, want work", model.sessionID)
	}
}

func TestCommandDialogKeyboardWorkflow(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	m := New(nil, cfg, runner.PluginConfig{}, svc)

	updated, _ := m.handleKey(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	model := updated.(*Model)
	if model.commandDialog == nil || len(model.commandDialog.Commands) == 0 {
		t.Fatalf("commandDialog = %+v", model.commandDialog)
	}
	target := 0
	for i, command := range model.commandDialog.Commands {
		if command.ID == "new" {
			target = i
			break
		}
	}
	for model.commandDialog.Selected < target {
		updated, _ = model.handleKey(tea.KeyPressMsg{Code: tea.KeyDown})
		model = updated.(*Model)
	}
	updated, _ = model.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(*Model)
	if model.commandDialog != nil {
		t.Fatal("command dialog should close after enter")
	}
	if model.sessionID == "1" {
		t.Fatalf("sessionID = %q, want a new session", model.sessionID)
	}
}

func TestSlashOpensCommandDialogOnEmptyInput(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	m := New(nil, cfg, runner.PluginConfig{}, svc)

	updated, _ := m.handleKey(tea.KeyPressMsg{Text: "/", Code: '/'})
	model := updated.(*Model)
	if model.commandDialog == nil {
		t.Fatal("slash should open command dialog")
	}
	if model.inputValue() != "" {
		t.Fatalf("input = %q, want empty", model.inputValue())
	}
}

func TestCommandPanelRendersBelowComposer(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	m := New(nil, cfg, runner.PluginConfig{}, svc)
	m.width = 80
	m.height = 24

	updated, _ := m.handleKey(tea.KeyPressMsg{Text: "/", Code: '/'})
	model := updated.(*Model)
	view := model.View()
	composerIdx := strings.Index(view.Content, "╭ message ")
	panelIdx := strings.Index(view.Content, "Commands")
	if composerIdx < 0 || panelIdx < 0 {
		t.Fatalf("view missing composer or command panel:\n%s", view.Content)
	}
	if panelIdx < composerIdx {
		t.Fatalf("command panel should render below composer:\n%s", view.Content)
	}
}

func TestSlashDoesNotOpenCommandDialogInsidePrompt(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	m := New(nil, cfg, runner.PluginConfig{}, svc)
	m.setInput("open ")

	updated, _ := m.handleKey(tea.KeyPressMsg{Text: "/", Code: '/'})
	model := updated.(*Model)
	if model.commandDialog != nil {
		t.Fatal("slash inside prompt should not open command dialog")
	}
	if model.inputValue() != "open /" {
		t.Fatalf("input = %q, want open /", model.inputValue())
	}
}

func TestCommandDialogFilterWorkflow(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	path := filepath.Join(cfg.Workspace, "main.go")
	if _, err := svc.RecordFileRevision("app", "tui-user", "1", path, "patch", "old", "new", false); err != nil {
		t.Fatal(err)
	}
	m := New(nil, cfg, runner.PluginConfig{}, svc)
	updated, _ := m.handleKey(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	model := updated.(*Model)

	for _, r := range "files" {
		updated, _ = model.handleKey(tea.KeyPressMsg{Text: string(r), Code: r})
		model = updated.(*Model)
	}
	filtered := model.filteredCommandDialogItems()
	if len(filtered) != 1 || filtered[0].ID != "files" {
		t.Fatalf("filtered = %#v", filtered)
	}
	updated, _ = model.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(*Model)
	if model.commandDialog != nil {
		t.Fatal("command dialog should close after command")
	}
	if len(model.messages) == 0 || !strings.Contains(model.messages[len(model.messages)-1].Content, "main.go") {
		t.Fatalf("messages = %+v", model.messages)
	}
}

func TestCommandDialogSearchActionPrefillsSlashCommand(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	m := New(nil, cfg, runner.PluginConfig{}, svc)
	updated, _ := m.handleKey(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	model := updated.(*Model)

	for _, r := range "search" {
		updated, _ = model.handleKey(tea.KeyPressMsg{Text: string(r), Code: r})
		model = updated.(*Model)
	}
	filtered := model.filteredCommandDialogItems()
	if len(filtered) != 1 || filtered[0].ID != "search" {
		t.Fatalf("filtered = %#v", filtered)
	}
	updated, _ = model.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(*Model)
	if model.commandDialog != nil {
		t.Fatal("command dialog should close after search action")
	}
	if model.input != "/search " || model.cursorPos != len("/search ") {
		t.Fatalf("input = %q cursor=%d", model.input, model.cursorPos)
	}
}

func TestToolsetsCommand(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	cfg.Skills.Enabled = true
	cfg.MCP.Servers = []config.MCPServerConfig{{
		Name:                 "filesystem",
		Enabled:              true,
		Command:              "mcp-filesystem",
		Tools:                []string{"read_file"},
		RequiresConfirmation: true,
	}}
	m := New(nil, cfg, runner.PluginConfig{}, svc)

	if handled, _ := m.processCommand("/toolsets"); !handled {
		t.Fatal("/toolsets should be handled")
	}
	if len(m.messages) == 0 {
		t.Fatal("expected toolsets output message")
	}
	content := m.messages[len(m.messages)-1].Content
	for _, want := range []string{"ADK toolsets", "skills:skills", "mcp:filesystem", "read_file", "confirm"} {
		if !strings.Contains(content, want) {
			t.Fatalf("toolsets output missing %q:\n%s", want, content)
		}
	}
}

func TestFormatToolsetsEmpty(t *testing.T) {
	if got := formatToolsets(nil, 20); !strings.Contains(got, "No ADK Skill or MCP toolsets") {
		t.Fatalf("formatToolsets(nil) = %q", got)
	}
}

func TestFormatToolsetsLimit(t *testing.T) {
	got := formatToolsets([]supermanagent.ToolsetDescriptor{
		{Name: "skills:one", Kind: "skill", Source: "/tmp/one"},
		{Name: "mcp:two", Kind: "mcp", Source: "server", Tools: []string{"read"}, RequiresConfirmation: true},
	}, 1)
	if !strings.Contains(got, "skills:one") || strings.Contains(got, "mcp:two") || !strings.Contains(got, "1 more") {
		t.Fatalf("limited toolsets output = %q", got)
	}
}

func TestOverlayBlock(t *testing.T) {
	got := overlayBlock("a\nb\nc", "\nX\n")
	if got != "a\nX\nc" {
		t.Fatalf("overlay = %q", got)
	}
}

func TestFileHistoryCommands(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	path := cfg.Workspace + "/main.go"
	if _, err := svc.RecordFileRevision("app", "tui-user", "1", path, "patch", "old", "new", false); err != nil {
		t.Fatal(err)
	}
	m := New(nil, cfg, runner.PluginConfig{}, svc)

	if handled, _ := m.processCommand("/history"); !handled {
		t.Fatal("/history should be handled")
	}
	if len(m.messages) == 0 || !strings.Contains(m.messages[len(m.messages)-1].Content, "patch") || !strings.Contains(m.messages[len(m.messages)-1].Content, "main.go") {
		t.Fatalf("history messages = %+v", m.messages)
	}

	if handled, _ := m.processCommand("/diff " + path); !handled {
		t.Fatal("/diff should be handled")
	}
	content := m.messages[len(m.messages)-1].Content
	if !strings.Contains(content, "Unified diff") || !strings.Contains(content, "--- a/") || !strings.Contains(content, "-old") || !strings.Contains(content, "+new") {
		t.Fatalf("diff output = %q", content)
	}
}

func TestSearchCommand(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	created, err := svc.Get(t.Context(), &adksession.GetRequest{AppName: "app", UserID: "tui-user", SessionID: "1"})
	if err != nil {
		t.Fatal(err)
	}
	ev := adksession.NewEvent("inv-search")
	ev.Author = "user"
	ev.Content = genai.NewContentFromText("Recall cache invalidation policy", genai.RoleUser)
	if err := svc.AppendEvent(t.Context(), created.Session, ev); err != nil {
		t.Fatal(err)
	}
	m := New(nil, cfg, runner.PluginConfig{}, svc)

	if handled, _ := m.processCommand("/search cache"); !handled {
		t.Fatal("/search should be handled")
	}
	if m.input != "" {
		t.Fatalf("input = %q, want empty", m.input)
	}
	if len(m.messages) == 0 || !strings.Contains(m.messages[len(m.messages)-1].Content, "Session search") || !strings.Contains(m.messages[len(m.messages)-1].Content, "Recall cache invalidation policy") {
		t.Fatalf("messages = %+v", m.messages)
	}
}

func TestSearchDialogInsertReference(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	m := New(nil, cfg, runner.PluginConfig{}, svc)
	m.searchDialog = &searchDialogState{
		Query: "cache",
		Results: []supermansession.MessageSearchResult{
			{
				Metadata: supermansession.Metadata{SessionID: "1", Title: "Cache Work"},
				Message:  supermansession.Message{Role: supermansession.MessageUser, Content: "Recall cache invalidation policy"},
				Preview:  "Recall cache invalidation policy",
			},
		},
	}

	updated, _ := m.handleKey(tea.KeyPressMsg{Text: "i", Code: 'i'})
	model := updated.(*Model)

	if model.searchDialog != nil {
		t.Fatal("search dialog should close after insert")
	}
	input := model.inputValue()
	for _, want := range []string{"[session:1 role:user]", "Recall cache invalidation policy"} {
		if !strings.Contains(input, want) {
			t.Fatalf("input missing %q: %q", want, input)
		}
	}
}

func TestRevertCommandRestoresLatestBeforeSnapshot(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	path := filepath.Join(cfg.Workspace, "main.go")
	if err := os.WriteFile(path, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecordFileRevision("app", "tui-user", "1", path, "patch", "old", "new", false); err != nil {
		t.Fatal(err)
	}
	m := New(nil, cfg, runner.PluginConfig{}, svc)

	if handled, _ := m.processCommand("/revert " + path); !handled {
		t.Fatal("/revert should be handled")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "old" {
		t.Fatalf("file content = %q, want old", data)
	}
	revisions, err := svc.FileRevisions("app", "tui-user", "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 2 || revisions[1].Action != "revert" || revisions[1].Before.Preview != "new" || revisions[1].After.Preview != "old" {
		t.Fatalf("revisions = %#v", revisions)
	}
}

func TestRevertCommandRestoresMissingFile(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	path := filepath.Join(cfg.Workspace, "created.go")
	if err := os.WriteFile(path, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecordFileRevision("app", "tui-user", "1", path, "write", "", "new", true); err != nil {
		t.Fatal(err)
	}
	m := New(nil, cfg, runner.PluginConfig{}, svc)

	if handled, _ := m.processCommand("/revert " + path); !handled {
		t.Fatal("/revert should be handled")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file should be removed, stat err = %v", err)
	}
	revisions, err := svc.FileRevisions("app", "tui-user", "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 2 || revisions[1].Action != "revert" || !revisions[1].After.Missing {
		t.Fatalf("revisions = %#v", revisions)
	}
}

func TestRevertCommandRestoresTruncatedSnapshotFromStore(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	path := filepath.Join(cfg.Workspace, "large.txt")
	large := strings.Repeat("x", 4100)
	if err := os.WriteFile(path, []byte("current"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecordFileRevision("app", "tui-user", "1", path, "patch", large, "current", false); err != nil {
		t.Fatal(err)
	}
	m := New(nil, cfg, runner.PluginConfig{}, svc)

	if handled, _ := m.processCommand("/revert " + path); !handled {
		t.Fatal("/revert should be handled")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != large {
		t.Fatalf("file content length = %d, want %d", len(data), len(large))
	}
}

func TestSessionCommands(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	if _, err := svc.Create(t.Context(), &adksession.CreateRequest{AppName: "app", UserID: "tui-user", SessionID: "2"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Rename("app", "tui-user", "2", "Work Session"); err != nil {
		t.Fatal(err)
	}
	created, err := svc.Get(t.Context(), &adksession.GetRequest{AppName: "app", UserID: "tui-user", SessionID: "2"})
	if err != nil {
		t.Fatal(err)
	}
	event := adksession.NewEvent("inv")
	event.Author = "user"
	event.Content = genai.NewContentFromText("hello from work", genai.RoleUser)
	if err := svc.AppendEvent(t.Context(), created.Session, event); err != nil {
		t.Fatal(err)
	}

	m := New(nil, cfg, runner.PluginConfig{}, svc)
	if handled, _ := m.processCommand("/sessions"); !handled {
		t.Fatal("/sessions should be handled")
	}
	if !strings.Contains(m.messages[len(m.messages)-1].Content, "Work Session") {
		t.Fatalf("sessions output = %q", m.messages[len(m.messages)-1].Content)
	}

	if handled, _ := m.processCommand("/switch 2"); !handled {
		t.Fatal("/switch should be handled")
	}
	if m.sessionID != "2" {
		t.Fatalf("sessionID = %q, want work", m.sessionID)
	}
	if len(m.messages) < 2 || !strings.Contains(m.messages[0].Content, "hello from work") {
		t.Fatalf("messages = %+v", m.messages)
	}

	if handled, _ := m.processCommand("/rename Focus"); !handled {
		t.Fatal("/rename should be handled")
	}
	meta, err := svc.Metadata("app", "tui-user", "2")
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "Focus" {
		t.Fatalf("title = %q", meta.Title)
	}

	if handled, _ := m.processCommand("/new"); !handled {
		t.Fatal("/new should be handled")
	}
	if m.sessionID == "2" || len(m.messages) != 1 || !strings.Contains(m.messages[0].Content, "New session:") {
		t.Fatalf("new session state id=%q messages=%+v", m.sessionID, m.messages)
	}
}

func TestRunningEnterQueuesPrompt(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	m := &Model{
		running:        true,
		cfg:            cfg,
		sessionService: svc,
		sessionID:      "1",
		input:          "next task",
		cursorPos:      len("next task"),
		toolStarts:     make(map[string]time.Time),
		chatCacheDirty: true,
	}
	m.textarea = newTextarea()
	m.setInput("next task")

	updated, cmd := m.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	model := updated.(*Model)

	if cmd != nil {
		t.Fatal("queueing should not start a command immediately")
	}
	queue, err := svc.PromptQueue("app", "tui-user", "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(queue) != 1 || queue[0].Content != "next task" {
		t.Fatalf("queue = %#v", queue)
	}
	if model.input != "" {
		t.Fatalf("input = %q, want empty", model.input)
	}
	if model.inputValue() != "" || model.textarea.Value() != "" {
		t.Fatalf("textarea input not cleared: inputValue=%q textarea=%q", model.inputValue(), model.textarea.Value())
	}
	if len(model.messages) != 1 || model.messages[0].Role != "system" {
		t.Fatalf("messages = %+v", model.messages)
	}
}

func TestQueueCommands(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	if _, err := svc.EnqueuePrompt("app", "tui-user", "1", "one"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.EnqueuePrompt("app", "tui-user", "1", "two"); err != nil {
		t.Fatal(err)
	}
	m := New(nil, cfg, runner.PluginConfig{}, svc)

	if handled, _ := m.processCommand("/queue"); !handled {
		t.Fatal("/queue should be handled")
	}
	if len(m.messages) != 1 || !strings.Contains(m.messages[0].Content, "Prompt queue (2)") {
		t.Fatalf("messages = %+v", m.messages)
	}
	if handled, _ := m.processCommand("/clearqueue"); !handled {
		t.Fatal("/clearqueue should be handled")
	}
	queue, err := svc.PromptQueue("app", "tui-user", "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(queue) != 0 || m.queueCount != 0 {
		t.Fatalf("queue = %#v count=%d", queue, m.queueCount)
	}
}

func TestPrepareNextQueuedPrompt(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	refPath := filepath.Join(cfg.Workspace, "queued.go")
	if _, err := svc.EnqueuePrompt("app", "tui-user", "1", "queued @queued.go"); err != nil {
		t.Fatal(err)
	}
	m := &Model{
		cfg:            cfg,
		sessionService: svc,
		sessionID:      "1",
		runner:         &runner.Runner{},
		toolStarts:     make(map[string]time.Time),
	}

	prompt, ok := m.prepareNextQueuedPrompt()

	if !ok || prompt != "queued @queued.go" {
		t.Fatalf("prompt = %q ok = %v", prompt, ok)
	}
	if !m.running {
		t.Fatal("model should be running queued prompt")
	}
	queue, err := svc.PromptQueue("app", "tui-user", "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(queue) != 0 || m.queueCount != 0 {
		t.Fatalf("queue = %#v count=%d", queue, m.queueCount)
	}
	if len(m.messages) != 1 || m.messages[0].Role != "user" || m.messages[0].Content != "queued @queued.go" {
		t.Fatalf("messages = %+v", m.messages)
	}
	files, err := svc.SessionFiles("app", "tui-user", "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != refPath || files[0].ReadCount != 1 {
		t.Fatalf("files = %#v want %s read once", files, refPath)
	}
}

func TestExtractFileReferences(t *testing.T) {
	got := supermansession.ExtractFileReferences(`read @main.go and @"docs/design doc.md", ignore https://example.com/@x and duplicate @main.go.`)
	want := []string{"main.go", "docs/design doc.md"}
	if len(got) != len(want) {
		t.Fatalf("refs = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("refs = %#v, want %#v", got, want)
		}
	}
}

func TestRecordPromptFileReferences(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	mainPath := filepath.Join(cfg.Workspace, "main.go")
	designPath := filepath.Join(cfg.Workspace, "docs/design doc.md")
	m := New(nil, cfg, runner.PluginConfig{}, svc)

	count := m.recordPromptFileReferences(`review @main.go and @"docs/design doc.md" then @main.go again`)

	if count != 2 {
		t.Fatalf("record count = %d, want 2", count)
	}
	files, err := svc.SessionFiles("app", "tui-user", "1")
	if err != nil {
		t.Fatal(err)
	}
	byPath := make(map[string]supermansession.SessionFile)
	for _, file := range files {
		byPath[file.Path] = file
	}
	if byPath[mainPath].ReadCount != 1 {
		t.Fatalf("main file = %#v", byPath[mainPath])
	}
	if byPath[designPath].ReadCount != 1 {
		t.Fatalf("design file = %#v", byPath[designPath])
	}
}

func TestRecordPromptSessionReferences(t *testing.T) {
	svc, cfg := newTestQueuedSession(t)
	m := New(nil, cfg, runner.PluginConfig{}, svc)

	count := m.recordPromptSessionReferences(`continue from [session:past role:user] historical cache decision`)

	if count != 1 {
		t.Fatalf("record count = %d, want 1", count)
	}
	refs, err := svc.SessionReferences("app", "tui-user", "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].SessionID != "past" || refs[0].Role != supermansession.MessageUser || refs[0].Preview != "historical cache decision" {
		t.Fatalf("refs = %#v", refs)
	}
}

func newTestQueuedSession(t *testing.T) (*supermansession.Service, *config.Config) {
	t.Helper()
	cfg := &config.Config{
		Workspace: t.TempDir(),
		Model:     config.ModelConfig{Provider: "test", Name: "model"},
		Session:   config.SessionConfig{AppName: "app"},
	}
	global.SetConfig(cfg)
	t.Cleanup(func() { global.SetConfig(nil) })
	svc, err := supermansession.NewService()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(t.Context(), &adksession.CreateRequest{AppName: "app", UserID: "tui-user", SessionID: "1"}); err != nil {
		t.Fatal(err)
	}
	return svc, cfg
}
