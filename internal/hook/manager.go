package hook

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/plugin"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"
)

// eventDirs maps hook event names to their subdirectory names.
var eventDirs = []string{
	"on_user_message",
	"before_run",
	"after_run",
	"before_agent",
	"after_agent",
	"before_model",
	"after_model",
	"on_model_error",
	"before_tool",
	"after_tool",
	"on_tool_error",
}

// Manager scans the hooks/ directory and creates an ADK Plugin that
// dispatches to scripts for each lifecycle event.
type Manager struct {
	baseDir string
	scripts map[string][]string // event name → sorted script paths
}

// NewManager scans the base directory for hook subdirectories and
// builds the event→script mapping. Missing directories are silently skipped.
func NewManager(baseDir string) (*Manager, error) {
	m := &Manager{
		baseDir: baseDir,
		scripts: make(map[string][]string),
	}

	for _, evt := range eventDirs {
		dir := filepath.Join(baseDir, evt)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read %s: %w", dir, err)
		}

		var scripts []string
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			// Only include executable files
			if info.Mode()&0111 != 0 {
				scripts = append(scripts, filepath.Join(dir, entry.Name()))
			}
		}
		sort.Strings(scripts)
		if len(scripts) > 0 {
			m.scripts[evt] = scripts
			log.Printf("[hook] %s: %d script(s)", evt, len(scripts))
		}
	}

	return m, nil
}

// Plugin creates an ADK Plugin with all registered callbacks wired to
// the corresponding hook scripts.
func (m *Manager) Plugin() *plugin.Plugin {
	p, err := plugin.New(plugin.Config{
		Name:                  "hook_manager",
		OnUserMessageCallback: m.onUserMessage,
		BeforeRunCallback:     m.beforeRun,
		AfterRunCallback:      m.afterRun,
		BeforeAgentCallback:   m.beforeAgent,
		AfterAgentCallback:    m.afterAgent,
		BeforeModelCallback:   m.beforeModel,
		AfterModelCallback:    m.afterModel,
		OnModelErrorCallback:  m.onModelError,
		BeforeToolCallback:    m.beforeTool,
		AfterToolCallback:     m.afterTool,
		OnToolErrorCallback:   m.onToolError,
	})
	if err != nil {
		log.Printf("[hook] plugin.New failed: %v", err)
		return nil
	}
	return p
}

// scriptsFor returns the scripts registered for an event.
func (m *Manager) scriptsFor(evt string) []string {
	return m.scripts[evt]
}

// sessionID extracts the session ID from various ADK context types.
// agent.InvocationContext uses Session().ID(), while agent.CallbackContext
// and tool.Context use SessionID() from agent.ReadonlyContext.
func (m *Manager) sessionID(ctx any) string {
	if ic, ok := ctx.(agent.InvocationContext); ok {
		if ic == nil || ic.Session() == nil {
			return ""
		}
		return ic.Session().ID()
	}
	if rc, ok := ctx.(agent.ReadonlyContext); ok {
		return rc.SessionID()
	}
	return ""
}

// runHooks executes all scripts for the given event. For before_* events,
// if any script returns allow:false, the remaining scripts are skipped
// and RunHooksResult.Block is set to true.
func (m *Manager) runHooks(ctx context.Context, evt string, event HookEvent) RunHooksResult {
	scripts := m.scriptsFor(evt)
	if len(scripts) == 0 {
		return RunHooksResult{}
	}

	for _, script := range scripts {
		result, err := RunScript(ctx, script, event)
		if err != nil {
			log.Printf("[hook] %s/%s: script error: %v", evt, filepath.Base(script), err)
		}
		if !result.Allow && strings.HasPrefix(evt, "before_") {
			log.Printf("[hook] %s/%s: blocked — %s", evt, filepath.Base(script), result.Reason)
			return RunHooksResult{Block: true, Reason: result.Reason}
		}
	}
	return RunHooksResult{}
}

// RunHooksResult captures whether a before_* hook blocked execution.
type RunHooksResult struct {
	Block  bool
	Reason string
}

// onUserMessage handles OnUserMessage.
func (m *Manager) onUserMessage(ic agent.InvocationContext, content *genai.Content) (*genai.Content, error) {
	m.runHooks(ic, "on_user_message", HookEvent{
		Event:     "on_user_message",
		SessionID: m.sessionID(ic),
	})
	return nil, nil
}

