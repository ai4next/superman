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
	remaining := width - lipgloss.Width(prompt) - 2
	runes := []rune(text)
	if cursorPos > len(runes) {
		cursorPos = len(runes)
	}
	if len(runes) > remaining && remaining > 0 {
		start := cursorPos - remaining
		if start < 0 {
			start = 0
		}
		if start+remaining > len(runes) {
			start = len(runes) - remaining
		}
		if start < 0 {
			start = 0
		}
		runes = runes[start:]
		cursorPos -= start
	}
	before := string(runes[:cursorPos])
	after := string(runes[cursorPos:])
	cursor := styles.CursorStyle.Render("█")
	cursorCol := lipgloss.Width(prompt) + lipgloss.Width(before) + 1
	return prompt + before + cursor + after, cursorCol
}
