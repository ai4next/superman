package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	supermanagent "github.com/ai4next/superman/internal/agent"
	supermansession "github.com/ai4next/superman/internal/session"
	"github.com/ai4next/superman/internal/tui/components"
)

func (m *Model) processRunningCommand(input string) (bool, tea.Cmd) {
	switch input {
	case "/cancel":
		m.clearInput()
		_, cmd := m.cancelRun()
		return true, cmd
	case "/queue":
		m.clearInput()
		queue, err := m.promptQueue()
		if err != nil {
			m.messages = append(m.messages, components.Message{Role: "error", Content: fmt.Sprintf("Load prompt queue failed: %v", err)})
		} else {
			m.messages = append(m.messages, components.Message{Role: "system", Content: formatPromptQueue(queue, 12)})
		}
		m.chatCacheDirty = true
		return true, nil
	case "/clearqueue":
		m.clearInput()
		cleared, err := m.clearPromptQueue()
		if err != nil {
			m.messages = append(m.messages, components.Message{Role: "error", Content: fmt.Sprintf("Clear prompt queue failed: %v", err)})
			m.chatCacheDirty = true
			return true, nil
		}
		m.messages = append(m.messages, components.Message{Role: "system", Content: fmt.Sprintf("Cleared %d queued prompt(s)", cleared)})
		m.chatCacheDirty = true
		return true, nil
	}
	return false, nil
}
func (m *Model) processCommand(input string) (bool, tea.Cmd) {
	switch input {
	case "/sessions":
		m.clearInput()
		sessions, err := m.sessionMetadata()
		if err != nil {
			m.messages = append(m.messages, components.Message{Role: "error", Content: fmt.Sprintf("Load sessions failed: %v", err)})
		} else {
			m.messages = append(m.messages, components.Message{Role: "system", Content: formatSessions(sessions, m.sessionID, 12)})
		}
		m.chatCacheDirty = true
		return true, nil
	case "/new":
		m.clearInput()
		if err := m.newSession(); err != nil {
			m.messages = append(m.messages, components.Message{Role: "error", Content: fmt.Sprintf("Create session failed: %v", err)})
		}
		m.chatCacheDirty = true
		return true, nil
	case "/queue":
		m.clearInput()
		queue, err := m.promptQueue()
		if err != nil {
			m.messages = append(m.messages, components.Message{Role: "error", Content: fmt.Sprintf("Load prompt queue failed: %v", err)})
		} else {
			m.messages = append(m.messages, components.Message{Role: "system", Content: formatPromptQueue(queue, 12)})
		}
		m.chatCacheDirty = true
		return true, nil
	case "/clearqueue":
		m.clearInput()
		cleared, err := m.clearPromptQueue()
		if err != nil {
			m.messages = append(m.messages, components.Message{Role: "error", Content: fmt.Sprintf("Clear prompt queue failed: %v", err)})
			m.chatCacheDirty = true
			return true, nil
		}
		m.messages = append(m.messages, components.Message{Role: "system", Content: fmt.Sprintf("Cleared %d queued prompt(s)", cleared)})
		m.chatCacheDirty = true
		return true, nil
	case "/toolsets", "/tools":
		m.clearInput()
		m.showWelcome = false
		m.messages = append(m.messages, components.Message{Role: "system", Content: formatToolsets(m.toolsets, 20)})
		m.chatCacheDirty = true
		return true, nil
	case "/files":
		m.clearInput()
		m.showWelcome = false
		svc, ok := m.sessionService.(*supermansession.Service)
		if !ok {
			m.messages = append(m.messages, components.Message{Role: "error", Content: "Session files are not available for this session service"})
			m.chatCacheDirty = true
			return true, nil
		}
		changes, err := svc.SessionFileChanges(m.cfg.Session.AppName, "tui-user", m.sessionID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				m.messages = append(m.messages, components.Message{Role: "system", Content: "No files recorded for this session"})
			} else {
				m.messages = append(m.messages, components.Message{Role: "error", Content: fmt.Sprintf("Load session files failed: %v", err)})
			}
			m.chatCacheDirty = true
			return true, nil
		}
		files, err := svc.SessionFiles(m.cfg.Session.AppName, "tui-user", m.sessionID)
		if err != nil {
			m.messages = append(m.messages, components.Message{Role: "error", Content: fmt.Sprintf("Load session files failed: %v", err)})
			m.chatCacheDirty = true
			return true, nil
		}
		m.fileCount = len(files)
		m.messages = append(m.messages, components.Message{Role: "system", Content: formatSessionFileChanges(changes, files, 12)})
		m.chatCacheDirty = true
		return true, nil
	case "/history":
		m.clearInput()
		revisions, err := m.fileRevisions()
		if err != nil {
			m.messages = append(m.messages, components.Message{Role: "error", Content: fmt.Sprintf("Load file history failed: %v", err)})
		} else {
			m.messages = append(m.messages, components.Message{Role: "system", Content: formatFileRevisions(revisions, 12)})
		}
		m.chatCacheDirty = true
		return true, nil
	case "/compact", "/summarize":
		m.clearInput()
		m.showWelcome = false
		compactor := m.compactor()
		if compactor == nil {
			m.messages = append(m.messages, components.Message{Role: "error", Content: "Session compaction is not available for this session service"})
			m.chatCacheDirty = true
			return true, nil
		}
		compacted, count, err := compactor.Compact(m.cfg.Session.AppName, "tui-user", m.sessionID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				m.messages = append(m.messages, components.Message{Role: "system", Content: "Session is already within the context window"})
			} else {
				m.messages = append(m.messages, components.Message{Role: "error", Content: fmt.Sprintf("Compaction failed: %v", err)})
			}
		} else if compacted {
			m.messages = append(m.messages, components.Message{Role: "system", Content: fmt.Sprintf("Session compacted (%d messages summarized)", count)})
		} else {
			m.messages = append(m.messages, components.Message{Role: "system", Content: "Session is already within the context window"})
		}
		m.chatCacheDirty = true
		return true, nil
	}
	if strings.HasPrefix(input, "/switch ") {
		m.clearInput()
		sessionID := strings.TrimSpace(strings.TrimPrefix(input, "/switch "))
		if err := m.switchSession(sessionID); err != nil {
			m.messages = append(m.messages, components.Message{Role: "error", Content: fmt.Sprintf("Switch session failed: %v", err)})
		}
		m.chatCacheDirty = true
		return true, nil
	}
	if strings.HasPrefix(input, "/search ") {
		m.clearInput()
		query := strings.TrimSpace(strings.TrimPrefix(input, "/search "))
		results, err := m.searchMessages(query)
		if err != nil {
			m.messages = append(m.messages, components.Message{Role: "error", Content: fmt.Sprintf("Search session history failed: %v", err)})
		} else if len(results) > 0 {
			m.searchDialog = &searchDialogState{Query: query, Results: results}
			m.messages = append(m.messages, components.Message{Role: "system", Content: formatMessageSearchResults(query, results, 5)})
		} else {
			m.messages = append(m.messages, components.Message{Role: "system", Content: formatMessageSearchResults(query, results, 12)})
		}
		m.chatCacheDirty = true
		return true, nil
	}
	if strings.HasPrefix(input, "/rename ") {
		m.clearInput()
		title := strings.TrimSpace(strings.TrimPrefix(input, "/rename "))
		if err := m.renameSession(title); err != nil {
			m.messages = append(m.messages, components.Message{Role: "error", Content: fmt.Sprintf("Rename session failed: %v", err)})
		} else {
			m.messages = append(m.messages, components.Message{Role: "system", Content: "Session renamed: " + title})
			m.sessionTitle = title
		}
		m.chatCacheDirty = true
		return true, nil
	}
	if strings.HasPrefix(input, "/diff ") {
		m.clearInput()
		path := strings.TrimSpace(strings.TrimPrefix(input, "/diff "))
		revision, ok, err := m.latestFileRevision(path)
		if err != nil {
			m.messages = append(m.messages, components.Message{Role: "error", Content: fmt.Sprintf("Load file diff failed: %v", err)})
		} else if !ok {
			m.messages = append(m.messages, components.Message{Role: "system", Content: "No file history found for " + path})
		} else if diff, err := m.formatFileRevisionDiff(revision); err != nil {
			m.messages = append(m.messages, components.Message{Role: "error", Content: fmt.Sprintf("Load file diff failed: %v", err)})
		} else {
			m.messages = append(m.messages, components.Message{Role: "system", Content: diff})
		}
		m.chatCacheDirty = true
		return true, nil
	}
	if strings.HasPrefix(input, "/revert ") {
		m.clearInput()
		path := strings.TrimSpace(strings.TrimPrefix(input, "/revert "))
		if err := m.revertFile(path); err != nil {
			m.messages = append(m.messages, components.Message{Role: "error", Content: fmt.Sprintf("Revert file failed: %v", err)})
		} else {
			m.messages = append(m.messages, components.Message{Role: "system", Content: "Reverted " + path + " to the latest recorded before snapshot"})
		}
		m.chatCacheDirty = true
		return true, nil
	}
	return false, nil
}
func (m *Model) enqueuePrompt(prompt string) *Model {
	m.clearInput()
	m.historyIndex = -1
	m.historyDraft = ""
	m.showWelcome = false
	queued, err := m.enqueuePromptStore(prompt)
	if err != nil {
		m.messages = append(m.messages, components.Message{Role: "error", Content: fmt.Sprintf("Queue prompt failed: %v", err)})
	} else {
		m.messages = append(m.messages, components.Message{Role: "system", Content: fmt.Sprintf("Queued prompt #%d: %s", m.queueCount, components.TruncateRunes(queued.Content, 120))})
	}
	m.chatCacheDirty = true
	return m
}
func formatPromptQueue(queue []supermansession.QueuedPrompt, limit int) string {
	if len(queue) == 0 {
		return "Prompt queue is empty"
	}
	if limit <= 0 || limit > len(queue) {
		limit = len(queue)
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Prompt queue (%d)", len(queue)))
	for i := 0; i < limit; i++ {
		b.WriteString(fmt.Sprintf("\n%d. %s", i+1, components.TruncateRunes(queue[i].Content, 160)))
	}
	if len(queue) > limit {
		b.WriteString(fmt.Sprintf("\n... %d more", len(queue)-limit))
	}
	return b.String()
}
func formatSessions(sessions []supermansession.Metadata, currentID string, limit int) string {
	if len(sessions) == 0 {
		return "No sessions yet"
	}
	if limit <= 0 || limit > len(sessions) {
		limit = len(sessions)
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Sessions (%d)", len(sessions)))
	for i := 0; i < limit; i++ {
		meta := sessions[i]
		marker := " "
		if meta.SessionID == currentID {
			marker = "*"
		}
		b.WriteString(fmt.Sprintf(
			"\n%s %s  %s  messages:%d files:%d queued:%d",
			marker,
			meta.SessionID,
			components.TruncateRunes(meta.Title, 60),
			meta.MessageCount,
			meta.FileCount,
			meta.QueuedPrompts,
		))
	}
	if len(sessions) > limit {
		b.WriteString(fmt.Sprintf("\n... %d more", len(sessions)-limit))
	}
	return b.String()
}
func formatToolsets(toolsets []supermanagent.ToolsetDescriptor, limit int) string {
	if len(toolsets) == 0 {
		return "No ADK Skill or MCP toolsets configured"
	}
	if limit <= 0 || limit > len(toolsets) {
		limit = len(toolsets)
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("ADK toolsets (%d)", len(toolsets)))
	for i := 0; i < limit; i++ {
		ts := toolsets[i]
		b.WriteString("\n- ")
		b.WriteString(ts.Name)
		b.WriteString(" [")
		b.WriteString(ts.Kind)
		b.WriteString("]")
		if len(ts.Tools) > 0 {
			b.WriteString(" tools:")
			b.WriteString(strings.Join(ts.Tools, ","))
		}
		if ts.Source != "" {
			b.WriteString("  ")
			b.WriteString(components.TruncateRunes(ts.Source, 120))
		}
	}
	if len(toolsets) > limit {
		b.WriteString(fmt.Sprintf("\n- ... %d more", len(toolsets)-limit))
	}
	return b.String()
}
func formatMessageSearchResults(query string, results []supermansession.MessageSearchResult, limit int) string {
	query = strings.TrimSpace(query)
	if len(results) == 0 {
		if query == "" {
			return "No matching messages"
		}
		return "No matching messages for " + query
	}
	if limit <= 0 || limit > len(results) {
		limit = len(results)
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Session search (%d)", len(results)))
	if query != "" {
		b.WriteString(": ")
		b.WriteString(components.TruncateRunes(query, 80))
	}
	for i := 0; i < limit; i++ {
		result := results[i]
		b.WriteString(fmt.Sprintf(
			"\n- %s  %s  %s",
			result.Metadata.SessionID,
			result.Message.Role,
			components.TruncateRunes(result.Metadata.Title, 44),
		))
		preview := strings.Join(strings.Fields(result.Preview), " ")
		if preview != "" {
			b.WriteString("\n  ")
			b.WriteString(components.TruncateRunes(preview, 180))
		}
	}
	if len(results) > limit {
		b.WriteString(fmt.Sprintf("\n- ... %d more", len(results)-limit))
	}
	return b.String()
}
func formatFileRevisions(revisions []supermansession.FileRevision, limit int) string {
	if len(revisions) == 0 {
		return "No file history recorded for this session"
	}
	if limit <= 0 || limit > len(revisions) {
		limit = len(revisions)
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("File history (%d)", len(revisions)))
	start := len(revisions) - limit
	for i := len(revisions) - 1; i >= start; i-- {
		rev := revisions[i]
		b.WriteString(fmt.Sprintf(
			"\n- %s  %s  %d -> %d bytes",
			rev.Action,
			rev.Path,
			rev.Before.Size,
			rev.After.Size,
		))
		if rev.Before.Missing {
			b.WriteString(" (created)")
		}
	}
	if len(revisions) > limit {
		b.WriteString(fmt.Sprintf("\n- ... %d older", len(revisions)-limit))
	}
	return b.String()
}
