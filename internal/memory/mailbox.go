package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/global"
	persistdb "github.com/ai4next/superman/internal/store/db"
	"gorm.io/gorm"
)

const (
	MailboxStatusPending  = "pending"
	MailboxStatusAccepted = "accepted"
	MailboxStatusRejected = "rejected"
	MailboxStatusDeferred = "deferred"
)

type MailboxService struct {
	cfg *config.Config
}

type MailboxMessage struct {
	ID        string `json:"id"`
	Recipient string `json:"recipient"`
	Sender    string `json:"sender,omitempty"`
	Content   string `json:"content"`
	Status    string `json:"status"`
}

type MailboxUpdate struct {
	ID     string
	Status string
}

func NewMailboxService(cfg *config.Config) *MailboxService {
	if cfg == nil {
		cfg = global.Config()
	}
	return &MailboxService{cfg: cfg}
}

func (m *MailboxService) Send(msg MailboxMessage) (MailboxMessage, error) {
	if m == nil || m.cfg == nil {
		return MailboxMessage{}, fmt.Errorf("memory mailbox config is unavailable")
	}
	if !m.cfg.Memory.Mailbox.Enabled {
		return MailboxMessage{}, fmt.Errorf("memory mailbox is disabled")
	}
	msg.Recipient = strings.TrimSpace(msg.Recipient)
	msg.Content = strings.TrimSpace(msg.Content)
	if msg.Recipient == "" {
		return MailboxMessage{}, fmt.Errorf("recipient is required")
	}
	if msg.Content == "" {
		return MailboxMessage{}, fmt.Errorf("content is required")
	}
	if err := validateRecipient(m.cfg.Workspace, msg.Recipient); err != nil {
		return MailboxMessage{}, err
	}
	if msg.Status == "" {
		msg.Status = MailboxStatusPending
	}
	db, err := openMailboxDB()
	if err != nil {
		return MailboxMessage{}, err
	}
	stored, err := db.CreateMailboxMessage(storeMessageFromMailbox(msg))
	if err != nil {
		return MailboxMessage{}, fmt.Errorf("insert mailbox message: %w", err)
	}
	return mailboxMessageFromStore(stored), nil
}

func (m *MailboxService) Pending(recipient string, limit int) ([]MailboxMessage, error) {
	if m == nil || m.cfg == nil || !m.cfg.Memory.Mailbox.Enabled {
		return nil, nil
	}
	recipient = strings.TrimSpace(recipient)
	if recipient == "" {
		return nil, fmt.Errorf("recipient is required")
	}
	if limit <= 0 {
		limit = 20
	}
	db, err := openMailboxDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.PendingMailboxMessages(recipient, MailboxStatusPending, limit)
	if err != nil {
		return nil, fmt.Errorf("query mailbox: %w", err)
	}
	out := make([]MailboxMessage, len(rows))
	for i, row := range rows {
		out[i] = mailboxMessageFromStore(row)
	}
	return out, nil
}

func (m *MailboxService) Mark(update MailboxUpdate) error {
	if m == nil || m.cfg == nil || !m.cfg.Memory.Mailbox.Enabled {
		return nil
	}
	update.ID = strings.TrimSpace(update.ID)
	update.Status = strings.TrimSpace(update.Status)
	if update.ID == "" || update.Status == "" {
		return fmt.Errorf("id and status are required")
	}
	switch update.Status {
	case MailboxStatusAccepted, MailboxStatusRejected, MailboxStatusDeferred:
	default:
		return fmt.Errorf("invalid mailbox status %q", update.Status)
	}
	db, err := openMailboxDB()
	if err != nil {
		return err
	}
	id, err := strconv.ParseUint(update.ID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid mailbox message id %q", update.ID)
	}
	updated, err := db.MarkMailboxMessage(uint(id), update.Status)
	if err != nil {
		return fmt.Errorf("update mailbox message: %w", err)
	}
	if !updated {
		return fmt.Errorf("mailbox message %q not found", update.ID)
	}
	return nil
}

func openMailboxDB() (*persistdb.DB, error) {
	registry, err := global.DBRegistry()
	if err != nil {
		return nil, err
	}
	return registry.GlobalDB, nil
}

func storeMessageFromMailbox(msg MailboxMessage) persistdb.MailboxMessageRow {
	id, _ := strconv.ParseUint(strings.TrimSpace(msg.ID), 10, 64)
	return persistdb.MailboxMessageRow{
		Model:     gorm.Model{ID: uint(id)},
		Recipient: msg.Recipient,
		Sender:    msg.Sender,
		Content:   msg.Content,
		Status:    msg.Status,
	}
}

func mailboxMessageFromStore(row persistdb.MailboxMessageRow) MailboxMessage {
	return MailboxMessage{
		ID:        strconv.FormatUint(uint64(row.ID), 10),
		Recipient: row.Recipient,
		Sender:    row.Sender,
		Content:   row.Content,
		Status:    row.Status,
	}
}

func validateRecipient(workspace, recipient string) error {
	if recipient == OwnerSuperman {
		return nil
	}
	if recipient == "" || strings.ContainsAny(recipient, `/\`) {
		return fmt.Errorf("invalid recipient %q", recipient)
	}
	if _, err := os.Stat(filepath.Join(workspace, "state", recipient, "soul.md")); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("recipient %q not found", recipient)
		}
		return err
	}
	return nil
}
