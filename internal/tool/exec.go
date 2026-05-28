package tool

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type execInput struct {
	Command string `json:"command" jsonschema:"Shell command to execute"`
}

type execOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	Duration string `json:"duration"`
}

var shell string

func init() {
	if runtime.GOOS == "windows" {
		shell = "powershell"
	} else if path, err := exec.LookPath("bash"); err == nil && path != "" {
		shell = "bash"
	} else {
		shell = "sh"
	}
}

func newExecTool(deps Dependencies) tool.Tool {
	handler := func(tctx tool.Context, input execInput) (execOutput, error) {
		return runExec(deps, input)
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "exec",
		Description: fmt.Sprintf("Execute a %s command", shell),
	}, handler)
	return t
}

func runExec(deps Dependencies, input execInput) (execOutput, error) {
	timeout := deps.Config.Tools.Exec.Timeout.AsDuration()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if strings.TrimSpace(input.Command) == "" {
		return execOutput{}, fmt.Errorf("command is required")
	}

	shell, args := shellCommand(input.Command)
	cmd := exec.CommandContext(ctx, shell, args...)

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
			return execOutput{}, fmt.Errorf("execution failed: %w", err)
		}
	}

	return execOutput{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Duration: elapsed.Truncate(time.Millisecond).String(),
	}, nil
}

func shellCommand(command string) (string, []string) {
	switch shell {
	case "powershell":
		return shell, []string{"-NoProfile", "-NonInteractive", "-Command", command}
	case "bash":
		return shell, []string{"-c", command}
	default:
		return shell, []string{"-c", command}
	}
}
