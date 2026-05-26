package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ai4next/superman/internal/global"
	adksession "google.golang.org/adk/session"
)

const (
	sessionStateTitle            = "session:title"
	sessionStateSummaryMessageID = "session:summary_message_id"
	sessionStatePromptQueue      = "session:prompt_queue"
)

func Messages(svc adksession.Service, appName, userID, sessionID string) ([]Message, error) {
	if extended, ok := svc.(*Service); ok {
		return extended.Messages(appName, userID, sessionID)
	}
	resp, err := svc.Get(context.Background(), &adksession.GetRequest{AppName: appName, UserID: userID, SessionID: sessionID})
	if err != nil {
		return nil, err
	}
	return messagesFromSession(resp.Session), nil
}

func PromptHistory(svc adksession.Service, appName, userID, sessionID string, limit int) ([]string, error) {
	messages, err := Messages(svc, appName, userID, sessionID)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
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

func SearchMessages(svc adksession.Service, appName, userID string, opts MessageSearchOptions) ([]MessageSearchResult, error) {
	if extended, ok := svc.(*Service); ok {
		return extended.SearchMessages(appName, userID, opts)
	}
	query := strings.TrimSpace(opts.Query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	resp, err := svc.List(context.Background(), &adksession.ListRequest{AppName: appName, UserID: userID})
	if err != nil {
		return nil, err
	}
	roleFilter := make(map[MessageRole]struct{}, len(opts.Roles))
	for _, role := range opts.Roles {
		if role != "" {
			roleFilter[role] = struct{}{}
		}
	}
	needle := strings.ToLower(query)
	var results []MessageSearchResult
	for _, sess := range resp.Sessions {
		if opts.SessionID != "" && sess.ID() != opts.SessionID {
			continue
		}
		meta := MetadataForSession(sess)
		for _, msg := range messagesFromSession(sess) {
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

func SessionMetadata(svc adksession.Service, appName, userID, sessionID string) (Metadata, error) {
	if extended, ok := svc.(*Service); ok {
		return extended.Metadata(appName, userID, sessionID)
	}
	resp, err := svc.Get(context.Background(), &adksession.GetRequest{AppName: appName, UserID: userID, SessionID: sessionID})
	if err != nil {
		return Metadata{}, err
	}
	return MetadataForSession(resp.Session), nil
}

func ListSessionMetadata(svc adksession.Service, appName, userID string) []Metadata {
	if extended, ok := svc.(*Service); ok {
		return extended.ListMetadata(appName, userID)
	}
	resp, err := svc.List(context.Background(), &adksession.ListRequest{AppName: appName, UserID: userID})
	if err != nil {
		return nil
	}
	out := make([]Metadata, 0, len(resp.Sessions))
	for _, sess := range resp.Sessions {
		out = append(out, MetadataForSession(sess))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

func MetadataForSession(sess adksession.Session) Metadata {
	if sess == nil {
		return Metadata{}
	}
	messages := messagesFromSession(sess)
	files, _ := filesFromSession(sess)
	meta := Metadata{
		AppName:          sess.AppName(),
		UserID:           sess.UserID(),
		SessionID:        sess.ID(),
		Title:            stateString(sess.State(), sessionStateTitle),
		MessageCount:     len(messages),
		FileCount:        len(files),
		SummaryMessageID: stateString(sess.State(), sessionStateSummaryMessageID),
		CreatedAt:        sess.LastUpdateTime(),
		UpdatedAt:        sess.LastUpdateTime(),
	}
	if meta.Title == "" {
		meta.Title = titleFromMessages(sess.ID(), messages)
	}
	return meta
}

func Rename(svc adksession.Service, appName, userID, sessionID, title string) error {
	resp, err := svc.Get(context.Background(), &adksession.GetRequest{AppName: appName, UserID: userID, SessionID: sessionID})
	if err != nil {
		return err
	}
	event := adksession.NewEvent("rename-" + strconv.FormatInt(time.Now().UnixNano(), 10))
	event.Author = "user"
	event.Actions.StateDelta = map[string]any{sessionStateTitle: strings.TrimSpace(title)}
	return svc.AppendEvent(context.Background(), resp.Session, event)
}

func SessionFiles(svc adksession.Service, appName, userID, sessionID string) ([]SessionFile, error) {
	if extended, ok := svc.(*Service); ok {
		return extended.SessionFiles(appName, userID, sessionID)
	}
	resp, err := svc.Get(context.Background(), &adksession.GetRequest{AppName: appName, UserID: userID, SessionID: sessionID})
	if err != nil {
		return nil, err
	}
	return filesFromSession(resp.Session)
}

func FileRevisions(svc adksession.Service, appName, userID, sessionID string) ([]FileRevision, error) {
	if extended, ok := svc.(*Service); ok {
		return extended.FileRevisions(appName, userID, sessionID)
	}
	resp, err := svc.Get(context.Background(), &adksession.GetRequest{AppName: appName, UserID: userID, SessionID: sessionID})
	if err != nil {
		return nil, err
	}
	records := recordsFromSession(resp.Session)
	out := make([]FileRevision, 0, len(records.FileRevisions))
	for _, note := range records.FileRevisions {
		revision, err := buildFileRevision(note.Path, note.Action, note.Before, note.After, note.BeforeMissing, note.AfterMissing)
		if err != nil {
			continue
		}
		out = append(out, revision)
	}
	return out, nil
}

func SessionFileChanges(svc adksession.Service, appName, userID, sessionID string) ([]FileChangeSummary, error) {
	files, err := SessionFiles(svc, appName, userID, sessionID)
	if err != nil {
		return nil, err
	}
	revisions, err := FileRevisions(svc, appName, userID, sessionID)
	if err != nil {
		return nil, err
	}
	return fileChangeSummaries(files, revisions)
}

func PromptQueue(svc adksession.Service, appName, userID, sessionID string) ([]QueuedPrompt, error) {
	if extended, ok := svc.(*Service); ok {
		return extended.PromptQueue(appName, userID, sessionID)
	}
	resp, err := svc.Get(context.Background(), &adksession.GetRequest{AppName: appName, UserID: userID, SessionID: sessionID})
	if err != nil {
		return nil, err
	}
	return promptQueueFromState(resp.Session.State()), nil
}

func EnqueuePrompt(svc adksession.Service, appName, userID, sessionID, content string) (QueuedPrompt, error) {
	if extended, ok := svc.(*Service); ok {
		return extended.EnqueuePrompt(appName, userID, sessionID, content)
	}
	resp, err := svc.Get(context.Background(), &adksession.GetRequest{AppName: appName, UserID: userID, SessionID: sessionID})
	if err != nil {
		return QueuedPrompt{}, err
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return QueuedPrompt{}, fmt.Errorf("prompt is required")
	}
	queue := promptQueueFromState(resp.Session.State())
	prompt := QueuedPrompt{ID: "queue-" + strconv.FormatInt(time.Now().UnixNano(), 10), Content: content, CreatedAt: time.Now()}
	queue = append(queue, prompt)
	if err := appendStateEvent(svc, resp.Session, "queue-add", map[string]any{sessionStatePromptQueue: queue}); err != nil {
		return QueuedPrompt{}, err
	}
	return prompt, nil
}

func DequeuePrompt(svc adksession.Service, appName, userID, sessionID string) (QueuedPrompt, bool, error) {
	if extended, ok := svc.(*Service); ok {
		return extended.DequeuePrompt(appName, userID, sessionID)
	}
	resp, err := svc.Get(context.Background(), &adksession.GetRequest{AppName: appName, UserID: userID, SessionID: sessionID})
	if err != nil {
		return QueuedPrompt{}, false, err
	}
	queue := promptQueueFromState(resp.Session.State())
	if len(queue) == 0 {
		return QueuedPrompt{}, false, nil
	}
	prompt := queue[0]
	if err := appendStateEvent(svc, resp.Session, "queue-pop", map[string]any{sessionStatePromptQueue: queue[1:]}); err != nil {
		return QueuedPrompt{}, false, err
	}
	return prompt, true, nil
}

func ClearPromptQueue(svc adksession.Service, appName, userID, sessionID string) (int, error) {
	if extended, ok := svc.(*Service); ok {
		return extended.ClearPromptQueue(appName, userID, sessionID)
	}
	resp, err := svc.Get(context.Background(), &adksession.GetRequest{AppName: appName, UserID: userID, SessionID: sessionID})
	if err != nil {
		return 0, err
	}
	queue := promptQueueFromState(resp.Session.State())
	if len(queue) == 0 {
		return 0, nil
	}
	if err := appendStateEvent(svc, resp.Session, "queue-clear", map[string]any{sessionStatePromptQueue: []QueuedPrompt{}}); err != nil {
		return 0, err
	}
	return len(queue), nil
}

func RecordFileRead(svc adksession.Service, appName, userID, sessionID, path string) error {
	if extended, ok := svc.(*Service); ok {
		return extended.RecordFileRead(appName, userID, sessionID, path)
	}
	resp, err := svc.Get(context.Background(), &adksession.GetRequest{AppName: appName, UserID: userID, SessionID: sessionID})
	if err != nil {
		return err
	}
	actions := adksession.EventActions{StateDelta: make(map[string]any)}
	AddFileRead(&actions, path)
	return appendStateEvent(svc, resp.Session, "file-read", actions.StateDelta)
}

func RecordSessionReference(svc adksession.Service, appName, userID, sessionID string, ref SessionReference) error {
	if extended, ok := svc.(*Service); ok {
		return extended.RecordSessionReference(appName, userID, sessionID, ref)
	}
	resp, err := svc.Get(context.Background(), &adksession.GetRequest{AppName: appName, UserID: userID, SessionID: sessionID})
	if err != nil {
		return err
	}
	actions := adksession.EventActions{StateDelta: make(map[string]any)}
	AddSessionReference(&actions, ref)
	return appendStateEvent(svc, resp.Session, "session-reference", actions.StateDelta)
}

func RecordFileRevision(svc adksession.Service, appName, userID, sessionID string, note FileRevisionNote) error {
	if extended, ok := svc.(*Service); ok {
		_, err := extended.RecordFileRevisionWithMissing(appName, userID, sessionID, note.Path, note.Action, note.Before, note.After, note.BeforeMissing, note.AfterMissing)
		return err
	}
	resp, err := svc.Get(context.Background(), &adksession.GetRequest{AppName: appName, UserID: userID, SessionID: sessionID})
	if err != nil {
		return err
	}
	actions := adksession.EventActions{StateDelta: make(map[string]any)}
	AddFileRevision(&actions, note)
	return appendStateEvent(svc, resp.Session, "file-revision", actions.StateDelta)
}

func FileSnapshotContent(snapshot FileSnapshot) (string, bool, error) {
	if snapshot.Missing {
		return "", true, nil
	}
	if snapshot.Hash == "" {
		if snapshot.Truncated {
			return "", false, fmt.Errorf("snapshot content is truncated and has no hash")
		}
		return snapshot.Preview, false, nil
	}
	data, err := os.ReadFile(global.SessionSnapshotPath(snapshot.Hash))
	if err == nil {
		return string(data), false, nil
	}
	if !os.IsNotExist(err) {
		return "", false, err
	}
	if snapshot.Truncated {
		return "", false, fmt.Errorf("snapshot content missing for hash %s", snapshot.Hash)
	}
	return snapshot.Preview, false, nil
}

func fileChangeSummaries(files []SessionFile, revisions []FileRevision) ([]FileChangeSummary, error) {
	filesByPath := make(map[string]SessionFile, len(files))
	for _, file := range files {
		filesByPath[file.Path] = file
	}
	byPath := make(map[string][]FileRevision)
	for _, revision := range revisions {
		byPath[revision.Path] = append(byPath[revision.Path], revision)
	}
	out := make([]FileChangeSummary, 0, len(byPath))
	for path, versions := range byPath {
		if len(versions) == 0 {
			continue
		}
		sort.Slice(versions, func(i, j int) bool {
			return versions[i].CreatedAt.Before(versions[j].CreatedAt)
		})
		first := versions[0]
		latest := versions[len(versions)-1]
		before, _, err := FileSnapshotContent(first.Before)
		if err != nil {
			return nil, fmt.Errorf("load first snapshot for %s: %w", path, err)
		}
		after, _, err := FileSnapshotContent(latest.After)
		if err != nil {
			return nil, fmt.Errorf("load latest snapshot for %s: %w", path, err)
		}
		additions, deletions := lineChangeCounts(before, after)
		out = append(out, FileChangeSummary{
			File:           filesByPath[path],
			FirstRevision:  first,
			LatestRevision: latest,
			Additions:      additions,
			Deletions:      deletions,
		})
		if out[len(out)-1].File.Path == "" {
			out[len(out)-1].File.Path = path
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return fileLastAccess(out[i].File).After(fileLastAccess(out[j].File))
	})
	return out, nil
}

func filesFromSession(sess adksession.Session) ([]SessionFile, error) {
	records := recordsFromSession(sess)
	files := make(map[string]SessionFile)
	for _, path := range records.FileReads {
		file := files[path]
		file.Path = path
		file.ReadCount++
		file.LastAccess = FileRead
		files[path] = file
	}
	for _, path := range records.FileWrites {
		file := files[path]
		file.Path = path
		file.WriteCount++
		file.LastAccess = FileWritten
		files[path] = file
	}
	for _, revision := range records.FileRevisions {
		path := cleanContextPath(revision.Path)
		if path == "" {
			continue
		}
		file := files[path]
		file.Path = path
		file.WriteCount++
		file.LastAccess = FileWritten
		files[path] = file
	}
	out := make([]SessionFile, 0, len(files))
	for _, file := range files {
		out = append(out, file)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func SessionReferences(svc adksession.Service, appName, userID, sessionID string) ([]SessionReference, error) {
	if extended, ok := svc.(*Service); ok {
		return extended.SessionReferences(appName, userID, sessionID)
	}
	resp, err := svc.Get(context.Background(), &adksession.GetRequest{AppName: appName, UserID: userID, SessionID: sessionID})
	if err != nil {
		return nil, err
	}
	records := recordsFromSession(resp.Session)
	out := make([]SessionReference, 0, len(records.References))
	seen := make(map[string]struct{})
	for _, ref := range records.References {
		ref.SessionID = strings.TrimSpace(ref.SessionID)
		if ref.SessionID == "" {
			continue
		}
		key := fmt.Sprintf("%s\x00%s\x00%s", ref.SessionID, ref.Role, ref.Preview)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, ref)
	}
	return out, nil
}

func messagesFromSession(sess adksession.Session) []Message {
	if sess == nil || sess.Events() == nil {
		return nil
	}
	var out []Message
	for event := range sess.Events().All() {
		out = append(out, ProjectEvent(sess.ID(), event)...)
	}
	return out
}

func titleFromMessages(sessionID string, messages []Message) string {
	for _, msg := range messages {
		if msg.Role == MessageUser && strings.TrimSpace(msg.Content) != "" {
			return titleFromContent(msg.Content)
		}
	}
	return defaultTitle(sessionID)
}

func stateString(state adksession.State, key string) string {
	if state == nil {
		return ""
	}
	value, err := state.Get(key)
	if err != nil {
		return ""
	}
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func promptQueueFromState(state adksession.State) []QueuedPrompt {
	if state == nil {
		return nil
	}
	value, err := state.Get(sessionStatePromptQueue)
	if err != nil {
		return nil
	}
	switch queue := value.(type) {
	case []QueuedPrompt:
		return append([]QueuedPrompt(nil), queue...)
	case []any:
		data, err := json.Marshal(queue)
		if err != nil {
			return nil
		}
		var out []QueuedPrompt
		if err := json.Unmarshal(data, &out); err != nil {
			return nil
		}
		return out
	default:
		return nil
	}
}

func appendStateEvent(svc adksession.Service, sess adksession.Session, prefix string, delta map[string]any) error {
	event := adksession.NewEvent(prefix + "-" + strconv.FormatInt(time.Now().UnixNano(), 10))
	event.Author = "user"
	event.Actions.StateDelta = delta
	return svc.AppendEvent(context.Background(), sess, event)
}

func recordsFromSession(sess adksession.Session) ContextRecords {
	var records ContextRecords
	if sess == nil || sess.Events() == nil {
		return records
	}
	for event := range sess.Events().All() {
		eventRecords := contextRecords(&event.Actions)
		records.FileReads = append(records.FileReads, eventRecords.FileReads...)
		records.FileWrites = append(records.FileWrites, eventRecords.FileWrites...)
		records.FileRevisions = append(records.FileRevisions, eventRecords.FileRevisions...)
		records.References = append(records.References, eventRecords.References...)
	}
	return records
}
