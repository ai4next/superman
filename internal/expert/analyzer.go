package expert

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ToolChain represents a sequence of tool calls extracted from one session.
type ToolChain struct {
	SessionID   string
	TaskSummary string
	ToolCount   int
	FileTypes   []string
	Duration    time.Duration
	Success     bool
}

// ToolChainCluster groups similar tool chains for expert extraction.
type ToolChainCluster struct {
	TaskSummary  string
	ToolCount    int
	SessionCount int
	FileTypes    []string
	Confidence   float64
}

// Analyzer reads session logs and extracts expert drafts from repeated patterns.
type Analyzer struct {
	sessionDir string
	registry   *Registry
}

// NewAnalyzer creates an Analyzer that reads sessions from the given directory.
func NewAnalyzer(sessionDir string, registry *Registry) *Analyzer {
	return &Analyzer{
		sessionDir: sessionDir,
		registry:   registry,
	}
}

// ExtractToolChains reads session JSONL files and returns tool chains.
func (a *Analyzer) ExtractToolChains() ([]ToolChain, error) {
	entries, err := os.ReadDir(a.sessionDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read session dir: %w", err)
	}

	var chains []ToolChain
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(a.sessionDir, entry.Name()))
		if err != nil {
			continue
		}
		chain := a.parseSession(entry.Name(), string(data))
		if chain.ToolCount > 0 {
			chains = append(chains, chain)
		}
	}
	return chains, nil
}

func (a *Analyzer) parseSession(name, data string) ToolChain {
	lines := strings.Split(strings.TrimSpace(data), "\n")
	chain := ToolChain{
		SessionID: strings.TrimSuffix(name, ".jsonl"),
	}
	fileSet := make(map[string]bool)

	// Session JSONL format: {"turn":N,"timestamp":"...","user_message":"...","agent_response":"...","tool_calls":N}
	var toolCallsTotal int
	for _, line := range lines {
		if line == "" {
			continue
		}

		// Extract tool_calls count
		if idx := strings.Index(line, `"tool_calls":`); idx > 0 {
			after := line[idx+len(`"tool_calls":`):]
			var tc int
			if _, err := fmt.Sscanf(after, "%d", &tc); err == nil {
				toolCallsTotal += tc
			}
		}

		// Extract user message as task summary from first non-empty message
		if chain.TaskSummary == "" {
			if parts := strings.SplitN(line, `"user_message":`, 2); len(parts) > 1 {
				if end := strings.Index(parts[1], `","agent_response"`); end > 0 {
					msg := strings.Trim(parts[1][:end], `"`)
					if msg != "" {
						chain.TaskSummary = truncate(msg, 100)
					}
				}
			}
		}

		// Detect file types from the message content
		content := strings.ToLower(line)
		if strings.Contains(content, ".go") {
			fileSet[".go"] = true
		}
		if strings.Contains(content, ".sh") || strings.Contains(content, "bash") {
			fileSet[".sh"] = true
		}
		if strings.Contains(content, ".py") {
			fileSet[".py"] = true
		}
		if strings.Contains(content, ".yaml") || strings.Contains(content, ".yml") {
			fileSet[".yaml"] = true
		}
		if strings.Contains(content, ".json") {
			fileSet[".json"] = true
		}
		if strings.Contains(content, ".md") {
			fileSet[".md"] = true
		}
		if strings.Contains(content, ".ts") || strings.Contains(content, ".tsx") || strings.Contains(content, ".js") || strings.Contains(content, ".jsx") {
			fileSet[".js"] = true
		}
	}

	chain.ToolCount = toolCallsTotal
	for ft := range fileSet {
		chain.FileTypes = append(chain.FileTypes, ft)
	}
	sort.Strings(chain.FileTypes)

	return chain
}

