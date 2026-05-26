package tool

import (
	"fmt"
	"os"
	"path/filepath"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type fileWriteInput struct {
	Path    string `json:"path" jsonschema:"File path"`
	Content string `json:"content" jsonschema:"File content"`
	Mode    string `json:"mode,omitempty" jsonschema:"overwrite or append"`
}

type fileWriteOutput struct {
	FilePath string `json:"file_path"`
	Bytes    int    `json:"bytes_written"`
	Mode     string `json:"mode"`
}

func newWriteTool(deps Dependencies) tool.Tool {
	handler := func(tctx tool.Context, input fileWriteInput) (fileWriteOutput, error) {
		return writeFile(tctx, deps, input)
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "write",
		Description: "Write a file.",
	}, handler)
	return t
}

func writeFile(tctx tool.Context, deps Dependencies, input fileWriteInput) (fileWriteOutput, error) {
	abs, err := filepath.Abs(input.Path)
	if err != nil {
		return fileWriteOutput{}, fmt.Errorf("invalid path: %w", err)
	}

	if len(input.Content) > int(deps.Config.Tools.Write.MaxSize) {
		return fileWriteOutput{}, fmt.Errorf("content too large: %d bytes (max %d)", len(input.Content), deps.Config.Tools.Write.MaxSize)
	}

	mode := input.Mode
	if mode == "" {
		mode = "overwrite"
	}

	beforeBytes, readErr := os.ReadFile(abs)
	beforeMissing := os.IsNotExist(readErr)
	if readErr != nil && !beforeMissing {
		return fileWriteOutput{}, fmt.Errorf("read existing file failed: %w", readErr)
	}
	before := string(beforeBytes)

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
	after := input.Content
	if mode == "append" && !beforeMissing {
		after = before + input.Content
	}

	out := fileWriteOutput{
		FilePath: abs,
		Bytes:    n,
		Mode:     mode,
	}
	recordFileRevision(tctx, abs, mode, before, after, beforeMissing)
	return out, nil
}
