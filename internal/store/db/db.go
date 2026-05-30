package db

import (
	"fmt"
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DB struct {
	db *gorm.DB
}

func OpenPath(dbPath string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create sqlite db dir: %w", err)
	}
	gormDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}
	db := &DB{db: gormDB}
	if err := db.init(); err != nil {
		return nil, err
	}
	return db, nil
}

func (d *DB) init() error {
	if err := d.db.Exec("PRAGMA journal_mode = WAL").Error; err != nil {
		return fmt.Errorf("initialize sqlite db: %w", err)
	}
	if err := d.db.AutoMigrate(
		&SessionRow{},
		&MessageRow{},
		&FileRow{},
		&FileRevisionRow{},
		&QueuedPromptRow{},
		&ReferenceRow{},
		&MailboxMessageRow{},
		&MemoryIndexRow{},
		&MemoryIndexMetaRow{},
	); err != nil {
		return fmt.Errorf("migrate sqlite db: %w", err)
	}
	return nil
}
