package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ai4next/superman/internal/config"
)

func workspacePath(cfg *config.Config, path string, allowMissing bool) (string, error) {
	if cfg == nil || strings.TrimSpace(cfg.Workspace) == "" {
		return "", fmt.Errorf("workspace is required")
	}
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path is required")
	}

	workspace, err := filepath.Abs(cfg.Workspace)
	if err != nil {
		return "", fmt.Errorf("resolve workspace: %w", err)
	}
	if allowMissing {
		if err := os.MkdirAll(workspace, 0o755); err != nil {
			return "", fmt.Errorf("create workspace: %w", err)
		}
	}
	workspace, err = filepath.EvalSymlinks(workspace)
	if err != nil {
		return "", fmt.Errorf("resolve workspace: %w", err)
	}

	candidate := path
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(workspace, candidate)
	}
	candidate = filepath.Clean(candidate)
	if !isWithinWorkspace(workspace, candidate) {
		return "", fmt.Errorf("path %q is outside workspace", path)
	}

	if !allowMissing {
		resolved, err := filepath.EvalSymlinks(candidate)
		if err != nil {
			return "", err
		}
		if !isWithinWorkspace(workspace, resolved) {
			return "", fmt.Errorf("path %q resolves outside workspace", path)
		}
		return resolved, nil
	}

	if resolved, err := filepath.EvalSymlinks(candidate); err == nil {
		if !isWithinWorkspace(workspace, resolved) {
			return "", fmt.Errorf("path %q resolves outside workspace", path)
		}
		return resolved, nil
	}
	parent, err := nearestExistingParent(filepath.Dir(candidate))
	if err != nil {
		return "", err
	}
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	if !isWithinWorkspace(workspace, resolvedParent) {
		return "", fmt.Errorf("path %q resolves outside workspace", path)
	}
	return candidate, nil
}

func nearestExistingParent(path string) (string, error) {
	for {
		if _, err := os.Lstat(path); err == nil {
			return path, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", fmt.Errorf("no existing parent for %q", path)
		}
		path = parent
	}
}

func isWithinWorkspace(workspace, path string) bool {
	rel, err := filepath.Rel(workspace, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