// beforeRun handles BeforeRun. Returns non-nil content to preempt the run.
func (m *Manager) beforeRun(ic agent.InvocationContext) (*genai.Content, error) {
	result := m.runHooks(ic, "before_run", HookEvent{
		Event:     "before_run",
		SessionID: m.sessionID(ic),
	})
	if result.Block {
		return genai.NewContentFromText("blocked: "+result.Reason, genai.RoleModel), nil
	}
	return nil, nil
}

// afterRun handles AfterRun.
func (m *Manager) afterRun(ic agent.InvocationContext) {
	m.runHooks(ic, "after_run", HookEvent{
		Event:     "after_run",
		SessionID: m.sessionID(ic),
	})
}

// beforeAgent handles BeforeAgent.
func (m *Manager) beforeAgent(ctx agent.CallbackContext) (*genai.Content, error) {
	result := m.runHooks(ctx, "before_agent", HookEvent{
		Event:     "before_agent",
		SessionID: m.sessionID(ctx),
	})
	if result.Block {
		return genai.NewContentFromText("blocked: "+result.Reason, genai.RoleModel), nil
	}
	return nil, nil
}

// afterAgent handles AfterAgent.
func (m *Manager) afterAgent(ctx agent.CallbackContext) (*genai.Content, error) {
	m.runHooks(ctx, "after_agent", HookEvent{
		Event:     "after_agent",
		SessionID: m.sessionID(ctx),
	})
	return nil, nil
}

// beforeModel handles BeforeModel.
func (m *Manager) beforeModel(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
	result := m.runHooks(ctx, "before_model", HookEvent{
		Event:     "before_model",
		SessionID: m.sessionID(ctx),
	})
	if result.Block {
		return nil, fmt.Errorf("%s", result.Reason)
	}
	return nil, nil
}

// afterModel handles AfterModel.
func (m *Manager) afterModel(ctx agent.CallbackContext, resp *model.LLMResponse, err error) (*model.LLMResponse, error) {
	hookErr := ""
	if err != nil {
		hookErr = err.Error()
	}
	m.runHooks(ctx, "after_model", HookEvent{
		Event:     "after_model",
		SessionID: m.sessionID(ctx),
		Error:     hookErr,
	})
	return nil, nil
}

// onModelError handles OnModelError.
func (m *Manager) onModelError(ctx agent.CallbackContext, req *model.LLMRequest, err error) (*model.LLMResponse, error) {
	hookErr := ""
	if err != nil {
		hookErr = err.Error()
	}
	m.runHooks(ctx, "on_model_error", HookEvent{
		Event:     "on_model_error",
		SessionID: m.sessionID(ctx),
		Error:     hookErr,
	})
	return nil, nil
}

// beforeTool handles BeforeTool.
func (m *Manager) beforeTool(ctx tool.Context, t tool.Tool, args map[string]any) (map[string]any, error) {
	result := m.runHooks(ctx, "before_tool", HookEvent{
		Event:     "before_tool",
		SessionID: m.sessionID(ctx),
		ToolName:  t.Name(),
		ToolArgs:  args,
	})
	if result.Block {
		return nil, fmt.Errorf("%s", result.Reason)
	}
	// Deferred to v2: result.Modified tool_args handling (see spec "不做什么")
	return nil, nil
}

// afterTool handles AfterTool.
func (m *Manager) afterTool(ctx tool.Context, t tool.Tool, args, result map[string]any, err error) (map[string]any, error) {
	hookErr := ""
	if err != nil {
		hookErr = err.Error()
	}
	m.runHooks(ctx, "after_tool", HookEvent{
		Event:     "after_tool",
		SessionID: m.sessionID(ctx),
		ToolName:  t.Name(),
		ToolArgs:  args,
		Error:     hookErr,
	})
	return nil, nil
}

// onToolError handles OnToolError.
func (m *Manager) onToolError(ctx tool.Context, t tool.Tool, args map[string]any, err error) (map[string]any, error) {
	hookErr := ""
	if err != nil {
		hookErr = err.Error()
	}
	m.runHooks(ctx, "on_tool_error", HookEvent{
		Event:     "on_tool_error",
		SessionID: m.sessionID(ctx),
		ToolName:  t.Name(),
		ToolArgs:  args,
		Error:     hookErr,
	})
	return nil, nil
}
