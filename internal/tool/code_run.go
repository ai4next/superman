package tool

import (
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type codeRunInput struct {
	Language string `json:"language" jsonschema:"python, bash, or sh"`
	Code     string `json:"code" jsonschema:"Code to run"`
}

type codeRunOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	Duration string `json:"duration"`
}

func newCodeRunTool(deps Dependencies) tool.Tool {
	handler := func(tctx tool.Context, input codeRunInput) (codeRunOutput, error) {
		return runCode(deps, input)
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "code_run",
		Description: "Run Python or shell code.",
		RequireConfirmationProvider: func(input codeRunInput) bool {
			return deps.requiresConfirmation("code_run", input.Language, input)
		},
	}, handler)
	return t
}

func runCode(deps Dependencies, input codeRunInput) (codeRunOutput, error) {
	timeout := deps.Config.Tools.CodeRun.Timeout.AsDuration()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	allowed := deps.Config.Tools.CodeRun.AllowedLanguages
	if len(allowed) > 0 && !slices.Contains(allowed, input.Language) {
		return codeRunOutput{}, fmt.Errorf("language %q is not in the allowed list: %v", input.Language, allowed)
	}

	var cmd *exec.Cmd
	switch input.Language {
	case "python":
		cmd = exec.CommandContext(ctx, "python3", "-c", input.Code)
	case "bash", "sh":
		cmd = exec.CommandContext(ctx, "bash", "-c", input.Code)
	default:
		return codeRunOutput{}, fmt.Errorf("unsupported language: %s", input.Language)
	}

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return codeRunOutput{}, fmt.Errorf("execution failed: %w", err)
		}
	}

	return codeRunOutput{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Duration: elapsed.Truncate(time.Millisecond).String(),
	}, nil
}
