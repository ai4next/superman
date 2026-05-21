package components

import (
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/ai4next/superman/internal/tui/styles"
)

type Message struct {
	Role    string // "user", "agent", or "tool"
	Content string
	Tool    string // tool name for tool messages
}

func RenderChat(messages []Message, width int) string {
	var sb strings.Builder
	mdRenderer, _ := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width-4),
	)

	for _, msg := range messages {
		switch msg.Role {
		case "user":
			sb.WriteString(styles.UserPrefix.Render(" > "))
			sb.WriteString(msg.Content)
			sb.WriteString("\n\n")
		case "agent":
			sb.WriteString(styles.AgentPrefix.Render(" > "))
			rendered, _ := mdRenderer.Render(msg.Content)
			sb.WriteString(strings.TrimSpace(rendered))
			sb.WriteString("\n\n")
		case "tool":
			sb.WriteString(styles.ToolExecuting.Render("  - "))
			sb.WriteString(msg.Tool)
			if msg.Content != "" {
				sb.WriteString(" ")
				sb.WriteString(styles.ToolOutput.Render(msg.Content))
			}
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func RenderWelcome(modelName, cwd, sessionID string) string {
	logo := styles.WelcomeTitle.Render(" ▟█▀▀▀█▙\n ▐█████▌")
	title := styles.WelcomeTitle.Render("Welcome to Superman Agent!")
	info := styles.WelcomeText.Render(
		"\n\nDirectory    " + cwd +
			"\nSession ID   " + sessionID +
			"\nModel        " + modelName,
	)
	content := lipgloss.JoinVertical(lipgloss.Left, logo, "", title, info)
	return styles.WelcomeBorder.Render(content)
}