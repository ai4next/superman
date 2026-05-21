package memory

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

type Entry struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Summary   string    `json:"summary"`
	Layer     int       `json:"layer"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Service struct {
	mu      sync.RWMutex
	entries map[string]*Entry
	l1Index []string
	maxL1   int
}

func New(maxL1Entries int) *Service {
	return &Service{
		entries: make(map[string]*Entry),
		maxL1:   maxL1Entries,
	}
}

func (s *Service) Store(ctx context.Context, content, category string) (*Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("mem-%d", time.Now().UnixNano())
	entry := &Entry{
		ID:        id,
		Content:   content,
		Summary:   summarize(content, 100),
		Layer:     2,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.entries[id] = entry

	l1Summary := summarize(content, 80)
	s.updateL1Index(l1Summary)

	log.Printf("[memory] stored entry %s (category=%s, size=%d)", id, category, len(content))
	return entry, nil
}

// StoreString stores content and returns just the entry ID.
// This implements the tools.MemoryStorer interface.
func (s *Service) StoreString(ctx context.Context, content, category string) (string, error) {
	entry, err := s.Store(ctx, content, category)
	if err != nil {
		return "", err
	}
	return entry.ID, nil
}

func (s *Service) Search(ctx context.Context, query string) ([]*Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Entry
	for _, entry := range s.entries {
		if containsIgnoreCase(entry.Content, query) || containsIgnoreCase(entry.Summary, query) {
			results = append(results, entry)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].UpdatedAt.After(results[j].UpdatedAt)
	})
	return results, nil
}

func (s *Service) GetL1Index() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, len(s.l1Index))
	copy(result, s.l1Index)
	return result
}

func (s *Service) GetL2Entries() []*Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Entry
	for _, e := range s.entries {
		if e.Layer == 2 {
			result = append(result, e)
		}
	}
	return result
}

func (s *Service) Archive(ctx context.Context, olderThan time.Duration) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	var archived int
	for _, e := range s.entries {
		if e.Layer == 2 && e.UpdatedAt.Before(cutoff) {
			e.Layer = 3
			e.Summary = summarize(e.Content, 50)
			archived++
		}
	}
	log.Printf("[memory] archived %d entries to L3", archived)
	return archived, nil
}

func (s *Service) updateL1Index(summary string) {
	s.l1Index = append(s.l1Index, summary)
	if len(s.l1Index) > s.maxL1 {
		s.l1Index = s.l1Index[len(s.l1Index)-s.maxL1:]
	}
}

func summarize(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	trimmed := content[:maxLen]
	for i := len(trimmed) - 1; i > 0; i-- {
		if trimmed[i] == ' ' {
			return trimmed[:i] + "..."
		}
	}
	return trimmed + "..."
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}