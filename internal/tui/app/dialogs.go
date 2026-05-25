package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	supermansession "github.com/ai4next/superman/internal/session"
	"github.com/ai4next/superman/internal/tui/components"
)

func (m *Model) openSessionDialog() {
	sessions, err := m.sessionMetadata()
	if err != nil {
		m.messages = append(m.messages, components.Message{Role: "error", Content: fmt.Sprintf("Load sessions failed: %v", err)})
		m.chatCacheDirty = true
		return
	}
	selected := 0
	for i, meta := range sessions {
		if meta.SessionID == m.sessionID {
			selected = i
			break
		}
	}
	m.sessionDialog = &sessionDialogState{Sessions: sessions, Selected: selected}
	m.chatCacheDirty = true
}
func (m *Model) openCommandDialog() {
	commands := []components.CommandDialogItem{
		{ID: "new", Title: "New Session", Description: "Start a fresh ADK session", Key: "ctrl+n"},
		{ID: "sessions", Title: "Sessions", Description: "Switch session", Key: "ctrl+s"},
		{ID: "filepicker", Title: "Open File Picker", Description: "Insert a workspace file reference", Key: "ctrl+f"},
		{ID: "toolsets", Title: "Toolsets", Description: "Show ADK Skill and MCP toolsets", Key: "/toolsets"},
		{ID: "search", Title: "Search History", Description: "Search persisted session messages", Key: "/search"},
		{ID: "files", Title: "Files", Description: "Show session working files", Key: "/files"},
		{ID: "history", Title: "File History", Description: "Show file revisions", Key: "/history"},
		{ID: "compact", Title: "Compact Session", Description: "Summarize old context", Key: "/compact"},
		{ID: "queue", Title: "Prompt Queue", Description: "Show queued prompts", Key: "/queue"},
		{ID: "clearqueue", Title: "Clear Queue", Description: "Remove queued prompts", Key: "/clearqueue"},
	}
	if m.running {
		commands = append([]components.CommandDialogItem{{ID: "cancel", Title: "Cancel Run", Description: "Stop current agent run", Key: "/cancel"}}, commands...)
	}
	m.commandDialog = &commandDialogState{Commands: commands}
	m.chatCacheDirty = true
}
func (m *Model) handleCommandDialogKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.commandDialog == nil {
		return m, nil
	}
	key := msg.Key()
	filtered := m.filteredCommandDialogItems()
	switch {
	case key.Code == tea.KeyEsc || msg.String() == "ctrl+c":
		m.commandDialog = nil
	case key.Code == tea.KeyUp:
		if m.commandDialog.Selected > 0 {
			m.commandDialog.Selected--
		}
	case key.Code == tea.KeyDown:
		if m.commandDialog.Selected < len(filtered)-1 {
			m.commandDialog.Selected++
		}
	case key.Code == tea.KeyBackspace:
		if m.commandDialog.Query != "" {
			runes := []rune(m.commandDialog.Query)
			m.commandDialog.Query = string(runes[:len(runes)-1])
			m.commandDialog.Selected = 0
		}
	case key.Code == tea.KeyEnter:
		filtered = m.filteredCommandDialogItems()
		if len(filtered) == 0 {
			m.commandDialog = nil
			return m, nil
		}
		if m.commandDialog.Selected >= len(filtered) {
			m.commandDialog.Selected = len(filtered) - 1
		}
		command := filtered[m.commandDialog.Selected]
		m.commandDialog = nil
		return m.runCommandDialogAction(command.ID)
	case key.Text != "":
		m.commandDialog.Query += key.Text
		m.commandDialog.Selected = 0
	}
	return m, nil
}
func (m *Model) filteredCommandDialogItems() []components.CommandDialogItem {
	if m.commandDialog == nil {
		return nil
	}
	query := strings.ToLower(strings.TrimSpace(m.commandDialog.Query))
	if query == "" {
		return m.commandDialog.Commands
	}
	var out []components.CommandDialogItem
	for _, command := range m.commandDialog.Commands {
		haystack := strings.ToLower(command.Title + " " + command.Description + " " + command.Key + " " + command.ID)
		if strings.Contains(haystack, query) {
			out = append(out, command)
		}
	}
	if m.commandDialog.Selected >= len(out) && len(out) > 0 {
		m.commandDialog.Selected = len(out) - 1
	}
	return out
}
func (m *Model) runCommandDialogAction(id string) (tea.Model, tea.Cmd) {
	switch id {
	case "filepicker":
		m.openFilePicker()
		return m, nil
	case "sessions":
		m.openSessionDialog()
		return m, nil
	case "search":
		m.setInput("/search ")
		return m, nil
	case "cancel":
		return m.cancelRun()
	case "new", "files", "history", "compact", "queue", "clearqueue", "toolsets":
		handled, cmd := m.processCommand("/" + id)
		if !handled && m.running {
			handled, cmd = m.processRunningCommand("/" + id)
		}
		return m, cmd
	default:
		return m, nil
	}
}
func (m *Model) handleSessionDialogKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.sessionDialog == nil {
		return m, nil
	}
	key := msg.Key()
	switch {
	case key.Code == tea.KeyEsc || msg.String() == "ctrl+c":
		m.sessionDialog = nil
	case key.Code == tea.KeyUp:
		if m.sessionDialog.Selected > 0 {
			m.sessionDialog.Selected--
		}
	case key.Code == tea.KeyDown:
		if m.sessionDialog.Selected < len(m.sessionDialog.Sessions)-1 {
			m.sessionDialog.Selected++
		}
	case key.Code == tea.KeyEnter:
		if len(m.sessionDialog.Sessions) == 0 {
			m.sessionDialog = nil
			return m, nil
		}
		selected := m.sessionDialog.Sessions[m.sessionDialog.Selected]
		m.sessionDialog = nil
		if selected.SessionID != m.sessionID {
			if err := m.switchSession(selected.SessionID); err != nil {
				m.messages = append(m.messages, components.Message{Role: "error", Content: fmt.Sprintf("Switch session failed: %v", err)})
				m.chatCacheDirty = true
			}
		}
	}
	return m, nil
}
func (m *Model) handleSearchDialogKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.searchDialog == nil {
		return m, nil
	}
	key := msg.Key()
	switch {
	case key.Code == tea.KeyEsc || msg.String() == "ctrl+c":
		m.searchDialog = nil
	case key.Code == tea.KeyUp:
		if m.searchDialog.Selected > 0 {
			m.searchDialog.Selected--
		}
	case key.Code == tea.KeyDown:
		if m.searchDialog.Selected < len(m.searchDialog.Results)-1 {
			m.searchDialog.Selected++
		}
	case strings.EqualFold(key.Text, "i"):
		if result, ok := m.selectedSearchResult(); ok {
			m.searchDialog = nil
			m.insertSearchReference(result)
			m.chatCacheDirty = true
		}
	case key.Code == tea.KeyEnter:
		if len(m.searchDialog.Results) == 0 {
			m.searchDialog = nil
			return m, nil
		}
		selected, _ := m.selectedSearchResult()
		m.searchDialog = nil
		if selected.Metadata.SessionID != "" && selected.Metadata.SessionID != m.sessionID {
			if err := m.switchSession(selected.Metadata.SessionID); err != nil {
				m.messages = append(m.messages, components.Message{Role: "error", Content: fmt.Sprintf("Switch session failed: %v", err)})
			} else {
				m.messages = append(m.messages, components.Message{Role: "system", Content: "Selected search result: " + components.TruncateRunes(selected.Preview, 160)})
			}
		} else if selected.Preview != "" {
			m.messages = append(m.messages, components.Message{Role: "system", Content: "Selected search result: " + components.TruncateRunes(selected.Preview, 160)})
		}
		m.chatCacheDirty = true
	}
	return m, nil
}

