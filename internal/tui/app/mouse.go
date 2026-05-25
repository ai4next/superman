package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/ai4next/superman/internal/tui/components"
)

const mouseSelectScrollThreshold = 1

func (m *Model) handleMouseWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	m.clearSelection()
	mouse := msg.Mouse()
	if !m.mouseInChat(mouse) {
		return m, nil
	}
	switch mouse.Button {
	case tea.MouseWheelUp:
		m.scrollOffset += m.scrollWheelStep()
	case tea.MouseWheelDown:
		m.scrollOffset = max(0, m.scrollOffset-m.scrollWheelStep())
	}
	return m, nil
}

func (m *Model) handleMouseClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	mouse := msg.Mouse()
	if mouse.Button != tea.MouseLeft || !m.mouseInChat(mouse) {
		m.clearSelection()
		return m, nil
	}
	m.selection = selectionState{
		Active: true,
		StartX: mouse.X,
		StartY: mouse.Y,
		EndX:   mouse.X,
		EndY:   mouse.Y,
	}
	return m, nil
}

func (m *Model) handleMouseMotion(msg tea.MouseMotionMsg) (tea.Model, tea.Cmd) {
	if !m.selection.Active {
		return m, nil
	}
	mouse := msg.Mouse()
	if !m.mouseInMain(mouse) {
		return m, nil
	}
	if mouse.Y <= mouseSelectScrollThreshold && m.scrollOffset < m.maxScrollOffset() {
		m.scrollOffset += 1
	}
	if mouse.Y >= m.chatViewportHeight()-1 {
		m.scrollOffset = max(0, m.scrollOffset-1)
	}
	m.selection.EndX = mouse.X
	m.selection.EndY = min(max(mouse.Y, 0), max(0, m.chatViewportHeight()-1))
	return m, nil
}

func (m *Model) handleMouseRelease(msg tea.MouseReleaseMsg) (tea.Model, tea.Cmd) {
	if !m.selection.Active {
		return m, nil
	}
	mouse := msg.Mouse()
	if m.mouseInMain(mouse) {
		m.selection.EndX = mouse.X
		m.selection.EndY = min(max(mouse.Y, 0), max(0, m.chatViewportHeight()-1))
	}
	if m.selection.StartX == m.selection.EndX && m.selection.StartY == m.selection.EndY {
		m.clearSelection()
		return m, nil
	}
	selected := m.selectedChatText()
	if strings.TrimSpace(selected) == "" {
		return m, nil
	}
	return m, tea.SetClipboard(selected)
}

func (m *Model) mouseInChat(mouse tea.Mouse) bool {
	if m.width <= 0 || m.height <= 0 {
		return false
	}
	if mouse.X < 0 || mouse.Y < 0 {
		return false
	}
	if !m.mouseInMain(mouse) {
		return false
	}
	chatHeight := m.chatViewportHeight()
	return mouse.Y < chatHeight
}

func (m *Model) mouseInMain(mouse tea.Mouse) bool {
	if mouse.X < 0 || mouse.Y < 0 {
		return false
	}
	return mouse.X < m.width
}

func (m *Model) chatViewportHeight() int {
	mainWidth := m.width
	m.textarea.SetWidth(max(4, mainWidth-2))
	inputHeight := max(1, m.textarea.Height()+2)
	return max(0, m.height-inputHeight)
}

func (m *Model) maxScrollOffset() int {
	if m.showWelcome && len(m.messages) == 0 {
		return 0
	}
	_, chatLines := m.renderChat()
	return max(0, len(chatLines)-m.chatViewportHeight())
}

func (m *Model) clearSelection() {
	m.selection = selectionState{}
}

func (m *Model) selectedChatText() string {
	if !m.selection.Active {
		return ""
	}
	content := m.visibleChatContent()
	lines := strings.Split(content, "\n")
	startY, endY, startX, endX := normalizedSelection(m.selection)
	var out []string
	for y := startY; y <= endY && y < len(lines); y++ {
		if y < 0 {
			continue
		}
		lineStart, lineEnd := 0, ansi.StringWidth(ansi.Strip(lines[y]))
		if y == startY {
			lineStart = startX
		}
		if y == endY {
			lineEnd = endX
		}
		out = append(out, cutPlainLine(ansi.Strip(lines[y]), lineStart, lineEnd))
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func (m *Model) applySelection(content string) string {
	if !m.selection.Active {
		return content
	}
	lines := strings.Split(content, "\n")
	startY, endY, startX, endX := normalizedSelection(m.selection)
	for y := startY; y <= endY && y < len(lines); y++ {
		if y < 0 {
			continue
		}
		lineStart, lineEnd := 0, ansi.StringWidth(ansi.Strip(lines[y]))
		if y == startY {
			lineStart = startX
		}
		if y == endY {
			lineEnd = endX
		}
		if lineStart > lineEnd {
			lineStart, lineEnd = lineEnd, lineStart
		}
		lines[y] = highlightLineRange(lines[y], lineStart, lineEnd)
	}
	return strings.Join(lines, "\n")
}

func normalizedSelection(selection selectionState) (startY, endY, startX, endX int) {
	startY, endY = selection.StartY, selection.EndY
	startX, endX = selection.StartX, selection.EndX
	if startY > endY || (startY == endY && startX > endX) {
		startY, endY = endY, startY
		startX, endX = endX, startX
	}
	return startY, endY, startX, endX
}

func (m *Model) visibleChatContent() string {
	var chatContent string
	var chatLines []string
	if m.showWelcome && len(m.messages) == 0 {
		cwd := ""
		chatContent = components.RenderWelcome(m.modelName, cwd, m.sessionID)
		chatLines = strings.Split(chatContent, "\n")
	} else {
		chatContent, chatLines = m.renderChat()
	}
	availHeight := m.chatViewportHeight()
	if len(chatLines) > availHeight {
		maxOffset := len(chatLines) - availHeight
		if m.scrollOffset > maxOffset {
			m.scrollOffset = maxOffset
		}
		startLine := max(0, len(chatLines)-availHeight-m.scrollOffset)
		return strings.Join(chatLines[startLine:startLine+availHeight], "\n")
	}
	return chatContent
}

func highlightLineRange(line string, startCol, endCol int) string {
	if startCol == endCol {
		return line
	}
	plain := ansi.Strip(line)
	if plain == "" {
		return line
	}
	startCol = max(0, min(startCol, ansi.StringWidth(plain)))
	endCol = max(0, min(endCol, ansi.StringWidth(plain)))
	if startCol >= endCol {
		return line
	}

	var before, selected, after strings.Builder
	col := 0
	for _, r := range plain {
		cell := string(r)
		width := ansi.StringWidth(cell)
		if width <= 0 {
			width = 1
		}
		switch {
		case col+width <= startCol:
			before.WriteRune(r)
		case col >= endCol:
			after.WriteRune(r)
		default:
			selected.WriteRune(r)
		}
		col += width
	}
	return before.String() + "\x1b[7m" + selected.String() + "\x1b[0m" + after.String()
}

func cutPlainLine(line string, startCol, endCol int) string {
	if startCol > endCol {
		startCol, endCol = endCol, startCol
	}
	startCol = max(0, min(startCol, ansi.StringWidth(line)))
	endCol = max(0, min(endCol, ansi.StringWidth(line)))
	if startCol >= endCol {
		return ""
	}
	var out strings.Builder
	col := 0
	for _, r := range line {
		cell := string(r)
		width := ansi.StringWidth(cell)
		if width <= 0 {
			width = 1
		}
		if col+width > startCol && col < endCol {
			out.WriteRune(r)
		}
		col += width
	}
	return out.String()
}
