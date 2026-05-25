package app

import (
	"strings"
	"unicode"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	supermansession "github.com/ai4next/superman/internal/session"
	"github.com/ai4next/superman/internal/tui/styles"
)

func newTextarea() textarea.Model {
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.CharLimit = -1
	ta.SetVirtualCursor(false)
	ta.DynamicHeight = true
	ta.MinHeight = 1
	ta.MaxHeight = 6
	ta.SetPromptFunc(4, textareaPrompt)
	ta.Placeholder = "Type a request"
	ta.SetStyles(styles.TextareaStyle)
	ta.Focus()
	return ta
}
func textareaPrompt(info textarea.PromptInfo) string {
	if info.LineNumber == 0 {
		if info.Focused {
			return "  > "
		}
		return "::: "
	}
	if info.Focused {
		return "::: "
	}
	return "::: "
}
func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.Key()
	if m.searchDialog != nil {
		return m.handleSearchDialogKey(msg)
	}
	if m.filePicker != nil {
		return m.handleFilePickerKey(msg)
	}
	if m.commandDialog != nil {
		return m.handleCommandDialogKey(msg)
	}
	if m.sessionDialog != nil {
		return m.handleSessionDialogKey(msg)
	}
	if m.pendingConfirm != nil {
		switch strings.ToLower(key.Text) {
		case "y":
			return m.resumeConfirmation(true)
		case "n":
			return m.resumeConfirmation(false)
		}
	}
	switch {
	case msg.String() == "ctrl+c":
		if m.running {
			return m.cancelRun()
		}
		return m, tea.Quit
	case msg.String() == "ctrl+s":
		m.openSessionDialog()
		return m, nil
	case msg.String() == "ctrl+p":
		m.openCommandDialog()
		return m, nil
	case isSlashCommandTrigger(msg) && strings.TrimSpace(m.inputValue()) == "":
		m.openCommandDialog()
		return m, nil
	case msg.String() == "ctrl+f":
		m.openFilePicker()
		return m, nil
	case key.Code == tea.KeyEnter:
		input := strings.TrimSpace(m.inputValue())
		if input != "" {
			if !m.running {
				if handled, cmd := m.processCommand(input); handled {
					return m, cmd
				}
				return m.processInput()
			}
			if handled, cmd := m.processRunningCommand(input); handled {
				return m, cmd
			}
			return m.enqueuePrompt(input), nil
		}
		if m.scrollOffset > 0 {
			m.scrollOffset = 0
		}
	case msg.String() == "ctrl+j":
		m.resetPromptHistoryDraft()
		m.textarea.InsertRune('\n')
		m.syncInputFromTextarea()
	case key.Code == tea.KeyUp:
		m.historyPrev()
	case key.Code == tea.KeyDown:
		m.historyNext()
	case msg.String() == "ctrl+k":
		m.resetPromptHistoryDraft()
	case key.Code == tea.KeyPgUp || msg.String() == "ctrl+u":
		m.scrollOffset += m.scrollPageStep()
	case key.Code == tea.KeyPgDown || msg.String() == "ctrl+d":
		m.scrollOffset = max(0, m.scrollOffset-m.scrollPageStep())
	default:
		old := m.inputValue()
		msg = normalizePrintableKey(msg)
		ta, cmd := m.textarea.Update(msg)
		m.textarea = ta
		m.syncInputFromTextarea()
		if m.inputValue() != old {
			m.resetPromptHistoryDraft()
		}
		return m, cmd
	}
	return m, nil
}

func isSlashCommandTrigger(msg tea.KeyPressMsg) bool {
	key := msg.Key()
	return key.Text == "/" || key.Code == '/'
}

func normalizePrintableKey(msg tea.KeyPressMsg) tea.KeyPressMsg {
	key := msg.Key()
	if key.Text != "" || key.Code == 0 || key.Mod != 0 {
		return msg
	}
	if unicode.IsPrint(key.Code) {
		key.Text = string(key.Code)
		return tea.KeyPressMsg(key)
	}
	return msg
}

func (m *Model) handleLegacyTextKey(key tea.Key) {
	if key.Text != "" {
		m.resetPromptHistoryDraft()
		m.insertText(key.Text)
	}
}
func (m *Model) insertText(text string) {
	m.textarea.InsertString(text)
	m.syncInputFromTextarea()
}
func (m *Model) setInput(text string) {
	m.textarea.SetValue(text)
	m.textarea.MoveToEnd()
	m.syncInputFromTextarea()
}
func (m *Model) clearInput() {
	m.textarea.SetValue("")
	m.input = ""
	m.cursorPos = 0
}
func (m *Model) inputValue() string {
	if m.textarea.Value() != "" || m.input == "" {
		return m.textarea.Value()
	}
	return m.input
}
func (m *Model) syncInputFromTextarea() {
	m.input = m.textarea.Value()
	m.cursorPos = len([]rune(m.input))
}
func (m *Model) resetPromptHistoryDraft() {
	if m.historyIndex >= 0 {
		m.historyIndex = -1
		m.historyDraft = m.inputValue()
	}
}
func (m *Model) historyPrev() bool {
	if len(m.promptHistory) == 0 {
		return false
	}
	if m.historyIndex == -1 {
		m.historyDraft = m.inputValue()
	}
	next := m.historyIndex + 1
	if next >= len(m.promptHistory) {
		return false
	}
	m.historyIndex = next
	m.setInput(m.promptHistory[next])
	return true
}
func (m *Model) historyNext() bool {
	if m.historyIndex < 0 {
		return false
	}
	next := m.historyIndex - 1
	if next < 0 {
		m.historyIndex = -1
		m.setInput(m.historyDraft)
		return true
	}
	m.historyIndex = next
	m.setInput(m.promptHistory[next])
	return true
}
func (m *Model) refreshPromptHistory() {
	svc, ok := m.sessionService.(*supermansession.Service)
	if !ok {
		return
	}
	history, err := svc.PromptHistory(m.cfg.Session.AppName, "tui-user", m.sessionID, 100)
	if err != nil {
		m.promptHistory = nil
		m.historyIndex = -1
		m.historyDraft = ""
		return
	}
	m.promptHistory = history
	m.historyIndex = -1
	m.historyDraft = ""
}
func (m *Model) addPromptHistory(prompt string) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return
	}
	history := make([]string, 0, min(len(m.promptHistory)+1, 100))
	history = append(history, prompt)
	for _, existing := range m.promptHistory {
		if existing == prompt {
			continue
		}
		history = append(history, existing)
		if len(history) >= 100 {
			break
		}
	}
	m.promptHistory = history
}
