package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/ai4next/superman/internal/agent/tools"
	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/tui/components"
)

type Model struct {
	width          int
	height         int
	agent          agent.Agent
	cfg            *config.Config
	runner         *runner.Runner
	sessionService session.Service
	pluginCfg      runner.PluginConfig
	messages       []components.Message
	input          string
	cursorPos      int
	running        bool
	showWelcome    bool
	sessionID      string
	userID         string
	err            error
	modelName      string
	cursorRow      int
	cursorCol      int
}

func New(a agent.Agent, cfg *config.Config, pluginCfg runner.PluginConfig, sessSvc session.Service) *Model {
	return &Model{
		agent:          a,
		cfg:            cfg,
		pluginCfg:      pluginCfg,
		sessionService: sessSvc,
		messages:       []components.Message{},
		showWelcome:    true,
		sessionID:      fmt.Sprintf("tui-%d", os.Getpid()),
		userID:         "tui-user",
		modelName:      fmt.Sprintf("%s/%s", cfg.Model.Provider, cfg.Model.Name),
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.ShowCursor
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			if m.input != "" && !m.running {
				return m.processInput()
			}
		case "backspace":
			if msg.Alt {
				m.deleteWordBackward()
			} else if m.cursorPos > 0 {
				runes := []rune(m.input)
				m.input = string(runes[:m.cursorPos-1]) + string(runes[m.cursorPos:])
				m.cursorPos--
			}
		case "left", "alt+left", "alt+b":
			// macOS Terminal.app sends alt+b for Option+Left
			// iTerm2 / modern terminals send alt+left (CSI)
			if msg.Alt {
				m.moveWordBackward()
			} else if m.cursorPos > 0 {
				m.cursorPos--
			}
		case "ctrl+left":
			m.moveWordBackward()
		case "right", "alt+right", "alt+f":
			// macOS Terminal.app sends alt+f for Option+Right
			if msg.Alt {
				m.moveWordForward()
			} else if m.cursorPos < len([]rune(m.input)) {
				m.cursorPos++
			}
		case "ctrl+right":
			m.moveWordForward()
		case "ctrl+a":
			m.cursorPos = 0
		case "ctrl+e":
			m.cursorPos = len([]rune(m.input))
		case "ctrl+w":
			m.deleteWordBackward()
		case "ctrl+k":
			runes := []rune(m.input)
			m.input = string(runes[:m.cursorPos])
		case "home":
			m.cursorPos = 0
		case "end":
			m.cursorPos = len([]rune(m.input))
		default:
			if !msg.Alt && len(msg.Runes) > 0 {
				r := string(msg.Runes)
				runes := []rune(m.input)
				m.input = string(runes[:m.cursorPos]) + r + string(runes[m.cursorPos:])
				m.cursorPos += len(msg.Runes)
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func isWordRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

func (m *Model) moveWordForward() {
	runes := []rune(m.input)
	n := len(runes)
	if m.cursorPos >= n {
		return
	}
	if isWordRune(runes[m.cursorPos]) {
		for m.cursorPos < n && isWordRune(runes[m.cursorPos]) {
			m.cursorPos++
		}
	} else {
		for m.cursorPos < n && !isWordRune(runes[m.cursorPos]) {
			m.cursorPos++
		}
	}
	for m.cursorPos < n && runes[m.cursorPos] == ' ' {
		m.cursorPos++
	}
}

func (m *Model) moveWordBackward() {
	runes := []rune(m.input)
	if m.cursorPos == 0 {
		return
	}
	for m.cursorPos > 0 && runes[m.cursorPos-1] == ' ' {
		m.cursorPos--
	}
	if m.cursorPos > 0 {
		if isWordRune(runes[m.cursorPos-1]) {
			for m.cursorPos > 0 && isWordRune(runes[m.cursorPos-1]) {
				m.cursorPos--
			}
		} else {
			for m.cursorPos > 0 && !isWordRune(runes[m.cursorPos-1]) {
				m.cursorPos--
			}
		}
	}
}

func (m *Model) deleteWordBackward() {
	pos := m.cursorPos
	m.moveWordBackward()
	runes := []rune(m.input)
	m.input = string(runes[:m.cursorPos]) + string(runes[pos:])
}

func (m *Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var above string
	var inputLine string

	if m.showWelcome && len(m.messages) == 0 {
		cwd, _ := os.Getwd()
		welcome := components.RenderWelcome(m.modelName, cwd, m.sessionID)
		inputLine, m.cursorCol = components.RenderInputLine(m.input, m.cursorPos, m.width)
		toolbar := components.RenderToolbar(components.ToolbarData{
			ModelName: m.modelName,
			CWD:       cwd,
		}, m.width)
		above = lipgloss.JoinVertical(lipgloss.Left, welcome, "", toolbar)
	} else {
		chatContent := components.RenderChat(m.messages, m.width)
		inputLine, m.cursorCol = components.RenderInputLine(m.input, m.cursorPos, m.width)
		toolbar := components.RenderToolbar(components.ToolbarData{
			ModelName: m.modelName,
		}, m.width)
		above = lipgloss.JoinVertical(lipgloss.Left, chatContent, toolbar)
	}

	m.cursorRow = len(strings.Split(above, "\n")) + 1
	return above + "\n" + inputLine
}

func (m *Model) processInput() (tea.Model, tea.Cmd) {
	m.showWelcome = false
	prompt := strings.TrimSpace(m.input)
	m.input = ""
	m.cursorPos = 0

	// Inject working memory (checkpoints) into the prompt
	cps := tools.GetCheckpoints()
	if len(cps) > 0 {
		var wm strings.Builder
		wm.WriteString("\n\n<working_memory>\n<key_info>\n")
		for k, v := range cps {
			wm.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
		wm.WriteString("</key_info>\n</working_memory>\n")
		prompt += wm.String()
	}

	m.messages = append(m.messages, components.Message{
		Role:    "user",
		Content: prompt,
	})

	m.running = true

	if m.runner == nil {
		var err error
		m.runner, err = runner.New(runner.Config{
			Agent:              m.agent,
			AppName:            m.cfg.Session.AppName,
			SessionService:     m.sessionService,
			PluginConfig:       m.pluginCfg,
			AutoCreateSession:  true,
		})
		if err != nil {
			m.messages = append(m.messages, components.Message{
				Role:    "agent",
				Content: fmt.Sprintf("Error creating runner: %v", err),
			})
			m.running = false
			return m, nil
		}
	}

	ctx := context.Background()
	msg := genai.NewContentFromText(prompt, "user")

	var response strings.Builder
	for evt, evtErr := range m.runner.Run(ctx, m.userID, m.sessionID, msg, agent.RunConfig{}) {
		if evtErr != nil {
			m.messages = append(m.messages, components.Message{
				Role:    "agent",
				Content: fmt.Sprintf("Error: %v", evtErr),
			})
			m.running = false
			return m, nil
		}
		if evt != nil && evt.Content != nil {
			for _, part := range evt.Content.Parts {
				if part.Text != "" {
					response.WriteString(part.Text)
				}
				if part.FunctionCall != nil {
					m.messages = append(m.messages, components.Message{
						Role: "tool",
						Tool: part.FunctionCall.Name,
					})
				}
			}
		}
	}

	m.messages = append(m.messages, components.Message{
		Role:    "agent",
		Content: response.String(),
	})
	m.running = false
	return m, nil
}

type cursorWriter struct {
	out   *os.File
	model *Model
}

func (w *cursorWriter) Write(data []byte) (int, error) {
	n, err := w.out.Write(data)
	if w.model.cursorRow > 0 && w.model.cursorCol > 0 {
		seq := fmt.Sprintf("\033[%d;%dH\033[4 q", w.model.cursorRow, w.model.cursorCol)
		w.out.Write([]byte(seq))
	}
	return n, err
}

func (w *cursorWriter) Read(p []byte) (int, error)  { return w.out.Read(p) }
func (w *cursorWriter) Close() error                 { return nil }
func (w *cursorWriter) Fd() uintptr                  { return w.out.Fd() }

func Run(ctx context.Context, a agent.Agent, cfg *config.Config, pluginCfg runner.PluginConfig, sessSvc session.Service) error {
	m := New(a, cfg, pluginCfg, sessSvc)
	cw := &cursorWriter{out: os.Stdout, model: m}
	p := tea.NewProgram(m, tea.WithOutput(cw), tea.WithAltScreen())
	_, err := p.Run()
	return err
}