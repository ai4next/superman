package memory

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/global"
	persistdb "github.com/ai4next/superman/internal/store/db"
)

const (
	OwnerSuperman = "superman"
	LayerL1       = "l1"
	LayerL2       = "l2"
)

type SearchOptions struct {
	Query  string
	Owners []string
	Layers []string
	Limit  int
}

type SearchResult struct {
	Owner     string  `json:"owner"`
	Layer     string  `json:"layer"`
	Path      string  `json:"path"`
	Title     string  `json:"title,omitempty"`
	Score     float64 `json:"score"`
	MatchType string  `json:"match_type"`
	Snippet   string  `json:"snippet"`
}

type SearchService struct {
	cfg *config.Config
}

type memoryDocument struct {
	ID      string
	Owner   string
	Layer   string
	Path    string
	Title   string
	Content string
}

func NewSearchService(cfg *config.Config) *SearchService {
	if cfg == nil {
		cfg = global.Config()
	}
	return &SearchService{cfg: cfg}
}

func (s *SearchService) Search(opts SearchOptions) ([]SearchResult, error) {
	opts.Query = strings.TrimSpace(opts.Query)
	if opts.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if s == nil || s.cfg == nil {
		return nil, fmt.Errorf("memory search config is unavailable")
	}
	if !s.cfg.Memory.Search.Enabled {
		return nil, fmt.Errorf("memory search is disabled")
	}
	if opts.Limit <= 0 {
		opts.Limit = s.cfg.Memory.Search.MaxResults
	}
	if opts.Limit <= 0 {
		opts.Limit = 8
	}

	docs, err := discoverDocuments(s.cfg.Workspace, opts.Owners, opts.Layers)
	if err != nil {
		return nil, err
	}
	results := make(map[string]SearchResult)
	if s.cfg.Memory.Search.FTSEnabled {
		fts, err := s.searchFTS(opts, docs)
		if err != nil {
			return nil, err
		}
		mergeResults(results, fts)
	}
	if s.cfg.Memory.Search.ScanEnabled {
		mergeResults(results, scanDocuments(opts, docs))
	}
	if s.cfg.Memory.Search.VectorEnabled {
		mergeResults(results, vectorPlaceholderResults(opts, docs))
	}

	out := make([]SearchResult, 0, len(results))
	for _, result := range results {
		out = append(out, result)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].Path < out[j].Path
		}
		return out[i].Score > out[j].Score
	})
	if len(out) > opts.Limit {
		out = out[:opts.Limit]
	}
	return out, nil
}

func discoverDocuments(workspace string, owners, layers []string) ([]memoryDocument, error) {
	ownerSet := filterSet(owners)
	layerSet := filterSet(layers)
	var docs []memoryDocument
	addOwnerDocs := func(owner, memoryDir string) error {
		if len(ownerSet) > 0 && !ownerSet[owner] {
			return nil
		}
		ownerDocs, err := documentsForOwner(owner, memoryDir, layerSet)
		if err != nil {
			return err
		}
		docs = append(docs, ownerDocs...)
		return nil
	}
	memoriesDir := filepath.Join(workspace, "memory")
	entries, err := os.ReadDir(memoriesDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read memory dir: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if err := addOwnerDocs(entry.Name(), filepath.Join(memoriesDir, entry.Name())); err != nil {
			return nil, err
		}
	}
	return docs, nil
}

func documentsForOwner(owner, memoryDir string, layerSet map[string]bool) ([]memoryDocument, error) {
	var docs []memoryDocument
	if len(layerSet) == 0 || layerSet[LayerL1] {
		l1Path := filepath.Join(memoryDir, "l1.toml")
		content, err := os.ReadFile(l1Path)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("read %s: %w", l1Path, err)
		}
		if len(content) > 0 {
			docs = append(docs, memoryDocument{
				ID:      owner + ":l1:" + l1Path,
				Owner:   owner,
				Layer:   LayerL1,
				Path:    l1Path,
				Title:   "l1.toml",
				Content: string(content),
			})
		}
	}
	if len(layerSet) == 0 || layerSet[LayerL2] {
		l2Dir := filepath.Join(memoryDir, "l2")
		entries, err := os.ReadDir(l2Dir)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("read %s: %w", l2Dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			path := filepath.Join(l2Dir, entry.Name())
			content, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", path, err)
			}
			docs = append(docs, memoryDocument{
				ID:      owner + ":l2:" + path,
				Owner:   owner,
				Layer:   LayerL2,
				Path:    path,
				Title:   entry.Name(),
				Content: string(content),
			})
		}
	}
	return docs, nil
}

func (s *SearchService) searchFTS(opts SearchOptions, docs []memoryDocument) ([]SearchResult, error) {
	if len(docs) == 0 {
		return nil, nil
	}
	registry, err := global.DBRegistry()
	if err != nil {
		return nil, err
	}
	rows, err := registry.GlobalDB.SearchMemoryIndex(opts.Query, opts.Limit, memoryDocsToRows(docs))
	if err != nil {
		return nil, fmt.Errorf("search memory index: %w", err)
	}
	return memoryRowsToResults(rows), nil
}

