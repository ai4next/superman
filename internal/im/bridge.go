package im

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"github.com/ai4next/superman/internal/config"
	supermanruntime "github.com/ai4next/superman/internal/runtime"
	adkrunner "google.golang.org/adk/runner"
	adksession "google.golang.org/adk/session"
	"google.golang.org/genai"
)

type AgentBridge struct {
	Runner         *adkrunner.Runner
	SessionService adksession.Service
	Config         *config.Config
	Logger         *slog.Logger

	mu      sync.Mutex
	running map[string]struct{}
}

func NewAgentBridge(run *adkrunner.Runner, sessionService adksession.Service, cfg *config.Config, logger *slog.Logger) (*AgentBridge, error) {
	if run == nil {
		return nil, errors.New("im: runner is required")
	}
	if cfg == nil {
		return nil, errors.New("im: config is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &AgentBridge{
		Runner:         run,
		SessionService: sessionService,
		Config:         cfg,
		Logger:         logger,
		running:        make(map[string]struct{}),
	}, nil
}

func (b *AgentBridge) Handler() Handler {
	return b.HandleMessage
}

func (b *AgentBridge) HandleMessage(ctx context.Context, client *Client, platform Platform, msg *Message) {
	if msg == nil || client == nil {
		return
	}
	prompt := incomingPrompt(msg)
	if strings.TrimSpace(prompt) == "" {
		return
	}

	sessionID := SessionID(msg)
	if !b.tryStart(sessionID) {
		if err := client.Reply(ctx, msg, "上一条消息还在处理中，请稍后再发。"); err != nil {
			b.Logger.Warn("im: busy reply failed", "platform", msg.Platform, "session", sessionID, "error", err)
		}
		return
	}

	go func() {
		defer b.done(sessionID)
		if err := b.runAndReply(ctx, client, msg, sessionID, prompt); err != nil {
			b.Logger.Error("im: run failed", "platform", msg.Platform, "session", sessionID, "error", err)
			if replyErr := client.Reply(ctx, msg, "运行失败: "+err.Error()); replyErr != nil {
				b.Logger.Warn("im: error reply failed", "platform", msg.Platform, "session", sessionID, "error", replyErr)
			}
		}
	}()
}

func (b *AgentBridge) runAndReply(ctx context.Context, client *Client, msg *Message, sessionID, prompt string) error {
	if err := b.ensureSession(ctx, msg, sessionID); err != nil {
		return err
	}

	var out strings.Builder
	req := supermanruntime.RunRequest{
		AppName:    b.Config.Session.AppName,
		UserID:     UserID(msg),
		SessionID:  sessionID,
		Message:    genai.NewContentFromText(prompt, genai.RoleUser),
		StateDelta: supermanruntime.PromptStateDelta(b.Config.Workspace, prompt),
		LoopDetection: supermanruntime.LoopDetectionConfig{
			Enabled:    b.Config.Session.LoopDetection.Enabled,
			WindowSize: b.Config.Session.LoopDetection.WindowSize,
			MaxRepeats: b.Config.Session.LoopDetection.MaxRepeats,
		},
		Compact: supermanruntime.SessionCompactor(b.SessionService, b.Config.Session.MaxTurns),
	}

	for event, err := range supermanruntime.StreamRun(ctx, b.Runner, req, nil) {
		if err != nil {
			return err
		}
		switch event.Type {
		case supermanruntime.EventTextDelta:
			out.WriteString(event.Text)
		case supermanruntime.EventPermissionRequested:
			return fmt.Errorf("工具 %s 需要人工确认；当前 IM 接入暂不支持确认流程", firstNonEmpty(event.ToolName, event.ToolID))
		case supermanruntime.EventRunFailed:
			if event.Error != "" {
				return errors.New(event.Error)
			}
			return errors.New("run failed")
		}
	}

	reply := strings.TrimSpace(out.String())
	if reply == "" {
		reply = "已完成。"
	}
	return client.Reply(ctx, msg, reply)
}

func (b *AgentBridge) ensureSession(ctx context.Context, msg *Message, sessionID string) error {
	if b.SessionService == nil {
		return nil
	}
	userID := UserID(msg)
	if _, err := b.SessionService.Get(ctx, &adksession.GetRequest{
		AppName:   b.Config.Session.AppName,
		UserID:    userID,
		SessionID: sessionID,
	}); err == nil {
		return nil
	}
	created, err := b.SessionService.Create(ctx, &adksession.CreateRequest{
		AppName:   b.Config.Session.AppName,
		UserID:    userID,
		SessionID: sessionID,
	})
	if err != nil {
		return fmt.Errorf("create IM session: %w", err)
	}
	if created != nil && created.Session != nil && created.Session.ID() != sessionID {
		b.Logger.Debug("im: session id normalized", "requested", sessionID, "actual", created.Session.ID())
	}
	return nil
}

func (b *AgentBridge) tryStart(sessionID string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.running[sessionID]; ok {
		return false
	}
	b.running[sessionID] = struct{}{}
	return true
}

func (b *AgentBridge) done(sessionID string) {
	b.mu.Lock()
	delete(b.running, sessionID)
	b.mu.Unlock()
}

func incomingPrompt(msg *Message) string {
	parts := make([]string, 0, 3)
	if msg.ExtraContent != "" {
		parts = append(parts, msg.ExtraContent)
	}
	if msg.Content != "" {
		parts = append(parts, msg.Content)
	}
	if len(msg.Images) > 0 {
		parts = append(parts, fmt.Sprintf("[收到 %d 张图片]", len(msg.Images)))
	}
	if len(msg.Files) > 0 {
		names := make([]string, 0, len(msg.Files))
		for _, f := range msg.Files {
			if f.FileName != "" {
				names = append(names, f.FileName)
			}
		}
		if len(names) == 0 {
			parts = append(parts, fmt.Sprintf("[收到 %d 个文件]", len(msg.Files)))
		} else {
			parts = append(parts, "[收到文件: "+strings.Join(names, ", ")+"]")
		}
	}
	if msg.Audio != nil {
		parts = append(parts, "[收到语音消息]")
	}
	if msg.Location != nil {
		parts = append(parts, fmt.Sprintf("[收到位置: %.6f, %.6f]", msg.Location.Latitude, msg.Location.Longitude))
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func SessionID(msg *Message) string {
	key := strings.TrimSpace(msg.SessionKey)
	if key == "" {
		key = strings.Join([]string{msg.Platform, msg.ChannelKey, msg.UserID}, ":")
	}
	sum := sha1.Sum([]byte(key))
	id := int64(binary.BigEndian.Uint64(sum[:8]) & 0x7fffffffffffffff)
	if id == 0 {
		id = 1
	}
	return strconv.FormatInt(id, 10)
}

func UserID(msg *Message) string {
	if msg == nil {
		return "im-user"
	}
	parts := []string{"im", msg.Platform, msg.UserID}
	return strings.Trim(strings.Join(nonEmpty(parts), ":"), ":")
}

func nonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, strings.TrimSpace(value))
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