// Cluster groups tool chains by similar characteristics (language + tool count bucket).
func (a *Analyzer) Cluster(chains []ToolChain) []ToolChainCluster {
	type clusterKey struct {
		langs       string
		countBucket int
	}

	buckets := make(map[clusterKey][]ToolChain)
	for _, c := range chains {
		langs := strings.Join(c.FileTypes, ",")
		bucket := c.ToolCount / 5 * 5 // bucket by 5: 0-4, 5-9, 10-14...
		key := clusterKey{langs, bucket}
		buckets[key] = append(buckets[key], c)
	}

	var clusters []ToolChainCluster
	for _, group := range buckets {
		if len(group) < 2 {
			continue // need at least 2 sessions to form a pattern
		}
		totalTools := 0
		tasks := make([]string, len(group))
		for i, c := range group {
			totalTools += c.ToolCount
			tasks[i] = c.TaskSummary
		}

		clusters = append(clusters, ToolChainCluster{
			TaskSummary:  findCommonPrefix(tasks),
			ToolCount:    totalTools,
			SessionCount: len(group),
			FileTypes:    group[0].FileTypes,
			Confidence:   calculateConfidence(len(group), totalTools),
		})
	}

	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Confidence > clusters[j].Confidence
	})
	return clusters
}

// GenerateDraft creates an expert draft from a cluster and saves it to the registry.
func (a *Analyzer) GenerateDraft(cluster ToolChainCluster) (*Spec, error) {
	name := generateExpertName(cluster.TaskSummary)
	spec := Spec{
		Name:           name,
		Summary:        cluster.TaskSummary,
		Description:    fmt.Sprintf("Automatically extracted from %d similar sessions involving %v files", cluster.SessionCount, cluster.FileTypes),
		TriggerPattern: cluster.TaskSummary,
		ToolAllowlist:  inferTools(cluster.FileTypes),
		SystemPrompt:   fmt.Sprintf("You are a specialist extracted from %d similar task executions.\n\nYour expertise covers: %s\n\nFocus on tasks involving: %v", cluster.SessionCount, cluster.TaskSummary, cluster.FileTypes),
		Status:         StatusDraft,
		Frequency:      cluster.SessionCount,
		Confidence:     cluster.Confidence,
	}
	return a.registry.Create(spec)
}

// RunAnalysis performs a full analysis cycle: extract -> cluster -> generate drafts.
func (a *Analyzer) RunAnalysis() ([]*Spec, error) {
	chains, err := a.ExtractToolChains()
	if err != nil {
		return nil, err
	}
	if len(chains) == 0 {
		return nil, nil
	}

	clusters := a.Cluster(chains)
	var created []*Spec
	for _, c := range clusters {
		if c.Confidence >= 0.5 {
			draft, err := a.GenerateDraft(c)
			if err != nil {
				continue
			}
			created = append(created, draft)
		}
	}
	return created, nil
}

func findCommonPrefix(tasks []string) string {
	for _, t := range tasks {
		if t != "" {
			return t
		}
	}
	return ""
}

func calculateConfidence(sessionCount, totalTools int) float64 {
	if sessionCount < 2 {
		return 0
	}
	sessionScore := float64(sessionCount) / 20.0
	toolScore := float64(totalTools) / 50.0
	confidence := (sessionScore + toolScore) / 2
	if confidence > 1.0 {
		return 1.0
	}
	return confidence
}

func generateExpertName(summary string) string {
	lower := strings.ToLower(summary)
	parts := strings.Fields(lower)
	if len(parts) > 4 {
		parts = parts[:4]
	}
	name := strings.Join(parts, "-")
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return -1 // remove the character
	}, name)
	name = strings.Trim(name, "-")
	if name == "" {
		return fmt.Sprintf("expert-%d", time.Now().Unix())
	}
	return name
}

func inferTools(fileTypes []string) []string {
	toolSet := make(map[string]bool)
	for _, ft := range fileTypes {
		switch ft {
		case ".go", ".py", ".sh":
			toolSet["file_read"] = true
			toolSet["file_patch"] = true
			toolSet["code_run"] = true
		case ".yaml", ".json", ".md":
			toolSet["file_read"] = true
			toolSet["file_write"] = true
		}
	}
	var tools []string
	for t := range toolSet {
		tools = append(tools, t)
	}
	sort.Strings(tools)
	return tools
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
