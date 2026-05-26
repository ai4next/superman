package config

import "time"

// Config is the top-level configuration for the Superman agent.
type Config struct {
	Workspace string         `mapstructure:"workspace"`
	Model     ModelConfig    `mapstructure:"model"`
	Server    ServerConfig   `mapstructure:"server"`
	Tools     ToolsConfig    `mapstructure:"tools"`
	Memory    MemoryConfig   `mapstructure:"memory"`
	Plugins   []PluginConfig `mapstructure:"plugins"`
	Skills    SkillsConfig   `mapstructure:"skills"`
	MCP       MCPConfig      `mapstructure:"mcp"`
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
	CodeRun CodeRunConfig `mapstructure:"code_run"`
	Read    ReadConfig    `mapstructure:"read"`
	Write   WriteConfig   `mapstructure:"write"`
	Patch   PatchConfig   `mapstructure:"patch"`
	AskUser AskUserConfig `mapstructure:"ask_user"`
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

type SkillsConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	Paths   []string `mapstructure:"paths"`
}

type MCPConfig struct {
	Servers []MCPServerConfig `mapstructure:"servers"`
}

type MCPServerConfig struct {
	Name                 string   `mapstructure:"name"`
	Enabled              bool     `mapstructure:"enabled"`
	Command              string   `mapstructure:"command"`
	Args                 []string `mapstructure:"args"`
	Tools                []string `mapstructure:"tools"`
	RequiresConfirmation bool     `mapstructure:"requires_confirmation"`
}

// SessionConfig configures session management.
type SessionConfig struct {
	AppName       string              `mapstructure:"app_name"`
	MaxTurns      int                 `mapstructure:"max_turns"`
	LoopDetection LoopDetectionConfig `mapstructure:"loop_detection"`

	ArchiveInterval Duration `mapstructure:"archive_interval"`
	SessionTTL      Duration `mapstructure:"session_ttl"`
}

type LoopDetectionConfig struct {
	Enabled    bool `mapstructure:"enabled"`
	WindowSize int  `mapstructure:"window_size"`
	MaxRepeats int  `mapstructure:"max_repeats"`
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
