package cli

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/spf13/cobra"

	adkplugin "google.golang.org/adk/plugin"
	"google.golang.org/adk/runner"
	adksession "google.golang.org/adk/session"

	"github.com/ai4next/superman/internal/agent"
	"github.com/ai4next/superman/internal/agent/tools"
	"github.com/ai4next/superman/internal/expert"
	"github.com/ai4next/superman/internal/memory"
	"github.com/ai4next/superman/internal/model"
	"github.com/ai4next/superman/internal/plugin"
	"github.com/ai4next/superman/internal/session"
)

// memorySearchAdapter wraps memory.Service to implement tools.MemorySearcher.
type memorySearchAdapter struct {
	svc *memory.Service
}

func (a *memorySearchAdapter) Search(ctx context.Context, query string) ([]tools.SearchResult, error) {
	entries, err := a.svc.Search(ctx, query)
	if err != nil {
		return nil, err
	}
	results := make([]tools.SearchResult, len(entries))
	for i, e := range entries {
		results[i] = tools.SearchResult{
			ID:      e.ID,
			Summary: e.Summary,
			Layer:   e.Layer,
			Content: e.Content,
		}
	}
	return results, nil
}

// RunServe launches the TUI chat interface. Shared by the root command and serve subcommand.
func RunServe(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Model
	llm := model.MustNew(ctx, cfg.Model)

	// Memory service (L1-L3) with file persistence
	memSvc := memory.New(cfg.Memory.L1.MaxEntries, cfg.Memory.L2.Dir)
	if err := memSvc.LoadFromDisk(); err != nil {
		log.Printf("[cli] memory load warning: %v", err)
	}

	// L0 SOP store — load templates and inject into agent prompt
	var sopContent string
	l0, err := memory.NewL0Store(cfg.Memory.L0.SOPDir)
	if err != nil {
		log.Printf("[cli] L0Store warning: %v", err)
	}
	if l0 != nil {
		rules := l0.All()
		if len(rules) > 0 {
			log.Printf("[cli] loaded %d L0 SOP rules", len(rules))
			for name, content := range rules {
				sopContent += "\n### " + name + "\n" + content + "\n"
				log.Printf("[cli]   SOP: %s", name)
			}
		}
	}

	// Periodic archiving (L2 → L3)
	go func() {
		ticker := time.NewTicker(cfg.Memory.L3.ArchiveInterval.AsDuration())
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if archived, _ := memSvc.Archive(ctx, 48*time.Hour); archived > 0 {
					log.Printf("[memory] archived %d entries", archived)
				}
			}
		}
	}()

	// L4 archiver: compress old session files
	if cfg.Memory.L4.Enabled {
		go func() {
			ttl := cfg.Memory.L4.SessionTTL.AsDuration()
			ticker := time.NewTicker(cfg.Memory.L4.ArchiveInterval.AsDuration())
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if archived, _ := memory.ArchiveSessions(ctx, cfg.Session.HistoryPath, cfg.Memory.L2.Dir, ttl); archived > 0 {
						log.Printf("[memory] L4 archived %d sessions", archived)
					}
				}
			}
		}()
	}

	// Expert Registry
	var expertRegistry *expert.Registry
	if cfg.Expert.Enabled {
		expertRegistry = expert.NewRegistry(cfg.Expert.Dir)
		if err := expertRegistry.LoadFromDisk(); err != nil {
			log.Printf("[expert] load warning: %v", err)
		}
		log.Printf("[expert] loaded %d experts", len(expertRegistry.List()))
	}

	// Pattern analysis for expert discovery (Phase 2)
	if cfg.Expert.Enabled && expertRegistry != nil {
		patternAnalyzer := expert.NewAnalyzer(cfg.Session.HistoryPath, expertRegistry)
		go func() {
			ticker := time.NewTicker(30 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					created, err := patternAnalyzer.RunAnalysis()
					if err != nil {
						log.Printf("[expert] pattern analysis: %v", err)
					} else if len(created) > 0 {
						log.Printf("[expert] pattern analysis created %d new expert drafts", len(created))
						for _, s := range created {
							log.Printf("[expert]   draft: %s (confidence: %.2f)", s.Name, s.Confidence)
						}
					}
				}
			}
		}()
	}

	// Delegate runner for expert sub-agent execution (Phase 2)
	var delegateRunner tools.DelegateRunner
	if cfg.Expert.Enabled && expertRegistry != nil {
		delegateRunner = expert.NewDelegateService(cfg, llm, expertRegistry)
	}

	// Session manager with JSONL persistence
	sessMgr := session.New(adksession.InMemoryService(), cfg.Session.HistoryPath, cfg.Session.MaxTurns)

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

	// Create memory search adapter
	searchAdapter := &memorySearchAdapter{svc: memSvc}

	// Agent with memory service, search, and SOP templates
	a, err := agent.New(llm, cfg, memSvc, searchAdapter, sopContent, expertRegistry, delegateRunner)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	log.Printf("[cli] starting TUI with model %s/%s (%d plugins, sessions: %s)",
		cfg.Model.Provider, cfg.Model.Name, len(adkPlugins), cfg.Session.HistoryPath)
	return runTUI(ctx, a, cfg, runner.PluginConfig{Plugins: adkPlugins}, sessMgr)
}