package expert

import (
	"encoding/json"
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
	SessionIDs   []string
}

// Candidate is a reviewable expert extraction or optimization proposal.
type Candidate struct {
	ID             string    `json:"id"`
	Type           string    `json:"type"`
	Name           string    `json:"name"`
	Summary        string    `json:"summary"`
	Description    string    `json:"description"`
	TriggerPattern string    `json:"trigger_pattern"`
	ToolAllowlist  []string  `json:"tools"`
	SystemPrompt   string    `json:"system_prompt"`
	Confidence     float64   `json:"confidence"`
	Evidence       []string  `json:"evidence,omitempty"`
	Source         string    `json:"source"`
	Reason         string    `json:"reason"`
	CreatedAt      time.Time `json:"created_at"`
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
		sessionIDs := make([]string, 0, len(group))
		for i, c := range group {
			totalTools += c.ToolCount
			tasks[i] = c.TaskSummary
			sessionIDs = append(sessionIDs, c.SessionID)
		}
		sort.Strings(sessionIDs)

		clusters = append(clusters, ToolChainCluster{
			TaskSummary:  findCommonPrefix(tasks),
			ToolCount:    totalTools,
			SessionCount: len(group),
			FileTypes:    group[0].FileTypes,
			Confidence:   calculateConfidence(len(group), totalTools),
			SessionIDs:   sessionIDs,
		})
	}

	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Confidence > clusters[j].Confidence
	})
	return clusters
}

// BuildCandidate creates a reviewable expert candidate from a cluster without
// mutating the registry.
func (a *Analyzer) BuildCandidate(cluster ToolChainCluster) Candidate {
	now := time.Now()
	name := generateExpertName(cluster.TaskSummary)
	return Candidate{
		ID:             fmt.Sprintf("expert-cand-%d-%s", now.UnixNano(), name),
		Type:           "create",
		Name:           name,
		Summary:        cluster.TaskSummary,
		Description:    fmt.Sprintf("Automatically extracted from %d similar sessions involving %v files", cluster.SessionCount, cluster.FileTypes),
		TriggerPattern: cluster.TaskSummary,
		ToolAllowlist:  inferTools(cluster.FileTypes),
		SystemPrompt:   fmt.Sprintf("You are a specialist extracted from %d similar task executions.\n\nYour expertise covers: %s\n\nFocus on tasks involving: %v", cluster.SessionCount, cluster.TaskSummary, cluster.FileTypes),
		Confidence:     cluster.Confidence,
		Evidence:       cluster.SessionIDs,
		Source:         "session_analysis",
		Reason:         "Repeated task pattern may benefit from a dedicated expert.",
		CreatedAt:      now,
	}
}

// RunAnalysis performs a full analysis cycle: extract -> cluster -> build
// reviewable candidates. It does not create or update official experts.
func (a *Analyzer) RunAnalysis() ([]Candidate, error) {
	chains, err := a.ExtractToolChains()
	if err != nil {
		return nil, err
	}
	if len(chains) == 0 {
		return a.buildCallLogCandidates(), nil
	}

	clusters := a.Cluster(chains)
	var candidates []Candidate
	for _, c := range clusters {
		if c.Confidence < 0.5 || a.isCovered(c) {
			continue
		}
		candidates = append(candidates, a.BuildCandidate(c))
	}
	candidates = append(candidates, a.buildCallLogCandidates()...)
	return candidates, nil
}

// AutoEvolve applies reviewable analysis candidates automatically. It keeps
// RunAnalysis non-mutating for callers that still want an audit-only flow.
func (a *Analyzer) AutoEvolve() ([]EvolutionRecord, error) {
	if a.registry == nil {
		return nil, nil
	}
	candidates, err := a.RunAnalysis()
	if err != nil {
		return nil, err
	}
	var records []EvolutionRecord
	for _, candidate := range candidates {
		switch candidate.Type {
		case "create":
			spec := Spec{
				Name:           candidate.Name,
				Summary:        candidate.Summary,
				Description:    candidate.Description,
				Domain:         inferDomain(candidate.ToolAllowlist),
				Capabilities:   []string{candidate.Summary},
				TriggerPattern: candidate.TriggerPattern,
				ToolAllowlist:  candidate.ToolAllowlist,
				SystemPrompt:   candidate.SystemPrompt,
				Status:         StatusActive,
				Confidence:     candidate.Confidence,
				CreatedBy:      "auto_evolution",
				Evidence:       candidate.Evidence,
			}
			created, createErr := a.registry.Create(spec)
			if createErr != nil {
				continue
			}
			record := nowRecord(EvolutionCreate, created.Name, candidate.Reason, candidate.Evidence)
			record.Version = created.Version
			_ = a.registry.LogEvolution(record)
			records = append(records, record)
		case "optimize":
			updated := Spec{
				Summary:        candidate.Summary,
				Description:    candidate.Description,
				TriggerPattern: candidate.TriggerPattern,
				ToolAllowlist:  candidate.ToolAllowlist,
				SystemPrompt:   candidate.SystemPrompt + "\n\nImprove reliability by being explicit about assumptions, required inputs, and failure conditions.",
				Status:         StatusActive,
				Confidence:     candidate.Confidence,
				CreatedBy:      "auto_evolution",
				Evidence:       candidate.Evidence,
			}
			created, versionErr := a.registry.CreateVersion(candidate.Name, updated)
			if versionErr != nil {
				continue
			}
			record := nowRecord(EvolutionOptimize, created.Name, candidate.Reason, candidate.Evidence)
			record.Version = created.Version
			_ = a.registry.LogEvolution(record)
			records = append(records, record)
		}
	}
	return records, nil
}

