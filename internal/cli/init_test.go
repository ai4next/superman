package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteConfigTemplate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	template := "workspace: ${HOME}/.sm\n"

	if err := writeConfigTemplate(path, template); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != template {
		t.Fatalf("config content = %q, want %q", string(got), template)
	}
	if err := writeConfigTemplate(path, template); err == nil {
		t.Fatal("existing config should not be overwritten")
	}
}

func TestWriteConfigTemplateCreatesParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".sm", "config.yaml")

	if err := writeConfigTemplate(path, "workspace: ${HOME}/.sm\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
