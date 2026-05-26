package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/global"
	supermansession "github.com/ai4next/superman/internal/session"
	adksession "google.golang.org/adk/session"
	"google.golang.org/genai"
)

func TestWriteSessionListAndShow(t *testing.T) {
	svc, cfg := newCLITestSession(t)
	var buf bytes.Buffer
	if err := writeSessionList(&buf, svc, cfg, "cli-user", false); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "1") || !strings.Contains(out, "Work") || !strings.Contains(out, "MESSAGES") {
		t.Fatalf("session list = %s", out)
	}

	buf.Reset()
	if err := writeSessionShow(&buf, svc, cfg, "cli-user", "1", false); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "[user]") || !strings.Contains(out, "hello from cli") {
		t.Fatalf("session show = %s", out)
	}
}

func TestWriteSessionSearch(t *testing.T) {
	svc, cfg := newCLITestSession(t)
	created, err := svc.Get(t.Context(), &adksession.GetRequest{AppName: "app", UserID: "cli-user", SessionID: "1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"Find persistent cache policy", "Unrelated message"} {
		ev := adksession.NewEvent("inv-search")
		ev.Author = "user"
		ev.Content = genai.NewContentFromText(text, genai.RoleUser)
		if err := svc.AppendEvent(t.Context(), created.Session, ev); err != nil {
			t.Fatal(err)
		}
	}

	var buf bytes.Buffer
	if err := writeSessionSearch(&buf, svc, cfg, "cli-user", "cache", "", "user", 10, false); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "SESSION") || !strings.Contains(out, "Find persistent cache policy") || strings.Contains(out, "Unrelated") {
		t.Fatalf("search output = %s", out)
	}

	buf.Reset()
	if err := writeSessionSearch(&buf, svc, cfg, "cli-user", "cache", "1", "", 10, true); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, `"preview"`) || !strings.Contains(out, `"session_id": "1"`) {
		t.Fatalf("search json = %s", out)
	}
}

func TestWriteSessionFilesAndQueue(t *testing.T) {
	svc, cfg := newCLITestSession(t)
	path := filepath.Join(cfg.Workspace, "main.go")
	if err := svc.RecordFileRead("app", "cli-user", "1", path); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.EnqueuePrompt("app", "cli-user", "1", "next task"); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := writeSessionFiles(&buf, svc, cfg, "cli-user", "1", false); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "main.go") || !strings.Contains(out, "READS") {
		t.Fatalf("session files = %s", out)
	}

	buf.Reset()
	if err := writeSessionQueue(&buf, svc, cfg, "cli-user", "1", false); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "1. next task") {
		t.Fatalf("session queue = %s", out)
	}
}

func TestWriteSessionExportFormats(t *testing.T) {
	svc, cfg := newCLITestSession(t)
	path := filepath.Join(cfg.Workspace, "main.go")
	if err := svc.RecordFileRead("app", "cli-user", "1", path); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecordFileRevision("app", "cli-user", "1", path, "patch", "old\n", "new\n", false); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.EnqueuePrompt("app", "cli-user", "1", "queued export task"); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecordSessionReference("app", "cli-user", "1", supermansession.SessionReference{
		SessionID: "past",
		Role:      supermansession.MessageUser,
		Preview:   "historical cache decision",
	}); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := writeSessionExport(&buf, svc, cfg, "cli-user", "1", "markdown"); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "# Superman Session Export") || !strings.Contains(out, "hello from cli") || !strings.Contains(out, "File Changes") || !strings.Contains(out, "Session References") || !strings.Contains(out, "past") {
		t.Fatalf("markdown export = %s", out)
	}

	buf.Reset()
	if err := writeSessionExport(&buf, svc, cfg, "cli-user", "1", "json"); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, `"metadata"`) || !strings.Contains(out, `"messages"`) || !strings.Contains(out, `"prompt_queue"`) || !strings.Contains(out, `"references"`) {
		t.Fatalf("json export = %s", out)
	}

	buf.Reset()
	if err := writeSessionExport(&buf, svc, cfg, "cli-user", "1", "jsonl"); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, `"type":"metadata"`) || !strings.Contains(out, `"type":"message"`) || !strings.Contains(out, `"type":"file_revision"`) || !strings.Contains(out, `"type":"session_reference"`) {
		t.Fatalf("jsonl export = %s", out)
	}
}

