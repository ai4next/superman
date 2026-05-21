package config

import "time"

// Config is the top-level configuration for the Superman agent.
type Config struct {
	Model    ModelConfig    `mapstructure:"model"`
	Server   ServerConfig   `mapstructure:"server"`
	Tools    ToolsConfig    `mapstructure:"tools"`
	Memory   MemoryConfig   `mapstructure:"memory"`
	Plugins  []PluginConfig `mapstructure:"plugins"`
	Session  SessionConfig  `mapstructure:"session"`
	Reflect  ReflectConfig  `mapstructure:"reflect"`
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
	CodeRun        CodeRunConfig        `mapstructure:"code_run"`
	FileRead       FileReadConfig       `mapstructure:"file_read"`
	FileWrite      FileWriteConfig      `mapstructure:"file_write"`
	FilePatch      FilePatchConfig      `mapstructure:"file_patch"`
	WebScan        WebScanConfig        `mapstructure:"web_scan"`
	WebExecute     WebExecuteConfig     `mapstructure:"web_execute"`
	AskUser        AskUserConfig        `mapstructure:"ask_user"`
	Checkpoint     CheckpointConfig     `mapstructure:"checkpoint"`
	LongTermMemory LongTermMemoryConfig `mapstructure:"long_term_memory"`
}

// CodeRunConfig allows executing code snippets.
type CodeRunConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	Timeout          Duration `mapstructure:"timeout"`
	AllowedLanguages []string `mapstructure:"allowed_languages"`
	Workspace        string   `mapstructure:"workspace"`
}

// FileReadConfig allows reading local files.
type FileReadConfig struct {
	Enabled      bool     `mapstructure:"enabled"`
	MaxSize      int64    `mapstructure:"max_size"`
	AllowedPaths []string `mapstructure:"allowed_paths"`
}

// FileWriteConfig allows writing local files.
type FileWriteConfig struct {
	Enabled      bool     `mapstructure:"enabled"`
	MaxSize      int64    `mapstructure:"max_size"`
	AllowedPaths []string `mapstructure:"allowed_paths"`
}

// FilePatchConfig allows patching local files.
type FilePatchConfig struct {
	Enabled      bool     `mapstructure:"enabled"`
	AllowedPaths []string `mapstructure:"allowed_paths"`
}

// WebScanConfig allows scraping web pages.
type WebScanConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	Timeout Duration `mapstructure:"timeout"`
}

// WebExecuteConfig allows automated browser actions.
type WebExecuteConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// AskUserConfig allows asking the user for input.
type AskUserConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// CheckpointConfig enables checkpoint snapshots.
type CheckpointConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// LongTermMemoryConfig enables long-term memory storage.
type LongTermMemoryConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// MemoryConfig configures the memory subsystem.
type MemoryConfig struct {
	L0 L0Config `mapstructure:"l0"`
	L1 L1Config `mapstructure:"l1"`
	L3 L3Config `mapstructure:"l3"`
}

// L0Config is for the Standard Operating Procedures memory level.
type L0Config struct {
	SOPDir string `mapstructure:"sop_dir"`
}

// L1Config is for short-term agent memory.
type L1Config struct {
	MaxEntries int `mapstructure:"max_entries"`
}

// L3Config is for long-term archived memory.
type L3Config struct {
	ArchiveInterval Duration `mapstructure:"archive_interval"`
}

// PluginConfig configures an individual plugin.
type PluginConfig struct {
	Name    string                 `mapstructure:"name"`
	Enabled bool                   `mapstructure:"enabled"`
	Config  map[string]interface{} `mapstructure:"config"`
}

// SessionConfig configures session management.
type SessionConfig struct {
	AppName     string `mapstructure:"app_name"`
	MaxTurns    int    `mapstructure:"max_turns"`
	HistoryPath string `mapstructure:"history_path"`
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