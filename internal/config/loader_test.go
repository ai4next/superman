package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaultsEnableSkills(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("workspace: /tmp/superman-test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Skills.Enabled {
		t.Fatalf("Skills.Enabled = false, want default true")
	}
}

func TestLoadPreservesExplicitSkillsDisabled(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("workspace: /tmp/superman-test\nskills:\n  enabled: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Skills.Enabled {
		t.Fatalf("Skills.Enabled = true, want explicit false")
	}
}

func TestLoadDefaultsEnableLoopDetection(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("workspace: /tmp/superman-test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Session.LoopDetection.Enabled {
		t.Fatalf("loop detection disabled, want default enabled")
	}
	if cfg.Session.LoopDetection.WindowSize != 10 || cfg.Session.LoopDetection.MaxRepeats != 5 {
		t.Fatalf("loop detection defaults = %#v", cfg.Session.LoopDetection)
	}
}

func TestLoadPreservesExplicitLoopDetectionDisabled(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("workspace: /tmp/superman-test\nsession:\n  loop_detection:\n    enabled: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Session.LoopDetection.Enabled {
		t.Fatalf("loop detection enabled, want explicit false")
	}
}

func TestLoadNormalizesSkillPathsAndExpandsMCP(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SUPERMAN_TEST_CMD", filepath.Join(tmp, "server"))

	cfgPath := filepath.Join(tmp, "config.yaml")
	data := []byte(`workspace: ` + tmp + `
skills:
  paths:
    - skills
    - /opt/shared-skills
mcp:
  servers:
    - name: fs
      enabled: true
      command: ${SUPERMAN_TEST_CMD}
      args:
        - ${SUPERMAN_TEST_CMD}
`)
	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(tmp, "skills"); cfg.Skills.Paths[0] != want {
		t.Fatalf("relative skill path = %q, want %q", cfg.Skills.Paths[0], want)
	}
	if want := "/opt/shared-skills"; cfg.Skills.Paths[1] != want {
		t.Fatalf("absolute skill path = %q, want %q", cfg.Skills.Paths[1], want)
	}
	if want := filepath.Join(tmp, "server"); cfg.MCP.Servers[0].Command != want {
		t.Fatalf("mcp command = %q, want %q", cfg.MCP.Servers[0].Command, want)
	}
	if want := filepath.Join(tmp, "server"); cfg.MCP.Servers[0].Args[0] != want {
		t.Fatalf("mcp arg = %q, want %q", cfg.MCP.Servers[0].Args[0], want)
	}
}

func TestLoadExpandsIMPlatformOptions(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SUPERMAN_TEST_TOKEN", "expanded-token")

	cfgPath := filepath.Join(tmp, "config.yaml")
	data := []byte(`workspace: ` + tmp + `
im:
  platforms:
    - name: telegram
      enabled: true
      options:
        token: ${SUPERMAN_TEST_TOKEN}
        allow_from: user-1
        group_reply_all: true
`)
	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.IM.Platforms) != 1 {
		t.Fatalf("IM platform count = %d, want 1", len(cfg.IM.Platforms))
	}
	if got := cfg.IM.Platforms[0].Options["token"]; got != "expanded-token" {
		t.Fatalf("token = %#v, want expanded-token", got)
	}
	if got := cfg.IM.Platforms[0].Options["group_reply_all"]; got != true {
		t.Fatalf("group_reply_all = %#v, want true", got)
	}
}

func TestLoadExpandsModelHeaders(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SUPERMAN_TEST_HEADER", "expanded-header")

	cfgPath := filepath.Join(tmp, "config.yaml")
	data := []byte(`workspace: ` + tmp + `
model:
  headers:
    X-Custom-Token: ${SUPERMAN_TEST_HEADER}
    X-Static: static-value
`)
	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := configHeaderValue(cfg.Model.Headers, "X-Custom-Token"); got != "expanded-header" {
		t.Fatalf("X-Custom-Token = %q, want expanded-header", got)
	}
	if got := configHeaderValue(cfg.Model.Headers, "X-Static"); got != "static-value" {
		t.Fatalf("X-Static = %q, want static-value", got)
	}
}

func configHeaderValue(headers map[string]string, key string) string {
	for k, v := range headers {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}
