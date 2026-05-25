package tui

import (
	"context"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"

	"github.com/ai4next/superman/internal/config"
	tuiapp "github.com/ai4next/superman/internal/tui/app"
)

func New(a agent.Agent, cfg *config.Config, pluginCfg runner.PluginConfig, sessSvc session.Service) *tuiapp.Model {
	return tuiapp.New(a, cfg, pluginCfg, sessSvc)
}

func Run(ctx context.Context, a agent.Agent, cfg *config.Config, pluginCfg runner.PluginConfig, sessSvc session.Service) error {
	return tuiapp.Run(ctx, a, cfg, pluginCfg, sessSvc)
}
