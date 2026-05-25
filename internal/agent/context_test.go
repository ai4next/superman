package agent

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/global"
	"github.com/ai4next/superman/internal/session"
	adksession "google.golang.org/adk/session"
	"google.golang.org/genai"
)

type readonlyCtx struct {
	appName   string
	userID    string
	sessionID string
}

func (c readonlyCtx) Deadline() (deadline time.Time, ok bool) { return time.Time{}, false }
func (c readonlyCtx) Done() <-chan struct{}                   { return nil }
func (c readonlyCtx) Err() error                              { return nil }
func (c readonlyCtx) Value(key any) any                       { return nil }
func (c readonlyCtx) UserContent() *genai.Content             { return nil }
func (c readonlyCtx) InvocationID() string                    { return "inv" }
func (c readonlyCtx) AgentName() string                       { return "superman" }
func (c readonlyCtx) ReadonlyState() adksession.ReadonlyState { return nil }
func (c readonlyCtx) UserID() string                          { return c.userID }
func (c readonlyCtx) AppName() string                         { return c.appName }
func (c readonlyCtx) SessionID() string                       { return c.sessionID }
func (c readonlyCtx) Branch() string                          { return "" }

func TestInstructionProviderInjectsSessionContext(t *testing.T) {
	setTestWorkspace(t)
	svc, err := session.NewService()
	if err != nil {
		t.Fatal(err)
	}
	created, err := svc.Create(t.Context(), &adksession.CreateRequest{AppName: "app", UserID: "user", SessionID: "1"})
	if err != nil {
		t.Fatal(err)
	}
	ev := adksession.NewEvent("inv")
	ev.Author = "user"
	ev.Content = genai.NewContentFromText("remember this important detail", genai.RoleUser)
	if err := svc.AppendEvent(t.Context(), created.Session, ev); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetSummary("app", "user", "1", "prior concise summary"); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "main.go")
	if err := svc.RecordFileRead("app", "user", "1", file); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecordFileWrite("app", "user", "1", file); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecordSessionReference("app", "user", "1", session.SessionReference{
		SessionID: "past-session",
		Role:      session.MessageUser,
		Preview:   "historical cache decision",
	}); err != nil {
		t.Fatal(err)
	}
	provider := instructionProvider(BuildConfig{
		Instruction:     "base",
		SessionService:  svc,
		ContextMessages: 4,
	})
	got, err := provider(readonlyCtx{appName: "app", userID: "user", sessionID: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "prior concise summary") {
		t.Fatalf("missing summary in instruction:\n%s", got)
	}
	if !strings.Contains(got, "## Session Context Usage") || !strings.Contains(got, "Working files are path/status pointers only") || !strings.Contains(got, "Session references are user-selected historical pointers") {
		t.Fatalf("missing context usage guidance in instruction:\n%s", got)
	}
	if !strings.Contains(got, "remember this important detail") {
		t.Fatalf("missing recent context in instruction:\n%s", got)
	}
	if !strings.Contains(got, "## Session Working Files") || !strings.Contains(got, file) || !strings.Contains(got, "modified") {
		t.Fatalf("missing working files in instruction:\n%s", got)
	}
	if !strings.Contains(got, "## Session References") || !strings.Contains(got, "past-session [user]") || !strings.Contains(got, "historical cache decision") {
		t.Fatalf("missing session references in instruction:\n%s", got)
	}
}

func TestInstructionProviderSkipsSessionUsageWhenContextEmpty(t *testing.T) {
	setTestWorkspace(t)
	svc, err := session.NewService()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(t.Context(), &adksession.CreateRequest{AppName: "app", UserID: "user", SessionID: "empty"}); err != nil {
		t.Fatal(err)
	}
	provider := instructionProvider(BuildConfig{
		Instruction:     "base",
		SessionService:  svc,
		ContextMessages: 4,
	})
	got, err := provider(readonlyCtx{appName: "app", userID: "user", sessionID: "empty"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "## Session Context Usage") {
		t.Fatalf("empty context should not inject usage guidance:\n%s", got)
	}
}

func setTestWorkspace(t *testing.T) {
	t.Helper()
	global.SetConfig(&config.Config{Workspace: t.TempDir()})
	t.Cleanup(func() { global.SetConfig(nil) })
}
