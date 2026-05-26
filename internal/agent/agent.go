package agent

import (
	"context"
	_ "embed"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/plugin"
	adksession "google.golang.org/adk/session"
	adktool "google.golang.org/adk/tool"
	"google.golang.org/adk/tool/mcptoolset"
	"google.golang.org/adk/tool/skilltoolset"
	"google.golang.org/adk/tool/skilltoolset/skill"

	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/expert"
	"github.com/ai4next/superman/internal/hook"
	"github.com/ai4next/superman/internal/memory"
	"github.com/ai4next/superman/internal/tool"
)

//go:embed system.txt
var systemPrompt string

// BuildConfig describes one concrete agent instance. Superman and experts use
// this same builder so their runtime wiring stays consistent.
type BuildConfig struct {
	Name              string
	Instruction       string
	MemoryService     *memory.Service
	SessionService    adksession.Service
	ContextMessages   int
	SOPContent        string
	ExpertRegistry    *expert.Registry
	DelegateRunner    tool.DelegateRunner
	EnableExpertTools bool
	EvolutionSignal   hook.EvolutionSignal
	EvolutionCh       chan<- hook.EvolutionSignal // completed-run signal receiver
}

// ToolsetDescriptor describes one configured ADK toolset before it is attached
// to an agent. It is intentionally config-derived so CLI/TUI/debug surfaces can
// explain Superman's external abilities without opening network/process-backed
// toolsets.
type ToolsetDescriptor struct {
	Name                 string   `json:"name"`
	Kind                 string   `json:"kind"`
	Source               string   `json:"source"`
	Tools                []string `json:"tools,omitempty"`
	RequiresConfirmation bool     `json:"requires_confirmation"`
}

// DescribeConfiguredToolsets returns the ADK toolsets Superman will attach for
// the given config, excluding disabled or incomplete entries.
func DescribeConfiguredToolsets(cfg *config.Config) []ToolsetDescriptor {
	if cfg == nil {
		return nil
	}
	var out []ToolsetDescriptor
	if cfg.Skills.Enabled {
		for _, skillsDir := range configuredSkillPaths(cfg) {
			if strings.TrimSpace(skillsDir) == "" {
				continue
			}
			out = append(out, ToolsetDescriptor{
				Name:   "skills:" + filepath.Base(skillsDir),
				Kind:   "skill",
				Source: skillsDir,
			})
		}
	}
	for _, server := range cfg.MCP.Servers {
		if !server.Enabled || strings.TrimSpace(server.Command) == "" {
			continue
		}
		source := strings.TrimSpace(strings.Join(append([]string{server.Command}, server.Args...), " "))
		out = append(out, ToolsetDescriptor{
			Name:                 "mcp:" + firstNonEmpty(server.Name, server.Command),
			Kind:                 "mcp",
			Source:               source,
			Tools:                append([]string(nil), server.Tools...),
			RequiresConfirmation: false,
		})
	}
	return out
}

// NewFromConfig creates an agent with the shared Superman runtime wiring:
// configured tools, optional isolated memory, shared hooks, and shared skills.
func NewFromConfig(llm model.LLM, cfg *config.Config, build BuildConfig) (adkagent.Agent, []*plugin.Plugin, error) {
	expertRegistry := build.ExpertRegistry
	delegateRunner := build.DelegateRunner
	if !build.EnableExpertTools {
		expertRegistry = nil
		delegateRunner = nil
	}

	deps := tool.Dependencies{
		Config:         cfg,
		ExpertManager:  expertRegistry,
		DelegateRunner: delegateRunner,
		ExpertTools:    build.EnableExpertTools,
	}

	toolList := tool.RegisterAll(deps)

	var extraPlugins []*plugin.Plugin
	builtin, err := NewBuiltin(build)
	if err != nil {
		return nil, nil, err
	}
	extraPlugins = append(extraPlugins, builtin)
	signal := build.EvolutionSignal
	if signal.AgentName == "" {
		signal.AgentName = build.Name
	}
	hookMgr, err := hook.NewManagerWithSignal(filepath.Join(cfg.Workspace, "hooks"), signal, build.EvolutionCh)
	if err != nil {
		log.Printf("[agent] hook manager: %v", err)
	} else {
		extraPlugins = append(extraPlugins, hookMgr.Plugin())
	}

	agentToolsets := buildToolsets(context.Background(), cfg)

	agentConfig := llmagent.Config{
		Name:     build.Name,
		Model:    llm,
	}
	if len(toolList) > 0 {
		agentConfig.Tools = toolList
	}
	if len(agentToolsets) > 0 {
		agentConfig.Toolsets = agentToolsets
	}
	a, err := llmagent.New(agentConfig)
	if err != nil {
		return nil, nil, err
	}

	return a, extraPlugins, nil
}