func TestWriteSessionHistoryAndDiff(t *testing.T) {
	svc, cfg := newCLITestSession(t)
	path := filepath.Join(cfg.Workspace, "main.go")
	if _, err := svc.RecordFileRevision("app", "cli-user", "1", path, "patch", "old\n", "new\n", false); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := writeSessionHistory(&buf, svc, cfg, "cli-user", "1", false); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "ACTION") || !strings.Contains(out, "patch") || !strings.Contains(out, path) {
		t.Fatalf("history output = %s", out)
	}

	buf.Reset()
	if err := writeSessionDiff(&buf, svc, cfg, "cli-user", "1", path, false); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "--- a/") || !strings.Contains(out, "-old") || !strings.Contains(out, "+new") {
		t.Fatalf("diff output = %s", out)
	}

	buf.Reset()
	if err := writeSessionDiff(&buf, svc, cfg, "cli-user", "1", path, true); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, `"diff"`) || !strings.Contains(out, `"revision"`) {
		t.Fatalf("diff json = %s", out)
	}
}

func TestWriteSessionRevertModifiedFile(t *testing.T) {
	svc, cfg := newCLITestSession(t)
	path := filepath.Join(cfg.Workspace, "main.go")
	if err := os.WriteFile(path, []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecordFileRevision("app", "cli-user", "1", path, "patch", "old\n", "new\n", false); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := writeSessionRevert(&buf, svc, cfg, "cli-user", "1", path, false); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "Reverted") || !strings.Contains(out, "1") {
		t.Fatalf("revert output = %s", out)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "old\n" {
		t.Fatalf("file content = %q, want old", data)
	}
	revisions, err := svc.FileRevisions("app", "cli-user", "1")
	if err != nil {
		t.Fatal(err)
	}
	got := revisions[len(revisions)-1]
	if got.Action != "revert" {
		t.Fatalf("action = %q, want revert", got.Action)
	}
	before, beforeMissing, err := svc.FileSnapshotContent(got.Before)
	if err != nil {
		t.Fatal(err)
	}
	after, afterMissing, err := svc.FileSnapshotContent(got.After)
	if err != nil {
		t.Fatal(err)
	}
	if beforeMissing || afterMissing || before != "new\n" || after != "old\n" {
		t.Fatalf("snapshots before=%q missing=%v after=%q missing=%v", before, beforeMissing, after, afterMissing)
	}
}

func TestWriteSessionRevertCreatedFileRemovesIt(t *testing.T) {
	svc, cfg := newCLITestSession(t)
	path := filepath.Join(cfg.Workspace, "created.txt")
	if err := os.WriteFile(path, []byte("created\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecordFileRevisionWithMissing("app", "cli-user", "1", path, "write", "", "created\n", true, false); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := writeSessionRevert(&buf, svc, cfg, "cli-user", "1", path, true); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, `"reverted": true`) || !strings.Contains(out, `"action": "revert"`) {
		t.Fatalf("revert json = %s", out)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file should be removed, stat err = %v", err)
	}
	revisions, err := svc.FileRevisions("app", "cli-user", "1")
	if err != nil {
		t.Fatal(err)
	}
	got := revisions[len(revisions)-1]
	if got.Action != "revert" || !got.After.Missing {
		t.Fatalf("revision = %#v, want revert with missing after", got)
	}
	before, beforeMissing, err := svc.FileSnapshotContent(got.Before)
	if err != nil {
		t.Fatal(err)
	}
	_, afterMissing, err := svc.FileSnapshotContent(got.After)
	if err != nil {
		t.Fatal(err)
	}
	if beforeMissing || before != "created\n" || !afterMissing {
		t.Fatalf("snapshots before=%q missing=%v afterMissing=%v", before, beforeMissing, afterMissing)
	}
}

func TestWriteSessionCompact(t *testing.T) {
	svc, cfg := newCLITestSession(t)
	created, err := svc.Get(t.Context(), &adksession.GetRequest{AppName: "app", UserID: "cli-user", SessionID: "1"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		ev := adksession.NewEvent("inv-compact")
		ev.Author = "user"
		ev.Content = genai.NewContentFromText("compact message", genai.RoleUser)
		if err := svc.AppendEvent(t.Context(), created.Session, ev); err != nil {
			t.Fatal(err)
		}
	}

	var buf bytes.Buffer
	if err := writeSessionCompact(&buf, svc, cfg, "cli-user", "1", supermansession.CompactOptions{
		MaxMessages: 3,
		KeepLast:    2,
	}, false); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "Compacted session 1") || !strings.Contains(out, "summary=") {
		t.Fatalf("compact output = %s", out)
	}
	messages, err := svc.Messages("app", "cli-user", "1")
	if err != nil {
		t.Fatal(err)
	}
	summary := messages[len(messages)-1]
	if !summary.Summary || !strings.Contains(summary.Content, "Deterministic session summary") {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestWriteSessionCompactJSONNoop(t *testing.T) {
	svc, cfg := newCLITestSession(t)
	var buf bytes.Buffer
	if err := writeSessionCompact(&buf, svc, cfg, "cli-user", "1", supermansession.CompactOptions{
		MaxMessages: 100,
		KeepLast:    20,
	}, true); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, `"compacted": false`) || !strings.Contains(out, `"session_id": "1"`) {
		t.Fatalf("compact json = %s", out)
	}
}

func TestWriteSessionImportJSON(t *testing.T) {
	src, cfg := newCLITestSession(t)
	if _, err := src.EnqueuePrompt("app", "cli-user", "1", "queued import task"); err != nil {
		t.Fatal(err)
	}
	if err := src.RecordSessionReference("app", "cli-user", "1", supermansession.SessionReference{
		SessionID: "past",
		Role:      supermansession.MessageUser,
		Preview:   "historical cache decision",
	}); err != nil {
		t.Fatal(err)
	}
	var exported bytes.Buffer
	if err := writeSessionExport(&exported, src, cfg, "cli-user", "1", "json"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "session.json")
	if err := os.WriteFile(path, exported.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	dst, dstCfg := newEmptyCLITestSessionService(t)
	var out bytes.Buffer
	if err := writeSessionImport(&out, dst, dstCfg, "cli-user", path, false, false); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); !strings.Contains(got, "Imported session 1") {
		t.Fatalf("import output = %s", got)
	}
	messages, err := dst.Messages("app", "cli-user", "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0].Content != "hello from cli" {
		t.Fatalf("messages = %#v", messages)
	}
	queue, err := dst.PromptQueue("app", "cli-user", "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(queue) != 1 || queue[0].Content != "queued import task" {
		t.Fatalf("queue = %#v", queue)
	}
	refs, err := dst.SessionReferences("app", "cli-user", "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].SessionID != "past" || refs[0].Preview != "historical cache decision" {
		t.Fatalf("references = %#v", refs)
	}
}

func TestWriteSessionImportJSONLAndRejectDuplicate(t *testing.T) {
	src, cfg := newCLITestSession(t)
	if err := src.RecordSessionReference("app", "cli-user", "1", supermansession.SessionReference{
		SessionID: "past",
		Role:      supermansession.MessageUser,
		Preview:   "jsonl reference",
	}); err != nil {
		t.Fatal(err)
	}
	var exported bytes.Buffer
	if err := writeSessionExport(&exported, src, cfg, "cli-user", "1", "jsonl"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(path, exported.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	dst, dstCfg := newEmptyCLITestSessionService(t)
	var out bytes.Buffer
	if err := writeSessionImport(&out, dst, dstCfg, "cli-user", path, false, true); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); !strings.Contains(got, `"session_id": "1"`) {
		t.Fatalf("import json = %s", got)
	}
	refs, err := dst.SessionReferences("app", "cli-user", "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].SessionID != "past" || refs[0].Preview != "jsonl reference" {
		t.Fatalf("references = %#v", refs)
	}
	if err := writeSessionImport(&bytes.Buffer{}, dst, dstCfg, "cli-user", path, false, false); err == nil {
		t.Fatal("duplicate import should fail without overwrite")
	}
	if err := writeSessionImport(&bytes.Buffer{}, dst, dstCfg, "cli-user", path, true, false); err != nil {
		t.Fatalf("overwrite import: %v", err)
	}
}

func TestWriteSessionQueueAddAndClear(t *testing.T) {
	svc, cfg := newCLITestSession(t)
	var buf bytes.Buffer
	if err := writeSessionQueueAdd(&buf, svc, cfg, "cli-user", "1", "queued by cli", false); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "Queued prompt") || !strings.Contains(out, "1") {
		t.Fatalf("queue add output = %s", out)
	}
	queue, err := svc.PromptQueue("app", "cli-user", "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(queue) != 1 || queue[0].Content != "queued by cli" {
		t.Fatalf("queue = %#v", queue)
	}

	buf.Reset()
	if err := writeSessionQueueClear(&buf, svc, cfg, "cli-user", "1", false); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "Cleared 1") {
		t.Fatalf("queue clear output = %s", out)
	}
	queue, err = svc.PromptQueue("app", "cli-user", "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(queue) != 0 {
		t.Fatalf("queue after clear = %#v", queue)
	}
}

func TestWriteSessionQueueAddJSON(t *testing.T) {
	svc, cfg := newCLITestSession(t)
	var buf bytes.Buffer
	if err := writeSessionQueueAdd(&buf, svc, cfg, "cli-user", "1", "json prompt", true); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, `"content": "json prompt"`) || !strings.Contains(out, `"id":`) {
		t.Fatalf("queue add json = %s", out)
	}
}

func TestWriteSessionQueueClearJSON(t *testing.T) {
	svc, cfg := newCLITestSession(t)
	if _, err := svc.EnqueuePrompt("app", "cli-user", "1", "one"); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := writeSessionQueueClear(&buf, svc, cfg, "cli-user", "1", true); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, `"session_id": "1"`) || !strings.Contains(out, `"cleared": 1`) {
		t.Fatalf("queue clear json = %s", out)
	}
}

