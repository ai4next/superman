package app

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"

	supermanagent "github.com/ai4next/superman/internal/agent"
	"github.com/ai4next/superman/internal/bus"
	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/global"
	supermansession "github.com/ai4next/superman/internal/session"
	"github.com/ai4next/superman/internal/tui/components"
)

func New(a agent.Agent, cfg *config.Config, pluginCfg runner.PluginConfig, sessSvc session.Service) *Model {
	m := &Model{
		agent:          a,
		cfg:            cfg,
		pluginCfg:      pluginCfg,
		sessionService: sessSvc,
		messages:       []components.Message{},
		showWelcome:    true,
		sessionID:      "1",
		sessionTitle:   "Session 1",
		modelName:      fmt.Sprintf("%s/%s", cfg.Model.Provider, cfg.Model.Name),
		runtimeBroker:  bus.NewMemoryBroker(),
		auditLogger:    bus.NewAuditLogger(global.BusEventsPath()),
		toolStarts:     make(map[string]time.Time),
		chatCacheDirty: true,
		toolsets:       supermanagent.DescribeConfiguredToolsets(cfg),
		historyIndex:   -1,
	}
	m.textarea = newTextarea()
	m.ensureCurrentSession()
	m.loadPersistedMessages()
	m.refreshSessionTitle()
	m.refreshSessionFiles()
	m.refreshPromptQueue()
	m.refreshPromptHistory()
	return m
}

func (m *Model) ensureCurrentSession() {
	if _, err := supermansession.SessionMetadata(m.sessionService, m.cfg.Session.AppName, "tui-user", m.sessionID); err == nil {
		return
	}
	sessions := supermansession.ListSessionMetadata(m.sessionService, m.cfg.Session.AppName, "tui-user")
	if len(sessions) > 0 {
		m.sessionID = sessions[0].SessionID
		m.sessionTitle = sessions[0].Title
		return
	}
	created, err := m.sessionService.Create(context.Background(), &session.CreateRequest{
		AppName: m.cfg.Session.AppName,
		UserID:  "tui-user",
	})
	if err != nil {
		return
	}
	m.sessionID = created.Session.ID()
	m.sessionTitle = "Session " + m.sessionID
}

func (m *Model) loadPersistedMessages() {
	m.messages = nil
	msgs, err := supermansession.Messages(m.sessionService, m.cfg.Session.AppName, "tui-user", m.sessionID)
	if err != nil {
		return
	}
	for _, msg := range msgs {
		switch msg.Role {
		case supermansession.MessageUser:
			m.messages = append(m.messages, components.Message{Role: "user", Content: msg.Content})
		case supermansession.MessageAssistant:
			m.messages = append(m.messages, components.Message{Role: "agent", Content: msg.Content})
		case supermansession.MessageTool:
			m.messages = append(m.messages, components.Message{
				Role:   "tool",
				Tool:   msg.ToolName,
				ToolID: msg.ToolID,
				Args:   components.TruncateRunes(msg.Args, 180),
				Result: components.TruncateRunes(msg.Result, 220),
				Status: msg.Status,
			})
		case supermansession.MessageError:
			m.messages = append(m.messages, components.Message{Role: "error", Content: msg.Content})
		}
	}
	if len(m.messages) > 0 {
		m.showWelcome = false
	}
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case runtimeEventMsg:
		if !msg.OK {
			if m.running {
				m.finishRun()
			}
			return m.startNextQueuedPrompt()
		}
		m.applyRuntimeEvent(msg.Event)
		if msg.Event.Type == bus.EventRunFinished || msg.Event.Type == bus.EventRunFailed {
			return m.startNextQueuedPrompt()
		}
		if m.runtimeCh != nil {
			return m, waitForRuntimeEvent(m.runtimeCh)
		}
	case pulseMsg:
		if m.running {
			m.pulseOn = !m.pulseOn
			return m, pulseTick()
		}
		m.pulseOn = false
	case resizeRenderMsg:
		if msg.Seq == m.resizeSeq {
			m.chatCacheDirty = true
		}
	case tea.KeyPressMsg:
		m.clearSelection()
		return m.handleKey(msg)
	case tea.PasteMsg:
		m.clearSelection()
		return m.handlePaste(msg)
	case tea.MouseClickMsg:
		return m.handleMouseClick(msg)
	case tea.MouseMotionMsg:
		return m.handleMouseMotion(msg)
	case tea.MouseReleaseMsg:
		return m.handleMouseRelease(msg)
	case tea.MouseWheelMsg:
		return m.handleMouseWheel(msg)
	case tea.WindowSizeMsg:
		if m.width != msg.Width {
			m.resizeSeq++
			m.width = msg.Width
			m.height = msg.Height
			return m, resizeRenderTick(m.resizeSeq)
		}
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}
