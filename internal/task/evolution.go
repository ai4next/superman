package task

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	adksession "google.golang.org/adk/session"
	"google.golang.org/genai"

	supermanagent "github.com/ai4next/superman/internal/agent"
	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/global"
	"github.com/ai4next/superman/internal/hook"
	"github.com/ai4next/superman/internal/memory"
	"github.com/ai4next/superman/internal/prompt"
	supermanruntime "github.com/ai4next/superman/internal/runtime"
	supermansession "github.com/ai4next/superman/internal/session"
)

// evolutionPromptData is the template data for the runtime evolution prompt.
type evolutionPromptData struct {
	Role               string
	AgentName          string
	RootDir            string
	L1Path             string
	SOPDir             string
	SoulPath           string
	ExpertDir          string
	SessionLogPath     string
	CanAddL1Section    bool
	CanDeleteL1Section bool
	CanCreateExpert    bool
	CultivateExperts   bool
	CanEditSoul        bool
}

// Evolution processes completed-run signal and evolves long-lived runtime assets
// from completed sessions using an ADK agent with read, write, patch,
// and code_run tools.
//
// The agent handles memory consolidation (facts + SOPs) and can optionally
// cultivate experts. The completed-run signal defines the root directory:
//   - Superman → workspace memory/sessions and experts/{name}/soul.md
//   - Expert   → experts/{expert_name}/memory and sessions under that expert
type Evolution struct {
	supermanRunner *runner.Runner
	expertRunner   *runner.Runner
	signal         chan hook.EvolutionSignal
	sessions       adksession.Service
	broker         *supermanruntime.Broker
}

// NewEvolution creates an ADK agent for memory consolidation and optional expert cultivation.
func NewEvolution(llm model.LLM, sessions adksession.Service) (*Evolution, error) {
	memSvc := memory.NewInRoot(global.EvolutionMemoryDir())
	if err := memSvc.LoadFromDisk(); err != nil {
		return nil, fmt.Errorf("load evolution memory: %w", err)
	}
	sessionService, err := supermansession.NewServiceInRoot(global.EvolutionDir())
	if err != nil {
		return nil, fmt.Errorf("create evolution session service: %w", err)
	}

	supermanRunner, err := newEvolutionRunner(llm, memSvc, sessionService, supermanagent.SupermanEvolverName, supermanagent.NewEvolver)
	if err != nil {
		return nil, err
	}
	expertRunner, err := newEvolutionRunner(llm, memSvc, sessionService, supermanagent.ExpertEvolverName, supermanagent.NewExpertEvolver)
	if err != nil {
		return nil, err
	}

	return &Evolution{
		supermanRunner: supermanRunner,
		expertRunner:   expertRunner,
		signal:         make(chan hook.EvolutionSignal, 16),
		sessions:       sessions,
	}, nil
}

type evolverFactory func(model.LLM, *config.Config, *memory.Service) (adkagent.Agent, error)