func memoryDocsToRows(docs []memoryDocument) []persistdb.MemoryDocumentRow {
	rows := make([]persistdb.MemoryDocumentRow, len(docs))
	for i, doc := range docs {
		rows[i] = persistdb.MemoryDocumentRow{
			ID:      doc.ID,
			Owner:   doc.Owner,
			Layer:   doc.Layer,
			Path:    doc.Path,
			Title:   doc.Title,
			Content: doc.Content,
		}
	}
	return rows
}

func memoryRowsToResults(rows []persistdb.MemorySearchResultRow) []SearchResult {
	results := make([]SearchResult, len(rows))
	for i, row := range rows {
		results[i] = SearchResult{
			Owner:     row.Owner,
			Layer:     row.Layer,
			Path:      row.Path,
			Title:     row.Title,
			Score:     row.Score,
			MatchType: row.MatchType,
			Snippet:   row.Snippet,
		}
	}
	return results
}

func scanDocuments(opts SearchOptions, docs []memoryDocument) []SearchResult {
	terms := queryTerms(opts.Query)
	if len(terms) == 0 {
		return nil
	}
	var out []SearchResult
	for _, doc := range docs {
		content := strings.ToLower(doc.Title + "\n" + doc.Content)
		matches := 0
		for _, term := range terms {
			if strings.Contains(content, term) {
				matches++
			}
		}
		if matches == 0 {
			continue
		}
		out = append(out, SearchResult{
			Owner:     doc.Owner,
			Layer:     doc.Layer,
			Path:      doc.Path,
			Title:     doc.Title,
			Score:     float64(matches),
			MatchType: "scan",
			Snippet:   makeSnippet(doc.Content, terms),
		})
	}
	return out
}

func vectorPlaceholderResults(opts SearchOptions, docs []memoryDocument) []SearchResult {
	queryVector := termVector(opts.Query)
	if len(queryVector) == 0 {
		return nil
	}
	queryNorm := vectorNorm(queryVector)
	var out []SearchResult
	for _, doc := range docs {
		docVector := termVector(doc.Title + "\n" + doc.Content)
		score := cosine(queryVector, queryNorm, docVector)
		if score <= 0 {
			continue
		}
		out = append(out, SearchResult{
			Owner:     doc.Owner,
			Layer:     doc.Layer,
			Path:      doc.Path,
			Title:     doc.Title,
			Score:     100 * score,
			MatchType: "vector",
			Snippet:   makeSnippet(doc.Content, queryTerms(opts.Query)),
		})
	}
	return out
}

func mergeResults(dst map[string]SearchResult, src []SearchResult) {
	for _, result := range src {
		key := result.Owner + "|" + result.Layer + "|" + result.Path
		if current, ok := dst[key]; ok {
			if result.Score > current.Score {
				dst[key] = result
			}
			continue
		}
		dst[key] = result
	}
}

func filterSet(values []string) map[string]bool {
	out := make(map[string]bool)
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
		if value != "" {
			out[value] = true
		}
	}
	return out
}

func queryTerms(query string) []string {
	fields := strings.Fields(strings.ToLower(query))
	seen := make(map[string]bool)
	var out []string
	for _, field := range fields {
		field = strings.Trim(field, `"':;,.!?()[]{}<>`)
		if field == "" || seen[field] {
			continue
		}
		seen[field] = true
		out = append(out, field)
	}
	return out
}

func makeSnippet(content string, terms []string) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}
	lower := strings.ToLower(content)
	pos := -1
	for _, term := range terms {
		if idx := strings.Index(lower, term); idx >= 0 && (pos < 0 || idx < pos) {
			pos = idx
		}
	}
	runes := []rune(content)
	if pos < 0 {
		if len(runes) > 240 {
			return string(runes[:240]) + "..."
		}
		return content
	}
	start := max(0, len([]rune(content[:pos]))-80)
	end := min(len(runes), start+240)
	prefix := ""
	if start > 0 {
		prefix = "..."
	}
	suffix := ""
	if end < len(runes) {
		suffix = "..."
	}
	return prefix + strings.TrimSpace(string(runes[start:end])) + suffix
}

func termVector(text string) map[string]float64 {
	out := make(map[string]float64)
	for _, term := range queryTerms(text) {
		out[term]++
	}
	return out
}

func vectorNorm(vector map[string]float64) float64 {
	var sum float64
	for _, value := range vector {
		sum += value * value
	}
	return math.Sqrt(sum)
}

func cosine(query map[string]float64, queryNorm float64, doc map[string]float64) float64 {
	if queryNorm == 0 || len(doc) == 0 {
		return 0
	}
	var dot float64
	for term, qv := range query {
		dot += qv * doc[term]
	}
	docNorm := vectorNorm(doc)
	if docNorm == 0 {
		return 0
	}
	return dot / (queryNorm * docNorm)
}
