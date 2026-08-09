package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	adksession "google.golang.org/adk/session"
	"google.golang.org/genai"
)

type CompactOptions struct {
	MaxMessages     int
	KeepLast        int
	MaxSummaryRunes int
}

type CompactResult struct {
	Compacted bool
	Summary   Message
	Scanned   int
	Kept      int
}

func (s *Service) Compact(appName, userID, sessionID string, opts CompactOptions) (CompactResult, error) {
	return Compact(s, appName, userID, sessionID, opts)
}

func (s *Service) CompactContext(ctx context.Context, appName, userID, sessionID string, opts CompactOptions) (CompactResult, error) {
	return CompactContext(ctx, s, appName, userID, sessionID, opts)
}

func Compact(svc adksession.Service, appName, userID, sessionID string, opts CompactOptions) (CompactResult, error) {
	return CompactContext(context.Background(), svc, appName, userID, sessionID, opts)
}

func CompactContext(ctx context.Context, svc adksession.Service, appName, userID, sessionID string, opts CompactOptions) (CompactResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return CompactResult{}, err
	}
	if svc == nil {
		return CompactResult{}, fmt.Errorf("session service is required")
	}
	if opts.MaxMessages <= 0 {
		opts.MaxMessages = 60
	}
	if opts.KeepLast <= 0 {
		opts.KeepLast = 20
	}
	if opts.MaxSummaryRunes <= 0 {
		opts.MaxSummaryRunes = 4000
	}
	if opts.KeepLast > opts.MaxMessages {
		opts.KeepLast = opts.MaxMessages
	}

	resp, err := svc.Get(ctx, &adksession.GetRequest{AppName: appName, UserID: userID, SessionID: sessionID})
	if err != nil {
		return CompactResult{}, err
	}
	var previousSummary string
	var active []Message
	for event := range resp.Session.Events().All() {
		if err := ctx.Err(); err != nil {
			return CompactResult{}, err
		}
		for _, msg := range ProjectEvent(sessionID, event) {
			if msg.Summary {
				previousSummary = msg.Content
				if len(active) > opts.KeepLast {
					active = append([]Message(nil), active[len(active)-opts.KeepLast:]...)
				}
				continue
			}
			active = append(active, msg)
		}
	}
	result := CompactResult{Scanned: len(active), Kept: min(opts.KeepLast, len(active))}
	if len(active) <= opts.MaxMessages {
		return result, nil
	}

	cutoff := len(active) - opts.KeepLast
	if cutoff <= 0 {
		return result, nil
	}
	summaryText := buildDeterministicSummary(previousSummary, active[:cutoff], opts.MaxSummaryRunes)
	now := time.Now()
	summaryID := "summary-" + formatStoredTime(now)
	summary := Message{
		ID:        summaryID,
		SessionID: sessionID,
		Role:      MessageAssistant,
		Content:   summaryText,
		Summary:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	event := adksession.NewEvent(summaryID)
	event.Author = "assistant"
	event.Content = genai.NewContentFromText(summaryText, genai.RoleModel)
	event.Actions.SkipSummarization = true
	event.Actions.StateDelta = map[string]any{sessionStateSummaryMessageID: summaryID}
	if err := svc.AppendEvent(ctx, resp.Session, event); err != nil {
		return CompactResult{}, err
	}
	result.Compacted = true
	result.Summary = summary
	return result, nil
}

func buildDeterministicSummary(previous string, messages []Message, maxRunes int) string {
	var b strings.Builder
	b.WriteString("Deterministic session summary generated from older messages.\n\n")
	previous = strings.Join(strings.Fields(previous), " ")
	if previous != "" {
		previousRunes := []rune(previous)
		previousBudget := max(maxRunes/2, 240)
		if len(previousRunes) > previousBudget {
			previous = string(previousRunes[:previousBudget-3]) + "..."
		}
		b.WriteString("Previous summary: ")
		b.WriteString(previous)
		b.WriteString("\n\n")
	}
	for _, msg := range messages {
		line := compactMessageLine(msg)
		if line == "" {
			continue
		}
		if b.Len() > 0 {
			nextLen := len([]rune(b.String())) + len([]rune(line)) + 3
			if maxRunes > 0 && nextLen > maxRunes {
				b.WriteString("- ... older context truncated during compaction\n")
				break
			}
		}
		b.WriteString("- ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	summary := []rune(strings.TrimSpace(b.String()))
	if maxRunes > 0 && len(summary) > maxRunes {
		summary = summary[:maxRunes]
	}
	return strings.TrimSpace(string(summary))
}

func compactMessageLine(msg Message) string {
	role := string(msg.Role)
	if msg.ToolName != "" {
		role += "/" + msg.ToolName
	}
	var body string
	switch {
	case strings.TrimSpace(msg.Content) != "":
		body = msg.Content
	case strings.TrimSpace(msg.Result) != "":
		body = "result: " + msg.Result
	case strings.TrimSpace(msg.Args) != "":
		body = "args: " + msg.Args
	default:
		body = msg.Status
	}
	body = strings.Join(strings.Fields(body), " ")
	if body == "" {
		return ""
	}
	if len([]rune(body)) > 240 {
		body = string([]rune(body)[:239]) + "..."
	}
	if msg.Status != "" && msg.Role == MessageTool {
		return fmt.Sprintf("%s [%s]: %s", role, msg.Status, body)
	}
	return fmt.Sprintf("%s: %s", role, body)
}