func TestWriteSessionStorageAndGC(t *testing.T) {
	svc, cfg := newCLITestSession(t)
	path := filepath.Join(cfg.Workspace, "main.go")
	if _, err := svc.RecordFileRevision("app", "cli-user", "1", path, "patch", "old", "new", false); err != nil {
		t.Fatal(err)
	}
	orphanPath := filepath.Join(cfg.Workspace, "sessions", "snapshots", "aa", "aa-orphan")
	if err := os.MkdirAll(filepath.Dir(orphanPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(orphanPath, []byte("orphan"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := writeSessionStorage(&buf, svc, cfg, false); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "orphan snapshots") || !strings.Contains(out, "snapshots") {
		t.Fatalf("storage output = %s", out)
	}

	buf.Reset()
	if err := writeSessionStorageGC(&buf, svc, cfg, true, true); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, `"dry_run": true`) || !strings.Contains(out, `"removed": 1`) {
		t.Fatalf("storage gc dry-run json = %s", out)
	}
	if _, err := os.Stat(orphanPath); err != nil {
		t.Fatalf("dry-run should keep orphan: %v", err)
	}

	buf.Reset()
	if err := writeSessionStorageGC(&buf, svc, cfg, false, false); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "Removed 1 orphan") {
		t.Fatalf("storage gc output = %s", out)
	}
	if _, err := os.Stat(orphanPath); !os.IsNotExist(err) {
		t.Fatalf("orphan should be removed, stat err = %v", err)
	}
}

func TestQueuePromptInputPriority(t *testing.T) {
	oldPrompt, oldFile := queuePrompt, queueFile
	t.Cleanup(func() {
		queuePrompt, queueFile = oldPrompt, oldFile
	})
	queuePrompt = "from flag"
	queueFile = ""
	got, err := queuePromptInput([]string{"1", "from arg"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "from flag" {
		t.Fatalf("prompt = %q, want flag value", got)
	}
}

func TestWriteSessionListJSON(t *testing.T) {
	svc, cfg := newCLITestSession(t)
	var buf bytes.Buffer
	if err := writeSessionList(&buf, svc, cfg, "cli-user", true); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, `"session_id": "1"`) || !strings.Contains(out, `"title": "Work"`) {
		t.Fatalf("session list json = %s", out)
	}
}

func TestWriteSessionLast(t *testing.T) {
	svc, cfg := newCLITestSession(t)
	created, err := svc.Create(t.Context(), &adksession.CreateRequest{AppName: "app", UserID: "cli-user", SessionID: "2"})
	if err != nil {
		t.Fatal(err)
	}
	ev := adksession.NewEvent("inv2")
	ev.Author = "user"
	ev.Content = genai.NewContentFromText("newer message", genai.RoleUser)
	if err := svc.AppendEvent(t.Context(), created.Session, ev); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := writeSessionLast(&buf, svc, cfg, "cli-user", false); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "newer message") {
		t.Fatalf("session last = %s", out)
	}
}

func TestWriteSessionRenameAndDelete(t *testing.T) {
	svc, cfg := newCLITestSession(t)
	var buf bytes.Buffer
	if err := writeSessionRename(&buf, svc, cfg, "cli-user", "1", "Renamed Work", false); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, `Renamed session 1 to "Renamed Work"`) {
		t.Fatalf("rename output = %s", out)
	}
	meta, err := svc.Metadata("app", "cli-user", "1")
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "Renamed Work" {
		t.Fatalf("title = %q", meta.Title)
	}

	buf.Reset()
	if err := writeSessionDelete(&buf, svc, cfg, "cli-user", "1", true); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, `"deleted": true`) || !strings.Contains(out, `"session_id": "1"`) {
		t.Fatalf("delete json = %s", out)
	}
	if _, err := svc.Metadata("app", "cli-user", "1"); err == nil {
		t.Fatal("session should be deleted")
	}
}

