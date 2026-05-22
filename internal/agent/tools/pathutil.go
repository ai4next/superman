package tools

import (
	"fmt"
	"path/filepath"
	"strings"
)

func isPathAllowed(path string, allowedPaths []string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, allowed := range allowedPaths {
		absAllowed, err := filepath.Abs(allowed)
		if err != nil {
			continue
		}
		if strings.HasPrefix(abs, absAllowed) {
			return true
		}
	}
	return false
}

func validatePath(path string, allowedPaths []string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	if !isPathAllowed(abs, allowedPaths) {
		return "", fmt.Errorf("path %s is outside allowed directories", abs)
	}
	return abs, nil
}