func (m *Model) selectedSearchResult() (supermansession.MessageSearchResult, bool) {
	if m.searchDialog == nil || len(m.searchDialog.Results) == 0 {
		return supermansession.MessageSearchResult{}, false
	}
	if m.searchDialog.Selected >= len(m.searchDialog.Results) {
		m.searchDialog.Selected = len(m.searchDialog.Results) - 1
	}
	if m.searchDialog.Selected < 0 {
		m.searchDialog.Selected = 0
	}
	return m.searchDialog.Results[m.searchDialog.Selected], true
}

func (m *Model) insertSearchReference(result supermansession.MessageSearchResult) {
	ref := formatSearchReference(result)
	if ref == "" {
		return
	}
	if m.inputValue() != "" && !strings.HasSuffix(m.inputValue(), " ") && !strings.HasSuffix(m.inputValue(), "\n") {
		m.insertText(" ")
	}
	m.insertText(ref)
}

func formatSearchReference(result supermansession.MessageSearchResult) string {
	sessionID := strings.TrimSpace(result.Metadata.SessionID)
	if sessionID == "" {
		return ""
	}
	preview := strings.Join(strings.Fields(result.Preview), " ")
	if preview == "" {
		preview = strings.Join(strings.Fields(result.Message.Content), " ")
	}
	if preview == "" {
		preview = strings.Join(strings.Fields(result.Message.Result), " ")
	}
	if preview == "" {
		preview = strings.Join(strings.Fields(result.Message.Args), " ")
	}
	if preview == "" {
		preview = string(result.Message.Role)
	}
	return fmt.Sprintf("[session:%s role:%s] %s", sessionID, result.Message.Role, components.TruncateRunes(preview, 180))
}