func newEvolutionRunner(llm model.LLM, memSvc *memory.Service, sessionService adksession.Service, appName string, factory evolverFactory) (*runner.Runner, error) {
	a, err := factory(llm, global.Config(), memSvc)
	if err != nil {
		return nil, err
	}
	r, err := runner.New(runner.Config{
		Agent:             a,
		AppName:           appName,
		SessionService:    sessionService,
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, fmt.Errorf("create %s runner: %w", appName, err)
	}
	return r, nil
}

func (e *Evolution) SetBroker(broker *supermanruntime.Broker) {
	e.broker = broker
}

func renderEvolutionPrompt(data evolutionPromptData) (string, error) {
	return prompt.EvolutionRuntime(data)
}

func evolutionDataFromSignal(signal hook.EvolutionSignal) evolutionPromptData {
	scope := evolutionScopeFromSignal(signal)
	cfg := global.Config()
	maxL1Sections := 100
	if cfg != nil && cfg.Memory.L1.MaxSections > 0 {
		maxL1Sections = cfg.Memory.L1.MaxSections
	}
	canCreateExpert := false
	if scope.ExpertDir != "" {
		canCreateExpert = true
		if cfg != nil && cfg.Expert.MaxCount > 0 {
			canCreateExpert = countExpertDirs(scope.ExpertDir) < cfg.Expert.MaxCount
		}
	}
	l1Sections := memory.CountL1Sections(scope.L1Path)
	return evolutionPromptData{
		Role:               scope.Role,
		AgentName:          scope.AgentName,
		RootDir:            scope.RootDir,
		L1Path:             scope.L1Path,
		SOPDir:             scope.SOPDir,
		SoulPath:           scope.SoulPath,
		ExpertDir:          scope.ExpertDir,
		SessionLogPath:     scope.SessionLogPath,
		CanAddL1Section:    l1Sections < maxL1Sections,
		CanDeleteL1Section: l1Sections >= maxL1Sections,
		CanCreateExpert:    canCreateExpert,
		CultivateExperts:   scope.ExpertDir != "",
		CanEditSoul:        scope.CanEditSoul,
	}
}

type evolutionScope struct {
	Role           string
	AgentName      string
	RootDir        string
	L1Path         string
	SOPDir         string
	SoulPath       string
	ExpertDir      string
	SessionLogPath string
	CanEditSoul    bool
}

func evolutionScopeFromSignal(signal hook.EvolutionSignal) evolutionScope {
	role := signal.Role
	if role == "" {
		role = "superman"
	}
	rootDir := signal.RootDir
	if rootDir == "" {
		if role == "expert" && signal.AgentName != "" {
			rootDir = global.ExpertDir(signal.AgentName)
		} else {
			rootDir = global.Config().Workspace
		}
	}
	agentName := signal.AgentName
	if agentName == "" {
		agentName = role
	}
	scope := evolutionScope{
		Role:           role,
		AgentName:      agentName,
		RootDir:        rootDir,
		L1Path:         global.MemoryL1Path(filepath.Join(rootDir, "memory")),
		SOPDir:         global.MemoryL2Dir(filepath.Join(rootDir, "memory")),
		SessionLogPath: filepath.Join(rootDir, "sessions", signal.SessionID+".log"),
	}
	if role == "superman" {
		scope.ExpertDir = filepath.Join(rootDir, "experts")
	} else if role == "expert" {
		scope.SoulPath = filepath.Join(rootDir, "soul.md")
		scope.CanEditSoul = true
	}
	return scope
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
	data := evolutionDataFromSignal(signal)
	instruction, err := renderEvolutionPrompt(data)
	if err != nil {
		return fmt.Errorf("render prompt: %w", err)
	}

	scope := evolutionScopeFromSignal(signal)
	run, err := e.runnerFor(scope)
	if err != nil {
		return err
	}
	log.Printf("[evolution] processing %s session %s", scope.Role, signal.SessionID)
	msg := genai.NewContentFromText(instruction, genai.RoleUser)
	for evt, err := range run.Run(ctx, "evolution", "evolution-"+scope.Role+"-"+scope.AgentName+"-"+signal.SessionID, msg, adkagent.RunConfig{}) {
		if err != nil {
			return fmt.Errorf("agent run: %w", err)
		}
		_ = evt
	}
	log.Printf("[evolution] %s session %s done", scope.Role, signal.SessionID)
	if err := validateEvolutionOutput(scope); err != nil {
		return fmt.Errorf("validate output: %w", err)
	}
	return nil
}

func (e *Evolution) runnerFor(scope evolutionScope) (*runner.Runner, error) {
	if scope.Role == "expert" {
		if e.expertRunner == nil {
			return nil, fmt.Errorf("missing expert evolution runner")
		}
		return e.expertRunner, nil
	}
	if e.supermanRunner == nil {
		return nil, fmt.Errorf("missing superman evolution runner")
	}
	return e.supermanRunner, nil
}

func (e *Evolution) publish(event supermanruntime.Event) {
	if e != nil && e.broker != nil {
		e.broker.Publish(event)
	}
}

func validateEvolutionOutput(scope evolutionScope) error {
	if err := validateL1TOML(scope.L1Path); err != nil {
		return err
	}
	if err := validateSOPDir(scope.SOPDir); err != nil {
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
