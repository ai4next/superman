package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/adk/session"

	supermansession "github.com/ai4next/superman/internal/session"
	"github.com/ai4next/superman/internal/tui/components"
)

func (m *Model) formatFileRevisionDiff(rev supermansession.FileRevision) (string, error) {
	before, beforeMissing, err := supermansession.FileSnapshotContent(rev.Before)
	if err != nil {
		return "", err
	}
	after, afterMissing, err := supermansession.FileSnapshotContent(rev.After)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("Latest file revision\n")
	b.WriteString("Path: ")
	b.WriteString(rev.Path)
	b.WriteString("\nAction: ")
	b.WriteString(rev.Action)
	b.WriteString(fmt.Sprintf("\nSize: %d -> %d bytes", rev.Before.Size, rev.After.Size))
	if beforeMissing {
		b.WriteString("\nBefore: <missing>")
	}
	if afterMissing {
		b.WriteString("\nAfter: <missing>")
	}
	b.WriteString("\n\nUnified diff:\n")
	b.WriteString(components.TruncateRunes(unifiedDiff(rev.Path, before, after), 3200))
	return b.String(), nil
}
func (m *Model) promptQueue() ([]supermansession.QueuedPrompt, error) {
	queue, err := supermansession.PromptQueue(m.sessionService, m.cfg.Session.AppName, "tui-user", m.sessionID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			m.queueCount = 0
			return nil, nil
		}
		return nil, err
	}
	m.queueCount = len(queue)
	return queue, nil
}
func (m *Model) fileRevisions() ([]supermansession.FileRevision, error) {
	revisions, err := supermansession.FileRevisions(m.sessionService, m.cfg.Session.AppName, "tui-user", m.sessionID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return nil, err
	}
	return revisions, nil
}
func (m *Model) searchMessages(query string) ([]supermansession.MessageSearchResult, error) {
	return supermansession.SearchMessages(m.sessionService, m.cfg.Session.AppName, "tui-user", supermansession.MessageSearchOptions{
		Query: query,
		Limit: 50,
	})
}
func (m *Model) latestFileRevision(path string) (supermansession.FileRevision, bool, error) {
	if strings.TrimSpace(path) == "" {
		return supermansession.FileRevision{}, false, fmt.Errorf("path is required")
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return supermansession.FileRevision{}, false, fmt.Errorf("invalid path: %w", err)
	}
	revisions, err := m.fileRevisions()
	if err != nil {
		return supermansession.FileRevision{}, false, err
	}
	for i := len(revisions) - 1; i >= 0; i-- {
		if revisions[i].Path == target {
			return revisions[i], true, nil
		}
	}
	return supermansession.FileRevision{}, false, nil
}
func (m *Model) revertFile(path string) error {
	revision, ok, err := m.latestFileRevision(path)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no file history found for %s", path)
	}

	current, currentMissing, err := readFileSnapshot(revision.Path)
	if err != nil {
		return err
	}
	before, beforeMissing, err := supermansession.FileSnapshotContent(revision.Before)
	if err != nil {
		return err
	}
	if beforeMissing {
		if err := os.Remove(revision.Path); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := supermansession.RecordFileRevision(m.sessionService, m.cfg.Session.AppName, "tui-user", m.sessionID, supermansession.FileRevisionNote{Path: revision.Path, Action: "revert", Before: current, After: "", BeforeMissing: currentMissing, AfterMissing: true}); err != nil {
			return err
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(revision.Path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(revision.Path, []byte(before), 0o644); err != nil {
		return err
	}
	if err := supermansession.RecordFileRevision(m.sessionService, m.cfg.Session.AppName, "tui-user", m.sessionID, supermansession.FileRevisionNote{Path: revision.Path, Action: "revert", Before: current, After: before, BeforeMissing: currentMissing}); err != nil {
		return err
	}
	return nil
}
func (m *Model) enqueuePromptStore(prompt string) (supermansession.QueuedPrompt, error) {
	queued, err := supermansession.EnqueuePrompt(m.sessionService, m.cfg.Session.AppName, "tui-user", m.sessionID, prompt)
	if err != nil {
		return supermansession.QueuedPrompt{}, err
	}
	m.refreshPromptQueue()
	return queued, nil
}
func (m *Model) dequeuePromptStore() (supermansession.QueuedPrompt, bool, error) {
	queued, ok, err := supermansession.DequeuePrompt(m.sessionService, m.cfg.Session.AppName, "tui-user", m.sessionID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			m.queueCount = 0
			return supermansession.QueuedPrompt{}, false, nil
		}
		return supermansession.QueuedPrompt{}, false, err
	}
	m.refreshPromptQueue()
	return queued, ok, nil
}
func (m *Model) clearPromptQueue() (int, error) {
	cleared, err := supermansession.ClearPromptQueue(m.sessionService, m.cfg.Session.AppName, "tui-user", m.sessionID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			m.queueCount = 0
			return 0, nil
		}
		return 0, err
	}
	m.refreshPromptQueue()
	return cleared, nil
}
func (m *Model) sessionMetadata() ([]supermansession.Metadata, error) {
	return supermansession.ListSessionMetadata(m.sessionService, m.cfg.Session.AppName, "tui-user"), nil
}
func (m *Model) newSession() error {
	created, err := m.sessionService.Create(context.Background(), &session.CreateRequest{
		AppName: m.cfg.Session.AppName,
		UserID:  "tui-user",
	})
	if err != nil {
		return err
	}
	m.switchToSession(created.Session.ID(), false)
	m.messages = append(m.messages, components.Message{Role: "system", Content: "New session: " + created.Session.ID()})
	return nil
}
func (m *Model) switchSession(sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("session id is required")
	}
	if _, err := supermansession.SessionMetadata(m.sessionService, m.cfg.Session.AppName, "tui-user", sessionID); err != nil {
		return err
	}
	m.switchToSession(sessionID, true)
	return nil
}
func (m *Model) switchToSession(sessionID string, announce bool) {
	if m.runtimeCancel != nil {
		m.runtimeCancel()
		m.runtimeCancel = nil
	}
	m.sessionID = sessionID
	m.runner = nil
	m.running = false
	m.runtimeCh = nil
	m.pendingConfirm = nil
	m.currentTool = ""
	m.responseBuffer.Reset()
	clear(m.toolStarts)
	m.scrollOffset = 0
	m.chatCacheDirty = true
	m.fileCount = 0
	m.queueCount = 0
	m.messages = nil
	m.loadPersistedMessages()
	m.refreshSessionTitle()
	m.refreshSessionFiles()
	m.refreshPromptQueue()
	m.refreshPromptHistory()
	m.showWelcome = len(m.messages) == 0
	if announce {
		m.messages = append(m.messages, components.Message{Role: "system", Content: "Switched session: " + sessionID})
		m.showWelcome = false
	}
}
func (m *Model) renameSession(title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Errorf("title is required")
	}
	return supermansession.Rename(m.sessionService, m.cfg.Session.AppName, "tui-user", m.sessionID, title)
}
func formatSessionFiles(files []supermansession.SessionFile, limit int) string {
	if len(files) == 0 {
		return "No files recorded for this session"
	}
	if limit <= 0 || limit > len(files) {
		limit = len(files)
	}
	var b strings.Builder
	b.WriteString("Session files")
	for i := 0; i < limit; i++ {
		file := files[i]
		b.WriteString("\n- ")
		b.WriteString(file.Path)
		switch {
		case file.ReadCount > 0 && file.WriteCount > 0:
			b.WriteString(fmt.Sprintf(" (read %d, wrote %d)", file.ReadCount, file.WriteCount))
		case file.ReadCount > 0:
			b.WriteString(fmt.Sprintf(" (read %d)", file.ReadCount))
		case file.WriteCount > 0:
			b.WriteString(fmt.Sprintf(" (wrote %d)", file.WriteCount))
		}
	}
	if len(files) > limit {
		b.WriteString(fmt.Sprintf("\n- ... %d more", len(files)-limit))
	}
	return b.String()
}
func formatSessionFileChanges(changes []supermansession.FileChangeSummary, files []supermansession.SessionFile, limit int) string {
	if len(changes) == 0 {
		return formatSessionFiles(files, limit)
	}
	if limit <= 0 || limit > len(changes) {
		limit = len(changes)
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Session files (%d changed)", len(changes)))
	for i := 0; i < limit; i++ {
		change := changes[i]
		file := change.File
		b.WriteString("\n- ")
		b.WriteString(file.Path)
		b.WriteString(fmt.Sprintf(" (+%d -%d)", change.Additions, change.Deletions))
		switch {
		case file.ReadCount > 0 && file.WriteCount > 0:
			b.WriteString(fmt.Sprintf(" read:%d wrote:%d", file.ReadCount, file.WriteCount))
		case file.ReadCount > 0:
			b.WriteString(fmt.Sprintf(" read:%d", file.ReadCount))
		case file.WriteCount > 0:
			b.WriteString(fmt.Sprintf(" wrote:%d", file.WriteCount))
		}
		b.WriteString(" latest:")
		b.WriteString(change.LatestRevision.Action)
	}
	if len(changes) > limit {
		b.WriteString(fmt.Sprintf("\n- ... %d more changed", len(changes)-limit))
	}
	if len(files) > len(changes) {
		b.WriteString(fmt.Sprintf("\n- ... %d read-only/unchanged", len(files)-len(changes)))
	}
	return b.String()
}
func (m *Model) refreshSessionFiles() {
	if m.sessionService == nil || m.cfg == nil {
		return
	}
	files, err := supermansession.SessionFiles(m.sessionService, m.cfg.Session.AppName, "tui-user", m.sessionID)
	if err != nil {
		m.fileCount = 0
		return
	}
	m.fileCount = len(files)
	changes, err := supermansession.SessionFileChanges(m.sessionService, m.cfg.Session.AppName, "tui-user", m.sessionID)
	if err != nil {
		m.fileChanges = nil
		return
	}
	m.fileChanges = changes
}
func (m *Model) refreshPromptQueue() {
	if m.sessionService == nil || m.cfg == nil {
		return
	}
	queue, err := supermansession.PromptQueue(m.sessionService, m.cfg.Session.AppName, "tui-user", m.sessionID)
	if err != nil {
		m.queueCount = 0
		return
	}
	m.queueCount = len(queue)
}
func (m *Model) refreshSessionTitle() {
	if m.sessionService == nil || m.cfg == nil {
		return
	}
	meta, err := supermansession.SessionMetadata(m.sessionService, m.cfg.Session.AppName, "tui-user", m.sessionID)
	if err != nil {
		m.sessionTitle = "Session " + m.sessionID
		return
	}
	m.sessionTitle = meta.Title
}
