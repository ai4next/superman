package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Candidate struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Summary   string    `json:"summary"`
	EntryIDs  []string  `json:"entry_ids,omitempty"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

// Evolve scans current memories and writes reviewable candidates without
// changing official long-term memories or SOP rules.
func (s *Service) Evolve(ctx context.Context, candidateDir string) ([]Candidate, error) {
	s.mu.RLock()
	entries := make([]*Entry, 0, len(s.entries))
	for _, entry := range s.entries {
		entries = append(entries, cloneEntry(entry))
	}
	s.mu.RUnlock()

	if candidateDir == "" {
		candidateDir = filepath.Join(s.memoryDir, "candidates")
	}
	if candidateDir == "" {
		return nil, fmt.Errorf("candidate dir is empty")
	}
	if err := os.MkdirAll(filepath.Join(candidateDir, "sop"), 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(candidateDir, "experts"), 0755); err != nil {
		return nil, err
	}

	candidates := buildMemoryCandidates(entries)
	if err := appendCandidates(filepath.Join(candidateDir, "memory.jsonl"), candidates); err != nil {
		return nil, err
	}
	if err := writeSOPCandidates(filepath.Join(candidateDir, "sop"), entries); err != nil {
		return nil, err
	}
	return candidates, nil
}

func buildMemoryCandidates(entries []*Entry) []Candidate {
	now := time.Now()
	var candidates []Candidate

	for key, group := range groupSimilarActive(entries) {
		if len(group) < 2 {
			continue
		}
		candidates = append(candidates, Candidate{
			ID:        fmt.Sprintf("cand-%d-%s", now.UnixNano(), safeName(key)),
			Type:      "merge",
			Summary:   fmt.Sprintf("Review %d similar memories about %s", len(group), key),
			EntryIDs:  entryIDs(group),
			Reason:    "Similar active memories may be clearer if merged or rewritten.",
			CreatedAt: now,
		})
	}

	for _, entry := range entries {
		if len(entry.Supersedes) == 0 {
			continue
		}
		ids := append([]string{entry.ID}, entry.Supersedes...)
		candidates = append(candidates, Candidate{
			ID:        fmt.Sprintf("cand-%d-%s", now.UnixNano(), safeName(entry.ID)),
			Type:      "conflict",
			Summary:   fmt.Sprintf("Memory %s supersedes older memory", entry.ID),
			EntryIDs:  ids,
			Reason:    "Confirm the older memory should remain archived and excluded from hot recall.",
			CreatedAt: now,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Type == candidates[j].Type {
			return candidates[i].Summary < candidates[j].Summary
		}
		return candidates[i].Type < candidates[j].Type
	})
	return candidates
}

func groupSimilarActive(entries []*Entry) map[string][]*Entry {
	groups := make(map[string][]*Entry)
	for _, entry := range entries {
		if entry.Layer != 2 {
			continue
		}
		keys := entry.Tags
		if len(keys) == 0 {
			keys = []string{entry.Category}
		}
		for _, tag := range keys {
			if tag == "" || tag == entry.Category {
				continue
			}
			key := entry.Category + ":" + tag
			groups[key] = append(groups[key], entry)
			break
		}
	}
	return groups
}

func appendCandidates(path string, candidates []Candidate) error {
	if len(candidates) == 0 {
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, candidate := range candidates {
		if err := enc.Encode(candidate); err != nil {
			return err
		}
	}
	return nil
}

func writeSOPCandidates(dir string, entries []*Entry) error {
	groups := make(map[string][]*Entry)
	for _, entry := range entries {
		if entry.Layer != 2 || entry.Confidence < 0.7 {
			continue
		}
		if entry.Category != "workflow" && entry.Category != "preference" {
			continue
		}
		groups[entry.Category] = append(groups[entry.Category], entry)
	}
	for category, group := range groups {
		if len(group) < 2 {
			continue
		}
		sort.Slice(group, func(i, j int) bool {
			return group[i].Importance > group[j].Importance
		})
		var b strings.Builder
		b.WriteString("# " + category + "-memory-candidate\n\n")
		for i, entry := range group {
			if i >= 5 {
				break
			}
			b.WriteString("- " + summarize(entry.Summary, 120) + "\n")
		}
		path := filepath.Join(dir, category+"-memory-candidate.md")
		if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
			return err
		}
	}
	return nil
}

func entryIDs(entries []*Entry) []string {
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.ID)
	}
	sort.Strings(ids)
	return ids
}

func safeName(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			continue
		}
		current := b.String()
		if len(current) > 0 && current[len(current)-1] != '-' {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
