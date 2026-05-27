package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

// Load reads the config file at configPath (or searches default locations)
// and unmarshals it into a Config struct with defaults applied.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		if home, _ := os.UserHomeDir(); home != "" {
			v.AddConfigPath(filepath.Join(home, ".sm"))
		}
	}

	v.SetEnvPrefix("SUPERMAN")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	v.SetDefault("session.archive_interval", Duration(6*time.Hour))
	v.SetDefault("session.session_ttl", Duration(48*time.Hour))

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	skillsEnabledSet := v.IsSet("skills.enabled")
	loopDetectionEnabledSet := v.IsSet("session.loop_detection.enabled")

	if err := v.Unmarshal(&cfg, viper.DecodeHook(stringToDurationHook())); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// Expand ${VAR} references in api_key
	cfg.Model.APIKey = os.ExpandEnv(cfg.Model.APIKey)
	expandPaths(&cfg)
	expandIMOptions(&cfg)

	applyDefaults(&cfg, skillsEnabledSet, loopDetectionEnabledSet)
	normalizePaths(&cfg)
	return &cfg, nil
}

func expandIMOptions(cfg *Config) {
	for i := range cfg.IM.Platforms {
		for key, value := range cfg.IM.Platforms[i].Options {
			if s, ok := value.(string); ok {
				cfg.IM.Platforms[i].Options[key] = os.ExpandEnv(s)
			}
		}
	}
}

// stringToDurationHook returns a mapstructure DecodeHookFunc that converts
// string values (like "30s", "5m") to config.Duration.
func stringToDurationHook() mapstructure.DecodeHookFunc {
	return func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
		if f.Kind() != reflect.String {
			return data, nil
		}
		if t != reflect.TypeOf(Duration(0)) {
			return data, nil
		}
		d, err := time.ParseDuration(data.(string))
		if err != nil {
			return nil, err
		}
		return Duration(d), nil
	}
}

func expandPaths(cfg *Config) {
	cfg.Workspace = os.ExpandEnv(cfg.Workspace)
	cfg.Reflect.Scheduler.TasksDir = os.ExpandEnv(cfg.Reflect.Scheduler.TasksDir)
	for i, path := range cfg.Skills.Paths {
		cfg.Skills.Paths[i] = os.ExpandEnv(path)
	}
	for i := range cfg.MCP.Servers {
		cfg.MCP.Servers[i].Command = os.ExpandEnv(cfg.MCP.Servers[i].Command)
		for j, arg := range cfg.MCP.Servers[i].Args {
			cfg.MCP.Servers[i].Args[j] = os.ExpandEnv(arg)
		}
	}
}

func normalizePaths(cfg *Config) {
	for i, path := range cfg.Skills.Paths {
		if path == "" || filepath.IsAbs(path) {
			continue
		}
		cfg.Skills.Paths[i] = filepath.Join(cfg.Workspace, path)
	}
}

// applyDefaults fills in sensible defaults for any zero-value fields.
func applyDefaults(cfg *Config, skillsEnabledSet bool, loopDetectionEnabledSet bool) {
	if cfg.Workspace == "" {
		cfg.Workspace = os.ExpandEnv("$HOME/.sm")
	}
	if cfg.Model.Provider == "" {
		cfg.Model.Provider = "openai"
	}
	if cfg.Model.Name == "" {
		cfg.Model.Name = "gpt-4o"
	}
	if cfg.Server.Addr == "" {
		cfg.Server.Addr = "127.0.0.1:8080"
	}
	if !skillsEnabledSet {
		cfg.Skills.Enabled = true
	}
	if cfg.Tools.CodeRun.Timeout == 0 {
		cfg.Tools.CodeRun.Timeout = Duration(30 * time.Second)
	}
	if cfg.Tools.Read.MaxSize == 0 {
		cfg.Tools.Read.MaxSize = 10_485_760 // 10MB
	}
	if cfg.Tools.Write.MaxSize == 0 {
		cfg.Tools.Write.MaxSize = 10_485_760
	}
	if cfg.Memory.L1.MaxIndexItems == 0 {
		cfg.Memory.L1.MaxIndexItems = 50
	}
	if cfg.Memory.L1.MaxSections == 0 {
		cfg.Memory.L1.MaxSections = 100
	}
	if cfg.Memory.L2.MaxIndexItems == 0 {
		cfg.Memory.L2.MaxIndexItems = 50
	}
	if cfg.Session.MaxTurns == 0 {
		cfg.Session.MaxTurns = 75
	}
	if cfg.Session.LoopDetection.WindowSize == 0 {
		cfg.Session.LoopDetection.WindowSize = 10
	}
	if cfg.Session.LoopDetection.MaxRepeats == 0 {
		cfg.Session.LoopDetection.MaxRepeats = 5
	}
	if !loopDetectionEnabledSet {
		cfg.Session.LoopDetection.Enabled = true
	}
	if cfg.Session.AppName == "" {
		cfg.Session.AppName = "superman"
	}
	if cfg.Session.ArchiveInterval == 0 {
		cfg.Session.ArchiveInterval = Duration(6 * time.Hour)
	}
	if cfg.Session.SessionTTL == 0 {
		cfg.Session.SessionTTL = Duration(48 * time.Hour)
	}
	if cfg.Expert.MaxCount == 0 {
		cfg.Expert.MaxCount = 10
	}
	if cfg.Reflect.Autonomous.IdleTimeout == 0 {
		cfg.Reflect.Autonomous.IdleTimeout = Duration(30 * time.Minute)
	}
	if cfg.Reflect.Scheduler.TasksDir == "" {
		cfg.Reflect.Scheduler.TasksDir = "./config/tasks"
	}
}
