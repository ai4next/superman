package app

import (
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/ai4next/superman/internal/tui/components"
)

func resizeRenderTick(seq int) tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(time.Time) tea.Msg {
		return resizeRenderMsg{Seq: seq}
	})
}
func (m *Model) scrollPageStep() int {
	return max(8, m.height-4)
}
func (m *Model) scrollWheelStep() int {
	return 3
}
func overlayBlock(base, overlay string) string {
	if strings.TrimSpace(overlay) == "" {
		return base
	}
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")
	if len(baseLines) < len(overlayLines) {
		for len(baseLines) < len(overlayLines) {
			baseLines = append(baseLines, "")
		}
	}
	for i, line := range overlayLines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		baseLines[i] = line
	}
	return strings.Join(baseLines, "\n")
}
func (m *Model) View() tea.View {
	if m.width == 0 {
		view := tea.NewView("Loading...")
		view.AltScreen = true
		return view
	}

	var chatContent string
	var chatLines []string
	cwd, _ := os.Getwd()
	mainWidth := m.width
	m.textarea.SetWidth(max(4, mainWidth-2))
	m.syncInputFromTextarea()
	textareaView := m.textarea.View()
	cursor := m.textarea.Cursor()
	cursorX, cursorY := 0, 0
	if cursor != nil {
		cursorX = cursor.X
		cursorY = cursor.Y
	}
	composer := components.RenderTextareaComposer(textareaView, cursorX, cursorY, m.textarea.Height(), mainWidth, m.running)
	commandPanel := ""
	if m.commandDialog != nil {
		commandPanel = components.RenderCommandPanel(components.CommandDialogData{
			Commands: m.filteredCommandDialogItems(),
			Selected: m.commandDialog.Selected,
			Query:    m.commandDialog.Query,
		}, mainWidth, min(8, max(3, m.height/3)))
	}
	commandPanelHeight := 0
	if commandPanel != "" {
		commandPanelHeight = len(strings.Split(commandPanel, "\n"))
	}

	if m.showWelcome && len(m.messages) == 0 {
		chatContent = components.RenderWelcome(m.modelName, cwd, m.sessionID)
		chatLines = strings.Split(chatContent, "\n")
	} else {
		chatContent, chatLines = m.renderChat()
	}

	inputHeight := max(1, composer.Height)
	availHeight := max(0, m.height-inputHeight-commandPanelHeight)
	if len(chatLines) > availHeight {
		maxOffset := len(chatLines) - availHeight
		if m.scrollOffset > maxOffset {
			m.scrollOffset = maxOffset
		}
		startLine := max(0, len(chatLines)-availHeight-m.scrollOffset)
		chatContent = strings.Join(chatLines[startLine:startLine+availHeight], "\n")
	} else {
		m.scrollOffset = 0
	}
	chatContent = m.applySelection(chatContent)

	mainOutput := chatContent + "\n" + composer.Content
	if commandPanel != "" {
		mainOutput += "\n" + commandPanel
	}
	fullOutput := mainOutput
	if m.sessionDialog != nil {
		dialog := components.RenderSessionDialog(components.SessionDialogData{
			Sessions: m.sessionDialog.Sessions,
			Selected: m.sessionDialog.Selected,
			Current:  m.sessionID,
		}, m.width, m.height)
		fullOutput = overlayBlock(fullOutput, dialog)
	}
	if m.searchDialog != nil {
		dialog := components.RenderSearchResults(components.SearchResultsData{
			Results:  m.searchDialog.Results,
			Selected: m.searchDialog.Selected,
			Query:    m.searchDialog.Query,
		}, m.width, m.height)
		fullOutput = overlayBlock(fullOutput, dialog)
	}
	m.cursorCol = composer.CursorX
	m.cursorRow = len(strings.Split(chatContent, "\n")) + composer.CursorY + 1
	view := tea.NewView(fullOutput)
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	if cursor != nil {
		cursor.Position.X = max(0, m.cursorCol)
		cursor.Position.Y = max(0, m.cursorRow-1)
		view.Cursor = cursor
	} else {
		view.Cursor = tea.NewCursor(0, 0)
		view.Cursor.Shape = tea.CursorBar
	}
	return view
}
func (m *Model) renderChat() (string, []string) {
	if !m.chatCacheDirty && m.chatCacheWidth == m.width && m.chatCachePulse == m.pulseOn {
		return m.chatCache, m.chatLinesCache
	}
	m.chatCache = components.RenderChatWithPulse(m.messages, m.width, m.pulseOn)
	m.chatLinesCache = strings.Split(m.chatCache, "\n")
	m.chatCacheWidth = m.width
	m.chatCachePulse = m.pulseOn
	m.chatCacheDirty = false
	return m.chatCache, m.chatLinesCache
}
