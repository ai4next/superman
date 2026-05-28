package tool

import (
	"runtime"
	"strings"
	"testing"

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
	_, err := runExec(Dependencies{Config: &config.Config{}}, execInput{})
	if err == nil || !strings.Contains(err.Error(), "command is required") {
		t.Fatalf("err = %v, want command required", err)
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
