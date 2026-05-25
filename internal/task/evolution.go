package task

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/pelletier/go-toml/v2"
	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	adksession "google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/global"
	"github.com/ai4next/superman/internal/hook"
	"github.com/ai4next/superman/internal/memory"
	supermanruntime "github.com/ai4next/superman/internal/runtime"
	supermansession "github.com/ai4next/superman/internal/session"
	"github.com/ai4next/superman/internal/tool"
)

//go:embed evolution_prompt.txt
var evolutionPrompt string

// evolutionPromptData is the template data for evolution_prompt.txt.
type evolutionPromptData struct {
	L1Path             string
	SopDir             string
	L3Path             string
	ExpertDir          string
	SessionDir         string
	CanAddL1Section    bool
	CanDeleteL1Section bool
	CanCreateExpert    bool
	CultivateExperts   bool
}

// Evolution processes completed-run signal and evolves long-lived runtime assets
// from completed sessions using an ADK agent with read, write, patch,
// and code_run tools.
//
// The agent handles memory consolidation (facts + SOPs) and can optionally cultivate experts:
//   - L0 (runtime index) → l1.toml sections + l2/*.md names
//   - L1 (facts)         → l1.toml
//   - L2 (SOPs)          → l2/*.md
//   - Session logs       → sessions/*.log
//   - Expert definitions → {expertDir}/{name}/expert.yaml when expertDir is set
type Evolution struct {
	runner   *runner.Runner
	signal   chan hook.EvolutionSignal
	sessions *supermansession.Service
	broker   *supermanruntime.Broker
}

// NewEvolution creates an ADK agent for memory consolidation and optional expert cultivation.
func NewEvolution(llm model.LLM, sessions *supermansession.Service) (*Evolution, error) {
	deps := tool.Dependencies{
		Config: evolutionToolConfig(),
	}

	tools := tool.RegisterAll(deps)

	a, err := llmagent.New(llmagent.Config{
		Name:  "superman-evolution",
		Model: llm,
		InstructionProvider: func(adkagent.ReadonlyContext) (string, error) {
			return "You are the evolution agent. Follow the current user message for the session, role, and directories to process.", nil
		},
		Tools: tools,
	})
	if err != nil {
		return nil, fmt.Errorf("create evolution agent: %w", err)
	}

	r, err := runner.New(runner.Config{
		Agent:             a,
		AppName:           "superman-evolution",
		SessionService:    adksession.InMemoryService(),
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, fmt.Errorf("create evolution runner: %w", err)
	}

	return &Evolution{
		runner:   r,
		signal:   make(chan hook.EvolutionSignal, 16),
		sessions: sessions,
	}, nil
}

func evolutionToolConfig() *config.Config {
	return &config.Config{
		Permissions: config.PermissionsConfig{
			SkipRequests: true,
		},
		Tools: config.ToolsConfig{
			Read: config.ReadConfig{
				Enabled: true,
				MaxSize: 10 * 1024 * 1024,
			},
			Write: config.WriteConfig{
				Enabled: true,
				MaxSize: 10 * 1024 * 1024,
			},
			Patch: config.PatchConfig{
				Enabled: true,
			},
			CodeRun: config.CodeRunConfig{
				Enabled:          true,
				Timeout:          config.Duration(30 * time.Second),
				AllowedLanguages: []string{"python", "bash"},
			},
		},
	}
}

func (e *Evolution) SetBroker(broker *supermanruntime.Broker) {
	e.broker = broker
}

func renderEvolutionPrompt(data evolutionPromptData) (string, error) {
	tmpl, err := template.New("evolution").Parse(evolutionPrompt)
	if err != nil {
		return "", fmt.Errorf("parse prompt template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute prompt template: %w", err)
	}
	return buf.String(), nil
}

func evolutionDataFromSignal(signal hook.EvolutionSignal) evolutionPromptData {
	cfg := global.Config()
	maxL1Sections := 100
	if cfg != nil && cfg.Memory.L1.MaxSections > 0 {
		maxL1Sections = cfg.Memory.L1.MaxSections
	}
	memoryDir := global.MemoryDir()
	expertDir := ""
	if signal.Role == "superman" {
		expertDir = global.ExpertsDir()
	}
	canCreateExpert := false
	if expertDir != "" {
		canCreateExpert = true
		if cfg != nil && cfg.Expert.MaxCount > 0 {
			canCreateExpert = countExpertDirs(expertDir) < cfg.Expert.MaxCount
		}
	}
	l1Path := global.MemoryL1Path(memoryDir)
	sopDir := global.MemoryL2Dir(memoryDir)
	sessionDir := global.SessionsDir()
	sessionLogPath := global.SessionLogPath(signal.SessionID)
	l1Sections := memory.CountL1Sections(l1Path)
	return evolutionPromptData{
		L1Path:             l1Path,
		SopDir:             sopDir,
		L3Path:             sessionLogPath,
		ExpertDir:          expertDir,
		SessionDir:         sessionDir,
		CanAddL1Section:    l1Sections < maxL1Sections,
		CanDeleteL1Section: l1Sections >= maxL1Sections,
		CanCreateExpert:    canCreateExpert,
		CultivateExperts:   expertDir != "",
	}
}

func countExpertDirs(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			count++
		}
	}
	return count
}

