package memory

import (
	"path/filepath"
	"testing"
)

func TestMailboxSendPendingAndMark(t *testing.T) {
	workspace := t.TempDir()
	setTestConfig(t, workspace)
	writeFile(t, filepath.Join(workspace, "state", "reviewer", "soul.md"), "review")

	svc := NewMailboxService(nil)
	msg, err := svc.Send(MailboxMessage{
		Recipient: "reviewer",
		Sender:    OwnerSuperman,
		Content:   "Remember cache invalidation policy.",
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if msg.ID == "" || msg.Status != MailboxStatusPending {
		t.Fatalf("unexpected message: %+v", msg)
	}

	pending, err := svc.Pending("reviewer", 10)
	if err != nil {
		t.Fatalf("Pending returned error: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != msg.ID {
		t.Fatalf("pending = %+v, want message %s", pending, msg.ID)
	}

	if err := svc.Mark(MailboxUpdate{ID: msg.ID, Status: MailboxStatusAccepted}); err != nil {
		t.Fatalf("Mark returned error: %v", err)
	}
	pending, err = svc.Pending("reviewer", 10)
	if err != nil {
		t.Fatalf("Pending returned error: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("pending after mark = %+v, want empty", pending)
	}
}

func TestMailboxRejectsUnknownRecipient(t *testing.T) {
	workspace := t.TempDir()
	setTestConfig(t, workspace)

	_, err := NewMailboxService(nil).Send(MailboxMessage{Recipient: "missing", Content: "x"})
	if err == nil {
		t.Fatalf("expected unknown recipient error")
	}
}
