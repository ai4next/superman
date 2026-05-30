package db

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MemoryDocumentRow struct {
	ID      string
	Owner   string
	Layer   string
	Path    string
	Title   string
	Content string
}

type MemorySearchResultRow struct {
	Owner     string
	Layer     string
	Path      string
	Title     string
	Score     float64
	MatchType string
	Snippet   string
}

type MemoryIndexRow struct {
	gorm.Model
	DocumentID string `gorm:"column:document_id;not null;index"`
	Owner      string `gorm:"not null;index:idx_memory_index_owner_layer,priority:1"`
	Layer      string `gorm:"not null;index:idx_memory_index_owner_layer,priority:2"`
	Path       string `gorm:"not null;index"`
	Title      string `gorm:"not null"`
	Content    string `gorm:"not null"`
}

func (MemoryIndexRow) TableName() string { return "memory_index" }

type MemoryIndexMetaRow struct {
	gorm.Model
	Key   string `gorm:"column:key;not null;uniqueIndex"`
	Value string `gorm:"not null"`
}

func (MemoryIndexMetaRow) TableName() string { return "memory_index_meta" }

func (d *DB) SearchMemoryIndex(query string, limit int, docs []MemoryDocumentRow) ([]MemorySearchResultRow, error) {
	if len(docs) == 0 {
		return nil, nil
	}
	if err := d.db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(id UNINDEXED, owner UNINDEXED, layer UNINDEXED, path UNINDEXED, title, content)`).Error; err != nil {
		if strings.Contains(err.Error(), "no such module: fts5") {
			return d.searchMemoryFallbackIndex(query, docs)
		}
		return nil, fmt.Errorf("create memory fts: %w", err)
	}
	tx := d.db.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	if err := tx.Exec(`DELETE FROM memory_fts`).Error; err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	for _, doc := range docs {
		if err := tx.Exec(`INSERT INTO memory_fts(id, owner, layer, path, title, content) VALUES (?, ?, ?, ?, ?, ?)`, doc.ID, doc.Owner, doc.Layer, doc.Path, doc.Title, doc.Content).Error; err != nil {
			_ = tx.Rollback()
			return nil, err
		}
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&MemoryIndexMetaRow{
		Key:   "indexed_at",
		Value: time.Now().UTC().Format(time.RFC3339),
	}).Error; err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	rows, err := d.db.Raw(`SELECT owner, layer, path, title, snippet(memory_fts, 5, '[', ']', '...', 16), bm25(memory_fts) FROM memory_fts WHERE memory_fts MATCH ? ORDER BY bm25(memory_fts) LIMIT ?`, memoryFTSQuery(query), limit*2).Rows()
	if err != nil {
		if strings.Contains(err.Error(), "fts5: syntax error") {
			return nil, nil
		}
		return nil, fmt.Errorf("query memory fts: %w", err)
	}
	defer rows.Close()
	var out []MemorySearchResultRow
	for rows.Next() {
		var result MemorySearchResultRow
		var rank float64
		if err := rows.Scan(&result.Owner, &result.Layer, &result.Path, &result.Title, &result.Snippet, &rank); err != nil {
			return nil, err
		}
		result.MatchType = "fts"
		result.Score = 1000 - rank
		out = append(out, result)
	}
	return out, rows.Err()
}

func (d *DB) searchMemoryFallbackIndex(query string, docs []MemoryDocumentRow) ([]MemorySearchResultRow, error) {
	if err := d.replaceMemoryIndexRows(docs); err != nil {
		return nil, err
	}
	terms := memoryQueryTerms(query)
	if len(terms) == 0 {
		return nil, nil
	}
	var rows []MemoryIndexRow
	if err := d.db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query memory index: %w", err)
	}
	var out []MemorySearchResultRow
	for _, row := range rows {
		lower := strings.ToLower(row.Title + "\n" + row.Content)
		matches := 0
		for _, term := range terms {
			if strings.Contains(lower, term) {
				matches++
			}
		}
		if matches == 0 {
			continue
		}
		out = append(out, MemorySearchResultRow{
			Owner:     row.Owner,
			Layer:     row.Layer,
			Path:      row.Path,
			Title:     row.Title,
			Score:     500 + float64(matches),
			MatchType: "fts",
			Snippet:   memorySnippet(row.Content, terms),
		})
	}
	return out, nil
}

func (d *DB) replaceMemoryIndexRows(docs []MemoryDocumentRow) error {
	rows := make([]MemoryIndexRow, len(docs))
	for i, doc := range docs {
		rows[i] = MemoryIndexRow{
			DocumentID: doc.ID,
			Owner:      doc.Owner,
			Layer:      doc.Layer,
			Path:       doc.Path,
			Title:      doc.Title,
			Content:    doc.Content,
		}
	}
	return d.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("1 = 1").Delete(&MemoryIndexRow{}).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}

func memoryQueryTerms(query string) []string {
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

func memoryFTSQuery(query string) string {
	terms := memoryQueryTerms(query)
	if len(terms) == 0 {
		return `""`
	}
	for i, term := range terms {
		terms[i] = `"` + strings.ReplaceAll(term, `"`, `""`) + `"`
	}
	return strings.Join(terms, " OR ")
}

func memorySnippet(content string, terms []string) string {
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
