package db

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SessionRow struct {
	gorm.Model
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
}

func (SessionRow) TableName() string { return "session" }

type FileRow struct {
	gorm.Model
	SessionID  int64  `gorm:"not null;index:idx_session_file_path,priority:1"`
	Path       string `gorm:"not null;index:idx_session_file_path,priority:2"`
	ReadAt     string
	WrittenAt  string
	ReadCount  int
	WriteCount int
	LastAccess string
}

func (FileRow) TableName() string { return "session_file" }

type FileRevisionRow struct {
	gorm.Model
	RevisionID string `gorm:"column:revision_id;not null;index"`
	SessionID  int64  `gorm:"not null;index"`
	Position   int    `gorm:"not null"`
	Path       string `gorm:"index"`
	Action     string
	DataJSON   string `gorm:"column:data_json;not null"`
}

func (FileRevisionRow) TableName() string { return "file_revision" }

type QueuedPromptRow struct {
	gorm.Model
	PromptID  string `gorm:"column:prompt_id;not null;index"`
	SessionID int64  `gorm:"not null;index"`
	Position  int    `gorm:"not null"`
	Content   string
}

func (QueuedPromptRow) TableName() string { return "queued_prompt" }

type ReferenceRow struct {
	gorm.Model
	SessionID int64  `gorm:"not null;index"`
	Position  int    `gorm:"not null"`
	RefID     string `gorm:"column:ref_id;index"`
	Role      string
	Preview   string
}

func (ReferenceRow) TableName() string { return "session_reference" }

func (d *DB) LoadSessions() ([]SessionRow, error) {
	var rows []SessionRow
	if err := d.db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("load session metadata: %w", err)
	}
	return rows, nil
}

func (d *DB) SaveSession(row SessionRow) (SessionRow, error) {
	var err error
	if row.ID > 0 {
		err = d.db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"app_name", "user_id", "title", "state_json", "prompt_tokens", "completion_tokens",
				"summary_message_id", "file_count", "message_count", "queued_prompts", "updated_at",
			}),
		}).Create(&row).Error
	} else {
		err = d.db.Create(&row).Error
	}
	if err != nil {
		return SessionRow{}, err
	}
	return row, nil
}

func (d *DB) DeleteSession(id int64, appName, userID string) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		for _, table := range []any{&MessageRow{}, &FileRow{}, &FileRevisionRow{}, &QueuedPromptRow{}, &ReferenceRow{}} {
			if err := tx.Where("session_id = ?", id).Delete(table).Error; err != nil {
				return err
			}
		}
		return tx.Where("id = ? AND app_name = ? AND user_id = ?", id, appName, userID).Delete(&SessionRow{}).Error
	})
}

func (d *DB) LoadDetails(sessionID int64) ([]FileRow, []FileRevisionRow, []QueuedPromptRow, []ReferenceRow, error) {
	var files []FileRow
	if err := d.db.Where("session_id = ?", sessionID).Find(&files).Error; err != nil {
		return nil, nil, nil, nil, err
	}
	var revisions []FileRevisionRow
	if err := d.db.Where("session_id = ?", sessionID).Order("position asc").Find(&revisions).Error; err != nil {
		return nil, nil, nil, nil, err
	}
	var queue []QueuedPromptRow
	if err := d.db.Where("session_id = ?", sessionID).Order("position asc").Find(&queue).Error; err != nil {
		return nil, nil, nil, nil, err
	}
	var refs []ReferenceRow
	if err := d.db.Where("session_id = ?", sessionID).Order("position asc").Find(&refs).Error; err != nil {
		return nil, nil, nil, nil, err
	}
	return files, revisions, queue, refs, nil
}

func (d *DB) SaveDetails(sessionID int64, files []FileRow, revisions []FileRevisionRow, queue []QueuedPromptRow, refs []ReferenceRow) error {
	for i, row := range files {
		row.SessionID = sessionID
		row.ID = 0
		files[i] = row
	}
	for i, row := range revisions {
		row.SessionID = sessionID
		row.Position = i
		row.ID = 0
		revisions[i] = row
	}
	for i, row := range queue {
		row.SessionID = sessionID
		row.Position = i
		row.ID = 0
		queue[i] = row
	}
	for i, row := range refs {
		row.SessionID = sessionID
		row.Position = i
		row.ID = 0
		refs[i] = row
	}
	return d.db.Transaction(func(tx *gorm.DB) error {
		for _, table := range []any{&FileRow{}, &FileRevisionRow{}, &QueuedPromptRow{}, &ReferenceRow{}} {
			if err := tx.Where("session_id = ?", sessionID).Delete(table).Error; err != nil {
				return err
			}
		}
		if len(files) > 0 {
			if err := tx.Create(&files).Error; err != nil {
				return err
			}
		}
		if len(revisions) > 0 {
			if err := tx.Create(&revisions).Error; err != nil {
				return err
			}
		}
		if len(queue) > 0 {
			if err := tx.Create(&queue).Error; err != nil {
				return err
			}
		}
		if len(refs) > 0 {
			if err := tx.Create(&refs).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
