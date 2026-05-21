package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
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
	mu        sync.RWMutex
	entries   map[string]*Entry
	l1Index   []string
	maxL1     int
	memoryDir string
}

func New(maxL1Entries int, memoryDir string) *Service {
	return &Service{
		entries:   make(map[string]*Entry),
		maxL1:     maxL1Entries,
		memoryDir: memoryDir,
	}
}

// LoadFromDisk reads L2 entries and L1 index from disk, rebuilding in-memory state.
func (s *Service) LoadFromDisk() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.memoryDir == "" {
		return nil
	}

	// Load L2 entries
	l2Path := filepath.Join(s.memoryDir, "l2_entries.jsonl")
	data, err := os.ReadFile(l2Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read l2 entries: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var entry Entry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			log.Printf("[memory] skip corrupt entry: %v", err)
			continue
		}
		s.entries[entry.ID] = &entry
	}

	// Load L1 index
	l1Path := filepath.Join(s.memoryDir, "l1_index.txt")
	l1Data, err := os.ReadFile(l1Path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read l1 index: %w", err)
	}
	if len(l1Data) > 0 {
		lines := strings.Split(strings.TrimSpace(string(l1Data)), "\n")
		for _, line := range lines {
			if line != "" && !strings.HasPrefix(line, "#") {
				s.l1Index = append(s.l1Index, line)
			}
		}
		if len(s.l1Index) > s.maxL1 {
			s.l1Index = s.l1Index[len(s.l1Index)-s.maxL1:]
		}
	}

	log.Printf("[memory] loaded %d entries, %d L1 index entries from %s", len(s.entries), len(s.l1Index), s.memoryDir)
	return nil
}

// GetL1Content returns the formatted L1 index for system prompt injection.
func (s *Service) GetL1Content() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.l1Index) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("# [Memory Index]\n")
	for _, entry := range s.l1Index {
		b.WriteString(entry)
		b.WriteByte('\n')
	}
	b.WriteString("L4: l4_archive/ historical sessions available\n")
	return b.String()
}

// persistL2Entries writes all L2 entries to disk as JSONL.
func (s *Service) persistL2Entries() error {
	path := filepath.Join(s.memoryDir, "l2_entries.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	encoder := json.NewEncoder(f)
	for _, entry := range s.entries {
		if entry.Layer == 2 {
			if err := encoder.Encode(entry); err != nil {
				return err
			}
		}
	}
	return nil
}

// persistL1Index writes L1 index to disk.
func (s *Service) persistL1Index() error {
	path := filepath.Join(s.memoryDir, "l1_index.txt")
	return os.WriteFile(path, []byte(s.formatL1Index()), 0644)
}

// formatL1Index builds the L1 text without the header.
func (s *Service) formatL1Index() string {
	if len(s.l1Index) == 0 {
		return ""
	}
	return strings.Join(s.l1Index, "\n") + "\n"
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

	// Persist to disk
	if s.memoryDir != "" {
		os.MkdirAll(s.memoryDir, 0755)
		l2Path := filepath.Join(s.memoryDir, "l2_entries.jsonl")
		f, err := os.OpenFile(l2Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("[memory] persist error: %v", err)
		} else {
			json.NewEncoder(f).Encode(entry)
			f.Close()
		}
		if err := s.persistL1Index(); err != nil {
			log.Printf("[memory] persist L1 error: %v", err)
		}
	}

	log.Printf("[memory] stored entry %s (category=%s, size=%d)", id, category, len(content))
	return entry, nil
}

// StoreString stores content and returns just the entry ID.
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
	cutoffUnix := cutoff.UnixNano()
	var archived int
	for _, e := range s.entries {
		if e.Layer == 2 && e.UpdatedAt.UnixNano() < cutoffUnix {
			e.Layer = 3
			e.Summary = summarize(e.Content, 50)
			archived++
		}
	}

	if archived > 0 && s.memoryDir != "" {
		os.MkdirAll(s.memoryDir, 0755)
		l3Path := filepath.Join(s.memoryDir, "l3_archive.jsonl")
		f, err := os.OpenFile(l3Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("[memory] L3 persist error: %v", err)
		} else {
			for _, e := range s.entries {
				if e.Layer == 3 && e.UpdatedAt.UnixNano() < cutoffUnix {
					json.NewEncoder(f).Encode(e)
				}
			}
			f.Close()
		}
		if err := s.persistL2Entries(); err != nil {
			log.Printf("[memory] L2 rewrite error: %v", err)
		}
		if err := s.persistL1Index(); err != nil {
			log.Printf("[memory] L1 persist error: %v", err)
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