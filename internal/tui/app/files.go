package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	supermansession "github.com/ai4next/superman/internal/session"
	"github.com/ai4next/superman/internal/tui/components"
)

func (m *Model) openFilePicker() {
	files, err := workspaceFiles(m.cfg.Workspace, 500)
	if err != nil {
		m.messages = append(m.messages, components.Message{Role: "error", Content: fmt.Sprintf("Load files failed: %v", err)})
		m.chatCacheDirty = true
		return
	}
	m.filePicker = &filePickerState{Files: files}
	m.chatCacheDirty = true
}
func (m *Model) handleFilePickerKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.filePicker == nil {
		return m, nil
	}
	key := msg.Key()
	filtered := m.filteredFilePickerItems()
	switch {
	case key.Code == tea.KeyEsc || msg.String() == "ctrl+c":
		m.filePicker = nil
	case key.Code == tea.KeyUp:
		if m.filePicker.Selected > 0 {
			m.filePicker.Selected--
		}
	case key.Code == tea.KeyDown:
		if m.filePicker.Selected < len(filtered)-1 {
			m.filePicker.Selected++
		}
	case key.Code == tea.KeyBackspace:
		if m.filePicker.Query != "" {
			runes := []rune(m.filePicker.Query)
			m.filePicker.Query = string(runes[:len(runes)-1])
			m.filePicker.Selected = 0
		}
	case key.Code == tea.KeyEnter:
		filtered = m.filteredFilePickerItems()
		if len(filtered) == 0 {
			m.filePicker = nil
			return m, nil
		}
		if m.filePicker.Selected >= len(filtered) {
			m.filePicker.Selected = len(filtered) - 1
		}
		selected := filtered[m.filePicker.Selected]
		m.filePicker = nil
		m.insertFileReference(selected)
	case key.Text != "":
		m.filePicker.Query += key.Text
		m.filePicker.Selected = 0
	}
	return m, nil
}
func (m *Model) filteredFilePickerItems() []string {
	if m.filePicker == nil {
		return nil
	}
	query := strings.ToLower(strings.TrimSpace(m.filePicker.Query))
	if query == "" {
		return m.filePicker.Files
	}
	var out []string
	for _, path := range m.filePicker.Files {
		if strings.Contains(strings.ToLower(path), query) {
			out = append(out, path)
		}
	}
	if m.filePicker.Selected >= len(out) && len(out) > 0 {
		m.filePicker.Selected = len(out) - 1
	}
	return out
}
func (m *Model) insertFileReference(path string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	if m.inputValue() != "" && !strings.HasSuffix(m.inputValue(), " ") && !strings.HasSuffix(m.inputValue(), "\n") {
		m.insertText(" ")
	}
	m.insertText("@" + path)
	if svc, ok := m.sessionService.(*supermansession.Service); ok {
		abs := path
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(m.cfg.Workspace, path)
		}
		_ = svc.RecordFileRead(m.cfg.Session.AppName, "tui-user", m.sessionID, abs)
		m.refreshSessionFiles()
	}
}
func (m *Model) recordPromptFileReferences(prompt string) int {
	svc, ok := m.sessionService.(*supermansession.Service)
	if !ok || m.cfg == nil {
		return 0
	}
	count := 0
	for _, ref := range supermansession.ExtractFileReferences(prompt) {
		path := supermansession.ResolveWorkspacePath(m.cfg.Workspace, ref)
		if path == "" {
			continue
		}
		if err := svc.RecordFileRead(m.cfg.Session.AppName, "tui-user", m.sessionID, path); err == nil {
			count++
		}
	}
	if count > 0 {
		m.refreshSessionFiles()
	}
	return count
}

func (m *Model) recordPromptSessionReferences(prompt string) int {
	svc, ok := m.sessionService.(*supermansession.Service)
	if !ok || m.cfg == nil {
		return 0
	}
	count := 0
	for _, ref := range supermansession.ExtractSessionReferences(prompt) {
		if err := svc.RecordSessionReference(m.cfg.Session.AppName, "tui-user", m.sessionID, ref); err == nil {
			count++
		}
	}
	return count
}
func readFileSnapshot(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return string(data), false, nil
	}
	if os.IsNotExist(err) {
		return "", true, nil
	}
	return "", false, err
}
func workspaceFiles(root string, limit int) ([]string, error) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			switch name {
			case ".git", "node_modules", ".next", "dist", "build", "vendor", "runtime":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		files = append(files, filepath.ToSlash(rel))
		if limit > 0 && len(files) >= limit {
			return filepath.SkipAll
		}
		return nil
	})
	return files, err
}
