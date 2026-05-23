package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type fileReadInput struct {
	Path    string `json:"path" jsonschema:"File path"`
	Offset  int    `json:"offset,omitempty" jsonschema:"Start line, 1-based"`
	Limit   int    `json:"limit,omitempty" jsonschema:"Max lines"`
	Keyword string `json:"keyword,omitempty" jsonschema:"Start at first matching line"`
}

type fileReadOutput struct {
	Content    string `json:"content"`
	TotalLines int    `json:"total_lines"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	FilePath   string `json:"file_path"`
}

func newReadTool(deps Dependencies) tool.Tool {
	handler := func(tctx tool.Context, input fileReadInput) (fileReadOutput, error) {
		return readFile(deps, input)
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "read",
		Description: "Read file lines.",
	}, handler)
	return t
}

func readFile(deps Dependencies, input fileReadInput) (fileReadOutput, error) {
	abs, err := filepath.Abs(input.Path)
	if err != nil {
		return fileReadOutput{}, fmt.Errorf("invalid path: %w", err)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return fileReadOutput{}, fmt.Errorf("file not found: %w", err)
	}
	if info.Size() > deps.Config.Tools.Read.MaxSize {
		return fileReadOutput{}, fmt.Errorf("file too large: %d bytes (max %d)", info.Size(), deps.Config.Tools.Read.MaxSize)
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return fileReadOutput{}, fmt.Errorf("read failed: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	totalLines := len(lines)

	startLine := 1
	if input.Offset > 0 {
		startLine = input.Offset
	}
	if input.Keyword != "" {
		for i, line := range lines {
			if strings.Contains(line, input.Keyword) {
				startLine = i + 1
				break
			}
		}
	}
	if startLine > totalLines {
		startLine = totalLines
	}

	limit := input.Limit
	if limit <= 0 || limit > 2000 {
		limit = 2000
	}
	endLine := startLine + limit - 1
	if endLine > totalLines {
		endLine = totalLines
	}

	var result strings.Builder
	for i := startLine - 1; i < endLine; i++ {
		fmt.Fprintf(&result, "%d\t%s\n", i+1, lines[i])
	}

	return fileReadOutput{
		Content:    result.String(),
		TotalLines: totalLines,
		StartLine:  startLine,
		EndLine:    endLine,
		FilePath:   abs,
	}, nil
}
