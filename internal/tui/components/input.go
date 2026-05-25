package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/ai4next/superman/internal/tui/styles"
)

type ComposerView struct {
	Content   string
	CursorX   int
	CursorY   int
	Height    int
	LineCount int
}

func RenderTextareaComposer(textareaView string, cursorX, cursorY, textareaHeight, width int, running bool) ComposerView {
	if width <= 0 {
		return ComposerView{Content: "", Height: 0}
	}
	innerWidth := max(8, width-2)
	title := " message "
	if running {
		title = " waiting "
	}
	top := styles.InputBorder.Render("╭" + title + strings.Repeat("─", max(0, innerWidth-lipgloss.Width(title))) + "╮")
	bottomHint := " Enter send  Ctrl+J newline  Up/Down history  /cancel "
	if !running {
		bottomHint = " Enter send  Ctrl+J newline  Up/Down history  Ctrl+C quit "
	}
	bottomFill := max(0, innerWidth-lipgloss.Width(bottomHint))
	bottom := styles.InputBorder.Render("╰" + strings.Repeat("─", bottomFill) + bottomHint + "╯")

	lines := strings.Split(textareaView, "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = []string{""}
	}
	for i, line := range lines {
		lines[i] = styles.InputLine.Render("│" + padRight(line, innerWidth) + "│")
	}
	content := top + "\n" + strings.Join(lines, "\n") + "\n" + bottom
	return ComposerView{
		Content:   content,
		CursorX:   cursorX + 1,
		CursorY:   cursorY + 1,
		Height:    max(textareaHeight, len(lines)) + 2,
		LineCount: len(lines),
	}
}

func RenderComposer(text string, cursorPos int, width int, running bool) ComposerView {
	if width <= 0 {
		return ComposerView{Content: "", Height: 0}
	}

	innerWidth := max(8, width-2)
	bodyWidth := max(4, innerWidth-2)
	runes := []rune(text)
	if cursorPos > len(runes) {
		cursorPos = len(runes)
	}

	lines, cursorLine, cursorCol := wrapInput(runes, cursorPos, bodyWidth)
	if len(lines) == 0 {
		lines = []string{""}
	}

	maxVisible := 6
	startLine := 0
	if cursorLine >= maxVisible {
		startLine = cursorLine - maxVisible + 1
	}
	endLine := min(len(lines), startLine+maxVisible)
	visible := lines[startLine:endLine]

	title := " message "
	if running {
		title = " waiting "
	}
	top := styles.InputBorder.Render("╭" + title + strings.Repeat("─", max(0, innerWidth-lipgloss.Width(title))) + "╮")
	bottomHint := " Enter send  Ctrl+J newline  Ctrl+C quit "
	bottomFill := max(0, innerWidth-lipgloss.Width(bottomHint))
	bottom := styles.InputBorder.Render("╰" + strings.Repeat("─", bottomFill) + bottomHint + "╯")

	var sb strings.Builder
	sb.WriteString(top)
	sb.WriteString("\n")
	for i, line := range visible {
		globalLine := startLine + i
		prefix := "  "
		if globalLine == cursorLine {
			prefix = styles.InputPrompt.Render("> ")
		}
		content := line
		if globalLine == cursorLine {
			colRunes := []rune(line)
			if cursorCol > len(colRunes) {
				cursorCol = len(colRunes)
			}
			content = string(colRunes[:cursorCol]) + styles.CursorStyle.Render("|") + string(colRunes[cursorCol:])
		}
		padded := padRight(prefix+content, innerWidth)
		sb.WriteString(styles.InputLine.Render("│" + padded + "│"))
		if i < len(visible)-1 {
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n")
	sb.WriteString(bottom)

	cursorY := 1 + cursorLine - startLine
	cursorX := 1 + lipgloss.Width("> ") + lipgloss.Width(string([]rune(lines[cursorLine])[:cursorCol]))
	return ComposerView{
		Content:   sb.String(),
		CursorX:   cursorX,
		CursorY:   cursorY,
		Height:    len(visible) + 2,
		LineCount: len(lines),
	}
}

func wrapInput(runes []rune, cursorPos int, width int) ([]string, int, int) {
	var lines []string
	var line []rune
	lineWidth := 0
	cursorLine := 0
	cursorCol := 0

	commitCursor := func() {
		cursorLine = len(lines)
		cursorCol = len(line)
	}

	for i, r := range runes {
		if i == cursorPos {
			commitCursor()
		}
		if r == '\n' {
			lines = append(lines, string(line))
			line = line[:0]
			lineWidth = 0
			continue
		}
		rw := lipgloss.Width(string(r))
		if lineWidth > 0 && lineWidth+rw > width {
			lines = append(lines, string(line))
			line = line[:0]
			lineWidth = 0
		}
		line = append(line, r)
		lineWidth += rw
	}
	if cursorPos == len(runes) {
		commitCursor()
	}
	lines = append(lines, string(line))
	return lines, cursorLine, cursorCol
}

func padRight(s string, width int) string {
	return s + strings.Repeat(" ", max(0, width-lipgloss.Width(s)))
}
