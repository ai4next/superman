package store

import (
	"fmt"
	"os"
	"strings"

	"github.com/ai4next/superman/internal/global"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Session struct {
	ID               int64
	AppName          string
	UserID           string
	Title            string
	StateJSON        string
	PromptTokens     int64
	CompletionTokens int64
	SummaryMessageID string
	FileCount        int
	MessageCount     int
	QueuedPrompts    int
	CreatedAt        string
	UpdatedAt        string
}

type File struct {
	SessionID  int64
	Path       string
	ReadAt     string
	WrittenAt  string
	ReadCount  int
	WriteCount int
	LastAccess string
}

type FileRevision struct {
	ID        string
	SessionID int64
	Position  int
	Path      string
	Action    string
	DataJSON  string
	CreatedAt string
}

type QueuedPrompt struct {
	ID        string
	SessionID int64
	Position  int
	Content   string
	CreatedAt string
}

type Reference struct {
	ID        int64
	SessionID int64
	Position  int
	RefID     string
	Role      string
	Preview   string
	CreatedAt string
}

type sessionRow struct {
	ID               int64  `gorm:"primaryKey;autoIncrement"`
	AppName          string `gorm:"not null;index:idx_session_scope_updated,priority:1"`
	UserID           string `gorm:"not null;index:idx_session_scope_updated,priority:2"`
	Agent            string `gorm:"not null;default:''"`
	Title            string `gorm:"not null"`
	StateJSON        string `gorm:"column:state_json"`
	PromptTokens     int64  `gorm:"not null;default:0"`
	CompletionTokens int64  `gorm:"not null;default:0"`
	SummaryMessageID string `gorm:"not null;default:''"`
	FileCount        int    `gorm:"not null;default:0"`
	MessageCount     int    `gorm:"not null;default:0"`
	QueuedPrompts    int    `gorm:"not null;default:0"`
	CreatedAt        string `gorm:"not null"`
	UpdatedAt        string `gorm:"not null;index:idx_session_scope_updated,priority:3,sort:desc"`
}

func (sessionRow) TableName() string { return "session" }

type fileRow struct {
	SessionID  int64  `gorm:"primaryKey;autoIncrement:false"`
	Path       string `gorm:"primaryKey"`
	ReadAt     string
	WrittenAt  string
	ReadCount  int
	WriteCount int
	LastAccess string
}

func (fileRow) TableName() string { return "session_file" }

type fileRevisionRow struct {
	ID        string `gorm:"primaryKey"`
	SessionID int64  `gorm:"not null;index"`
	Position  int    `gorm:"not null"`
	Path      string `gorm:"index"`
	Action    string
	DataJSON  string `gorm:"column:data_json;not null"`
	CreatedAt string `gorm:"not null;index"`
}

func (fileRevisionRow) TableName() string { return "file_revision" }

type queuedPromptRow struct {
	ID        string `gorm:"primaryKey"`
	SessionID int64  `gorm:"not null;index"`
	Position  int    `gorm:"not null"`
	Content   string
	CreatedAt string `gorm:"not null"`
}

func (queuedPromptRow) TableName() string { return "queued_prompt" }

type referenceRow struct {
	ID        int64  `gorm:"primaryKey;autoIncrement"`
	SessionID int64  `gorm:"not null;index"`
	Position  int    `gorm:"not null"`
	RefID     string `gorm:"column:ref_id;index"`
	Role      string
	Preview   string
	CreatedAt string `gorm:"not null"`
}

func (referenceRow) TableName() string { return "session_reference" }

func Open() (*DB, error) {
	return OpenPath(global.StateDBPath())
}

func OpenPath(dbPath string) (*DB, error) {
	gormDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open session state db: %w", err)
	}
	db := &DB{db: gormDB}
	if err := db.init(); err != nil {
		return nil, err
	}
	return db, nil
}

func (d *DB) init() error {
	if err := d.db.Exec("PRAGMA journal_mode = WAL").Error; err != nil {
		return fmt.Errorf("initialize session state db: %w", err)
	}
	if err := d.db.AutoMigrate(&sessionRow{}, &messageRow{}, &fileRow{}, &fileRevisionRow{}, &queuedPromptRow{}, &referenceRow{}); err != nil {
		return fmt.Errorf("migrate session state db: %w", err)
	}
	return nil
}

