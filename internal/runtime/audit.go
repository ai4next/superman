package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type AuditLogger struct {
	path string
	mu   sync.Mutex
}

func NewAuditLogger(path string) *AuditLogger {
	return &AuditLogger{path: path}
}

func (l *AuditLogger) Subscribe(ctx context.Context, broker *Broker) {
	if l == nil || broker == nil {
		return
	}
	events := broker.Subscribe(ctx)
	go func() {
		for event := range events {
			if err := l.Write(event); err != nil {
				// Audit logging must not break agent execution.
				continue
			}
		}
	}()
}

func (l *AuditLogger) Write(event Event) error {
	if l == nil || l.path == "" {
		return nil
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(l.path), 0755); err != nil {
		return fmt.Errorf("create audit dir: %w", err)
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}
	return nil
}

type AuditFilter struct {
	SessionID string
	RunID     string
	Types     []EventType
	Limit     int
}

type AuditSummary struct {
	Events        int                  `json:"events"`
	ByType        map[EventType]int    `json:"by_type"`
	Sessions      map[string]int       `json:"sessions,omitempty"`
	Runs          map[string]int       `json:"runs,omitempty"`
	Tools         map[string]int       `json:"tools,omitempty"`
	Errors        int                  `json:"errors"`
	FirstAt       time.Time            `json:"first_at,omitempty"`
	LastAt        time.Time            `json:"last_at,omitempty"`
	Duration      string               `json:"duration,omitempty"`
	LastBySession map[string]time.Time `json:"last_by_session,omitempty"`
}

func ReadAuditLog(path string, filter AuditFilter) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open audit log: %w", err)
	}
	defer f.Close()
	return DecodeAuditEvents(f, filter)
}

func DecodeAuditEvents(r io.Reader, filter AuditFilter) ([]Event, error) {
	typeFilter := make(map[EventType]struct{}, len(filter.Types))
	for _, typ := range filter.Types {
		if typ == "" {
			continue
		}
		typeFilter[typ] = struct{}{}
	}
	scanner := bufio.NewScanner(r)
	var events []Event
	line := 0
	for scanner.Scan() {
		line++
		text := scanner.Bytes()
		if len(text) == 0 {
			continue
		}
		var event Event
		if err := json.Unmarshal(text, &event); err != nil {
			return nil, fmt.Errorf("decode audit event line %d: %w", line, err)
		}
		if filter.SessionID != "" && event.SessionID != filter.SessionID {
			continue
		}
		if filter.RunID != "" && event.RunID != filter.RunID {
			continue
		}
		if len(typeFilter) > 0 {
			if _, ok := typeFilter[event.Type]; !ok {
				continue
			}
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan audit log: %w", err)
	}
	if filter.Limit > 0 && len(events) > filter.Limit {
		events = events[len(events)-filter.Limit:]
	}
	return events, nil
}

func SummarizeAuditEvents(events []Event) AuditSummary {
	summary := AuditSummary{
		ByType:        make(map[EventType]int),
		Sessions:      make(map[string]int),
		Runs:          make(map[string]int),
		Tools:         make(map[string]int),
		LastBySession: make(map[string]time.Time),
	}
	for _, event := range events {
		summary.Events++
		summary.ByType[event.Type]++
		if event.SessionID != "" {
			summary.Sessions[event.SessionID]++
			if event.At.After(summary.LastBySession[event.SessionID]) {
				summary.LastBySession[event.SessionID] = event.At
			}
		}
		if event.RunID != "" {
			summary.Runs[event.RunID]++
		}
		if event.ToolName != "" {
			summary.Tools[event.ToolName]++
		}
		if event.Error != "" || event.Type == EventRunFailed || event.Type == EventEvolutionFailed {
			summary.Errors++
		}
		if !event.At.IsZero() {
			if summary.FirstAt.IsZero() || event.At.Before(summary.FirstAt) {
				summary.FirstAt = event.At
			}
			if summary.LastAt.IsZero() || event.At.After(summary.LastAt) {
				summary.LastAt = event.At
			}
		}
	}
	if !summary.FirstAt.IsZero() && !summary.LastAt.IsZero() {
		summary.Duration = summary.LastAt.Sub(summary.FirstAt).String()
	}
	if len(summary.Sessions) == 0 {
		summary.Sessions = nil
	}
	if len(summary.Runs) == 0 {
		summary.Runs = nil
	}
	if len(summary.Tools) == 0 {
		summary.Tools = nil
	}
	if len(summary.LastBySession) == 0 {
		summary.LastBySession = nil
	}
	return summary
}
