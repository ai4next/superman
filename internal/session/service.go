package session

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ai4next/superman/internal/global"
	persistdb "github.com/ai4next/superman/internal/store/db"
	persistfs "github.com/ai4next/superman/internal/store/fs"
	"github.com/google/uuid"
	adksession "google.golang.org/adk/session"
	"gorm.io/gorm"
)

type Service struct {
	mu         sync.RWMutex
	db         *persistdb.DB
	rootDir    string
	sessionDir string
	sessions   map[string]*storedSession
	appState   map[string]map[string]any
	userState  map[string]map[string]map[string]any
	subs       map[chan Event[Message]]struct{}
}

func NewService() (adksession.Service, error) {
	return NewServiceInRoot("")
}

func NewServiceInRoot(rootDir string) (adksession.Service, error) {
	agentName := agentNameFromRoot(rootDir)
	if rootDir == "" {
		rootDir = global.AgentStateDir(agentName)
	}
	s := &Service{
		rootDir:    rootDir,
		sessionDir: global.AgentSessionsDir(agentName),
		sessions:   make(map[string]*storedSession),
		appState:   make(map[string]map[string]any),
		userState:  make(map[string]map[string]map[string]any),
		subs:       make(map[chan Event[Message]]struct{}),
	}
	if err := os.MkdirAll(s.sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}
	registry, err := global.DBRegistry()
	if err != nil {
		return nil, err
	}
	s.db, err = registry.EnsureAgentDB(agentName, global.AgentStateDBPath(agentName))
	if err != nil {
		return nil, err
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func agentNameFromRoot(rootDir string) string {
	if rootDir == "" || rootDir == global.Config().Workspace {
		return "superman"
	}
	return filepath.Base(rootDir)
}

func (s *Service) sessionLogPath(sessionID string) string {
	return filepath.Join(s.sessionDir, sessionID+".log")
}

func (s *Service) Create(ctx context.Context, req *adksession.CreateRequest) (*adksession.CreateResponse, error) {
	if req.AppName == "" || req.UserID == "" {
		return nil, fmt.Errorf("app_name and user_id are required")
	}
	requestedID := parseOptionalSessionID(req.SessionID)
	sessionID := formatSessionID(requestedID)
	now := time.Now()
	stored := &storedSession{
		ID:        requestedID,
		AppName:   req.AppName,
		UserID:    req.UserID,
		SessionID: sessionID,
		Title:     defaultTitle(sessionID),
		State:     sessionState(req.State),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if stored.State == nil {
		stored.State = make(map[string]any)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if requestedID > 0 {
		key := sessionKey(req.AppName, req.UserID, sessionID)
		if _, ok := s.sessions[key]; ok {
			return nil, fmt.Errorf("session %s already exists", sessionID)
		}
	}
	s.applyScopedStateLocked(req.AppName, req.UserID, req.State)
	if err := s.persistLocked(stored); err != nil {
		return nil, err
	}
	stored.SessionID = formatSessionID(stored.ID)
	if _, ok := s.sessions[sessionKey(req.AppName, req.UserID, stored.SessionID)]; ok {
		return nil, fmt.Errorf("session %s already exists", sessionID)
	}
	s.sessions[sessionKey(req.AppName, req.UserID, stored.SessionID)] = stored
	return &adksession.CreateResponse{Session: stored.snapshot(s.mergedStateLocked(stored))}, nil
}

func (s *Service) Get(ctx context.Context, req *adksession.GetRequest) (*adksession.GetResponse, error) {
	sessionID, err := normalizeSessionID(req.SessionID)
	if err != nil {
		return nil, err
	}
	if req.AppName == "" || req.UserID == "" || sessionID == "" {
		return nil, fmt.Errorf("app_name, user_id, and session_id are required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	stored, ok := s.sessions[sessionKey(req.AppName, req.UserID, sessionID)]
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}
	snap := stored.snapshot(s.mergedStateLocked(stored))
	snap.events = filterEvents(snap.events, req.NumRecentEvents, req.After)
	return &adksession.GetResponse{Session: snap}, nil
}

func (s *Service) List(ctx context.Context, req *adksession.ListRequest) (*adksession.ListResponse, error) {
	if req.AppName == "" {
		return nil, fmt.Errorf("app_name is required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]adksession.Session, 0)
	for _, stored := range s.sessions {
		if stored.AppName != req.AppName {
			continue
		}
		if req.UserID != "" && stored.UserID != req.UserID {
			continue
		}
		snap := stored.snapshot(s.mergedStateLocked(stored))
		snap.events = nil
		out = append(out, snap)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].LastUpdateTime().After(out[j].LastUpdateTime())
	})
	return &adksession.ListResponse{Sessions: out}, nil
}

func (s *Service) Delete(ctx context.Context, req *adksession.DeleteRequest) error {
	sessionID, err := normalizeSessionID(req.SessionID)
	if err != nil {
		return err
	}
	if req.AppName == "" || req.UserID == "" || sessionID == "" {
		return fmt.Errorf("app_name, user_id, and session_id are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := sessionKey(req.AppName, req.UserID, sessionID)
	if stored, ok := s.sessions[key]; ok {
		for _, msg := range stored.Messages {
			s.publishLocked(DeletedEvent, msg)
		}
	}
	delete(s.sessions, key)
	path := s.sessionLogPath(sessionID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	if s.db != nil {
		id, _ := strconv.ParseInt(sessionID, 10, 64)
		return s.db.DeleteSession(id, req.AppName, req.UserID)
	}
	return nil
}

func (s *Service) AppendEvent(ctx context.Context, curSession adksession.Session, event *adksession.Event) error {
	if curSession == nil {
		return fmt.Errorf("session is nil")
	}
	if event == nil {
		return fmt.Errorf("event is nil")
	}
	if event.Partial {
		return nil
	}
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	key := sessionKey(curSession.AppName(), curSession.UserID(), curSession.ID())
	stored, ok := s.sessions[key]
	if !ok {
		return fmt.Errorf("session %s not found", curSession.ID())
	}
	eventCopy := cloneEvent(event)
	s.applyScopedStateLocked(stored.AppName, stored.UserID, eventCopy.Actions.StateDelta)
	applySessionStateDelta(stored.State, eventCopy.Actions.StateDelta)
	applySessionMetadataDelta(stored, eventCopy.Actions.StateDelta)
	if err := s.applyContextRecordsLocked(stored, eventCopy.Actions.StateDelta); err != nil {
		return err
	}
	stored.Events = append(stored.Events, eventCopy)
	messages := projectEvent(stored.SessionID, eventCopy)
	for _, msg := range messages {
		s.upsertMessageLocked(stored, msg)
	}
	updateUsage(stored, eventCopy)
	stored.UpdatedAt = eventCopy.Timestamp
	if err := s.persistLocked(stored); err != nil {
		return err
	}
	if view, ok := curSession.(*sessionView); ok {
		view.appendEvent(eventCopy, s.mergedStateLocked(stored), stored.UpdatedAt)
	}
	for _, msg := range messages {
		s.publishLocked(UpdatedEvent, msg)
	}
	return nil
}

func (s *Service) Messages(appName, userID, sessionID string) ([]Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stored, ok := s.sessions[sessionKey(appName, userID, sessionID)]
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}
	out := make([]Message, len(stored.Messages))
	copy(out, stored.Messages)
	return out, nil
}

func (s *Service) PromptHistory(appName, userID, sessionID string, limit int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stored, ok := s.sessions[sessionKey(appName, userID, sessionID)]
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for i := len(stored.Messages) - 1; i >= 0; i-- {
		msg := stored.Messages[i]
		if msg.Role != MessageUser {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		if _, ok := seen[content]; ok {
			continue
		}
		seen[content] = struct{}{}
		out = append(out, content)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (s *Service) SearchMessages(appName, userID string, opts MessageSearchOptions) ([]MessageSearchResult, error) {
	query := strings.TrimSpace(opts.Query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	roleFilter := make(map[MessageRole]struct{}, len(opts.Roles))
	for _, role := range opts.Roles {
		if role == "" {
			continue
		}
		roleFilter[role] = struct{}{}
	}
	needle := strings.ToLower(query)

	s.mu.RLock()
	defer s.mu.RUnlock()
	results := make([]MessageSearchResult, 0)
	for _, stored := range s.sessions {
		if stored.AppName != appName {
			continue
		}
		if userID != "" && stored.UserID != userID {
			continue
		}
		if opts.SessionID != "" && stored.SessionID != opts.SessionID {
			continue
		}
		meta := metadata(stored)
		for _, msg := range stored.Messages {
			if len(roleFilter) > 0 {
				if _, ok := roleFilter[msg.Role]; !ok {
					continue
				}
			}
			haystack := messageSearchText(msg)
			if !strings.Contains(strings.ToLower(haystack), needle) {
				continue
			}
			results = append(results, MessageSearchResult{
				Metadata: meta,
				Message:  msg,
				Preview:  matchPreview(haystack, query, 180),
			})
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Message.CreatedAt.After(results[j].Message.CreatedAt)
	})
	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}
	return results, nil
}

func (s *Service) RecordSessionReference(appName, userID, sessionID string, ref SessionReference) error {
	ref.SessionID = strings.TrimSpace(ref.SessionID)
	if ref.SessionID == "" {
		return fmt.Errorf("referenced session id is required")
	}
	ref.Preview = compactPreview(ref.Preview, 240)
	now := time.Now()
	if ref.CreatedAt.IsZero() {
		ref.CreatedAt = now
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := s.sessions[sessionKey(appName, userID, sessionID)]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}
	for i := len(stored.References) - 1; i >= 0; i-- {
		existing := stored.References[i]
		if existing.SessionID == ref.SessionID && existing.Role == ref.Role && existing.Preview == ref.Preview {
			stored.References[i].CreatedAt = ref.CreatedAt
			stored.UpdatedAt = now
			return s.persistLocked(stored)
		}
	}
	stored.References = append(stored.References, ref)
	stored.UpdatedAt = now
	return s.persistLocked(stored)
}

func (s *Service) SessionReferences(appName, userID, sessionID string) ([]SessionReference, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stored, ok := s.sessions[sessionKey(appName, userID, sessionID)]
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}
	return recentSessionReferences(stored.References, 0), nil
}

func (s *Service) RecordFileRead(appName, userID, sessionID, path string) error {
	return s.recordFile(appName, userID, sessionID, path, FileRead)
}

func (s *Service) RecordFileWrite(appName, userID, sessionID, path string) error {
	return s.recordFile(appName, userID, sessionID, path, FileWritten)
}

func (s *Service) RecordFileRevision(appName, userID, sessionID, path, action, before, after string, beforeMissing bool) (FileRevision, error) {
	return s.RecordFileRevisionWithMissing(appName, userID, sessionID, path, action, before, after, beforeMissing, false)
}

func (s *Service) RecordFileRevisionWithMissing(appName, userID, sessionID, path, action, before, after string, beforeMissing, afterMissing bool) (FileRevision, error) {
	if err := s.writeSnapshotContent(before, beforeMissing); err != nil {
		return FileRevision{}, err
	}
	if err := s.writeSnapshotContent(after, afterMissing); err != nil {
		return FileRevision{}, err
	}
	revision, err := buildFileRevision(path, action, before, after, beforeMissing, afterMissing)
	if err != nil {
		return FileRevision{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := s.sessions[sessionKey(appName, userID, sessionID)]
	if !ok {
		return FileRevision{}, fmt.Errorf("session %s not found", sessionID)
	}
	stored.FileRevisions = append(stored.FileRevisions, revision)
	if stored.Files == nil {
		stored.Files = make(map[string]SessionFile)
	}
	file := stored.Files[revision.Path]
	file.Path = revision.Path
	file.LastAccess = FileWritten
	file.WrittenAt = revision.CreatedAt
	file.WriteCount++
	stored.Files[revision.Path] = file
	stored.UpdatedAt = revision.CreatedAt
	if err := s.persistLocked(stored); err != nil {
		return FileRevision{}, err
	}
	return revision, nil
}

func buildFileRevision(path, action, before, after string, beforeMissing, afterMissing bool) (FileRevision, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return FileRevision{}, fmt.Errorf("invalid path: %w", err)
	}
	revision := FileRevision{
		ID:        uuid.NewString(),
		Path:      abs,
		Action:    strings.TrimSpace(action),
		Before:    snapshotContent(before, beforeMissing),
		After:     snapshotContent(after, afterMissing),
		CreatedAt: time.Now(),
	}
	if revision.Action == "" {
		revision.Action = "write"
	}
	return revision, nil
}

func (s *Service) FileRevisions(appName, userID, sessionID string) ([]FileRevision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stored, ok := s.sessions[sessionKey(appName, userID, sessionID)]
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}
	out := make([]FileRevision, len(stored.FileRevisions))
	copy(out, stored.FileRevisions)
	return out, nil
}

func (s *Service) SessionFileChanges(appName, userID, sessionID string) ([]FileChangeSummary, error) {
	s.mu.RLock()
	stored, ok := s.sessions[sessionKey(appName, userID, sessionID)]
	if !ok {
		s.mu.RUnlock()
		return nil, fmt.Errorf("session %s not found", sessionID)
	}
	files := make(map[string]SessionFile, len(stored.Files))
	maps.Copy(files, stored.Files)
	revisions := make([]FileRevision, len(stored.FileRevisions))
	copy(revisions, stored.FileRevisions)
	s.mu.RUnlock()

	fileList := make([]SessionFile, 0, len(files))
	for _, file := range files {
		fileList = append(fileList, file)
	}
	return fileChangeSummaries(fileList, revisions)
}

func (s *Service) FileSnapshotContent(snapshot FileSnapshot) (string, bool, error) {
	return FileSnapshotContent(snapshot)
}

func (s *Service) StorageStats() (StorageStats, error) {
	s.mu.RLock()
	sessionRefs := s.referencedSnapshotHashesLocked()
	stats := StorageStats{RootDir: s.sessionDir}
	for _, stored := range s.sessions {
		stats.Sessions++
		stats.Messages += len(stored.Messages)
		stats.Files += len(stored.Files)
		stats.FileRevisions += len(stored.FileRevisions)
		stats.PromptQueue += len(stored.PromptQueue)
		stats.References += len(stored.References)
		if info, err := os.Stat(s.sessionLogPath(stored.SessionID)); err == nil {
			stats.SessionBytes += info.Size()
		} else if !os.IsNotExist(err) {
			s.mu.RUnlock()
			return StorageStats{}, err
		}
	}
	s.mu.RUnlock()

	snapshots, err := s.snapshotFiles()
	if err != nil {
		return StorageStats{}, err
	}
	for _, snapshot := range snapshots {
		stats.SnapshotCount++
		stats.SnapshotBytes += snapshot.Size
		if _, ok := sessionRefs[snapshot.Hash]; ok {
			stats.ReferencedSnapshotCount++
			stats.ReferencedSnapshotBytes += snapshot.Size
		} else {
			stats.OrphanSnapshotCount++
			stats.OrphanSnapshotBytes += snapshot.Size
		}
	}
	return stats, nil
}

func (s *Service) CleanupOrphanSnapshots(dryRun bool) (SnapshotCleanupResult, error) {
	s.mu.RLock()
	referenced := s.referencedSnapshotHashesLocked()
	s.mu.RUnlock()

	snapshots, err := s.snapshotFiles()
	if err != nil {
		return SnapshotCleanupResult{}, err
	}
	result := SnapshotCleanupResult{DryRun: dryRun}
	for _, snapshot := range snapshots {
		if _, ok := referenced[snapshot.Hash]; ok {
			result.Kept++
			result.KeptBytes += snapshot.Size
			continue
		}
		result.Orphans = append(result.Orphans, snapshot)
		result.Removed++
		result.RemovedBytes += snapshot.Size
		if !dryRun {
			if err := os.Remove(snapshot.Path); err != nil && !os.IsNotExist(err) {
				return SnapshotCleanupResult{}, err
			}
		}
	}
	if !dryRun {
		result.Orphans = nil
		_ = s.removeEmptySnapshotDirs()
	}
	return result, nil
}

func (s *Service) EnqueuePrompt(appName, userID, sessionID, content string) (QueuedPrompt, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return QueuedPrompt{}, fmt.Errorf("prompt content is required")
	}
	now := time.Now()
	prompt := QueuedPrompt{
		ID:        uuid.NewString(),
		Content:   content,
		CreatedAt: now,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := s.sessions[sessionKey(appName, userID, sessionID)]
	if !ok {
		return QueuedPrompt{}, fmt.Errorf("session %s not found", sessionID)
	}
	stored.PromptQueue = append(stored.PromptQueue, prompt)
	stored.setSessionState(sessionStatePromptQueue, stored.PromptQueue)
	stored.UpdatedAt = now
	if err := s.persistLocked(stored); err != nil {
		return QueuedPrompt{}, err
	}
	return prompt, nil
}

func (s *Service) DequeuePrompt(appName, userID, sessionID string) (QueuedPrompt, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := s.sessions[sessionKey(appName, userID, sessionID)]
	if !ok {
		return QueuedPrompt{}, false, fmt.Errorf("session %s not found", sessionID)
	}
	if len(stored.PromptQueue) == 0 {
		return QueuedPrompt{}, false, nil
	}
	prompt := stored.PromptQueue[0]
	copy(stored.PromptQueue, stored.PromptQueue[1:])
	stored.PromptQueue = stored.PromptQueue[:len(stored.PromptQueue)-1]
	stored.setSessionState(sessionStatePromptQueue, stored.PromptQueue)
	stored.UpdatedAt = time.Now()
	if err := s.persistLocked(stored); err != nil {
		return QueuedPrompt{}, false, err
	}
	return prompt, true, nil
}

func (s *Service) PromptQueue(appName, userID, sessionID string) ([]QueuedPrompt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stored, ok := s.sessions[sessionKey(appName, userID, sessionID)]
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}
	out := make([]QueuedPrompt, len(stored.PromptQueue))
	copy(out, stored.PromptQueue)
	return out, nil
}

func (s *Service) ClearPromptQueue(appName, userID, sessionID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := s.sessions[sessionKey(appName, userID, sessionID)]
	if !ok {
		return 0, fmt.Errorf("session %s not found", sessionID)
	}
	count := len(stored.PromptQueue)
	if count == 0 {
		return 0, nil
	}
	stored.PromptQueue = nil
	stored.setSessionState(sessionStatePromptQueue, []QueuedPrompt{})
	stored.UpdatedAt = time.Now()
	if err := s.persistLocked(stored); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Service) SessionFiles(appName, userID, sessionID string) ([]SessionFile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stored, ok := s.sessions[sessionKey(appName, userID, sessionID)]
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}
	out := make([]SessionFile, 0, len(stored.Files))
	for _, file := range stored.Files {
		out = append(out, file)
	}
	sort.Slice(out, func(i, j int) bool {
		return fileLastAccess(out[i]).After(fileLastAccess(out[j]))
	})
	return out, nil
}

func (s *Service) LastFileRead(appName, userID, sessionID, path string) (time.Time, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid path: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	stored, ok := s.sessions[sessionKey(appName, userID, sessionID)]
	if !ok {
		return time.Time{}, fmt.Errorf("session %s not found", sessionID)
	}
	file, ok := stored.Files[abs]
	if !ok {
		return time.Time{}, nil
	}
	return file.ReadAt, nil
}

func (s *Service) recordFile(appName, userID, sessionID, path string, access FileAccess) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := s.sessions[sessionKey(appName, userID, sessionID)]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}
	if stored.Files == nil {
		stored.Files = make(map[string]SessionFile)
	}
	file := stored.Files[abs]
	file.Path = abs
	file.LastAccess = access
	switch access {
	case FileRead:
		file.ReadAt = now
		file.ReadCount++
	case FileWritten:
		file.WrittenAt = now
		file.WriteCount++
	}
	stored.Files[abs] = file
	stored.UpdatedAt = now
	return s.persistLocked(stored)
}

func (s *Service) applyContextRecordsLocked(stored *storedSession, stateDelta map[string]any) error {
	records := contextRecords(&adksession.EventActions{StateDelta: stateDelta})
	now := time.Now()
	for _, path := range records.FileReads {
		if err := applyFileRecord(stored, path, FileRead, now); err != nil {
			return err
		}
	}
	for _, path := range records.FileWrites {
		if err := applyFileRecord(stored, path, FileWritten, now); err != nil {
			return err
		}
	}
	for _, note := range records.FileRevisions {
		revision, err := buildFileRevision(note.Path, note.Action, note.Before, note.After, note.BeforeMissing, note.AfterMissing)
		if err != nil {
			return err
		}
		stored.FileRevisions = append(stored.FileRevisions, revision)
		if stored.Files == nil {
			stored.Files = make(map[string]SessionFile)
		}
		file := stored.Files[revision.Path]
		file.Path = revision.Path
		file.WrittenAt = revision.CreatedAt
		file.WriteCount++
		file.LastAccess = FileWritten
		stored.Files[revision.Path] = file
		if err := s.writeSnapshotContent(note.Before, note.BeforeMissing); err != nil {
			return err
		}
		if err := s.writeSnapshotContent(note.After, note.AfterMissing); err != nil {
			return err
		}
	}
	for _, ref := range records.References {
		upsertSessionReference(stored, ref, now)
	}
	return nil
}

func applyFileRecord(stored *storedSession, path string, access FileAccess, now time.Time) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	if stored.Files == nil {
		stored.Files = make(map[string]SessionFile)
	}
	file := stored.Files[abs]
	file.Path = abs
	file.LastAccess = access
	switch access {
	case FileRead:
		file.ReadAt = now
		file.ReadCount++
	case FileWritten:
		file.WrittenAt = now
		file.WriteCount++
	}
	stored.Files[abs] = file
	return nil
}

func upsertSessionReference(stored *storedSession, ref SessionReference, now time.Time) {
	ref.SessionID = strings.TrimSpace(ref.SessionID)
	if ref.SessionID == "" {
		return
	}
	ref.Preview = compactPreview(ref.Preview, 240)
	if ref.CreatedAt.IsZero() {
		ref.CreatedAt = now
	}
	for i := len(stored.References) - 1; i >= 0; i-- {
		existing := stored.References[i]
		if existing.SessionID == ref.SessionID && existing.Role == ref.Role && existing.Preview == ref.Preview {
			stored.References[i].CreatedAt = ref.CreatedAt
			return
		}
	}
	stored.References = append(stored.References, ref)
}

func (s *Service) Subscribe(ctx context.Context) <-chan Event[Message] {
	ch := make(chan Event[Message], 256)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()
	go func() {
		<-ctx.Done()
		s.mu.Lock()
		delete(s.subs, ch)
		close(ch)
		s.mu.Unlock()
	}()
	return ch
}

func (s *Service) Metadata(appName, userID, sessionID string) (Metadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stored, ok := s.sessions[sessionKey(appName, userID, sessionID)]
	if !ok {
		return Metadata{}, fmt.Errorf("session %s not found", sessionID)
	}
	return metadata(stored), nil
}

func (s *Service) ListMetadata(appName, userID string) []Metadata {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Metadata, 0)
	for _, stored := range s.sessions {
		if stored.AppName != appName {
			continue
		}
		if userID != "" && stored.UserID != userID {
			continue
		}
		out = append(out, metadata(stored))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

func (s *Service) SetSummary(appName, userID, sessionID, summary string) (Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := s.sessions[sessionKey(appName, userID, sessionID)]
	if !ok {
		return Message{}, fmt.Errorf("session %s not found", sessionID)
	}
	now := time.Now()
	msg := Message{
		ID:        firstNonEmpty(stored.SummaryMessageID, uuid.NewString()),
		SessionID: sessionID,
		Role:      MessageAssistant,
		Content:   summary,
		Summary:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	stored.SummaryMessageID = msg.ID
	stored.setSessionState(sessionStateSummaryMessageID, msg.ID)
	s.upsertMessageLocked(stored, msg)
	stored.UpdatedAt = now
	if err := s.persistLocked(stored); err != nil {
		return Message{}, err
	}
	s.publishLocked(UpdatedEvent, msg)
	return msg, nil
}

func (s *Service) Rename(appName, userID, sessionID, title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := s.sessions[sessionKey(appName, userID, sessionID)]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}
	stored.Title = strings.TrimSpace(title)
	stored.setSessionState(sessionStateTitle, stored.Title)
	stored.UpdatedAt = time.Now()
	return s.persistLocked(stored)
}

func (s *Service) Import(appName, userID string, data ImportData) (Metadata, error) {
	if appName == "" || userID == "" {
		return Metadata{}, fmt.Errorf("app_name and user_id are required")
	}
	sessionID, err := normalizeSessionID(data.Metadata.SessionID)
	if err != nil {
		return Metadata{}, err
	}
	sessionIntID, _ := strconv.ParseInt(sessionID, 10, 64)
	now := time.Now()
	stored := &storedSession{
		ID:               sessionIntID,
		AppName:          appName,
		UserID:           userID,
		SessionID:        sessionID,
		Title:            strings.TrimSpace(data.Metadata.Title),
		Messages:         normalizeImportedMessages(sessionID, data.Messages, now),
		Files:            normalizeImportedFiles(data.Files),
		FileRevisions:    normalizeImportedRevisions(data.FileRevisions, now),
		PromptQueue:      normalizeImportedQueue(data.PromptQueue, now),
		References:       normalizeImportedReferences(data.References, now),
		PromptTokens:     data.Metadata.PromptTokens,
		CompletionTokens: data.Metadata.CompletionTokens,
		SummaryMessageID: data.Metadata.SummaryMessageID,
		CreatedAt:        data.Metadata.CreatedAt,
		UpdatedAt:        data.Metadata.UpdatedAt,
	}
	if stored.Title == "" {
		stored.Title = defaultTitle(sessionID)
	}
	if stored.CreatedAt.IsZero() {
		stored.CreatedAt = now
	}
	if stored.UpdatedAt.IsZero() {
		stored.UpdatedAt = now
	}
	if stored.Files == nil {
		stored.Files = make(map[string]SessionFile)
	}
	if stored.SummaryMessageID == "" {
		for _, msg := range stored.Messages {
			if msg.Summary {
				stored.SummaryMessageID = msg.ID
				break
			}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	key := sessionKey(appName, userID, sessionID)
	if _, ok := s.sessions[key]; ok && !data.Overwrite {
		return Metadata{}, fmt.Errorf("session %s already exists", sessionID)
	}
	s.sessions[key] = stored
	if err := s.persistLocked(stored); err != nil {
		delete(s.sessions, key)
		return Metadata{}, err
	}
	stored.SessionID = formatSessionID(stored.ID)
	if stored.SessionID != sessionID {
		delete(s.sessions, key)
		s.sessions[sessionKey(appName, userID, stored.SessionID)] = stored
	}
	return metadata(stored), nil
}

func (s *Service) load() error {
	if s.db != nil {
		return s.loadFromDB()
	}
	entries, err := os.ReadDir(s.sessionDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read session dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.sessionDir, entry.Name()))
		if err != nil {
			return err
		}
		var stored storedSession
		if err := json.Unmarshal(data, &stored); err != nil {
			return fmt.Errorf("decode %s: %w", entry.Name(), err)
		}
		if stored.State == nil {
			stored.State = make(map[string]any)
		}
		if stored.Files == nil {
			stored.Files = make(map[string]SessionFile)
		}
		s.applyScopedStateLocked(stored.AppName, stored.UserID, stored.State)
		s.sessions[sessionKey(stored.AppName, stored.UserID, stored.SessionID)] = &stored
	}
	return nil
}

func (s *Service) loadFromDB() error {
	rows, err := s.db.LoadSessions()
	if err != nil {
		return err
	}
	for _, row := range rows {
		stored := storedSession{
			ID:               int64(row.ID),
			AppName:          row.AppName,
			UserID:           row.UserID,
			SessionID:        formatSessionID(int64(row.ID)),
			Title:            row.Title,
			PromptTokens:     row.PromptTokens,
			CompletionTokens: row.CompletionTokens,
			SummaryMessageID: row.SummaryMessageID,
			CreatedAt:        row.CreatedAt,
			UpdatedAt:        row.UpdatedAt,
		}
		if row.StateJSON != "" {
			if err := json.Unmarshal([]byte(row.StateJSON), &stored.State); err != nil {
				return fmt.Errorf("decode session state %s: %w", stored.SessionID, err)
			}
		}
		if err := s.loadMessagesFromDB(&stored); err != nil {
			return err
		}
		if err := s.loadSessionDetailsFromDB(&stored); err != nil {
			return err
		}
		if stored.State == nil {
			stored.State = make(map[string]any)
		}
		if stored.Files == nil {
			stored.Files = make(map[string]SessionFile)
		}
		s.applyScopedStateLocked(stored.AppName, stored.UserID, stored.State)
		s.sessions[sessionKey(stored.AppName, stored.UserID, stored.SessionID)] = &stored
	}
	return nil
}

func (s *Service) loadMessagesFromDB(stored *storedSession) error {
	rows, err := s.db.LoadMessages(stored.ID)
	if err != nil {
		return fmt.Errorf("load messages for session %s: %w", stored.SessionID, err)
	}
	if len(rows) == 0 {
		return nil
	}
	stored.Messages = make([]Message, 0, len(rows))
	for _, row := range rows {
		stored.Messages = append(stored.Messages, Message{
			ID:           row.MessageID,
			SessionID:    stored.SessionID,
			EventID:      row.EventID,
			InvocationID: row.InvocationID,
			Role:         MessageRole(row.Role),
			Content:      row.Content,
			ToolName:     row.ToolName,
			ToolID:       row.ToolID,
			Args:         row.Args,
			Result:       row.Result,
			Status:       row.Status,
			Summary:      row.Summary,
			CreatedAt:    row.CreatedAt,
			UpdatedAt:    row.UpdatedAt,
		})
	}
	return nil
}

func (s *Service) loadSessionDetailsFromDB(stored *storedSession) error {
	files, revisions, queue, refs, err := s.db.LoadDetails(stored.ID)
	if err != nil {
		return err
	}
	if len(files) > 0 {
		stored.Files = make(map[string]SessionFile, len(files))
		for _, row := range files {
			file := SessionFile{
				Path:       row.Path,
				ReadCount:  row.ReadCount,
				WriteCount: row.WriteCount,
				LastAccess: FileAccess(row.LastAccess),
			}
			file.ReadAt, _ = parseStoredTime(row.ReadAt)
			file.WrittenAt, _ = parseStoredTime(row.WrittenAt)
			stored.Files[file.Path] = file
		}
	}
	for _, row := range revisions {
		var revision FileRevision
		if err := json.Unmarshal([]byte(row.DataJSON), &revision); err != nil {
			return err
		}
		stored.FileRevisions = append(stored.FileRevisions, revision)
	}
	for _, row := range queue {
		prompt := QueuedPrompt{ID: row.PromptID, Content: row.Content, CreatedAt: row.CreatedAt}
		stored.PromptQueue = append(stored.PromptQueue, prompt)
	}
	for _, row := range refs {
		ref := SessionReference{SessionID: row.RefID, Role: MessageRole(row.Role), Preview: row.Preview, CreatedAt: row.CreatedAt}
		stored.References = append(stored.References, ref)
	}
	return nil
}

func (s *Service) applyScopedStateLocked(appName, userID string, state map[string]any) {
	if len(state) == 0 {
		return
	}
	for k, v := range state {
		switch {
		case strings.HasPrefix(k, adksession.KeyPrefixApp):
			if s.appState[appName] == nil {
				s.appState[appName] = make(map[string]any)
			}
			s.appState[appName][k] = v
		case strings.HasPrefix(k, adksession.KeyPrefixUser):
			if s.userState[appName] == nil {
				s.userState[appName] = make(map[string]map[string]any)
			}
			if s.userState[appName][userID] == nil {
				s.userState[appName][userID] = make(map[string]any)
			}
			s.userState[appName][userID][k] = v
		}
	}
}

func (s *Service) mergedStateLocked(stored *storedSession) map[string]any {
	state := maps.Clone(stored.State)
	if state == nil {
		state = make(map[string]any)
	}
	maps.Copy(state, s.appState[stored.AppName])
	if users := s.userState[stored.AppName]; users != nil {
		maps.Copy(state, users[stored.UserID])
	}
	return state
}

func (s *Service) persistLocked(stored *storedSession) error {
	if err := os.MkdirAll(s.sessionDir, 0755); err != nil {
		return err
	}
	if s.db != nil {
		if err := s.persistMetadataLocked(stored); err != nil {
			return err
		}
		if err := s.persistMessagesLocked(stored); err != nil {
			return err
		}
		if err := s.persistDetailsLocked(stored); err != nil {
			return err
		}
	}
	if s.db != nil {
		return persistfs.WriteSessionLogPath(s.sessionLogPath(stored.SessionID), logMessages(stored.Messages))
	}
	return nil
}

func (s *Service) persistMetadataLocked(stored *storedSession) error {
	stateJSON, err := json.Marshal(stored.State)
	if err != nil {
		return err
	}
	row, err := s.db.SaveSession(persistdb.SessionRow{
		Model:            gorm.Model{ID: uint(stored.ID), CreatedAt: stored.CreatedAt, UpdatedAt: stored.UpdatedAt},
		AppName:          stored.AppName,
		UserID:           stored.UserID,
		Title:            stored.Title,
		StateJSON:        string(stateJSON),
		PromptTokens:     stored.PromptTokens,
		CompletionTokens: stored.CompletionTokens,
		SummaryMessageID: stored.SummaryMessageID,
		FileCount:        len(stored.Files),
		MessageCount:     len(stored.Messages),
		QueuedPrompts:    len(stored.PromptQueue),
	})
	if err != nil {
		return err
	}
	if stored.ID == 0 {
		stored.ID = int64(row.ID)
		stored.SessionID = formatSessionID(stored.ID)
	}
	return nil
}

func (s *Service) persistMessagesLocked(stored *storedSession) error {
	rows := make([]persistdb.MessageRow, 0, len(stored.Messages))
	for i, msg := range stored.Messages {
		id := msg.ID
		if id == "" {
			id = uuid.NewString()
			stored.Messages[i].ID = id
		}
		rows = append(rows, persistdb.MessageRow{
			Model:        gorm.Model{CreatedAt: msg.CreatedAt, UpdatedAt: msg.UpdatedAt},
			MessageID:    id,
			Position:     i,
			EventID:      msg.EventID,
			InvocationID: msg.InvocationID,
			Role:         string(msg.Role),
			Content:      msg.Content,
			ToolName:     msg.ToolName,
			ToolID:       msg.ToolID,
			Args:         msg.Args,
			Result:       msg.Result,
			Status:       msg.Status,
			Summary:      msg.Summary,
		})
	}
	return s.db.SaveMessages(stored.ID, rows)
}

func (s *Service) persistDetailsLocked(stored *storedSession) error {
	files := recentSessionFiles(stored.Files, 0)
	fileRows := make([]persistdb.FileRow, 0, len(files))
	for _, file := range files {
		fileRows = append(fileRows, persistdb.FileRow{
			Path:       file.Path,
			ReadAt:     formatStoredTime(file.ReadAt),
			WrittenAt:  formatStoredTime(file.WrittenAt),
			ReadCount:  file.ReadCount,
			WriteCount: file.WriteCount,
			LastAccess: string(file.LastAccess),
		})
	}
	revisionRows := make([]persistdb.FileRevisionRow, 0, len(stored.FileRevisions))
	for i, revision := range stored.FileRevisions {
		data, err := json.Marshal(revision)
		if err != nil {
			return err
		}
		revisionRows = append(revisionRows, persistdb.FileRevisionRow{
			Model:      gorm.Model{CreatedAt: revision.CreatedAt, UpdatedAt: revision.CreatedAt},
			RevisionID: revision.ID,
			Position:   i,
			Path:       revision.Path,
			Action:     revision.Action,
			DataJSON:   string(data),
		})
	}
	queueRows := make([]persistdb.QueuedPromptRow, 0, len(stored.PromptQueue))
	for i, prompt := range stored.PromptQueue {
		queueRows = append(queueRows, persistdb.QueuedPromptRow{
			Model:    gorm.Model{CreatedAt: prompt.CreatedAt, UpdatedAt: prompt.CreatedAt},
			PromptID: prompt.ID,
			Position: i,
			Content:  prompt.Content,
		})
	}
	refRows := make([]persistdb.ReferenceRow, 0, len(stored.References))
	for i, ref := range stored.References {
		refRows = append(refRows, persistdb.ReferenceRow{
			Model:    gorm.Model{CreatedAt: ref.CreatedAt, UpdatedAt: ref.CreatedAt},
			Position: i,
			RefID:    ref.SessionID,
			Role:     string(ref.Role),
			Preview:  ref.Preview,
		})
	}
	return s.db.SaveDetails(stored.ID, fileRows, revisionRows, queueRows, refRows)
}

func logMessages(messages []Message) []persistfs.LogMessage {
	out := make([]persistfs.LogMessage, 0, len(messages))
	for _, msg := range messages {
		out = append(out, persistfs.LogMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
			Tool:    msg.ToolName,
			Args:    msg.Args,
			Result:  msg.Result,
			Summary: msg.Summary,
		})
	}
	return out
}

func (s *Service) upsertMessageLocked(stored *storedSession, msg Message) {
	if msg.UpdatedAt.IsZero() {
		msg.UpdatedAt = msg.CreatedAt
	}
	if msg.Role == MessageUser && stored.Title == defaultTitle(stored.SessionID) && msg.Content != "" {
		stored.Title = titleFromContent(msg.Content)
	}
	if msg.Role == MessageTool && msg.ToolID != "" {
		for i := range stored.Messages {
			existing := &stored.Messages[i]
			if existing.Role == MessageTool && existing.ToolID == msg.ToolID {
				if msg.ToolName != "" {
					existing.ToolName = msg.ToolName
				}
				if msg.Args != "" {
					existing.Args = msg.Args
				}
				if msg.Result != "" {
					existing.Result = msg.Result
				}
				if msg.Status != "" {
					existing.Status = msg.Status
				}
				existing.UpdatedAt = msg.UpdatedAt
				return
			}
		}
	}
	stored.Messages = append(stored.Messages, msg)
}

func (s *Service) publishLocked(t EventType, msg Message) {
	event := Event[Message]{Type: t, Payload: msg}
	for ch := range s.subs {
		select {
		case ch <- event:
		default:
		}
	}
}

func (s *Service) writeSnapshotContent(content string, missing bool) error {
	if missing {
		return nil
	}
	data := []byte(content)
	sum := sha256.Sum256(data)
	hash := fmt.Sprintf("%x", sum[:])
	path := s.sessionSnapshotPath(hash)
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp := path + ".tmp." + uuid.NewString()
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		if _, statErr := os.Stat(path); statErr == nil {
			return nil
		}
		return err
	}
	return nil
}

func (s *Service) sessionSnapshotsDir() string {
	return filepath.Join(s.sessionDir, "snapshots")
}

func (s *Service) sessionSnapshotPath(hash string) string {
	return filepath.Join(s.sessionSnapshotsDir(), hash[:2], hash)
}

func (s *Service) referencedSnapshotHashesLocked() map[string]struct{} {
	out := make(map[string]struct{})
	for _, stored := range s.sessions {
		for _, revision := range stored.FileRevisions {
			if revision.Before.Hash != "" && !revision.Before.Missing {
				out[revision.Before.Hash] = struct{}{}
			}
			if revision.After.Hash != "" && !revision.After.Missing {
				out[revision.After.Hash] = struct{}{}
			}
		}
	}
	return out
}

func (s *Service) snapshotFiles() ([]SnapshotInfo, error) {
	root := s.sessionSnapshotsDir()
	var out []SnapshotInfo
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		name := entry.Name()
		if strings.Contains(name, ".tmp.") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		out = append(out, SnapshotInfo{
			Hash: name,
			Path: path,
			Size: info.Size(),
		})
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Hash < out[j].Hash
	})
	return out, nil
}

func (s *Service) removeEmptySnapshotDirs() error {
	root := s.sessionSnapshotsDir()
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		children, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		if len(children) == 0 {
			if err := os.Remove(dir); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

func projectEvent(sessionID string, event *adksession.Event) []Message {
	return ProjectEvent(sessionID, event)
}

func ProjectEvent(sessionID string, event *adksession.Event) []Message {
	if event == nil || event.Content == nil {
		return nil
	}
	var out []Message
	role := MessageAssistant
	if event.Author == "user" {
		role = MessageUser
	}
	for _, part := range event.Content.Parts {
		if part.Text != "" {
			out = append(out, Message{
				ID:           uuid.NewString(),
				SessionID:    sessionID,
				EventID:      event.ID,
				InvocationID: event.InvocationID,
				Role:         role,
				Content:      part.Text,
				Summary:      event.Actions.SkipSummarization,
				CreatedAt:    event.Timestamp,
				UpdatedAt:    event.Timestamp,
			})
		}
		if part.FunctionCall != nil {
			args, _ := json.Marshal(part.FunctionCall.Args)
			out = append(out, Message{
				ID:           uuid.NewString(),
				SessionID:    sessionID,
				EventID:      event.ID,
				InvocationID: event.InvocationID,
				Role:         MessageTool,
				ToolName:     part.FunctionCall.Name,
				ToolID:       firstNonEmpty(part.FunctionCall.ID, part.FunctionCall.Name),
				Args:         string(args),
				Status:       "running",
				CreatedAt:    event.Timestamp,
				UpdatedAt:    event.Timestamp,
			})
		}
		if part.FunctionResponse != nil {
			resp, _ := json.Marshal(part.FunctionResponse.Response)
			out = append(out, Message{
				ID:           uuid.NewString(),
				SessionID:    sessionID,
				EventID:      event.ID,
				InvocationID: event.InvocationID,
				Role:         MessageTool,
				ToolName:     part.FunctionResponse.Name,
				ToolID:       firstNonEmpty(part.FunctionResponse.ID, part.FunctionResponse.Name),
				Result:       string(resp),
				Status:       toolStatus(part.FunctionResponse.Response),
				CreatedAt:    event.Timestamp,
				UpdatedAt:    event.Timestamp,
			})
		}
	}
	return out
}

func updateUsage(stored *storedSession, event *adksession.Event) {
	if event == nil || event.UsageMetadata == nil {
		return
	}
	stored.PromptTokens += int64(event.UsageMetadata.PromptTokenCount)
	stored.CompletionTokens += int64(event.UsageMetadata.CandidatesTokenCount)
}

func messageSearchText(msg Message) string {
	parts := []string{
		string(msg.Role),
		msg.Content,
		msg.ToolName,
		msg.ToolID,
		msg.Args,
		msg.Result,
		msg.Status,
	}
	return strings.Join(parts, "\n")
}

func matchPreview(text, query string, maxRunes int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if maxRunes <= 0 {
		return text
	}
	lowerText := strings.ToLower(text)
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	index := strings.Index(lowerText, lowerQuery)
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return strings.Join(strings.Fields(text), " ")
	}
	start := 0
	if index >= 0 {
		queryRuneIndex := len([]rune(text[:index]))
		start = queryRuneIndex - maxRunes/3
		if start < 0 {
			start = 0
		}
		if start+maxRunes > len(runes) {
			start = len(runes) - maxRunes
		}
	}
	end := start + maxRunes
	if end > len(runes) {
		end = len(runes)
	}
	preview := strings.Join(strings.Fields(string(runes[start:end])), " ")
	if start > 0 {
		preview = "..." + preview
	}
	if end < len(runes) {
		preview += "..."
	}
	return preview
}

func metadata(stored *storedSession) Metadata {
	return Metadata{
		AppName:          stored.AppName,
		UserID:           stored.UserID,
		SessionID:        stored.SessionID,
		Title:            stored.Title,
		MessageCount:     len(stored.Messages),
		PromptTokens:     stored.PromptTokens,
		CompletionTokens: stored.CompletionTokens,
		SummaryMessageID: stored.SummaryMessageID,
		FileCount:        len(stored.Files),
		QueuedPrompts:    len(stored.PromptQueue),
		CreatedAt:        stored.CreatedAt,
		UpdatedAt:        stored.UpdatedAt,
	}
}

func formatStoredTime(t time.Time) string {
	if t.IsZero() {
		return time.Time{}.UTC().Format(time.RFC3339Nano)
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseStoredTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, value)
}

func parseSessionID(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("session id must be a positive integer")
	}
	return id, nil
}

func parseOptionalSessionID(value string) int64 {
	id, err := parseSessionID(value)
	if err != nil {
		return 0
	}
	return id
}

func normalizeSessionID(value string) (string, error) {
	id, err := parseSessionID(value)
	if err != nil {
		return "", err
	}
	if id == 0 {
		return "", nil
	}
	return formatSessionID(id), nil
}

func formatSessionID(id int64) string {
	if id <= 0 {
		return ""
	}
	return strconv.FormatInt(id, 10)
}

func normalizeImportedMessages(sessionID string, messages []Message, now time.Time) []Message {
	out := make([]Message, 0, len(messages))
	for _, msg := range messages {
		if msg.ID == "" {
			msg.ID = uuid.NewString()
		}
		msg.SessionID = sessionID
		if msg.CreatedAt.IsZero() {
			msg.CreatedAt = now
		}
		if msg.UpdatedAt.IsZero() {
			msg.UpdatedAt = msg.CreatedAt
		}
		out = append(out, msg)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func normalizeImportedFiles(files []SessionFile) map[string]SessionFile {
	out := make(map[string]SessionFile, len(files))
	for _, file := range files {
		if strings.TrimSpace(file.Path) == "" {
			continue
		}
		out[file.Path] = file
	}
	return out
}

func normalizeImportedRevisions(revisions []FileRevision, now time.Time) []FileRevision {
	out := make([]FileRevision, 0, len(revisions))
	for _, revision := range revisions {
		if strings.TrimSpace(revision.Path) == "" {
			continue
		}
		if revision.ID == "" {
			revision.ID = uuid.NewString()
		}
		if revision.Action == "" {
			revision.Action = "import"
		}
		if revision.CreatedAt.IsZero() {
			revision.CreatedAt = now
		}
		out = append(out, revision)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func normalizeImportedQueue(queue []QueuedPrompt, now time.Time) []QueuedPrompt {
	out := make([]QueuedPrompt, 0, len(queue))
	for _, prompt := range queue {
		prompt.Content = strings.TrimSpace(prompt.Content)
		if prompt.Content == "" {
			continue
		}
		if prompt.ID == "" {
			prompt.ID = uuid.NewString()
		}
		if prompt.CreatedAt.IsZero() {
			prompt.CreatedAt = now
		}
		out = append(out, prompt)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func normalizeImportedReferences(refs []SessionReference, now time.Time) []SessionReference {
	out := make([]SessionReference, 0, len(refs))
	for _, ref := range refs {
		ref.SessionID = strings.TrimSpace(ref.SessionID)
		if ref.SessionID == "" {
			continue
		}
		ref.Preview = compactPreview(ref.Preview, 240)
		if ref.CreatedAt.IsZero() {
			ref.CreatedAt = now
		}
		out = append(out, ref)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func fileLastAccess(file SessionFile) time.Time {
	if file.WrittenAt.After(file.ReadAt) {
		return file.WrittenAt
	}
	return file.ReadAt
}

func recentSessionFiles(files map[string]SessionFile, limit int) []SessionFile {
	if len(files) == 0 {
		return nil
	}
	out := make([]SessionFile, 0, len(files))
	for _, file := range files {
		out = append(out, file)
	}
	sort.Slice(out, func(i, j int) bool {
		return fileLastAccess(out[i]).After(fileLastAccess(out[j]))
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func recentSessionReferences(refs []SessionReference, limit int) []SessionReference {
	out := make([]SessionReference, 0, len(refs))
	for _, ref := range refs {
		if strings.TrimSpace(ref.SessionID) == "" {
			continue
		}
		out = append(out, ref)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func snapshotContent(content string, missing bool) FileSnapshot {
	if missing {
		return FileSnapshot{Missing: true}
	}
	data := []byte(content)
	sum := sha256.Sum256(data)
	preview := previewContent(content, 4000)
	return FileSnapshot{
		Hash:      fmt.Sprintf("%x", sum[:]),
		Size:      len(data),
		Preview:   preview,
		Truncated: len([]rune(preview)) < len([]rune(content)),
	}
}

func compactPreview(text string, maxRunes int) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	return previewContent(text, maxRunes)
}

func previewContent(content string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(content)
	if len(runes) <= maxRunes {
		return content
	}
	if maxRunes <= 1 {
		return "…"
	}
	return string(runes[:maxRunes-1]) + "…"
}

func lineChangeCounts(before, after string) (int, int) {
	beforeLines := splitLines(before)
	afterLines := splitLines(after)
	lcs := make([][]int, len(beforeLines)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(afterLines)+1)
	}
	for i := len(beforeLines) - 1; i >= 0; i-- {
		for j := len(afterLines) - 1; j >= 0; j-- {
			if beforeLines[i] == afterLines[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	common := lcs[0][0]
	return len(afterLines) - common, len(beforeLines) - common
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.SplitAfter(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func defaultTitle(sessionID string) string {
	if sessionID == "" {
		return "New session"
	}
	return "Session " + sessionID
}

func titleFromContent(content string) string {
	content = strings.Join(strings.Fields(content), " ")
	runes := []rune(content)
	if len(runes) > 60 {
		return string(runes[:59]) + "…"
	}
	if content == "" {
		return "New session"
	}
	return content
}

func filterEvents(events []*adksession.Event, limit int, after time.Time) []*adksession.Event {
	filtered := events
	if !after.IsZero() {
		start := sort.Search(len(filtered), func(i int) bool {
			return !filtered[i].Timestamp.Before(after)
		})
		filtered = filtered[start:]
	}
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}
	return filtered
}

func applySessionStateDelta(state map[string]any, delta map[string]any) {
	if state == nil || len(delta) == 0 {
		return
	}
	for k, v := range delta {
		if strings.HasPrefix(k, adksession.KeyPrefixTemp) ||
			strings.HasPrefix(k, adksession.KeyPrefixApp) ||
			strings.HasPrefix(k, adksession.KeyPrefixUser) {
			continue
		}
		state[k] = v
	}
}

func applySessionMetadataDelta(stored *storedSession, delta map[string]any) {
	if stored == nil || len(delta) == 0 {
		return
	}
	if title, ok := delta[sessionStateTitle].(string); ok && strings.TrimSpace(title) != "" {
		stored.Title = strings.TrimSpace(title)
	}
	if summaryID, ok := delta[sessionStateSummaryMessageID].(string); ok {
		stored.SummaryMessageID = strings.TrimSpace(summaryID)
	}
}

func sessionState(in map[string]any) map[string]any {
	out := make(map[string]any)
	for k, v := range in {
		if strings.HasPrefix(k, adksession.KeyPrefixTemp) ||
			strings.HasPrefix(k, adksession.KeyPrefixApp) ||
			strings.HasPrefix(k, adksession.KeyPrefixUser) {
			continue
		}
		out[k] = v
	}
	return out
}

func cloneEvents(in []*adksession.Event) []*adksession.Event {
	out := make([]*adksession.Event, len(in))
	for i, event := range in {
		out[i] = cloneEvent(event)
	}
	return out
}

func cloneEvent(event *adksession.Event) *adksession.Event {
	if event == nil {
		return nil
	}
	cp := *event
	cp.Actions.StateDelta = maps.Clone(event.Actions.StateDelta)
	cp.Actions.ArtifactDelta = maps.Clone(event.Actions.ArtifactDelta)
	cp.Actions.RequestedToolConfirmations = maps.Clone(event.Actions.RequestedToolConfirmations)
	cp.LongRunningToolIDs = append([]string(nil), event.LongRunningToolIDs...)
	return &cp
}

func sessionKey(appName, userID, sessionID string) string {
	return appName + "\x00" + userID + "\x00" + sessionID
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func toolStatus(resp map[string]any) string {
	if _, ok := resp["error"]; ok {
		return "error"
	}
	return "done"
}