// WriteCandidates appends expert candidates to candidateDir/candidates.jsonl.
func (a *Analyzer) WriteCandidates(candidateDir string, candidates []Candidate) error {
	if len(candidates) == 0 {
		return nil
	}
	if candidateDir == "" {
		return fmt.Errorf("candidate dir is empty")
	}
	if err := os.MkdirAll(candidateDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(candidateDir, "candidates.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, candidate := range candidates {
		if err := enc.Encode(candidate); err != nil {
			return err
		}
	}
	return nil
}

func (a *Analyzer) isCovered(cluster ToolChainCluster) bool {
	if a.registry == nil {
		return false
	}
	results := a.registry.Search(cluster.TaskSummary)
	for _, spec := range results {
		if spec.Status == StatusArchived {
			continue
		}
		if strings.EqualFold(spec.Name, generateExpertName(cluster.TaskSummary)) {
			return true
		}
		if jaccard(tokenSet(spec.TriggerPattern+" "+spec.Summary), tokenSet(cluster.TaskSummary)) >= 0.5 {
			return true
		}
	}
	return false
}

func (a *Analyzer) buildCallLogCandidates() []Candidate {
	if a.registry == nil {
		return nil
	}
	var candidates []Candidate
	now := time.Now()
	for _, spec := range a.registry.List() {
		if spec.Status == StatusArchived {
			continue
		}
		records := a.registry.GetCallRecords(spec.Name)
		stats := ComputeStats(records)
		if stats.TotalCalls < 3 || stats.SuccessRate >= 0.5 {
			continue
		}
		candidates = append(candidates, Candidate{
			ID:             fmt.Sprintf("expert-cand-%d-%s-optimize", now.UnixNano(), spec.Name),
			Type:           "optimize",
			Name:           spec.Name,
			Summary:        "Improve expert reliability: " + spec.Summary,
			Description:    fmt.Sprintf("Expert has %d calls with %.0f%% success rate.", stats.TotalCalls, stats.SuccessRate*100),
			TriggerPattern: spec.TriggerPattern,
			ToolAllowlist:  spec.ToolAllowlist,
			SystemPrompt:   spec.SystemPrompt,
			Confidence:     clamp(1-stats.SuccessRate, 0.5, 1),
			Evidence:       callEvidence(records, 5),
			Source:         "call_log_analysis",
			Reason:         "Repeated failed calls suggest the expert prompt, scope, or tool allowlist should be reviewed.",
			CreatedAt:      now,
		})
	}
	return candidates
}

func callEvidence(records []CallRecord, limit int) []string {
	var evidence []string
	for i := len(records) - 1; i >= 0 && len(evidence) < limit; i-- {
		if records[i].Success {
			continue
		}
		task := strings.TrimSpace(records[i].TaskDesc)
		if task == "" {
			task = string(records[i].Mode)
		}
		evidence = append(evidence, truncate(task, 120))
	}
	return evidence
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

func inferDomain(tools []string) string {
	for _, tool := range tools {
		switch tool {
		case "file_patch", "code_run":
			return "software_engineering"
		case "web_scan", "web_execute":
			return "research"
		}
	}
	return "general"
}

func tokenSet(s string) map[string]bool {
	result := make(map[string]bool)
	for _, field := range strings.Fields(strings.ToLower(s)) {
		token := strings.Trim(field, ".,;:!?()[]{}\"'")
		if len(token) >= 2 {
			result[token] = true
		}
	}
	return result
}

func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	var intersection int
	for token := range a {
		if b[token] {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