func sessionFromRow(row sessionRow) Session {
	return Session{
		ID:               row.ID,
		AppName:          row.AppName,
		UserID:           row.UserID,
		Title:            row.Title,
		StateJSON:        row.StateJSON,
		PromptTokens:     row.PromptTokens,
		CompletionTokens: row.CompletionTokens,
		SummaryMessageID: row.SummaryMessageID,
		FileCount:        row.FileCount,
		MessageCount:     row.MessageCount,
		QueuedPrompts:    row.QueuedPrompts,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}

func sessionToRow(row Session) sessionRow {
	return sessionRow{
		ID:               row.ID,
		AppName:          row.AppName,
		UserID:           row.UserID,
		Title:            row.Title,
		StateJSON:        row.StateJSON,
		PromptTokens:     row.PromptTokens,
		CompletionTokens: row.CompletionTokens,
		SummaryMessageID: row.SummaryMessageID,
		FileCount:        row.FileCount,
		MessageCount:     row.MessageCount,
		QueuedPrompts:    row.QueuedPrompts,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}

func (d *DB) LoadSessions() ([]Session, error) {
	var rows []sessionRow
	if err := d.db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("load session metadata: %w", err)
	}
	out := make([]Session, len(rows))
	for i, row := range rows {
		out[i] = sessionFromRow(row)
	}
	return out, nil
}

func (d *DB) SaveSession(row Session) (Session, error) {
	dbRow := sessionToRow(row)
	var err error
	if dbRow.ID > 0 {
		err = d.db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"app_name", "user_id", "title", "state_json", "prompt_tokens", "completion_tokens",
				"summary_message_id", "file_count", "message_count", "queued_prompts", "updated_at",
			}),
		}).Create(&dbRow).Error
	} else {
		err = d.db.Create(&dbRow).Error
	}
	if err != nil {
		return Session{}, err
	}
	return sessionFromRow(dbRow), nil
}

func (d *DB) DeleteSession(id int64, appName, userID string) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		for _, table := range []any{&messageRow{}, &fileRow{}, &fileRevisionRow{}, &queuedPromptRow{}, &referenceRow{}} {
			if err := tx.Where("session_id = ?", id).Delete(table).Error; err != nil {
				return err
			}
		}
		return tx.Where("id = ? AND app_name = ? AND user_id = ?", id, appName, userID).Delete(&sessionRow{}).Error
	})
}

func (d *DB) LoadDetails(sessionID int64) ([]File, []FileRevision, []QueuedPrompt, []Reference, error) {
	var fileRows []fileRow
	if err := d.db.Where("session_id = ?", sessionID).Find(&fileRows).Error; err != nil {
		return nil, nil, nil, nil, err
	}
	files := make([]File, len(fileRows))
	for i, row := range fileRows {
		files[i] = File(row)
	}
	var revisionRows []fileRevisionRow
	if err := d.db.Where("session_id = ?", sessionID).Order("position asc").Find(&revisionRows).Error; err != nil {
		return nil, nil, nil, nil, err
	}
	revisions := make([]FileRevision, len(revisionRows))
	for i, row := range revisionRows {
		revisions[i] = FileRevision(row)
	}
	var queueRows []queuedPromptRow
	if err := d.db.Where("session_id = ?", sessionID).Order("position asc").Find(&queueRows).Error; err != nil {
		return nil, nil, nil, nil, err
	}
	queue := make([]QueuedPrompt, len(queueRows))
	for i, row := range queueRows {
		queue[i] = QueuedPrompt(row)
	}
	var refRows []referenceRow
	if err := d.db.Where("session_id = ?", sessionID).Order("position asc").Find(&refRows).Error; err != nil {
		return nil, nil, nil, nil, err
	}
	refs := make([]Reference, len(refRows))
	for i, row := range refRows {
		refs[i] = Reference(row)
	}
	return files, revisions, queue, refs, nil
}

func (d *DB) SaveDetails(sessionID int64, files []File, revisions []FileRevision, queue []QueuedPrompt, refs []Reference) error {
	fileRows := make([]fileRow, len(files))
	for i, row := range files {
		row.SessionID = sessionID
		fileRows[i] = fileRow(row)
	}
	revisionRows := make([]fileRevisionRow, len(revisions))
	for i, row := range revisions {
		row.SessionID = sessionID
		row.Position = i
		revisionRows[i] = fileRevisionRow(row)
	}
	queueRows := make([]queuedPromptRow, len(queue))
	for i, row := range queue {
		row.SessionID = sessionID
		row.Position = i
		queueRows[i] = queuedPromptRow(row)
	}
	refRows := make([]referenceRow, len(refs))
	for i, row := range refs {
		row.SessionID = sessionID
		row.Position = i
		refRows[i] = referenceRow(row)
	}
	return d.db.Transaction(func(tx *gorm.DB) error {
		for _, table := range []any{&fileRow{}, &fileRevisionRow{}, &queuedPromptRow{}, &referenceRow{}} {
			if err := tx.Where("session_id = ?", sessionID).Delete(table).Error; err != nil {
				return err
			}
		}
		if len(fileRows) > 0 {
			if err := tx.Create(&fileRows).Error; err != nil {
				return err
			}
		}
		if len(revisionRows) > 0 {
			if err := tx.Create(&revisionRows).Error; err != nil {
				return err
			}
		}
		if len(queueRows) > 0 {
			if err := tx.Create(&queueRows).Error; err != nil {
				return err
			}
		}
		if len(refRows) > 0 {
			if err := tx.Create(&refRows).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func safeName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "default"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", string(os.PathSeparator), "_")
	return replacer.Replace(value)
}
