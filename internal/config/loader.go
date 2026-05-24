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
	if err := v.Unmarshal(&cfg, viper.DecodeHook(stringToDurationHook())); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// Expand ${VAR} references in api_key
	cfg.Model.APIKey = os.ExpandEnv(cfg.Model.APIKey)
	expandPaths(&cfg)

	applyDefaults(&cfg)
	return &cfg, nil
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
	cfg.Tools.WebExecute.UserDataDir = os.ExpandEnv(cfg.Tools.WebExecute.UserDataDir)
	cfg.Tools.WebExecute.BrowserPath = os.ExpandEnv(cfg.Tools.WebExecute.BrowserPath)
	cfg.Tools.BrowserUse.UserDataDir = os.ExpandEnv(cfg.Tools.BrowserUse.UserDataDir)
	cfg.Tools.BrowserUse.BrowserPath = os.ExpandEnv(cfg.Tools.BrowserUse.BrowserPath)
}

// applyDefaults fills in sensible defaults for any zero-value fields.
func applyDefaults(cfg *Config) {
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
	if cfg.Tools.CodeRun.Timeout == 0 {
		cfg.Tools.CodeRun.Timeout = Duration(30 * time.Second)
	}
	if cfg.Tools.Read.MaxSize == 0 {
		cfg.Tools.Read.MaxSize = 10_485_760 // 10MB
	}
	if cfg.Tools.Write.MaxSize == 0 {
		cfg.Tools.Write.MaxSize = 10_485_760
	}
	if cfg.Tools.WebScan.Timeout == 0 {
		cfg.Tools.WebScan.Timeout = Duration(15 * time.Second)
	}
	if cfg.Tools.WebExecute.Timeout == 0 {
		cfg.Tools.WebExecute.Timeout = Duration(15 * time.Second)
	}
	if cfg.Tools.WebExecute.UserDataDir == "" {
		cfg.Tools.WebExecute.UserDataDir = filepath.Join(cfg.Workspace, "chrome-profile")
	}
	if cfg.Tools.BrowserUse.Timeout == 0 {
		cfg.Tools.BrowserUse.Timeout = Duration(15 * time.Second)
	}
	if cfg.Tools.BrowserUse.UserDataDir == "" {
		cfg.Tools.BrowserUse.UserDataDir = filepath.Join(cfg.Workspace, "browser-use-profile")
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
