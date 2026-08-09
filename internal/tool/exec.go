package tool

import (
	"context"
	"errors"
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
	Stdout          string `json:"stdout"`
	Stderr          string `json:"stderr"`
	ExitCode        int    `json:"exit_code"`
	Duration        string `json:"duration"`
	StdoutTruncated bool   `json:"stdout_truncated,omitempty"`
	StderrTruncated bool   `json:"stderr_truncated,omitempty"`
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
		return runExec(tctx, deps, input)
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "exec",
		Description: fmt.Sprintf("Execute a %s command", shell),
	}, handler)
	return t
}

func runExec(ctx context.Context, deps Dependencies, input execInput) (execOutput, error) {
	if strings.TrimSpace(input.Command) == "" {
		return execOutput{}, fmt.Errorf("command is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := 30 * time.Second
	maxOutputSize := int64(1_048_576)
	if deps.Config != nil {
		timeout = deps.Config.Tools.Exec.Timeout.AsDuration()
		maxOutputSize = deps.Config.Tools.Exec.MaxOutputSize
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	shell, args := shellCommand(input.Command)
	cmd := exec.CommandContext(ctx, shell, args...)

	if maxOutputSize <= 0 {
		maxOutputSize = 1_048_576
	}
	stdout := newLimitedBuffer(maxOutputSize)
	stderr := newLimitedBuffer(maxOutputSize)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)

	out := execOutput{
		Stdout:          stdout.String(),
		Stderr:          stderr.String(),
		Duration:        elapsed.Truncate(time.Millisecond).String(),
		StdoutTruncated: stdout.Truncated(),
		StderrTruncated: stderr.Truncated(),
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			return out, fmt.Errorf("command timed out after %s: %w", timeout, ctxErr)
		}
		return out, fmt.Errorf("command canceled: %w", ctxErr)
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			out.ExitCode = exitErr.ExitCode()
		} else {
			return out, fmt.Errorf("execution failed: %w", err)
		}
	}
	return out, nil
}

// limitedBuffer keeps draining a child process after the result budget is
// exhausted, preventing both unbounded memory growth and a blocked producer.
type limitedBuffer struct {
	text      strings.Builder
	limit     int64
	truncated bool
}

func newLimitedBuffer(limit int64) limitedBuffer {
	return limitedBuffer{limit: limit}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	originalLen := len(p)
	remaining := b.limit - int64(b.text.Len())
	if remaining <= 0 {
		b.truncated = b.truncated || originalLen > 0
		return originalLen, nil
	}
	if int64(len(p)) > remaining {
		p = p[:int(remaining)]
		b.truncated = true
	}
	_, _ = b.text.Write(p)
	return originalLen, nil
}

func (b *limitedBuffer) String() string { return b.text.String() }

func (b *limitedBuffer) Truncated() bool { return b.truncated }

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
