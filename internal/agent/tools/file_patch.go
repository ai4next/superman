package tools

import (
	"fmt"
	"os"
	"strings"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type filePatchInput struct {
	Path      string `json:"path" jsonschema:"Path to the file to patch"`
	OldString string `json:"old_string" jsonschema:"The exact text to replace"`
	NewString string `json:"new_string" jsonschema:"The text to replace it with"`
}

type filePatchOutput struct {
	FilePath string `json:"file_path"`
	Applied  bool   `json:"applied"`
	Error    string `json:"error,omitempty"`
}

func newFilePatchTool(deps Dependencies) tool.Tool {
	handler := func(tctx tool.Context, input filePatchInput) (filePatchOutput, error) {
		return patchFile(deps, input)
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "file_patch",
		Description: "Make precise edits to a file by replacing old_string with new_string",
	}, handler)
	return t
}

func patchFile(deps Dependencies, input filePatchInput) (filePatchOutput, error) {
	abs, err := validatePath(input.Path, deps.Config.Tools.FilePatch.AllowedPaths)
	if err != nil {
		return filePatchOutput{}, err
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return filePatchOutput{}, fmt.Errorf("read failed: %w", err)
	}

	content := string(data)
	count := strings.Count(content, input.OldString)

	if count == 0 {
		return filePatchOutput{
			FilePath: abs,
			Applied:  false,
			Error:    fmt.Sprintf("old_string not found in file. File has %d lines.", len(strings.Split(content, "\n"))),
		}, nil
	}
	if count > 1 {
		return filePatchOutput{
			FilePath: abs,
			Applied:  false,
			Error:    fmt.Sprintf("old_string found %d times in file. Provide more context to make it unique.", count),
		}, nil
	}

	newContent := strings.Replace(content, input.OldString, input.NewString, 1)
	if err := os.WriteFile(abs, []byte(newContent), 0644); err != nil {
		return filePatchOutput{}, fmt.Errorf("write failed: %w", err)
	}

	return filePatchOutput{
		FilePath: abs,
		Applied:  true,
	}, nil
}