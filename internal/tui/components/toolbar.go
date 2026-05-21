package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/ai4next/superman/internal/tui/styles"
)

type ToolbarData struct {
	ModelName  string
	CWD        string
	GitBranch  string
	ContextPct float64
}

func RenderToolbar(data ToolbarData, width int) string {
	left := fmt.Sprintf("agent (%s)  %s", data.ModelName, data.CWD)
	if data.GitBranch != "" {
		left += "  " + data.GitBranch
	}

	right := ""
	if data.ContextPct > 0 {
		right = fmt.Sprintf("context: %.1f%%", data.ContextPct)
	}

	spacer := max(0, width-lipgloss.Width(left)-lipgloss.Width(right))
	return styles.ToolbarStyle.Render(left + strings.Repeat(" ", spacer) + right)
}