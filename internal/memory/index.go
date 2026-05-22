package memory

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type MetadataIndex interface {
	Rebuild(entries map[string]*Entry) error
	Query(query string, opts RecallOptions) map[string]float64
}

type SemanticIndex interface {
	Rebuild(entries map[string]*Entry) error
	Query(query string) map[string]float64
}

type RecallOptions struct {
	Scope   string
	Project string
	Types   []string
	Limit   int
}

type indexedEntry struct {
	ID             string   `json:"id"`
	Type           string   `json:"type,omitempty"`
	Category       string   `json:"category,omitempty"`
	Scope          string   `json:"scope,omitempty"`
	Project        string   `json:"project,omitempty"`
	Status         string   `json:"status,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	Layer          int      `json:"layer"`
	Importance     float64  `json:"importance,omitempty"`
	Confidence     float64  `json:"confidence,omitempty"`
	AccessCount    int      `json:"access_count,omitempty"`
	Superseded     bool     `json:"superseded,omitempty"`
	Sensitive      bool     `json:"sensitive,omitempty"`
	SearchDocument string   `json:"search_document"`
}

type memoryMetadataIndex struct {
	mu      sync.RWMutex
	path    string
	entries map[string]indexedEntry
}

func newMemoryMetadataIndex() *memoryMetadataIndex {
	return &memoryMetadataIndex{entries: make(map[string]indexedEntry)}
}

func newFileMetadataIndex(path string) *memoryMetadataIndex {
	return &memoryMetadataIndex{path: path, entries: make(map[string]indexedEntry)}
}

func (idx *memoryMetadataIndex) Rebuild(entries map[string]*Entry) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	superseded := supersededIDs(entries)
	next := make(map[string]indexedEntry, len(entries))
	for _, entry := range entries {
		next[entry.ID] = indexedEntry{
			ID:             entry.ID,
			Type:           entry.Type,
			Category:       entry.Category,
			Scope:          entry.Scope,
			Project:        entry.Project,
			Status:         entry.Status,
			Tags:           append([]string(nil), entry.Tags...),
			Layer:          entry.Layer,
			Importance:     entry.Importance,
			Confidence:     entry.Confidence,
			AccessCount:    entry.AccessCount,
			Superseded:     superseded[entry.ID],
			Sensitive:      entry.Sensitive,
			SearchDocument: entrySearchDocument(entry),
		}
	}
	idx.entries = next
	if idx.path == "" {
		return nil
	}
	return writeJSONL(idx.path, sortedIndexedEntries(next))
}

func (idx *memoryMetadataIndex) Query(query string, opts RecallOptions) map[string]float64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	queryTokens := tokenSet(query)
	typeSet := make(map[string]bool, len(opts.Types))
	for _, typ := range opts.Types {
		typeSet[normalizeType(typ)] = true
	}

	results := make(map[string]float64)
	for id, entry := range idx.entries {
		if entry.Status == "isolated" || entry.Sensitive {
			continue
		}
		if opts.Scope != "" && entry.Scope != "" && entry.Scope != opts.Scope && entry.Scope != "global" {
			continue
		}
		if opts.Project != "" && entry.Project != "" && entry.Project != opts.Project {
			continue
		}
		if len(typeSet) > 0 && !typeSet[entry.Type] {
			continue
		}
		score := metadataScore(entry, queryTokens)
		if score > 0 {
			results[id] = score
		}
	}
	return results
}

type vectorEntry struct {
	ID     string             `json:"id"`
	Vector map[string]float64 `json:"vector"`
}

type memorySemanticIndex struct {
	mu      sync.RWMutex
	path    string
	vectors map[string]map[string]float64
}

func newMemorySemanticIndex() *memorySemanticIndex {
	return &memorySemanticIndex{vectors: make(map[string]map[string]float64)}
}

func newFileSemanticIndex(path string) *memorySemanticIndex {
	return &memorySemanticIndex{path: path, vectors: make(map[string]map[string]float64)}
}

func (idx *memorySemanticIndex) Rebuild(entries map[string]*Entry) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	next := make(map[string]map[string]float64, len(entries))
	for _, entry := range entries {
		if entry.Status == "isolated" || entry.Sensitive {
			continue
		}
		next[entry.ID] = textVector(entrySearchDocument(entry))
	}
	idx.vectors = next
	if idx.path == "" {
		return nil
	}

	rows := make([]vectorEntry, 0, len(next))
	for id, vec := range next {
		rows = append(rows, vectorEntry{ID: id, Vector: vec})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return writeJSONL(idx.path, rows)
}

func (idx *memorySemanticIndex) Query(query string) map[string]float64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	qv := textVector(query)
	results := make(map[string]float64)
	for id, vec := range idx.vectors {
		if score := cosine(qv, vec); score > 0 {
			results[id] = score
		}
	}
	return results
}

func metadataScore(entry indexedEntry, queryTokens map[string]bool) float64 {
	if len(queryTokens) == 0 {
		return 0
	}
	docTokens := tokenSet(entry.SearchDocument)
	var score float64
	for token := range queryTokens {
		if docTokens[token] {
			score += 1
		}
	}
	if score == 0 {
		return 0
	}
	score += entry.Importance * 0.4
	score += entry.Confidence * 0.25
	score += float64(entry.AccessCount) * 0.03
	if entry.Layer == 3 {
		score *= 0.55
	}
	if entry.Superseded {
		score *= 0.2
	}
	return score
}

func entrySearchDocument(entry *Entry) string {
	return strings.Join([]string{
		entry.Type,
		entry.Category,
		entry.Scope,
		entry.Project,
		entry.Content,
		entry.Summary,
		strings.Join(entry.Tags, " "),
	}, " ")
}

func sortedIndexedEntries(entries map[string]indexedEntry) []indexedEntry {
	rows := make([]indexedEntry, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, entry)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows
}

func writeJSONL[T any](path string, rows []T) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, row := range rows {
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return nil
}

func textVector(text string) map[string]float64 {
	tokens := tokenSet(text)
	vec := make(map[string]float64, len(tokens))
	for token := range tokens {
		vec[token] = 1
		for _, shard := range semanticShards(token) {
			vec[shard] += 0.25
		}
	}
	return vec
}

func semanticShards(token string) []string {
	if len(token) < 5 {
		return nil
	}
	var shards []string
	for i := 0; i+4 <= len(token) && len(shards) < 4; i += 2 {
		shards = append(shards, token[i:i+4])
	}
	return shards
}

func cosine(a, b map[string]float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	var dot, na, nb float64
	for token, av := range a {
		na += av * av
		if bv, ok := b[token]; ok {
			dot += av * bv
		}
	}
	for _, bv := range b {
		nb += bv * bv
	}
	if dot == 0 || na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func supersededIDs(entries map[string]*Entry) map[string]bool {
	result := make(map[string]bool)
	for _, entry := range entries {
		for _, id := range entry.Supersedes {
			result[id] = true
		}
	}
	return result
}
