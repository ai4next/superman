package memory

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/ai4next/superman/internal/global"
)

// Service manages the L0-L3 memory file hierarchy.
//
//	L0: runtime index assembled from l1.toml sections and L2 SOP names
//	L1: l1.toml — global facts grouped by TOML sections
//	L2: l2/ — SOP files
//	L3: l3/raw_sessions/ — raw session JSONL files
type Service struct {
	mu              sync.RWMutex
	memoryDir       string
	l1Path          string
	l2Dir           string
	rawSessionDir   string
	maxL1IndexItems int
	maxL2IndexItems int
}

// New creates a memory service backed by flat files under memoryDir.
func New(memoryDir string) *Service {
	maxL1IndexItems := 50
	maxL2IndexItems := 50
	if cfg := global.Config(); cfg != nil {
		if cfg.Memory.L1.MaxIndexItems > 0 {
			maxL1IndexItems = cfg.Memory.L1.MaxIndexItems
		}
		if cfg.Memory.L2.MaxIndexItems > 0 {
			maxL2IndexItems = cfg.Memory.L2.MaxIndexItems
		}
	}
	s := &Service{
		memoryDir:       memoryDir,
		maxL1IndexItems: maxL1IndexItems,
		maxL2IndexItems: maxL2IndexItems,
	}
	if memoryDir != "" {
		s.l1Path = filepath.Join(memoryDir, "l1.toml")
		s.l2Dir = filepath.Join(memoryDir, "l2")
		s.rawSessionDir = filepath.Join(memoryDir, "l3", "raw_sessions")
	}
	return s
}

// MemoryDir returns the root directory backing this memory service.
func (s *Service) MemoryDir() string {
	if s == nil {
		return ""
	}
	return s.memoryDir
}

// LoadFromDisk ensures all layer directories exist on disk.
func (s *Service) LoadFromDisk() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.memoryDir == "" {
		return nil
	}

	for _, dir := range []string{"l2", "l3/raw_sessions"} {
		if err := os.MkdirAll(filepath.Join(s.memoryDir, dir), 0755); err != nil {
			return fmt.Errorf("create %s dir: %w", dir, err)
		}
	}
	if err := writeFileIfMissing(s.l1Path, ""); err != nil {
		return fmt.Errorf("initialize l1.toml: %w", err)
	}

	log.Printf("[memory] flat-file memory ready at %s", s.memoryDir)
	return nil
}

func writeFileIfMissing(path, content string) error {
	if path == "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}

// GetL0Content returns a runtime L0 index for system prompt injection.
func (s *Service) GetL0Content() string {
	if s.memoryDir == "" {
		return ""
	}
	sections := s.listL1Sections()
	sops := s.listL2SOPNames()
	return fmt.Sprintf("L1 Facts (%s): %s\nL2 SOPs (%s): %s\n", s.l1Path, bracketList(sections, s.maxL1IndexItems), s.l2Dir, bracketList(sops, s.maxL2IndexItems))
}

// GetL1Content returns the L1 facts TOML file for system prompt injection.
func (s *Service) GetL1Content() string {
	if s.l1Path == "" {
		return ""
	}
	data, err := os.ReadFile(s.l1Path)
	if err != nil {
		return ""
	}
	return string(data)
}

// CountL1Sections returns the number of TOML sections in an L1 facts file.
func CountL1Sections(path string) int {
	sections, err := listTOMLSections(path)
	if err != nil {
		return 0
	}
	return len(sections)
}

// ListSOPFiles returns sorted .md SOP file names in the l2/ directory.
func (s *Service) ListSOPFiles() ([]string, error) {
	if s.l2Dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(s.l2Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read l2 dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// ReadSOPFile returns the content of a specific SOP file from l2/.
func (s *Service) ReadSOPFile(name string) (string, error) {
	if s.l2Dir == "" {
		return "", fmt.Errorf("l2 dir not set")
	}
	data, err := os.ReadFile(filepath.Join(s.l2Dir, name))
	if err != nil {
		return "", fmt.Errorf("read l2/%s: %w", name, err)
	}
	return string(data), nil
}

func (s *Service) listL1Sections() []string {
	if s.l1Path == "" {
		return nil
	}
	sections, err := listTOMLSections(s.l1Path)
	if err != nil {
		return nil
	}
	sort.Strings(sections)
	return sections
}

func listTOMLSections(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	seen := make(map[string]bool)
	var sections []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") || strings.HasPrefix(line, "[[") {
			continue
		}
		name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		sections = append(sections, name)
	}
	sort.Strings(sections)
	return sections, nil
}

func (s *Service) listL2SOPNames() []string {
	names, err := s.ListSOPFiles()
	if err != nil {
		return nil
	}
	return names
}

func bracketList(values []string, maxItems int) string {
	if len(values) > maxItems {
		values = append(values[:maxItems], "etc...")
	}
	if len(values) == 0 {
		return "[]"
	}
	return "[" + strings.Join(values, ", ") + "]"
}
