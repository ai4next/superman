package config

import (
	"errors"
	"fmt"
	"strings"
)

// Validate rejects configuration values that would otherwise fail later in a
// model run or silently fall back to a different safety limit.
func Validate(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}

	var errs []error
	requireText := func(path, value string) {
		if strings.TrimSpace(value) == "" {
			errs = append(errs, fmt.Errorf("%s is required", path))
		}
	}
	requirePositive := func(path string, value int64) {
		if value <= 0 {
			errs = append(errs, fmt.Errorf("%s must be greater than zero", path))
		}
	}

	requireText("workspace", cfg.Workspace)
	requireText("model.provider", cfg.Model.Provider)
	requireText("model.name", cfg.Model.Name)
	requireText("server.addr", cfg.Server.Addr)
	requirePositive("tools.exec.timeout", int64(cfg.Tools.Exec.Timeout))
	requirePositive("tools.exec.max_output_size", cfg.Tools.Exec.MaxOutputSize)
	requirePositive("tools.read.max_size", cfg.Tools.Read.MaxSize)
	requirePositive("tools.write.max_size", cfg.Tools.Write.MaxSize)
	requirePositive("memory.l1.max_index_items", int64(cfg.Memory.L1.MaxIndexItems))
	requirePositive("memory.l1.max_sections", int64(cfg.Memory.L1.MaxSections))
	requirePositive("memory.l2.max_index_items", int64(cfg.Memory.L2.MaxIndexItems))
	requirePositive("memory.search.max_results", int64(cfg.Memory.Search.MaxResults))
	requireText("session.app_name", cfg.Session.AppName)
	requirePositive("session.max_turns", int64(cfg.Session.MaxTurns))
	requirePositive("session.archive_interval", int64(cfg.Session.ArchiveInterval))
	requirePositive("session.session_ttl", int64(cfg.Session.SessionTTL))
	if cfg.Session.LoopDetection.Enabled {
		if cfg.Session.LoopDetection.WindowSize < 2 {
			errs = append(errs, fmt.Errorf("session.loop_detection.window_size must be at least 2"))
		}
		if cfg.Session.LoopDetection.MaxRepeats < 2 {
			errs = append(errs, fmt.Errorf("session.loop_detection.max_repeats must be at least 2"))
		}
	}
	requirePositive("reflect.autonomous.idle_timeout", int64(cfg.Reflect.Autonomous.IdleTimeout))
	requireText("reflect.scheduler.tasks_dir", cfg.Reflect.Scheduler.TasksDir)
	requirePositive("expert.max_count", int64(cfg.Expert.MaxCount))
	requirePositive("bus.queue.max_size", int64(cfg.Bus.Queue.MaxSize))

	for i, server := range cfg.MCP.Servers {
		if server.Enabled && strings.TrimSpace(server.Command) == "" {
			errs = append(errs, fmt.Errorf("mcp.servers[%d].command is required when enabled", i))
		}
	}
	for i, platform := range cfg.IM.Platforms {
		if platform.Enabled && strings.TrimSpace(platform.Name) == "" {
			errs = append(errs, fmt.Errorf("im.platforms[%d].name is required when enabled", i))
		}
	}

	return errors.Join(errs...)
}