func TestResolveSessionIDPrefix(t *testing.T) {
	svc, cfg := newCLITestSession(t)
	if _, err := svc.Create(t.Context(), &adksession.CreateRequest{AppName: "app", UserID: "cli-user", SessionID: "123456"}); err != nil {
		t.Fatal(err)
	}
	got, err := resolveSessionID(svc, cfg, "cli-user", "123")
	if err != nil {
		t.Fatal(err)
	}
	if got != "123456" {
		t.Fatalf("resolved = %q, want 123456", got)
	}
}

func TestResolveSessionIDAmbiguousPrefix(t *testing.T) {
	svc, cfg := newCLITestSession(t)
	if _, err := svc.Create(t.Context(), &adksession.CreateRequest{AppName: "app", UserID: "cli-user", SessionID: "123456"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(t.Context(), &adksession.CreateRequest{AppName: "app", UserID: "cli-user", SessionID: "123789"}); err != nil {
		t.Fatal(err)
	}
	_, err := resolveSessionID(svc, cfg, "cli-user", "123")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("err = %v, want ambiguous", err)
	}
}

func newCLITestSession(t *testing.T) (*supermansession.Service, *config.Config) {
	t.Helper()
	svc, cfg := newEmptyCLITestSessionService(t)
	created, err := svc.Create(t.Context(), &adksession.CreateRequest{AppName: "app", UserID: "cli-user", SessionID: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Rename("app", "cli-user", "1", "Work"); err != nil {
		t.Fatal(err)
	}
	ev := adksession.NewEvent("inv")
	ev.Author = "user"
	ev.Content = genai.NewContentFromText("hello from cli", genai.RoleUser)
	if err := svc.AppendEvent(t.Context(), created.Session, ev); err != nil {
		t.Fatal(err)
	}
	return svc, cfg
}

func newEmptyCLITestSessionService(t *testing.T) (*supermansession.Service, *config.Config) {
	t.Helper()
	workspace := t.TempDir()
	cfg := &config.Config{
		Workspace: workspace,
		Session:   config.SessionConfig{AppName: "app"},
	}
	global.SetConfig(cfg)
	t.Cleanup(func() { global.SetConfig(nil) })
	svc, err := supermansession.NewService()
	if err != nil {
		t.Fatal(err)
	}
	extended := svc.(*supermansession.Service)
	return extended, cfg
}
