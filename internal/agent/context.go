package agent

import (
	"log"
	"strings"

	supermansession "github.com/ai4next/superman/internal/session"
	adkagent "google.golang.org/adk/agent"
)

func instructionProvider(build BuildConfig) func(adkagent.ReadonlyContext) (string, error) {
	builder := strings.Builder{}
	return func(ctx adkagent.ReadonlyContext) (string, error) {
		defer builder.Reset()
		builder.WriteString(build.Instruction)
		if build.SOPContent != "" {
			builder.WriteString("\n\n## SOP Rules\n")
			builder.WriteString(build.SOPContent)
		}
		if build.MemoryService != nil {
			if l0Content := build.MemoryService.GetL0Content(); l0Content != "" {
				builder.WriteString("\n\n")
				builder.WriteString(l0Content)
			}
		}
		if build.SessionService != nil && build.ContextMessages != 0 {
			limit := build.ContextMessages
			if limit < 0 {
				limit = 12
			}
			window, err := build.SessionService.ContextWindow(ctx.AppName(), ctx.UserID(), ctx.SessionID(), limit)
			if err == nil {
				if hasSessionContext(window) {
					builder.WriteString("\n\n## Session Context Usage\n")
					builder.WriteString("- Treat session context as compact hints, not complete history.\n")
					builder.WriteString("- Working files are path/status pointers only; call file tools before relying on file contents.\n")
					builder.WriteString("- Session references are user-selected historical pointers; use their preview as intent, not as full transcript.\n")
				}
				if strings.TrimSpace(window.Summary) != "" {
					builder.WriteString("\n\n## Session Summary\n")
					builder.WriteString(window.Summary)
				}
				if len(window.Messages) > 0 {
					builder.WriteString("\n\n## Recent Session Context\n")
					for _, msg := range window.Messages {
						builder.WriteString("- ")
						builder.WriteString(string(msg.Role))
						if msg.ToolName != "" {
							builder.WriteString("/")
							builder.WriteString(msg.ToolName)
						}
						builder.WriteString(": ")
						builder.WriteString(compactContextText(msg.Content, msg.Result, msg.Args))
						builder.WriteByte('\n')
					}
				}
				if len(window.Files) > 0 {
					builder.WriteString("\n\n## Session Working Files\n")
					for _, file := range window.Files {
						builder.WriteString("- ")
						builder.WriteString(file.Path)
						var parts []string
						if file.ReadCount > 0 {
							parts = append(parts, "read")
						}
						if file.WriteCount > 0 {
							parts = append(parts, "modified")
						}
						if len(parts) > 0 {
							builder.WriteString(" (")
							builder.WriteString(strings.Join(parts, ", "))
							builder.WriteString(")")
						}
						builder.WriteByte('\n')
					}
				}
				if len(window.References) > 0 {
					builder.WriteString("\n\n## Session References\n")
					for _, ref := range window.References {
						builder.WriteString("- ")
						builder.WriteString(ref.SessionID)
						if ref.Role != "" {
							builder.WriteString(" [")
							builder.WriteString(string(ref.Role))
							builder.WriteString("]")
						}
						if strings.TrimSpace(ref.Preview) != "" {
							builder.WriteString(": ")
							builder.WriteString(compactContextText(ref.Preview))
						}
						builder.WriteByte('\n')
					}
				}
			}
		}
		log.Printf("[agent] session context:\n%s", builder.String())
		return builder.String(), nil
	}
}

func hasSessionContext(window supermansession.ContextWindow) bool {
	return strings.TrimSpace(window.Summary) != "" || len(window.Messages) > 0 || len(window.Files) > 0 || len(window.References) > 0
}

func compactContextText(values ...string) string {
	text := strings.TrimSpace(strings.Join(values, " "))
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) > 600 {
		return string(runes[:599]) + "…"
	}
	return text
}
