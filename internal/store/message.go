package store

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ai4next/superman/internal/global"
	"gorm.io/gorm"
)

type Message struct {
	ID           string
	SessionID    int64
	Position     int
	EventID      string
	InvocationID string
	Role         string
	Content      string
	ToolName     string
	ToolID       string
	Args         string
	Result       string
	Status       string
	Summary      bool
	CreatedAt    string
	UpdatedAt    string
}

type LogMessage struct {
	Role    string
	Content string
	Tool    string
	Args    string
	Result  string
	Summary bool
}

type messageRow struct {
	ID           string `gorm:"primaryKey"`
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
	CreatedAt    string `gorm:"not null;index"`
	UpdatedAt    string `gorm:"not null"`
}

func (messageRow) TableName() string { return "message" }

func (d *DB) LoadMessages(sessionID int64) ([]Message, error) {
	var rows []messageRow
	if err := d.db.Where("session_id = ?", sessionID).Order("position asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]Message, len(rows))
	for i, row := range rows {
		out[i] = Message(row)
	}
	return out, nil
}

func (d *DB) SaveMessages(sessionID int64, rows []Message) error {
	dbRows := make([]messageRow, len(rows))
	for i, row := range rows {
		row.SessionID = sessionID
		row.Position = i
		dbRows[i] = messageRow(row)
	}
	return d.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("session_id = ?", sessionID).Delete(&messageRow{}).Error; err != nil {
			return err
		}
		if len(dbRows) == 0 {
			return nil
		}
		return tx.Create(&dbRows).Error
	})
}

func (d *DB) WriteSessionLog(sessionID string, messages []LogMessage) error {
	path := global.SessionLogPath(safeName(sessionID))
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, FormatSessionLog(messages), 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return nil
}

func FormatSessionLog(messages []LogMessage) []byte {
	var b strings.Builder
	for _, msg := range messages {
		if msg.Summary {
			continue
		}
		switch msg.Role {
		case "user":
			writeLogLine(&b, "U", msg.Content)
		case "assistant":
			writeLogLine(&b, "A", msg.Content)
		case "tool":
			if strings.TrimSpace(msg.Args) != "" {
				writeLogLine(&b, "T", toolCallLog(msg))
			}
			if strings.TrimSpace(msg.Result) != "" {
				writeLogLine(&b, "O", toolOutputLog(msg))
			}
		case "error":
			writeLogLine(&b, "E", msg.Content)
		}
	}
	return []byte(b.String())
}

func writeLogLine(b *strings.Builder, prefix, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	quoted := strconv.Quote(value)
	fmt.Fprintf(b, "%s: %s\n", prefix, quoted[1:len(quoted)-1])
}

func toolCallLog(msg LogMessage) string {
	if strings.TrimSpace(msg.Args) == "" {
		return msg.Tool
	}
	return fmt.Sprintf("%s(%s)", msg.Tool, msg.Args)
}

func toolOutputLog(msg LogMessage) string {
	result := strings.TrimSpace(msg.Result)
	if strings.HasPrefix(result, `{"output":`) {
		if start := strings.Index(result, `:"`); start >= 0 {
			if output, err := strconv.Unquote(result[start+1 : len(result)-1]); err == nil {
				return output
			}
		}
	}
	return result
}