func buildToolsets(ctx context.Context, cfg *config.Config) []adktool.Toolset {
	var toolsets []adktool.Toolset
	toolsets = append(toolsets, buildSkillToolsets(ctx, cfg)...)
	toolsets = append(toolsets, buildMCPToolsets(cfg)...)
	return toolsets
}

func buildSkillToolsets(ctx context.Context, cfg *config.Config) []adktool.Toolset {
	if cfg == nil || !cfg.Skills.Enabled {
		return nil
	}
	var toolsets []adktool.Toolset
	for _, skillsDir := range configuredSkillPaths(cfg) {
		if strings.TrimSpace(skillsDir) == "" {
			continue
		}
		skillFS := os.DirFS(skillsDir)
		skillSource := skill.NewFileSystemSource(skillFS)
		skillTS, err := skilltoolset.New(ctx, skilltoolset.Config{
			Name:   "skills:" + filepath.Base(skillsDir),
			Source: skillSource,
		})
		if err != nil {
			log.Printf("[agent] skill toolset %s: %v", skillsDir, err)
			continue
		}
		toolsets = append(toolsets, skillTS)
	}
	return toolsets
}

func configuredSkillPaths(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	if len(cfg.Skills.Paths) > 0 {
		return cfg.Skills.Paths
	}
	return []string{filepath.Join(cfg.Workspace, "skills")}
}

func buildMCPToolsets(cfg *config.Config) []adktool.Toolset {
	if cfg == nil {
		return nil
	}
	var toolsets []adktool.Toolset
	for _, server := range cfg.MCP.Servers {
		if !server.Enabled || strings.TrimSpace(server.Command) == "" {
			continue
		}
		ts, err := mcptoolset.New(mcptoolset.Config{
			Transport: &mcp.CommandTransport{
				Command: exec.Command(server.Command, server.Args...),
			},
			ToolFilter:                  mcpToolFilter(server.Tools),
			RequireConfirmation:         false,
			RequireConfirmationProvider: nil,
		})
		if err != nil {
			log.Printf("[agent] mcp toolset %s: %v", firstNonEmpty(server.Name, server.Command), err)
			continue
		}
		toolsets = append(toolsets, namedToolset{name: "mcp:" + firstNonEmpty(server.Name, server.Command), Toolset: ts})
		log.Printf("[agent] mcp toolset configured: %s", firstNonEmpty(server.Name, server.Command))
	}
	return toolsets
}

func mcpToolFilter(names []string) adktool.Predicate {
	if len(names) == 0 {
		return nil
	}
	return adktool.StringPredicate(names)
}

type namedToolset struct {
	name string
	adktool.Toolset
}

func (n namedToolset) Name() string {
	if n.name != "" {
		return n.name
	}
	return n.Toolset.Name()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// New creates a new Superman agent with all registered tools, optional SOP rules,
// L0 index injected into the system prompt, and completed-run signal receivers.
func New(llm model.LLM, cfg *config.Config, memSvc *memory.Service, sessionSvc adksession.Service, expertRegistry *expert.Registry, delegateRunner tool.DelegateRunner, evolutionCh chan<- hook.EvolutionSignal) (adkagent.Agent, []*plugin.Plugin, error) {
	return NewFromConfig(llm, cfg, BuildConfig{
		Name:              "superman",
		Instruction:       systemPrompt,
		MemoryService:     memSvc,
		SessionService:    sessionSvc,
		ContextMessages:   12,
		ExpertRegistry:    expertRegistry,
		DelegateRunner:    delegateRunner,
		EnableExpertTools: true,
		EvolutionSignal: hook.EvolutionSignal{
			UserID: "tui-user",
			Role:   "superman",
		},
		EvolutionCh: evolutionCh,
	})
}
