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

	v.SetEnvPrefix("SM")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

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

// applyDefaults fills in sensible defaults for any zero-value fields.
func applyDefaults(cfg *Config) {
	if cfg.Dir == "" {
		cfg.Dir = os.ExpandEnv("$HOME/.sm")
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
	if cfg.Tools.CodeRun.Workspace == "" {
		cfg.Tools.CodeRun.Workspace = "./workspace"
	}
	if cfg.Tools.FileRead.MaxSize == 0 {
		cfg.Tools.FileRead.MaxSize = 10_485_760 // 10MB
	}
	if cfg.Tools.FileWrite.MaxSize == 0 {
		cfg.Tools.FileWrite.MaxSize = 10_485_760
	}
	if cfg.Tools.WebScan.Timeout == 0 {
		cfg.Tools.WebScan.Timeout = Duration(15 * time.Second)
	}
	if cfg.Memory.L0.SOPDir == "" {
		cfg.Memory.L0.SOPDir = "./internal/memory/templates"
	}
	if cfg.Memory.L1.MaxEntries == 0 {
		cfg.Memory.L1.MaxEntries = 50
	}
	if cfg.Memory.L3.ArchiveInterval == 0 {
		cfg.Memory.L3.ArchiveInterval = Duration(24 * time.Hour)
	}
	if cfg.Session.MaxTurns == 0 {
		cfg.Session.MaxTurns = 75
	}
	if cfg.Session.HistoryPath == "" {
		cfg.Session.HistoryPath = "./data/sessions"
	}
	if cfg.Session.AppName == "" {
		cfg.Session.AppName = "superman"
	}
	if cfg.Reflect.Autonomous.IdleTimeout == 0 {
		cfg.Reflect.Autonomous.IdleTimeout = Duration(30 * time.Minute)
	}
	if cfg.Reflect.Scheduler.TasksDir == "" {
		cfg.Reflect.Scheduler.TasksDir = "./config/tasks"
	}
}