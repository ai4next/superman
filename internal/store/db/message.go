package db

import "gorm.io/gorm"

type MessageRow struct {
	gorm.Model
	MessageID    string `gorm:"column:message_id;not null;index"`
	SessionID    int64  `gorm:"not null;index:idx_message_session_order,priority:1"`
	Position     int    `gorm:"not null;index:idx_message_session_order,priority:2"`
	EventID      string `gorm:"index"`
	InvocationID string
	Role         string `gorm:"not null;index"`
	Content      string
	ToolName     string
	ToolID       string `gorm:"index"`
	Args         string
	Result       string
	Status       string
	Summary      bool
}

func (MessageRow) TableName() string { return "message" }

func (d *DB) LoadMessages(sessionID int64) ([]MessageRow, error) {
	var rows []MessageRow
	if err := d.db.Where("session_id = ?", sessionID).Order("position asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (d *DB) SaveMessages(sessionID int64, rows []MessageRow) error {
	for i, row := range rows {
		row.SessionID = sessionID
		row.Position = i
		row.ID = 0
		rows[i] = row
	}
	return d.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("session_id = ?", sessionID).Delete(&MessageRow{}).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}
