package db

import (
	"gorm.io/gorm"
)

type MailboxMessageRow struct {
	gorm.Model
	Recipient string `gorm:"column:recipient;not null;index:idx_memory_mailbox_recipient_status_created,priority:1"`
	Sender    string `gorm:"column:sender;not null;default:''"`
	Content   string `gorm:"not null"`
	Status    string `gorm:"not null;index:idx_memory_mailbox_recipient_status_created,priority:2"`
}

func (MailboxMessageRow) TableName() string { return "memory_mailbox" }

func (d *DB) CreateMailboxMessage(msg MailboxMessageRow) (MailboxMessageRow, error) {
	if err := d.db.Create(&msg).Error; err != nil {
		return MailboxMessageRow{}, err
	}
	return msg, nil
}

func (d *DB) PendingMailboxMessages(recipient, status string, limit int) ([]MailboxMessageRow, error) {
	var rows []MailboxMessageRow
	if err := d.db.Where("recipient = ? AND status = ?", recipient, status).Order("created_at ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (d *DB) MarkMailboxMessage(id uint, status string) (bool, error) {
	res := d.db.Model(&MailboxMessageRow{}).Where("id = ?", id).Update("status", status)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}
