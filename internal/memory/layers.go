package memory

import (
	"os"
	"path/filepath"
	"strings"
)

type L0Store struct {
	sopDir string
	rules  map[string]string
}

func NewL0Store(sopDir string) (*L0Store, error) {
	s := &L0Store{
		sopDir: sopDir,
		rules:  make(map[string]string),
	}
	if err := s.Load(); err != nil {
		return s, err
	}
	return s, nil
}

func (s *L0Store) Load() error {
	entries, err := os.ReadDir(s.sopDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".txt") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(s.sopDir, name))
		if err != nil {
			continue
		}
		key := strings.TrimSuffix(name, filepath.Ext(name))
		s.rules[key] = string(content)
	}
	return nil
}

func (s *L0Store) Get(name string) (string, bool) {
	content, ok := s.rules[name]
	return content, ok
}

func (s *L0Store) All() map[string]string {
	result := make(map[string]string, len(s.rules))
	for k, v := range s.rules {
		result[k] = v
	}
	return result
}

func (s *L0Store) Save(name, content string) error {
	if err := os.MkdirAll(s.sopDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(s.sopDir, name+".md")
	return os.WriteFile(path, []byte(content), 0644)
}
