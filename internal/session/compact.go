package session

import (
	"fmt"
	"strings"
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
	if opts.MaxMessages <= 0 {
		opts.MaxMessages = 60
	}
	if opts.KeepLast <= 0 {
		opts.KeepLast = 20
	}
	if opts.MaxSummaryRunes <= 0 {
		opts.MaxSummaryRunes = 4000
	}

	messages, err := s.Messages(appName, userID, sessionID)
	if err != nil {
		return CompactResult{}, err
	}
	var nonSummary []Message
	for _, msg := range messages {
		if !msg.Summary {
			nonSummary = append(nonSummary, msg)
		}
	}
	result := CompactResult{Scanned: len(nonSummary), Kept: min(opts.KeepLast, len(nonSummary))}
	if len(nonSummary) <= opts.MaxMessages {
		return result, nil
	}

	cutoff := len(nonSummary) - opts.KeepLast
	if cutoff <= 0 {
		return result, nil
	}
	summaryText := buildDeterministicSummary(nonSummary[:cutoff], opts.MaxSummaryRunes)
	summary, err := s.SetSummary(appName, userID, sessionID, summaryText)
	if err != nil {
		return CompactResult{}, err
	}
	result.Compacted = true
	result.Summary = summary
	return result, nil
}

func buildDeterministicSummary(messages []Message, maxRunes int) string {
	var b strings.Builder
	b.WriteString("Deterministic session summary generated from older messages.\n\n")
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
	return strings.TrimSpace(b.String())
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
