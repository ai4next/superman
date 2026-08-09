package tool

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ai4next/superman/internal/config"
)

func TestExecToolUsesOSShell(t *testing.T) {
	shell, args := shellCommand("echo ok")
	if runtime.GOOS == "windows" {
		if shell != "powershell" {
			t.Fatalf("shell = %q, want powershell", shell)
		}
		if !contains(args, "-Command") {
			t.Fatalf("powershell args missing -Command: %#v", args)
		}
		return
	}
	if !strings.HasSuffix(shell, "bash") && shell != "sh" {
		t.Fatalf("shell = %q, want bash or sh", shell)
	}
	if len(args) < 2 || args[len(args)-2] != "-c" || args[len(args)-1] != "echo ok" {
		t.Fatalf("shell args = %#v", args)
	}
}

func TestRunExecRequiresCommand(t *testing.T) {
	_, err := runExec(t.Context(), Dependencies{Config: &config.Config{}}, execInput{})
	if err == nil || !strings.Contains(err.Error(), "command is required") {
		t.Fatalf("err = %v, want command required", err)
	}
}

func TestRunExecHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := runExec(ctx, Dependencies{Config: &config.Config{}}, execInput{Command: "echo ignored"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestRunExecReportsTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command is Unix-specific")
	}
	deps := Dependencies{Config: &config.Config{Tools: config.ToolsConfig{
		Exec: config.ExecConfig{Timeout: config.Duration(10 * time.Millisecond)},
	}}}

	_, err := runExec(t.Context(), deps, execInput{Command: "sleep 1"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestRunExecTruncatesOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("printf command is Unix-specific")
	}
	deps := Dependencies{Config: &config.Config{Tools: config.ToolsConfig{
		Exec: config.ExecConfig{
			Timeout:       config.Duration(time.Second),
			MaxOutputSize: 5,
		},
	}}}

	out, err := runExec(t.Context(), deps, execInput{Command: "printf 123456789"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Stdout != "12345" || !out.StdoutTruncated {
		t.Fatalf("stdout = %q, truncated = %v", out.Stdout, out.StdoutTruncated)
	}
}

func TestLimitedBufferDrainsAfterLimit(t *testing.T) {
	buf := newLimitedBuffer(3)
	if n, err := buf.Write([]byte("abcdef")); err != nil || n != 6 {
		t.Fatalf("first write = (%d, %v), want (6, nil)", n, err)
	}
	if n, err := buf.Write([]byte("gh")); err != nil || n != 2 {
		t.Fatalf("second write = (%d, %v), want (2, nil)", n, err)
	}
	if got := buf.String(); got != "abc" || !buf.Truncated() {
		t.Fatalf("buffer = %q, truncated = %v", got, buf.Truncated())
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
