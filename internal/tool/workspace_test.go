package tool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ai4next/superman/internal/config"
)

func workspaceTestDeps(workspace string) Dependencies {
	return Dependencies{Config: &config.Config{
		Workspace: workspace,
		Tools: config.ToolsConfig{
			Read:  config.ReadConfig{MaxSize: 1024 * 1024},
			Write: config.WriteConfig{MaxSize: 1024 * 1024},
		},
	}}
}

func TestFileToolsResolveRelativePathsInWorkspace(t *testing.T) {
	workspace := t.TempDir()
	deps := workspaceTestDeps(workspace)
	if _, err := writeFile(nil, deps, fileWriteInput{Path: "notes/result.txt", Content: "done"}); err != nil {
		t.Fatal(err)
	}
	got, err := readFile(nil, deps, fileReadInput{Path: "notes/result.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Content, "done") {
		t.Fatalf("content = %q", got.Content)
	}
}

func TestFileToolsRejectPathsOutsideWorkspace(t *testing.T) {
	workspace := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := workspaceTestDeps(workspace)

	if _, err := readFile(nil, deps, fileReadInput{Path: outside}); err == nil || !strings.Contains(err.Error(), "outside workspace") {
		t.Fatalf("read error = %v", err)
	}
	if _, err := writeFile(nil, deps, fileWriteInput{Path: outside, Content: "changed"}); err == nil || !strings.Contains(err.Error(), "outside workspace") {
		t.Fatalf("write error = %v", err)
	}
	if _, err := patchFile(nil, deps, filePatchInput{Path: outside, OldString: "secret", NewString: "changed"}); err == nil || !strings.Contains(err.Error(), "outside workspace") {
		t.Fatalf("patch error = %v", err)
	}
}

func TestFileToolsRejectSymlinkEscapes(t *testing.T) {
	workspace := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(workspace, "escape.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	deps := workspaceTestDeps(workspace)
	if _, err := readFile(nil, deps, fileReadInput{Path: "escape.txt"}); err == nil || !strings.Contains(err.Error(), "outside workspace") {
		t.Fatalf("read error = %v", err)
	}
	if _, err := writeFile(nil, deps, fileWriteInput{Path: "escape.txt", Content: "changed"}); err == nil || !strings.Contains(err.Error(), "outside workspace") {
		t.Fatalf("write error = %v", err)
	}
}
