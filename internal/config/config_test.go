package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"30s", 30 * time.Second},
		{"5m", 5 * time.Minute},
		{"2h", 2 * time.Hour},
		{"24h", 24 * time.Hour},
		{"1h30m", 90 * time.Minute},
	}
	for _, tt := range tests {
		var d Duration
		if err := d.UnmarshalText([]byte(tt.input)); err != nil {
			t.Errorf("UnmarshalText(%q) = %v", tt.input, err)
			continue
		}
		if d.AsDuration() != tt.expected {
			t.Errorf("UnmarshalText(%q) = %v, want %v", tt.input, d.AsDuration(), tt.expected)
		}
	}
}

func TestDuration_Invalid(t *testing.T) {
	var d Duration
	if err := d.UnmarshalText([]byte("not-a-duration")); err == nil {
		t.Error("expected error for invalid duration")
	}
}

func TestDurationRoundtrip(t *testing.T) {
	original := Duration(5 * time.Minute)
	text, err := original.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText: %v", err)
	}
	var restored Duration
	if err := restored.UnmarshalText(text); err != nil {
		t.Fatalf("UnmarshalText: %v", err)
	}
	if original != restored {
		t.Errorf("roundtrip: %v != %v", original, restored)
	}
}

func TestLoad_Defaults(t *testing.T) {
	// Load with no config file — should use defaults
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() with no file: %v", err)
	}
	if cfg.Model.Provider != "openai" {
		t.Errorf("default provider = %q, want openai", cfg.Model.Provider)
	}
	if cfg.Model.Name != "gpt-4o" {
		t.Errorf("default model name = %q, want gpt-4o", cfg.Model.Name)
	}
	if cfg.Tools.CodeRun.Timeout.AsDuration() != 30*time.Second {
		t.Errorf("default code_run timeout = %v, want 30s", cfg.Tools.CodeRun.Timeout.AsDuration())
	}
	if cfg.Tools.FileRead.MaxSize != 10_485_760 {
		t.Errorf("default file_read max_size = %d, want 10MB", cfg.Tools.FileRead.MaxSize)
	}
	if cfg.Tools.WebScan.Timeout.AsDuration() != 15*time.Second {
		t.Errorf("default web_scan timeout = %v, want 15s", cfg.Tools.WebScan.Timeout.AsDuration())
	}
	if cfg.Memory.L3.ArchiveInterval.AsDuration() != 24*time.Hour {
		t.Errorf("default archive interval = %v, want 24h", cfg.Memory.L3.ArchiveInterval.AsDuration())
	}
	if cfg.Session.MaxTurns != 75 {
		t.Errorf("default max_turns = %d, want 75", cfg.Session.MaxTurns)
	}
	if cfg.Dir != filepath.Join(home, ".sm") {
		t.Errorf("default dir = %q, want %q", cfg.Dir, filepath.Join(home, ".sm"))
	}
	if cfg.Expert.Dir != filepath.Join(home, ".sm", "superman", "experts") {
		t.Errorf("default expert dir = %q, want %q", cfg.Expert.Dir, filepath.Join(home, ".sm", "superman", "experts"))
	}
	if cfg.Expert.TopK != 2 {
		t.Errorf("default expert top_k = %d, want 2", cfg.Expert.TopK)
	}
	if cfg.Reflect.Autonomous.IdleTimeout.AsDuration() != 30*time.Minute {
		t.Errorf("default idle_timeout = %v, want 30m", cfg.Reflect.Autonomous.IdleTimeout.AsDuration())
	}
}

func TestLoad_FromFile(t *testing.T) {
	dir := t.TempDir()
	yamlContent := []byte(`
model:
  provider: deepseek
  name: deepseek-chat
  base_url: https://api.deepseek.com/v1
  api_key: ${TEST_API_KEY}
tools:
  code_run:
    enabled: true
    timeout: 60s
`)
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, yamlContent, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("TEST_API_KEY", "sk-test123")
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load(%q): %v", configPath, err)
	}
	if cfg.Model.Provider != "deepseek" {
		t.Errorf("provider = %q, want deepseek", cfg.Model.Provider)
	}
	if cfg.Model.Name != "deepseek-chat" {
		t.Errorf("model name = %q, want deepseek-chat", cfg.Model.Name)
	}
	if cfg.Model.BaseURL != "https://api.deepseek.com/v1" {
		t.Errorf("base_url = %q", cfg.Model.BaseURL)
	}
	if cfg.Model.APIKey != "sk-test123" {
		t.Errorf("api_key = %q, want sk-test123", cfg.Model.APIKey)
	}
	if cfg.Tools.CodeRun.Timeout.AsDuration() != 60*time.Second {
		t.Errorf("timeout = %v, want 60s", cfg.Tools.CodeRun.Timeout.AsDuration())
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	// Write a minimal config so Viper knows about the structure
	minConfig := []byte("model:\n  provider: openai\n  name: gpt-4o\nserver:\n  addr: :8080\n")
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, minConfig, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("SUPERMAN_MODEL_PROVIDER", "ollama")
	t.Setenv("SUPERMAN_MODEL_NAME", "qwen3:8b")
	t.Setenv("SUPERMAN_SERVER_ADDR", ":9090")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.Model.Provider != "ollama" {
		t.Errorf("provider = %q, want ollama (from env)", cfg.Model.Provider)
	}
	if cfg.Model.Name != "qwen3:8b" {
		t.Errorf("model name = %q, want qwen3:8b", cfg.Model.Name)
	}
	if cfg.Server.Addr != ":9090" {
		t.Errorf("server addr = %q, want :9090", cfg.Server.Addr)
	}
}
