package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type filePatchInput struct {
	Path      string `json:"path" jsonschema:"File path"`
	OldString string `json:"old_string" jsonschema:"Exact text to replace"`
	NewString string `json:"new_string" jsonschema:"Replacement text"`
}

type filePatchOutput struct {
	FilePath string `json:"file_path"`
	Applied  bool   `json:"applied"`
}

func newPatchTool(deps Dependencies) tool.Tool {
	handler := func(tctx tool.Context, input filePatchInput) (filePatchOutput, error) {
		return patchFile(tctx, deps, input)
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "patch",
		Description: "Replace one exact text match in a file.",
		RequireConfirmationProvider: func(input filePatchInput) bool {
			return deps.requiresConfirmation("patch", "replace", input)
		},
	}, handler)
	return t
}

func patchFile(tctx tool.Context, deps Dependencies, input filePatchInput) (filePatchOutput, error) {
	abs, err := filepath.Abs(input.Path)
	if err != nil {
		return filePatchOutput{}, fmt.Errorf("invalid path: %w", err)
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return filePatchOutput{}, fmt.Errorf("read failed: %w", err)
	}

	content := string(data)
	count := strings.Count(content, input.OldString)

	if count == 0 {
		return filePatchOutput{}, fmt.Errorf("old_string not found in file. File has %d lines.", len(strings.Split(content, "\n")))
	}
	if count > 1 {
		return filePatchOutput{}, fmt.Errorf("old_string found %d times in file. Provide more context to make it unique.", count)
	}

	newContent := strings.Replace(content, input.OldString, input.NewString, 1)
	if err := os.WriteFile(abs, []byte(newContent), 0644); err != nil {
		return filePatchOutput{}, fmt.Errorf("write failed: %w", err)
	}

	out := filePatchOutput{
		FilePath: abs,
		Applied:  true,
	}
	recordFileRevision(tctx, deps, abs, "patch", content, newContent, false)
	return out, nil
}
