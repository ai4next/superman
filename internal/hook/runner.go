package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// HookEvent is the JSON input sent to hook scripts via stdin.
type HookEvent struct {
	Event     string         `json:"event"`
	SessionID string         `json:"session_id"`
	RunID     string         `json:"run_id,omitempty"`
	ToolName  string         `json:"tool_name,omitempty"`
	ToolArgs  map[string]any `json:"tool_args,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// HookResult is the parsed JSON output from hook scripts.
type HookResult struct {
	Allow    bool           `json:"allow"`
	Reason   string         `json:"reason,omitempty"`
	Modified map[string]any `json:"modified,omitempty"`
}

const (
	defaultTimeout = 30 * time.Second
	maxStdoutSize  = 64 * 1024 // 64KB
)

// RunScript executes a hook script with the given event data.
// It writes JSON to stdin, reads up to maxStdoutSize from stdout,
// and returns the parsed HookResult. Default result (Allow: true)
// is returned when stdout is empty or not valid JSON.
func RunScript(ctx context.Context, scriptPath string, event HookEvent) (HookResult, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, scriptPath)
	cmd.Dir = filepath.Dir(scriptPath)
	cmd.Env = append(os.Environ(),
		"SUPERMAN_SESSION_ID="+event.SessionID,
		"SUPERMAN_RUN_ID="+event.RunID,
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return HookResult{Allow: true}, fmt.Errorf("stdin pipe: %w", err)
	}

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return HookResult{Allow: true}, fmt.Errorf("start: %w", err)
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		return HookResult{Allow: true}, fmt.Errorf("json marshal: %w", err)
	}
	if _, err := stdin.Write(eventJSON); err != nil {
		return HookResult{Allow: true}, fmt.Errorf("stdin write: %w", err)
	}
	if _, err := stdin.Write([]byte("\n")); err != nil {
		return HookResult{Allow: true}, fmt.Errorf("stdin write: %w", err)
	}
	stdin.Close()

	err = cmd.Wait()

	// Read stdout (truncated to maxStdoutSize)
	output := stdout.Bytes()
	if len(output) > maxStdoutSize {
		output = output[:maxStdoutSize]
	}

	result := HookResult{Allow: true}
	output = bytes.TrimSpace(output)
	if len(output) == 0 {
		return result, err
	}

	if jsonErr := json.Unmarshal(output, &result); jsonErr != nil {
		// Non-JSON output is logged but doesn't block
		return HookResult{Allow: true}, err
	}

	return result, err
}
