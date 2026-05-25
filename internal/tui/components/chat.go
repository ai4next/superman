package components

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/lipgloss"

	"github.com/ai4next/superman/internal/tui/styles"
)

var markdownRenderers sync.Map

type Message struct {
	Role     string // "user", "agent", "tool", "system", or "error"
	Content  string
	Tool     string // tool name for tool messages
	ToolID   string
	Args     string // compact tool args for tool messages
	Result   string
	Status   string // "running", "done", or "error"
	Duration string
}

func RenderChat(messages []Message, width int) string {
	return RenderChatWithPulse(messages, width, false)
}

func RenderChatWithPulse(messages []Message, width int, pulseOn bool) string {
	var sb strings.Builder
	mdRenderer := markdownRenderer(width)
	contentWidth := max(24, width-4)

	for _, msg := range messages {
		switch msg.Role {
		case "user":
			sb.WriteString("\n")
			sb.WriteString(styles.UserBubble.Width(contentWidth).Render(msg.Content))
			sb.WriteString("\n\n")
		case "agent":
			rendered, _ := mdRenderer.Render(msg.Content)
			sb.WriteString("\n")
			sb.WriteString(styles.AgentBubble.Width(contentWidth).Render(strings.TrimSpace(rendered)))
			sb.WriteString("\n\n")
		case "error":
			sb.WriteString(styles.MessageRole.Render("Error"))
			sb.WriteString("\n")
			sb.WriteString(styles.ErrorBubble.Width(contentWidth).Render(msg.Content))
			sb.WriteString("\n\n")
		case "system":
			sb.WriteString(styles.MessageRole.Render("System"))
			sb.WriteString("\n")
			sb.WriteString(styles.ToolOutput.Render(indentWrapped(msg.Content, contentWidth)))
			sb.WriteString("\n\n")
		case "tool":
			header := toolStatusStyle(msg.Status, pulseOn).Render(toolStatusIcon(msg.Status) + " ")
			header += styles.ToolName.Render(msg.Tool)
			if msg.Args != "" {
				header += " " + styles.ToolOutput.Render(msg.Args)
			}
			meta := toolMeta(msg)
			if meta != "" {
				header += " " + styles.ToolOutput.Render(meta)
			}
			sb.WriteString(header)
			sb.WriteString("\n")
			if msg.Result != "" {
				sb.WriteString(styles.ToolOutput.Render("  " + indentWrapped(msg.Result, contentWidth-2)))
				sb.WriteString("\n")
			} else if msg.Content != "" {
				sb.WriteString(styles.ToolOutput.Render("  " + indentWrapped(msg.Content, contentWidth-2)))
				sb.WriteString("\n")
			}
		}
	}
	return sb.String()
}

func markdownRenderer(width int) *glamour.TermRenderer {
	wrapWidth := max(20, width-4)
	if renderer, ok := markdownRenderers.Load(wrapWidth); ok {
		return renderer.(*glamour.TermRenderer)
	}
	renderer, err := glamour.NewTermRenderer(
		compactMarkdownStyle(),
		glamour.WithWordWrap(wrapWidth),
	)
	if err != nil {
		renderer, _ = glamour.NewTermRenderer(glamour.WithWordWrap(wrapWidth))
	}
	actual, _ := markdownRenderers.LoadOrStore(wrapWidth, renderer)
	return actual.(*glamour.TermRenderer)
}

func compactMarkdownStyle() glamour.TermRendererOption {
	margin := uint(0)
	indent := uint(0)
	codeIndent := uint(2)
	return glamour.WithStyles(ansi.StyleConfig{
		Document: ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Color: stringPtr("#d1d5db")}, Margin: &margin},
		Paragraph: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{BlockPrefix: "", BlockSuffix: ""},
			Margin:         &margin,
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Color: stringPtr("#67e8f9"), Bold: boolPtr(true)},
			Margin:         &margin,
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Color: stringPtr("#86efac")},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{Color: stringPtr("#d1d5db")},
				Margin:         &margin,
				Indent:         &codeIndent,
			},
			Theme: "dracula",
		},
		List: ansi.StyleList{
			StyleBlock: ansi.StyleBlock{Margin: &margin, Indent: &indent},
		},
		Item: ansi.StylePrimitive{BlockPrefix: "", Prefix: "• "},
	})
}

