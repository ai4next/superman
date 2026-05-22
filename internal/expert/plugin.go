package expert

import (
	"fmt"
	"log"
	"strings"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/plugin"
	"google.golang.org/genai"
)

// NewDispatcherPlugin creates an ADK plugin that registers the expert dispatcher
// in the plugin pipeline. The BeforeRunCallback returns nil, nil to avoid
// pre-empting the agent flow; expert context is injected via BuildExpertContext
// called from the application layer before each turn.
func NewDispatcherPlugin() (*plugin.Plugin, error) {
	return plugin.New(plugin.Config{
		Name: "expert_dispatcher",
		BeforeRunCallback: func(ic agent.InvocationContext) (*genai.Content, error) {
			return nil, nil
		},
	})
}

// BuildExpertContext searches for matching experts and returns a formatted
// context string suitable for injection into the agent prompt. It records each
// activation as a consult-mode call.
func BuildExpertContext(registry *Registry, userMessage string, topK int) string {
	if registry == nil || userMessage == "" {
		return ""
	}
	results := registry.Search(userMessage)
	if len(results) == 0 {
		return ""
	}
	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}

	var b strings.Builder
	b.WriteString("\n## Activated Experts\n")
	for _, spec := range results {
		b.WriteString(fmt.Sprintf("\n### %s\n%s\nAllowed tools: %s\n",
			spec.Name, spec.SystemPrompt, strings.Join(spec.ToolAllowlist, ", ")))

		// Record the call
		_ = registry.RecordCall(spec.Name, CallRecord{
			Timestamp: time.Now(),
			TaskDesc:  userMessage,
			Mode:      ModeConsult,
		})
		log.Printf("[expert] activated expert %q (consult mode)", spec.Name)
	}
	return b.String()
}

// RegisterDispatcherPlugin creates and registers the dispatcher plugin into the
// provided plugin slice, returning the updated slice. Errors are logged but
// non-fatal; a nil return signals a skip.
func RegisterDispatcherPlugin(plugins []*plugin.Plugin) []*plugin.Plugin {
	p, err := NewDispatcherPlugin()
	if err != nil {
		log.Printf("[expert] dispatcher plugin: %v", err)
		return plugins
	}
	return append(plugins, p)
}