// SignalCh returns the send-only channel for completed-run signal.
func (e *Evolution) SignalCh() chan<- hook.EvolutionSignal {
	return e.signal
}

// Loop listens for completed-run signal and runs the evolution agent.
func (e *Evolution) Loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case signal := <-e.signal:
			e.publish(supermanruntime.EvolutionStarted(signal.SessionID, signal.Role))
			if err := e.runAgent(ctx, signal); err != nil {
				e.publish(supermanruntime.EvolutionFailed(signal.SessionID, signal.Role, err))
				log.Printf("[evolution] %s: %v", signal.SessionID, err)
			} else {
				e.publish(supermanruntime.EvolutionFinished(signal.SessionID, signal.Role, ""))
			}
		}
	}
}

// runAgent launches the evolution agent with the signal-scoped session.
func (e *Evolution) runAgent(ctx context.Context, signal hook.EvolutionSignal) error {
	if signal.SessionID == "" {
		return fmt.Errorf("missing session ID")
	}
	if e.runner == nil {
		return fmt.Errorf("missing evolution runner")
	}

	data := evolutionDataFromSignal(signal)
	instruction, err := renderEvolutionPrompt(data)
	if err != nil {
		return fmt.Errorf("render prompt: %w", err)
	}

	log.Printf("[evolution] processing session %s", signal.SessionID)
	msg := genai.NewContentFromText(instruction, genai.RoleUser)
	for evt, err := range e.runner.Run(ctx, "evolution", "evolution-"+signal.SessionID, msg, adkagent.RunConfig{}) {
		if err != nil {
			return fmt.Errorf("agent run: %w", err)
		}
		_ = evt
	}
	log.Printf("[evolution] session %s done", signal.SessionID)
	if err := validateEvolutionOutput(data); err != nil {
		return fmt.Errorf("validate output: %w", err)
	}
	return nil
}

func (e *Evolution) publish(event supermanruntime.Event) {
	if e != nil && e.broker != nil {
		e.broker.Publish(event)
	}
}

func validateEvolutionOutput(data evolutionPromptData) error {
	if err := validateL1TOML(data.L1Path); err != nil {
		return err
	}
	if err := validateSOPDir(data.SopDir); err != nil {
		return err
	}
	return nil
}

func validateL1TOML(path string) error {
	if path == "" {
		return nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read l1: %w", err)
	}
	var parsed map[string]any
	if err := toml.Unmarshal(content, &parsed); err != nil {
		return fmt.Errorf("parse l1 toml: %w", err)
	}
	return nil
}

func validateSOPDir(dir string) error {
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read sop dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if strings.ContainsAny(entry.Name(), `/\`) {
			return fmt.Errorf("invalid sop filename: %s", entry.Name())
		}
		path := filepath.Join(dir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read sop %s: %w", entry.Name(), err)
		}
		if strings.TrimSpace(string(content)) == "" {
			return fmt.Errorf("empty sop file: %s", entry.Name())
		}
	}
	return nil
}
