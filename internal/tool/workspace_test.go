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

func TestWriteFileRejectsUnknownModeWithoutChangingFile(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, "notes.txt")
	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := workspaceTestDeps(workspace)

	_, err := writeFile(nil, deps, fileWriteInput{Path: "notes.txt", Content: "replacement", Mode: "replace"})
	if err == nil || !strings.Contains(err.Error(), "unsupported write mode") {
		t.Fatalf("write error = %v", err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "original" {
		t.Fatalf("file content = %q, want original", data)
	}
}

func TestWriteFileEnforcesFinalAppendSize(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, "notes.txt")
	if err := os.WriteFile(path, []byte("1234"), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := workspaceTestDeps(workspace)
	deps.Config.Tools.Write.MaxSize = 5

	_, err := writeFile(nil, deps, fileWriteInput{Path: "notes.txt", Content: "56", Mode: "append"})
	if err == nil || !strings.Contains(err.Error(), "appended file too large") {
		t.Fatalf("write error = %v", err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "1234" {
		t.Fatalf("file content = %q, want unchanged", data)
	}
}

func TestPatchFileEnforcesReadAndWriteLimits(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, "notes.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := workspaceTestDeps(workspace)

	deps.Config.Tools.Read.MaxSize = 4
	if _, err := patchFile(nil, deps, filePatchInput{Path: "notes.txt", OldString: "hello", NewString: "hi"}); err == nil || !strings.Contains(err.Error(), "file too large") {
		t.Fatalf("read limit error = %v", err)
	}

	deps.Config.Tools.Read.MaxSize = 10
	deps.Config.Tools.Write.MaxSize = 5
	if _, err := patchFile(nil, deps, filePatchInput{Path: "notes.txt", OldString: "hello", NewString: "too large"}); err == nil || !strings.Contains(err.Error(), "patched file too large") {
		t.Fatalf("write limit error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("file content = %q, want unchanged", data)
	}
}
