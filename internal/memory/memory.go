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
type Service struct {
	mu              sync.RWMutex
	expertName      string
	rootDir         string
	maxL1IndexItems int
	maxL2IndexItems int
}

// New creates the primary memory service.
func New() *Service {
	return newService("")
}

// NewExpert creates an expert-scoped memory service.
func NewExpert(name string) *Service {
	return newService(name)
}

// NewInRoot creates a memory service rooted at an explicit memory directory.
func NewInRoot(rootDir string) *Service {
	s := newService("")
	s.rootDir = rootDir
	return s
}

func newService(expertName string) *Service {
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
	return &Service{
		expertName:      expertName,
		maxL1IndexItems: maxL1IndexItems,
		maxL2IndexItems: maxL2IndexItems,
	}
}

// MemoryDir returns the root directory backing this memory service.
func (s *Service) MemoryDir() string {
	if s == nil {
		return ""
	}
	if s.expertName != "" {
		return global.ExpertMemoryDir(s.expertName)
	}
	if s.rootDir != "" {
		return s.rootDir
	}
	return global.MemoryDir()
}

// LoadFromDisk ensures all layer directories exist on disk.
func (s *Service) LoadFromDisk() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	memoryDir := s.MemoryDir()
	if memoryDir == "" {
		return nil
	}

	for _, dir := range []string{s.l2Dir()} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create %s dir: %w", dir, err)
		}
	}
	if err := writeFileIfMissing(s.l1Path(), ""); err != nil {
		return fmt.Errorf("initialize l1.toml: %w", err)
	}

	log.Printf("[memory] flat-file memory ready at %s", memoryDir)
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
	if s.MemoryDir() == "" {
		return ""
	}
	l1Path := s.l1Path()
	l2Dir := s.l2Dir()
	sections := s.listL1Sections()
	sops := s.listL2SOPNames()
	sb := strings.Builder{}
	if facts := bracketList(sections, s.maxL1IndexItems); facts != "" {
		sb.WriteString("Facts (")
		sb.WriteString(l1Path)
		sb.WriteString("): ")
		sb.WriteString(facts)
		sb.WriteString("\n")
	}
	if sops := bracketList(sops, s.maxL2IndexItems); sops != "" {
		sb.WriteString("SOPs (")
		sb.WriteString(l2Dir)
		sb.WriteString("): ")
		sb.WriteString(sops)
		sb.WriteString("\n")
	}
	return sb.String()
}

// GetL1Content returns the L1 facts TOML file for system prompt injection.
func (s *Service) GetL1Content() string {
	l1Path := s.l1Path()
	if l1Path == "" {
		return ""
	}
	data, err := os.ReadFile(l1Path)
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
	l2Dir := s.l2Dir()
	if l2Dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(l2Dir)
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
	l2Dir := s.l2Dir()
	if l2Dir == "" {
		return "", fmt.Errorf("l2 dir not set")
	}
	data, err := os.ReadFile(filepath.Join(l2Dir, name))
	if err != nil {
		return "", fmt.Errorf("read l2/%s: %w", name, err)
	}
	return string(data), nil
}

func (s *Service) listL1Sections() []string {
	l1Path := s.l1Path()
	if l1Path == "" {
		return nil
	}
	sections, err := listTOMLSections(l1Path)
	if err != nil {
		return nil
	}
	sort.Strings(sections)
	return sections
}

func (s *Service) l1Path() string {
	return global.MemoryL1Path(s.MemoryDir())
}

func (s *Service) l2Dir() string {
	return global.MemoryL2Dir(s.MemoryDir())
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
		return ""
	}
	return "[" + strings.Join(values, ", ") + "]"
}
