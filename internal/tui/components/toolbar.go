package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/ai4next/superman/internal/tui/styles"
)

type ToolbarData struct {
	ModelName    string
	CWD          string
	GitBranch    string
	ContextPct   float64
	ScrollOffset int // 0 = at bottom (newest), >0 = scrolled up
}

func RenderToolbar(data ToolbarData, width int) string {
	left := fmt.Sprintf("agent (%s)  %s", data.ModelName, data.CWD)
	if data.GitBranch != "" {
		left += "  " + data.GitBranch
	}

	right := ""
	if data.ScrollOffset > 0 {
		right = fmt.Sprintf("↑ %d", data.ScrollOffset)
	}
	if data.ContextPct > 0 {
		if right != "" {
			right += "  "
		}
		right += fmt.Sprintf("ctx: %.1f%%", data.ContextPct)
	}

	spacer := max(0, width-lipgloss.Width(left)-lipgloss.Width(right))
	return styles.ToolbarStyle.Render(left + strings.Repeat(" ", spacer) + right)
}
