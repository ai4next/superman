package session

import (
	"context"
	"errors"
	"fmt"
	"testing"

	adksession "google.golang.org/adk/session"
	"google.golang.org/genai"
)

func TestCompactDoesNotRepeatWithoutNewMessages(t *testing.T) {
	svc := adksession.InMemoryService()
	created, err := svc.Create(t.Context(), &adksession.CreateRequest{
		AppName: "app",
		UserID:  "user",
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 5 {
		event := adksession.NewEvent(fmt.Sprintf("message-%d", i))
		event.Author = "user"
		event.Content = genai.NewContentFromText(fmt.Sprintf("message %d", i), genai.RoleUser)
		if err := svc.AppendEvent(t.Context(), created.Session, event); err != nil {
			t.Fatal(err)
		}
	}

	opts := CompactOptions{MaxMessages: 3, KeepLast: 2}
	first, err := CompactContext(t.Context(), svc, "app", "user", created.Session.ID(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Compacted || first.Scanned != 5 || first.Kept != 2 {
		t.Fatalf("first compaction = %#v", first)
	}
	second, err := CompactContext(t.Context(), svc, "app", "user", created.Session.ID(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if second.Compacted || second.Scanned != 2 {
		t.Fatalf("second compaction = %#v, want no-op with two active messages", second)
	}

	messages, err := Messages(svc, "app", "user", created.Session.ID())
	if err != nil {
		t.Fatal(err)
	}
	summaries := 0
	for _, message := range messages {
		if message.Summary {
			summaries++
		}
	}
	if summaries != 1 {
		t.Fatalf("summary count = %d, want 1", summaries)
	}
	resp, err := svc.Get(t.Context(), &adksession.GetRequest{AppName: "app", UserID: "user", SessionID: created.Session.ID()})
	if err != nil {
		t.Fatal(err)
	}
	metadata := MetadataForSession(resp.Session)
	if metadata.SummaryMessageID == "" {
		t.Fatal("summary message id was not recorded in session state")
	}
	for _, message := range messages {
		if message.Summary && message.ID != metadata.SummaryMessageID {
			t.Fatalf("summary message id = %q, metadata id = %q", message.ID, metadata.SummaryMessageID)
		}
	}
}

func TestCompactCapsKeepLastAtMaxMessages(t *testing.T) {
	svc := adksession.InMemoryService()
	created, err := svc.Create(t.Context(), &adksession.CreateRequest{AppName: "app", UserID: "user"})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 5 {
		event := adksession.NewEvent(fmt.Sprintf("message-%d", i))
		event.Author = "user"
		event.Content = genai.NewContentFromText("message", genai.RoleUser)
		if err := svc.AppendEvent(t.Context(), created.Session, event); err != nil {
			t.Fatal(err)
		}
	}

	result, err := CompactContext(t.Context(), svc, "app", "user", created.Session.ID(), CompactOptions{
		MaxMessages: 3,
		KeepLast:    100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Compacted || result.Kept != 3 {
		t.Fatalf("compaction = %#v, want compacted with three retained messages", result)
	}
}

func TestCompactContextHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := CompactContext(ctx, nil, "app", "user", "session", CompactOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CompactContext() error = %v, want context.Canceled", err)
	}
}

func TestSessionServiceRejectsCanceledRequestsBeforeValidation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	svc := &Service{}

	if _, err := svc.Create(ctx, &adksession.CreateRequest{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Create() error = %v, want context.Canceled", err)
	}
	if err := svc.AppendEvent(ctx, nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("AppendEvent() error = %v, want context.Canceled", err)
	}
}

func TestProjectEventDoesNotTreatControlFlowAsSummary(t *testing.T) {
	event := adksession.NewEvent("permission-request")
	event.Author = "assistant"
	event.Content = genai.NewContentFromText("confirmation required", genai.RoleModel)
	event.Actions.SkipSummarization = true

	messages := ProjectEvent("session", event)
	if len(messages) != 1 || messages[0].Summary {
		t.Fatalf("projected messages = %#v, want a regular assistant message", messages)
	}
}
