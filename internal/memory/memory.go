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
	ID             string    `json:"id"`
	Content        string    `json:"content"`
	Summary        string    `json:"summary"`
	Category       string    `json:"category,omitempty"`
	Scope          string    `json:"scope,omitempty"`
	Source         string    `json:"source,omitempty"`
	Tags           []string  `json:"tags,omitempty"`
	Layer          int       `json:"layer"`
	Importance     float64   `json:"importance,omitempty"`
	Confidence     float64   `json:"confidence,omitempty"`
	AccessCount    int       `json:"access_count,omitempty"`
	Supersedes     []string  `json:"supersedes,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	LastAccessedAt time.Time `json:"last_accessed_at,omitempty"`
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

// LoadFromDisk reads L2 entries and L3 archive from disk, rebuilding in-memory state.
func (s *Service) LoadFromDisk() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.memoryDir == "" {
		return nil
	}

	s.entries = make(map[string]*Entry)
	s.l1Index = nil

	// Ensure all layer subdirectories exist
	for _, dir := range []string{"l1", "l2", "l3", "l4"} {
		if err := os.MkdirAll(filepath.Join(s.memoryDir, dir), 0755); err != nil {
			return fmt.Errorf("create %s dir: %w", dir, err)
		}
	}

	if err := s.loadEntriesFile(filepath.Join(s.memoryDir, "l2", "entries.jsonl"), 2); err != nil {
		return err
	}
	if err := s.loadEntriesFile(filepath.Join(s.memoryDir, "l3", "archive.jsonl"), 3); err != nil {
		return err
	}

	s.rebuildL1IndexLocked()
	if err := s.persistL1Index(); err != nil {
		return fmt.Errorf("persist rebuilt l1 index: %w", err)
	}

	log.Printf("[memory] loaded %d entries, %d L1 index entries from %s", len(s.entries), len(s.l1Index), s.memoryDir)
	return nil
}

func (s *Service) loadEntriesFile(path string, defaultLayer int) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read memory entries: %w", err)
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
		normalizeEntry(&entry, defaultLayer)
		s.entries[entry.ID] = &entry
	}
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
	b.WriteString("## Memory Index\n")
	for _, entry := range s.l1Index {
		b.WriteString(entry)
		b.WriteByte('\n')
	}
	b.WriteString("L4: l4/ historical sessions available\n")
	return b.String()
}

// persistL2Entries writes all L2 entries to disk as JSONL.
func (s *Service) persistL2Entries() error {
	if s.memoryDir == "" {
		return nil
	}
	dir := filepath.Join(s.memoryDir, "l2")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "entries.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	encoder := json.NewEncoder(f)
	entries := s.sortedEntriesLocked(2)
	for _, entry := range entries {
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
	if s.memoryDir == "" {
		return nil
	}
	dir := filepath.Join(s.memoryDir, "l1")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "index.txt")
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

	now := time.Now()
	category = normalizeCategory(category)
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("memory content is empty")
	}

	if existing := s.findDuplicateLocked(content, category); existing != nil {
		existing.UpdatedAt = now
		existing.AccessCount++
		existing.Importance = clamp(existing.Importance+0.05, 0, 1)
		if existing.Summary == "" || len(content) < len(existing.Content) {
			existing.Summary = summarize(content, 120)
		}
		existing.Tags = mergeTags(existing.Tags, deriveTags(content, category))
		s.rebuildL1IndexLocked()
		if err := s.persistAllLocked(); err != nil {
			log.Printf("[memory] persist duplicate update error: %v", err)
		}
		log.Printf("[memory] updated duplicate entry %s (category=%s)", existing.ID, category)
		return existing, nil
	}

	superseded := s.findSupersededLocked(content, category)
	id := fmt.Sprintf("mem-%d", now.UnixNano())
	entry := &Entry{
		ID:         id,
		Content:    content,
		Summary:    summarize(content, 120),
		Category:   category,
		Scope:      "user",
		Source:     "long_term_memory",
		Tags:       deriveTags(content, category),
		Layer:      2,
		Importance: defaultImportance(category),
		Confidence: 0.95,
		Supersedes: superseded,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	s.entries[id] = entry
	for _, oldID := range superseded {
		if old := s.entries[oldID]; old != nil {
			old.Importance = clamp(old.Importance-0.3, 0, 1)
			old.UpdatedAt = now
			old.Layer = 3
		}
	}
	s.rebuildL1IndexLocked()

	// Persist to disk
	if s.memoryDir != "" {
		if err := s.persistAllLocked(); err != nil {
			log.Printf("[memory] persist error: %v", err)
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
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var scored []scoredEntry
	superseded := s.supersededIDsLocked()
	for _, entry := range s.entries {
		score := s.searchScoreLocked(entry, query, superseded)
		if score > 0 {
			scored = append(scored, scoredEntry{entry: entry, score: score})
		}
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].entry.UpdatedAt.After(scored[j].entry.UpdatedAt)
		}
		return scored[i].score > scored[j].score
	})
	results := make([]*Entry, 0, len(scored))
	for _, item := range scored {
		item.entry.AccessCount++
		item.entry.LastAccessedAt = now
		results = append(results, cloneEntry(item.entry))
	}
	if len(results) > 0 {
		s.rebuildL1IndexLocked()
		if err := s.persistAllLocked(); err != nil {
			log.Printf("[memory] persist search metadata error: %v", err)
		}
	}
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
			result = append(result, cloneEntry(e))
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
		s.rebuildL1IndexLocked()
		if err := s.persistAllLocked(); err != nil {
			log.Printf("[memory] archive persist error: %v", err)
		}
	}

	log.Printf("[memory] archived %d entries to L3", archived)
	return archived, nil
}

type scoredEntry struct {
	entry *Entry
	score float64
}

func (s *Service) persistAllLocked() error {
	if s.memoryDir == "" {
		return nil
	}
	if err := os.MkdirAll(s.memoryDir, 0755); err != nil {
		return err
	}
	for _, dir := range []string{"l1", "l2", "l3"} {
		if err := os.MkdirAll(filepath.Join(s.memoryDir, dir), 0755); err != nil {
			return err
		}
	}
	if err := s.persistL2Entries(); err != nil {
		return err
	}
	if err := s.persistLayerEntries(filepath.Join(s.memoryDir, "l3", "archive.jsonl"), 3); err != nil {
		return err
	}
	return s.persistL1Index()
}

func (s *Service) persistLayerEntries(path string, layer int) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	for _, entry := range s.sortedEntriesLocked(layer) {
		if err := encoder.Encode(entry); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) sortedEntriesLocked(layer int) []*Entry {
	entries := make([]*Entry, 0, len(s.entries))
	for _, entry := range s.entries {
		if entry.Layer == layer {
			entries = append(entries, entry)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CreatedAt.Before(entries[j].CreatedAt)
	})
	return entries
}

func (s *Service) rebuildL1IndexLocked() {
	if s.maxL1 <= 0 {
		s.l1Index = nil
		return
	}

	superseded := s.supersededIDsLocked()
	candidates := make([]scoredEntry, 0, len(s.entries))
	now := time.Now()
	for _, entry := range s.entries {
		if entry.Layer != 2 || entry.Confidence < 0.5 || superseded[entry.ID] {
			continue
		}
		score := entry.Importance*4 + float64(entry.AccessCount)*0.5
		score += recencyBoost(now, entry.UpdatedAt, 30*24*time.Hour)
		score += recencyBoost(now, entry.LastAccessedAt, 14*24*time.Hour)
		candidates = append(candidates, scoredEntry{entry: entry, score: score})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].entry.UpdatedAt.After(candidates[j].entry.UpdatedAt)
		}
		return candidates[i].score > candidates[j].score
	})

	limit := s.maxL1
	if len(candidates) < limit {
		limit = len(candidates)
	}
	s.l1Index = make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		s.l1Index = append(s.l1Index, formatL1Entry(candidates[i].entry))
	}
}

func (s *Service) findDuplicateLocked(content, category string) *Entry {
	newTokens := tokenSet(content)
	for _, entry := range s.entries {
		if entry.Layer != 2 || entry.Category != category {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(entry.Content), content) {
			return entry
		}
		if jaccard(newTokens, tokenSet(entry.Content)) >= 0.82 {
			return entry
		}
	}
	return nil
}

func (s *Service) findSupersededLocked(content, category string) []string {
	newTokens := tokenSet(content)
	var ids []string
	for _, entry := range s.entries {
		if entry.Layer != 2 || entry.Category != category {
			continue
		}
		if jaccard(newTokens, tokenSet(entry.Content)) >= 0.33 && hasConflictCue(content, entry.Content) {
			ids = append(ids, entry.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

func (s *Service) supersededIDsLocked() map[string]bool {
	result := make(map[string]bool)
	for _, entry := range s.entries {
		for _, id := range entry.Supersedes {
			result[id] = true
		}
	}
	return result
}

func (s *Service) searchScoreLocked(entry *Entry, query string, superseded map[string]bool) float64 {
	if strings.TrimSpace(query) == "" {
		return 0
	}
	queryTokens := tokenSet(query)
	if len(queryTokens) == 0 {
		return 0
	}

	var score float64
	fields := []struct {
		text   string
		weight float64
	}{
		{entry.Content, 3},
		{entry.Summary, 2},
		{strings.Join(entry.Tags, " "), 2},
		{entry.Category, 1.5},
		{entry.Scope, 0.5},
	}
	for _, field := range fields {
		tokens := tokenSet(field.text)
		for token := range queryTokens {
			if tokens[token] {
				score += field.weight
			}
		}
	}
	if score == 0 {
		return 0
	}
	score += entry.Importance
	score += float64(entry.AccessCount) * 0.05
	if entry.Scope == "user" {
		score += 0.2
	}
	if superseded[entry.ID] {
		score *= 0.2
	}
	if entry.Layer == 3 {
		score *= 0.6
	}
	return score
}

func normalizeEntry(entry *Entry, defaultLayer int) {
	now := time.Now()
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("mem-%d", now.UnixNano())
	}
	entry.Content = strings.TrimSpace(entry.Content)
	if entry.Summary == "" {
		entry.Summary = summarize(entry.Content, 120)
	}
	entry.Category = normalizeCategory(entry.Category)
	if entry.Scope == "" {
		entry.Scope = "user"
	}
	if entry.Source == "" {
		entry.Source = "migration"
	}
	if entry.Layer == 0 {
		entry.Layer = defaultLayer
	}
	if entry.Importance == 0 {
		entry.Importance = defaultImportance(entry.Category)
	}
	if entry.Confidence == 0 {
		entry.Confidence = 0.8
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	if entry.UpdatedAt.IsZero() {
		entry.UpdatedAt = entry.CreatedAt
	}
	entry.Tags = mergeTags(entry.Tags, deriveTags(entry.Content, entry.Category))
}

func normalizeCategory(category string) string {
	category = strings.ToLower(strings.TrimSpace(category))
	category = strings.ReplaceAll(category, "preferences", "preference")
	if category == "" {
		return "fact"
	}
	return category
}

func defaultImportance(category string) float64 {
	switch normalizeCategory(category) {
	case "preference", "decision", "workflow":
		return 0.8
	case "project":
		return 0.65
	default:
		return 0.55
	}
}

func deriveTags(content, category string) []string {
	tags := []string{normalizeCategory(category)}
	for token := range tokenSet(content) {
		if len(token) >= 4 && len(tags) < 8 {
			tags = append(tags, token)
		}
	}
	return mergeTags(nil, tags)
}

func mergeTags(existing, additional []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, tag := range append(existing, additional...) {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		result = append(result, tag)
	}
	sort.Strings(result)
	return result
}

func tokenSet(s string) map[string]bool {
	tokens := make(map[string]bool)
	for _, field := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r >= '一' && r <= '鿿')
	}) {
		field = strings.TrimSpace(field)
		if len(field) < 2 || isStopWord(field) {
			continue
		}
		tokens[field] = true
	}
	return tokens
}

func isStopWord(s string) bool {
	switch s {
	case "the", "and", "for", "with", "that", "this", "user", "prefers", "prefer", "uses", "use":
		return true
	default:
		return false
	}
}

func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	var intersection int
	for token := range a {
		if b[token] {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func hasConflictCue(newContent, oldContent string) bool {
	newLower := strings.ToLower(newContent)
	oldLower := strings.ToLower(oldContent)
	cues := []string{"now", "no longer", "instead", "rather than", "现在", "改为", "不再", "而不是"}
	for _, cue := range cues {
		if strings.Contains(newLower, cue) {
			return true
		}
	}
	return strings.Contains(newLower, "prefer") && strings.Contains(oldLower, "prefer") && newLower != oldLower
}

func recencyBoost(now, t time.Time, window time.Duration) float64 {
	if t.IsZero() {
		return 0
	}
	age := now.Sub(t)
	if age <= 0 {
		return 1
	}
	if age >= window {
		return 0
	}
	return 1 - float64(age)/float64(window)
}

func formatL1Entry(entry *Entry) string {
	text := entry.Summary
	if text == "" {
		text = summarize(entry.Content, 120)
	}
	return fmt.Sprintf("- [%s] %s", entry.Category, summarize(text, 120))
}

func cloneEntry(entry *Entry) *Entry {
	if entry == nil {
		return nil
	}
	clone := *entry
	clone.Tags = append([]string(nil), entry.Tags...)
	clone.Supersedes = append([]string(nil), entry.Supersedes...)
	return &clone
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
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
