package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	adkplugin "google.golang.org/adk/plugin"
	"google.golang.org/adk/runner"
	adksession "google.golang.org/adk/session"

	"github.com/ai4next/superman/internal/agent"
	"github.com/ai4next/superman/internal/expert"
	"github.com/ai4next/superman/internal/global"
	"github.com/ai4next/superman/internal/memory"
	"github.com/ai4next/superman/internal/model"
	"github.com/ai4next/superman/internal/plugin"
	"github.com/ai4next/superman/internal/task"
	"github.com/ai4next/superman/internal/tool"
)

// RunServe launches the TUI chat interface. Shared by the root command and serve subcommand.
func RunServe(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	cfg := global.Config()

	// Model
	llm, err := model.New(ctx, cfg.Model)
	if err != nil {
		return fmt.Errorf("create model: %w", err)
	}

	// Memory service with file persistence
	supermanMemoryDir := filepath.Join(cfg.Workspace, "memory")
	memSvc := memory.New(supermanMemoryDir)
	if err := memSvc.LoadFromDisk(); err != nil {
		log.Printf("[cli] memory load warning: %v", err)
	}

	// Load SOP content from l2/ directory
	var sopContent string
	l2Dir := filepath.Join(supermanMemoryDir, "l2")
	if files, err := os.ReadDir(l2Dir); err == nil {
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
				continue
			}
			data, _ := os.ReadFile(filepath.Join(l2Dir, f.Name()))
			if len(data) > 0 {
				sopContent += "\n### " + f.Name() + "\n" + string(data) + "\n"
			}
		}
	}

	// Evolution service: ADK agent for memory consolidation + optional expert cultivation.
	evolution, err := task.NewEvolution(llm)
	if err != nil {
		return fmt.Errorf("create evolution agent: %w", err)
	}
	go evolution.Loop(ctx)

	evolutionCh := evolution.SignalCh()

	// Expert Registry
	var expertRegistry *expert.Registry
	var delegateRunner tool.DelegateRunner
	if cfg.Expert.Enabled {
		expertRegistry = expert.NewRegistry(cfg.ExpertDir())
		if err := expertRegistry.LoadFromDisk(); err != nil {
			log.Printf("[expert] load warning: %v", err)
		}
		delegateRunner = newDelegateService(llm, expertRegistry, evolutionCh)
		log.Printf("[expert] loaded %d experts", len(expertRegistry.List()))
	}

	sessionService := adksession.InMemoryService()

	// Plugins
	var adkPlugins []*adkplugin.Plugin
	for _, pc := range cfg.Plugins {
		if !pc.Enabled {
			continue
		}
		p, err := plugin.Create(pc.Name)
		if err != nil {
			log.Printf("[cli] plugin %s skipped: %v", pc.Name, err)
			continue
		}
		if p != nil {
			adkPlugins = append(adkPlugins, p)
		}
	}

	a, extraPlugins, err := agent.New(llm, cfg, memSvc, sopContent, expertRegistry, delegateRunner, evolutionCh)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}
	adkPlugins = append(adkPlugins, extraPlugins...)

	log.Printf("[cli] starting TUI with model %s/%s (%d plugins)",
		cfg.Model.Provider, cfg.Model.Name, len(adkPlugins))
	return runTUI(ctx, a, cfg, runner.PluginConfig{Plugins: adkPlugins}, sessionService)
}
