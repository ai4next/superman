package cli

import (
	"context"
	"fmt"
	"log"

	"github.com/spf13/cobra"

	adkplugin "google.golang.org/adk/plugin"
	"google.golang.org/adk/runner"

	"github.com/ai4next/superman/internal/agent"
	"github.com/ai4next/superman/internal/expert"
	"github.com/ai4next/superman/internal/global"
	"github.com/ai4next/superman/internal/memory"
	"github.com/ai4next/superman/internal/model"
	"github.com/ai4next/superman/internal/plugin"
	supermanruntime "github.com/ai4next/superman/internal/runtime"
	supermansession "github.com/ai4next/superman/internal/session"
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
	memSvc := memory.New()
	if err := memSvc.LoadFromDisk(); err != nil {
		log.Printf("[cli] memory load warning: %v", err)
	}

	sessionService, err := supermansession.NewService()
	if err != nil {
		return fmt.Errorf("create session service: %w", err)
	}

	// Evolution service: ADK agent for memory consolidation + optional expert cultivation.
	evolution, err := task.NewEvolution(llm, sessionService)
	if err != nil {
		return fmt.Errorf("create evolution agent: %w", err)
	}
	evolutionBroker := supermanruntime.NewBroker()
	evolution.SetBroker(evolutionBroker)
	supermanruntime.NewAuditLogger(global.RuntimeEventsPath()).Subscribe(ctx, evolutionBroker)
	go evolution.Loop(ctx)

	evolutionCh := evolution.SignalCh()

	// Expert Registry
	var expertRegistry *expert.Registry
	var delegateRunner tool.DelegateRunner
	if cfg.Expert.Enabled {
		expertRegistry = expert.NewRegistry(global.ExpertsDir())
		if err := expertRegistry.LoadFromDisk(); err != nil {
			log.Printf("[expert] load warning: %v", err)
		}
		delegateRunner = newDelegateService(llm, expertRegistry, evolutionCh)
		log.Printf("[expert] loaded %d experts", len(expertRegistry.List()))
	}

	var adkPlugins []*adkplugin.Plugin
	a, extraPlugins, err := agent.New(llm, cfg, memSvc, sessionService, expertRegistry, delegateRunner, evolutionCh)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}
	adkPlugins = append(adkPlugins, extraPlugins...)

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

	log.Printf("[cli] starting TUI with model %s/%s (%d plugins)",
		cfg.Model.Provider, cfg.Model.Name, len(adkPlugins))
	return runTUI(ctx, a, cfg, runner.PluginConfig{Plugins: adkPlugins}, sessionService)
}
