package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type fileWriteInput struct {
	Path    string `json:"path" jsonschema:"File path"`
	Content string `json:"content" jsonschema:"File content"`
	Mode    string `json:"mode,omitempty" jsonschema:"overwrite or append"`
}

type fileWriteOutput struct {
	Bytes int `json:"bytes_written"`
}

func newWriteTool(deps Dependencies) tool.Tool {
	handler := func(tctx tool.Context, input fileWriteInput) (fileWriteOutput, error) {
		return writeFile(tctx, deps, input)
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "write",
		Description: "Write a file",
	}, handler)
	return t
}

func writeFile(tctx tool.Context, deps Dependencies, input fileWriteInput) (fileWriteOutput, error) {
	if tctx != nil {
		if err := tctx.Err(); err != nil {
			return fileWriteOutput{}, err
		}
	}
	mode := strings.ToLower(strings.TrimSpace(input.Mode))
	if mode == "" {
		mode = "overwrite"
	}
	if mode != "overwrite" && mode != "append" {
		return fileWriteOutput{}, fmt.Errorf("unsupported write mode %q: use overwrite or append", input.Mode)
	}

	abs, err := workspacePath(deps.Config, input.Path, true)
	if err != nil {
		return fileWriteOutput{}, fmt.Errorf("invalid path: %w", err)
	}

	if int64(len(input.Content)) > deps.Config.Tools.Write.MaxSize {
		return fileWriteOutput{}, fmt.Errorf("content too large: %d bytes (max %d)", len(input.Content), deps.Config.Tools.Write.MaxSize)
	}

	beforeBytes, readErr := os.ReadFile(abs)
	beforeMissing := os.IsNotExist(readErr)
	if readErr != nil && !beforeMissing {
		return fileWriteOutput{}, fmt.Errorf("read existing file failed: %w", readErr)
	}
	before := string(beforeBytes)
	if mode == "append" && int64(len(beforeBytes))+int64(len(input.Content)) > deps.Config.Tools.Write.MaxSize {
		return fileWriteOutput{}, fmt.Errorf("appended file too large: %d bytes (max %d)", len(beforeBytes)+len(input.Content), deps.Config.Tools.Write.MaxSize)
	}
	if tctx != nil {
		if err := tctx.Err(); err != nil {
			return fileWriteOutput{}, err
		}
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
	if err := f.Close(); err != nil {
		return fileWriteOutput{}, fmt.Errorf("close file failed: %w", err)
	}
	after := input.Content
	if mode == "append" && !beforeMissing {
		after = before + input.Content
	}

	out := fileWriteOutput{
		Bytes: n,
	}
	recordFileRevision(tctx, abs, mode, before, after, beforeMissing)
	return out, nil
}
