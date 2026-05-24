package config

import (
	"path/filepath"
	"time"
)

// Config is the top-level configuration for the Superman agent.
type Config struct {
	Workspace string         `mapstructure:"workspace"`
	Model     ModelConfig    `mapstructure:"model"`
	Server    ServerConfig   `mapstructure:"server"`
	Tools     ToolsConfig    `mapstructure:"tools"`
	Memory    MemoryConfig   `mapstructure:"memory"`
	Plugins   []PluginConfig `mapstructure:"plugins"`
	Session   SessionConfig  `mapstructure:"session"`
	Reflect   ReflectConfig  `mapstructure:"reflect"`
	Expert    ExpertConfig   `mapstructure:"expert"`
}

// ModelConfig configures the LLM provider.
type ModelConfig struct {
	Provider string `mapstructure:"provider"`
	Name     string `mapstructure:"name"`
	BaseURL  string `mapstructure:"base_url"`
	APIKey   string `mapstructure:"api_key"`
}

// ServerConfig configures the HTTP server.
type ServerConfig struct {
	Addr string `mapstructure:"addr"`
}

// ToolsConfig contains sub-configs for each tool.
type ToolsConfig struct {
	CodeRun    CodeRunConfig    `mapstructure:"code_run"`
	Read       ReadConfig       `mapstructure:"read"`
	Write      WriteConfig      `mapstructure:"write"`
	Patch      PatchConfig      `mapstructure:"patch"`
	WebScan    WebScanConfig    `mapstructure:"web_scan"`
	WebExecute WebExecuteConfig `mapstructure:"web_execute"`
	BrowserUse BrowserUseConfig `mapstructure:"browser_use"`
	AskUser    AskUserConfig    `mapstructure:"ask_user"`
}

// CodeRunConfig allows executing code snippets.
type CodeRunConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	Timeout          Duration `mapstructure:"timeout"`
	AllowedLanguages []string `mapstructure:"allowed_languages"`
}

// ReadConfig allows reading local files.
type ReadConfig struct {
	Enabled bool  `mapstructure:"enabled"`
	MaxSize int64 `mapstructure:"max_size"`
}

// WriteConfig allows writing local files.
type WriteConfig struct {
	Enabled bool  `mapstructure:"enabled"`
	MaxSize int64 `mapstructure:"max_size"`
}

// PatchConfig allows patching local files.
type PatchConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// WebScanConfig allows scraping web pages.
type WebScanConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	Timeout Duration `mapstructure:"timeout"`
}

// WebExecuteConfig allows automated browser actions.
type WebExecuteConfig struct {
	Enabled            bool     `mapstructure:"enabled"`
	Timeout            Duration `mapstructure:"timeout"`
	Headless           bool     `mapstructure:"headless"`
	UserDataDir        string   `mapstructure:"user_data_dir"`
	BrowserPath        string   `mapstructure:"browser_path"`
	RemoteDebuggingURL string   `mapstructure:"remote_debugging_url"`
}

// BrowserUseConfig enables high-level browser actions.
type BrowserUseConfig struct {
	Enabled           bool     `mapstructure:"enabled"`
	Timeout           Duration `mapstructure:"timeout"`
	Headless          bool     `mapstructure:"headless"`
	DisableSecurity   bool     `mapstructure:"disable_security"`
	ExtraChromiumArgs []string `mapstructure:"extra_chromium_args"`
	UserDataDir       string   `mapstructure:"user_data_dir"`
	BrowserPath       string   `mapstructure:"browser_path"`
	ProxyServer       string   `mapstructure:"proxy_server"`
}

// AskUserConfig allows asking the user for input.
type AskUserConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// MemoryConfig configures the memory subsystem.
type MemoryConfig struct {
	L0 L0Config `mapstructure:"l0"`
	L1 L1Config `mapstructure:"l1"`
	L2 L2Config `mapstructure:"l2"`
}

// L0Config configures the runtime L0 index.
type L0Config struct{}

// L1Config configures TOML facts memory.
type L1Config struct {
	MaxIndexItems int `mapstructure:"max_index_items"`
	MaxSections   int `mapstructure:"max_sections"`
}

// L2Config configures the SOP directory.
type L2Config struct {
	MaxIndexItems int `mapstructure:"max_index_items"`
}

// PluginConfig configures an individual plugin.
type PluginConfig struct {
	Name    string                 `mapstructure:"name"`
	Enabled bool                   `mapstructure:"enabled"`
	Config  map[string]interface{} `mapstructure:"config"`
}

// SessionConfig configures session management.
type SessionConfig struct {
	AppName         string   `mapstructure:"app_name"`
	MaxTurns        int      `mapstructure:"max_turns"`
	ArchiveInterval Duration `mapstructure:"archive_interval"`
	SessionTTL      Duration `mapstructure:"session_ttl"`
}

// ReflectConfig configures the reflection subsystem.
type ReflectConfig struct {
	Autonomous AutonomousConfig `mapstructure:"autonomous"`
	Scheduler  SchedulerConfig  `mapstructure:"scheduler"`
}

// AutonomousConfig configures autonomous reflection.
type AutonomousConfig struct {
	IdleTimeout Duration `mapstructure:"idle_timeout"`
}

// SchedulerConfig configures scheduled reflection tasks.
type SchedulerConfig struct {
	TasksDir string `mapstructure:"tasks_dir"`
}

// ExpertConfig configures the expert subsystem.
type ExpertConfig struct {
	Enabled  bool `mapstructure:"enabled"`
	MaxCount int  `mapstructure:"max_count"`
}

// ExpertDir returns the workspace-scoped expert directory.
func (c *Config) ExpertDir() string {
	if c == nil {
		return ""
	}
	return filepath.Join(c.Workspace, "experts")
}

// ExpertMemoryDir returns the workspace-scoped memory directory for an expert.
func (c *Config) ExpertMemoryDir(name string) string {
	if c == nil || name == "" {
		return ""
	}
	return filepath.Join(c.ExpertDir(), name, "memory")
}

// Duration unmarshals YAML duration strings like "30s", "2h", "24h".
type Duration time.Duration

// UnmarshalText parses a duration string into a Duration.
func (d *Duration) UnmarshalText(text []byte) error {
	parsed, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}

// MarshalText converts a Duration to its string representation.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

// AsDuration returns the underlying time.Duration.
func (d Duration) AsDuration() time.Duration {
	return time.Duration(d)
}
