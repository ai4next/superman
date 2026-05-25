package app

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"

	"github.com/ai4next/superman/internal/config"
)

func Run(ctx context.Context, a agent.Agent, cfg *config.Config, pluginCfg runner.PluginConfig, sessSvc session.Service) error {
	logPath := filepath.Join(cfg.Workspace, "tui.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	} else {
		log.SetOutput(io.Discard)
	}

	m := New(a, cfg, pluginCfg, sessSvc)
	p := tea.NewProgram(m, tea.WithContext(ctx))
	_, err = p.Run()
	return err
}
