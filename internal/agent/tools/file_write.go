package tools

import (
	"fmt"
	"os"
	"path/filepath"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type fileWriteInput struct {
	Path    string `json:"path" jsonschema:"Path to the file to write"`
	Content string `json:"content" jsonschema:"Content to write to the file"`
	Mode    string `json:"mode,omitempty" jsonschema:"Write mode: overwrite (default) or append"`
}

type fileWriteOutput struct {
	FilePath string `json:"file_path"`
	Bytes    int    `json:"bytes_written"`
	Mode     string `json:"mode"`
}

func newFileWriteTool(deps Dependencies) tool.Tool {
	handler := func(tctx tool.Context, input fileWriteInput) (fileWriteOutput, error) {
		return writeFile(deps, input)
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "file_write",
		Description: "Create, overwrite, or append to a file",
	}, handler)
	return t
}

func writeFile(deps Dependencies, input fileWriteInput) (fileWriteOutput, error) {
	abs, err := validatePath(input.Path, deps.Config.Tools.FileWrite.AllowedPaths)
	if err != nil {
		return fileWriteOutput{}, err
	}

	if len(input.Content) > int(deps.Config.Tools.FileWrite.MaxSize) {
		return fileWriteOutput{}, fmt.Errorf("content too large: %d bytes (max %d)", len(input.Content), deps.Config.Tools.FileWrite.MaxSize)
	}

	mode := input.Mode
	if mode == "" {
		mode = "overwrite"
	}

	dir := filepath.Dir(abs)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fileWriteOutput{}, fmt.Errorf("create directory failed: %w", err)
	}

	var flag int
	if mode == "append" {
		flag = os.O_APPEND | os.O_CREATE | os.O_WRONLY
	} else {
		flag = os.O_TRUNC | os.O_CREATE | os.O_WRONLY
	}

	f, err := os.OpenFile(abs, flag, 0644)
	if err != nil {
		return fileWriteOutput{}, fmt.Errorf("open file failed: %w", err)
	}
	defer f.Close()

	n, err := f.WriteString(input.Content)
	if err != nil {
		return fileWriteOutput{}, fmt.Errorf("write failed: %w", err)
	}

	return fileWriteOutput{
		FilePath: abs,
		Bytes:    n,
		Mode:     mode,
	}, nil
}
