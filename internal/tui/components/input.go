package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/ai4next/superman/internal/tui/styles"
)

func RenderInputSeparator(width int) string {
	sep := strings.Repeat("─", max(0, width-12))
	return styles.InputSeparator.Render("── input " + sep)
}

func RenderInputLine(text string, cursorPos int, width int) (string, int) {
	prompt := styles.InputPrompt.Render("> ")
	remaining := max(0, width-lipgloss.Width(prompt)-1)
	runes := []rune(text)
	if cursorPos > len(runes) {
		cursorPos = len(runes)
	}

	start := 0
	for lipgloss.Width(string(runes[start:cursorPos])) > remaining && start < cursorPos {
		start++
	}
	end := cursorPos
	for end < len(runes) && lipgloss.Width(string(runes[start:end+1])) <= remaining {
		end++
	}
	runes = runes[start:end]
	cursorPos -= start

	before := string(runes[:cursorPos])
	after := string(runes[cursorPos:])
	cursor := styles.CursorStyle.Render("█")
	cursorCol := lipgloss.Width(prompt) + lipgloss.Width(before) + 1
	return prompt + before + cursor + after, cursorCol
}
