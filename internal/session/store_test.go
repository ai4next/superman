package session_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/global"
	supermansession "github.com/ai4next/superman/internal/session"
	_ "github.com/mattn/go-sqlite3"
	adksession "google.golang.org/adk/session"
	adksessiontest "google.golang.org/adk/session/session_test"
	"google.golang.org/genai"
)

func TestService_ADKCompatibility(t *testing.T) {
	adksessiontest.RunServiceTests(t, adksessiontest.SuiteOptions{
		SupportsUserProvidedSessionID: false,
	}, func(t *testing.T) adksession.Service {
		t.Helper()
		setTestWorkspace(t)
		svc, err := supermansession.NewService()
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		return svc
	})
}

func TestServicePromptHistory(t *testing.T) {
	ctx := context.Background()
	setTestWorkspace(t)
	svc, err := supermansession.NewService()
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	created, err := svc.Create(ctx, &adksession.CreateRequest{AppName: "app", UserID: "user", SessionID: "1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	for _, text := range []string{"first", "second", "first", "third"} {
		event := adksession.NewEvent("inv")
		event.Author = "user"
		event.Content = genai.NewContentFromText(text, genai.RoleUser)
		if err := svc.AppendEvent(ctx, created.Session, event); err != nil {
			t.Fatalf("AppendEvent %q: %v", text, err)
		}
	}

	history, err := svc.PromptHistory("app", "user", "1", 0)
	if err != nil {
		t.Fatalf("PromptHistory: %v", err)
	}
	want := []string{"third", "first", "second"}
	if strings.Join(history, "|") != strings.Join(want, "|") {
		t.Fatalf("history = %#v, want %#v", history, want)
	}
	limited, err := svc.PromptHistory("app", "user", "1", 2)
	if err != nil {
		t.Fatalf("PromptHistory limited: %v", err)
	}
	if strings.Join(limited, "|") != "third|first" {
		t.Fatalf("limited = %#v", limited)
	}
}

func TestAppendEventUpdatesCurrentSessionView(t *testing.T) {
	ctx := context.Background()
	setTestWorkspace(t)
	svc, err := supermansession.NewService()
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	created, err := svc.Create(ctx, &adksession.CreateRequest{AppName: "app", UserID: "user"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	event := adksession.NewEvent("inv")
	event.Author = "user"
	event.Content = genai.NewContentFromText("first prompt", genai.RoleUser)
	if err := svc.AppendEvent(ctx, created.Session, event); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	events := created.Session.Events()
	if events.Len() != 1 {
		t.Fatalf("current session events len = %d, want 1", events.Len())
	}
	got := events.At(0)
	if got == nil || got.Content == nil || len(got.Content.Parts) == 0 || got.Content.Parts[0].Text != "first prompt" {
		t.Fatalf("current session event = %#v", got)
	}
}

func TestServiceUsesSQLiteMetadataAndSessionJSONL(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(setTestWorkspace(t), "sessions")
	svc, err := supermansession.NewService()
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	created, err := svc.Create(ctx, &adksession.CreateRequest{AppName: "app", UserID: "user", SessionID: "1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	event := adksession.NewEvent("inv")
	event.Author = "user"
	event.Content = genai.NewContentFromText("hello sqlite log\nwith newline", genai.RoleUser)
	if err := svc.AppendEvent(ctx, created.Session, event); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	db, err := sql.Open("sqlite3", filepath.Join(filepath.Dir(root), "state.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	var id int64
	var title string
	var messageCount int
	if err := db.QueryRow(`SELECT id, title, message_count FROM session WHERE app_name = ? AND user_id = ? AND id = ?`, "app", "user", 1).Scan(&id, &title, &messageCount); err != nil {
		t.Fatalf("query session row: %v", err)
	}
	if id == 0 || !strings.Contains(title, "hello sqlite log") || messageCount != 1 {
		t.Fatalf("sqlite metadata id=%d title=%q message_count=%d", id, title, messageCount)
	}
	var content string
	if err := db.QueryRow(`SELECT content FROM message WHERE session_id = ? AND role = ?`, 1, "user").Scan(&content); err != nil {
		t.Fatalf("query message row: %v", err)
	}
	if content != "hello sqlite log\nwith newline" {
		t.Fatalf("message content = %q", content)
	}
	data, err := os.ReadFile(filepath.Join(root, "1.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	logText := string(data)
	if !strings.Contains(logText, `U: hello sqlite log\nwith newline`) || strings.Contains(logText, `U: "`) || strings.Contains(logText, "table.messages") || strings.Contains(logText, "\nwith newline") {
		t.Fatalf("log sidecar not compact enough: %s", data)
	}
}

func TestServiceSessionReferences(t *testing.T) {
	ctx := context.Background()
	setTestWorkspace(t)
	svc, err := supermansession.NewService()
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if _, err := svc.Create(ctx, &adksession.CreateRequest{AppName: "app", UserID: "user", SessionID: "1"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.RecordSessionReference("app", "user", "1", supermansession.SessionReference{
		SessionID: "past",
		Role:      supermansession.MessageUser,
		Preview:   "  useful historical context  ",
	}); err != nil {
		t.Fatalf("RecordSessionReference: %v", err)
	}
	refs, err := svc.SessionReferences("app", "user", "1")
	if err != nil {
		t.Fatalf("SessionReferences: %v", err)
	}
	if len(refs) != 1 || refs[0].SessionID != "past" || refs[0].Preview != "useful historical context" {
		t.Fatalf("refs = %#v", refs)
	}
	window, err := svc.ContextWindow("app", "user", "1", 4)
	if err != nil {
		t.Fatalf("ContextWindow: %v", err)
	}
	if len(window.References) != 1 || window.References[0].SessionID != "past" {
		t.Fatalf("window refs = %#v", window.References)
	}
}

func TestServiceSearchMessages(t *testing.T) {
	ctx := context.Background()
	setTestWorkspace(t)
	svc, err := supermansession.NewService()
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	s1, err := svc.Create(ctx, &adksession.CreateRequest{AppName: "app", UserID: "user", SessionID: "1"})
	if err != nil {
		t.Fatalf("Create s1: %v", err)
	}
	s2, err := svc.Create(ctx, &adksession.CreateRequest{AppName: "app", UserID: "user", SessionID: "2"})
	if err != nil {
		t.Fatalf("Create s2: %v", err)
	}
	for _, item := range []struct {
		session adksession.Session
		author  string
		text    string
	}{
		{s1.Session, "user", "Investigate cache warming strategy"},
		{s1.Session, "superman", "Cache warming should preserve bounded context"},
		{s2.Session, "user", "Review permission audit trail"},
	} {
		event := adksession.NewEvent("inv")
		event.Author = item.author
		event.Content = genai.NewContentFromText(item.text, genai.Role(item.author))
		if err := svc.AppendEvent(ctx, item.session, event); err != nil {
			t.Fatalf("AppendEvent %q: %v", item.text, err)
		}
		time.Sleep(time.Millisecond)
	}

	results, err := svc.SearchMessages("app", "user", supermansession.MessageSearchOptions{Query: "cache"})
	if err != nil {
		t.Fatalf("SearchMessages: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results len = %d, want 2: %#v", len(results), results)
	}
	if results[0].Message.CreatedAt.Before(results[1].Message.CreatedAt) {
		t.Fatalf("results should be newest first: %#v", results)
	}
	if !strings.Contains(results[0].Preview, "Cache warming") {
		t.Fatalf("preview = %q", results[0].Preview)
	}

	filtered, err := svc.SearchMessages("app", "user", supermansession.MessageSearchOptions{
		Query: "cache",
		Roles: []supermansession.MessageRole{supermansession.MessageUser},
		Limit: 1,
	})
	if err != nil {
		t.Fatalf("SearchMessages filtered: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Message.Role != supermansession.MessageUser || filtered[0].Metadata.SessionID != "1" {
		t.Fatalf("filtered = %#v", filtered)
	}

	scoped, err := svc.SearchMessages("app", "user", supermansession.MessageSearchOptions{
		Query:     "permission",
		SessionID: "2",
	})
	if err != nil {
		t.Fatalf("SearchMessages scoped: %v", err)
	}
	if len(scoped) != 1 || scoped[0].Metadata.SessionID != "2" {
		t.Fatalf("scoped = %#v", scoped)
	}
}

func TestService_MessageProjectionAndContext(t *testing.T) {
	ctx := context.Background()
	setTestWorkspace(t)
	svc, err := supermansession.NewService()
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	created, err := svc.Create(ctx, &adksession.CreateRequest{AppName: "app", UserID: "user", SessionID: "1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	sub := svc.Subscribe(ctx)
	userEvent := adksession.NewEvent("inv1")
	userEvent.Author = "user"
	userEvent.Content = genai.NewContentFromText("Please inspect this project and report risks", genai.RoleUser)
	if err := svc.AppendEvent(ctx, created.Session, userEvent); err != nil {
		t.Fatalf("AppendEvent user: %v", err)
	}

	callEvent := adksession.NewEvent("inv1")
	callEvent.Author = "superman"
	callEvent.Content = &genai.Content{Parts: []*genai.Part{{FunctionCall: &genai.FunctionCall{
		ID:   "tool-1",
		Name: "read",
		Args: map[string]any{"path": "README.md"},
	}}}}
	if err := svc.AppendEvent(ctx, created.Session, callEvent); err != nil {
		t.Fatalf("AppendEvent call: %v", err)
	}

	responseEvent := adksession.NewEvent("inv1")
	responseEvent.Author = "superman"
	responseEvent.Content = &genai.Content{Parts: []*genai.Part{{FunctionResponse: &genai.FunctionResponse{
		ID:       "tool-1",
		Name:     "read",
		Response: map[string]any{"output": "hello"},
	}}}}
	if err := svc.AppendEvent(ctx, created.Session, responseEvent); err != nil {
		t.Fatalf("AppendEvent response: %v", err)
	}

	messages, err := svc.Messages("app", "user", "1")
	if err != nil {
		t.Fatalf("Messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("messages len = %d, want 2: %#v", len(messages), messages)
	}
	if messages[0].Role != supermansession.MessageUser {
		t.Fatalf("first role = %s", messages[0].Role)
	}
	if messages[1].Role != supermansession.MessageTool || messages[1].Status != "done" || messages[1].Result == "" {
		t.Fatalf("tool projection not merged: %#v", messages[1])
	}

	if _, err := svc.SetSummary("app", "user", "1", "Earlier summary"); err != nil {
		t.Fatalf("SetSummary: %v", err)
	}
	window, err := svc.ContextWindow("app", "user", "1", 1)
	if err != nil {
		t.Fatalf("ContextWindow: %v", err)
	}
	if window.Summary != "Earlier summary" {
		t.Fatalf("summary = %q", window.Summary)
	}
	if len(window.Messages) != 1 || window.Messages[0].Role != supermansession.MessageTool {
		t.Fatalf("window messages = %#v", window.Messages)
	}

	meta, err := svc.Metadata("app", "user", "1")
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if meta.Title == "" || meta.MessageCount != 3 || meta.SummaryMessageID == "" {
		t.Fatalf("metadata = %#v", meta)
	}

	select {
	case ev := <-sub:
		if ev.Type != supermansession.UpdatedEvent && ev.Type != supermansession.CreatedEvent {
			t.Fatalf("event type = %s", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("expected pubsub event")
	}
}

func TestServiceCompact(t *testing.T) {
	ctx := context.Background()
	setTestWorkspace(t)
	svc, err := supermansession.NewService()
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	created, err := svc.Create(ctx, &adksession.CreateRequest{AppName: "app", UserID: "user", SessionID: "1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	for i := 0; i < 8; i++ {
		event := adksession.NewEvent("inv")
		event.Author = "user"
		event.Content = genai.NewContentFromText(fmt.Sprintf("message %d", i), genai.RoleUser)
		if err := svc.AppendEvent(ctx, created.Session, event); err != nil {
			t.Fatalf("AppendEvent %d: %v", i, err)
		}
	}
	result, err := svc.Compact("app", "user", "1", supermansession.CompactOptions{
		MaxMessages:     5,
		KeepLast:        2,
		MaxSummaryRunes: 1000,
	})
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if !result.Compacted || result.Scanned != 8 || result.Kept != 2 {
		t.Fatalf("compact result = %#v", result)
	}
	window, err := svc.ContextWindow("app", "user", "1", 2)
	if err != nil {
		t.Fatalf("ContextWindow: %v", err)
	}
	if !strings.Contains(window.Summary, "Deterministic session summary") || !strings.Contains(window.Summary, "message 0") {
		t.Fatalf("summary = %q", window.Summary)
	}
	if len(window.Messages) != 2 {
		t.Fatalf("window messages len = %d, want 2", len(window.Messages))
	}
}

func TestServiceTracksSessionFiles(t *testing.T) {
	ctx := context.Background()
	dir := setTestWorkspace(t)
	svc, err := supermansession.NewService()
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if _, err := svc.Create(ctx, &adksession.CreateRequest{AppName: "app", UserID: "user", SessionID: "1"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	file := filepath.Join(dir, "README.md")

	if err := svc.RecordFileRead("app", "user", "1", file); err != nil {
		t.Fatalf("RecordFileRead: %v", err)
	}
	if err := svc.RecordFileWrite("app", "user", "1", file); err != nil {
		t.Fatalf("RecordFileWrite: %v", err)
	}
	revision, err := svc.RecordFileRevision("app", "user", "1", file, "patch", "old", "new", false)
	if err != nil {
		t.Fatalf("RecordFileRevision: %v", err)
	}
	if revision.Before.Hash == revision.After.Hash || revision.Before.Preview != "old" || revision.After.Preview != "new" {
		t.Fatalf("revision = %#v", revision)
	}
	before, missing, err := svc.FileSnapshotContent(revision.Before)
	if err != nil {
		t.Fatalf("FileSnapshotContent before: %v", err)
	}
	if missing || before != "old" {
		t.Fatalf("before content = %q missing=%v", before, missing)
	}
	window, err := svc.ContextWindow("app", "user", "1", 4)
	if err != nil {
		t.Fatalf("ContextWindow: %v", err)
	}
	if len(window.Files) != 1 || window.Files[0].Path != file {
		t.Fatalf("window files = %#v", window.Files)
	}
	files, err := svc.SessionFiles("app", "user", "1")
	if err != nil {
		t.Fatalf("SessionFiles: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("files len = %d, want 1: %#v", len(files), files)
	}
	if files[0].Path != file || files[0].ReadCount != 1 || files[0].WriteCount != 2 || files[0].LastAccess != supermansession.FileWritten {
		t.Fatalf("file = %#v", files[0])
	}
	revisions, err := svc.FileRevisions("app", "user", "1")
	if err != nil {
		t.Fatalf("FileRevisions: %v", err)
	}
	if len(revisions) != 1 || revisions[0].Action != "patch" {
		t.Fatalf("revisions = %#v", revisions)
	}
	lastRead, err := svc.LastFileRead("app", "user", "1", file)
	if err != nil {
		t.Fatalf("LastFileRead: %v", err)
	}
	if lastRead.IsZero() {
		t.Fatal("last read should be recorded")
	}
	changes, err := svc.SessionFileChanges("app", "user", "1")
	if err != nil {
		t.Fatalf("SessionFileChanges: %v", err)
	}
	if len(changes) != 1 || changes[0].File.Path != file || changes[0].Additions != 1 || changes[0].Deletions != 1 {
		t.Fatalf("changes = %#v", changes)
	}
	meta, err := svc.Metadata("app", "user", "1")
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if meta.FileCount != 1 {
		t.Fatalf("file count = %d, want 1", meta.FileCount)
	}

	reloaded, err := supermansession.NewService()
	if err != nil {
		t.Fatalf("reload NewService: %v", err)
	}
	files, err = reloaded.SessionFiles("app", "user", "1")
	if err != nil {
		t.Fatalf("reloaded SessionFiles: %v", err)
	}
	if len(files) != 1 || files[0].Path != file {
		t.Fatalf("reloaded files = %#v", files)
	}
	revisions, err = reloaded.FileRevisions("app", "user", "1")
	if err != nil {
		t.Fatalf("reloaded FileRevisions: %v", err)
	}
	if len(revisions) != 1 || revisions[0].After.Preview != "new" {
		t.Fatalf("reloaded revisions = %#v", revisions)
	}
	after, missing, err := reloaded.FileSnapshotContent(revisions[0].After)
	if err != nil {
		t.Fatalf("reloaded FileSnapshotContent after: %v", err)
	}
	if missing || after != "new" {
		t.Fatalf("after content = %q missing=%v", after, missing)
	}
}

func TestServiceStoresFullFileRevisionSnapshots(t *testing.T) {
	ctx := context.Background()
	dir := setTestWorkspace(t)
	svc, err := supermansession.NewService()
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if _, err := svc.Create(ctx, &adksession.CreateRequest{AppName: "app", UserID: "user", SessionID: "1"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	content := strings.Repeat("x", 4100)
	revision, err := svc.RecordFileRevision("app", "user", "1", filepath.Join(dir, "large.txt"), "patch", content, "after", false)
	if err != nil {
		t.Fatalf("RecordFileRevision: %v", err)
	}
	if !revision.Before.Truncated {
		t.Fatalf("before snapshot should be marked truncated: %#v", revision.Before)
	}

	reloaded, err := supermansession.NewService()
	if err != nil {
		t.Fatalf("reload NewService: %v", err)
	}
	revisions, err := reloaded.FileRevisions("app", "user", "1")
	if err != nil {
		t.Fatalf("FileRevisions: %v", err)
	}
	full, missing, err := reloaded.FileSnapshotContent(revisions[0].Before)
	if err != nil {
		t.Fatalf("FileSnapshotContent: %v", err)
	}
	if missing || full != content {
		t.Fatalf("full content len = %d missing=%v", len(full), missing)
	}
	changes, err := reloaded.SessionFileChanges("app", "user", "1")
	if err != nil {
		t.Fatalf("SessionFileChanges: %v", err)
	}
	if len(changes) != 1 || changes[0].Additions != 1 || changes[0].Deletions != 1 {
		t.Fatalf("changes = %#v", changes)
	}
}

func TestServiceStorageStatsAndSnapshotCleanup(t *testing.T) {
	ctx := context.Background()
	dir := setTestWorkspace(t)
	root := filepath.Join(dir, "sessions")
	svc, err := supermansession.NewService()
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if _, err := svc.Create(ctx, &adksession.CreateRequest{AppName: "app", UserID: "user", SessionID: "1"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.RecordFileRevision("app", "user", "1", filepath.Join(dir, "main.go"), "patch", "old", "new", false); err != nil {
		t.Fatalf("RecordFileRevision: %v", err)
	}
	orphanPath := filepath.Join(root, "snapshots", "aa", "aa-orphan")
	if err := os.MkdirAll(filepath.Dir(orphanPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(orphanPath, []byte("orphan"), 0o644); err != nil {
		t.Fatal(err)
	}

	stats, err := svc.StorageStats()
	if err != nil {
		t.Fatalf("StorageStats: %v", err)
	}
	if stats.Sessions != 1 || stats.FileRevisions != 1 || stats.SnapshotCount != 3 || stats.ReferencedSnapshotCount != 2 || stats.OrphanSnapshotCount != 1 {
		t.Fatalf("stats = %#v", stats)
	}

	dryRun, err := svc.CleanupOrphanSnapshots(true)
	if err != nil {
		t.Fatalf("CleanupOrphanSnapshots dry run: %v", err)
	}
	if !dryRun.DryRun || dryRun.Removed != 1 || dryRun.RemovedBytes != int64(len("orphan")) || len(dryRun.Orphans) != 1 {
		t.Fatalf("dry run = %#v", dryRun)
	}
	if _, err := os.Stat(orphanPath); err != nil {
		t.Fatalf("dry run should keep orphan snapshot: %v", err)
	}

	applied, err := svc.CleanupOrphanSnapshots(false)
	if err != nil {
		t.Fatalf("CleanupOrphanSnapshots apply: %v", err)
	}
	if applied.DryRun || applied.Removed != 1 || len(applied.Orphans) != 0 {
		t.Fatalf("applied = %#v", applied)
	}
	if _, err := os.Stat(orphanPath); !os.IsNotExist(err) {
		t.Fatalf("orphan snapshot should be removed, stat err = %v", err)
	}
	stats, err = svc.StorageStats()
	if err != nil {
		t.Fatalf("StorageStats after cleanup: %v", err)
	}
	if stats.OrphanSnapshotCount != 0 || stats.ReferencedSnapshotCount != 2 {
		t.Fatalf("stats after cleanup = %#v", stats)
	}
}

func TestServicePromptQueue(t *testing.T) {
	ctx := context.Background()
	setTestWorkspace(t)
	svc, err := supermansession.NewService()
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if _, err := svc.Create(ctx, &adksession.CreateRequest{AppName: "app", UserID: "user", SessionID: "1"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	first, err := svc.EnqueuePrompt("app", "user", "1", " first ")
	if err != nil {
		t.Fatalf("EnqueuePrompt first: %v", err)
	}
	second, err := svc.EnqueuePrompt("app", "user", "1", "second")
	if err != nil {
		t.Fatalf("EnqueuePrompt second: %v", err)
	}
	if first.Content != "first" || second.Content != "second" || first.ID == second.ID {
		t.Fatalf("queued prompts = %#v %#v", first, second)
	}
	queue, err := svc.PromptQueue("app", "user", "1")
	if err != nil {
		t.Fatalf("PromptQueue: %v", err)
	}
	if len(queue) != 2 || queue[0].Content != "first" || queue[1].Content != "second" {
		t.Fatalf("queue = %#v", queue)
	}
	meta, err := svc.Metadata("app", "user", "1")
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if meta.QueuedPrompts != 2 {
		t.Fatalf("queued prompts = %d, want 2", meta.QueuedPrompts)
	}
	prompt, ok, err := svc.DequeuePrompt("app", "user", "1")
	if err != nil {
		t.Fatalf("DequeuePrompt: %v", err)
	}
	if !ok || prompt.Content != "first" {
		t.Fatalf("dequeue prompt = %#v ok=%v", prompt, ok)
	}
	cleared, err := svc.ClearPromptQueue("app", "user", "1")
	if err != nil {
		t.Fatalf("ClearPromptQueue: %v", err)
	}
	if cleared != 1 {
		t.Fatalf("cleared = %d, want 1", cleared)
	}

	reloaded, err := supermansession.NewService()
	if err != nil {
		t.Fatalf("reload NewService: %v", err)
	}
	queue, err = reloaded.PromptQueue("app", "user", "1")
	if err != nil {
		t.Fatalf("reloaded PromptQueue: %v", err)
	}
	if len(queue) != 0 {
		t.Fatalf("reloaded queue = %#v", queue)
	}
}

func TestServiceImport(t *testing.T) {
	dir := setTestWorkspace(t)
	svc, err := supermansession.NewService()
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	now := time.Now()
	meta, err := svc.Import("app", "user", supermansession.ImportData{
		Metadata: supermansession.Metadata{
			SessionID: "3",
			Title:     "Imported Session",
			CreatedAt: now,
			UpdatedAt: now,
		},
		Messages: []supermansession.Message{
			{Role: supermansession.MessageUser, Content: "hello imported"},
		},
		Files: []supermansession.SessionFile{
			{Path: filepath.Join(dir, "main.go"), ReadCount: 1, LastAccess: supermansession.FileRead},
		},
		PromptQueue: []supermansession.QueuedPrompt{
			{Content: "next imported task"},
		},
	})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if meta.SessionID != "3" || meta.MessageCount != 1 || meta.FileCount != 1 || meta.QueuedPrompts != 1 {
		t.Fatalf("metadata = %#v", meta)
	}
	if _, err := svc.Import("app", "user", supermansession.ImportData{Metadata: supermansession.Metadata{SessionID: "3"}}); err == nil {
		t.Fatal("duplicate import should fail without overwrite")
	}

	reloaded, err := supermansession.NewService()
	if err != nil {
		t.Fatalf("reload NewService: %v", err)
	}
	messages, err := reloaded.Messages("app", "user", "3")
	if err != nil {
		t.Fatalf("Messages: %v", err)
	}
	if len(messages) != 1 || messages[0].SessionID != "3" || messages[0].Content != "hello imported" {
		t.Fatalf("messages = %#v", messages)
	}
	queue, err := reloaded.PromptQueue("app", "user", "3")
	if err != nil {
		t.Fatalf("PromptQueue: %v", err)
	}
	if len(queue) != 1 || queue[0].Content != "next imported task" {
		t.Fatalf("queue = %#v", queue)
	}
}

func setTestWorkspace(t *testing.T) string {
	t.Helper()
	workspace := t.TempDir()
	global.SetConfig(&config.Config{Workspace: workspace})
	t.Cleanup(func() { global.SetConfig(nil) })
	return workspace
}
