package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/tool/toolconfirmation"
	"google.golang.org/genai"

	"github.com/ai4next/superman/internal/bus"
	supermanruntime "github.com/ai4next/superman/internal/runtime"
	"github.com/ai4next/superman/internal/tui/components"
)

func (m *Model) updateToolMessage(update components.Message) {
	for i := len(m.messages) - 1; i >= 0; i-- {
		msg := &m.messages[i]
		if msg.Role != "tool" {
			continue
		}
		if update.ToolID != "" && msg.ToolID == update.ToolID {
			mergeToolMessage(msg, update)
			return
		}
		if update.ToolID == "" && msg.Tool == update.Tool && msg.Status == "running" {
			mergeToolMessage(msg, update)
			return
		}
	}
	m.messages = append(m.messages, update)
}
func mergeToolMessage(dst *components.Message, src components.Message) {
	if src.Status != "" {
		dst.Status = src.Status
	}
	if src.Result != "" {
		dst.Result = src.Result
	}
	if src.Duration != "" {
		dst.Duration = src.Duration
	}
	if src.Content != "" {
		dst.Content = src.Content
	}
}
func (m *Model) appendAgentDelta(text string) {
	if text == "" {
		return
	}
	last := len(m.messages) - 1
	if last >= 0 && m.messages[last].Role == "agent" {
		m.messages[last].Content += text
	} else {
		m.messages = append(m.messages, components.Message{Role: "agent", Content: text})
	}
	m.chatCacheDirty = true
}
func (m *Model) applyRuntimeEvent(event bus.Event) {
	switch event.Type {
	case bus.EventTextDelta:
		m.appendAgentDelta(event.Text)
	case bus.EventToolCallStarted:
		m.toolStarts[event.ToolID] = time.Now()
		m.currentTool = event.ToolName
		m.messages = append(m.messages, components.Message{
			Role:   "tool",
			Tool:   event.ToolName,
			ToolID: event.ToolID,
			Args:   components.TruncateRunes(event.Args, 180),
			Status: "running",
		})
		m.chatCacheDirty = true
	case bus.EventToolCallFinished:
		update := components.Message{
			Role:   "tool",
			Tool:   event.ToolName,
			ToolID: event.ToolID,
			Status: event.Status,
			Result: components.TruncateRunes(event.Result, 220),
		}
		if startedAt, ok := m.toolStarts[event.ToolID]; ok {
			update.Duration = formatDuration(time.Since(startedAt))
		}
		m.updateToolMessage(update)
		m.refreshSessionFiles()
		m.chatCacheDirty = true
	case bus.EventPermissionRequested:
		m.pendingConfirm = &pendingConfirmation{
			ID:       event.ToolID,
			ToolName: event.ToolName,
			Args:     event.Args,
		}
		m.messages = append(m.messages, components.Message{
			Role:    "tool",
			Tool:    event.ToolName,
			ToolID:  event.ToolID,
			Args:    components.TruncateRunes(event.Args, 180),
			Status:  "awaiting_permission",
			Content: "Permission required: press y to allow once, n to deny.",
		})
		m.chatCacheDirty = true
	case bus.EventPermissionGranted:
		m.resolvePermissionMessage(event.ToolID, "granted", "Permission granted")
		m.pendingConfirm = nil
		m.chatCacheDirty = true
	case bus.EventPermissionDenied:
		m.resolvePermissionMessage(event.ToolID, "denied", "Permission denied")
		m.pendingConfirm = nil
		m.chatCacheDirty = true
	case bus.EventRunFinished:
		m.refreshSessionFiles()
		m.finishRun()
	case bus.EventRunFailed:
		content := event.Error
		if content == "" {
			content = "run failed"
		}
		m.messages = append(m.messages, components.Message{Role: "error", Content: fmt.Sprintf("Error: %s", content)})
		m.chatCacheDirty = true
		m.refreshSessionFiles()
		m.finishRun()
	case bus.EventRunCanceled:
		m.messages = append(m.messages, components.Message{Role: "system", Content: "Run canceled"})
		m.chatCacheDirty = true
		m.finishRun()
	case bus.EventSessionCompacted:
		m.messages = append(m.messages, components.Message{Role: "system", Content: fmt.Sprintf("Session compacted (%d messages summarized)", event.Count)})
		m.chatCacheDirty = true
	}
}
func (m *Model) finishRun() {
	m.running = false
	m.runtimeCh = nil
	if m.runtimeCancel != nil {
		m.runtimeCancel()
		m.runtimeCancel = nil
	}
	m.pulseOn = false
	m.currentTool = ""
	m.responseBuffer.Reset()
	clear(m.toolStarts)
}
func (m *Model) cancelRun() (tea.Model, tea.Cmd) {
	if !m.running {
		return m, nil
	}
	if m.runtimeCancel != nil {
		m.runtimeCancel()
		m.runtimeCancel = nil
	}
	if m.runtimeBroker != nil {
		_ = m.runtimeBroker.Publish(context.Background(), bus.RunCanceled(m.sessionID, ""))
	}
	return m, nil
}
func (m *Model) resolvePermissionMessage(toolID, status, content string) {
	for i := len(m.messages) - 1; i >= 0; i-- {
		msg := &m.messages[i]
		if msg.Role == "tool" && msg.ToolID == toolID {
			msg.Status = status
			msg.Content = content
			return
		}
	}
	m.messages = append(m.messages, components.Message{Role: "tool", ToolID: toolID, Status: status, Content: content})
}
func (m *Model) resumeConfirmation(confirmed bool) (tea.Model, tea.Cmd) {
	pending := m.pendingConfirm
	if pending == nil || m.runner == nil || m.runtimeBroker == nil {
		return m, nil
	}
	status := "denied"
	content := "Permission denied"
	if confirmed {
		status = "granted"
		content = "Permission granted"
	}
	m.resolvePermissionMessage(pending.ID, status, content)
	m.pendingConfirm = nil
	m.running = true
	m.responseBuffer.Reset()
	clear(m.toolStarts)
	m.chatCacheDirty = true

	return m.startRuntime(func(ctx context.Context, broker bus.Broker) tea.Cmd {
		return startConfirmation(ctx, m.runner, broker, m.cfg.Session.AppName, m.sessionID, pending.ID, confirmed, m.compactor())
	})
}
func (m *Model) processInput() (tea.Model, tea.Cmd) {
	m.showWelcome = false
	m.scrollOffset = 0
	prompt := strings.TrimSpace(m.inputValue())
	m.clearInput()
	m.historyIndex = -1
	m.historyDraft = ""
	m.recordPromptFileReferences(prompt)
	m.recordPromptSessionReferences(prompt)
	m.addPromptHistory(prompt)
	m.messages = append(m.messages, components.Message{Role: "user", Content: prompt})
	m.chatCacheDirty = true
	m.running = true
	m.responseBuffer.Reset()
	clear(m.toolStarts)

	if err := m.ensureRunner(); err != nil {
		m.messages = append(m.messages, components.Message{Role: "error", Content: fmt.Sprintf("Error creating runner: %v", err)})
		m.running = false
		return m, nil
	}

	return m.startRuntime(func(ctx context.Context, broker bus.Broker) tea.Cmd {
		return startAgent(ctx, m.runner, broker, m.cfg.Session.AppName, m.sessionID, prompt, m.compactor())
	})
}
func (m *Model) ensureRunner() error {
	if m.runner != nil {
		return nil
	}
	var err error
	m.runner, err = runner.New(runner.Config{
		Agent:             m.agent,
		AppName:           m.cfg.Session.AppName,
		SessionService:    m.sessionService,
		PluginConfig:      m.pluginCfg,
		AutoCreateSession: true,
	})
	return err
}
func (m *Model) startRuntime(command func(context.Context, bus.Broker) tea.Cmd) (tea.Model, tea.Cmd) {
	m.runtimeBroker = bus.NewMemoryBroker()
	runCtx, cancel := context.WithCancel(context.Background())
	m.runtimeCancel = cancel
	if m.auditLogger != nil {
		_ = m.auditLogger.Subscribe(runCtx, m.runtimeBroker, bus.EventFilter{})
	}
	runtimeCh, err := m.runtimeBroker.Subscribe(runCtx, bus.EventFilter{})
	if err != nil {
		runtimeCh = closedRuntimeEventChannel()
	}
	m.runtimeCh = runtimeCh
	return m, tea.Batch(command(runCtx, m.runtimeBroker), waitForRuntimeEvent(runtimeCh), pulseTick())
}
func (m *Model) startNextQueuedPrompt() (tea.Model, tea.Cmd) {
	prompt, ok := m.prepareNextQueuedPrompt()
	if !ok {
		return m, nil
	}
	return m.startRuntime(func(ctx context.Context, broker bus.Broker) tea.Cmd {
		return startAgent(ctx, m.runner, broker, m.cfg.Session.AppName, m.sessionID, prompt, m.compactor())
	})
}
func (m *Model) prepareNextQueuedPrompt() (string, bool) {
	if m.pendingConfirm != nil || m.running {
		return "", false
	}
	queued, ok, err := m.dequeuePromptStore()
	if err != nil {
		m.messages = append(m.messages, components.Message{Role: "error", Content: fmt.Sprintf("Dequeue prompt failed: %v", err)})
		m.chatCacheDirty = true
		return "", false
	}
	if !ok {
		return "", false
	}
	prompt := queued.Content
	m.recordPromptFileReferences(prompt)
	m.recordPromptSessionReferences(prompt)
	m.showWelcome = false
	m.scrollOffset = 0
	m.messages = append(m.messages, components.Message{Role: "user", Content: prompt})
	m.chatCacheDirty = true
	m.running = true
	m.responseBuffer.Reset()
	clear(m.toolStarts)
	if err := m.ensureRunner(); err != nil {
		m.messages = append(m.messages, components.Message{Role: "error", Content: fmt.Sprintf("Error creating runner: %v", err)})
		m.running = false
		return "", false
	}
	return prompt, true
}
func (m *Model) compactor() supermanruntime.Compactor {
	return supermanruntime.SessionCompactor(m.sessionService, m.cfg.Session.MaxTurns)
}
func startAgent(ctx context.Context, run *runner.Runner, broker bus.Broker, appName, sessionID, prompt string, compactor supermanruntime.Compactor) tea.Cmd {
	return func() tea.Msg {
		go runAgent(ctx, run, broker, appName, sessionID, prompt, compactor)
		return nil
	}
}
func startConfirmation(ctx context.Context, run *runner.Runner, broker bus.Broker, appName, sessionID, confirmationID string, confirmed bool, compactor supermanruntime.Compactor) tea.Cmd {
	return func() tea.Msg {
		go runConfirmation(ctx, run, broker, appName, sessionID, confirmationID, confirmed, compactor)
		return nil
	}
}
func waitForRuntimeEvent(ch <-chan bus.Event) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		return runtimeEventMsg{Event: msg, OK: ok}
	}
}
func closedRuntimeEventChannel() <-chan bus.Event {
	ch := make(chan bus.Event)
	close(ch)
	return ch
}
func pulseTick() tea.Cmd {
	return tea.Tick(350*time.Millisecond, func(time.Time) tea.Msg {
		return pulseMsg{}
	})
}
func runAgent(ctx context.Context, run *runner.Runner, broker bus.Broker, appName, sessionID, prompt string, compactor supermanruntime.Compactor) {
	msg := genai.NewContentFromText(prompt, genai.RoleUser)
	for _, evtErr := range supermanruntime.StreamRun(ctx, run, supermanruntime.RunRequest{
		AppName:    appName,
		UserID:     "tui-user",
		SessionID:  sessionID,
		Message:    msg,
		StateDelta: supermanruntime.PromptStateDelta("", prompt),
		Config:     agent.RunConfig{},
		Compact:    compactor,
		LoopDetection: supermanruntime.LoopDetectionConfig{
			Enabled:    true,
			WindowSize: supermanruntime.DefaultLoopWindowSize,
			MaxRepeats: supermanruntime.DefaultLoopMaxRepeats,
		},
	}, broker) {
		if evtErr != nil {
			return
		}
	}
}
func runConfirmation(ctx context.Context, run *runner.Runner, broker bus.Broker, appName, sessionID, confirmationID string, confirmed bool, compactor supermanruntime.Compactor) {
	content := genai.NewContentFromFunctionResponse(
		toolconfirmation.FunctionCallName,
		map[string]any{"confirmed": confirmed},
		genai.RoleUser,
	)
	if len(content.Parts) > 0 && content.Parts[0].FunctionResponse != nil {
		content.Parts[0].FunctionResponse.ID = confirmationID
	}
	for _, evtErr := range supermanruntime.StreamRun(ctx, run, supermanruntime.RunRequest{
		AppName:   appName,
		UserID:    "tui-user",
		SessionID: sessionID,
		Message:   content,
		Config:    agent.RunConfig{},
		Compact:   compactor,
		LoopDetection: supermanruntime.LoopDetectionConfig{
			Enabled:    true,
			WindowSize: supermanruntime.DefaultLoopWindowSize,
			MaxRepeats: supermanruntime.DefaultLoopMaxRepeats,
		},
	}, broker) {
		if evtErr != nil {
			return
		}
	}
}
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
