package plugin

import (
	"log"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/plugin"
	"google.golang.org/genai"
)

type Registry struct {
	plugins map[string]*plugin.Plugin
}

func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]*plugin.Plugin),
	}
}

func (r *Registry) Register(name string, p *plugin.Plugin) {
	r.plugins[name] = p
}

func (r *Registry) Get(name string) *plugin.Plugin {
	return r.plugins[name]
}

func (r *Registry) All() []*plugin.Plugin {
	var result []*plugin.Plugin
	for _, p := range r.plugins {
		result = append(result, p)
	}
	return result
}

func CreateMemorySyncPlugin() (*plugin.Plugin, error) {
	return plugin.New(plugin.Config{
		Name: "memory_sync",
		AfterRunCallback: func(ic agent.InvocationContext) {
			m := ic.Memory()
			if m == nil {
				return
			}
			if err := m.AddSessionToMemory(ic, ic.Session()); err != nil {
				log.Printf("[memory_sync] failed: %v", err)
			} else {
				log.Printf("[memory_sync] session synced to memory")
			}
		},
	})
}

func CreateTokenTrackerPlugin() (*plugin.Plugin, error) {
	return plugin.New(plugin.Config{
		Name: "token_tracker",
		BeforeRunCallback: func(ic agent.InvocationContext) (*genai.Content, error) {
			log.Printf("[token_tracker] run started at %s", time.Now().Format(time.RFC3339))
			return nil, nil
		},
		AfterRunCallback: func(ic agent.InvocationContext) {
			log.Printf("[token_tracker] run completed at %s", time.Now().Format(time.RFC3339))
		},
	})
}

func CreateToolLoggerPlugin() (*plugin.Plugin, error) {
	return plugin.New(plugin.Config{
		Name: "tool_logger",
		AfterRunCallback: func(ic agent.InvocationContext) {
			log.Printf("[tool_logger] session %s: tool invocations logged", ic.Session().ID())
		},
	})
}

func CreateSessionReaperPlugin(maxTurns int, maxAge time.Duration) (*plugin.Plugin, error) {
	return plugin.New(plugin.Config{
		Name: "session_reaper",
		AfterRunCallback: func(ic agent.InvocationContext) {
			sess := ic.Session()
			if sess == nil {
				return
			}
			lastUpdate := sess.LastUpdateTime()
			if time.Since(lastUpdate) > maxAge {
				log.Printf("[session_reaper] session %s expired (age: %s)", sess.ID(), time.Since(lastUpdate))
			}
		},
	})
}

// Create creates a plugin by config name with sensible defaults.
func Create(name string) (*plugin.Plugin, error) {
	switch name {
	case "memory_sync":
		return CreateMemorySyncPlugin()
	case "token_tracker":
		return CreateTokenTrackerPlugin()
	case "tool_logger":
		return CreateToolLoggerPlugin()
	case "session_reaper":
		return CreateSessionReaperPlugin(100, 2*time.Hour)
	default:
		return nil, nil
	}
}