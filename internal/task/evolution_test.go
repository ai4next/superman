package task

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/global"
	"github.com/ai4next/superman/internal/hook"
	"github.com/ai4next/superman/internal/permission"
	supermanruntime "github.com/ai4next/superman/internal/runtime"
)

func TestEvolutionLoopPublishesFailedEvent(t *testing.T) {
	global.SetConfig(&config.Config{Workspace: t.TempDir()})
	t.Cleanup(func() { global.SetConfig(nil) })

	broker := supermanruntime.NewBroker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := broker.Subscribe(ctx)

	e := &Evolution{signal: make(chan hook.EvolutionSignal, 1)}
	e.SetBroker(broker)
	go e.Loop(ctx)
	e.signal <- hook.EvolutionSignal{SessionID: "s1", Role: "superman"}

	var got []supermanruntime.Event
	deadline := time.After(time.Second)
	for len(got) < 2 {
		select {
		case event := <-events:
			got = append(got, event)
		case <-deadline:
			t.Fatalf("timed out waiting for evolution events: %+v", got)
		}
	}
	if got[0].Type != supermanruntime.EventEvolutionStarted {
		t.Fatalf("first event = %+v", got[0])
	}
	if got[1].Type != supermanruntime.EventEvolutionFailed || strings.TrimSpace(got[1].Error) == "" {
		t.Fatalf("second event = %+v", got[1])
	}
}

func TestEvolutionToolConfigSkipsConfirmations(t *testing.T) {
	cfg := evolutionToolConfig()
	policy := permission.NewPolicy(
		cfg.Permissions.SkipRequests,
		cfg.Permissions.AllowedTools,
		cfg.Permissions.RiskyTools,
	)
	for _, toolName := range []string{permission.ToolCodeRun, permission.ToolWrite, permission.ToolPatch} {
		if policy.RequiresConfirmation(permission.Request{ToolName: toolName}) {
			t.Fatalf("%s should not require confirmation in background evolution", toolName)
		}
	}
}
