package cli

import (
	"context"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"

	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/session"
	"github.com/ai4next/superman/internal/tui"
)

func runTUI(ctx context.Context, a agent.Agent, cfg *config.Config, pluginCfg runner.PluginConfig, sessMgr *session.Manager) error {
	return tui.Run(ctx, a, cfg, pluginCfg, sessMgr.Service())
}