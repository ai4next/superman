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

func TestLoadExecOutputLimit(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(cfgPath, []byte("workspace: /tmp/superman-test\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(cfgPath)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Tools.Exec.MaxOutputSize != 1_048_576 {
			t.Fatalf("max output size = %d, want 1048576", cfg.Tools.Exec.MaxOutputSize)
		}
	})

	t.Run("configured", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.yaml")
		data := []byte("workspace: /tmp/superman-test\ntools:\n  exec:\n    max_output_size: 2048\n")
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(cfgPath)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Tools.Exec.MaxOutputSize != 2048 {
			t.Fatalf("max output size = %d, want 2048", cfg.Tools.Exec.MaxOutputSize)
		}
	})
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

func TestLoadDefaultsBusAndExpertConfig(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("workspace: "+tmp+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Expert.MaxCount != 10 {
		t.Fatalf("expert max count = %d, want 10", cfg.Expert.MaxCount)
	}
	if cfg.Bus.Path != "" {
		t.Fatalf("bus path = %q, want empty for channel queue", cfg.Bus.Path)
	}
	if cfg.Bus.AuditLog != filepath.Join(tmp, "bus", "events.jsonl") {
		t.Fatalf("bus audit log = %q", cfg.Bus.AuditLog)
	}
	if cfg.Bus.Queue.MaxSize != 100 {
		t.Fatalf("bus queue defaults = %#v", cfg.Bus.Queue)
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

func TestLoadRejectsInvalidSafetyLimits(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "exec timeout",
			yaml: "tools:\n  exec:\n    timeout: -1s\n",
			want: "tools.exec.timeout",
		},
		{
			name: "exec output",
			yaml: "tools:\n  exec:\n    max_output_size: -1\n",
			want: "tools.exec.max_output_size",
		},
		{
			name: "file read",
			yaml: "tools:\n  read:\n    max_size: -1\n",
			want: "tools.read.max_size",
		},
		{
			name: "loop window",
			yaml: "session:\n  loop_detection:\n    enabled: true\n    window_size: 1\n",
			want: "session.loop_detection.window_size",
		},
		{
			name: "enabled MCP",
			yaml: "mcp:\n  servers:\n    - name: broken\n      enabled: true\n",
			want: "mcp.servers[0].command",
		},
		{
			name: "enabled IM platform",
			yaml: "im:\n  platforms:\n    - enabled: true\n",
			want: "im.platforms[0].name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfgPath := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(cfgPath, []byte(tc.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := Load(cfgPath)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Load() error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestValidateReportsAllInvalidFields(t *testing.T) {
	err := Validate(&Config{})
	if err == nil {
		t.Fatal("Validate() error = nil")
	}
	for _, want := range []string{"workspace", "model.provider", "tools.exec.timeout", "session.max_turns"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate() error = %q, want %q", err, want)
		}
	}
}
