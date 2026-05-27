package im

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"

	cccore "github.com/chenhg5/cc-connect/core"
)

type stubPlatform struct {
	name    string
	handler cccore.MessageHandler

	mu      sync.Mutex
	stopped bool
	replies []string
}

func (p *stubPlatform) Name() string { return p.name }

func (p *stubPlatform) Start(handler cccore.MessageHandler) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handler = handler
	return nil
}

func (p *stubPlatform) Reply(ctx context.Context, replyCtx any, content string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.replies = append(p.replies, content)
	return nil
}

func (p *stubPlatform) Send(ctx context.Context, replyCtx any, content string) error {
	return nil
}

func (p *stubPlatform) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopped = true
	return nil
}

func (p *stubPlatform) emit(msg *cccore.Message) {
	p.mu.Lock()
	handler := p.handler
	p.mu.Unlock()
	handler(p, msg)
}

type failingPlatform struct {
	stubPlatform
	err error
}

func (p *failingPlatform) Start(cccore.MessageHandler) error {
	return p.err
}

func TestClientDispatchAndReply(t *testing.T) {
	got := make(chan *Message, 1)
	c, err := NewClient(func(ctx context.Context, c *Client, p Platform, msg *Message) {
		got <- msg
		if err := c.Reply(ctx, msg, "pong"); err != nil {
			t.Errorf("Reply() error = %v", err)
		}
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	p := &stubPlatform{name: "stub"}
	if err := c.AddPlatform(p); err != nil {
		t.Fatalf("AddPlatform() error = %v", err)
	}
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	p.emit(&cccore.Message{Platform: "stub", Content: "ping", ReplyCtx: "ctx"})
	msg := <-got
	if msg.Content != "ping" {
		t.Fatalf("Content = %q, want ping", msg.Content)
	}
	if len(p.replies) != 1 || p.replies[0] != "pong" {
		t.Fatalf("replies = %#v, want [pong]", p.replies)
	}
}

func TestClientStartRollback(t *testing.T) {
	c, err := NewClient(func(context.Context, *Client, Platform, *Message) {})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	ok := &stubPlatform{name: "ok"}
	fail := &failingPlatform{stubPlatform: stubPlatform{name: "zz-fail"}, err: errors.New("boom")}
	if err := c.AddPlatform(ok); err != nil {
		t.Fatalf("AddPlatform(ok) error = %v", err)
	}
	if err := c.AddPlatform(fail); err != nil {
		t.Fatalf("AddPlatform(fail) error = %v", err)
	}
	if err := c.Start(context.Background()); err == nil {
		t.Fatal("Start() error = nil, want error")
	}
	if !ok.stopped {
		t.Fatal("started platform was not stopped on rollback")
	}
}

func TestSessionIDStable(t *testing.T) {
	msg := &Message{SessionKey: "telegram:chat:user", Platform: "telegram", UserID: "user"}
	first := SessionID(msg)
	second := SessionID(msg)
	if first == "" || first != second {
		t.Fatalf("SessionID unstable: %q vs %q", first, second)
	}
	if id, err := strconv.ParseInt(first, 10, 64); err != nil || id <= 0 {
		t.Fatalf("SessionID() = %q, want positive integer", first)
	}
	if got := UserID(msg); got != "im:telegram:user" {
		t.Fatalf("UserID() = %q, want im:telegram:user", got)
	}
}