func indentWrapped(s string, width int) string {
	if width <= 0 || lipgloss.Width(s) <= width {
		return s
	}
	var out []string
	var line []rune
	lineWidth := 0
	for _, r := range []rune(s) {
		if r == '\n' {
			out = append(out, string(line))
			line = line[:0]
			lineWidth = 0
			continue
		}
		rw := lipgloss.Width(string(r))
		if lineWidth > 0 && lineWidth+rw > width {
			out = append(out, string(line))
			line = line[:0]
			lineWidth = 0
		}
		line = append(line, r)
		lineWidth += rw
	}
	out = append(out, string(line))
	return strings.Join(out, "\n  ")
}

func stringPtr(s string) *string { return &s }
func boolPtr(v bool) *bool       { return &v }

func CompactToolArgs(args map[string]any, maxRunes int) string {
	if len(args) == 0 {
		return "{}"
	}

	preferred := []string{"path", "file", "url", "query", "language", "command", "cmd", "mode", "action", "index", "limit", "offset", "keyword"}
	parts := make([]string, 0, min(len(args), len(preferred)))
	seen := make(map[string]bool, len(preferred))
	for _, key := range preferred {
		if val, ok := args[key]; ok {
			parts = append(parts, key+"="+compactValue(val))
			seen[key] = true
		}
	}
	for key, val := range args {
		if seen[key] {
			continue
		}
		parts = append(parts, key+"="+compactValue(val))
		if len(parts) >= 4 {
			break
		}
	}
	if len(parts) == 0 {
		return "{}"
	}
	return TruncateRunes(strings.Join(parts, " "), maxRunes)
}

func CompactToolResult(result map[string]any, maxRunes int) string {
	if len(result) == 0 {
		return ""
	}
	if val, ok := result["error"]; ok {
		return TruncateRunes("error="+compactValue(val), maxRunes)
	}
	for _, key := range []string{"output", "result", "content", "message", "status", "path", "url"} {
		if val, ok := result[key]; ok {
			return TruncateRunes(key+"="+compactValue(val), maxRunes)
		}
	}
	data, err := json.Marshal(result)
	if err != nil {
		return "{...}"
	}
	return TruncateRunes(string(data), maxRunes)
}

func ToolResultStatus(result map[string]any) string {
	if _, ok := result["error"]; ok {
		return "error"
	}
	return "done"
}

func TruncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxRunes {
		return s
	}
	if maxRunes <= 1 {
		return "…"
	}
	var b strings.Builder
	width := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if width+rw > maxRunes-1 {
			break
		}
		b.WriteRune(r)
		width += rw
	}
	return b.String() + "…"
}

func compactValue(val any) string {
	switch v := val.(type) {
	case string:
		return fmt.Sprintf("%q", TruncateRunes(strings.ReplaceAll(v, "\n", "\\n"), 80))
	case nil:
		return "null"
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return TruncateRunes(string(data), 80)
	}
}

func toolStatusIcon(status string) string {
	switch status {
	case "done":
		return styles.ToolSuccessIcon
	case "error":
		return styles.ToolErrorIcon
	default:
		return styles.ToolPendingIcon
	}
}

func toolStatusStyle(status string, pulseOn bool) lipgloss.Style {
	switch status {
	case "done":
		return styles.ToolSuccess
	case "error":
		return styles.ToolError
	default:
		if pulseOn {
			return styles.ToolExecuting.Bold(true)
		}
		return styles.ToolExecuting.Faint(true)
	}
}

func toolMeta(msg Message) string {
	var parts []string
	if msg.Duration != "" {
		parts = append(parts, msg.Duration)
	}
	if len(parts) == 0 {
		return ""
	}
	return "[" + strings.Join(parts, " ") + "]"
}

func RenderWelcome(modelName, cwd, sessionID string) string {
	logo := styles.WelcomeTitle.Render("  SUPERMAN")
	title := styles.WelcomeTitle.Render("Ready for agent work")
	info := styles.WelcomeText.Render(
		"\n\nDirectory    " + cwd +
			"\nSession ID   " + sessionID +
			"\nModel        " + modelName +
			"\n\nType a request below. Use Ctrl+J for multiline input.",
	)
	content := lipgloss.JoinVertical(lipgloss.Left, logo, "", title, info)
	return styles.WelcomeBorder.Render(content)
}
